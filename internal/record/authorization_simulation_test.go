package record

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/authorization"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/metadata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAuthorizationLocalSimulation exercises the authorization SQL against a
// real local PostgreSQL dataset. It intentionally uses record-to-group
// projections and organization closure at query time, never a record-to-user
// ACL expansion. It is opt-in because it creates a sizeable local dataset.
func TestAuthorizationLocalSimulation(t *testing.T) {
	if os.Getenv("AI_NATIVE_RUN_AUTHORIZATION_SIMULATION") != "1" {
		t.Skip("set AI_NATIVE_RUN_AUTHORIZATION_SIMULATION=1 to run the local authorization simulation")
	}
	recordCount := simulationEnvInt(t, "AI_NATIVE_AUTHORIZATION_SIMULATION_RECORDS", 100_000, 1)
	concurrency := simulationEnvInt(t, "AI_NATIVE_AUTHORIZATION_SIMULATION_CONCURRENCY", 50, 1)

	admin, control, runtime := recordTestPools(t)
	metadataService := metadata.NewService(runtime, control)
	invoker := capability.NewInvoker(capability.NewRegistry(metadata.CapabilityDefinitions(metadataService)), 4)
	principal := recordPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa", "authorization-simulation-owner")
	ids := publishRecordModel(t, invoker, principal)

	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: "authorization-simulation-reader"}
	organizationRoot, organizationShared := uuid.New(), uuid.New()
	groupID, ruleID := uuid.New(), uuid.New()
	seedAuthorizationSimulation(t, runtime, tenant, ids.CustomerID, organizationRoot, organizationShared, groupID, ruleID)

	var metadataVersionID uuid.UUID
	if err := control.QueryRow(context.Background(), "select metadata_version_id from tenant_registry where tenant_id=$1", tenant.TenantID).Scan(&metadataVersionID); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	recordIDExpression := "('019d0000-0000-7000-8000-' || lpad(to_hex(i),12,'0'))::uuid"
	if _, err := admin.Exec(context.Background(), fmt.Sprintf(`
		insert into object_record(tenant_bucket,tenant_id,metadata_version_id,object_id,record_id,owner_id,data_organization_id,lifecycle_state,data,revision,created_by,updated_by)
		select 7,$1,$2,$3,%s,'authorization-simulation-owner',
			case when mod(i,10)=0 then $5::uuid else $4::uuid end,
			'active',jsonb_build_object('name','authorization-simulation-'||i),1,'authorization-simulation-owner','authorization-simulation-owner'
		from generate_series(1,$6) i`, recordIDExpression), tenant.TenantID, metadataVersionID, ids.CustomerID, organizationRoot, organizationShared, recordCount); err != nil {
		t.Fatal(err)
	}
	insertDuration := time.Since(started)
	if _, err := admin.Exec(context.Background(), "analyze object_record; analyze share_projection"); err != nil {
		t.Fatal(err)
	}
	var projected int
	if err := admin.QueryRow(context.Background(), "select count(*) from share_projection where tenant_bucket=$1 and tenant_id=$2 and rule_id=$3", tenant.Bucket, tenant.TenantID, ruleID).Scan(&projected); err != nil {
		t.Fatal(err)
	}
	if want := recordCount / 10; projected != want {
		t.Fatalf("record-to-group projection count=%d want=%d", projected, want)
	}

	model := objectModel{MetadataVersionID: metadataVersionID.String(), ObjectID: ids.CustomerID.String(), APIName: "customer"}
	organizationScope := authorization.RecordScope{DescendantRoots: []uuid.UUID{organizationRoot}}
	sharedScope := authorization.RecordScope{IncludeTeamShare: true}
	if records := querySimulationRecords(t, runtime, tenant, model, organizationScope, 50); len(records) != 50 {
		t.Fatalf("organization scope returned %d records, want 50", len(records))
	}
	sharedRecords := querySimulationRecords(t, runtime, tenant, model, sharedScope, 50)
	if len(sharedRecords) != 50 {
		t.Fatalf("record-to-group sharing returned %d records, want 50", len(sharedRecords))
	}
	for _, record := range sharedRecords {
		if record.DataOrganizationID != organizationShared.String() {
			t.Fatalf("shared scope exposed record outside rule target organization: %#v", record)
		}
	}

	concurrentRuntime := poolAs(t, os.Getenv("TEST_DATABASE_URL"), "ai_native_runtime", int32(concurrency))
	defer concurrentRuntime.Close()
	latencies := make([]time.Duration, concurrency)
	errs := make(chan error, concurrency)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for index := 0; index < concurrency; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			workerStarted := time.Now()
			var count int
			err := database.WithTenant(context.Background(), concurrentRuntime, tenant, func(tx pgx.Tx) error {
				statement, arguments := buildQuery(tenant, model, nil, uuid.Nil, 50, sharedScope, tenant.ActorID)
				rows, err := tx.Query(context.Background(), statement, arguments...)
				if err != nil {
					return err
				}
				defer rows.Close()
				for rows.Next() {
					count++
					if _, err := rows.Values(); err != nil {
						return err
					}
				}
				return rows.Err()
			})
			latencies[index] = time.Since(workerStarted)
			if err != nil {
				errs <- err
				return
			}
			if count != 50 {
				errs <- fmt.Errorf("worker %d returned %d records, want 50", index, count)
			}
		}(index)
	}
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Slice(latencies, func(left, right int) bool { return latencies[left] < latencies[right] })
	t.Logf("AUTHORIZATION_SIMULATION records=%d projections=%d concurrency=%d insert_ms=%.3f query_p50_ms=%.3f query_p95_ms=%.3f", recordCount, projected, concurrency, durationMilliseconds(insertDuration), durationMilliseconds(latencies[len(latencies)/2]), durationMilliseconds(latencies[(len(latencies)*95)/100]))
}

