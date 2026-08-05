package principal

import (
	"context"
	"testing"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestServicePrincipalSyncReconcilesTrustedClientIDRename(t *testing.T) {
	_, control, runtime := principalTestPools(t)
	const tenantID = "11111111-1111-4111-8111-111111111111"
	const companyID = "orgaaaaaaaaaaaaaaaaa"
	const ownerID = "human-oliver"
	const serviceID = "service-wukong"
	const oldClientID = "dev-autopilot-developer"
	const newClientID = "dev-autopilot-developer-wukong"
	tenant := database.TenantContext{TenantID: uuid.MustParse(tenantID), Bucket: 7, ActorID: serviceID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `insert into principal_projection(
			tenant_bucket,tenant_id,principal_id,principal_type,status,display_name,owner_principal_id,client_id)
			values($1,$2,$3,'user','active','Oliver',null,null),
			($1,$2,$4,'service','active','悟空',$3,$5)`,
			tenant.Bucket, tenant.TenantID, ownerID, serviceID, oldClientID)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	actor := capability.Actor{ID: serviceID, Scopes: []string{"identity.principal.sync"}}
	trusted := capability.TrustedPrincipal{
		TenantID: tenantID, CompanyID: companyID, PrincipalID: serviceID, PrincipalType: "SERVICE",
		OwnerPrincipalID: ownerID, ClientID: newClientID, Actor: actor, Source: "official_oact",
	}
	service := NewService(runtime, control)
	projection, stableErr := service.Sync(context.Background(), capability.Request{
		RequestID: "req-client-id-rename", CapabilityID: "identity.principal.sync", TenantID: tenantID,
		Actor: actor, Principal: &trusted,
	}, SyncInput{})
	if stableErr != nil {
		t.Fatalf("sync trusted client ID rename: %#v", stableErr)
	}
	if projection.PrincipalID != serviceID || projection.PrincipalType != "SERVICE" || projection.OwnerPrincipalID != ownerID || projection.ClientID != newClientID || projection.Status != "active" {
		t.Fatalf("projection=%#v", projection)
	}

	var clientID string
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `select client_id from principal_projection
			where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3`, tenant.Bucket, tenant.TenantID, serviceID).Scan(&clientID)
	}); err != nil {
		t.Fatal(err)
	}
	if clientID != newClientID {
		t.Fatalf("client_id=%q want %q", clientID, newClientID)
	}
}
