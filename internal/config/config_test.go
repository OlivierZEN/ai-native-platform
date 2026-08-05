package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	values := map[string]string{
		"AI_NATIVE_HTTP_LISTEN":           "127.0.0.1:9090",
		"AI_NATIVE_DATABASE_URL":          "postgres://runtime:secret@127.0.0.1/native?sslmode=disable",
		"AI_NATIVE_CONTROL_DATABASE_URL":  "postgres://control@127.0.0.1/native?sslmode=disable",
		"AI_NATIVE_RUNTIME_DATABASE_URL":  "postgres://runtime@127.0.0.1/native?sslmode=disable",
		"AI_NATIVE_DATABASE_MAX_CONNS":    "12",
		"AI_NATIVE_DATABASE_MIN_CONNS":    "2",
		"AI_NATIVE_DATABASE_MAX_LIFETIME": "45m",
		"AI_NATIVE_LOG_LEVEL":             "debug",
		"AI_NATIVE_LOG_FORMAT":            "text",
		"AI_NATIVE_IDENTITY_ISSUER":       "https://identity.example.test",
		"AI_NATIVE_IDENTITY_AUDIENCE":     "native-platform",
		"AI_NATIVE_IDENTITY_ALGORITHM":    "HS256",
		"AI_NATIVE_IDENTITY_HMAC_KEY":     "test-only-key-material-that-is-long-enough",
	}
	cfg, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPListen != "127.0.0.1:9090" || cfg.Database.MaxConns != 12 || cfg.Database.MinConns != 2 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.ControlDatabaseURL == "" || cfg.RuntimeDatabaseURL == "" {
		t.Fatalf("application database role URLs not loaded: %#v", cfg)
	}
	if cfg.Database.MaxLifetime != 45*time.Minute || cfg.Log.Level != "debug" || cfg.Log.Format != "text" {
		t.Fatalf("unexpected duration/log config: %#v", cfg)
	}
	if cfg.Identity.Issuer == "" || cfg.Identity.HMACKey == "" {
		t.Fatalf("identity config not loaded: %#v", cfg.Identity)
	}
}

