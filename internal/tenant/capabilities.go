package tenant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

func CapabilityDefinitions(service *Service, includeProvision ...bool) []capability.Definition {
	if service == nil {
		panic("tenant capability definitions require service")
	}
	definitions := []capability.Definition{
		definition("tenant.get-status", "Get the Native product lifecycle, route and entitlement projection.", "low", "tenant.status.read", emptySchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input struct{}
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.GetStatus(ctx, request)
		}),
		definition("tenant.suspend", "Suspend Native product access without affecting another product.", "medium", "tenant.lifecycle.write", lifecycleSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input LifecycleInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.Suspend(ctx, request, input)
		}),
		definition("tenant.resume", "Resume Native product access when the global lifecycle allows it.", "medium", "tenant.lifecycle.write", lifecycleSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input LifecycleInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.Resume(ctx, request, input)
		}),
		definition("tenant.update-entitlement", "Update the versioned Native service-tier entitlement projection.", "medium", "tenant.entitlement.write", entitlementSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input EntitlementInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.UpdateEntitlement(ctx, request, input)
		}),
		definition("tenant.decommission", "Request asynchronous, independently approved Native product decommissioning.", "high", "tenant.decommission", lifecycleSchema(), capability.ExecutionPolicy{Mode: capability.ExecutionAsynchronous, ApprovalRequired: true}, func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input LifecycleInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.RequestDecommission(ctx, request, input)
		}),
	}
	directProvision := true
	if len(includeProvision) > 0 {
		directProvision = includeProvision[0]
	}
	if directProvision {
		definitions = append([]capability.Definition{definition("tenant.provision", "Provision the Native product projection for an operations-owned global tenant.", "medium", "tenant.provision", provisionSchema(), synchronous(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ProvisionInput
			if stableErr := decodeInput(request.Input, &input); stableErr != nil {
				return nil, stableErr
			}
			return service.Provision(ctx, request, input)
		})}, definitions...)
	}
	return definitions
}

type tenantHandler func(context.Context, capability.Request) (any, *capability.StableError)

func definition(id, description, risk, scope string, inputSchema json.RawMessage, execution capability.ExecutionPolicy, handler tenantHandler) capability.Definition {
	return capability.Definition{
		Descriptor: capability.Descriptor{
			ID:            id,
			Version:       "v2",
			Description:   description,
			RiskLevel:     risk,
			State:         capability.PublicationPublished,
			RequiredScope: scope,
			InputSchema:   inputSchema,
			OutputSchema:  statusSchema(),
			Idempotency:   capability.IdempotencyPolicy{Enabled: true},
			Execution:     execution,
		},
		ValidateInput: func(raw json.RawMessage) *capability.StableError {
			var value map[string]json.RawMessage
			if stableErr := decodeInput(raw, &value); stableErr != nil {
				return stableErr
			}
			if value == nil {
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

func provisionSchema() json.RawMessage {
	return schema(
		[]string{"operation_id", "tenant_id", "company_id", "tenant_revision", "product_revision", "display_name", "service_tier", "global_lifecycle_status"},
		map[string]any{
			"operation_id":            stringProperty(),
			"tenant_id":               map[string]any{"type": "string", "format": "uuid"},
			"company_id":              map[string]any{"type": "string", "pattern": "^org[a-z0-9]{17}$"},
			"tenant_revision":         positiveIntegerProperty(),
			"product_revision":        positiveIntegerProperty(),
			"display_name":            stringProperty(),
			"service_tier":            stringProperty(),
			"global_lifecycle_status": map[string]any{"type": "string", "enum": []string{"active"}},
			"entitlements":            map[string]any{"type": "object"},
		},
	)
}

func lifecycleSchema() json.RawMessage {
	return schema([]string{"operation_id", "product_revision"}, map[string]any{
		"operation_id":     stringProperty(),
		"product_revision": positiveIntegerProperty(),
	})
}

func entitlementSchema() json.RawMessage {
	return schema([]string{"operation_id", "product_revision", "entitlements"}, map[string]any{
		"operation_id":     stringProperty(),
		"product_revision": positiveIntegerProperty(),
		"entitlements":     map[string]any{"type": "object"},
	})
}

func statusSchema() json.RawMessage {
	return schema([]string{"tenant_id", "company_id", "shard_id", "tenant_bucket", "native_status", "tenant_revision", "product_revision", "route_revision"}, map[string]any{
		"tenant_id":               map[string]any{"type": "string", "format": "uuid"},
		"company_id":              map[string]any{"type": "string"},
		"shard_id":                map[string]any{"type": "string"},
		"tenant_bucket":           map[string]any{"type": "integer", "minimum": 0, "maximum": 127},
		"service_tier":            map[string]any{"type": "string"},
		"global_lifecycle_status": map[string]any{"type": "string"},
		"native_status":           map[string]any{"type": "string"},
		"tenant_revision":         positiveIntegerProperty(),
		"product_revision":        positiveIntegerProperty(),
		"route_revision":          positiveIntegerProperty(),
		"entitlements":            map[string]any{"type": "object"},
		"last_operation_id":       map[string]any{"type": "string"},
		"operation_status":        map[string]any{"type": "string"},
	})
}

func schema(required []string, properties map[string]any) json.RawMessage {
	encoded, err := json.Marshal(map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"additionalProperties": false,
		"required":             required,
		"properties":           properties,
	})
	if err != nil {
		panic(fmt.Sprintf("marshal tenant capability schema: %v", err))
	}
	return encoded
}

func stringProperty() map[string]any {
	return map[string]any{"type": "string", "minLength": 1}
}

func positiveIntegerProperty() map[string]any {
	return map[string]any{"type": "integer", "minimum": 1}
}
