package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/database/migrate"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTenantContextValidation(t *testing.T) {
	valid := TenantContext{TenantID: uuid.New(), Bucket: 127, ActorID: "agent-1"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid context: %v", err)
	}
	for _, invalid := range []TenantContext{
		{Bucket: 1, ActorID: "agent-1"},
		{TenantID: uuid.New(), Bucket: -1, ActorID: "agent-1"},
		{TenantID: uuid.New(), Bucket: 128, ActorID: "agent-1"},
		{TenantID: uuid.New(), Bucket: 1},
	} {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("context %#v passed validation", invalid)
		}
	}
}

func TestPostgreSQLTenantIsolationBaseline(t *testing.T) {
	admin, runtime := isolationTestPools(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	assertSchemaAndRole(t, ctx, admin)
	tenantA := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	insertTenant(t, ctx, admin, tenantA, "orgaaaaaaaaaaaaaaaaa", 7)
	insertTenant(t, ctx, admin, tenantB, "orgbbbbbbbbbbbbbbbbb", 42)

	objectA := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	objectB := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	recordA := uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000001")
	recordA2 := uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000002")
	recordB := uuid.MustParse("bbbbbbbb-0000-4000-8000-000000000001")
	mustTenantExec(t, ctx, runtime, TenantContext{TenantID: tenantA, Bucket: 7, ActorID: "agent-a"},
		"insert into object_record(tenant_bucket, tenant_id, object_id, record_id, data) values ($1, $2, $3, $4, $5), ($1, $2, $3, $6, $7)",
		int16(7), tenantA, objectA, recordA, json.RawMessage("{\"name\":\"a1\"}"), recordA2, json.RawMessage("{\"name\":\"a2\"}"))
	mustTenantExec(t, ctx, runtime, TenantContext{TenantID: tenantB, Bucket: 42, ActorID: "agent-b"},
		"insert into object_record(tenant_bucket, tenant_id, object_id, record_id, data) values ($1, $2, $3, $4, $5)",
		int16(42), tenantB, objectB, recordB, json.RawMessage("{\"name\":\"b1\"}"))

	if count := tenantRecordCount(t, ctx, runtime, TenantContext{TenantID: tenantA, Bucket: 7, ActorID: "agent-a"}); count != 2 {
		t.Fatalf("tenant A count=%d", count)
	}
	if count := tenantRecordCount(t, ctx, runtime, TenantContext{TenantID: tenantB, Bucket: 42, ActorID: "agent-b"}); count != 1 {
		t.Fatalf("tenant B count=%d", count)
	}
	if count := tenantRecordCount(t, ctx, runtime, TenantContext{TenantID: tenantA, Bucket: 42, ActorID: "agent-a"}); count != 0 {
		t.Fatalf("wrong bucket exposed %d records", count)
	}
	var missingContextCount int
	if err := runtime.QueryRow(ctx, "select count(*) from object_record").Scan(&missingContextCount); err != nil {
		t.Fatalf("missing context query: %v", err)
	}
	if missingContextCount != 0 {
		t.Fatalf("missing context exposed %d records", missingContextCount)
	}
	var missingAuthorizationContextCount int
	if err := runtime.QueryRow(ctx, "select count(*) from authorization_role").Scan(&missingAuthorizationContextCount); err != nil {
		t.Fatalf("missing authorization context query: %v", err)
	}
	if missingAuthorizationContextCount != 0 {
		t.Fatalf("missing context exposed %d authorization roles", missingAuthorizationContextCount)
	}

	spoofErr := WithTenant(ctx, runtime, TenantContext{TenantID: tenantA, Bucket: 7, ActorID: "agent-a"}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "insert into object_record(tenant_bucket, tenant_id, object_id, record_id) values ($1, $2, $3, $4)", int16(42), tenantB, objectB, uuid.New())
		return err
	})
	if spoofErr == nil {
		t.Fatal("RLS accepted another tenant ID")
	}
	authorizationSpoofErr := WithTenant(ctx, runtime, TenantContext{TenantID: tenantA, Bucket: 7, ActorID: "agent-a"}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "insert into authorization_role(tenant_bucket,tenant_id,role_id,name) values ($1,$2,$3,$4)", int16(42), tenantB, uuid.New(), "cross-tenant-role")
		return err
	})
	if authorizationSpoofErr == nil {
		t.Fatal("RLS accepted a cross-tenant authorization role")
	}

	mustTenantExec(t, ctx, runtime, TenantContext{TenantID: tenantA, Bucket: 7, ActorID: "agent-a"},
		"insert into record_relation(tenant_bucket, tenant_id, relation_id, relation_definition_id, source_object_id, source_record_id, target_object_id, target_record_id) values ($1,$2,$3,$4,$5,$6,$5,$7)",
		int16(7), tenantA, uuid.New(), uuid.New(), objectA, recordA, recordA2)
	crossRelationErr := WithTenant(ctx, runtime, TenantContext{TenantID: tenantA, Bucket: 7, ActorID: "agent-a"}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			"insert into record_relation(tenant_bucket, tenant_id, relation_id, relation_definition_id, source_object_id, source_record_id, target_object_id, target_record_id) values ($1,$2,$3,$4,$5,$6,$7,$8)",
			int16(7), tenantA, uuid.New(), uuid.New(), objectA, recordA, objectB, recordB)
		return err
	})
	if crossRelationErr == nil {
		t.Fatal("cross-tenant target relation was accepted")
	}

	sentinel := errors.New("rollback path")
	rollbackRecord := uuid.New()
	err := WithTenant(ctx, runtime, TenantContext{TenantID: tenantA, Bucket: 7, ActorID: "agent-a"}, func(tx pgx.Tx) error {
		if _, execErr := tx.Exec(ctx, "insert into object_record(tenant_bucket, tenant_id, object_id, record_id) values ($1,$2,$3,$4)", int16(7), tenantA, objectA, rollbackRecord); execErr != nil {
			return execErr
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("rollback error=%v", err)
	}
	assertRecordAbsent(t, ctx, admin, tenantA, rollbackRecord)

	panicRecord := uuid.New()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("WithTenant swallowed panic")
			}
		}()
		_ = WithTenant(ctx, runtime, TenantContext{TenantID: tenantA, Bucket: 7, ActorID: "agent-a"}, func(tx pgx.Tx) error {
			if _, execErr := tx.Exec(ctx, "insert into object_record(tenant_bucket, tenant_id, object_id, record_id) values ($1,$2,$3,$4)", int16(7), tenantA, objectA, panicRecord); execErr != nil {
				return execErr
			}
			panic("test panic")
		})
	}()
	assertRecordAbsent(t, ctx, admin, tenantA, panicRecord)

	var residualTenant, residualBucket, residualActor string
	if err := runtime.QueryRow(ctx,
		"select coalesce(current_setting('app.tenant_id', true), ''), coalesce(current_setting('app.tenant_bucket', true), ''), coalesce(current_setting('app.actor_id', true), '')",
	).Scan(&residualTenant, &residualBucket, &residualActor); err != nil {
		t.Fatalf("read residual context: %v", err)
	}
	if residualTenant != "" || residualBucket != "" || residualActor != "" {
		t.Fatalf("connection leaked tenant context: tenant=%q bucket=%q actor=%q", residualTenant, residualBucket, residualActor)
	}

	assertPartitionPruning(t, ctx, admin, tenantA, 7)
	if bucket, err := SelectLeastUsedBucket(ctx, admin); err != nil || bucket != 0 {
		t.Fatalf("least-used bucket=%d err=%v", bucket, err)
	}
	insertTenant(t, ctx, admin, uuid.MustParse("33333333-3333-4333-8333-333333333333"), "orgccccccccccccccccc", 0)
	if bucket, err := SelectLeastUsedBucket(ctx, admin); err != nil || bucket != 1 {
		t.Fatalf("least-used bucket after bucket 0 use=%d err=%v", bucket, err)
	}
}

