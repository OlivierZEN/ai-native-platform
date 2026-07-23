package record

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/governance"
	"github.com/google/uuid"
)

func decodeJSONObject(raw json.RawMessage, field string) (map[string]any, *capability.StableError) {
	if len(raw) == 0 {
		return nil, validationError(field + " is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, validationError(field + " must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, validationError(field + " must contain exactly one JSON value")
		}
		return nil, validationError(field + " contains invalid trailing JSON")
	}
	return value, nil
}

func normalizeRecordData(model objectModel, current, changes map[string]any, creating bool) (map[string]any, *capability.StableError) {
	result := cloneObject(current)
	if creating {
		result = make(map[string]any, len(model.Fields)+len(model.Relations))
		for name, field := range model.Fields {
			if len(field.DefaultValue) == 0 || !fieldAcceptsSystemValue(field) {
				continue
			}
			value, err := decodeJSONValue(field.DefaultValue)
			if err != nil {
				return nil, internalError()
			}
			result[name] = value
		}
	}
	for name, value := range changes {
		field, fieldExists := model.Fields[name]
		relation, relationExists := model.Relations[name]
		if !fieldExists && !relationExists {
			return nil, validationError("unknown record property: " + name)
		}
		if value == nil {
			if fieldExists && !fieldIsWritable(field) {
				return nil, preconditionError("field is not writable in lifecycle state " + field.LifecycleState + ": " + name)
			}
			delete(result, name)
			continue
		}
		if fieldExists {
			if !fieldIsWritable(field) {
				return nil, preconditionError("field is not writable in lifecycle state " + field.LifecycleState + ": " + name)
			}
			normalized, stableErr := normalizeFieldValue(field, value)
			if stableErr != nil {
				return nil, stableErr
			}
			result[name] = normalized
			continue
		}
		normalized, stableErr := normalizeRelationValue(relation, value)
		if stableErr != nil {
			return nil, stableErr
		}
		result[name] = normalized
	}
	for name, value := range cloneObject(result) {
		if field, exists := model.Fields[name]; exists {
			normalized, stableErr := normalizeFieldValue(field, value)
			if stableErr != nil {
				return nil, stableErr
			}
			result[name] = normalized
			continue
		}
		if relation, exists := model.Relations[name]; exists {
			normalized, stableErr := normalizeRelationValue(relation, value)
			if stableErr != nil {
				return nil, stableErr
			}
			result[name] = normalized
			continue
		}
		return nil, preconditionError("record property is not present in the current metadata: " + name)
	}
	for name, field := range model.Fields {
		if field.Required {
			if value, exists := result[name]; !exists || value == nil {
				return nil, validationError("required field is missing: " + name)
			}
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, internalError()
	}
	if len(encoded) > governance.MaxRecordJSONBytes {
		return nil, validationError(fmt.Sprintf("record data exceeds the %d byte JSONB application limit", governance.MaxRecordJSONBytes))
	}
	return result, nil
}

func normalizeFieldValue(field fieldSpec, value any) (any, *capability.StableError) {
	var normalized any
	switch field.DataType {
	case "text":
		text, ok := value.(string)
		if !ok {
			return nil, validationError(field.APIName + " must be text")
		}
		normalized = text
	case "number":
		number, ok := canonicalNumber(value)
		if !ok {
			return nil, validationError(field.APIName + " must be a number")
		}
		normalized = json.Number(number)
	case "boolean":
		boolean, ok := value.(bool)
		if !ok {
			return nil, validationError(field.APIName + " must be a boolean")
		}
		normalized = boolean
	case "date":
		text, ok := value.(string)
		if !ok {
			return nil, validationError(field.APIName + " must be an ISO date")
		}
		parsed, err := time.Parse("2006-01-02", text)
		if err != nil {
			return nil, validationError(field.APIName + " must be an ISO date")
		}
		normalized = parsed.Format("2006-01-02")
	case "datetime":
		text, ok := value.(string)
		if !ok {
			return nil, validationError(field.APIName + " must be an RFC3339 datetime")
		}
		parsed, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return nil, validationError(field.APIName + " must be an RFC3339 datetime")
		}
		normalized = parsed.UTC().Format(time.RFC3339Nano)
	case "uuid":
		text, ok := value.(string)
		parsed, err := uuid.Parse(text)
		if !ok || err != nil || parsed == uuid.Nil {
			return nil, validationError(field.APIName + " must be a UUID")
		}
		normalized = parsed.String()
	case "json":
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, validationError(field.APIName + " must be valid JSON")
		}
		if len(encoded) > governance.MaxJSONFieldBytes {
			return nil, validationError(fmt.Sprintf("%s exceeds the %d byte JSON field limit", field.APIName, governance.MaxJSONFieldBytes))
		}
		if stableErr := validateJSONShape(field.APIName, value, 1); stableErr != nil {
			return nil, stableErr
		}
		normalized = value
	default:
		return nil, preconditionError("unsupported metadata field type: " + field.DataType)
	}
	if stableErr := validateFieldConstraints(field, normalized); stableErr != nil {
		return nil, stableErr
	}
	return normalized, nil
}

