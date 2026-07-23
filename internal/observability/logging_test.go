package observability

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/OlivierZEN/ai-native-platform/internal/config"
)

func TestNewJSONLoggerEmitsStableFieldsAndRedactsSecrets(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(config.Log{Level: "info", Format: "json"}, &output)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("invoked", "request_id", "req-1", "tenant_id", "tenant-1", "token", "super-secret", "database_url", "postgres://user:password@db/native")
	line := output.String()
	for _, expected := range []string{"\"msg\":\"invoked\"", "\"request_id\":\"req-1\"", "\"tenant_id\":\"tenant-1\"", "\"token\":\"[REDACTED]\"", "\"database_url\":\"[REDACTED]\""} {
		if !strings.Contains(line, expected) {
			t.Fatalf("log %q missing %q", line, expected)
		}
	}
	if strings.Contains(line, "super-secret") || strings.Contains(line, "password@") {
		t.Fatalf("log leaked sensitive value: %q", line)
	}
}

func TestNewTextLoggerHonorsLevel(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(config.Log{Level: "warn", Format: "text"}, &output)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("hidden")
	logger.Warn("visible", slog.String("actor_id", "agent-1"))
	if strings.Contains(output.String(), "hidden") || !strings.Contains(output.String(), "visible") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestNewRejectsUnknownFormat(t *testing.T) {
	if _, err := New(config.Log{Level: "info", Format: "xml"}, &bytes.Buffer{}); err == nil {
		t.Fatal("New succeeded, want error")
	}
}
