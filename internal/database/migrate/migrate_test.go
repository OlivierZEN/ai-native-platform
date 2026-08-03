package migrate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplyIsAtomicRepeatableAndChecksumProtected(t *testing.T) {
	pool := migrationTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	base := []Migration{{Version: 1, Name: "widgets", SQL: "create table widget(id bigint primary key);"}}
	if err := Apply(ctx, pool, base); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	if err := Apply(ctx, pool, base); err != nil {
		t.Fatalf("repeat Apply: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, "select count(*) from schema_migration").Scan(&count); err != nil || count != 1 {
		t.Fatalf("migration count=%d err=%v", count, err)
	}

	drift := []Migration{{Version: 1, Name: "widgets", SQL: "create table widget(id uuid primary key);"}}
	if err := Apply(ctx, pool, drift); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("drift Apply error=%v, want checksum error", err)
	}

	broken := append(base, Migration{Version: 2, Name: "broken", SQL: "create table"})
	if err := Apply(ctx, pool, broken); err == nil {
		t.Fatal("broken Apply succeeded")
	}
	if err := pool.QueryRow(ctx, "select count(*) from schema_migration").Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed migration was registered: count=%d err=%v", count, err)
	}
}

func TestApplySerializesConcurrentRunners(t *testing.T) {
	pool := migrationTestPool(t)
	migrations := []Migration{{Version: 1, Name: "widgets", SQL: "create table widget(id bigint primary key);"}}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var group sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			errors <- Apply(ctx, pool, migrations)
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent Apply: %v", err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "select count(*) from schema_migration").Scan(&count); err != nil || count != 1 {
		t.Fatalf("migration count=%d err=%v", count, err)
	}
}

func TestBuiltinPreservesPublishedMigration17Checksum(t *testing.T) {
	const publishedChecksum = "add7e8042c8a177431849080d4bb519212d49c288cf48b39ab02ee8b1f8fed20"
	for _, migration := range Builtin() {
		if migration.Version == 17 {
			if actual := checksum(migration.SQL); actual != publishedChecksum {
				t.Fatalf("migration 17 checksum=%s, want published %s", actual, publishedChecksum)
			}
			return
		}
	}
	t.Fatal("migration 17 is not registered")
}

func migrationTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	sum := sha256.Sum256([]byte(t.Name()))
	schema := fmt.Sprintf("migration_%x", sum[:6])
	admin, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "drop schema if exists "+schema+" cascade"); err != nil {
		t.Fatalf("drop test schema: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "create schema "+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	admin.Close()

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	config.MaxConns = 4
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanup, cleanupErr := pgxpool.New(context.Background(), url)
		if cleanupErr == nil {
			_, _ = cleanup.Exec(context.Background(), "drop schema if exists "+schema+" cascade")
			cleanup.Close()
		}
	})
	return pool
}
