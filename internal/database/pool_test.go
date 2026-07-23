package database

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/config"
)

func TestParsePoolConfig(t *testing.T) {
	parsed, err := ParsePoolConfig(config.Database{
		URL:         "postgres://runtime:secret@127.0.0.1/native?sslmode=disable",
		MaxConns:    8,
		MinConns:    2,
		MaxLifetime: time.Hour,
		MaxIdleTime: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("ParsePoolConfig: %v", err)
	}
	if parsed.MaxConns != 8 || parsed.MinConns != 2 || parsed.MaxConnLifetime != time.Hour || parsed.MaxConnIdleTime != 10*time.Minute {
		t.Fatalf("unexpected pgxpool config: %#v", parsed)
	}
	if parsed.ConnConfig.RuntimeParams["application_name"] != "ai-native-platform" {
		t.Fatalf("application_name = %q", parsed.ConnConfig.RuntimeParams["application_name"])
	}
}

func TestParsePoolConfigDoesNotLeakURL(t *testing.T) {
	_, err := ParsePoolConfig(config.Database{URL: "postgres://runtime:do-not-leak@[]", MaxConns: 1})
	if err == nil {
		t.Fatal("ParsePoolConfig succeeded, want error")
	}
	if strings.Contains(err.Error(), "do-not-leak") {
		t.Fatalf("error leaked database credential: %v", err)
	}
}

func TestOpenPoolPingsPostgreSQL(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := OpenPool(ctx, config.Database{URL: url, MaxConns: 2, MaxLifetime: time.Hour, MaxIdleTime: 5 * time.Minute})
	if err != nil {
		t.Fatalf("OpenPool: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
