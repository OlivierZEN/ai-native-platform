package identity

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

const testIdentityKey = "0123456789abcdef0123456789abcdef"

func TestVerifierAcceptsOnlyBoundIssuerAudienceAlgorithmAndClaims(t *testing.T) {
	verifier, err := NewVerifier(config.Identity{
		Issuer:    "https://identity.example.test",
		Audience:  "native-platform",
		Algorithm: "HS256",
		HMACKey:   testIdentityKey,
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	validClaims := Claims{
		TenantID:  "11111111-1111-4111-8111-111111111111",
		CompanyID: "orgaaaaaaaaaaaaaaaaa",
		Scopes:    []string{"tenant.provision"},
		Approvals: []string{"approval-metadata-1"},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://identity.example.test",
			Subject:   "operations-agent",
			Audience:  jwt.ClaimStrings{"native-platform"},
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	validToken := signToken(t, validClaims, testIdentityKey)
	principal, err := verifier.Verify(context.Background(), validToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if principal.TenantID != validClaims.TenantID || principal.CompanyID != validClaims.CompanyID || principal.Actor.ID != validClaims.Subject || len(principal.Approvals) != 1 {
		t.Fatalf("principal=%#v", principal)
	}

	cases := []struct {
		name   string
		claims Claims
		key    string
	}{
		{name: "wrong issuer", claims: mutateClaims(validClaims, func(c *Claims) { c.Issuer = "wrong" }), key: testIdentityKey},
		{name: "wrong audience", claims: mutateClaims(validClaims, func(c *Claims) { c.Audience = jwt.ClaimStrings{"other"} }), key: testIdentityKey},
		{name: "expired", claims: mutateClaims(validClaims, func(c *Claims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }), key: testIdentityKey},
		{name: "wrong key", claims: validClaims, key: "abcdef0123456789abcdef0123456789"},
		{name: "invalid tenant", claims: mutateClaims(validClaims, func(c *Claims) { c.TenantID = "tenant-1" }), key: testIdentityKey},
		{name: "invalid org", claims: mutateClaims(validClaims, func(c *Claims) { c.CompanyID = "org-short" }), key: testIdentityKey},
		{name: "missing subject", claims: mutateClaims(validClaims, func(c *Claims) { c.Subject = "" }), key: testIdentityKey},
		{name: "missing scope", claims: mutateClaims(validClaims, func(c *Claims) { c.Scopes = nil }), key: testIdentityKey},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := verifier.Verify(context.Background(), signToken(t, testCase.claims, testCase.key)); err == nil {
				t.Fatal("Verify succeeded, want rejection")
			}
		})
	}

	legacyClaims := jwt.MapClaims{
		"tenant_id": validClaims.TenantID,
		"org_id":    validClaims.CompanyID,
		"scopes":    validClaims.Scopes,
		"iss":       validClaims.Issuer,
		"sub":       validClaims.Subject,
		"aud":       []string{"native-platform"},
		"iat":       time.Now().Add(-time.Minute).Unix(),
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	legacyToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, legacyClaims).SignedString([]byte(testIdentityKey))
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}
	if _, err := verifier.Verify(context.Background(), legacyToken); err == nil {
		t.Fatal("legacy org_id token was accepted")
	}
}

func TestVerifierUsesConfiguredJWKSAndOAuthScopeWithoutPerRequestFetch(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"keys":[{"kty":"RSA","kid":"test-key","use":"sig","alg":"RS256","n":"%s","e":"%s"}]}`,
			base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
			base64.RawURLEncoding.EncodeToString(bigEndian(privateKey.PublicKey.E)))
	}))
	defer server.Close()

	issuerURL := "https://official-context.example.test"
	verifier, err := NewVerifier(config.Identity{TrustedIssuers: []config.TrustedIssuer{{
		Source: "official_context", Issuer: issuerURL, Audience: "semattice-api", JWKSURL: server.URL,
	}}})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	verifier.trusted[issuerURL].client = server.Client()
	claims := Claims{
		TenantID: "11111111-1111-4111-8111-111111111111", CompanyID: "orgaaaaaaaaaaaaaaaaa",
		Scope: "record.read record.write",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: issuerURL, Subject: "keycloak-subject", Audience: jwt.ClaimStrings{"semattice-api"},
			IssuedAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key"
	raw, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		principal, verifyErr := verifier.Verify(context.Background(), raw)
		if verifyErr != nil {
			t.Fatalf("Verify: %v", verifyErr)
		}
		if principal.Source != "official_context" || principal.Actor.ID != "keycloak-subject" || len(principal.Actor.Scopes) != 2 {
			t.Fatalf("principal=%#v", principal)
		}
	}
	if requests != 1 {
		t.Fatalf("JWKS requests=%d, want 1 cached request", requests)
	}
}

func TestVerifierRejectsUnconfiguredJWKSIssuer(t *testing.T) {
	verifier, err := NewVerifier(config.Identity{TrustedIssuers: []config.TrustedIssuer{{
		Source: "official_context", Issuer: "https://official-context.example.test", Audience: "semattice-api", JWKSURL: "https://keys.example.test/jwks",
	}}})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": "https://attacker.example.test"})
	raw, err := token.SignedString([]byte(testIdentityKey))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Fatal("Verify succeeded for an unconfigured issuer")
	}
}

func bigEndian(value int) []byte {
	if value == 0 {
		return []byte{0}
	}
	bytes := make([]byte, 0, 4)
	for value > 0 {
		bytes = append([]byte{byte(value)}, bytes...)
		value >>= 8
	}
	return bytes
}

func signToken(t *testing.T, claims Claims, key string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(key))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func mutateClaims(claims Claims, mutate func(*Claims)) Claims {
	copy := claims
	copy.Scopes = append([]string(nil), claims.Scopes...)
	copy.Audience = append(jwt.ClaimStrings(nil), claims.Audience...)
	mutate(&copy)
	return copy
}