func validateFieldConstraints(field fieldSpec, value any) *capability.StableError {
	if maxLength, ok := integerConstraint(field.Constraints, "max_length"); ok && field.DataType == "text" {
		if len([]rune(value.(string))) > maxLength {
			return validationError(fmt.Sprintf("%s exceeds max_length", field.APIName))
		}
	}
	if minLength, ok := integerConstraint(field.Constraints, "min_length"); ok && field.DataType == "text" {
		if len([]rune(value.(string))) < minLength {
			return validationError(fmt.Sprintf("%s is shorter than min_length", field.APIName))
		}
	}
	return nil
}

func normalizeRelationValue(relation relationSpec, value any) (any, *capability.StableError) {
	normalizeID := func(raw any) (string, bool) {
		text, ok := raw.(string)
		if !ok {
			return "", false
		}
		parsed, err := uuid.Parse(text)
		return parsed.String(), err == nil && parsed != uuid.Nil
	}
	if relation.RelationType == "many_to_many" {
		var values []any
		switch typed := value.(type) {
		case []any:
			values = typed
		case []string:
			values = make([]any, len(typed))
			for index := range typed {
				values[index] = typed[index]
			}
		default:
			return nil, validationError(relation.APIName + " must be an array of record UUIDs")
		}
		result := make([]string, 0, len(values))
		seen := make(map[string]struct{}, len(values))
		for _, raw := range values {
			id, valid := normalizeID(raw)
			if !valid {
				return nil, validationError(relation.APIName + " contains an invalid record UUID")
			}
			if _, duplicate := seen[id]; !duplicate {
				seen[id] = struct{}{}
				result = append(result, id)
			}
		}
		return result, nil
	}
	id, valid := normalizeID(value)
	if !valid {
		return nil, validationError(relation.APIName + " must be a record UUID")
	}
	return id, nil
}

