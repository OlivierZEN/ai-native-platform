package record

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/authorization"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/metadata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestRecordPhysicalBenchmark is opt-in because the accepted profile contains
// one million records. It measures the physical PostgreSQL design rather than
// claiming a production SLA. Run it only against the dedicated local test DB.
func TestRecordPhysicalBenchmark(t *testing.T) {
	if os.Getenv("AI_NATIVE_RUN_RECORD_BENCHMARK") != "1" {
		t.Skip("set AI_NATIVE_RUN_RECORD_BENCHMARK=1 to run the one-million-record profile")
	}
	recordCount := 1_000_000
	if raw := os.Getenv("AI_NATIVE_RECORD_BENCHMARK_COUNT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			t.Fatalf("invalid AI_NATIVE_RECORD_BENCHMARK_COUNT=%q", raw)
		}
		recordCount = parsed
	}

	admin, control, runtime := recordTestPools(t)
	metadataService := metadata.NewService(runtime, control)
	invoker := capability.NewInvoker(capability.NewRegistry(metadata.CapabilityDefinitions(metadataService)), 4)
	principal := recordPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa", "benchmark-agent")
	ids := publishBenchmarkModel(t, invoker, principal)
	var metadataVersionID uuid.UUID
	if err := control.QueryRow(context.Background(), "select metadata_version_id from tenant_registry where tenant_id=$1", principal.TenantID).Scan(&metadataVersionID); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	defer cancel()
	recordIDExpression := "('019c0000-0000-7000-8000-' || lpad(to_hex(i),12,'0'))::uuid"
	started := time.Now()
	if _, err := admin.Exec(ctx, fmt.Sprintf(
		"insert into object_record(tenant_bucket,tenant_id,metadata_version_id,object_id,record_id,owner_id,lifecycle_state,data,revision,created_by,updated_by) "+
			"select 7,$1,$2,$3,%s,'benchmark-agent','active',jsonb_build_object('name','customer-'||i,'amount',i),1,'benchmark-agent','benchmark-agent' from generate_series(1,$4) i",
		recordIDExpression,
	), principal.TenantID, metadataVersionID, ids.objectID, recordCount); err != nil {
		t.Fatal(err)
	}
	recordInsertDuration := time.Since(started)

	started = time.Now()
	if _, err := admin.Exec(ctx,
		"insert into record_index_text(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_text) "+
			"select tenant_bucket,tenant_id,metadata_version_id,object_id,$1,record_id,data->>'name' from object_record where tenant_bucket=7 and tenant_id=$2 and object_id=$3",
		ids.nameFieldID, principal.TenantID, ids.objectID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx,
		"insert into record_index_number(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_number) "+
			"select tenant_bucket,tenant_id,metadata_version_id,object_id,$1,record_id,(data->>'amount')::numeric from object_record where tenant_bucket=7 and tenant_id=$2 and object_id=$3",
		ids.amountFieldID, principal.TenantID, ids.objectID,
	); err != nil {
		t.Fatal(err)
	}
	indexInsertDuration := time.Since(started)
	if _, err := admin.Exec(ctx, "analyze object_record; analyze record_index_text; analyze record_index_number"); err != nil {
		t.Fatal(err)
	}

	var objectRows, textRows, numberRows int64
	if err := admin.QueryRow(ctx, "select count(*) from object_record where tenant_bucket=7 and tenant_id=$1 and object_id=$2", principal.TenantID, ids.objectID).Scan(&objectRows); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, "select count(*) from record_index_text where tenant_bucket=7 and tenant_id=$1 and object_id=$2", principal.TenantID, ids.objectID).Scan(&textRows); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, "select count(*) from record_index_number where tenant_bucket=7 and tenant_id=$1 and object_id=$2", principal.TenantID, ids.objectID).Scan(&numberRows); err != nil {
		t.Fatal(err)
	}
	if objectRows != int64(recordCount) || textRows != objectRows || numberRows != objectRows {
		t.Fatalf("row mismatch object=%d text=%d number=%d", objectRows, textRows, numberRows)
	}

	objectBytes := partitionTreeSize(t, ctx, admin, "object_record")
	textBytes := partitionTreeSize(t, ctx, admin, "record_index_text")
	numberBytes := partitionTreeSize(t, ctx, admin, "record_index_number")

	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	model := objectModel{MetadataVersionID: metadataVersionID.String(), ObjectID: ids.objectID.String(), APIName: "customer"}
	nameField := fieldSpec{FieldID: ids.nameFieldID.String(), APIName: "name", DataType: "text", Indexed: true}
	amountField := fieldSpec{FieldID: ids.amountFieldID.String(), APIName: "amount", DataType: "number", Indexed: true}
	latencies := make([]time.Duration, 0, 200)
	for index := 0; index < 100; index++ {
		value := 1 + (index*7919)%recordCount
		equalityFilter, stableErr := normalizeFilter(nameField, FilterInput{
			Field: "name", Operator: "eq", Value: json.RawMessage(strconv.Quote(fmt.Sprintf("customer-%d", value))),
		})
		if stableErr != nil {
			t.Fatal(stableErr)
		}
		equalitySQL, equalityArguments := buildQuery(tenant, model, []normalizedFilter{equalityFilter}, uuid.Nil, 50, authorization.RecordScope{AllowAll: true}, "benchmark")
		started = time.Now()
		rows, err := admin.Query(ctx, equalitySQL, equalityArguments...)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			if _, err := rows.Values(); err != nil {
				t.Fatal(err)
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		latencies = append(latencies, time.Since(started))

		lowerFilter, stableErr := normalizeFilter(amountField, FilterInput{
			Field: "amount", Operator: "gte", Value: json.RawMessage(strconv.Itoa(value)),
		})
		if stableErr != nil {
			t.Fatal(stableErr)
		}
		upperFilter, stableErr := normalizeFilter(amountField, FilterInput{
			Field: "amount", Operator: "lt", Value: json.RawMessage(strconv.Itoa(value + 50)),
		})
		if stableErr != nil {
			t.Fatal(stableErr)
		}
		rangeSQL, rangeArguments := buildQuery(tenant, model, []normalizedFilter{lowerFilter, upperFilter}, uuid.Nil, 50, authorization.RecordScope{AllowAll: true}, "benchmark")
		started = time.Now()
		rows, err = admin.Query(ctx, rangeSQL, rangeArguments...)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			if _, err := rows.Values(); err != nil {
				t.Fatal(err)
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		latencies = append(latencies, time.Since(started))
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	lowerFilter, stableErr := normalizeFilter(amountField, FilterInput{
		Field: "amount", Operator: "gte", Value: json.RawMessage(strconv.Itoa(recordCount / 2)),
	})
	if stableErr != nil {
		t.Fatal(stableErr)
	}
	upperFilter, stableErr := normalizeFilter(amountField, FilterInput{
		Field: "amount", Operator: "lt", Value: json.RawMessage(strconv.Itoa(recordCount/2 + 50)),
	})
	if stableErr != nil {
		t.Fatal(stableErr)
	}
	rangeSQL, rangeArguments := buildQuery(tenant, model, []normalizedFilter{lowerFilter, upperFilter}, uuid.Nil, 50, authorization.RecordScope{AllowAll: true}, "benchmark")
	plan := explainJSON(t, ctx, admin, rangeSQL, rangeArguments...)
	expectedIndex := "record_index_number_b007_value_idx"
	if !strings.Contains(plan, expectedIndex) {
		t.Fatalf("range plan does not use %s: %s", expectedIndex, plan)
	}
	if strings.Contains(plan, `"Node Type": "Seq Scan"`) && strings.Contains(plan, `"Relation Name": "object_record_b007"`) {
		t.Fatalf("range plan performs an object_record full scan: %s", plan)
	}

	totalBytes := objectBytes + textBytes + numberBytes
	result := map[string]any{
		"records": objectRows, "typed_index_rows": textRows + numberRows,
		"typed_rows_per_record": float64(textRows+numberRows) / float64(objectRows),
		"record_insert_seconds": recordInsertDuration.Seconds(), "typed_index_insert_seconds": indexInsertDuration.Seconds(),
		"object_bytes": objectBytes, "text_index_bytes": textBytes, "number_index_bytes": numberBytes,
		"total_bytes": totalBytes, "bytes_per_record": float64(totalBytes) / float64(objectRows),
		"query_samples": len(latencies), "query_p50_ms": durationMilliseconds(latencies[len(latencies)/2]),
		"query_p95_ms": durationMilliseconds(latencies[(len(latencies)*95)/100]), "expected_index": expectedIndex,
	}
	encoded, _ := json.Marshal(result)
	t.Logf("RECORD_BENCHMARK %s", encoded)
}

type benchmarkModelIDs struct {
	objectID, nameFieldID, amountFieldID uuid.UUID
}

func publishBenchmarkModel(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal) benchmarkModelIDs {
	t.Helper()
	ids := benchmarkModelIDs{mustRecordV7(t), mustRecordV7(t), mustRecordV7(t)}
	versionResponse := invokeRecord(t, invoker, principal, "metadata.version.create", map[string]any{})
	requireSuccess(t, versionResponse)
	var version metadata.Version
	if err := json.Unmarshal(versionResponse.Result, &version); err != nil {
		t.Fatal(err)
	}
	requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.object.upsert", map[string]any{
		"metadata_version_id": version.MetadataVersionID, "object_id": ids.objectID.String(), "api_name": "customer", "label": "Customer",
	}))
	for _, field := range []map[string]any{
		{"field_id": ids.nameFieldID.String(), "api_name": "name", "label": "Name", "data_type": "text", "required": true, "indexed": true},
		{"field_id": ids.amountFieldID.String(), "api_name": "amount", "label": "Amount", "data_type": "number", "indexed": true},
	} {
		field["metadata_version_id"] = version.MetadataVersionID
		field["object_id"] = ids.objectID.String()
		requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.field.upsert", field))
	}
	requireSuccess(t, invokeRecord(t, invoker, principal, "metadata.version.publish", map[string]any{
		"metadata_version_id": version.MetadataVersionID, "approval_id": "approval-record-model",
	}))
	return ids
}

func partitionTreeSize(t *testing.T, ctx context.Context, admin *pgxpool.Pool, table string) int64 {
	t.Helper()
	var size int64
	statement := "select coalesce(sum(pg_total_relation_size(relid)),0) from pg_partition_tree('" + table + "'::regclass)"
	if err := admin.QueryRow(ctx, statement).Scan(&size); err != nil {
		t.Fatal(err)
	}
	return size
}

func explainJSON(t *testing.T, ctx context.Context, admin *pgxpool.Pool, statement string, arguments ...any) string {
	t.Helper()
	var plan []byte
	if err := admin.QueryRow(ctx, "explain (format json,costs off) "+statement, arguments...).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	return string(plan)
}

func durationMilliseconds(value time.Duration) float64 {
	return float64(value.Microseconds()) / 1000
}
