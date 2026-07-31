package console

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

const cookieName = "semattice_console_session"

type IdentityVerifier interface {
	VerifyWithExpiration(context.Context, string) (capability.TrustedPrincipal, time.Time, error)
}

type Handler struct {
	verifier IdentityVerifier
	reader   Reader
	key      []byte
	now      func() time.Time
}

type session struct {
	TenantID  string   `json:"tenant_id"`
	CompanyID string   `json:"company_id"`
	Subject   string   `json:"subject"`
	Scopes    []string `json:"scopes"`
	ID        string   `json:"id"`
	ExpiresAt int64    `json:"exp"`
}

func NewHandler(verifier IdentityVerifier, key string, reader Reader) *Handler {
	if verifier == nil || len(key) < 32 || reader == nil {
		panic("console handler requires verifier, reader and 32-byte session key")
	}
	return &Handler{verifier: verifier, reader: reader, key: []byte(key), now: time.Now}
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/console/session" {
		handler.session(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/console/api/") || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	s, ok := handler.currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "需要有效的管理中心会话")
		return
	}
	if !hasManagementScope(s.Scopes) {
		writeError(w, http.StatusForbidden, "当前用户没有管理权限")
		return
	}
	value, err := handler.reader.Read(r.Context(), s, r.URL.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "无法读取当前租户治理数据")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (handler *Handler) session(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		token, ok := bearer(r.Header.Get("Authorization"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "需要有效的统一身份凭据")
			return
		}
		principal, expiry, err := handler.verifier.VerifyWithExpiration(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "统一身份凭据无效或已过期")
			return
		}
		if !hasManagementScope(principal.Actor.Scopes) {
			writeError(w, http.StatusForbidden, "当前用户没有管理权限")
			return
		}
		now := handler.now()
		if !expiry.After(now) {
			writeError(w, http.StatusUnauthorized, "统一身份凭据已过期")
			return
		}
		until := expiry
		if cap := now.Add(15 * time.Minute); until.After(cap) {
			until = cap
		}
		id := make([]byte, 16)
		if _, err := rand.Read(id); err != nil {
			writeError(w, http.StatusInternalServerError, "会话初始化失败")
			return
		}
		s := session{TenantID: principal.TenantID, CompanyID: principal.CompanyID, Subject: principal.Actor.ID, Scopes: principal.Actor.Scopes, ID: base64.RawURLEncoding.EncodeToString(id), ExpiresAt: until.Unix()}
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: handler.sign(s), Path: "/console", MaxAge: int(time.Until(until).Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
		writeJSON(w, http.StatusCreated, publicSession(s))
	case http.MethodGet:
		s, ok := handler.currentSession(r)
		if !ok {
			// An anonymous browser loading the static console shell is an expected
			// state, not an API failure. Returning an explicit public state avoids a
			// noisy browser-network error while keeping every governance endpoint
			// fail-closed behind the signed cookie.
			writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
			return
		}
		writeJSON(w, http.StatusOK, publicSession(s))
	case http.MethodDelete:
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/console", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "不支持的会话操作")
	}
}

func (handler *Handler) sign(s session) string {
	raw, _ := json.Marshal(s)
	mac := hmac.New(sha256.New, handler.key)
	mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (handler *Handler) currentSession(r *http.Request) (session, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return session{}, false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return session{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return session{}, false
	}
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return session{}, false
	}
	mac := hmac.New(sha256.New, handler.key)
	mac.Write(raw)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return session{}, false
	}
	var s session
	if json.Unmarshal(raw, &s) != nil || s.ExpiresAt <= handler.now().Unix() || s.TenantID == "" || s.CompanyID == "" || s.Subject == "" || !hasManagementScope(s.Scopes) {
		return session{}, false
	}
	return s, true
}
func bearer(header string) (string, bool) {
	parts := strings.Fields(header)
	returnValue := ""
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		returnValue = parts[1]
	}
	return returnValue, returnValue != ""
}
func hasManagementScope(scopes []string) bool {
	for _, scope := range scopes {
		switch scope {
		case "authorization.manage", "organization.manage", "system.manage", "audit.read":
			return true
		}
	}
	return false
}
func publicSession(s session) map[string]any {
	return map[string]any{"authenticated": true, "company_id": s.CompanyID, "subject": s.Subject, "scopes": s.Scopes, "expires_at": time.Unix(s.ExpiresAt, 0).UTC().Format(time.RFC3339)}
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}
