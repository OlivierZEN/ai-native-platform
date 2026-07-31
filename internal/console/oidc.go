package console

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/identity"
	"github.com/OlivierZEN/ai-native-platform/internal/tenant"
)

const (
	oidcStateCookieName = "semattice_oidc_state"
	oidcStateTTL        = 5 * time.Minute
	consoleSessionTTL   = 15 * time.Minute
	maxOIDCAttempts     = 1024
)

type WebOIDCVerifier interface {
	VerifyWithExpiration(context.Context, string) (identity.OIDCIdentity, time.Time, error)
	VerifyIDToken(context.Context, string, string) (string, error)
}

type OIDCTenantResolver interface {
	ResolveActiveCompany(context.Context, string) (tenant.TenantStatus, bool, error)
}

type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type oidcAttempt struct {
	nonce    string
	verifier string
	expires  time.Time
}

type OIDCLogin struct {
	config   OIDCConfig
	verifier WebOIDCVerifier
	resolver OIDCTenantResolver
	key      []byte
	client   *http.Client
	now      func() time.Time
	mu       sync.Mutex
	attempts map[string]oidcAttempt
}

func NewOIDCLogin(config OIDCConfig, verifier WebOIDCVerifier, resolver OIDCTenantResolver, sessionKey string) (*OIDCLogin, error) {
	issuer, issuerErr := url.Parse(config.Issuer)
	redirect, redirectErr := url.Parse(config.RedirectURI)
	if issuerErr != nil || redirectErr != nil || !secureOIDCURL(issuer) || !secureOIDCURL(redirect) ||
		redirect.Path != "/auth/oidc/callback" || config.ClientID == "" ||
		len(config.ClientSecret) < 16 || len(sessionKey) < 32 || verifier == nil || resolver == nil {
		return nil, fmt.Errorf("console OIDC configuration is invalid")
	}
	return &OIDCLogin{
		config: config, verifier: verifier, resolver: resolver, key: []byte(sessionKey),
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		now: time.Now, attempts: map[string]oidcAttempt{},
	}, nil
}

func (login *OIDCLogin) ServeHTTP(handler *Handler, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Pragma", "no-cache")
	switch r.URL.Path {
	case "/auth/oidc/login":
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		login.start(w, r)
	case "/auth/oidc/callback":
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		login.callback(handler, w, r)
	default:
		http.NotFound(w, r)
	}
}

func (login *OIDCLogin) start(w http.ResponseWriter, r *http.Request) {
	state, err := randomValue()
	if err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	nonce, err := randomValue()
	if err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	verifier, err := randomValue()
	if err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	now := login.now().UTC()
	login.mu.Lock()
	for key, attempt := range login.attempts {
		if !attempt.expires.After(now) {
			delete(login.attempts, key)
		}
	}
	if len(login.attempts) >= maxOIDCAttempts {
		login.mu.Unlock()
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	login.attempts[state] = oidcAttempt{nonce: nonce, verifier: verifier, expires: now.Add(oidcStateTTL)}
	login.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: oidcStateCookieName, Value: login.signState(state), Path: "/auth/oidc",
		MaxAge: int(oidcStateTTL.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
	challenge := sha256.Sum256([]byte(verifier))
	parameters := url.Values{
		"client_id":             {login.config.ClientID},
		"redirect_uri":          {login.config.RedirectURI},
		"response_type":         {"code"},
		"scope":                 {"openid profile email organization"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
	http.Redirect(w, r, strings.TrimRight(login.config.Issuer, "/")+"/protocol/openid-connect/auth?"+parameters.Encode(), http.StatusSeeOther)
}

func (login *OIDCLogin) callback(handler *Handler, w http.ResponseWriter, r *http.Request) {
	login.clearStateCookie(w)
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	cookie, err := r.Cookie(oidcStateCookieName)
	if err != nil || state == "" || code == "" || !login.verifyState(cookie.Value, state) {
		login.fail(w, r)
		return
	}
	login.mu.Lock()
	attempt, found := login.attempts[state]
	delete(login.attempts, state)
	login.mu.Unlock()
	if !found || !attempt.expires.After(login.now().UTC()) {
		login.fail(w, r)
		return
	}
	accessToken, idToken, err := login.exchange(r.Context(), code, attempt.verifier)
	if err != nil {
		login.fail(w, r)
		return
	}
	subject, err := login.verifier.VerifyIDToken(r.Context(), idToken, attempt.nonce)
	if err != nil {
		login.fail(w, r)
		return
	}
	verified, expiry, err := login.verifier.VerifyWithExpiration(r.Context(), accessToken)
	if err != nil || verified.Subject != subject || len(verified.Organizations) != 1 {
		login.fail(w, r)
		return
	}
	companyID := verified.Organizations[0]
	status, found, err := login.resolver.ResolveActiveCompany(r.Context(), companyID)
	if err != nil || !found || status.CompanyID != companyID ||
		status.GlobalLifecycleStatus != "active" || status.NativeStatus != "active" {
		login.fail(w, r)
		return
	}
	now := login.now().UTC()
	until := expiry
	if cap := now.Add(consoleSessionTTL); until.After(cap) {
		until = cap
	}
	if !until.After(now) {
		login.fail(w, r)
		return
	}
	id, err := randomValue()
	if err != nil {
		login.fail(w, r)
		return
	}
	handler.setSessionCookie(w, session{
		TenantID: status.TenantID, CompanyID: status.CompanyID, Subject: subject,
		Scopes: []string{"authorization.read"}, ID: id, ExpiresAt: until.Unix(),
	}, until.Sub(now))
	http.Redirect(w, r, "/console/", http.StatusSeeOther)
}

func (login *OIDCLogin) exchange(ctx context.Context, code, verifier string) (string, string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {login.config.RedirectURI},
		"code_verifier": {verifier},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(login.config.Issuer, "/")+"/protocol/openid-connect/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("create token request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(login.config.ClientID, login.config.ClientSecret)
	response, err := login.client.Do(request)
	if err != nil {
		return "", "", fmt.Errorf("exchange authorization code")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return "", "", fmt.Errorf("authorization code rejected")
	}
	var result struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		TokenType   string `json:"token_type"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if decoder.Decode(&result) != nil || result.AccessToken == "" || result.IDToken == "" ||
		!strings.EqualFold(result.TokenType, "Bearer") {
		return "", "", fmt.Errorf("invalid token response")
	}
	return result.AccessToken, result.IDToken, nil
}

func (login *OIDCLogin) signState(state string) string {
	mac := hmac.New(sha256.New, login.key)
	_, _ = mac.Write([]byte("oidc-state:" + state))
	return state + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (login *OIDCLogin) verifyState(signed, state string) bool {
	parts := strings.Split(signed, ".")
	if len(parts) != 2 || parts[0] != state {
		return false
	}
	expected := login.signState(state)
	return hmac.Equal([]byte(expected), []byte(signed))
}

func (login *OIDCLogin) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: oidcStateCookieName, Value: "", Path: "/auth/oidc",
		MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
}

func (login *OIDCLogin) fail(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/console/?login=failed", http.StatusSeeOther)
}

func randomValue() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func secureOIDCURL(value *url.URL) bool {
	if value == nil || value.Host == "" || value.User != nil || value.RawQuery != "" || value.Fragment != "" {
		return false
	}
	if value.Scheme == "https" {
		return true
	}
	hostname := value.Hostname()
	return value.Scheme == "http" && (hostname == "127.0.0.1" || hostname == "localhost" || hostname == "::1")
}
