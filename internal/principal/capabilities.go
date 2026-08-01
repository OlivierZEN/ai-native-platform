package principal

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

func CapabilityDefinitions(service *Service) []capability.Definition {
	if service == nil {
		panic("principal capability definitions require service")
	}
	return []capability.Definition{
		definition("identity.principal.sync", "Project the current verified AgentCiCi Principal into this tenant.", "medium", "identity.principal.sync", syncSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input SyncInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.Sync(ctx, request, input)
		}),
		definition("identity.principal.list", "List governed Principal projections in the current tenant.", "low", "authorization.manage", listSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ListInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.List(ctx, request, input)
		}),
		definition("identity.principal.set-status", "Suspend, activate or disable another tenant Principal projection.", "high", "authorization.manage", statusSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input SetStatusInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.SetStatus(ctx, request, input)
		}),
	}
}

type handler func(context.Context, capability.Request) (any, *capability.StableError)

func definition(id, description, risk, scope string, inputSchema json.RawMessage, invoke handler) capability.Definition {
	execution := capability.ExecutionPolicy{Mode: capability.ExecutionSynchronous}
	if risk == "high" {
		execution = capability.ExecutionPolicy{Mode: capability.ExecutionAsynchronous, ApprovalRequired: true}
	}
	return capability.Definition{
		Descriptor: capability.Descriptor{
			ID: id, Version: "v1", Description: description, RiskLevel: risk,
			State: capability.PublicationPublished, RequiredScope: scope,
			InputSchema:  inputSchema,
			OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":true}`),
			Idempotency:  capability.IdempotencyPolicy{Enabled: true},
			Execution:    execution,
		},
		ValidateInput: func(raw json.RawMessage) *capability.StableError {
			var value map[string]json.RawMessage
			return decodeInput(raw, &value)
		},
		Handler: func(ctx context.Context, request capability.Request, _ capability.RegistryView) (any, *capability.StableError) {
			return invoke(ctx, request)
		},
	}
}

func decodeInput(raw json.RawMessage, target any) *capability.StableError {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return validation("invalid principal capability input: " + err.Error())
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return validation("principal capability input must contain exactly one JSON value")
		}
		return validation("invalid principal capability input: " + err.Error())
	}
	return nil
}

func syncSchema() json.RawMessage {
	return json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"display_name":{"type":"string","maxLength":200},"public_id":{"type":"string","maxLength":64}}}`)
}

func listSchema() json.RawMessage {
	return json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"principal_type":{"type":"string","enum":["HUMAN","SERVICE","GROUP"]},"status":{"type":"string","enum":["active","suspended","disabled"]}}}`)
}

func statusSchema() json.RawMessage {
	return json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["principal_id","status","reason","approval_id"],"properties":{"principal_id":{"type":"string","minLength":1,"maxLength":200},"status":{"type":"string","enum":["active","suspended","disabled"]},"reason":{"type":"string","minLength":1,"maxLength":500},"approval_id":{"type":"string","minLength":1,"maxLength":200}}}`)
}
