package config

import (
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

func TestLoadProvisioningRequiresCompleteValidConfiguration(t *testing.T) {
	valid := map[string]string{
		"AI_NATIVE_AGENTCICI_BASE_URL":       "https://agentcici.example.test",
		"AI_NATIVE_AGENTCICI_HMAC_KEY":       "agentcici-outbound-key-material-that-is-long-enough",
		"AI_NATIVE_PROVISIONING_CALLER_KEYS": "agentcici=agentcici-inbound-key-material-that-is-long-enough;external-system=external-system-inbound-key-material-that-is-long-enough",
	}
	cfg, err := Load(func(key string) string { return valid[key] })
	if err != nil || !cfg.Provisioning.Enabled() || len(cfg.Provisioning.CallerKeys) != 2 {
		t.Fatalf("valid provisioning config rejected: cfg=%#v err=%v", cfg.Provisioning, err)
	}
	invalid := []map[string]string{
		{"AI_NATIVE_AGENTCICI_BASE_URL": valid["AI_NATIVE_AGENTCICI_BASE_URL"]},
		{"AI_NATIVE_AGENTCICI_BASE_URL": valid["AI_NATIVE_AGENTCICI_BASE_URL"], "AI_NATIVE_AGENTCICI_HMAC_KEY": valid["AI_NATIVE_AGENTCICI_HMAC_KEY"], "AI_NATIVE_PROVISIONING_CALLER_KEYS": "AgentCiCi=too-short"},
		{"AI_NATIVE_AGENTCICI_BASE_URL": valid["AI_NATIVE_AGENTCICI_BASE_URL"], "AI_NATIVE_AGENTCICI_HMAC_KEY": valid["AI_NATIVE_AGENTCICI_HMAC_KEY"], "AI_NATIVE_PROVISIONING_CALLER_KEYS": "agentcici=agentcici-inbound-key-material-that-is-long-enough;agentcici=duplicate-key-material-that-is-long-enough"},
	}
	for _, values := range invalid {
		if _, err := Load(func(key string) string { return values[key] }); err == nil {
			t.Fatalf("invalid provisioning configuration accepted: %#v", values)
		}
	}
}
