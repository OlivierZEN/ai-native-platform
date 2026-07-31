package console

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/identity"
	"github.com/OlivierZEN/ai-native-platform/internal/tenant"
)

type webVerifierStub struct {
	identity identity.OIDCIdentity
	expiry   time.Time
	subject  string
	nonce    string
}

func (stub *webVerifierStub) VerifyWithExpiration(_ context.Context, raw string) (identity.OIDCIdentity, time.Time, error) {
	if raw != "keycloak-access-token" {
		return identity.OIDCIdentity{}, time.Time{}, context.Canceled
	}
	return stub.identity, stub.expiry, nil
}

func (stub *webVerifierStub) VerifyIDToken(_ context.Context, raw, nonce string) (string, error) {
	if raw != "keycloak-id-token" || nonce != stub.nonce {
		return "", context.Canceled
	}
	return stub.subject, nil
}

type oidcResolverStub struct {
	status tenant.TenantStatus
	found  bool
}

func (stub oidcResolverStub) ResolveActiveCompany(_ context.Context, companyID string) (tenant.TenantStatus, bool, error) {
	if companyID != stub.status.CompanyID {
		return tenant.TenantStatus{}, false, nil
	}
	return stub.status, stub.found, nil
}

func TestWebOIDCLoginCreatesSecureConsoleSession(t *testing.T) {
	now := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	var expectedVerifier string
	var tokenRequests int
	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tokenRequests++
		if request.URL.Path != "/protocol/openid-connect/token" {
			t.Fatalf("token path=%s", request.URL.Path)
		}
		clientID, clientSecret, ok := request.BasicAuth()
		if !ok || clientID != "semattice-web" || clientSecret != "server-only-secret-value" {
			t.Fatalf("unexpected client authentication: id=%q secret=%q", clientID, clientSecret)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if request.Form.Get("grant_type") != "authorization_code" ||
			request.Form.Get("code") != "authorization-code" ||
			request.Form.Get("code_verifier") != expectedVerifier ||
			request.Form.Get("redirect_uri") != "https://semattice.example.test/auth/oidc/callback" {
			t.Fatalf("unexpected token form: %#v", request.Form)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{
			"access_token": "keycloak-access-token",
			"id_token":     "keycloak-id-token",
			"token_type":   "Bearer",
		})
	}))
	defer tokenServer.Close()

	subject := "e0dc8f2d-ebdc-4cb3-95f5-cd0ccc46f7d6"
	companyID := "org2sva14i4udjmi2t4s"
	verifier := &webVerifierStub{
		identity: identity.OIDCIdentity{Subject: subject, Organizations: []string{companyID}},
		expiry:   now.Add(time.Hour),
		subject:  subject,
	}
	login, err := NewOIDCLogin(OIDCConfig{
		Issuer: tokenServer.URL, ClientID: "semattice-web",
		ClientSecret: "server-only-secret-value",
		RedirectURI:  "https://semattice.example.test/auth/oidc/callback",
	}, verifier, oidcResolverStub{found: true, status: tenant.TenantStatus{
		TenantID: "93ff0c87-a626-529e-b8cf-195825df2488", CompanyID: companyID,
		GlobalLifecycleStatus: "active", NativeStatus: "active",
	}}, "console-session-key-material-that-is-long-enough")
	if err != nil {
		t.Fatalf("NewOIDCLogin: %v", err)
	}
	login.now = func() time.Time { return now }
	handler := NewHandler(verifierStub{}, "console-session-key-material-that-is-long-enough", &readerStub{}).EnableOIDC(login)
	handler.now = func() time.Time { return now }

	start := httptest.NewRecorder()
	handler.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/auth/oidc/login", nil))
	if start.Code != http.StatusSeeOther {
		t.Fatalf("login status=%d body=%s", start.Code, start.Body.String())
	}
	location, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatalf("login location: %v", err)
	}
	query := location.Query()
	state := query.Get("state")
	attempt := login.attempts[state]
	expectedVerifier = attempt.verifier
	verifier.nonce = attempt.nonce
	challenge := sha256.Sum256([]byte(attempt.verifier))
	if location.Path != "/protocol/openid-connect/auth" ||
		query.Get("client_id") != "semattice-web" ||
		query.Get("redirect_uri") != "https://semattice.example.test/auth/oidc/callback" ||
		query.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(challenge[:]) ||
		query.Get("code_challenge_method") != "S256" ||
		!strings.Contains(query.Get("scope"), "organization") ||
		strings.Contains(start.Header().Get("Location"), "server-only-secret-value") {
		t.Fatalf("unsafe or incomplete authorize redirect: %s", start.Header().Get("Location"))
	}
	stateCookie := cookieNamed(t, start.Result().Cookies(), oidcStateCookieName)
	if !stateCookie.HttpOnly || !stateCookie.Secure || stateCookie.SameSite != http.SameSiteLaxMode ||
		stateCookie.Path != "/auth/oidc" {
		t.Fatalf("unsafe state cookie: %#v", stateCookie)
	}

	callback := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=authorization-code&state="+url.QueryEscape(state), nil)
	request.AddCookie(stateCookie)
	handler.ServeHTTP(callback, request)
	if callback.Code != http.StatusSeeOther || callback.Header().Get("Location") != "/console/" {
		t.Fatalf("callback status=%d location=%q body=%s", callback.Code, callback.Header().Get("Location"), callback.Body.String())
	}
	sessionCookie := cookieNamed(t, callback.Result().Cookies(), cookieName)
	if !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteLaxMode ||
		sessionCookie.Path != "/console" || strings.Contains(sessionCookie.Value, "keycloak-") {
		t.Fatalf("unsafe session cookie: %#v", sessionCookie)
	}
	sessionResponse := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/console/session", nil)
	request.AddCookie(sessionCookie)
	handler.ServeHTTP(sessionResponse, request)
	if sessionResponse.Code != http.StatusOK ||
		!strings.Contains(sessionResponse.Body.String(), `"authenticated":true`) ||
		!strings.Contains(sessionResponse.Body.String(), `"authorization.read"`) {
		t.Fatalf("session status=%d body=%s", sessionResponse.Code, sessionResponse.Body.String())
	}

	replay := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=authorization-code&state="+url.QueryEscape(state), nil)
	request.AddCookie(stateCookie)
	handler.ServeHTTP(replay, request)
	if replay.Code != http.StatusSeeOther || replay.Header().Get("Location") != "/console/?login=failed" || tokenRequests != 1 {
		t.Fatalf("replay status=%d location=%q token_requests=%d", replay.Code, replay.Header().Get("Location"), tokenRequests)
	}
}

