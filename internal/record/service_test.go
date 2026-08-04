package record

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
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/api"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/cli"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/database/migrate"
	mcpserver "github.com/OlivierZEN/ai-native-platform/internal/mcp"
	"github.com/OlivierZEN/ai-native-platform/internal/metadata"
	"github.com/OlivierZEN/ai-native-platform/internal/metering"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var requestSequence atomic.Int64

func TestRecordUpdateToleratesMissingHistoricalMeteringBaseline(t *testing.T) {
	_, control, runtime := recordTestPools(t)
	metadataService := metadata.NewService(runtime, control)
	meter := metering.NewService(runtime, control)
	recordService := NewService(runtime, control, meter)
	definitions := append(metadata.CapabilityDefinitions(metadataService), CapabilityDefinitions(recordService)...)
	invoker := capability.NewInvoker(capability.NewRegistry(definitions), 8)
	principal := recordPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa", "record-meter-agent")
	model := publishRecordModel(t, invoker, principal)
	created := requireRecord(t, invokeRecord(t, invoker, principal, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "a deliberately long historical record name"},
	}))
	recordID := uuid.MustParse(created.RecordID)
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `
			update metering.tenant_usage_current_bucket
			set live_record_count=0,logical_data_bytes=0
			where object_id=$1 and counter_bucket=$2`, model.CustomerID, int16(recordID[15]%16))
		return err
	}); err != nil {
		t.Fatal(err)
	}

	requireSuccess(t, invokeRecord(t, invoker, principal, "runtime.record.update", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID, "expected_revision": created.Revision,
		"patch": map[string]any{"name": "short"},
	}))
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		var records, bytes int64
		if err := tx.QueryRow(context.Background(), `
			select live_record_count,logical_data_bytes
			from metering.tenant_usage_current_bucket
			where object_id=$1 and counter_bucket=$2`, model.CustomerID, int16(recordID[15]%16)).Scan(&records, &bytes); err != nil {
			return err
		}
		if records != 0 || bytes != 0 {
			return fmt.Errorf("bounded usage = (%d,%d), want (0,0)", records, bytes)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRecordRuntimeCRUDQueryRelationsIsolationAndParity(t *testing.T) {
	admin, control, runtime := recordTestPools(t)
	metadataService := metadata.NewService(runtime, control)
	recordService := NewService(runtime, control)
	definitions := append(metadata.CapabilityDefinitions(metadataService), CapabilityDefinitions(recordService)...)
	invoker := capability.NewInvoker(capability.NewRegistry(definitions), 8)
	principalA := recordPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa", "record-agent-a")
	modelA := publishRecordModel(t, invoker, principalA)
	draftResponse := invokeRecord(t, invoker, principalA, "metadata.version.create", map[string]any{})
	requireSuccess(t, draftResponse)
	var draft metadata.Version
	if err := json.Unmarshal(draftResponse.Result, &draft); err != nil {
		t.Fatal(err)
	}
	requireSuccess(t, invokeRecord(t, invoker, principalA, "metadata.object.upsert", map[string]any{
		"metadata_version_id": draft.MetadataVersionID, "object_id": mustRecordV7(t).String(), "api_name": "draft_only", "label": "Draft only",
	}))
	unpublishedObject := invokeRecord(t, invoker, principalA, "runtime.record.create", map[string]any{
		"object_api_name": "draft_only", "data": map[string]any{},
	})
	assertRecordError(t, unpublishedObject, capability.CodeResourceNotFound)
	invalidObjectName := invokeRecord(t, invoker, principalA, "runtime.record.query", map[string]any{"object_api_name": "Invalid Object"})
	assertRecordError(t, invalidObjectName, capability.CodeValidationFailed)
	idempotentInput := map[string]any{"object_api_name": "customer", "data": map[string]any{"name": "Idempotent"}}
	idempotentFirst := requireRecord(t, invokeRecordWithKey(t, invoker, principalA, "runtime.record.create", "record-create-restart-key", idempotentInput))
	restartedInvoker := capability.NewInvoker(capability.NewRegistry(definitions), 8)
	idempotentReplay := requireRecord(t, invokeRecordWithKey(t, restartedInvoker, principalA, "runtime.record.create", "record-create-restart-key", idempotentInput))
	if idempotentFirst.RecordID != idempotentReplay.RecordID || idempotentFirst.Revision != idempotentReplay.Revision {
		t.Fatalf("durable replay drift first=%#v replay=%#v", idempotentFirst, idempotentReplay)
	}
	idempotencyConflict := invokeRecordWithKey(t, restartedInvoker, principalA, "runtime.record.create", "record-create-restart-key", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Different"},
	})
	assertRecordError(t, idempotencyConflict, capability.CodeIdempotencyConflict)
	assertAllRecordAdapterParity(t, invoker, principalA)

	badUnknown := invokeRecord(t, invoker, principalA, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Invalid", "unknown": true},
	})
	assertRecordError(t, badUnknown, capability.CodeValidationFailed)
	assertAdapterErrorsEqual(t, invoker, principalA, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Invalid", "unknown": true},
	}, capability.CodeValidationFailed)
	badType := invokeRecord(t, invoker, principalA, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Invalid", "amount": "large"},
	})
	assertRecordError(t, badType, capability.CodeValidationFailed)
	assertAdapterErrorsEqual(t, invoker, principalA, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Oversized", "note": string(bytes.Repeat([]byte("x"), 262145))},
	}, capability.CodeValidationFailed)
	nonV7 := invokeRecord(t, invoker, principalA, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "record_id": "11111111-1111-4111-8111-111111111111", "data": map[string]any{"name": "Invalid"},
	})
	assertRecordError(t, nonV7, capability.CodeValidationFailed)

	externalID := mustRecordV7(t)
	created := requireRecord(t, invokeRecord(t, invoker, principalA, "runtime.record.create", map[string]any{
		"object_api_name": "customer",
		"data": map[string]any{
			"name": "Acme", "active": true, "opened": "2026-07-19", "external_id": externalID.String(), "note": "unindexed",
		},
	}))
	if parsed := uuid.MustParse(created.RecordID); parsed.Version() != 7 || created.Revision != 1 || created.LifecycleState != "active" {
		t.Fatalf("created record=%#v", created)
	}
	if amount, ok := created.Data["amount"].(float64); !ok || amount != 0 {
		t.Fatalf("default amount=%#v", created.Data["amount"])
	}
	assertTypedRows(t, admin, created.RecordID, map[string]int{
		"record_index_text": 1, "record_index_number": 1, "record_index_boolean": 1, "record_index_datetime": 1, "record_index_uuid": 1,
	})

	got := requireRecord(t, invokeRecord(t, invoker, principalA, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}))
	if !sameRecord(created, got) {
		t.Fatalf("get drift: created=%#v got=%#v", created, got)
	}
	publishRecordModelWithIDs(t, invoker, principalA, modelA)
	gotAfterMetadataUpgrade := requireRecord(t, invokeRecord(t, invoker, principalA, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}))
	expectedAfterMetadataUpgrade := created
	expectedAfterMetadataUpgrade.MetadataVersionID = gotAfterMetadataUpgrade.MetadataVersionID
	if gotAfterMetadataUpgrade.MetadataVersionID == created.MetadataVersionID || !sameRecord(expectedAfterMetadataUpgrade, gotAfterMetadataUpgrade) {
		t.Fatalf("metadata publication hid or changed old record: before=%#v after=%#v", created, gotAfterMetadataUpgrade)
	}
	oldVersionQuery := requireQuery(t, invokeRecord(t, invoker, principalA, "runtime.record.query", map[string]any{
		"object_api_name": "customer", "filters": []map[string]any{{"field": "name", "op": "eq", "value": "Idempotent"}}, "limit": 10,
	}))
	if len(oldVersionQuery.Records) != 1 || oldVersionQuery.Records[0].RecordID != idempotentFirst.RecordID {
		t.Fatalf("metadata publication hid old typed index row: %#v", oldVersionQuery)
	}
	assertRecordAdapterParity(t, invoker, principalA, created.RecordID)

	updated := requireRecord(t, invokeRecord(t, invoker, principalA, "runtime.record.update", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID, "expected_revision": 1,
		"patch": map[string]any{"amount": 125.5, "note": nil},
	}))
	if updated.Revision != 2 || updated.Data["note"] != nil || updated.MetadataVersionID == created.MetadataVersionID {
		t.Fatalf("updated record=%#v", updated)
	}
	stale := invokeRecord(t, invoker, principalA, "runtime.record.update", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID, "expected_revision": 1, "patch": map[string]any{"name": "stale"},
	})
	assertRecordError(t, stale, capability.CodeConflict)

	query := requireQuery(t, invokeRecord(t, invoker, principalA, "runtime.record.query", map[string]any{
		"object_api_name": "customer", "filters": []map[string]any{{"field": "amount", "op": "gte", "value": 100}}, "limit": 10,
	}))
	if len(query.Records) != 1 || query.Records[0].RecordID != created.RecordID || query.Plan.Strategy != "typed_index" {
		t.Fatalf("query result=%#v", query)
	}
	unindexed := invokeRecord(t, invoker, principalA, "runtime.record.query", map[string]any{
		"object_api_name": "customer", "filters": []map[string]any{{"field": "note", "op": "eq", "value": "unindexed"}},
	})
	assertRecordError(t, unindexed, capability.CodeFailedPrecondition)
	prefixQuery := requireQuery(t, invokeRecord(t, invoker, principalA, "runtime.record.query", map[string]any{
		"object_api_name": "customer", "filters": []map[string]any{{"field": "name", "op": "prefix", "value": "Ac"}}, "limit": 10,
	}))
	if len(prefixQuery.Records) != 1 || prefixQuery.Records[0].RecordID != created.RecordID {
		t.Fatalf("prefix query=%#v", prefixQuery)
	}
	pageOne := requireQuery(t, invokeRecord(t, invoker, principalA, "runtime.record.query", map[string]any{"object_api_name": "customer", "limit": 1}))
	if len(pageOne.Records) != 1 || pageOne.NextCursor == "" {
		t.Fatalf("first page=%#v", pageOne)
	}
	pageTwo := requireQuery(t, invokeRecord(t, invoker, principalA, "runtime.record.query", map[string]any{
		"object_api_name": "customer", "limit": 1, "after": pageOne.NextCursor,
	}))
	if len(pageTwo.Records) != 1 || pageTwo.Records[0].RecordID == pageOne.Records[0].RecordID {
		t.Fatalf("second page=%#v first=%#v", pageTwo, pageOne)
	}

	contact := requireRecord(t, invokeRecord(t, invoker, principalA, "runtime.record.create", map[string]any{
		"object_api_name": "contact", "data": map[string]any{"name": "Alice", "customer": created.RecordID},
	}))
	var relationCount int
	if err := admin.QueryRow(context.Background(), "select count(*) from record_relation where source_record_id=$1 and target_record_id=$2", contact.RecordID, created.RecordID).Scan(&relationCount); err != nil || relationCount != 1 {
		t.Fatalf("relation count=%d err=%v", relationCount, err)
	}
	referencedDelete := invokeRecord(t, invoker, principalA, "runtime.record.delete", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID, "expected_revision": 2,
	})
	assertRecordError(t, referencedDelete, capability.CodeFailedPrecondition)

	insertRecordTenant(t, control, "22222222-2222-4222-8222-222222222222", "orgbbbbbbbbbbbbbbbbb", 42)
	principalB := recordPrincipal("22222222-2222-4222-8222-222222222222", "orgbbbbbbbbbbbbbbbbb", "record-agent-b")
	publishRecordModelWithIDs(t, invoker, principalB, modelA)
	crossTenant := invokeRecord(t, invoker, principalB, "runtime.record.get", map[string]any{"object_api_name": "customer", "record_id": created.RecordID})
	assertRecordError(t, crossTenant, capability.CodeResourceNotFound)

	deletedContact := requireRecord(t, invokeRecord(t, invoker, principalA, "runtime.record.delete", map[string]any{
		"object_api_name": "contact", "record_id": contact.RecordID, "expected_revision": 1,
	}))
	if deletedContact.LifecycleState != "deleted" || deletedContact.Revision != 2 || deletedContact.DeletedAt == nil {
		t.Fatalf("deleted contact=%#v", deletedContact)
	}
	hidden := invokeRecord(t, invoker, principalA, "runtime.record.get", map[string]any{"object_api_name": "contact", "record_id": contact.RecordID})
	assertRecordError(t, hidden, capability.CodeResourceNotFound)
	visibleDeleted := requireRecord(t, invokeRecord(t, invoker, principalA, "runtime.record.get", map[string]any{
		"object_api_name": "contact", "record_id": contact.RecordID, "include_deleted": true,
	}))
	if visibleDeleted.LifecycleState != "deleted" {
		t.Fatalf("include deleted=%#v", visibleDeleted)
	}

	deletedCustomer := requireRecord(t, invokeRecord(t, invoker, principalA, "runtime.record.delete", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID, "expected_revision": 2,
	}))
	if deletedCustomer.LifecycleState != "deleted" || deletedCustomer.Revision != 3 {
		t.Fatalf("deleted customer=%#v", deletedCustomer)
	}
	assertTypedRows(t, admin, created.RecordID, map[string]int{
		"record_index_text": 0, "record_index_number": 0, "record_index_boolean": 0, "record_index_datetime": 0, "record_index_uuid": 0,
	})
	assertRecordSchema(t, admin, runtime)
}

