package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/api"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/cli"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/database/migrate"
	"github.com/OlivierZEN/ai-native-platform/internal/governance"
	mcpserver "github.com/OlivierZEN/ai-native-platform/internal/mcp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMetadataCoreDeterministicPublishAndIsolation(t *testing.T) {
	admin, runtime := metadataTestPools(t)
	service := NewService(runtime, admin)
	invoker := capability.NewInvoker(capability.NewRegistry(CapabilityDefinitions(service)), 4)
	principal := metadataPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa")

	version1 := createVersion(t, invoker, principal)
	if parsed := uuid.MustParse(version1.MetadataVersionID); parsed.Version() != 7 {
		t.Fatalf("metadata version is not UUIDv7: %s", parsed)
	}
	customerID := mustV7(t)
	contactID := mustV7(t)
	customerNameFieldID := mustV7(t)
	contactNameFieldID := mustV7(t)
	relationID := mustV7(t)
	upsertObject(t, invoker, principal, version1.MetadataVersionID, contactID, "contact", "Contact")
	upsertObject(t, invoker, principal, version1.MetadataVersionID, customerID, "customer", "Customer")
	upsertField(t, invoker, principal, version1.MetadataVersionID, contactNameFieldID, contactID, "name")
	upsertField(t, invoker, principal, version1.MetadataVersionID, customerNameFieldID, customerID, "name")
	upsertRelation(t, invoker, principal, version1.MetadataVersionID, relationID, "contact_customer", contactID, customerID)
	encoded1, digest1, stableErr := service.Compile(context.Background(), boundRequest(t, principal, "metadata.version.get", map[string]any{}), version1.MetadataVersionID)
	if stableErr != nil {
		t.Fatalf("compile version 1: %v", stableErr)
	}

	version2 := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, version2.MetadataVersionID, customerID, "customer", "Customer")
	upsertObject(t, invoker, principal, version2.MetadataVersionID, contactID, "contact", "Contact")
	upsertField(t, invoker, principal, version2.MetadataVersionID, customerNameFieldID, customerID, "name")
	upsertField(t, invoker, principal, version2.MetadataVersionID, contactNameFieldID, contactID, "name")
	upsertRelation(t, invoker, principal, version2.MetadataVersionID, relationID, "contact_customer", contactID, customerID)
	encoded2, digest2, stableErr := service.Compile(context.Background(), boundRequest(t, principal, "metadata.version.get", map[string]any{}), version2.MetadataVersionID)
	if stableErr != nil {
		t.Fatalf("compile version 2: %v", stableErr)
	}
	if digest1 != digest2 || !bytes.Equal(encoded1, encoded2) {
		t.Fatalf("deterministic compilation drift: digest1=%s digest2=%s\n%s\n%s", digest1, digest2, encoded1, encoded2)
	}

	publishedResponse := invokeMetadata(t, invoker, principal, "metadata.version.publish", map[string]any{
		"metadata_version_id": version1.MetadataVersionID, "approval_id": "approval-metadata-1",
	})
	if publishedResponse.Status != capability.StatusSucceeded {
		t.Fatalf("publish response=%#v", publishedResponse)
	}
	var published Version
	if err := json.Unmarshal(publishedResponse.Result, &published); err != nil {
		t.Fatalf("decode published version: %v", err)
	}
	if published.Status != "published" || published.SnapshotDigest != digest1 || len(published.Snapshot) == 0 {
		t.Fatalf("published version=%#v", published)
	}
	missingApproval := invokeMetadata(t, invoker, principal, "metadata.version.publish", map[string]any{
		"metadata_version_id": version2.MetadataVersionID, "approval_id": "not-approved",
	})
	assertMetadataError(t, missingApproval, capability.CodeFailedPrecondition)

	immutable := invokeMetadata(t, invoker, principal, "metadata.object.upsert", map[string]any{
		"metadata_version_id": version1.MetadataVersionID, "object_id": customerID.String(), "api_name": "customer", "label": "Changed",
	})
	assertMetadataError(t, immutable, capability.CodeFailedPrecondition)
	duplicate := invokeMetadata(t, invoker, principal, "metadata.object.upsert", map[string]any{
		"metadata_version_id": version2.MetadataVersionID, "object_id": mustV7(t).String(), "api_name": "customer", "label": "Duplicate",
	})
	assertMetadataError(t, duplicate, capability.CodeConflict)

	version3 := createVersion(t, invoker, principal)
	crossVersion := invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": version3.MetadataVersionID, "field_id": mustV7(t).String(), "object_id": customerID.String(),
		"api_name": "name", "label": "Name", "data_type": "text",
	})
	assertMetadataError(t, crossVersion, capability.CodeConflict)

	insertMetadataTenant(t, admin, "22222222-2222-4222-8222-222222222222", "orgbbbbbbbbbbbbbbbbb", 42)
	tenantB := metadataPrincipal("22222222-2222-4222-8222-222222222222", "orgbbbbbbbbbbbbbbbbb")
	crossTenant := invokeMetadata(t, invoker, tenantB, "metadata.version.get", map[string]any{"metadata_version_id": version1.MetadataVersionID})
	assertMetadataError(t, crossTenant, capability.CodeResourceNotFound)

	assertMetadataAdapterParity(t, invoker, principal, version1.MetadataVersionID)
	assertMetadataSchema(t, admin)
}

