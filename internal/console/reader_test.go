package console

import (
	"encoding/json"
	"testing"
)

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
