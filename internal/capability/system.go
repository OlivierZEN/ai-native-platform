package capability

import (
	"context"
	"encoding/json"
)

var listInputSchema = json.RawMessage(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "additionalProperties":false,
  "properties":{"include_deprecated":{"type":"boolean"}}
}`)

var listOutputSchema = json.RawMessage(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "additionalProperties":false,
  "required":["capabilities"],
  "properties":{"capabilities":{"type":"array","items":{"type":"object"}}}
}`)

func SystemCapabilityDefinitions() []Definition {
	return []Definition{
		{
			Descriptor: Descriptor{
				ID:            "system.capability.list",
				Version:       "v1",
				Description:   "List published low-risk capabilities visible to the actor.",
				RiskLevel:     "low",
				State:         PublicationPublished,
				RequiredScope: "system.capability.read",
				InputSchema:   listInputSchema,
				OutputSchema:  listOutputSchema,
				Idempotency:   IdempotencyPolicy{Enabled: true},
				Execution:     ExecutionPolicy{Mode: ExecutionSynchronous},
			},
			ValidateInput: validateListInput,
			Handler: func(_ context.Context, _ Request, registry RegistryView) (any, *StableError) {
				return ListResult{Capabilities: registry.Descriptors()}, nil
			},
		},
	}
}

func validateListInput(raw json.RawMessage) *StableError {
	var input map[string]json.RawMessage
	if err := json.Unmarshal(raw, &input); err != nil || input == nil {
		return &StableError{Code: CodeValidationFailed, Message: "input must be a JSON object"}
	}
	if rawDeprecated, ok := input["include_deprecated"]; ok {
		var includeDeprecated bool
		if err := json.Unmarshal(rawDeprecated, &includeDeprecated); err != nil {
			return &StableError{Code: CodeValidationFailed, Message: "include_deprecated must be a boolean"}
		}
	}
	for key := range input {
		if key != "include_deprecated" {
			return &StableError{Code: CodeValidationFailed, Message: "input contains an unknown property"}
		}
	}
	return nil
}
