package record

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/OlivierZEN/ai-native-platform/internal/authorization"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/google/uuid"
)

func TestNormalizeRecordDataAppliesDefaultsAndValidatesTypes(t *testing.T) {
	model := objectModel{Fields: map[string]fieldSpec{
		"name":   {APIName: "name", DataType: "text", Required: true, LifecycleState: "active"},
		"amount": {APIName: "amount", DataType: "number", Indexed: true, LifecycleState: "active", IndexState: "active", DefaultValue: json.RawMessage("0")},
		"active": {APIName: "active", DataType: "boolean", LifecycleState: "active"},
	}}

	normalized, stableErr := normalizeRecordData(model, nil, map[string]any{"name": "Acme", "active": true}, true)
	if stableErr != nil {
		t.Fatalf("normalize create: %v", stableErr)
	}
	want := map[string]any{"name": "Acme", "amount": json.Number("0"), "active": true}
	if !reflect.DeepEqual(normalized, want) {
		t.Fatalf("normalized=%#v want=%#v", normalized, want)
	}

	for _, invalid := range []map[string]any{
		{"amount": json.Number("1")},
		{"name": "Acme", "unknown": "value"},
		{"name": "Acme", "amount": "not-a-number"},
	} {
		if _, stableErr := normalizeRecordData(model, nil, invalid, true); stableErr == nil {
			t.Fatalf("invalid data passed: %#v", invalid)
		}
	}
}

func TestNormalizeRecordPatchUsesMergeSemanticsAndPreservesRequiredFields(t *testing.T) {
	model := objectModel{Fields: map[string]fieldSpec{
		"name": {APIName: "name", DataType: "text", Required: true, LifecycleState: "active"},
		"note": {APIName: "note", DataType: "text", LifecycleState: "active"},
	}}
	current := map[string]any{"name": "Before", "note": "remove me"}
	normalized, stableErr := normalizeRecordData(model, current, map[string]any{"name": "After", "note": nil}, false)
	if stableErr != nil {
		t.Fatalf("normalize update: %v", stableErr)
	}
	if !reflect.DeepEqual(normalized, map[string]any{"name": "After"}) {
		t.Fatalf("normalized=%#v", normalized)
	}
	if _, stableErr := normalizeRecordData(model, current, map[string]any{"name": nil}, false); stableErr == nil {
		t.Fatal("required field deletion passed")
	}
}

func TestNormalizeFilterRejectsUnindexedOrUnsupportedOperations(t *testing.T) {
	indexed := fieldSpec{FieldID: "019c1234-5678-7abc-8def-0123456789ab", APIName: "amount", DataType: "number", Indexed: true, LifecycleState: "active", IndexState: "active"}
	filter, stableErr := normalizeFilter(indexed, FilterInput{Field: "amount", Operator: "gte", Value: json.RawMessage("100")})
	if stableErr != nil || filter.Table != "record_index_number" || filter.OperatorSQL != ">=" || filter.Value != "100" {
		t.Fatalf("filter=%#v err=%v", filter, stableErr)
	}
	if _, stableErr := normalizeFilter(fieldSpec{APIName: "note", DataType: "text"}, FilterInput{Field: "note", Operator: "eq", Value: json.RawMessage(`"x"`)}); stableErr == nil {
		t.Fatal("unindexed filter passed")
	}
	if _, stableErr := normalizeFilter(indexed, FilterInput{Field: "amount", Operator: "contains", Value: json.RawMessage("100")}); stableErr == nil {
		t.Fatal("unsupported operator passed")
	}
	if _, stableErr := normalizeFilter(indexed, FilterInput{Field: "amount", Operator: "prefix", Value: json.RawMessage("100")}); stableErr == nil {
		t.Fatal("numeric prefix passed")
	}
}