func TestLoadConsoleOIDCAndProtectedSecretFile(t *testing.T) {
	values := map[string]string{
		"AI_NATIVE_IDENTITY_ISSUER":                 "https://semattice.example.test",
		"AI_NATIVE_IDENTITY_AUDIENCE":               "semattice-api",
		"AI_NATIVE_IDENTITY_ALGORITHM":              "HS256",
		"AI_NATIVE_IDENTITY_HMAC_KEY":               "semattice-signing-key-material-that-is-long-enough",
		"AI_NATIVE_KEYCLOAK_ISSUER":                 "https://sso.example.test/realms/example",
		"AI_NATIVE_KEYCLOAK_AUDIENCE":               "semattice-api",
		"AI_NATIVE_KEYCLOAK_JWKS_URL":               "https://sso.example.test/realms/example/protocol/openid-connect/certs",
		"AI_NATIVE_KEYCLOAK_CLIENT_ID":              "semattice-cli",
		"AI_NATIVE_OACT_ALLOWED_SCOPES":             "authorization.read",
		"AI_NATIVE_CONSOLE_OIDC_CLIENT_ID":          "semattice-web",
		"AI_NATIVE_CONSOLE_OIDC_CLIENT_SECRET_FILE": "/etc/semattice/secrets/semattice-web-client-secret",
		"AI_NATIVE_CONSOLE_OIDC_REDIRECT_URI":       "https://semattice.example.test/auth/oidc/callback",
	}
	cfg, err := Load(func(key string) string { return values[key] })
	if err != nil || !cfg.ConsoleOIDC.Enabled() {
		t.Fatalf("valid console OIDC config rejected: cfg=%#v err=%v", cfg.ConsoleOIDC, err)
	}
	for _, key := range []string{
		"AI_NATIVE_CONSOLE_OIDC_CLIENT_ID",
		"AI_NATIVE_CONSOLE_OIDC_CLIENT_SECRET_FILE",
		"AI_NATIVE_CONSOLE_OIDC_REDIRECT_URI",
	} {
		invalid := map[string]string{}
		for name, value := range values {
			invalid[name] = value
		}
		delete(invalid, key)
		if _, err := Load(func(name string) string { return invalid[name] }); err == nil {
			t.Fatalf("partial console OIDC config accepted without %s", key)
		}
	}

	path := filepath.Join(t.TempDir(), "client-secret")
	if err := os.WriteFile(path, []byte("protected-client-secret-value\n"), 0640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	secret, err := ReadSecretFile(path)
	if err != nil || secret != "protected-client-secret-value" {
		t.Fatalf("ReadSecretFile secret=%q err=%v", secret, err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if _, err := ReadSecretFile(path); err == nil {
		t.Fatal("world-readable client secret file accepted")
	}
}

func TestLoadDefaultsDoNotInventCredentials(t *testing.T) {
	cfg, err := Load(func(string) string { return "" })
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPListen != "127.0.0.1:8080" || cfg.Database.URL != "" || cfg.Identity.HMACKey != "" {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
	if cfg.Database.MaxConns != 16 || cfg.Database.MinConns != 0 || cfg.Log.Format != "json" || cfg.Log.Level != "info" {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

func TestLoadRejectsInvalidValuesWithoutLeakingSecrets(t *testing.T) {
	secretURL := "postgres://runtime:do-not-leak@127.0.0.1/native"
	tests := []map[string]string{
		{"AI_NATIVE_DATABASE_URL": secretURL, "AI_NATIVE_DATABASE_MAX_CONNS": "0"},
		{"AI_NATIVE_DATABASE_URL": secretURL, "AI_NATIVE_DATABASE_MAX_LIFETIME": "tomorrow"},
		{"AI_NATIVE_LOG_LEVEL": "verbose"},
		{"AI_NATIVE_LOG_FORMAT": "xml"},
		{"AI_NATIVE_IDENTITY_ISSUER": "issuer-only"},
		{"AI_NATIVE_CONTROL_DATABASE_URL": "postgres://control@127.0.0.1/native"},
	}
	for _, values := range tests {
		_, err := Load(func(key string) string { return values[key] })
		if err == nil {
			t.Fatalf("Load(%v) succeeded, want error", values)
		}
		if strings.Contains(err.Error(), "do-not-leak") {
			t.Fatalf("error leaked database credential: %v", err)
		}
	}
}

func TestLoadAccessContextRequiresCompleteValidConfiguration(t *testing.T) {
	valid := map[string]string{
		"AI_NATIVE_IDENTITY_ISSUER":           "https://semattice.example.test",
		"AI_NATIVE_IDENTITY_AUDIENCE":         "semattice-api",
		"AI_NATIVE_IDENTITY_ALGORITHM":        "HS256",
		"AI_NATIVE_IDENTITY_HMAC_KEY":         "semattice-signing-key-material-that-is-long-enough",
		"AI_NATIVE_KEYCLOAK_ISSUER":           "https://sso.example.test/realms/example",
		"AI_NATIVE_KEYCLOAK_AUDIENCE":         "semattice-api",
		"AI_NATIVE_KEYCLOAK_JWKS_URL":         "https://sso.example.test/realms/example/protocol/openid-connect/certs",
		"AI_NATIVE_KEYCLOAK_CLIENT_IDS":       "semattice-cli,storefront-web,admin-web",
		"AI_NATIVE_KEYCLOAK_SERVICE_BINDINGS": "commerce-service=org2sva14i4udjmi2t4s@9daab753-75c8-4e3d-a22b-7472cb7da579",
		"AI_NATIVE_OACT_ALLOWED_SCOPES":       "system.capability.read,record.read",
		"AI_NATIVE_OACT_TTL":                  "15m",
	}
	cfg, err := Load(func(key string) string { return valid[key] })
	if err != nil || !cfg.AccessContext.Enabled() || len(cfg.AccessContext.KeycloakClientIDs) != 3 ||
		len(cfg.AccessContext.KeycloakServiceBindings) != 1 || len(cfg.AccessContext.AllowedScopes) != 2 ||
		cfg.AccessContext.TokenTTL != 15*time.Minute {
		t.Fatalf("valid access context config rejected: cfg=%#v err=%v", cfg.AccessContext, err)
	}
	invalid := []map[string]string{
		{"AI_NATIVE_KEYCLOAK_ISSUER": valid["AI_NATIVE_KEYCLOAK_ISSUER"]},
		{
			"AI_NATIVE_IDENTITY_ISSUER": valid["AI_NATIVE_IDENTITY_ISSUER"], "AI_NATIVE_IDENTITY_AUDIENCE": valid["AI_NATIVE_IDENTITY_AUDIENCE"],
			"AI_NATIVE_IDENTITY_ALGORITHM": valid["AI_NATIVE_IDENTITY_ALGORITHM"], "AI_NATIVE_IDENTITY_HMAC_KEY": valid["AI_NATIVE_IDENTITY_HMAC_KEY"],
			"AI_NATIVE_KEYCLOAK_ISSUER": valid["AI_NATIVE_KEYCLOAK_ISSUER"], "AI_NATIVE_KEYCLOAK_AUDIENCE": valid["AI_NATIVE_KEYCLOAK_AUDIENCE"],
			"AI_NATIVE_KEYCLOAK_JWKS_URL": "http://external.example.test/certs", "AI_NATIVE_KEYCLOAK_CLIENT_IDS": valid["AI_NATIVE_KEYCLOAK_CLIENT_IDS"],
			"AI_NATIVE_OACT_ALLOWED_SCOPES": valid["AI_NATIVE_OACT_ALLOWED_SCOPES"],
		},
		{
			"AI_NATIVE_IDENTITY_ISSUER": valid["AI_NATIVE_IDENTITY_ISSUER"], "AI_NATIVE_IDENTITY_AUDIENCE": valid["AI_NATIVE_IDENTITY_AUDIENCE"],
			"AI_NATIVE_IDENTITY_ALGORITHM": valid["AI_NATIVE_IDENTITY_ALGORITHM"], "AI_NATIVE_IDENTITY_HMAC_KEY": valid["AI_NATIVE_IDENTITY_HMAC_KEY"],
			"AI_NATIVE_KEYCLOAK_ISSUER": valid["AI_NATIVE_KEYCLOAK_ISSUER"], "AI_NATIVE_KEYCLOAK_AUDIENCE": valid["AI_NATIVE_KEYCLOAK_AUDIENCE"],
			"AI_NATIVE_KEYCLOAK_JWKS_URL": valid["AI_NATIVE_KEYCLOAK_JWKS_URL"], "AI_NATIVE_KEYCLOAK_CLIENT_IDS": valid["AI_NATIVE_KEYCLOAK_CLIENT_IDS"],
			"AI_NATIVE_OACT_ALLOWED_SCOPES": "record.read,record.read",
		},
		{
			"AI_NATIVE_IDENTITY_ISSUER": valid["AI_NATIVE_IDENTITY_ISSUER"], "AI_NATIVE_IDENTITY_AUDIENCE": valid["AI_NATIVE_IDENTITY_AUDIENCE"],
			"AI_NATIVE_IDENTITY_ALGORITHM": valid["AI_NATIVE_IDENTITY_ALGORITHM"], "AI_NATIVE_IDENTITY_HMAC_KEY": valid["AI_NATIVE_IDENTITY_HMAC_KEY"],
			"AI_NATIVE_KEYCLOAK_ISSUER": valid["AI_NATIVE_KEYCLOAK_ISSUER"], "AI_NATIVE_KEYCLOAK_AUDIENCE": valid["AI_NATIVE_KEYCLOAK_AUDIENCE"],
			"AI_NATIVE_KEYCLOAK_JWKS_URL": valid["AI_NATIVE_KEYCLOAK_JWKS_URL"], "AI_NATIVE_KEYCLOAK_CLIENT_IDS": "semattice-cli",
			"AI_NATIVE_KEYCLOAK_SERVICE_BINDINGS": "commerce-service=bad-company@not-a-uuid",
			"AI_NATIVE_OACT_ALLOWED_SCOPES":       valid["AI_NATIVE_OACT_ALLOWED_SCOPES"],
		},
	}
	for _, values := range invalid {
		if _, err := Load(func(key string) string { return values[key] }); err == nil {
			t.Fatalf("invalid access context configuration accepted: %#v", values)
		}
	}
}