func simulationEnvInt(t *testing.T, key string, defaultValue, minimum int) int {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum {
		t.Fatalf("invalid %s=%q", key, value)
	}
	return parsed
}

func seedAuthorizationSimulation(t *testing.T, runtime *pgxpool.Pool, tenant database.TenantContext, objectID, organizationRoot, organizationShared, groupID, ruleID uuid.UUID) {
	t.Helper()
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(), "insert into principal_projection(tenant_bucket,tenant_id,principal_id,principal_type) values ($1,$2,$3,'user')", tenant.Bucket, tenant.TenantID, tenant.ActorID); err != nil {
			return err
		}
		if _, err := tx.Exec(context.Background(), "insert into organization_node(tenant_bucket,tenant_id,organization_id,parent_organization_id,name) values ($1,$2,$3,null,'simulation-root'),($1,$2,$4,$3,'simulation-shared')", tenant.Bucket, tenant.TenantID, organizationRoot, organizationShared); err != nil {
			return err
		}
		if _, err := tx.Exec(context.Background(), "insert into organization_closure(tenant_bucket,tenant_id,ancestor_organization_id,descendant_organization_id,depth) values ($1,$2,$3,$3,0),($1,$2,$4,$4,0),($1,$2,$3,$4,1)", tenant.Bucket, tenant.TenantID, organizationRoot, organizationShared); err != nil {
			return err
		}
		if _, err := tx.Exec(context.Background(), "insert into access_group(tenant_bucket,tenant_id,group_id,name) values ($1,$2,$3,'simulation-readers')", tenant.Bucket, tenant.TenantID, groupID); err != nil {
			return err
		}
		if _, err := tx.Exec(context.Background(), "insert into group_membership(tenant_bucket,tenant_id,group_id,principal_id) values ($1,$2,$3,$4)", tenant.Bucket, tenant.TenantID, groupID, tenant.ActorID); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(), "insert into sharing_rule_def(tenant_bucket,tenant_id,rule_id,object_id,name,condition_expression,grantee_group_id,access_level,projection_state) values ($1,$2,$3,$4,'simulation-share-rule',jsonb_build_object('data_organization_id',$5::text),$6,'read','ready')", tenant.Bucket, tenant.TenantID, ruleID, objectID, organizationShared, groupID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func querySimulationRecords(t *testing.T, runtime *pgxpool.Pool, tenant database.TenantContext, model objectModel, scope authorization.RecordScope, limit int) []Record {
	t.Helper()
	var records []Record
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		statement, arguments := buildQuery(tenant, model, nil, uuid.Nil, limit, scope, tenant.ActorID)
		rows, err := tx.Query(context.Background(), statement, arguments...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var record Record
			if err := scanRecord(rows, model.APIName, &record); err != nil {
				return err
			}
			records = append(records, record)
		}
		return rows.Err()
	}); err != nil {
		t.Fatal(err)
	}
	return records
}
