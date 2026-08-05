package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

func CapabilityDefinitions(service *Service) []capability.Definition {
	if service == nil {
		panic("metadata capability definitions require service")
	}
	return []capability.Definition{
		definition("metadata.version.create", "Create the next tenant metadata draft version.", "medium", "metadata.version.write", emptySchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input CreateVersionInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.CreateVersion(ctx, request)
		}),
		definition("metadata.object.upsert", "Create or update an object definition in a draft metadata version.", "medium", "metadata.definition.write", objectSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ObjectUpsertInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.UpsertObject(ctx, request, input)
		}),
		definition("metadata.field.upsert", "Create or update a field definition in a draft metadata version.", "medium", "metadata.definition.write", fieldSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input FieldUpsertInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.UpsertField(ctx, request, input)
		}),
		definition("metadata.relation.upsert", "Create or update a relation definition in a draft metadata version.", "medium", "metadata.definition.write", relationSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input RelationUpsertInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.UpsertRelation(ctx, request, input)
		}),
		definition("metadata.version.publish", "Publish the first immutable metadata snapshot with an explicit manual approval ID.", "high", "metadata.publish", publishSchema(), capability.ExecutionPolicy{Mode: capability.ExecutionAsynchronous, ApprovalRequired: true}, func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input PublishInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.Publish(ctx, request, input)
		}),
		definition("metadata.version.get", "Get a metadata version with ordered object, field and relation definitions.", "low", "metadata.read", versionSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input VersionInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.Get(ctx, request, input)
		}),
		definition("metadata.version.get-current", "Get the current published metadata version with ordered object, field and relation definitions.", "low", "metadata.read", emptySchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input struct{}
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.GetCurrent(ctx, request)
		}),
		definition("metadata.changeset.validate", "Validate a draft metadata version, freeze its quota snapshot, and produce a deterministic evolution plan.", "medium", "metadata.changeset.write", changesetValidateSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ChangesetValidateInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.ValidateChangeset(ctx, request, input)
		}),
		definition("metadata.changeset.simulate", "Read the frozen impact simulation and execution plan for a metadata changeset.", "low", "metadata.changeset.read", changesetIDSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ChangesetIDInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.SimulateChangeset(ctx, request, input)
		}),
		definition("metadata.changeset.get-status", "Get the current state, coverage, error, approval, and activation status of a metadata changeset.", "low", "metadata.changeset.read", changesetIDSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ChangesetIDInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.GetChangeset(ctx, request, input)
		}),
		definition("metadata.changeset.approve", "Approve a validated metadata changeset with an explicit manual approval ID.", "high", "metadata.changeset.approve", changesetApproveSchema(), capability.ExecutionPolicy{Mode: capability.ExecutionAsynchronous, ApprovalRequired: true}, func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ChangesetApproveInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.ApproveChangeset(ctx, request, input)
		}),
		definition("metadata.changeset.publish", "Atomically activate an approved metadata changeset after all required evolution work is ready.", "high", "metadata.changeset.publish", changesetIDSchema(), capability.ExecutionPolicy{Mode: capability.ExecutionAsynchronous, ApprovalRequired: true}, func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ChangesetIDInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.PublishChangeset(ctx, request, input)
		}),
		definition("metadata.changeset.backfill", "Process one bounded, resumable batch of non-destructive candidate record and derived-state evolution.", "medium", "metadata.changeset.execute", changesetBatchSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ChangesetBatchInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.BackfillChangeset(ctx, request, input)
		}),
		definition("metadata.changeset.validate-coverage", "Freeze tenant writes, verify complete record/index/unique/reference coverage, and mark a changeset ready.", "medium", "metadata.changeset.execute", changesetIDSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ChangesetIDInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.ValidateCoverage(ctx, request, input)
		}),
		definition("metadata.changeset.purge", "Process one explicitly approved destructive field purge or tombstone batch.", "high", "metadata.changeset.purge", changesetPurgeSchema(), capability.ExecutionPolicy{Mode: capability.ExecutionAsynchronous, ApprovalRequired: true}, func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ChangesetPurgeInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.PurgeChangeset(ctx, request, input)
		}),
		definition("metadata.changeset.cancel", "Cancel a validated or approved metadata changeset before activation.", "medium", "metadata.changeset.write", changesetIDSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ChangesetIDInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.CancelChangeset(ctx, request, input)
		}),
		definition("metadata.changeset.rollback", "Roll back the active metadata pointer when the changeset is non-destructive and still current.", "high", "metadata.changeset.rollback", changesetRollbackSchema(), capability.ExecutionPolicy{Mode: capability.ExecutionAsynchronous, ApprovalRequired: true}, func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ChangesetRollbackInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.RollbackChangeset(ctx, request, input)
		}),
	}
}

type metadataHandler func(context.Context, capability.Request) (any, *capability.StableError)

func definition(id, description, risk, scope string, inputSchema json.RawMessage, execution capability.ExecutionPolicy, handler metadataHandler) capability.Definition {
	return capability.Definition{
		Descriptor: capability.Descriptor{
			ID:            id,
			Version:       "v1",
			Description:   description,
			RiskLevel:     risk,
			State:         capability.PublicationPublished,
			RequiredScope: scope,
			InputSchema:   inputSchema,
			OutputSchema:  genericOutputSchema(),
			Idempotency:   capability.IdempotencyPolicy{Enabled: true},
			Execution:     execution,
		},
		ValidateInput: func(raw json.RawMessage) *capability.StableError {
			var object map[string]json.RawMessage
			if stableErr := decodeInput(raw, &object); stableErr != nil {
				return stableErr
			}
			if object == nil {
				return validationError("capability input must be a JSON object")
			}
			return nil
		},
		Handler: func(ctx context.Context, request capability.Request, _ capability.RegistryView) (any, *capability.StableError) {
			return handler(ctx, request)
		},
	}
}

