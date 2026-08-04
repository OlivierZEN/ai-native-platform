package principal

import (
	"context"
	"os"
	"testing"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/database/migrate"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSetOrganizationMembershipMaintainsOnePrimaryAndInvalidatesSnapshot(t *testing.T) {
	_, control, runtime := principalTestPools(t)
	const tenantID = "11111111-1111-4111-8111-111111111111"
	const companyID = "orgaaaaaaaaaaaaaaaaa"
	const managerID = "human-manager"
	const targetID = "service-target"
	organizationA, organizationB := uuid.New(), uuid.New()
	seedPrincipalMembershipTestData(t, runtime, tenantID, managerID, targetID, organizationA, organizationB)

	actor := capability.Actor{ID: managerID, Scopes: []string{"authorization.manage"}}
	trusted := capability.TrustedPrincipal{
		TenantID: tenantID, CompanyID: companyID, PrincipalID: managerID,
		PrincipalType: "HUMAN", Actor: actor, Source: "official_oact",
		Approvals: []string{"approval-membership"},
	}
	request := capability.Request{
		RequestID: "req-membership-a", CapabilityID: "identity.principal.set-organization-membership",
		TenantID: tenantID, Actor: actor, Principal: &trusted,
	}
	service := NewService(runtime, control)

	first, stableErr := service.SetOrganizationMembership(context.Background(), request, SetOrganizationMembershipInput{
		PrincipalID: targetID, OrganizationID: organizationA.String(), Active: true, Primary: true, ApprovalID: "approval-membership",
	})
	if stableErr != nil {
		t.Fatalf("assign first primary membership: %#v", stableErr)
	}
	if first.Status != "active" || !first.Primary || first.OrganizationID != organizationA.String() {
		t.Fatalf("first membership=%#v", first)
	}
	assertPrincipalSnapshotCount(t, runtime, tenantID, targetID, 0)
	seedPrincipalSnapshot(t, runtime, tenantID, targetID)

	request.RequestID = "req-membership-b"
	second, stableErr := service.SetOrganizationMembership(context.Background(), request, SetOrganizationMembershipInput{
		PrincipalID: targetID, OrganizationID: organizationB.String(), Active: true, Primary: true, ApprovalID: "approval-membership",
	})
	if stableErr != nil {
		t.Fatalf("switch primary membership: %#v", stableErr)
	}
	if second.Status != "active" || !second.Primary || second.OrganizationID != organizationB.String() {
		t.Fatalf("second membership=%#v", second)
	}
	assertPrincipalMembershipCounts(t, runtime, tenantID, targetID, 1, 1)
	assertPrincipalMembershipState(t, runtime, tenantID, targetID, organizationA, "ended", false)
	assertPrincipalSnapshotCount(t, runtime, tenantID, targetID, 0)

	request.RequestID = "req-membership-end"
	ended, stableErr := service.SetOrganizationMembership(context.Background(), request, SetOrganizationMembershipInput{
		PrincipalID: targetID, OrganizationID: organizationB.String(), Active: false, Primary: false, ApprovalID: "approval-membership",
	})
	if stableErr != nil {
		t.Fatalf("end membership: %#v", stableErr)
	}
	if ended.Status != "ended" || ended.Primary {
		t.Fatalf("ended membership=%#v", ended)
	}
	assertPrincipalMembershipCounts(t, runtime, tenantID, targetID, 0, 0)

	tenant := database.TenantContext{TenantID: uuid.MustParse(tenantID), Bucket: 7, ActorID: managerID}
	var auditCount int
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `select count(*) from audit_event
			where tenant_bucket=$1 and tenant_id=$2 and actor_id=$3 and capability_id='identity.principal.set-organization-membership' and status='succeeded'`,
			tenant.Bucket, tenant.TenantID, managerID).Scan(&auditCount)
	}); err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Fatalf("membership audit count=%d want=3", auditCount)
	}
}

