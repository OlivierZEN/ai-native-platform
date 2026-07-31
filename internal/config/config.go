package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPListen         string
	Database           Database
	ControlDatabaseURL string
	RuntimeDatabaseURL string
	Log                Log
	Identity           Identity
	ConsoleSessionKey  string
	AccessContext      AccessContext
}

type Database struct {
	URL         string
	MaxConns    int32
	MinConns    int32
	MaxLifetime time.Duration
	MaxIdleTime time.Duration
}

type Log struct {
	Level  string
	Format string
}

type Identity struct {
	Issuer         string
	Audience       string
	Algorithm      string
	HMACKey        string
	Token          string
	TrustedIssuers []TrustedIssuer
}

// TrustedIssuer describes a non-HMAC JWT issuer verified through its published
// JWKS. The source name is audit-only and must not be trusted from a token.
type TrustedIssuer struct {
	Source   string
	Issuer   string
	Audience string
	JWKSURL  string
}

type AccessContext struct {
	KeycloakIssuer   string
	KeycloakAudience string
	KeycloakJWKSURL  string
	KeycloakClientID string
	AllowedScopes    []string
	TokenTTL         time.Duration
}

func (access AccessContext) Enabled() bool {
	return access.KeycloakIssuer != "" && access.KeycloakAudience != "" &&
		access.KeycloakJWKSURL != "" && access.KeycloakClientID != "" &&
		len(access.AllowedScopes) > 0
}

func LoadEnv() (Config, error) {
	return Load(os.Getenv)
}

func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		HTTPListen: valueOr(getenv("AI_NATIVE_HTTP_LISTEN"), "127.0.0.1:8080"),
		Database: Database{
			URL:         getenv("AI_NATIVE_DATABASE_URL"),
			MaxConns:    16,
			MinConns:    0,
			MaxLifetime: time.Hour,
			MaxIdleTime: 15 * time.Minute,
		},
		ControlDatabaseURL: getenv("AI_NATIVE_CONTROL_DATABASE_URL"),
		RuntimeDatabaseURL: getenv("AI_NATIVE_RUNTIME_DATABASE_URL"),
		Log: Log{
			Level:  valueOr(strings.ToLower(getenv("AI_NATIVE_LOG_LEVEL")), "info"),
			Format: valueOr(strings.ToLower(getenv("AI_NATIVE_LOG_FORMAT")), "json"),
		},
		Identity: Identity{
			Issuer:         getenv("AI_NATIVE_IDENTITY_ISSUER"),
			Audience:       getenv("AI_NATIVE_IDENTITY_AUDIENCE"),
			Algorithm:      getenv("AI_NATIVE_IDENTITY_ALGORITHM"),
			HMACKey:        getenv("AI_NATIVE_IDENTITY_HMAC_KEY"),
			Token:          getenv("AI_NATIVE_IDENTITY_TOKEN"),
			TrustedIssuers: nil,
		},
		ConsoleSessionKey: getenv("AI_NATIVE_CONSOLE_SESSION_HMAC_KEY"),
		AccessContext: AccessContext{
			KeycloakIssuer:   strings.TrimRight(getenv("AI_NATIVE_KEYCLOAK_ISSUER"), "/"),
			KeycloakAudience: getenv("AI_NATIVE_KEYCLOAK_AUDIENCE"),
			KeycloakJWKSURL:  getenv("AI_NATIVE_KEYCLOAK_JWKS_URL"),
			KeycloakClientID: getenv("AI_NATIVE_KEYCLOAK_CLIENT_ID"),
			AllowedScopes:    parseScopes(getenv("AI_NATIVE_OACT_ALLOWED_SCOPES")),
			TokenTTL:         10 * time.Minute,
		},
	}
	trustedIssuers, trustedIssuersErr := parseTrustedIssuers(getenv("AI_NATIVE_IDENTITY_TRUSTED_ISSUERS"))
	if trustedIssuersErr != nil {
		return Config{}, trustedIssuersErr
	}
	cfg.Identity.TrustedIssuers = trustedIssuers

	var err error
	if cfg.Database.MaxConns, err = parseInt32(getenv("AI_NATIVE_DATABASE_MAX_CONNS"), cfg.Database.MaxConns, 1); err != nil {
		return Config{}, fmt.Errorf("invalid database max connections")
	}
	if cfg.Database.MinConns, err = parseInt32(getenv("AI_NATIVE_DATABASE_MIN_CONNS"), cfg.Database.MinConns, 0); err != nil {
		return Config{}, fmt.Errorf("invalid database min connections")
	}
	if cfg.Database.MinConns > cfg.Database.MaxConns {
		return Config{}, fmt.Errorf("database min connections exceeds max connections")
	}
	if cfg.Database.MaxLifetime, err = parseDuration(getenv("AI_NATIVE_DATABASE_MAX_LIFETIME"), cfg.Database.MaxLifetime); err != nil {
		return Config{}, fmt.Errorf("invalid database max lifetime")
	}
	if cfg.Database.MaxIdleTime, err = parseDuration(getenv("AI_NATIVE_DATABASE_MAX_IDLE_TIME"), cfg.Database.MaxIdleTime); err != nil {
		return Config{}, fmt.Errorf("invalid database max idle time")
	}
	if cfg.AccessContext.TokenTTL, err = parseDuration(getenv("AI_NATIVE_OACT_TTL"), cfg.AccessContext.TokenTTL); err != nil {
		return Config{}, fmt.Errorf("invalid OACT TTL")
	}
	if cfg.Database.URL != "" {
		if !validDatabaseURL(cfg.Database.URL) {
			return Config{}, fmt.Errorf("invalid database URL")
		}
	}
	if cfg.ControlDatabaseURL != "" && !validDatabaseURL(cfg.ControlDatabaseURL) {
		return Config{}, fmt.Errorf("invalid control database URL")
	}
	if cfg.RuntimeDatabaseURL != "" && !validDatabaseURL(cfg.RuntimeDatabaseURL) {
		return Config{}, fmt.Errorf("invalid runtime database URL")
	}
	if (cfg.ControlDatabaseURL == "") != (cfg.RuntimeDatabaseURL == "") {
		return Config{}, fmt.Errorf("control and runtime database URLs must be configured together")
	}
	if cfg.Log.Level != "debug" && cfg.Log.Level != "info" && cfg.Log.Level != "warn" && cfg.Log.Level != "error" {
		return Config{}, fmt.Errorf("invalid log level")
	}
	if cfg.Log.Format != "json" && cfg.Log.Format != "text" {
		return Config{}, fmt.Errorf("invalid log format")
	}
	if err := validateIdentity(cfg.Identity); err != nil {
		return Config{}, err
	}
	if cfg.ConsoleSessionKey != "" && len(cfg.ConsoleSessionKey) < 32 {
		return Config{}, fmt.Errorf("console session key is too short")
	}
	if err := validateAccessContext(cfg.AccessContext, cfg.Identity); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseScopes(raw string) []string {
	return strings.Fields(strings.ReplaceAll(raw, ",", " "))
}