func TestRecordWritesProjectToApprovedCandidateDuringBackfill(t *testing.T) {
	_, control, runtime := recordTestPools(t)
	metadataService := metadata.NewService(runtime, control)
	recordService := NewService(runtime, control)
	invoker := capability.NewInvoker(capability.NewRegistry(append(metadata.CapabilityDefinitions(metadataService), CapabilityDefinitions(recordService)...)), 8)
	principal := recordPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa", "evolution-agent")
	ids := publishRecordModel(t, invoker, principal)
	legacy := requireRecord(t, invokeRecord(t, invoker, principal, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Before changeset"},
	}))
	legacyContact := requireRecord(t, invokeRecord(t, invoker, principal, "runtime.record.create", map[string]any{
		"object_api_name": "contact", "data": map[string]any{"name": "Before contact", "customer": legacy.RecordID},
	}))

	segmentID := mustRecordV7(t)
	candidate := buildRecordModelVersion(t, invoker, principal, ids, []map[string]any{{
		"field_id": segmentID.String(), "object_id": ids.CustomerID.String(), "api_name": "segment", "label": "Segment",
		"data_type": "text", "required": true, "indexed": true, "default_value": "standard", "default_semantics": "backfill_required",
	}})
	validated := invokeRecord(t, invoker, principal, "metadata.changeset.validate", map[string]any{"candidate_metadata_version_id": candidate.MetadataVersionID})
	requireSuccess(t, validated)
	var changeset metadata.Changeset
	if err := json.Unmarshal(validated.Result, &changeset); err != nil {
		t.Fatal(err)
	}
	requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.changeset.approve", map[string]any{
		"changeset_id": changeset.ChangesetID, "approval_id": "approval-record-model",
	}))
	during := requireRecord(t, invokeRecord(t, invoker, principal, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "During backfill"},
	}))
	duringContact := requireRecord(t, invokeRecord(t, invoker, principal, "runtime.record.create", map[string]any{
		"object_api_name": "contact", "data": map[string]any{"name": "During contact", "customer": during.RecordID},
	}))
	if _, exposed := during.Data["segment"]; exposed {
		t.Fatalf("candidate-only field leaked before activation: %#v", during)
	}
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		var physicalVersion, segment string
		if err := tx.QueryRow(context.Background(), "select metadata_version_id,data->>'segment' from object_record where record_id=$1", during.RecordID).Scan(&physicalVersion, &segment); err != nil {
			return err
		}
		if physicalVersion != candidate.MetadataVersionID || segment != "standard" {
			return fmt.Errorf("physical candidate projection version=%s segment=%s", physicalVersion, segment)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for range 4 {
		requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.changeset.backfill", map[string]any{
			"changeset_id": changeset.ChangesetID, "batch_size": 10,
		}))
	}
	requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.changeset.validate-coverage", map[string]any{"changeset_id": changeset.ChangesetID}))
	requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.changeset.publish", map[string]any{"changeset_id": changeset.ChangesetID}))
	for _, recordID := range []string{legacy.RecordID, during.RecordID} {
		got := requireRecord(t, invokeRecord(t, invoker, principal, "runtime.record.get", map[string]any{"object_api_name": "customer", "record_id": recordID}))
		if got.Data["segment"] != "standard" || got.MetadataVersionID != candidate.MetadataVersionID {
			t.Fatalf("activated projection missing for %s: %#v", recordID, got)
		}
	}
	var relationRows int
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			"select count(*) from record_relation where metadata_version_id=$1 and source_record_id in ($2,$3)",
			candidate.MetadataVersionID, legacyContact.RecordID, duringContact.RecordID,
		).Scan(&relationRows)
	}); err != nil || relationRows != 2 {
		t.Fatalf("candidate relation coverage rows=%d err=%v", relationRows, err)
	}
}