func principalTestPools(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := admin.Acquire(context.Background())
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	if _, err := lock.Exec(context.Background(), "select pg_advisory_lock(7167614658367249410)"); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), admin, migrate.Builtin()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(context.Background(), "alter role ai_native_control login"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(context.Background(), "alter role ai_native_runtime login"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(context.Background(), "truncate tenant_registry cascade"); err != nil {
		t.Fatal(err)
	}
	control := principalPoolAs(t, databaseURL, "ai_native_control")
	runtime := principalPoolAs(t, databaseURL, "ai_native_runtime")
	if _, err := control.Exec(context.Background(), `insert into tenant_registry(
		tenant_id,company_id,display_name,shard_id,tenant_bucket,service_tier,global_lifecycle_status,native_status,tenant_revision,product_revision,route_revision)
		values('11111111-1111-4111-8111-111111111111','orgaaaaaaaaaaaaaaaaa','Principal tenant','shard-001',7,'standard','active','active',1,1,1)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		runtime.Close()
		control.Close()
		_, _ = lock.Exec(context.Background(), "select pg_advisory_unlock(7167614658367249410)")
		lock.Release()
		admin.Close()
	})
	return admin, control, runtime
}

func principalPoolAs(t *testing.T, databaseURL, user string) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.User = user
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	return pool
}

func seedPrincipalMembershipTestData(t *testing.T, runtime *pgxpool.Pool, tenantID, managerID, targetID string, organizationA, organizationB uuid.UUID) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(tenantID), Bucket: 7, ActorID: managerID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(), `insert into principal_projection(tenant_bucket,tenant_id,principal_id,principal_type)
			values($1,$2,$3,'user'),($1,$2,$4,'service')`, tenant.Bucket, tenant.TenantID, managerID, targetID); err != nil {
			return err
		}
		if _, err := tx.Exec(context.Background(), `insert into organization_node(tenant_bucket,tenant_id,organization_id,name)
			values($1,$2,$3,'Engineering A'),($1,$2,$4,'Engineering B')`, tenant.Bucket, tenant.TenantID, organizationA, organizationB); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(), `insert into permission_snapshot(tenant_bucket,tenant_id,principal_id,snapshot_version,payload)
			values($1,$2,$3,1,'{}'::jsonb)`, tenant.Bucket, tenant.TenantID, targetID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func seedPrincipalSnapshot(t *testing.T, runtime *pgxpool.Pool, tenantID, targetID string) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(tenantID), Bucket: 7, ActorID: "human-manager"}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `insert into permission_snapshot(tenant_bucket,tenant_id,principal_id,snapshot_version,payload)
			values($1,$2,$3,2,'{}'::jsonb)`, tenant.Bucket, tenant.TenantID, targetID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func assertPrincipalSnapshotCount(t *testing.T, runtime *pgxpool.Pool, tenantID, targetID string, want int) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(tenantID), Bucket: 7, ActorID: "human-manager"}
	var count int
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `select count(*) from permission_snapshot where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3`, tenant.Bucket, tenant.TenantID, targetID).Scan(&count)
	}); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("permission snapshot count=%d want=%d", count, want)
	}
}

func assertPrincipalMembershipCounts(t *testing.T, runtime *pgxpool.Pool, tenantID, targetID string, active, primary int) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(tenantID), Bucket: 7, ActorID: "human-manager"}
	var activeCount, primaryCount int
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `select count(*) filter(where membership_state='active'),
			count(*) filter(where membership_state='active' and is_primary) from principal_org_membership
			where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3`, tenant.Bucket, tenant.TenantID, targetID).Scan(&activeCount, &primaryCount)
	}); err != nil {
		t.Fatal(err)
	}
	if activeCount != active || primaryCount != primary {
		t.Fatalf("membership counts active=%d primary=%d want=%d/%d", activeCount, primaryCount, active, primary)
	}
}

func assertPrincipalMembershipState(t *testing.T, runtime *pgxpool.Pool, tenantID, targetID string, organizationID uuid.UUID, status string, primary bool) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(tenantID), Bucket: 7, ActorID: "human-manager"}
	var gotStatus string
	var gotPrimary bool
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `select membership_state,is_primary from principal_org_membership
			where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and organization_id=$4
			order by created_at desc limit 1`, tenant.Bucket, tenant.TenantID, targetID, organizationID).Scan(&gotStatus, &gotPrimary)
	}); err != nil {
		t.Fatal(err)
	}
	if gotStatus != status || gotPrimary != primary {
		t.Fatalf("membership state=%s primary=%v want=%s/%v", gotStatus, gotPrimary, status, primary)
	}
}