func TestChangesetLifecyclePublicationRollbackAndAdapterParity(t *testing.T) {
	control, runtime := metadataTestPools(t)
	service := NewService(runtime, control)
	invoker := capability.NewInvoker(capability.NewRegistry(CapabilityDefinitions(service)), 4)
	principal := metadataPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa")
	principal.Actor.Scopes = append(principal.Actor.Scopes,
		"metadata.changeset.write", "metadata.changeset.read", "metadata.changeset.approve",
		"metadata.changeset.publish", "metadata.changeset.rollback",
	)

	objectID, nameFieldID := mustV7(t), mustV7(t)
	base := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, base.MetadataVersionID, objectID, "account", "Account")
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": base.MetadataVersionID, "field_id": nameFieldID.String(), "object_id": objectID.String(),
		"api_name": "name", "label": "Name", "data_type": "text",
	}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.version.publish", map[string]any{
		"metadata_version_id": base.MetadataVersionID, "approval_id": "approval-metadata-1",
	}))

	candidate := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, candidate.MetadataVersionID, objectID, "account", "Account")
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": candidate.MetadataVersionID, "field_id": nameFieldID.String(), "object_id": objectID.String(),
		"api_name": "name", "label": "Account name", "data_type": "text",
	}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": candidate.MetadataVersionID, "field_id": mustV7(t).String(), "object_id": objectID.String(),
		"api_name": "note", "label": "Note", "data_type": "text", "default_semantics": "on_create",
	}))
	validatedResponse := invokeMetadata(t, invoker, principal, "metadata.changeset.validate", map[string]any{
		"candidate_metadata_version_id": candidate.MetadataVersionID,
	})
	requireMetadataSuccess(t, validatedResponse)
	var changeset Changeset
	if err := json.Unmarshal(validatedResponse.Result, &changeset); err != nil {
		t.Fatal(err)
	}
	if changeset.State != "validated" || changeset.RequiresBackfill || changeset.OperationDigest == "" {
		t.Fatalf("validated changeset=%#v", changeset)
	}
	assertChangesetAdapterParity(t, invoker, principal, changeset.ChangesetID)

	approved := invokeMetadata(t, invoker, principal, "metadata.changeset.approve", map[string]any{
		"changeset_id": changeset.ChangesetID, "approval_id": "approval-metadata-1",
	})
	requireMetadataSuccess(t, approved)
	active := invokeMetadata(t, invoker, principal, "metadata.changeset.publish", map[string]any{"changeset_id": changeset.ChangesetID})
	requireMetadataSuccess(t, active)
	if err := json.Unmarshal(active.Result, &changeset); err != nil || changeset.State != "active" {
		t.Fatalf("active changeset=%#v err=%v", changeset, err)
	}

	bypass := createVersion(t, invoker, principal)
	direct := invokeMetadata(t, invoker, principal, "metadata.version.publish", map[string]any{
		"metadata_version_id": bypass.MetadataVersionID, "approval_id": "approval-metadata-1",
	})
	assertMetadataError(t, direct, capability.CodeFailedPrecondition)

	missingRollbackApproval := invokeMetadata(t, invoker, principal, "metadata.changeset.rollback", map[string]any{
		"changeset_id": changeset.ChangesetID, "approval_id": "unverified",
	})
	assertMetadataError(t, missingRollbackApproval, capability.CodeFailedPrecondition)
	rolledBack := invokeMetadata(t, invoker, principal, "metadata.changeset.rollback", map[string]any{
		"changeset_id": changeset.ChangesetID, "approval_id": "approval-metadata-1",
	})
	requireMetadataSuccess(t, rolledBack)
	if err := json.Unmarshal(rolledBack.Result, &changeset); err != nil || changeset.State != "rolled_back" {
		t.Fatalf("rolled back changeset=%#v err=%v", changeset, err)
	}

	var unsecured int
	if err := control.QueryRow(context.Background(),
		"select count(*) from pg_class where relname in ('metadata_changeset','metadata_changeset_object','record_unique_value','field_tombstone') and (not relrowsecurity or not relforcerowsecurity)",
	).Scan(&unsecured); err != nil || unsecured != 0 {
		t.Fatalf("unsecured changeset execution tables=%d err=%v", unsecured, err)
	}
}

