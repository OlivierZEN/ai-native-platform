package identity

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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
	Approvals []string `json:"approvals,omitempty"`
	jwt.RegisteredClaims
}

type Verifier struct {
	issuer    string
	audience  string
	algorithm string
	key       []byte
}

func NewVerifier(cfg config.Identity) (*Verifier, error) {
	if cfg.Issuer == "" || cfg.Audience == "" || cfg.Algorithm != "HS256" || len(cfg.HMACKey) < 32 {
		return nil, fmt.Errorf("identity verifier configuration is incomplete")
	}
	return &Verifier{issuer: cfg.Issuer, audience: cfg.Audience, algorithm: cfg.Algorithm, key: []byte(cfg.HMACKey)}, nil
}

func (verifier *Verifier) Verify(ctx context.Context, rawToken string) (capability.TrustedPrincipal, error) {
	principal, _, err := verifier.VerifyWithExpiration(ctx, rawToken)
	return principal, err
}

// VerifyWithExpiration validates a token and returns the verified principal
// together with the expiry carried by the token. HTTP transports use the
// expiry to authenticate every request and to bind a streamable MCP session to
// the same tenant principal.
func (verifier *Verifier) VerifyWithExpiration(_ context.Context, rawToken string) (capability.TrustedPrincipal, time.Time, error) {
	if strings.TrimSpace(rawToken) == "" {
		return capability.TrustedPrincipal{}, time.Time{}, fmt.Errorf("identity token is required")
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != verifier.algorithm {
			return nil, fmt.Errorf("unexpected signing algorithm")
		}
		return verifier.key, nil
	},
		jwt.WithValidMethods([]string{verifier.algorithm}),
		jwt.WithIssuer(verifier.issuer),
		jwt.WithAudience(verifier.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil || token == nil || !token.Valid {
		return capability.TrustedPrincipal{}, time.Time{}, fmt.Errorf("identity token is invalid")
	}
	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil || tenantID == uuid.Nil || !companyIDPattern.MatchString(claims.CompanyID) || strings.TrimSpace(claims.Subject) == "" || len(claims.Scopes) == 0 {
		return capability.TrustedPrincipal{}, time.Time{}, fmt.Errorf("identity claims are incomplete")
	}
	scopes := append([]string(nil), claims.Scopes...)
	return capability.TrustedPrincipal{
		TenantID:  tenantID.String(),
		CompanyID: claims.CompanyID,
		Actor:     capability.Actor{ID: claims.Subject, Scopes: scopes},
		Approvals: append([]string(nil), claims.Approvals...),
		Source:    "jwt",
	}, claims.ExpiresAt.Time, nil
}
