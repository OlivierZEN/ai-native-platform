package record

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

func CapabilityDefinitions(service *Service) []capability.Definition {
	if service == nil {
		panic("record capability definitions require service")
	}
	return []capability.Definition{
		definition("runtime.record.create", "Create a metadata-validated business record in the current tenant.", "medium", "runtime.record.create", createSchema(), recordOutputSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input CreateInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.Create(ctx, request, input)
		}),
		definition("runtime.record.get", "Get one business record by object API name and UUIDv7 record ID.", "low", "runtime.record.read", getSchema(), recordOutputSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input GetInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.Get(ctx, request, input)
		}),
		definition("runtime.record.update", "Merge-patch a business record with optimistic revision control.", "medium", "runtime.record.update", updateSchema(), recordOutputSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input UpdateInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.Update(ctx, request, input)
		}),
		definition("runtime.record.delete", "Soft-delete a business record with optimistic revision control.", "medium", "runtime.record.delete", deleteSchema(), recordOutputSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input DeleteInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.Delete(ctx, request, input)
		}),
		definition("runtime.record.query", "Query active business records through a bounded typed-index DSL.", "low", "runtime.record.read", querySchema(), queryOutputSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input QueryInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.Query(ctx, request, input)
		}),
	}
}

type handler func(context.Context, capability.Request) (any, *capability.StableError)

func definition(id, description, risk, scope string, inputSchema, outputSchema json.RawMessage, invoke handler) capability.Definition {
	return capability.Definition{
		Descriptor: capability.Descriptor{
			ID: id, Version: "v1", Description: description, RiskLevel: risk,
			State: capability.PublicationPublished, RequiredScope: scope,
			InputSchema: inputSchema, OutputSchema: outputSchema,
			Idempotency: capability.IdempotencyPolicy{Enabled: true},
			Execution:   capability.ExecutionPolicy{Mode: capability.ExecutionSynchronous},
		},
		ValidateInput: func(raw json.RawMessage) *capability.StableError {
			var value map[string]json.RawMessage
			if stableErr := decodeInput(raw, &value); stableErr != nil {
				return stableErr
			}
			if value == nil {
				return validationError("record capability input must be an object")
			}
			return nil
		},
		Handler: func(ctx context.Context, request capability.Request, _ capability.RegistryView) (any, *capability.StableError) {
			return invoke(ctx, request)
		},
	}
}

func decodeInput(raw json.RawMessage, target any) *capability.StableError {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return validationError("invalid record capability input: " + err.Error())
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return validationError("record capability input must contain exactly one JSON value")
		}
		return validationError("invalid record capability input: " + err.Error())
	}
	return nil
}

func createSchema() json.RawMessage {
	return schema([]string{"object_api_name", "data"}, map[string]any{
		"object_api_name": apiNameProperty(), "record_id": uuidProperty(),
		"data": map[string]any{"type": "object"},
	})
}

func getSchema() json.RawMessage {
	return schema([]string{"object_api_name", "record_id"}, map[string]any{
		"object_api_name": apiNameProperty(), "record_id": uuidProperty(), "include_deleted": map[string]any{"type": "boolean"},
	})
}

func updateSchema() json.RawMessage {
	return schema([]string{"object_api_name", "record_id", "expected_revision", "patch"}, map[string]any{
		"object_api_name": apiNameProperty(), "record_id": uuidProperty(), "expected_revision": positiveIntegerProperty(),
		"patch": map[string]any{"type": "object"},
	})
}

func deleteSchema() json.RawMessage {
	return schema([]string{"object_api_name", "record_id", "expected_revision"}, map[string]any{
		"object_api_name": apiNameProperty(), "record_id": uuidProperty(), "expected_revision": positiveIntegerProperty(),
	})
}

func querySchema() json.RawMessage {
	return schema([]string{"object_api_name"}, map[string]any{
		"object_api_name": apiNameProperty(), "after": uuidProperty(),
		"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
		"filters": map[string]any{"type": "array", "maxItems": 8, "items": map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"field", "op", "value"},
			"properties": map[string]any{
				"field": apiNameProperty(), "op": map[string]any{"type": "string", "enum": []string{"eq", "prefix", "gt", "gte", "lt", "lte"}}, "value": map[string]any{},
			},
		}},
	})
}

func recordOutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["metadata_version_id","object_id","object_api_name","record_id","lifecycle_state","data","revision"],"additionalProperties":true}`)
}

func queryOutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["records","plan"],"additionalProperties":true}`)
}

func schema(required []string, properties map[string]any) json.RawMessage {
	encoded, err := json.Marshal(map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
		"additionalProperties": false, "required": required, "properties": properties,
	})
	if err != nil {
		panic(err)
	}
	return encoded
}

func apiNameProperty() map[string]any {
	return map[string]any{"type": "string", "pattern": "^[a-z][a-z0-9_]{0,95}$"}
}

func uuidProperty() map[string]any {
	return map[string]any{"type": "string", "format": "uuid"}
}

func positiveIntegerProperty() map[string]any {
	return map[string]any{"type": "integer", "minimum": 1}
}