func TestOptionalOnCreateDefaultDoesNotRewriteHistoricalRecords(t *testing.T) {
	_, control, runtime := recordTestPools(t)
	metadataService := metadata.NewService(runtime, control)
	recordService := NewService(runtime, control)
	invoker := capability.NewInvoker(capability.NewRegistry(append(metadata.CapabilityDefinitions(metadataService), CapabilityDefinitions(recordService)...)), 8)
	principal := recordPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa", "optional-agent")
	ids := publishRecordModel(t, invoker, principal)
	legacy := requireRecord(t, invokeRecord(t, invoker, principal, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Historical"},
	}))
	regionID := mustRecordV7(t)
	candidate := buildRecordModelVersion(t, invoker, principal, ids, []map[string]any{{
		"field_id": regionID.String(), "object_id": ids.CustomerID.String(), "api_name": "region", "label": "Region",
		"data_type": "text", "default_value": "new-only", "default_semantics": "on_create",
	}})
	validated := invokeRecord(t, invoker, principal, "metadata.changeset.validate", map[string]any{"candidate_metadata_version_id": candidate.MetadataVersionID})
	requireSuccess(t, validated)
	var changeset metadata.Changeset
	if err := json.Unmarshal(validated.Result, &changeset); err != nil {
		t.Fatal(err)
	}
	if changeset.RequiresBackfill {
		t.Fatalf("on_create optional field unexpectedly requires backfill: %#v", changeset)
	}
	requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.changeset.approve", map[string]any{
		"changeset_id": changeset.ChangesetID, "approval_id": "approval-record-model",
	}))
	requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.changeset.publish", map[string]any{"changeset_id": changeset.ChangesetID}))
	historical := requireRecord(t, invokeRecord(t, invoker, principal, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": legacy.RecordID,
	}))
	if _, exists := historical.Data["region"]; exists {
		t.Fatalf("on_create default was projected onto historical record: %#v", historical)
	}
	created := requireRecord(t, invokeRecord(t, invoker, principal, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "New"},
	}))
	if created.Data["region"] != "new-only" {
		t.Fatalf("new record did not receive on_create default: %#v", created)
	}
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	var physicalVersion string
	var keyPresent bool
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), "select metadata_version_id,data ? 'region' from object_record where record_id=$1", legacy.RecordID).Scan(&physicalVersion, &keyPresent)
	}); err != nil {
		t.Fatal(err)
	}
	if physicalVersion != legacy.MetadataVersionID || keyPresent {
		t.Fatalf("historical row was rewritten: version=%s key_present=%v", physicalVersion, keyPresent)
	}
}

