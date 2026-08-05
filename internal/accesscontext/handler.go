package accesscontext

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/config"
	"github.com/OlivierZEN/ai-native-platform/internal/identity"
	"github.com/OlivierZEN/ai-native-platform/internal/tenant"
)

type TenantResolver interface {
	ResolveActiveCompany(context.Context, string) (tenant.TenantStatus, bool, error)
}

type SubjectVerifier interface {
	Verify(context.Context, string) (identity.OIDCIdentity, error)
}

type TokenSigner interface {
	SignHuman(string, string, string, []string, time.Time, time.Duration) (string, time.Time, error)
	SignService(string, string, string, string, string, []string, time.Time, time.Duration) (string, time.Time, error)
}

type handler struct {
	resolver TenantResolver
	verifier SubjectVerifier
	signer   TokenSigner
	allowed  map[string]struct{}
	services map[string]config.ServiceAccessBinding
	ttl      time.Duration
	now      func() time.Time
}

type tokenRequest struct {
	RequestedScopes []string `json:"requested_scopes"`
}

func NewHandler(resolver TenantResolver, verifier SubjectVerifier, signer TokenSigner, allowedScopes []string,
	serviceBindings map[string]config.ServiceAccessBinding, ttl time.Duration) http.Handler {
	allowed := make(map[string]struct{}, len(allowedScopes))
	for _, scope := range allowedScopes {
		allowed[scope] = struct{}{}
	}
	return &handler{
		resolver: resolver, verifier: verifier, signer: signer,
		allowed: allowed, services: serviceBindings, ttl: ttl, now: time.Now,
	}
}

func (h *handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Pragma", "no-cache")
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusNotFound, "not_found")
		return
	}
	rawToken, ok := bearerToken(request.Header.Get("Authorization"))
	if !ok {
		writeError(writer, http.StatusUnauthorized, "invalid_token")
		return
	}
	subject, err := h.verifier.Verify(request.Context(), rawToken)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "invalid_token")
		return
	}
	serviceBinding, serviceAccess := h.services[subject.ClientID]
	if (!serviceAccess && len(subject.Organizations) != 1) ||
		(serviceAccess && len(subject.Organizations) > 0 &&
			(len(subject.Organizations) != 1 || subject.Organizations[0] != serviceBinding.CompanyID)) {
		writeError(writer, http.StatusConflict, "organization_selection_required")
		return
	}
	var input tokenRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeError(writer, http.StatusBadRequest, "invalid_request")
		return
	}
	scopes, ok := h.requestedScopes(input.RequestedScopes)
	if !ok {
		writeError(writer, http.StatusForbidden, "invalid_scope")
		return
	}
	companyID := serviceBinding.CompanyID
	if !serviceAccess {
		companyID = subject.Organizations[0]
	}
	status, found, err := h.resolver.ResolveActiveCompany(request.Context(), companyID)
	if err != nil {
		writeError(writer, http.StatusServiceUnavailable, "service_unavailable")
		return
	}
	if !found {
		writeError(writer, http.StatusNotFound, "organization_not_provisioned")
		return
	}
	if status.CompanyID != companyID || status.GlobalLifecycleStatus != "active" || status.NativeStatus != "active" {
		writeError(writer, http.StatusForbidden, "organization_inactive")
		return
	}
	issuedAt := h.now().UTC()
	var token string
	var expiresAt time.Time
	if serviceAccess {
		token, expiresAt, err = h.signer.SignService(subject.Subject, serviceBinding.OwnerPrincipalID,
			subject.ClientID, status.TenantID, status.CompanyID, scopes, issuedAt, h.ttl)
	} else {
		token, expiresAt, err = h.signer.SignHuman(
			subject.Subject, status.TenantID, status.CompanyID, scopes, issuedAt, h.ttl)
	}
	if err != nil {
		writeError(writer, http.StatusServiceUnavailable, "service_unavailable")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   max(0, int64(expiresAt.Sub(issuedAt).Seconds())),
		"tenant_id":    status.TenantID,
		"company_id":   status.CompanyID,
	})
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	returnValue := ""
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		returnValue = parts[1]
	}
	return returnValue, returnValue != ""
}

func (h *handler) requestedScopes(requested []string) ([]string, bool) {
	if len(requested) == 0 || len(requested) > 64 {
		return nil, false
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(requested))
	for _, scope := range requested {
		if scope == "" {
			return nil, false
		}
		if _, allowed := h.allowed[scope]; !allowed {
			return nil, false
		}
		if _, duplicate := seen[scope]; duplicate {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	return result, len(result) > 0
}

func writeError(writer http.ResponseWriter, status int, code string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": code})
}
