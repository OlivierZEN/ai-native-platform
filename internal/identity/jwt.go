package identity

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var companyIDPattern = regexp.MustCompile("^org[a-z0-9]{17}$")

type Claims struct {
	TenantID  string   `json:"tenant_id"`
	CompanyID string   `json:"company_id"`
	Scopes    []string `json:"scopes"`
	Scope     string   `json:"scope"`
	Approvals []string `json:"approvals,omitempty"`
	jwt.RegisteredClaims
}

func (claims Claims) effectiveScopes() []string {
	if len(claims.Scopes) > 0 {
		return append([]string(nil), claims.Scopes...)
	}
	return strings.Fields(claims.Scope)
}

type Verifier struct {
	legacyIssuer   string
	legacyAudience string
	legacyKey      []byte
	trusted        map[string]*jwksIssuer
}

type jwksIssuer struct {
	source   config.TrustedIssuer
	client   *http.Client
	mu       sync.Mutex
	fetched  time.Time
	keysByID map[string]*rsa.PublicKey
}

func NewVerifier(cfg config.Identity) (*Verifier, error) {
	verifier := &Verifier{trusted: map[string]*jwksIssuer{}}
	legacyConfigured := cfg.Issuer != "" || cfg.Audience != "" || cfg.Algorithm != "" || cfg.HMACKey != ""
	if legacyConfigured {
		if cfg.Issuer == "" || cfg.Audience == "" || cfg.Algorithm != "HS256" || len(cfg.HMACKey) < 32 {
			return nil, fmt.Errorf("identity verifier configuration is incomplete")
		}
		verifier.legacyIssuer = cfg.Issuer
		verifier.legacyAudience = cfg.Audience
		verifier.legacyKey = []byte(cfg.HMACKey)
	}
	for _, trusted := range cfg.TrustedIssuers {
		if _, exists := verifier.trusted[trusted.Issuer]; exists {
			return nil, fmt.Errorf("duplicate trusted identity issuer")
		}
		verifier.trusted[trusted.Issuer] = &jwksIssuer{
			source: trusted,
			client: &http.Client{Timeout: 5 * time.Second},
		}
	}
	if !legacyConfigured && len(verifier.trusted) == 0 {
		return nil, fmt.Errorf("identity verifier configuration is incomplete")
	}
	return verifier, nil
}

func (verifier *Verifier) Verify(ctx context.Context, rawToken string) (capability.TrustedPrincipal, error) {
	principal, _, err := verifier.VerifyWithExpiration(ctx, rawToken)
	return principal, err
}

// VerifyWithExpiration validates locally against the configured HMAC key or a
// cached JWKS. It never calls an IdP per request: a JWKS fetch happens only on
// cold start, after the five-minute cache TTL, or once for an unknown key id.
func (verifier *Verifier) VerifyWithExpiration(ctx context.Context, rawToken string) (capability.TrustedPrincipal, time.Time, error) {
	if strings.TrimSpace(rawToken) == "" {
		return capability.TrustedPrincipal{}, time.Time{}, fmt.Errorf("identity token is required")
	}
	issuer, err := tokenIssuer(rawToken)
	if err != nil {
		return capability.TrustedPrincipal{}, time.Time{}, fmt.Errorf("identity token is invalid")
	}
	if trusted, exists := verifier.trusted[issuer]; exists {
		return trusted.verify(ctx, rawToken)
	}
	if verifier.legacyIssuer == issuer {
		return verifier.verifyLegacy(rawToken)
	}
	return capability.TrustedPrincipal{}, time.Time{}, fmt.Errorf("identity issuer is not trusted")
}

func tokenIssuer(rawToken string) (string, error) {
	claims := jwt.MapClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(rawToken, claims)
	if err != nil {
		return "", err
	}
	issuer, ok := claims["iss"].(string)
	if !ok || strings.TrimSpace(issuer) == "" {
		return "", fmt.Errorf("issuer is missing")
	}
	return issuer, nil
}