func TestChangesetBlocksIndexedActivationUntilBackfillIsReady(t *testing.T) {
	control, runtime := metadataTestPools(t)
	service := NewService(runtime, control)
	invoker := capability.NewInvoker(capability.NewRegistry(CapabilityDefinitions(service)), 4)
	principal := metadataPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa")
	principal.Actor.Scopes = append(principal.Actor.Scopes,
		"metadata.changeset.write", "metadata.changeset.read", "metadata.changeset.approve", "metadata.changeset.publish", "metadata.changeset.execute",
	)

	objectID, nameFieldID := mustV7(t), mustV7(t)
	base := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, base.MetadataVersionID, objectID, "account", "Account")
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": base.MetadataVersionID, "field_id": nameFieldID.String(), "object_id": objectID.String(),
		"api_name": "name", "label": "Name", "data_type": "text",
	}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.version.publish", map[string]any{
		"metadata_version_id": base.MetadataVersionID, "approval_id": "approval-metadata-1",
	}))
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	recordID := mustV7(t)
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			"insert into object_record(tenant_bucket,tenant_id,metadata_version_id,object_id,record_id,lifecycle_state,data,revision,created_by,updated_by) values ($1,$2,$3,$4,$5,'active',$6,1,$7,$7)",
			tenant.Bucket, tenant.TenantID, base.MetadataVersionID, objectID, recordID, json.RawMessage(`{"name":"Acme"}`), principal.Actor.ID,
		)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	candidate := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, candidate.MetadataVersionID, objectID, "account", "Account")
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": candidate.MetadataVersionID, "field_id": nameFieldID.String(), "object_id": objectID.String(),
		"api_name": "name", "label": "Name", "data_type": "text",
	}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": candidate.MetadataVersionID, "field_id": mustV7(t).String(), "object_id": objectID.String(),
		"api_name": "score", "label": "Score", "data_type": "number", "indexed": true,
		"unique_value": true, "default_value": 0, "default_semantics": "backfill_required",
	}))
	validatedResponse := invokeMetadata(t, invoker, principal, "metadata.changeset.validate", map[string]any{
		"candidate_metadata_version_id": candidate.MetadataVersionID,
	})
	requireMetadataSuccess(t, validatedResponse)
	var changeset Changeset
	if err := json.Unmarshal(validatedResponse.Result, &changeset); err != nil {
		t.Fatal(err)
	}
	if !changeset.RequiresBackfill || changeset.RiskLevel != "high" {
		t.Fatalf("risky changeset was not classified for backfill: %#v", changeset)
	}
	var frozenPolicy governance.Policy
	if err := json.Unmarshal(changeset.QuotaSnapshot, &frozenPolicy); err != nil || frozenPolicy.PolicyVersion != 1 || frozenPolicy.MaxFieldsPerObject != 500 || frozenPolicy.MaxActiveIndexedFields != 20 {
		t.Fatalf("frozen quota policy=%#v err=%v", frozenPolicy, err)
	}
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.approve", map[string]any{
		"changeset_id": changeset.ChangesetID, "approval_id": "approval-metadata-1",
	}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.approve", map[string]any{
		"changeset_id": changeset.ChangesetID, "approval_id": "approval-metadata-1",
	}))
	blocked := invokeMetadata(t, invoker, principal, "metadata.changeset.publish", map[string]any{"changeset_id": changeset.ChangesetID})
	assertMetadataError(t, blocked, capability.CodeFailedPrecondition)

	var currentVersion, indexState string
	if err := control.QueryRow(context.Background(), "select metadata_version_id from tenant_registry where tenant_id=$1", tenant.TenantID).Scan(&currentVersion); err != nil {
		t.Fatal(err)
	}
	if currentVersion != base.MetadataVersionID {
		t.Fatalf("blocked changeset changed tenant pointer to %s", currentVersion)
	}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			"select index_state from field_definition where metadata_version_id=$1 and api_name='score'",
			candidate.MetadataVersionID,
		).Scan(&indexState)
	}); err != nil {
		t.Fatal(err)
	}
	if indexState != "building" {
		t.Fatalf("candidate index state=%s want building", indexState)
	}
	batchResponse := invokeMetadata(t, invoker, principal, "metadata.changeset.backfill", map[string]any{
		"changeset_id": changeset.ChangesetID, "batch_size": 1,
	})
	requireMetadataSuccess(t, batchResponse)
	var batch BatchResult
	if err := json.Unmarshal(batchResponse.Result, &batch); err != nil || batch.SucceededRecords != 1 || batch.RemainingRecords != 0 {
		t.Fatalf("batch=%#v err=%v", batch, err)
	}
	coverage := invokeMetadata(t, invoker, principal, "metadata.changeset.validate-coverage", map[string]any{"changeset_id": changeset.ChangesetID})
	requireMetadataSuccess(t, coverage)
	assertMetadataCapabilityAdapterParity(t, invoker, principal, "metadata.changeset.backfill", map[string]any{
		"changeset_id": changeset.ChangesetID, "batch_size": 1,
	})
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.publish", map[string]any{"changeset_id": changeset.ChangesetID}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.publish", map[string]any{"changeset_id": changeset.ChangesetID}))
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		var score string
		if err := tx.QueryRow(context.Background(), "select data->>'score' from object_record where object_id=$1 and record_id=$2", objectID, recordID).Scan(&score); err != nil {
			return err
		}
		if score != "0" {
			return fmt.Errorf("backfilled score=%s", score)
		}
		var typedRows, uniqueRows int
		if err := tx.QueryRow(context.Background(), "select count(*) from record_index_number where object_id=$1 and record_id=$2", objectID, recordID).Scan(&typedRows); err != nil {
			return err
		}
		if err := tx.QueryRow(context.Background(), "select count(*) from record_unique_value where object_id=$1 and record_id=$2", objectID, recordID).Scan(&uniqueRows); err != nil {
			return err
		}
		if typedRows != 1 || uniqueRows != 1 {
			return fmt.Errorf("typed rows=%d unique rows=%d", typedRows, uniqueRows)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestChangesetPredecessorConversionResumesAfterRevisionConflict(t *testing.T) {
	control, runtime := metadataTestPools(t)
	service := NewService(runtime, control)
	invoker := capability.NewInvoker(capability.NewRegistry(CapabilityDefinitions(service)), 4)
	principal := metadataPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa")
	principal.Actor.Scopes = append(principal.Actor.Scopes,
		"metadata.changeset.write", "metadata.changeset.read", "metadata.changeset.approve", "metadata.changeset.publish", "metadata.changeset.execute",
	)
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	objectID, legacyFieldID := mustV7(t), mustV7(t)
	base := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, base.MetadataVersionID, objectID, "invoice", "Invoice")
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": base.MetadataVersionID, "field_id": legacyFieldID.String(), "object_id": objectID.String(),
		"api_name": "amount_text", "label": "Amount text", "data_type": "text",
	}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.version.publish", map[string]any{
		"metadata_version_id": base.MetadataVersionID, "approval_id": "approval-metadata-1",
	}))
	recordID := mustV7(t)
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			"insert into object_record(tenant_bucket,tenant_id,metadata_version_id,object_id,record_id,lifecycle_state,data,revision,created_by,updated_by) values ($1,$2,$3,$4,$5,'active',$6,1,$7,$7)",
			tenant.Bucket, tenant.TenantID, base.MetadataVersionID, objectID, recordID, json.RawMessage(`{"amount_text":"42.50"}`), principal.Actor.ID,
		)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	unsafeCandidate := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, unsafeCandidate.MetadataVersionID, objectID, "invoice", "Invoice")
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": unsafeCandidate.MetadataVersionID, "field_id": legacyFieldID.String(), "object_id": objectID.String(),
		"api_name": "amount_text", "label": "Unsafe amount", "data_type": "number",
	}))
	unsafeValidation := invokeMetadata(t, invoker, principal, "metadata.changeset.validate", map[string]any{
		"candidate_metadata_version_id": unsafeCandidate.MetadataVersionID,
	})
	assertMetadataError(t, unsafeValidation, capability.CodeFailedPrecondition)

	candidate := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, candidate.MetadataVersionID, objectID, "invoice", "Invoice")
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": candidate.MetadataVersionID, "field_id": legacyFieldID.String(), "object_id": objectID.String(),
		"api_name": "amount_text", "label": "Amount text", "data_type": "text", "lifecycle_state": "deprecated_read_write",
	}))
	amountFieldID := mustV7(t)
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": candidate.MetadataVersionID, "field_id": amountFieldID.String(), "object_id": objectID.String(),
		"api_name": "amount", "label": "Amount", "data_type": "number", "required": true, "indexed": true,
		"predecessor_field_id": legacyFieldID.String(),
	}))
	validated := invokeMetadata(t, invoker, principal, "metadata.changeset.validate", map[string]any{"candidate_metadata_version_id": candidate.MetadataVersionID})
	requireMetadataSuccess(t, validated)
	var changeset Changeset
	if err := json.Unmarshal(validated.Result, &changeset); err != nil {
		t.Fatal(err)
	}
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.approve", map[string]any{
		"changeset_id": changeset.ChangesetID, "approval_id": "approval-metadata-1",
	}))

	locked, release, blockerDone := make(chan string, 1), make(chan struct{}), make(chan error, 1)
	go func() {
		blockerDone <- database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
			if _, err := tx.Exec(context.Background(), "update object_record set revision=revision+1 where object_id=$1 and record_id=$2", objectID, recordID); err != nil {
				return err
			}
			var transactionID string
			if err := tx.QueryRow(context.Background(), "select txid_current()::text").Scan(&transactionID); err != nil {
				return err
			}
			locked <- transactionID
			<-release
			return nil
		})
	}()
	blockingTransactionID := <-locked
	batchRequest := boundRequest(t, principal, "metadata.changeset.backfill", map[string]any{
		"changeset_id": changeset.ChangesetID, "batch_size": 1,
	})
	batchDone := make(chan capability.Response, 1)
	go func() {
		batchDone <- invoker.Invoke(context.Background(), batchRequest)
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var waiting bool
		if err := control.QueryRow(context.Background(),
			"select exists(select 1 from pg_locks where locktype='transactionid' and transactionid::text=$1 and not granted)",
			blockingTransactionID,
		).Scan(&waiting); err != nil {
			close(release)
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case response := <-batchDone:
			close(release)
			t.Fatalf("backfill did not wait on the concurrent record update: %#v", response)
		default:
		}
		if time.Now().After(deadline) {
			close(release)
			t.Fatal("timed out waiting for backfill revision conflict")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	if err := <-blockerDone; err != nil {
		t.Fatal(err)
	}
	conflictedResponse := <-batchDone
	requireMetadataSuccess(t, conflictedResponse)
	var conflicted BatchResult
	if err := json.Unmarshal(conflictedResponse.Result, &conflicted); err != nil || conflicted.ConflictRecords != 1 || conflicted.SucceededRecords != 0 || conflicted.RemainingRecords != 1 {
		t.Fatalf("conflicted batch=%#v err=%v", conflicted, err)
	}

	retryResponse := invokeMetadata(t, invoker, principal, "metadata.changeset.backfill", map[string]any{
		"changeset_id": changeset.ChangesetID, "batch_size": 1,
	})
	requireMetadataSuccess(t, retryResponse)
	var retried BatchResult
	if err := json.Unmarshal(retryResponse.Result, &retried); err != nil || retried.SucceededRecords != 1 || retried.RemainingRecords != 0 {
		t.Fatalf("retried batch=%#v err=%v", retried, err)
	}
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.validate-coverage", map[string]any{"changeset_id": changeset.ChangesetID}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.publish", map[string]any{"changeset_id": changeset.ChangesetID}))
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		var amount string
		if err := tx.QueryRow(context.Background(), "select data->>'amount' from object_record where record_id=$1", recordID).Scan(&amount); err != nil {
			return err
		}
		if amount != "42.50" {
			return fmt.Errorf("converted amount=%s", amount)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestChangesetUniqueCollisionBlocksCoverageUntilCorrected(t *testing.T) {
	control, runtime := metadataTestPools(t)
	service := NewService(runtime, control)
	invoker := capability.NewInvoker(capability.NewRegistry(CapabilityDefinitions(service)), 4)
	principal := metadataPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa")
	principal.Actor.Scopes = append(principal.Actor.Scopes,
		"metadata.changeset.write", "metadata.changeset.read", "metadata.changeset.approve", "metadata.changeset.publish", "metadata.changeset.execute",
	)
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	objectID, codeFieldID := mustV7(t), mustV7(t)
	base := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, base.MetadataVersionID, objectID, "asset", "Asset")
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": base.MetadataVersionID, "field_id": codeFieldID.String(), "object_id": objectID.String(),
		"api_name": "code", "label": "Code", "data_type": "text", "indexed": true,
	}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.version.publish", map[string]any{
		"metadata_version_id": base.MetadataVersionID, "approval_id": "approval-metadata-1",
	}))
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		for i := 0; i < 2; i++ {
			if _, err := tx.Exec(context.Background(),
				"insert into object_record(tenant_bucket,tenant_id,metadata_version_id,object_id,record_id,lifecycle_state,data,revision,created_by,updated_by) values ($1,$2,$3,$4,$5,'active',$6,1,$7,$7)",
				tenant.Bucket, tenant.TenantID, base.MetadataVersionID, objectID, mustV7(t), json.RawMessage(`{"code":"DUPLICATE"}`), principal.Actor.ID,
			); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	candidate := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, candidate.MetadataVersionID, objectID, "asset", "Asset")
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": candidate.MetadataVersionID, "field_id": codeFieldID.String(), "object_id": objectID.String(),
		"api_name": "code", "label": "Code", "data_type": "text", "indexed": true, "unique_value": true,
	}))
	validated := invokeMetadata(t, invoker, principal, "metadata.changeset.validate", map[string]any{"candidate_metadata_version_id": candidate.MetadataVersionID})
	requireMetadataSuccess(t, validated)
	var changeset Changeset
	if err := json.Unmarshal(validated.Result, &changeset); err != nil {
		t.Fatal(err)
	}
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.approve", map[string]any{
		"changeset_id": changeset.ChangesetID, "approval_id": "approval-metadata-1",
	}))
	batchResponse := invokeMetadata(t, invoker, principal, "metadata.changeset.backfill", map[string]any{
		"changeset_id": changeset.ChangesetID, "batch_size": 10,
	})
	requireMetadataSuccess(t, batchResponse)
	var batch BatchResult
	if err := json.Unmarshal(batchResponse.Result, &batch); err != nil || batch.SucceededRecords != 1 || batch.FailedRecords != 1 || batch.RemainingRecords != 1 {
		t.Fatalf("unique collision batch=%#v err=%v", batch, err)
	}
	incomplete := invokeMetadata(t, invoker, principal, "metadata.changeset.validate-coverage", map[string]any{"changeset_id": changeset.ChangesetID})
	assertMetadataError(t, incomplete, capability.CodeFailedPrecondition)
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			"update object_record set data=jsonb_set(data,'{code}',$2::jsonb),revision=revision+1 where object_id=$1 and metadata_version_id is distinct from $3",
			objectID, json.RawMessage(`"UNIQUE"`), candidate.MetadataVersionID,
		)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.backfill", map[string]any{
		"changeset_id": changeset.ChangesetID, "batch_size": 10,
	}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.validate-coverage", map[string]any{"changeset_id": changeset.ChangesetID}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.publish", map[string]any{"changeset_id": changeset.ChangesetID}))
}