type recordModelIDs struct {
	CustomerID, ContactID                                                           uuid.UUID
	CustomerNameID, AmountID, ActiveID, OpenedID, ExternalID, NoteID, ContactNameID uuid.UUID
	RelationID                                                                      uuid.UUID
}

func publishRecordModel(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal) recordModelIDs {
	t.Helper()
	ids := recordModelIDs{
		CustomerID: mustRecordV7(t), ContactID: mustRecordV7(t), CustomerNameID: mustRecordV7(t), AmountID: mustRecordV7(t),
		ActiveID: mustRecordV7(t), OpenedID: mustRecordV7(t), ExternalID: mustRecordV7(t), NoteID: mustRecordV7(t),
		ContactNameID: mustRecordV7(t), RelationID: mustRecordV7(t),
	}
	publishRecordModelWithIDs(t, invoker, principal, ids)
	return ids
}

func publishRecordModelWithIDs(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, ids recordModelIDs) {
	t.Helper()
	version := buildRecordModelVersion(t, invoker, principal, ids, nil)
	publishResponse := invokeRecord(t, invoker, principal, "metadata.version.publish", map[string]any{
		"metadata_version_id": version.MetadataVersionID, "approval_id": "approval-record-model",
	})
	if publishResponse.Status == capability.StatusFailed && publishResponse.Error != nil && publishResponse.Error.Code == capability.CodeFailedPrecondition {
		validated := invokeRecord(t, invoker, principal, "metadata.changeset.validate", map[string]any{
			"candidate_metadata_version_id": version.MetadataVersionID,
		})
		requireSuccess(t, validated)
		var changeset metadata.Changeset
		if err := json.Unmarshal(validated.Result, &changeset); err != nil {
			t.Fatal(err)
		}
		requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.changeset.approve", map[string]any{
			"changeset_id": changeset.ChangesetID, "approval_id": "approval-record-model",
		}))
		requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.changeset.publish", map[string]any{
			"changeset_id": changeset.ChangesetID,
		}))
		return
	}
	requireSuccess(t, publishResponse)
}