func TestBuildQueryGroupsRangePredicatesAndReusesCursorParameter(t *testing.T) {
	field := fieldSpec{FieldID: "019c1234-5678-7abc-8def-0123456789ab", APIName: "amount", DataType: "number", Indexed: true, LifecycleState: "active", IndexState: "active"}
	lower, lowerErr := normalizeFilter(field, FilterInput{Field: "amount", Operator: "gte", Value: json.RawMessage("100")})
	upper, upperErr := normalizeFilter(field, FilterInput{Field: "amount", Operator: "lt", Value: json.RawMessage("150")})
	if lowerErr != nil || upperErr != nil {
		t.Fatalf("normalize range lower=%v upper=%v", lowerErr, upperErr)
	}
	after := uuid.MustParse("019c1234-5678-7abc-8def-0123456789ac")
	statement, arguments := buildQuery(
		database.TenantContext{TenantID: uuid.MustParse("11111111-1111-4111-8111-111111111111"), Bucket: 7},
		objectModel{MetadataVersionID: "019c1234-5678-7abc-8def-0123456789ad", ObjectID: "019c1234-5678-7abc-8def-0123456789ae"},
		[]normalizedFilter{lower, upper}, after, 51, authorization.RecordScope{AllowAll: true}, "test-principal",
	)
	if strings.Count(statement, "from record_index_number") != 1 || !strings.Contains(statement, "with candidates as materialized") {
		t.Fatalf("range predicates were not grouped: %s", statement)
	}
	if !strings.Contains(statement, "i0.record_id>$7") || !strings.Contains(statement, "r.record_id>$7") {
		t.Fatalf("cursor parameter drift: %s", statement)
	}
	if len(arguments) != 8 || arguments[6] != after || arguments[7] != 51 {
		t.Fatalf("arguments=%#v", arguments)
	}
}

func TestDynamicFieldLifecycleAndJSONGuardrails(t *testing.T) {
	readOnly := objectModel{Fields: map[string]fieldSpec{
		"legacy": {APIName: "legacy", DataType: "text", LifecycleState: "deprecated_read_only"},
	}}
	if _, stableErr := normalizeRecordData(readOnly, map[string]any{"legacy": "kept"}, map[string]any{"legacy": "changed"}, false); stableErr == nil {
		t.Fatal("read-only field accepted a business write")
	}

	jsonModel := objectModel{Fields: map[string]fieldSpec{
		"payload": {APIName: "payload", DataType: "json", LifecycleState: "active"},
	}}
	oversized := strings.Repeat("x", 64*1024)
	if _, stableErr := normalizeRecordData(jsonModel, nil, map[string]any{"payload": oversized}, true); stableErr == nil {
		t.Fatal("oversized JSON field passed")
	}
	tooMany := make([]any, 1001)
	if _, stableErr := normalizeRecordData(jsonModel, nil, map[string]any{"payload": tooMany}, true); stableErr == nil {
		t.Fatal("oversized JSON array passed")
	}
	nested := any("leaf")
	for range 9 {
		nested = map[string]any{"child": nested}
	}
	if _, stableErr := normalizeRecordData(jsonModel, nil, map[string]any{"payload": nested}, true); stableErr == nil {
		t.Fatal("over-deep JSON field passed")
	}

	record := Record{Data: map[string]any{"visible": "yes", "secret": "no"}}
	filterRecordData(objectModel{Fields: map[string]fieldSpec{
		"visible": {LifecycleState: "active"},
		"secret":  {LifecycleState: "hidden"},
	}}, &record, nil)
	if _, leaked := record.Data["secret"]; leaked || record.Data["visible"] != "yes" {
		t.Fatalf("filtered record=%#v", record.Data)
	}
}

func TestRecordJSONApplicationLimit(t *testing.T) {
	fields := make(map[string]fieldSpec, 5)
	changes := make(map[string]any, 5)
	for index := range 5 {
		name := fmt.Sprintf("part_%d", index)
		fields[name] = fieldSpec{APIName: name, DataType: "text", LifecycleState: "active"}
		changes[name] = strings.Repeat("x", 60*1024)
	}
	if _, stableErr := normalizeRecordData(objectModel{Fields: fields}, nil, changes, true); stableErr == nil {
		t.Fatal("record larger than the application JSONB limit passed")
	}
}