func TestDestructivePurgeTombstoneAndNameReservation(t *testing.T) {
	control, runtime := metadataTestPools(t)
	service := NewService(runtime, control)
	invoker := capability.NewInvoker(capability.NewRegistry(CapabilityDefinitions(service)), 4)
	principal := metadataPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa")
	principal.Actor.Scopes = append(principal.Actor.Scopes,
		"metadata.changeset.write", "metadata.changeset.read", "metadata.changeset.approve", "metadata.changeset.publish",
		"metadata.changeset.execute", "metadata.changeset.purge", "metadata.changeset.rollback",
	)
	objectID, fieldID := mustV7(t), mustV7(t)
	base := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, base.MetadataVersionID, objectID, "account", "Account")
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": base.MetadataVersionID, "field_id": fieldID.String(), "object_id": objectID.String(),
		"api_name": "legacy_code", "label": "Legacy code", "data_type": "text", "indexed": true,
	}))
	requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.version.publish", map[string]any{
		"metadata_version_id": base.MetadataVersionID, "approval_id": "approval-metadata-1",
	}))
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	recordID := mustV7(t)
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			"insert into object_record(tenant_bucket,tenant_id,metadata_version_id,object_id,record_id,lifecycle_state,data,revision,created_by,updated_by) values ($1,$2,$3,$4,$5,'active',$6,1,$7,$7)",
			tenant.Bucket, tenant.TenantID, base.MetadataVersionID, objectID, recordID, json.RawMessage(`{"legacy_code":"OLD"}`), principal.Actor.ID,
		)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	var tombstoneChangesetID string
	publishLifecycle := func(state string, indexed bool, destructive bool) Version {
		t.Helper()
		candidate := createVersion(t, invoker, principal)
		upsertObject(t, invoker, principal, candidate.MetadataVersionID, objectID, "account", "Account")
		requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
			"metadata_version_id": candidate.MetadataVersionID, "field_id": fieldID.String(), "object_id": objectID.String(),
			"api_name": "legacy_code", "label": "Legacy code", "data_type": "text", "indexed": indexed, "lifecycle_state": state,
		}))
		validated := invokeMetadata(t, invoker, principal, "metadata.changeset.validate", map[string]any{"candidate_metadata_version_id": candidate.MetadataVersionID})
		requireMetadataSuccess(t, validated)
		var changeset Changeset
		if err := json.Unmarshal(validated.Result, &changeset); err != nil {
			t.Fatal(err)
		}
		if state == "tombstone" {
			tombstoneChangesetID = changeset.ChangesetID
		}
		requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.approve", map[string]any{
			"changeset_id": changeset.ChangesetID, "approval_id": "approval-metadata-1",
		}))
		if destructive {
			blocked := invokeMetadata(t, invoker, principal, "metadata.changeset.backfill", map[string]any{"changeset_id": changeset.ChangesetID})
			assertMetadataError(t, blocked, capability.CodeFailedPrecondition)
			requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.purge", map[string]any{
				"changeset_id": changeset.ChangesetID, "batch_size": 10, "approval_id": "approval-metadata-1",
			}))
			requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.validate-coverage", map[string]any{"changeset_id": changeset.ChangesetID}))
		}
		requireMetadataSuccess(t, invokeMetadata(t, invoker, principal, "metadata.changeset.publish", map[string]any{"changeset_id": changeset.ChangesetID}))
		return candidate
	}

	publishLifecycle("deprecated_read_write", true, false)
	publishLifecycle("deprecated_read_only", true, false)
	publishLifecycle("hidden", true, false)
	publishLifecycle("purging", false, true)
	tombstoneVersion := publishLifecycle("tombstone", false, true)
	destructiveRollback := invokeMetadata(t, invoker, principal, "metadata.changeset.rollback", map[string]any{
		"changeset_id": tombstoneChangesetID, "approval_id": "approval-metadata-1",
	})
	assertMetadataError(t, destructiveRollback, capability.CodeFailedPrecondition)
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		var keyPresent bool
		if err := tx.QueryRow(context.Background(), "select data ? 'legacy_code' from object_record where record_id=$1", recordID).Scan(&keyPresent); err != nil {
			return err
		}
		if keyPresent {
			return fmt.Errorf("purged key still exists")
		}
		var tombstones int
		if err := tx.QueryRow(context.Background(), "select count(*) from field_tombstone where object_id=$1 and api_name='legacy_code' and metadata_version_id=$2", objectID, tombstoneVersion.MetadataVersionID).Scan(&tombstones); err != nil {
			return err
		}
		if tombstones != 1 {
			return fmt.Errorf("tombstones=%d", tombstones)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	reservedCandidate := createVersion(t, invoker, principal)
	upsertObject(t, invoker, principal, reservedCandidate.MetadataVersionID, objectID, "account", "Account")
	reserved := invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": reservedCandidate.MetadataVersionID, "field_id": mustV7(t).String(), "object_id": objectID.String(),
		"api_name": "legacy_code", "label": "Reused name", "data_type": "text",
	})
	assertMetadataError(t, reserved, capability.CodeFailedPrecondition)
}