func buildRecordModelVersion(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, ids recordModelIDs, extraFields []map[string]any) metadata.Version {
	t.Helper()
	versionResponse := invokeRecord(t, invoker, principal, "metadata.version.create", map[string]any{})
	if versionResponse.Status != capability.StatusSucceeded {
		t.Fatalf("create metadata version=%#v", versionResponse)
	}
	var version metadata.Version
	if err := json.Unmarshal(versionResponse.Result, &version); err != nil {
		t.Fatal(err)
	}
	for _, object := range []struct {
		id         uuid.UUID
		api, label string
	}{{ids.CustomerID, "customer", "Customer"}, {ids.ContactID, "contact", "Contact"}} {
		requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.object.upsert", map[string]any{
			"metadata_version_id": version.MetadataVersionID, "object_id": object.id.String(), "api_name": object.api, "label": object.label,
		}))
	}
	fields := []map[string]any{
		{"field_id": ids.CustomerNameID.String(), "object_id": ids.CustomerID.String(), "api_name": "name", "label": "Name", "data_type": "text", "required": true, "indexed": true, "constraints": map[string]any{"max_length": 200}},
		{"field_id": ids.AmountID.String(), "object_id": ids.CustomerID.String(), "api_name": "amount", "label": "Amount", "data_type": "number", "indexed": true, "default_value": 0},
		{"field_id": ids.ActiveID.String(), "object_id": ids.CustomerID.String(), "api_name": "active", "label": "Active", "data_type": "boolean", "indexed": true},
		{"field_id": ids.OpenedID.String(), "object_id": ids.CustomerID.String(), "api_name": "opened", "label": "Opened", "data_type": "date", "indexed": true},
		{"field_id": ids.ExternalID.String(), "object_id": ids.CustomerID.String(), "api_name": "external_id", "label": "External ID", "data_type": "uuid", "indexed": true},
		{"field_id": ids.NoteID.String(), "object_id": ids.CustomerID.String(), "api_name": "note", "label": "Note", "data_type": "text"},
		{"field_id": ids.ContactNameID.String(), "object_id": ids.ContactID.String(), "api_name": "name", "label": "Name", "data_type": "text", "required": true, "indexed": true},
	}
	fields = append(fields, extraFields...)
	for _, field := range fields {
		field["metadata_version_id"] = version.MetadataVersionID
		requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.field.upsert", field))
	}
	requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.relation.upsert", map[string]any{
		"metadata_version_id": version.MetadataVersionID, "relation_id": ids.RelationID.String(), "api_name": "customer",
		"source_object_id": ids.ContactID.String(), "target_object_id": ids.CustomerID.String(), "relation_type": "lookup", "delete_behavior": "restrict",
	}))
	return version
}

func invokeRecord(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, capabilityID string, input any) capability.Response {
	return invokeRecordWithKey(t, invoker, principal, capabilityID, "", input)
}

func invokeRecordWithKey(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, capabilityID, idempotencyKey string, input any) capability.Response {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	request, stableErr := capability.BindTrustedPrincipal(capability.Request{
		CapabilityID: capabilityID, RequestID: "req-record-" + time.Now().Format("150405.000000000") + "-" + strconv.FormatInt(requestSequence.Add(1), 10),
		IdempotencyKey: idempotencyKey, Input: raw,
	}, principal)
	if stableErr != nil {
		t.Fatal(stableErr)
	}
	return invoker.Invoke(context.Background(), request)
}