// parseTrustedIssuers accepts semicolon-separated source|issuer|audience|jwks_url entries.
// A product configuration must list each official or third-party issuer explicitly;
// token-provided iss/aud/JWKS values are never accepted.
func parseTrustedIssuers(raw string) ([]TrustedIssuer, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	result := make([]TrustedIssuer, 0)
	seenSource := map[string]struct{}{}
	seenIssuer := map[string]struct{}{}
	for _, rawItem := range strings.Split(raw, ";") {
		parts := strings.Split(rawItem, "|")
		if len(parts) != 4 {
			return nil, fmt.Errorf("invalid trusted identity issuer configuration")
		}
		entry := TrustedIssuer{
			Source: strings.TrimSpace(parts[0]), Issuer: strings.TrimSpace(parts[1]),
			Audience: strings.TrimSpace(parts[2]), JWKSURL: strings.TrimSpace(parts[3]),
		}
		parsed, err := url.Parse(entry.JWKSURL)
		if entry.Source == "" || entry.Issuer == "" || entry.Audience == "" || err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, fmt.Errorf("invalid trusted identity issuer configuration")
		}
		if _, exists := seenSource[entry.Source]; exists {
			return nil, fmt.Errorf("invalid trusted identity issuer configuration")
		}
		if _, exists := seenIssuer[entry.Issuer]; exists {
			return nil, fmt.Errorf("invalid trusted identity issuer configuration")
		}
		seenSource[entry.Source] = struct{}{}
		seenIssuer[entry.Issuer] = struct{}{}
		result = append(result, entry)
	}
	return result, nil
}

func validateAccessContext(access AccessContext, identity Identity) error {
	present := 0
	for _, value := range []bool{
		access.KeycloakIssuer != "", access.KeycloakAudience != "", access.KeycloakJWKSURL != "",
		access.KeycloakClientID != "", len(access.AllowedScopes) > 0,
	} {
		if value {
			present++
		}
	}
	if present == 0 {
		return nil
	}
	if present != 5 {
		return fmt.Errorf("access context configuration must be complete")
	}
	if !validHTTPSOrLoopbackURL(access.KeycloakIssuer) || !validHTTPSOrLoopbackURL(access.KeycloakJWKSURL) {
		return fmt.Errorf("invalid Keycloak access context URL")
	}
	if identity.Issuer == "" || identity.Audience == "" || identity.Algorithm != "HS256" || len(identity.HMACKey) < 32 {
		return fmt.Errorf("access context requires complete HS256 identity signing configuration")
	}
	if access.TokenTTL < time.Minute || access.TokenTTL > time.Hour {
		return fmt.Errorf("OACT TTL must be between 1m and 1h")
	}
	seen := map[string]struct{}{}
	for _, scope := range access.AllowedScopes {
		if !validScope(scope) {
			return fmt.Errorf("invalid OACT allowed scope")
		}
		if _, exists := seen[scope]; exists {
			return fmt.Errorf("duplicate OACT allowed scope")
		}
		seen[scope] = struct{}{}
	}
	return nil
}

func validScope(scope string) bool {
	if scope == "" || len(scope) > 128 {
		return false
	}
	for _, character := range scope {
		if !(character >= 'a' && character <= 'z') &&
			!(character >= '0' && character <= '9') &&
			character != '.' && character != ':' && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func validHTTPSOrLoopbackURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	return parsed.Scheme == "http" && (parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost" || parsed.Hostname() == "::1")
}

func validDatabaseURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") && parsed.Host != ""
}

func validateIdentity(identity Identity) error {
	present := 0
	for _, value := range []string{identity.Issuer, identity.Audience, identity.Algorithm, identity.HMACKey} {
		if value != "" {
			present++
		}
	}
	if present == 0 {
		if len(identity.TrustedIssuers) == 0 {
			return nil
		}
		return nil
	}
	if present != 4 {
		return fmt.Errorf("identity configuration must be complete")
	}
	if identity.Algorithm != "HS256" {
		return fmt.Errorf("identity algorithm is not allowed")
	}
	if len(identity.HMACKey) < 32 {
		return fmt.Errorf("identity HMAC key is too short")
	}
	return nil
}

func parseInt32(raw string, fallback, minimum int32) (int32, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value < int64(minimum) {
		return 0, fmt.Errorf("invalid integer")
	}
	return int32(value), nil
}

func parseDuration(raw string, fallback time.Duration) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid duration")
	}
	return value, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