func (verifier *Verifier) verifyLegacy(rawToken string) (capability.TrustedPrincipal, time.Time, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != "HS256" {
			return nil, fmt.Errorf("unexpected signing algorithm")
		}
		return verifier.legacyKey, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(verifier.legacyIssuer), jwt.WithAudience(verifier.legacyAudience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || token == nil || !token.Valid {
		return capability.TrustedPrincipal{}, time.Time{}, fmt.Errorf("identity token is invalid")
	}
	return principalFromClaims(*claims, "legacy_hs256")
}

func (issuer *jwksIssuer) verify(ctx context.Context, rawToken string) (capability.TrustedPrincipal, time.Time, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != "RS256" {
			return nil, fmt.Errorf("unexpected signing algorithm")
		}
		keyID, _ := token.Header["kid"].(string)
		if keyID == "" {
			return nil, fmt.Errorf("key id is required")
		}
		return issuer.key(ctx, keyID)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(issuer.source.Issuer), jwt.WithAudience(issuer.source.Audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || token == nil || !token.Valid {
		return capability.TrustedPrincipal{}, time.Time{}, fmt.Errorf("identity token is invalid")
	}
	return principalFromClaims(*claims, issuer.source.Source)
}

func principalFromClaims(claims Claims, source string) (capability.TrustedPrincipal, time.Time, error) {
	tenantID, err := uuid.Parse(claims.TenantID)
	scopes := claims.effectiveScopes()
	if err != nil || tenantID == uuid.Nil || !companyIDPattern.MatchString(claims.CompanyID) || strings.TrimSpace(claims.Subject) == "" || len(scopes) == 0 {
		return capability.TrustedPrincipal{}, time.Time{}, fmt.Errorf("identity claims are incomplete")
	}
	return capability.TrustedPrincipal{
		TenantID: tenantID.String(), CompanyID: claims.CompanyID,
		Actor:     capability.Actor{ID: claims.Subject, Scopes: scopes},
		Approvals: append([]string(nil), claims.Approvals...), Source: source,
	}, claims.ExpiresAt.Time, nil
}

func (issuer *jwksIssuer) key(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	issuer.mu.Lock()
	defer issuer.mu.Unlock()
	if key, found := issuer.keysByID[keyID]; found && time.Since(issuer.fetched) < 5*time.Minute {
		return key, nil
	}
	if err := issuer.refresh(ctx); err != nil {
		return nil, err
	}
	key, found := issuer.keysByID[keyID]
	if !found {
		return nil, fmt.Errorf("signing key is not trusted")
	}
	return key, nil
}

func (issuer *jwksIssuer) refresh(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer.source.JWKSURL, nil)
	if err != nil {
		return err
	}
	response, err := issuer.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", response.StatusCode)
	}
	var document struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&document); err != nil {
		return err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, candidate := range document.Keys {
		if candidate.Kty != "RSA" || candidate.Kid == "" || (candidate.Alg != "" && candidate.Alg != "RS256") || (candidate.Use != "" && candidate.Use != "sig") {
			continue
		}
		key, err := rsaKey(candidate.N, candidate.E)
		if err != nil {
			continue
		}
		keys[candidate.Kid] = key
	}
	if len(keys) == 0 {
		return fmt.Errorf("JWKS response has no eligible RSA signing keys")
	}
	issuer.keysByID = keys
	issuer.fetched = time.Now()
	return nil
}

func rsaKey(modulus, exponent string) (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(modulus)
	if err != nil || len(n) == 0 {
		return nil, fmt.Errorf("invalid RSA modulus")
	}
	e, err := base64.RawURLEncoding.DecodeString(exponent)
	if err != nil || len(e) == 0 || len(e) > 4 {
		return nil, fmt.Errorf("invalid RSA exponent")
	}
	exponentValue := 0
	for _, value := range e {
		exponentValue = exponentValue<<8 | int(value)
	}
	if exponentValue < 3 || exponentValue%2 == 0 {
		return nil, fmt.Errorf("invalid RSA exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponentValue}, nil
}
