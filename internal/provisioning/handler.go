package provisioning

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/config"
	"github.com/OlivierZEN/ai-native-platform/internal/tenant"
	"github.com/google/uuid"
)

var (
	companyIDPattern      = regexp.MustCompile(`^org[a-z0-9]{17}$`)
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$`)
)

type handler struct {
	service *tenant.Service
	cfg     config.Provisioning
	agent   *agentCiCiClient
	guard   *replayGuard
}

type provisionRequest struct {
	CompanyID      string          `json:"company_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	DisplayName    string          `json:"display_name"`
	ServiceTier    string          `json:"service_tier"`
	Entitlements   json.RawMessage `json:"entitlements"`
}

func NewHandler(service *tenant.Service, cfg config.Provisioning) http.Handler {
	return &handler{service: service, cfg: cfg, agent: newAgentCiCiClient(cfg), guard: newReplayGuard()}
}

func (h *handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusNotFound, capability.CodeCapabilityNotFound)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 1<<20))
	if err != nil {
		writeError(writer, http.StatusBadRequest, capability.CodeValidationFailed)
		return
	}
	serviceID := request.Header.Get("X-Internal-Service")
	key, allowed := h.cfg.CallerKeys[serviceID]
	if !allowed || verifySignature(key, serviceID, request.Method, request.URL.Path, request.Header.Get("X-Internal-Timestamp"), request.Header.Get("X-Internal-Nonce"), request.Header.Get("X-Internal-Signature"), body, h.guard, time.Now().UTC()) != nil {
		writeError(writer, http.StatusForbidden, capability.CodeUnauthorized)
		return
	}
	var input provisionRequest
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		!companyIDPattern.MatchString(input.CompanyID) || !idempotencyKeyPattern.MatchString(input.IdempotencyKey) ||
		strings.TrimSpace(input.DisplayName) == "" || len(input.DisplayName) > 256 ||
		strings.TrimSpace(input.ServiceTier) == "" || len(input.ServiceTier) > 64 || !validEntitlements(input.Entitlements) {
		writeError(writer, http.StatusBadRequest, capability.CodeValidationFailed)
		return
	}
	result, stableErr := h.provision(request.Context(), serviceID, input)
	if stableErr != nil {
		writeError(writer, httpStatus(stableErr.Code), stableErr.Code)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{"status": "succeeded", "result": result})
}

func (h *handler) provision(ctx context.Context, serviceID string, input provisionRequest) (tenant.TenantStatus, *capability.StableError) {
	key := serviceID + ":" + input.IdempotencyKey
	reservation, err := h.agent.reserve(ctx, input.CompanyID, key)
	if err != nil || reservation.ReservationID == "" || reservation.CompanyID != input.CompanyID ||
		(reservation.State != "RESERVED" && reservation.State != "FAILED" && reservation.State != "PROVISIONED") {
		return tenant.TenantStatus{}, &capability.StableError{Code: capability.CodeFailedPrecondition, Message: "company is not eligible for provisioning"}
	}
	tenantID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("semattice:"+key))
	operationID := "internal:" + key
	status, stableErr := h.service.ProvisionFromVerifiedCompany(ctx, serviceID, key, tenant.ProvisionInput{OperationID: operationID, TenantID: tenantID.String(), CompanyID: input.CompanyID, TenantRevision: 1, ProductRevision: 1, DisplayName: input.DisplayName, ServiceTier: input.ServiceTier, GlobalLifecycleStatus: "active", Entitlements: input.Entitlements})
	complete := map[string]any{"companyId": input.CompanyID, "tenantId": tenantID.String(), "operationId": operationID, "succeeded": stableErr == nil}
	if stableErr != nil {
		complete["failureCode"] = stableErr.Code
	}
	if err := h.agent.complete(ctx, reservation.ReservationID, complete); err != nil && stableErr == nil {
		return tenant.TenantStatus{}, &capability.StableError{Code: capability.CodeInternal, Message: "AgentCiCi completion was not acknowledged"}
	}
	return status, stableErr
}

func writeError(writer http.ResponseWriter, status int, code capability.ErrorCode) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"status": "failed", "error": map[string]string{"code": string(code)}})
}
func httpStatus(code capability.ErrorCode) int {
	if code == capability.CodeValidationFailed {
		return http.StatusBadRequest
	}
	if code == capability.CodeConflict || code == capability.CodeIdempotencyConflict {
		return http.StatusConflict
	}
	if code == capability.CodeFailedPrecondition {
		return http.StatusPreconditionFailed
	}
	return http.StatusInternalServerError
}

func validEntitlements(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var value map[string]any
	return json.Unmarshal(raw, &value) == nil && value != nil
}