func normalizeFilter(field fieldSpec, input FilterInput) (normalizedFilter, *capability.StableError) {
	if !field.Indexed || field.IndexState != "active" {
		return normalizedFilter{}, preconditionError("filter field has no active typed index: " + field.APIName)
	}
	value, err := decodeJSONValue(input.Value)
	if err != nil {
		return normalizedFilter{}, validationError("filter value is invalid JSON")
	}
	normalized, stableErr := normalizeFieldValue(field, value)
	if stableErr != nil {
		return normalizedFilter{}, stableErr
	}
	result := normalizedFilter{Field: field, OperatorSQL: operatorSQL(input.Operator)}
	if result.OperatorSQL == "" {
		return normalizedFilter{}, validationError("filter operator is not supported: " + input.Operator)
	}
	switch field.DataType {
	case "text":
		if input.Operator != "eq" && input.Operator != "prefix" {
			return normalizedFilter{}, validationError("text filter supports eq or prefix")
		}
		result.Table, result.ValueColumn, result.Value = "record_index_text", "value_text", normalized
		if input.Operator == "prefix" {
			result.OperatorSQL = "like"
			result.Value = escapeLike(normalized.(string)) + "%"
		}
	case "number":
		if input.Operator == "prefix" {
			return normalizedFilter{}, validationError("number filter does not support prefix")
		}
		result.Table, result.ValueColumn, result.Value = "record_index_number", "value_number", normalized.(json.Number).String()
	case "boolean":
		if input.Operator != "eq" {
			return normalizedFilter{}, validationError("boolean filter supports eq")
		}
		result.Table, result.ValueColumn, result.Value = "record_index_boolean", "value_boolean", normalized
	case "date", "datetime":
		if input.Operator == "prefix" {
			return normalizedFilter{}, validationError("date filter does not support prefix")
		}
		parsed, err := parseDateTime(field.DataType, normalized)
		if err != nil {
			return normalizedFilter{}, validationError(field.APIName + " has an invalid date value")
		}
		result.Table, result.ValueColumn, result.Value = "record_index_datetime", "value_datetime", parsed.UTC()
	case "uuid":
		if input.Operator != "eq" {
			return normalizedFilter{}, validationError("UUID filter supports eq")
		}
		result.Table, result.ValueColumn, result.Value = "record_index_uuid", "value_uuid", normalized
	default:
		return normalizedFilter{}, preconditionError("field type cannot be indexed: " + field.DataType)
	}
	return result, nil
}

func fieldIsWritable(field fieldSpec) bool {
	return field.LifecycleState == "active" || field.LifecycleState == "deprecated_read_write"
}

func fieldAcceptsSystemValue(field fieldSpec) bool {
	return field.LifecycleState != "purging" && field.LifecycleState != "tombstone"
}

func fieldIsVisible(field fieldSpec) bool {
	return field.LifecycleState != "hidden" && field.LifecycleState != "purging" && field.LifecycleState != "tombstone"
}

func validateJSONShape(fieldName string, value any, depth int) *capability.StableError {
	if depth > governance.MaxJSONDepth {
		return validationError(fmt.Sprintf("%s exceeds the maximum JSON depth of %d", fieldName, governance.MaxJSONDepth))
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			if stableErr := validateJSONShape(fieldName, child, depth+1); stableErr != nil {
				return stableErr
			}
		}
	case []any:
		if len(typed) > governance.MaxJSONArrayElements {
			return validationError(fmt.Sprintf("%s exceeds the maximum JSON array length of %d", fieldName, governance.MaxJSONArrayElements))
		}
		for _, child := range typed {
			if stableErr := validateJSONShape(fieldName, child, depth+1); stableErr != nil {
				return stableErr
			}
		}
	}
	return nil
}

func operatorSQL(operator string) string {
	switch operator {
	case "eq":
		return "="
	case "prefix":
		return "like"
	case "gt":
		return ">"
	case "gte":
		return ">="
	case "lt":
		return "<"
	case "lte":
		return "<="
	default:
		return ""
	}
}

func canonicalNumber(value any) (string, bool) {
	var raw string
	switch typed := value.(type) {
	case json.Number:
		raw = typed.String()
	case float64:
		raw = fmt.Sprintf("%v", typed)
	case float32:
		raw = fmt.Sprintf("%v", typed)
	case int:
		raw = fmt.Sprintf("%d", typed)
	case int64:
		raw = fmt.Sprintf("%d", typed)
	default:
		return "", false
	}
	if _, ok := new(big.Rat).SetString(raw); !ok {
		return "", false
	}
	return raw, true
}

func decodeJSONValue(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}

func cloneObject(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func integerConstraint(constraints map[string]any, name string) (int, bool) {
	if constraints == nil {
		return 0, false
	}
	value, exists := constraints[name]
	if !exists {
		return 0, false
	}
	number, ok := canonicalNumber(value)
	if !ok || strings.Contains(number, ".") {
		return 0, false
	}
	var parsed int
	if _, err := fmt.Sscanf(number, "%d", &parsed); err != nil || parsed < 0 {
		return 0, false
	}
	return parsed, true
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}