func createVersion(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal) Version {
	t.Helper()
	response := invokeMetadata(t, invoker, principal, "metadata.version.create", map[string]any{})
	if response.Status != capability.StatusSucceeded {
		t.Fatalf("create version response=%#v", response)
	}
	var version Version
	if err := json.Unmarshal(response.Result, &version); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	return version
}

func upsertObject(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, version string, objectID uuid.UUID, apiName, label string) {
	t.Helper()
	response := invokeMetadata(t, invoker, principal, "metadata.object.upsert", map[string]any{
		"metadata_version_id": version, "object_id": objectID.String(), "api_name": apiName, "label": label,
		"semantic": map[string]any{"purpose": apiName},
	})
	if response.Status != capability.StatusSucceeded {
		t.Fatalf("upsert object response=%#v", response)
	}
}

func upsertField(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, version string, fieldID, objectID uuid.UUID, apiName string) {
	t.Helper()
	response := invokeMetadata(t, invoker, principal, "metadata.field.upsert", map[string]any{
		"metadata_version_id": version, "field_id": fieldID.String(), "object_id": objectID.String(),
		"api_name": apiName, "label": "Name", "data_type": "text", "required": true, "indexed": true,
		"constraints": map[string]any{"max_length": 200},
	})
	if response.Status != capability.StatusSucceeded {
		t.Fatalf("upsert field response=%#v", response)
	}
}