func TestWebOIDCCallbackRejectsStateMismatchBeforeTokenExchange(t *testing.T) {
	var tokenRequests int
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { tokenRequests++ }))
	defer server.Close()
	login, err := NewOIDCLogin(OIDCConfig{
		Issuer: server.URL, ClientID: "semattice-web", ClientSecret: "server-only-secret-value",
		RedirectURI: "https://semattice.example.test/auth/oidc/callback",
	}, &webVerifierStub{}, oidcResolverStub{}, "console-session-key-material-that-is-long-enough")
	if err != nil {
		t.Fatalf("NewOIDCLogin: %v", err)
	}
	handler := NewHandler(verifierStub{}, "console-session-key-material-that-is-long-enough", &readerStub{}).EnableOIDC(login)
	start := httptest.NewRecorder()
	handler.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/auth/oidc/login", nil))
	callback := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=authorization-code&state=attacker-state", nil)
	request.AddCookie(cookieNamed(t, start.Result().Cookies(), oidcStateCookieName))
	handler.ServeHTTP(callback, request)
	if callback.Code != http.StatusSeeOther || callback.Header().Get("Location") != "/console/?login=failed" || tokenRequests != 0 {
		t.Fatalf("state mismatch status=%d location=%q requests=%d", callback.Code, callback.Header().Get("Location"), tokenRequests)
	}
}

func TestWebOIDCCallbackRejectsAmbiguousOrInactiveOrganization(t *testing.T) {
	now := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"access_token":"keycloak-access-token","id_token":"keycloak-id-token","token_type":"Bearer"}`))
	}))
	defer server.Close()
	companyID := "org2sva14i4udjmi2t4s"
	tests := []struct {
		name          string
		organizations []string
		globalStatus  string
		nativeStatus  string
	}{
		{name: "multiple organizations", organizations: []string{companyID, "orgaaaaaaaaaaaaaaaaa"}, globalStatus: "active", nativeStatus: "active"},
		{name: "inactive organization", organizations: []string{companyID}, globalStatus: "active", nativeStatus: "suspended"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			subject := "e0dc8f2d-ebdc-4cb3-95f5-cd0ccc46f7d6"
			verifier := &webVerifierStub{
				identity: identity.OIDCIdentity{Subject: subject, Organizations: test.organizations},
				expiry:   now.Add(time.Hour), subject: subject,
			}
			login, err := NewOIDCLogin(OIDCConfig{
				Issuer: server.URL, ClientID: "semattice-web", ClientSecret: "server-only-secret-value",
				RedirectURI: "https://semattice.example.test/auth/oidc/callback",
			}, verifier, oidcResolverStub{found: true, status: tenant.TenantStatus{
				TenantID: "93ff0c87-a626-529e-b8cf-195825df2488", CompanyID: companyID,
				GlobalLifecycleStatus: test.globalStatus, NativeStatus: test.nativeStatus,
			}}, "console-session-key-material-that-is-long-enough")
			if err != nil {
				t.Fatalf("NewOIDCLogin: %v", err)
			}
			login.now = func() time.Time { return now }
			handler := NewHandler(verifierStub{}, "console-session-key-material-that-is-long-enough", &readerStub{}).EnableOIDC(login)
			start := httptest.NewRecorder()
			handler.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/auth/oidc/login", nil))
			location, _ := url.Parse(start.Header().Get("Location"))
			state := location.Query().Get("state")
			verifier.nonce = login.attempts[state].nonce
			callback := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=authorization-code&state="+url.QueryEscape(state), nil)
			request.AddCookie(cookieNamed(t, start.Result().Cookies(), oidcStateCookieName))
			handler.ServeHTTP(callback, request)
			if callback.Code != http.StatusSeeOther || callback.Header().Get("Location") != "/console/?login=failed" {
				t.Fatalf("callback status=%d location=%q", callback.Code, callback.Header().Get("Location"))
			}
			for _, cookie := range callback.Result().Cookies() {
				if cookie.Name == cookieName {
					t.Fatalf("rejected login created console session: %#v", cookie)
				}
			}
		})
	}
}

func cookieNamed(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found: %#v", name, cookies)
	return nil
}
