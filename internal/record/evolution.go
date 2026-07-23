package record

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/governance"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func pendingEvolutionProjection(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, baseVersionID uuid.UUID, apiName string, source map[string]any) (objectModel, map[string]any, bool, error) {
	var candidateID uuid.UUID
	err := tx.QueryRow(ctx,
		"select candidate_metadata_version_id from metadata_changeset where base_metadata_version_id=$1 and requires_backfill and state in ('approved','backfilling','ready') order by updated_at desc limit 1",
		baseVersionID,
	).Scan(&candidateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return objectModel{}, nil, false, nil
	}
	if err != nil {
		return objectModel{}, nil, false, err
	}
	model, err := loadEvolutionModel(ctx, tx, candidateID, apiName)
	if err != nil {
		return objectModel{}, nil, false, err
	}
	projected := cloneObject(source)
	fieldsByID := make(map[string]fieldSpec, len(model.Fields))
	for _, field := range model.Fields {
		fieldsByID[field.FieldID] = field
	}
	for name, field := range model.Fields {
		if field.LifecycleState == "purging" || field.LifecycleState == "tombstone" {
			delete(projected, name)
			continue
		}
		if _, exists := projected[name]; exists {
			continue
		}
		if field.PredecessorFieldID != "" {
			if predecessor, exists := fieldsByID[field.PredecessorFieldID]; exists {
				if value, present := projected[predecessor.APIName]; present {
					converted, stableErr := convertEvolutionValue(field, value)
					if stableErr != nil {
						return objectModel{}, nil, false, nil
					}
					projected[name] = converted
					continue
				}
			}
		}
		if field.DefaultSemantics == "backfill_required" && len(field.DefaultValue) > 0 {
			value, err := decodeJSONValue(field.DefaultValue)
			if err != nil {
				return objectModel{}, nil, false, err
			}
			projected[name] = value
		}
	}
	normalized, stableErr := normalizeRecordData(model, projected, map[string]any{}, false)
	if stableErr != nil {
		return objectModel{}, nil, false, nil
	}
	if stableErr := enforcePolicyData(ctx, tx, tenant, model, normalized); stableErr != nil {
		return objectModel{}, nil, false, nil
	}
	return model, normalized, true, nil
}

func activeDataOnly(model objectModel, source map[string]any) map[string]any {
	result := make(map[string]any, len(model.Fields)+len(model.Relations))
	for name := range model.Fields {
		if value, exists := source[name]; exists {
			result[name] = value
		}
	}
	for name := range model.Relations {
		if value, exists := source[name]; exists {
			result[name] = value
		}
	}
	return result
}

func mergeActiveData(model objectModel, original, active map[string]any) map[string]any {
	result := cloneObject(original)
	for name := range model.Fields {
		delete(result, name)
	}
	for name := range model.Relations {
		delete(result, name)
	}
	for name, value := range active {
		result[name] = value
	}
	return result
}

func convertEvolutionValue(target fieldSpec, value any) (any, *capability.StableError) {
	return normalizeFieldValue(target, value)
}

func enforcePolicyData(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, model objectModel, data map[string]any) *capability.StableError {
	var serviceTier string
	if err := tx.QueryRow(ctx, "select service_tier from tenant_registry where tenant_id=$1", tenant.TenantID).Scan(&serviceTier); err != nil {
		return internalError()
	}
	policy, err := governance.LoadPolicy(ctx, tx, serviceTier)
	if err != nil {
		return internalError()
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return internalError()
	}
	if len(encoded) > policy.MaxRecordJSONBytes {
		return validationError(fmt.Sprintf("record data exceeds tenant policy limit of %d bytes", policy.MaxRecordJSONBytes))
	}
	for name, field := range model.Fields {
		if field.DataType != "json" {
			continue
		}
		value, exists := data[name]
		if !exists || value == nil {
			continue
		}
		fieldJSON, err := json.Marshal(value)
		if err != nil {
			return validationError(name + " must be valid JSON")
		}
		if len(fieldJSON) > policy.MaxJSONFieldBytes {
			return validationError(fmt.Sprintf("%s exceeds tenant JSON field limit of %d bytes", name, policy.MaxJSONFieldBytes))
		}
		if err := validatePolicyJSONShape(value, 1, policy); err != nil {
			return validationError(name + " " + err.Error())
		}
	}
	return nil
}

func validatePolicyJSONShape(value any, depth int, policy governance.Policy) error {
	if depth > policy.MaxJSONDepth {
		return fmt.Errorf("exceeds tenant JSON depth limit of %d", policy.MaxJSONDepth)
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			if err := validatePolicyJSONShape(child, depth+1, policy); err != nil {
				return err
			}
		}
	case []any:
		if len(typed) > policy.MaxJSONArrayElements {
			return fmt.Errorf("exceeds tenant JSON array limit of %d", policy.MaxJSONArrayElements)
		}
		for _, child := range typed {
			if err := validatePolicyJSONShape(child, depth+1, policy); err != nil {
				return err
			}
		}
	}
	return nil
}