func recordPrincipal(tenantID, companyID, actorID string) capability.TrustedPrincipal {
	return capability.TrustedPrincipal{
		TenantID: tenantID, CompanyID: companyID, Source: "test-jwt", Approvals: []string{"approval-record-model"},
		Actor: capability.Actor{ID: actorID, Scopes: []string{
			"metadata.version.write", "metadata.definition.write", "metadata.publish", "metadata.read",
			"metadata.changeset.write", "metadata.changeset.read", "metadata.changeset.approve", "metadata.changeset.publish", "metadata.changeset.execute", "metadata.changeset.purge",
			"runtime.record.create", "runtime.record.read", "runtime.record.update", "runtime.record.delete",
		}},
	}
}

func requireSuccess(t *testing.T, response capability.Response) {
	t.Helper()
	if response.Status != capability.StatusSucceeded {
		t.Fatalf("response=%#v", response)
	}
}

func requireRecord(t *testing.T, response capability.Response) Record {
	t.Helper()
	requireSuccess(t, response)
	var result Record
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func requireQuery(t *testing.T, response capability.Response) QueryResult {
	t.Helper()
	requireSuccess(t, response)
	var result QueryResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertRecordError(t *testing.T, response capability.Response, code capability.ErrorCode) {
	t.Helper()
	if response.Status != capability.StatusFailed || response.Error == nil || response.Error.Code != code {
		t.Fatalf("response=%#v want=%s", response, code)
	}
}

func assertTypedRows(t *testing.T, admin *pgxpool.Pool, recordID string, expected map[string]int) {
	t.Helper()
	for table, want := range expected {
		var count int
		if err := admin.QueryRow(context.Background(), "select count(*) from "+table+" where record_id=$1", recordID).Scan(&count); err != nil || count != want {
			t.Fatalf("%s rows=%d want=%d err=%v", table, count, want, err)
		}
	}
}

func assertRecordAdapterParity(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, recordID string) {
	t.Helper()
	input := map[string]any{"object_api_name": "customer", "record_id": recordID}
	raw, _ := json.Marshal(input)
	request := capability.Request{CapabilityID: "runtime.record.get", RequestID: "req-record-parity", Input: raw}
	body, _ := json.Marshal(request)
	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/v1/capabilities/runtime.record.get/invoke", bytes.NewReader(body))
	httpRequest.Header.Set("Authorization", "Bearer test")
	api.NewAuthenticatedHandler(invoker, recordVerifier{principal}).ServeHTTP(recorder, httpRequest)
	var apiResponse capability.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &apiResponse); err != nil || recorder.Code != http.StatusOK {
		t.Fatalf("API response=%s err=%v", recorder.Body.String(), err)
	}
	var stdout bytes.Buffer
	if exit := cli.RunAs(context.Background(), invoker, principal, []string{"capability", "invoke", "--id", request.CapabilityID}, bytes.NewReader(body), &stdout, io.Discard); exit != 0 {
		t.Fatalf("CLI output=%s", stdout.String())
	}
	var cliResponse capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &cliResponse); err != nil {
		t.Fatal(err)
	}
	mcpResponse := invokeRecordMCP(t, invoker, principal, request.RequestID, input)
	if !equalJSON(apiResponse.Result, cliResponse.Result) || !equalJSON(apiResponse.Result, mcpResponse.Result) {
		t.Fatalf("adapter drift API=%s CLI=%s MCP=%s", apiResponse.Result, cliResponse.Result, mcpResponse.Result)
	}
}

func assertAllRecordAdapterParity(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal) {
	t.Helper()
	recordID := mustRecordV7(t).String()
	create := map[string]any{"object_api_name": "customer", "record_id": recordID, "data": map[string]any{"name": "Parity"}}
	assertAdapterResultsEqual(t, invoker, principal, "runtime.record.create", "parity-create", create)
	update := map[string]any{"object_api_name": "customer", "record_id": recordID, "expected_revision": 1, "patch": map[string]any{"amount": 5}}
	assertAdapterResultsEqual(t, invoker, principal, "runtime.record.update", "parity-update", update)
	query := map[string]any{"object_api_name": "customer", "filters": []map[string]any{{"field": "name", "op": "eq", "value": "Parity"}}, "limit": 10}
	assertAdapterResultsEqual(t, invoker, principal, "runtime.record.query", "", query)
	deleteInput := map[string]any{"object_api_name": "customer", "record_id": recordID, "expected_revision": 2}
	assertAdapterResultsEqual(t, invoker, principal, "runtime.record.delete", "parity-delete", deleteInput)
	get := map[string]any{"object_api_name": "customer", "record_id": recordID, "include_deleted": true}
	assertAdapterResultsEqual(t, invoker, principal, "runtime.record.get", "", get)
}