func isolationTestPools(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	lock, err := admin.Acquire(context.Background())
	if err != nil {
		admin.Close()
		t.Fatalf("acquire integration lock connection: %v", err)
	}
	if _, err := lock.Exec(context.Background(), "select pg_advisory_lock(7167614658367249410)"); err != nil {
		lock.Release()
		admin.Close()
		t.Fatalf("acquire integration lock: %v", err)
	}
	if err := migrate.Apply(context.Background(), admin, migrate.Builtin()); err != nil {
		admin.Close()
		t.Fatalf("apply builtin migrations: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "alter role ai_native_runtime login"); err != nil {
		admin.Close()
		t.Fatalf("enable local test login: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "truncate record_relation, object_record, tenant_operation, audit_event, tenant_registry cascade"); err != nil {
		admin.Close()
		t.Fatalf("reset isolation data: %v", err)
	}
	runtimeConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		admin.Close()
		t.Fatalf("parse runtime config: %v", err)
	}
	runtimeConfig.ConnConfig.User = "ai_native_runtime"
	runtimeConfig.MaxConns = 1
	runtime, err := pgxpool.NewWithConfig(context.Background(), runtimeConfig)
	if err != nil {
		admin.Close()
		t.Fatalf("open runtime pool: %v", err)
	}
	if err := runtime.Ping(context.Background()); err != nil {
		admin.Close()
		runtime.Close()
		t.Fatalf("ping runtime pool: %v", err)
	}
	t.Cleanup(func() {
		runtime.Close()
		_, _ = lock.Exec(context.Background(), "select pg_advisory_unlock(7167614658367249410)")
		lock.Release()
		admin.Close()
	})
	return admin, runtime
}

func assertSchemaAndRole(t *testing.T, ctx context.Context, admin *pgxpool.Pool) {
	t.Helper()
	var companyColumn, legacyOrgColumn int
	if err := admin.QueryRow(ctx, "select count(*) from information_schema.columns where table_schema='public' and table_name='tenant_registry' and column_name='company_id'").Scan(&companyColumn); err != nil || companyColumn != 1 {
		t.Fatalf("tenant_registry.company_id column count=%d err=%v", companyColumn, err)
	}
	if err := admin.QueryRow(ctx, "select count(*) from information_schema.columns where table_schema='public' and table_name='tenant_registry' and column_name='org_id'").Scan(&legacyOrgColumn); err != nil || legacyOrgColumn != 0 {
		t.Fatalf("tenant_registry.org_id column count=%d err=%v", legacyOrgColumn, err)
	}
	var partitions int
	if err := admin.QueryRow(ctx, "select count(*) from pg_inherits where inhparent = 'object_record'::regclass").Scan(&partitions); err != nil || partitions != 128 {
		t.Fatalf("partitions=%d err=%v", partitions, err)
	}
	var unsecured int
	if err := admin.QueryRow(ctx,
		"select count(*) from pg_class where relname in ('tenant_registry','tenant_operation','audit_event','object_record','record_relation','principal_projection','organization_node','organization_closure','principal_org_membership','access_group','group_membership','authorization_role','permission_set','authorization_permission','permission_set_permission','role_permission_set','principal_role_assignment','role_data_scope','role_conflict','object_authorization_policy','record_team_member','share_grant','sharing_rule_def','share_projection','permission_snapshot','organization_merge_operation','record_organization_history') and (not relrowsecurity or not relforcerowsecurity)",
	).Scan(&unsecured); err != nil || unsecured != 0 {
		t.Fatalf("unsecured tenant tables=%d err=%v", unsecured, err)
	}
	var unsecuredPartitions int
	if err := admin.QueryRow(ctx,
		"select count(*) from pg_class child join pg_inherits i on i.inhrelid=child.oid where i.inhparent='object_record'::regclass and (not child.relrowsecurity or not child.relforcerowsecurity)",
	).Scan(&unsecuredPartitions); err != nil || unsecuredPartitions != 0 {
		t.Fatalf("unsecured partitions=%d err=%v", unsecuredPartitions, err)
	}
	for _, role := range []string{"ai_native_runtime", "ai_native_control"} {
		var superuser, bypassRLS bool
		if err := admin.QueryRow(ctx, "select rolsuper, rolbypassrls from pg_roles where rolname=$1", role).Scan(&superuser, &bypassRLS); err != nil {
			t.Fatalf("read %s role: %v", role, err)
		}
		if superuser || bypassRLS {
			t.Fatalf("unsafe %s role: superuser=%v bypassRLS=%v", role, superuser, bypassRLS)
		}
	}
	var applicationOwned int
	if err := admin.QueryRow(ctx,
		"select count(*) from pg_class where relname in ('tenant_registry','tenant_operation','audit_event','object_record','record_relation','principal_projection','organization_node','organization_closure','principal_org_membership','access_group','group_membership','authorization_role','permission_set','authorization_permission','permission_set_permission','role_permission_set','principal_role_assignment','role_data_scope','role_conflict','object_authorization_policy','record_team_member','share_grant','sharing_rule_def','share_projection','permission_snapshot','organization_merge_operation','record_organization_history') and pg_get_userbyid(relowner) in ('ai_native_runtime','ai_native_control')",
	).Scan(&applicationOwned); err != nil || applicationOwned != 0 {
		t.Fatalf("application-owned tenant tables=%d err=%v", applicationOwned, err)
	}
	var controlGrantedTables []string
	if err := admin.QueryRow(ctx,
		"select coalesce(array_agg(distinct table_name::text order by table_name::text), array[]::text[]) from information_schema.role_table_grants where grantee='ai_native_control' and table_schema='public'",
	).Scan(&controlGrantedTables); err != nil {
		t.Fatalf("read control role grants: %v", err)
	}
	expectedControlTables := []string{"audit_event", "tenant_operation", "tenant_registry"}
	if fmt.Sprint(controlGrantedTables) != fmt.Sprint(expectedControlTables) {
		t.Fatalf("control role tables=%v want=%v", controlGrantedTables, expectedControlTables)
	}
	for _, table := range []string{"shard_registry", "object_record", "record_relation", "metadata_version", "object_definition", "field_definition", "relation_definition", "authorization_role", "permission_set", "authorization_permission", "role_data_scope", "share_grant", "share_projection", "organization_merge_operation"} {
		var granted bool
		if err := admin.QueryRow(ctx,
			"select has_table_privilege('ai_native_control', $1, 'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')",
			table,
		).Scan(&granted); err != nil {
			t.Fatalf("read ai_native_control privilege for %s: %v", table, err)
		}
		if granted {
			t.Fatalf("ai_native_control unexpectedly has a privilege on %s", table)
		}
	}
}

func insertTenant(t *testing.T, ctx context.Context, admin *pgxpool.Pool, tenantID uuid.UUID, companyID string, bucket int16) {
	t.Helper()
	_, err := admin.Exec(ctx,
		"insert into tenant_registry(tenant_id,company_id,display_name,shard_id,tenant_bucket,service_tier,global_lifecycle_status,native_status,tenant_revision,product_revision,route_revision) values ($1,$2,$3,'shard-001',$4,'standard','active','active',1,1,1)",
		tenantID, companyID, "Tenant "+companyID, bucket)
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
}

func mustTenantExec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenant TenantContext, statement string, arguments ...any) {
	t.Helper()
	if err := WithTenant(ctx, pool, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, arguments...)
		return err
	}); err != nil {
		t.Fatalf("tenant exec: %v", err)
	}
}

func tenantRecordCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenant TenantContext) int {
	t.Helper()
	var count int
	if err := WithTenant(ctx, pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, "select count(*) from object_record").Scan(&count)
	}); err != nil {
		t.Fatalf("tenant count: %v", err)
	}
	return count
}

func assertRecordAbsent(t *testing.T, ctx context.Context, admin *pgxpool.Pool, tenantID, recordID uuid.UUID) {
	t.Helper()
	var exists bool
	if err := admin.QueryRow(ctx, "select exists(select 1 from object_record where tenant_id=$1 and record_id=$2)", tenantID, recordID).Scan(&exists); err != nil {
		t.Fatalf("check record: %v", err)
	}
	if exists {
		t.Fatalf("record %s survived rollback", recordID)
	}
}

func assertPartitionPruning(t *testing.T, ctx context.Context, admin *pgxpool.Pool, tenantID uuid.UUID, bucket int16) {
	t.Helper()
	var planBytes []byte
	if err := admin.QueryRow(ctx, "explain (format json, costs off) select * from object_record where tenant_bucket=$1 and tenant_id=$2", bucket, tenantID).Scan(&planBytes); err != nil {
		t.Fatalf("explain partition query: %v", err)
	}
	plan := string(planBytes)
	expected := fmt.Sprintf("object_record_b%03d", bucket)
	if !strings.Contains(plan, expected) {
		t.Fatalf("plan did not use %s: %s", expected, plan)
	}
	other := fmt.Sprintf("object_record_b%03d", (int(bucket)+1)%128)
	if strings.Contains(plan, other) {
		t.Fatalf("plan failed to prune %s: %s", other, plan)
	}
}