func upsertRelation(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, version string, relationID uuid.UUID, apiName string, sourceID, targetID uuid.UUID) {
	t.Helper()
	response := invokeMetadata(t, invoker, principal, "metadata.relation.upsert", map[string]any{
		"metadata_version_id": version, "relation_id": relationID.String(), "api_name": apiName,
		"source_object_id": sourceID.String(), "target_object_id": targetID.String(),
		"relation_type": "lookup", "delete_behavior": "restrict",
	})
	if response.Status != capability.StatusSucceeded {
		t.Fatalf("upsert relation response=%#v", response)
	}
}

func invokeMetadata(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, capabilityID string, input any) capability.Response {
	t.Helper()
	return invoker.Invoke(context.Background(), boundRequest(t, principal, capabilityID, input))
}

func boundRequest(t *testing.T, principal capability.TrustedPrincipal, capabilityID string, input any) capability.Request {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	request, stableErr := capability.BindTrustedPrincipal(capability.Request{
		CapabilityID: capabilityID, RequestID: "req-" + capabilityID + "-" + time.Now().Format("150405.000000000"), Input: raw,
	}, principal)
	if stableErr != nil {
		t.Fatalf("bind principal: %v", stableErr)
	}
	return request
}

func metadataPrincipal(tenantID, companyID string) capability.TrustedPrincipal {
	return capability.TrustedPrincipal{
		TenantID: tenantID, CompanyID: companyID, Source: "test-jwt", Approvals: []string{"approval-metadata-1"},
		Actor: capability.Actor{ID: "metadata-agent", Scopes: []string{
			"metadata.version.write", "metadata.definition.write", "metadata.publish", "metadata.read",
		}},
	}
}