func assertAdapterResultsEqual(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, capabilityID, idempotencyKey string, input any) {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	requestID := "req-parity-" + capability.MCPToolName(capabilityID)
	request := capability.Request{CapabilityID: capabilityID, RequestID: requestID, IdempotencyKey: idempotencyKey, Input: raw}
	body, _ := json.Marshal(request)
	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/v1/capabilities/"+capabilityID+"/invoke", bytes.NewReader(body))
	httpRequest.Header.Set("Authorization", "Bearer test")
	api.NewAuthenticatedHandler(invoker, recordVerifier{principal}).ServeHTTP(recorder, httpRequest)
	var apiResponse capability.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &apiResponse); err != nil || apiResponse.Status != capability.StatusSucceeded {
		t.Fatalf("%s API response=%s err=%v", capabilityID, recorder.Body.String(), err)
	}
	var stdout bytes.Buffer
	if exit := cli.RunAs(context.Background(), invoker, principal, []string{"capability", "invoke", "--id", capabilityID}, bytes.NewReader(body), &stdout, io.Discard); exit != 0 {
		t.Fatalf("%s CLI output=%s", capabilityID, stdout.String())
	}
	var cliResponse capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &cliResponse); err != nil {
		t.Fatal(err)
	}
	mcpResponse := invokeCapabilityMCP(t, invoker, principal, capabilityID, requestID, idempotencyKey, input)
	if !equalJSON(apiResponse.Result, cliResponse.Result) || !equalJSON(apiResponse.Result, mcpResponse.Result) {
		t.Fatalf("%s adapter drift API=%s CLI=%s MCP=%s", capabilityID, apiResponse.Result, cliResponse.Result, mcpResponse.Result)
	}
}

func assertAdapterErrorsEqual(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, capabilityID string, input any, expected capability.ErrorCode) {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	requestID := "req-error-parity-" + capability.MCPToolName(capabilityID)
	request := capability.Request{CapabilityID: capabilityID, RequestID: requestID, Input: raw}
	body, _ := json.Marshal(request)
	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/v1/capabilities/"+capabilityID+"/invoke", bytes.NewReader(body))
	httpRequest.Header.Set("Authorization", "Bearer test")
	api.NewAuthenticatedHandler(invoker, recordVerifier{principal}).ServeHTTP(recorder, httpRequest)
	var apiResponse capability.Response
	wantStatus := http.StatusBadRequest
	switch expected {
	case capability.CodeResourceNotFound, capability.CodeCapabilityNotFound:
		wantStatus = http.StatusNotFound
	case capability.CodeUnauthorized:
		wantStatus = http.StatusForbidden
	case capability.CodeFailedPrecondition:
		wantStatus = http.StatusPreconditionFailed
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &apiResponse); err != nil || recorder.Code != wantStatus {
		t.Fatalf("%s API error response=%s status=%d err=%v", capabilityID, recorder.Body.String(), recorder.Code, err)
	}

	var stdout bytes.Buffer
	if exit := cli.RunAs(context.Background(), invoker, principal, []string{"capability", "invoke", "--id", capabilityID}, bytes.NewReader(body), &stdout, io.Discard); exit != 1 {
		t.Fatalf("%s CLI error exit=%d output=%s", capabilityID, exit, stdout.String())
	}
	var cliResponse capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &cliResponse); err != nil {
		t.Fatal(err)
	}
	mcpResponse := invokeCapabilityMCP(t, invoker, principal, capabilityID, requestID, "", input)
	responses := []capability.Response{apiResponse, cliResponse, mcpResponse}
	for _, response := range responses {
		if response.Status != capability.StatusFailed || response.Error == nil || response.Error.Code != expected {
			t.Fatalf("%s adapter error=%#v", capabilityID, response)
		}
	}
	if apiResponse.Error.Message != cliResponse.Error.Message || apiResponse.Error.Message != mcpResponse.Error.Message {
		t.Fatalf("%s error message drift API=%q CLI=%q MCP=%q", capabilityID, apiResponse.Error.Message, cliResponse.Error.Message, mcpResponse.Error.Message)
	}
}

type recordVerifier struct{ principal capability.TrustedPrincipal }

func (verifier recordVerifier) Verify(context.Context, string) (capability.TrustedPrincipal, error) {
	return verifier.principal, nil
}

func invokeRecordMCP(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, requestID string, input any) capability.Response {
	return invokeCapabilityMCP(t, invoker, principal, "runtime.record.get", requestID, "", input)
}

