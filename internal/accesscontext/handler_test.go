package accesscontext

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/config"
	"github.com/OlivierZEN/ai-native-platform/internal/identity"
	"github.com/OlivierZEN/ai-native-platform/internal/tenant"
)

const (
	testCompanyID = "org2sva14i4udjmi2t4s"
	testTenantID  = "93ff0c87-a626-529e-b8cf-195825df2488"
	testSubjectID = "e0dc8f2d-ebdc-4cb3-95f5-cd0ccc46f7d6"
	testOACTKey   = "0123456789abcdef0123456789abcdef"
)

type resolverStub struct {
	status tenant.TenantStatus
	found  bool
	err    error
}

func (stub resolverStub) ResolveActiveCompany(_ context.Context, companyID string) (tenant.TenantStatus, bool, error) {
	if companyID != testCompanyID {
		return tenant.TenantStatus{}, false, nil
	}
	return stub.status, stub.found, stub.err
}

type verifierStub struct {
	identity identity.OIDCIdentity
	err      error
}

func (stub verifierStub) Verify(context.Context, string) (identity.OIDCIdentity, error) {
	return stub.identity, stub.err
}

func TestTokenExchangeIssuesSematticeOACTForMappedOrganization(t *testing.T) {
	signer, err := identity.NewSigner(config.Identity{
		Issuer: "https://semattice.example.test", Audience: "semattice-api",
		Algorithm: "HS256", HMACKey: testOACTKey,
	})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	resolver := resolverStub{found: true, status: tenant.TenantStatus{
		TenantID: testTenantID, CompanyID: testCompanyID,
		GlobalLifecycleStatus: "active", NativeStatus: "active",
	}}
	h := NewHandler(resolver, verifierStub{identity: identity.OIDCIdentity{
		Subject: testSubjectID, ClientID: "storefront-web", Organizations: []string{testCompanyID},
	}}, signer, []string{"system.capability.read", "record.read"}, nil, 10*time.Minute)

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/token",
		bytes.NewBufferString(`{"requested_scopes":["system.capability.read","system.capability.read"]}`))
	request.Header.Set("Authorization", "Bearer keycloak-token")
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
		TenantID    string `json:"tenant_id"`
		CompanyID   string `json:"company_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.AccessToken == "" || response.TokenType != "Bearer" || response.ExpiresIn != 600 ||
		response.TenantID != testTenantID || response.CompanyID != testCompanyID {
		t.Fatalf("unexpected response: %#v", response)
	}
	oactVerifier, err := identity.NewVerifier(config.Identity{
		Issuer: "https://semattice.example.test", Audience: "semattice-api",
		Algorithm: "HS256", HMACKey: testOACTKey,
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	principal, err := oactVerifier.Verify(context.Background(), response.AccessToken)
	if err != nil {
		t.Fatalf("issued OACT was rejected: %v", err)
	}
	if principal.PrincipalID != testSubjectID || principal.PrincipalType != "HUMAN" ||
		principal.TenantID != testTenantID || principal.CompanyID != testCompanyID ||
		len(principal.Actor.Scopes) != 1 || principal.Actor.Scopes[0] != "system.capability.read" {
		t.Fatalf("unexpected principal: %#v", principal)
	}
}

func TestTokenExchangeIssuesServiceOACTForServerBoundClient(t *testing.T) {
	signer, err := identity.NewSigner(config.Identity{
		Issuer: "https://semattice.example.test", Audience: "semattice-api",
		Algorithm: "HS256", HMACKey: testOACTKey,
	})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	const ownerID = "9daab753-75c8-4e3d-a22b-7472cb7da579"
	h := NewHandler(resolverStub{found: true, status: tenant.TenantStatus{
		TenantID: testTenantID, CompanyID: testCompanyID,
		GlobalLifecycleStatus: "active", NativeStatus: "active",
	}}, verifierStub{identity: identity.OIDCIdentity{
		Subject: testSubjectID, ClientID: "commerce-service",
	}}, signer, []string{"runtime.record.read"}, map[string]config.ServiceAccessBinding{
		"commerce-service": {CompanyID: testCompanyID, OwnerPrincipalID: ownerID},
	}, 10*time.Minute)

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/token",
		bytes.NewBufferString(`{"requested_scopes":["runtime.record.read"]}`))
	request.Header.Set("Authorization", "Bearer keycloak-service-token")
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	oactVerifier, err := identity.NewVerifier(config.Identity{
		Issuer: "https://semattice.example.test", Audience: "semattice-api",
		Algorithm: "HS256", HMACKey: testOACTKey,
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	principal, err := oactVerifier.Verify(context.Background(), response.AccessToken)
	if err != nil {
		t.Fatalf("issued service OACT was rejected: %v", err)
	}
	if principal.PrincipalType != "SERVICE" || principal.ClientID != "commerce-service" ||
		principal.OwnerPrincipalID != ownerID || principal.CompanyID != testCompanyID {
		t.Fatalf("unexpected service principal: %#v", principal)
	}
}

func TestTokenExchangeFailsClosedBeforeSigning(t *testing.T) {
	signer, err := identity.NewSigner(config.Identity{
		Issuer: "https://semattice.example.test", Audience: "semattice-api",
		Algorithm: "HS256", HMACKey: testOACTKey,
	})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	active := tenant.TenantStatus{
		TenantID: testTenantID, CompanyID: testCompanyID,
		GlobalLifecycleStatus: "active", NativeStatus: "active",
	}
	tests := []struct {
		name     string
		header   string
		body     string
		identity identity.OIDCIdentity
		status   tenant.TenantStatus
		found    bool
		want     int
		code     string
	}{
		{name: "missing bearer", body: `{"requested_scopes":["record.read"]}`, want: 401, code: "invalid_token"},
		{name: "multiple organizations", header: "Bearer token", body: `{"requested_scopes":["record.read"]}`, identity: identity.OIDCIdentity{Subject: testSubjectID, Organizations: []string{testCompanyID, "orgaaaaaaaaaaaaaaaaa"}}, status: active, found: true, want: 409, code: "organization_selection_required"},
		{name: "scope outside allowlist", header: "Bearer token", body: `{"requested_scopes":["record.write"]}`, identity: identity.OIDCIdentity{Subject: testSubjectID, Organizations: []string{testCompanyID}}, status: active, found: true, want: 403, code: "invalid_scope"},
		{name: "organization not mapped", header: "Bearer token", body: `{"requested_scopes":["record.read"]}`, identity: identity.OIDCIdentity{Subject: testSubjectID, Organizations: []string{testCompanyID}}, want: 404, code: "organization_not_provisioned"},
		{name: "tenant inactive", header: "Bearer token", body: `{"requested_scopes":["record.read"]}`, identity: identity.OIDCIdentity{Subject: testSubjectID, Organizations: []string{testCompanyID}}, status: tenant.TenantStatus{TenantID: testTenantID, CompanyID: testCompanyID, GlobalLifecycleStatus: "active", NativeStatus: "suspended"}, found: true, want: 403, code: "organization_inactive"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := NewHandler(
				resolverStub{status: test.status, found: test.found},
				verifierStub{identity: test.identity}, signer, []string{"record.read"}, nil, 10*time.Minute,
			)
			request := httptest.NewRequest(http.MethodPost, "/v1/auth/token", bytes.NewBufferString(test.body))
			if test.header != "" {
				request.Header.Set("Authorization", test.header)
			}
			recorder := httptest.NewRecorder()
			h.ServeHTTP(recorder, request)
			if recorder.Code != test.want || !bytes.Contains(recorder.Body.Bytes(), []byte(test.code)) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