func metadataTestPools(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
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
		t.Fatalf("apply migrations: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "alter role ai_native_runtime login"); err != nil {
		t.Fatalf("enable local runtime login: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "alter role ai_native_control login"); err != nil {
		t.Fatalf("enable local control login: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "truncate tenant_registry cascade"); err != nil {
		t.Fatalf("reset metadata data: %v", err)
	}
	controlConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse control pool: %v", err)
	}
	controlConfig.ConnConfig.User = "ai_native_control"
	controlConfig.MaxConns = 2
	control, err := pgxpool.NewWithConfig(context.Background(), controlConfig)
	if err != nil {
		t.Fatalf("open control pool: %v", err)
	}
	if err := control.Ping(context.Background()); err != nil {
		t.Fatalf("ping control pool: %v", err)
	}
	insertMetadataTenant(t, control, "11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa", 7)
	runtimeConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse runtime pool: %v", err)
	}
	runtimeConfig.ConnConfig.User = "ai_native_runtime"
	runtimeConfig.MaxConns = 2
	runtime, err := pgxpool.NewWithConfig(context.Background(), runtimeConfig)
	if err != nil {
		t.Fatalf("open runtime pool: %v", err)
	}
	if err := runtime.Ping(context.Background()); err != nil {
		t.Fatalf("ping runtime pool: %v", err)
	}
	t.Cleanup(func() {
		runtime.Close()
		control.Close()
		_, _ = lock.Exec(context.Background(), "select pg_advisory_unlock(7167614658367249410)")
		lock.Release()
		admin.Close()
	})
	return control, runtime
}

func insertMetadataTenant(t *testing.T, admin *pgxpool.Pool, tenantID, companyID string, bucket int16) {
	t.Helper()
	_, err := admin.Exec(context.Background(),
		"insert into tenant_registry(tenant_id,company_id,display_name,shard_id,tenant_bucket,service_tier,global_lifecycle_status,native_status,tenant_revision,product_revision,route_revision) values ($1,$2,$3,'shard-001',$4,'standard','active','active',1,1,1)",
		tenantID, companyID, "Tenant "+companyID, bucket)
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
}