func invokeCapabilityMCP(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, capabilityID, requestID, idempotencyKey string, input any) capability.Response {
	t.Helper()
	ctx := context.Background()
	server := mcpserver.NewServerAs(invoker, principal)
	client := mcp.NewClient(&mcp.Implementation{Name: "record-parity", Version: "v1"}, nil)
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
		Name: capability.MCPToolName(capabilityID), Arguments: map[string]any{"request_id": requestID, "idempotency_key": idempotencyKey, "input": input},
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

func equalJSON(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	return json.Unmarshal(left, &leftValue) == nil && json.Unmarshal(right, &rightValue) == nil && reflect.DeepEqual(leftValue, rightValue)
}

func sameRecord(left, right Record) bool {
	left.CreatedAt, left.UpdatedAt, right.CreatedAt, right.UpdatedAt = time.Time{}, time.Time{}, time.Time{}, time.Time{}
	return reflect.DeepEqual(left, right)
}

func recordTestPools(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(context.Background(), url)
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
	control := poolAs(t, url, "ai_native_control", 4)
	runtime := poolAs(t, url, "ai_native_runtime", 4)
	insertRecordTenant(t, control, "11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa", 7)
	t.Cleanup(func() {
		runtime.Close()
		control.Close()
		_, _ = lock.Exec(context.Background(), "select pg_advisory_unlock(7167614658367249410)")
		lock.Release()
		admin.Close()
	})
	return admin, control, runtime
}

func poolAs(t *testing.T, url, user string, max int32) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.User = user
	config.MaxConns = max
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

func insertRecordTenant(t *testing.T, control *pgxpool.Pool, tenantID, companyID string, bucket int16) {
	t.Helper()
	_, err := control.Exec(context.Background(),
		"insert into tenant_registry(tenant_id,company_id,display_name,shard_id,tenant_bucket,service_tier,global_lifecycle_status,native_status,tenant_revision,product_revision,route_revision) values ($1,$2,$3,'shard-001',$4,'standard','active','active',1,1,1)",
		tenantID, companyID, "Tenant "+companyID, bucket,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func assertRecordSchema(t *testing.T, admin, runtime *pgxpool.Pool) {
	t.Helper()
	var partitions int
	if err := admin.QueryRow(context.Background(),
		"select count(*) from pg_inherits where inhparent in ('record_index_text'::regclass,'record_index_number'::regclass,'record_index_boolean'::regclass,'record_index_datetime'::regclass,'record_index_uuid'::regclass)",
	).Scan(&partitions); err != nil || partitions != 640 {
		t.Fatalf("typed partitions=%d err=%v", partitions, err)
	}
	var unsecured int
	if err := admin.QueryRow(context.Background(),
		"select count(*) from pg_class where relname in ('record_index_text','record_index_number','record_index_boolean','record_index_datetime','record_index_uuid','record_operation') and (not relrowsecurity or not relforcerowsecurity)",
	).Scan(&unsecured); err != nil || unsecured != 0 {
		t.Fatalf("unsecured typed parents=%d err=%v", unsecured, err)
	}
	var unsecuredPartitions int
	if err := admin.QueryRow(context.Background(),
		"select count(*) from pg_class child join pg_inherits i on i.inhrelid=child.oid where i.inhparent in ('record_index_text'::regclass,'record_index_number'::regclass,'record_index_boolean'::regclass,'record_index_datetime'::regclass,'record_index_uuid'::regclass) and (not child.relrowsecurity or not child.relforcerowsecurity)",
	).Scan(&unsecuredPartitions); err != nil || unsecuredPartitions != 0 {
		t.Fatalf("unsecured typed partitions=%d err=%v", unsecuredPartitions, err)
	}
	var versionBoundIndexes int
	if err := admin.QueryRow(context.Background(),
		"select count(distinct conrelid) from pg_constraint where contype='f' and conrelid in ('record_index_text'::regclass,'record_index_number'::regclass,'record_index_boolean'::regclass,'record_index_datetime'::regclass,'record_index_uuid'::regclass) and pg_get_constraintdef(oid) like 'FOREIGN KEY (tenant_bucket, tenant_id, metadata_version_id, object_id, record_id)%'",
	).Scan(&versionBoundIndexes); err != nil || versionBoundIndexes != 5 {
		t.Fatalf("metadata-version-bound typed index FKs=%d err=%v", versionBoundIndexes, err)
	}
	var recordVersionUnique, relationVersionBound bool
	if err := admin.QueryRow(context.Background(),
		"select exists(select 1 from pg_constraint where conname='object_record_metadata_record_unique'),exists(select 1 from pg_constraint where conname='record_relation_metadata_source_fk')",
	).Scan(&recordVersionUnique, &relationVersionBound); err != nil || !recordVersionUnique || !relationVersionBound {
		t.Fatalf("record version constraints unique=%v relation=%v err=%v", recordVersionUnique, relationVersionBound, err)
	}
	for _, table := range []string{"object_record", "record_relation", "record_operation", "record_index_text", "record_index_number", "record_index_boolean", "record_index_datetime", "record_index_uuid"} {
		var control, runtime bool
		if err := admin.QueryRow(context.Background(), "select has_table_privilege('ai_native_control',$1,'SELECT,INSERT,UPDATE,DELETE'),has_table_privilege('ai_native_runtime',$1,'SELECT,INSERT,UPDATE,DELETE')", table).Scan(&control, &runtime); err != nil {
			t.Fatal(err)
		}
		if control || !runtime {
			t.Fatalf("table=%s control=%v runtime=%v", table, control, runtime)
		}
	}
	for _, table := range []string{"object_record", "record_relation", "record_operation", "record_index_text", "record_index_number", "record_index_boolean", "record_index_datetime", "record_index_uuid"} {
		var count int
		if err := runtime.QueryRow(context.Background(), "select count(*) from "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("missing TenantContext exposed %s rows=%d err=%v", table, count, err)
		}
	}
}

func mustRecordV7(t *testing.T) uuid.UUID {
	t.Helper()
	value, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return value
}
