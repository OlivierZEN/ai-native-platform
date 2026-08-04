package console

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/OlivierZEN/ai-native-platform/internal/database/migrate"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresReaderMembersUsesValidAggregateOrdering(t *testing.T) {
	admin, runtime, control := consoleTestPools(t)
	ctx := context.Background()
	tenantID := uuid.MustParse("50500000-0000-4000-8000-000000000001")
	companyID := "org50500000000000000"

	if _, err := admin.Exec(ctx, `
		insert into tenant_registry(
			tenant_id,company_id,display_name,shard_id,tenant_bucket,service_tier,
			global_lifecycle_status,native_status,tenant_revision,product_revision,route_revision
		) values ($1,$2,'Console aggregate test','shard-001',5,'standard','active','active',1,1,1)`, tenantID, companyID); err != nil {
		t.Fatalf("insert console tenant: %v", err)
	}
	if _, err := admin.Exec(ctx, `
		insert into principal_projection(
			tenant_bucket,tenant_id,principal_id,principal_type,display_name,public_id,status
		) values (5,$1,'console-director','user','Console Director','U-CONSOLE','active')`, tenantID); err != nil {
		t.Fatalf("insert console principal: %v", err)
	}

	value, err := NewPostgresReader(runtime, control).Read(ctx, session{
		TenantID: tenantID.String(), CompanyID: companyID, Subject: "console-director",
	}, "/console/api/members")
	if err != nil {
		t.Fatalf("read console members: %v", err)
	}
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("members result type=%T", value)
	}
	members, ok := result["members"].([]map[string]any)
	if !ok || len(members) != 1 {
		t.Fatalf("members=%#v", result["members"])
	}
	if members[0]["name"] != "Console Director" || members[0]["account"] != "U-CONSOLE" {
		t.Fatalf("member=%#v", members[0])
	}
}

func consoleTestPools(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open console admin pool: %v", err)
	}
	lock, err := admin.Acquire(ctx)
	if err != nil {
		admin.Close()
		t.Fatalf("acquire console integration lock connection: %v", err)
	}
	if _, err := lock.Exec(ctx, "select pg_advisory_lock(7167614658367249410)"); err != nil {
		lock.Release()
		admin.Close()
		t.Fatalf("acquire console integration lock: %v", err)
	}
	if err := migrate.Apply(ctx, admin, migrate.Builtin()); err != nil {
		_, _ = lock.Exec(ctx, "select pg_advisory_unlock(7167614658367249410)")
		lock.Release()
		admin.Close()
		t.Fatalf("apply console migrations: %v", err)
	}
	if _, err := admin.Exec(ctx, "alter role ai_native_runtime login; alter role ai_native_control login"); err != nil {
		_, _ = lock.Exec(ctx, "select pg_advisory_unlock(7167614658367249410)")
		lock.Release()
		admin.Close()
		t.Fatalf("enable console test roles: %v", err)
	}
	if _, err := admin.Exec(ctx, "truncate tenant_registry cascade"); err != nil {
		_, _ = lock.Exec(ctx, "select pg_advisory_unlock(7167614658367249410)")
		lock.Release()
		admin.Close()
		t.Fatalf("reset console test data: %v", err)
	}

	openRolePool := func(role string) *pgxpool.Pool {
		config, err := pgxpool.ParseConfig(databaseURL)
		if err != nil {
			t.Fatalf("parse %s console pool: %v", role, err)
		}
		config.ConnConfig.User = role
		config.MaxConns = 1
		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			t.Fatalf("open %s console pool: %v", role, err)
		}
		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			t.Fatalf("ping %s console pool: %v", role, err)
		}
		return pool
	}
	runtime := openRolePool("ai_native_runtime")
	control := openRolePool("ai_native_control")
	t.Cleanup(func() {
		control.Close()
		runtime.Close()
		_, _ = lock.Exec(context.Background(), "select pg_advisory_unlock(7167614658367249410)")
		lock.Release()
		admin.Close()
	})
	return admin, runtime, control
}

func TestObjectFieldsEncodeEmptyCollectionAsArray(t *testing.T) {
	payload, err := json.Marshal(map[string]any{"fields": objectFields(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != `{"fields":[]}` {
		t.Fatalf("payload=%s, want fields to be an empty array", payload)
	}
}

func TestObjectFieldsPreservePublishedFields(t *testing.T) {
	fields := []map[string]string{{"key": "name"}}
	if got := objectFields(fields); len(got) != 1 || got[0]["key"] != "name" {
		t.Fatalf("fields=%v, want published fields preserved", got)
	}
}