func assertMetadataAdapterParity(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, versionID string) {
	t.Helper()
	request := capability.Request{CapabilityID: "metadata.version.get", RequestID: "req-metadata-parity"}
	request.Input, _ = json.Marshal(map[string]any{"metadata_version_id": versionID})
	body, _ := json.Marshal(request)
	recorder := httptest.NewRecorder()
	apiRequest := httptest.NewRequest(http.MethodPost, "/v1/capabilities/metadata.version.get/invoke", bytes.NewReader(body))
	apiRequest.Header.Set("Authorization", "Bearer test")
	api.NewAuthenticatedHandler(invoker, metadataVerifier{principal}).ServeHTTP(recorder, apiRequest)
	var apiResponse capability.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &apiResponse); err != nil || recorder.Code != http.StatusOK {
		t.Fatalf("API response=%s err=%v", recorder.Body.String(), err)
	}
	var stdout bytes.Buffer
	if exit := cli.RunAs(context.Background(), invoker, principal, []string{"capability", "invoke", "--id", request.CapabilityID}, bytes.NewReader(body), &stdout, io.Discard); exit != 0 {
		t.Fatalf("CLI exit=%d output=%s", exit, stdout.String())
	}
	var cliResponse capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &cliResponse); err != nil {
		t.Fatalf("decode CLI: %v", err)
	}
	mcpResponse := metadataMCP(t, invoker, principal, request, map[string]any{"metadata_version_id": versionID})
	if !sameJSON(apiResponse.Result, cliResponse.Result) || !sameJSON(apiResponse.Result, mcpResponse.Result) {
		t.Fatalf("metadata adapter drift: API=%s CLI=%s MCP=%s", apiResponse.Result, cliResponse.Result, mcpResponse.Result)
	}
}

func assertChangesetAdapterParity(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, changesetID string) {
	t.Helper()
	assertMetadataCapabilityAdapterParity(t, invoker, principal, "metadata.changeset.get-status", map[string]any{"changeset_id": changesetID})
}

func assertMetadataCapabilityAdapterParity(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, capabilityID string, input map[string]any) {
	t.Helper()
	request := capability.Request{CapabilityID: capabilityID, RequestID: "req-parity-" + capability.MCPToolName(capabilityID)}
	request.Input, _ = json.Marshal(input)
	body, _ := json.Marshal(request)
	recorder := httptest.NewRecorder()
	apiRequest := httptest.NewRequest(http.MethodPost, "/v1/capabilities/"+capabilityID+"/invoke", bytes.NewReader(body))
	apiRequest.Header.Set("Authorization", "Bearer test")
	api.NewAuthenticatedHandler(invoker, metadataVerifier{principal}).ServeHTTP(recorder, apiRequest)
	var apiResponse capability.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &apiResponse); err != nil || recorder.Code != http.StatusOK {
		t.Fatalf("%s API response=%s err=%v", capabilityID, recorder.Body.String(), err)
	}
	var stdout bytes.Buffer
	if exit := cli.RunAs(context.Background(), invoker, principal, []string{"capability", "invoke", "--id", request.CapabilityID}, bytes.NewReader(body), &stdout, io.Discard); exit != 0 {
		t.Fatalf("%s CLI exit=%d output=%s", capabilityID, exit, stdout.String())
	}
	var cliResponse capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &cliResponse); err != nil {
		t.Fatal(err)
	}
	mcpResponse := metadataMCP(t, invoker, principal, request, input)
	if !sameJSON(apiResponse.Result, cliResponse.Result) || !sameJSON(apiResponse.Result, mcpResponse.Result) {
		t.Fatalf("%s adapter drift: API=%s CLI=%s MCP=%s", capabilityID, apiResponse.Result, cliResponse.Result, mcpResponse.Result)
	}
}

type metadataVerifier struct{ principal capability.TrustedPrincipal }

func (verifier metadataVerifier) Verify(context.Context, string) (capability.TrustedPrincipal, error) {
	return verifier.principal, nil
}

func metadataMCP(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, request capability.Request, input map[string]any) capability.Response {
	t.Helper()
	ctx := context.Background()
	server := mcpserver.NewServerAs(invoker, principal)
	client := mcp.NewClient(&mcp.Implementation{Name: "metadata-parity", Version: "v1"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      capability.MCPToolName(request.CapabilityID),
		Arguments: map[string]any{"request_id": request.RequestID, "input": input},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	var response capability.Response
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func assertMetadataSchema(t *testing.T, admin *pgxpool.Pool) {
	t.Helper()
	var unsecured int
	if err := admin.QueryRow(context.Background(),
		"select count(*) from pg_class where relname in ('metadata_version','object_definition','field_definition','relation_definition') and (not relrowsecurity or not relforcerowsecurity)",
	).Scan(&unsecured); err != nil || unsecured != 0 {
		t.Fatalf("unsecured metadata tables=%d err=%v", unsecured, err)
	}
}

func assertMetadataError(t *testing.T, response capability.Response, code capability.ErrorCode) {
	t.Helper()
	if response.Status != capability.StatusFailed || response.Error == nil || response.Error.Code != code {
		t.Fatalf("response=%#v, want %s", response, code)
	}
}

func requireMetadataSuccess(t *testing.T, response capability.Response) {
	t.Helper()
	if response.Status != capability.StatusSucceeded {
		t.Fatalf("response=%#v", response)
	}
}

func sameJSON(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	return json.Unmarshal(left, &leftValue) == nil && json.Unmarshal(right, &rightValue) == nil && reflect.DeepEqual(leftValue, rightValue)
}

func mustV7(t *testing.T) uuid.UUID {
	t.Helper()
	value, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("UUIDv7: %v", err)
	}
	return value
}