func synchronous() capability.ExecutionPolicy {
	return capability.ExecutionPolicy{Mode: capability.ExecutionSynchronous}
}

func decodeInput(raw json.RawMessage, target any) *capability.StableError {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return validationError("invalid capability input: " + err.Error())
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return validationError("capability input must contain exactly one JSON value")
		}
		return validationError("invalid capability input: " + err.Error())
	}
	return nil
}

func emptySchema() json.RawMessage {
	return schema(nil, map[string]any{})
}

func versionSchema() json.RawMessage {
	return schema([]string{"metadata_version_id"}, map[string]any{"metadata_version_id": uuidProperty()})
}

func publishSchema() json.RawMessage {
	return schema([]string{"metadata_version_id", "approval_id"}, map[string]any{
		"metadata_version_id": uuidProperty(),
		"approval_id":         stringProperty(),
	})
}

func changesetValidateSchema() json.RawMessage {
	return schema([]string{"candidate_metadata_version_id"}, map[string]any{"candidate_metadata_version_id": uuidProperty()})
}

func changesetIDSchema() json.RawMessage {
	return schema([]string{"changeset_id"}, map[string]any{"changeset_id": uuidProperty()})
}

func changesetApproveSchema() json.RawMessage {
	return schema([]string{"changeset_id", "approval_id"}, map[string]any{
		"changeset_id": uuidProperty(), "approval_id": stringProperty(),
	})
}

func changesetRollbackSchema() json.RawMessage {
	return schema([]string{"changeset_id", "approval_id"}, map[string]any{
		"changeset_id": uuidProperty(), "approval_id": stringProperty(),
	})
}

func changesetBatchSchema() json.RawMessage {
	return schema([]string{"changeset_id"}, map[string]any{
		"changeset_id": uuidProperty(), "batch_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
	})
}

func changesetPurgeSchema() json.RawMessage {
	return schema([]string{"changeset_id", "approval_id"}, map[string]any{
		"changeset_id": uuidProperty(), "batch_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
		"approval_id": stringProperty(),
	})
}

func objectSchema() json.RawMessage {
	return schema([]string{"metadata_version_id", "api_name", "label"}, map[string]any{
		"metadata_version_id": uuidProperty(), "object_id": uuidProperty(), "api_name": apiNameProperty(),
		"label": stringProperty(), "description": map[string]any{"type": "string"}, "semantic": map[string]any{"type": "object"},
	})
}

func fieldSchema() json.RawMessage {
	return schema([]string{"metadata_version_id", "object_id", "api_name", "label", "data_type"}, map[string]any{
		"metadata_version_id": uuidProperty(), "field_id": uuidProperty(), "object_id": uuidProperty(), "api_name": apiNameProperty(),
		"label": stringProperty(), "description": map[string]any{"type": "string"},
		"data_type": map[string]any{"type": "string", "enum": []string{"text", "number", "boolean", "date", "datetime", "uuid", "json"}},
		"required":  map[string]any{"type": "boolean"}, "indexed": map[string]any{"type": "boolean"}, "unique_value": map[string]any{"type": "boolean"},
		"lifecycle_state":      map[string]any{"type": "string", "enum": []string{"active", "deprecated_read_write", "deprecated_read_only", "hidden", "purging", "tombstone"}},
		"index_state":          map[string]any{"type": "string", "enum": []string{"none", "building", "validating", "active", "failed", "retiring"}},
		"default_semantics":    map[string]any{"type": "string", "enum": []string{"on_create", "backfill_required"}},
		"predecessor_field_id": uuidProperty(),
		"default_value":        map[string]any{}, "constraints": map[string]any{"type": "object"}, "semantic": map[string]any{"type": "object"},
	})
}

func relationSchema() json.RawMessage {
	return schema([]string{"metadata_version_id", "api_name", "source_object_id", "target_object_id", "relation_type", "delete_behavior"}, map[string]any{
		"metadata_version_id": uuidProperty(), "relation_id": uuidProperty(), "api_name": apiNameProperty(),
		"source_object_id": uuidProperty(), "target_object_id": uuidProperty(),
		"relation_type":   map[string]any{"type": "string", "enum": []string{"lookup", "master_detail", "many_to_many"}},
		"delete_behavior": map[string]any{"type": "string", "enum": []string{"restrict", "cascade", "set_null"}},
		"description":     map[string]any{"type": "string"}, "semantic": map[string]any{"type": "object"},
	})
}

func genericOutputSchema() json.RawMessage {
	return json.RawMessage("{\"type\":\"object\"}")
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

func uuidProperty() map[string]any {
	return map[string]any{"type": "string", "format": "uuid"}
}

func apiNameProperty() map[string]any {
	return map[string]any{"type": "string", "pattern": "^[a-z][a-z0-9_]{0,95}$"}
}

func stringProperty() map[string]any {
	return map[string]any{"type": "string", "minLength": 1}
}
