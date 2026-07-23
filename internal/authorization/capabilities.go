package authorization

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

func CapabilityDefinitions(service *Service) []capability.Definition {
	if service == nil {
		panic("authorization capability definitions require service")
	}
	definitions := []capability.Definition{
		definition("authorization.role.create", "Create a tenant business role after platform and RBAC authorization.", "medium", "authorization.manage", roleSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input CreateRoleInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.CreateRole(ctx, request, input)
		}),
		definition("authorization.permission-set.create", "Create a reusable tenant permission set.", "medium", "authorization.manage", permissionSetSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input CreatePermissionSetInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.CreatePermissionSet(ctx, request, input)
		}),
		definition("authorization.permission-set.grant", "Attach an atomic permission to a permission set.", "high", "authorization.manage", permissionGrantSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input GrantPermissionInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			if err := service.GrantPermission(ctx, request, input); err != nil {
				return nil, err
			}
			return map[string]bool{"granted": true}, nil
		}),
		definition("authorization.role.attach-permission-set", "Attach a reusable permission set to a role.", "high", "authorization.manage", attachSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input AttachPermissionSetInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			if err := service.AttachPermissionSet(ctx, request, input); err != nil {
				return nil, err
			}
			return map[string]bool{"attached": true}, nil
		}),
		definition("authorization.role.assign", "Assign a role to a projected principal after conflict checks.", "high", "authorization.manage", assignSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input AssignRoleInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			if err := service.AssignRole(ctx, request, input); err != nil {
				return nil, err
			}
			return map[string]bool{"assigned": true}, nil
		}),
		definition("authorization.role.revoke", "End an active role assignment and invalidate its authorization snapshot.", "high", "authorization.manage", revokeRoleSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input RevokeRoleInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			if err := service.RevokeRole(ctx, request, input); err != nil {
				return nil, err
			}
			return map[string]bool{"revoked": true}, nil
		}),
		definition("authorization.role.set-data-scope", "Attach an approved bounded data scope to a business role.", "high", "authorization.manage", roleDataScopeSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input SetRoleDataScopeInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			if err := service.SetRoleDataScope(ctx, request, input); err != nil {
				return nil, err
			}
			return map[string]bool{"updated": true}, nil
		}),
		definition("authorization.object-policy.set", "Publish an approved object authorization enforcement policy.", "high", "authorization.manage", objectPolicySchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input SetObjectPolicyInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			if err := service.SetObjectPolicy(ctx, request, input); err != nil {
				return nil, err
			}
			return map[string]bool{"updated": true}, nil
		}),
		definition("record.share.grant", "Grant a direct record share to a projected principal or group.", "high", "record.share.manage", shareGrantSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input GrantShareInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.GrantShare(ctx, request, input)
		}),
		definition("record.share.revoke", "Revoke an active direct record share.", "high", "record.share.manage", revokeSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input RevokeShareInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			if err := service.RevokeShare(ctx, request, input); err != nil {
				return nil, err
			}
			return map[string]bool{"revoked": true}, nil
		}),
		definition("organization.merge.start", "Start an approved, non-destructive organization merge operation.", "high", "organization.manage", organizationMergeStartSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input StartOrganizationMergeInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.StartOrganizationMerge(ctx, request, input)
		}),
		definition("organization.merge.execute", "Process one bounded batch of an approved organization merge.", "high", "organization.manage", organizationMergeExecuteSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ExecuteOrganizationMergeInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.ExecuteOrganizationMerge(ctx, request, input)
		}),
		definition("organization.merge.cancel", "Cancel an unstarted approved organization merge.", "high", "organization.manage", organizationMergeCancelSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input CancelOrganizationMergeInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.CancelOrganizationMerge(ctx, request, input)
		}),
		definition("authorization.group.create", "Create a tenant access group for reusable sharing.", "medium", "authorization.manage", groupSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input CreateGroupInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.CreateGroup(ctx, request, input)
		}),
		definition("authorization.group.set-membership", "Add or end a principal membership in an access group.", "high", "authorization.manage", groupMembershipSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input SetGroupMembershipInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			if err := service.SetGroupMembership(ctx, request, input); err != nil {
				return nil, err
			}
			return map[string]bool{"updated": true}, nil
		}),
		definition("record.team.add-member", "Add a projected principal to a record collaboration team.", "high", "record.share.manage", teamMemberSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input AddTeamMemberInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			if err := service.AddTeamMember(ctx, request, input); err != nil {
				return nil, err
			}
			return map[string]bool{"added": true}, nil
		}),
		definition("record.sharing-rule.upsert", "Create or replace a constrained organization-to-group sharing rule.", "high", "record.share.manage", sharingRuleSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input UpsertSharingRuleInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.UpsertSharingRule(ctx, request, input)
		}),
		definition("record.sharing-rule.refresh", "Project one bounded batch of a sharing rule into record-to-group edges.", "high", "record.share.manage", sharingRuleRefreshSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input RefreshSharingRuleInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.RefreshSharingRule(ctx, request, input)
		}),
		definition("record.sharing-rule.retry", "Restart a failed sharing-rule projection without exposing stale rows.", "high", "record.share.manage", sharingRuleRetrySchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input RetrySharingRuleInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.RetrySharingRule(ctx, request, input)
		}),
		definition("authorization.role.set-conflict", "Declare a symmetric separation-of-duties conflict between two roles.", "high", "authorization.manage", roleConflictSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input SetRoleConflictInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			if err := service.SetRoleConflict(ctx, request, input); err != nil {
				return nil, err
			}
			return map[string]bool{"conflict_set": true}, nil
		}),
		definition("authorization.access.explain", "Explain minimal matching record-access sources to an authorized auditor.", "low", "authorization.read", explainAccessSchema(), func(ctx context.Context, request capability.Request) (any, *capability.StableError) {
			var input ExplainAccessInput
			if err := decodeInput(request.Input, &input); err != nil {
				return nil, err
			}
			return service.ExplainAccess(ctx, request, input)
		}),
	}
	return withPersistentAudit(service, definitions)
}

type handler func(context.Context, capability.Request) (any, *capability.StableError)

// withPersistentAudit records every authorization management invocation in
// the tenant audit ledger. access.explain already creates its richer
// decision-level event inside its transaction, so it is deliberately not
// duplicated here.
func withPersistentAudit(service *Service, definitions []capability.Definition) []capability.Definition {
	for index := range definitions {
		definition := &definitions[index]
		if definition.Descriptor.ID == "authorization.access.explain" {
			continue
		}
		original := definition.Handler
		capabilityID := definition.Descriptor.ID
		definition.Handler = func(ctx context.Context, request capability.Request, view capability.RegistryView) (any, *capability.StableError) {
			result, stableErr := original(ctx, request, view)
			service.persistCapabilityAudit(ctx, request, capabilityID, stableErr)
			return result, stableErr
		}
	}
	return definitions
}

func definition(id, description, risk, scope string, inputSchema json.RawMessage, invoke handler) capability.Definition {
	execution := capability.ExecutionPolicy{Mode: capability.ExecutionSynchronous}
	if risk == "high" {
		execution = capability.ExecutionPolicy{Mode: capability.ExecutionAsynchronous, ApprovalRequired: true}
	}
	return capability.Definition{
		Descriptor: capability.Descriptor{ID: id, Version: "v1", Description: description, RiskLevel: risk, State: capability.PublicationPublished, RequiredScope: scope, InputSchema: inputSchema, OutputSchema: genericOutputSchema(), Idempotency: capability.IdempotencyPolicy{Enabled: true}, Execution: execution},
		ValidateInput: func(raw json.RawMessage) *capability.StableError {
			var value map[string]json.RawMessage
			if err := decodeInput(raw, &value); err != nil {
				return err
			}
			if value == nil {
				return validationError("authorization capability input must be an object")
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
	if err := decoder.Decode(target); err != nil {
		return validationError("invalid authorization capability input: " + err.Error())
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return validationError("authorization capability input must contain exactly one JSON value")
		}
		return validationError("invalid authorization capability input: " + err.Error())
	}
	return nil
}

func genericOutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":true}`)
}
func roleSchema() json.RawMessage {
	return schema([]string{"name"}, map[string]any{"role_id": uuidProperty(), "name": stringProperty(), "description": stringProperty()})
}
func permissionSetSchema() json.RawMessage {
	return schema([]string{"name"}, map[string]any{"permission_set_id": uuidProperty(), "name": stringProperty(), "description": stringProperty()})
}
func permissionGrantSchema() json.RawMessage {
	return schema([]string{"permission_set_id", "resource_type", "resource_ref", "action", "approval_id"}, map[string]any{"permission_set_id": uuidProperty(), "resource_type": enumProperty("platform", "object", "field"), "resource_ref": stringProperty(), "action": stringProperty(), "approval_id": stringProperty()})
}
func attachSchema() json.RawMessage {
	return schema([]string{"role_id", "permission_set_id", "approval_id"}, map[string]any{"role_id": uuidProperty(), "permission_set_id": uuidProperty(), "approval_id": stringProperty()})
}
func assignSchema() json.RawMessage {
	return schema([]string{"principal_id", "role_id", "approval_id"}, map[string]any{"principal_id": stringProperty(), "role_id": uuidProperty(), "expires_at": map[string]any{"type": "string", "format": "date-time"}, "approval_id": stringProperty()})
}
func revokeRoleSchema() json.RawMessage {
	return schema([]string{"principal_id", "role_id", "approval_id"}, map[string]any{"principal_id": stringProperty(), "role_id": uuidProperty(), "approval_id": stringProperty()})
}
func roleDataScopeSchema() json.RawMessage {
	return schema([]string{"role_id", "object_id", "action", "scope_type", "approval_id"}, map[string]any{
		"role_id":         uuidProperty(),
		"object_id":       uuidProperty(),
		"action":          enumProperty("read", "update", "delete"),
		"scope_type":      enumProperty("own", "organization", "organization_descendants", "assigned_organizations", "all_tenant", "conditional"),
		"organization_id": uuidProperty(),
		"condition": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"equals": map[string]any{
					"type":                 "object",
					"minProperties":        1,
					"maxProperties":        5,
					"additionalProperties": map[string]any{"type": []string{"string", "number", "boolean"}},
				},
			},
		},
		"approval_id": stringProperty(),
	})
}
func objectPolicySchema() json.RawMessage {
	return schema([]string{"object_id", "enforcement_state", "default_record_access", "approval_id"}, map[string]any{"object_id": uuidProperty(), "enforcement_state": enumProperty("disabled", "enforced"), "default_record_access": enumProperty("private", "read_all"), "approval_id": stringProperty()})
}
func shareGrantSchema() json.RawMessage {
	return schema([]string{"object_id", "record_id", "grantee_type", "grantee_ref", "access_level", "approval_id"}, map[string]any{"share_grant_id": uuidProperty(), "object_id": uuidProperty(), "record_id": uuidProperty(), "grantee_type": enumProperty("principal", "group"), "grantee_ref": stringProperty(), "access_level": enumProperty("read", "update", "delete"), "approval_id": stringProperty()})
}
func revokeSchema() json.RawMessage {
	return schema([]string{"share_grant_id", "approval_id"}, map[string]any{"share_grant_id": uuidProperty(), "approval_id": stringProperty()})
}
func organizationMergeStartSchema() json.RawMessage {
	return schema([]string{"source_organization_id", "target_organization_id", "approval_id"}, map[string]any{"source_organization_id": uuidProperty(), "target_organization_id": uuidProperty(), "approval_id": stringProperty()})
}
func organizationMergeExecuteSchema() json.RawMessage {
	return schema([]string{"operation_id", "approval_id"}, map[string]any{"operation_id": uuidProperty(), "batch_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000}, "approval_id": stringProperty()})
}
func organizationMergeCancelSchema() json.RawMessage {
	return schema([]string{"operation_id", "approval_id"}, map[string]any{"operation_id": uuidProperty(), "approval_id": stringProperty()})
}
func groupSchema() json.RawMessage {
	return schema([]string{"name"}, map[string]any{"group_id": uuidProperty(), "name": stringProperty(), "group_type": enumProperty("manual", "rule")})
}
func groupMembershipSchema() json.RawMessage {
	return schema([]string{"group_id", "principal_id", "active", "approval_id"}, map[string]any{"group_id": uuidProperty(), "principal_id": stringProperty(), "active": map[string]any{"type": "boolean"}, "approval_id": stringProperty()})
}
func teamMemberSchema() json.RawMessage {
	return schema([]string{"object_id", "record_id", "principal_id", "access_level", "approval_id"}, map[string]any{"object_id": uuidProperty(), "record_id": uuidProperty(), "principal_id": stringProperty(), "access_level": enumProperty("read", "update", "delete"), "approval_id": stringProperty()})
}
func sharingRuleSchema() json.RawMessage {
	return schema([]string{"object_id", "name", "data_organization_id", "grantee_group_id", "access_level", "approval_id"}, map[string]any{"rule_id": uuidProperty(), "object_id": uuidProperty(), "name": stringProperty(), "data_organization_id": uuidProperty(), "grantee_group_id": uuidProperty(), "access_level": enumProperty("read", "update", "delete"), "approval_id": stringProperty()})
}
func sharingRuleRefreshSchema() json.RawMessage {
	return schema([]string{"rule_id", "approval_id"}, map[string]any{"rule_id": uuidProperty(), "batch_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000}, "approval_id": stringProperty()})
}
func sharingRuleRetrySchema() json.RawMessage {
	return schema([]string{"rule_id", "approval_id"}, map[string]any{"rule_id": uuidProperty(), "approval_id": stringProperty()})
}
func roleConflictSchema() json.RawMessage {
	return schema([]string{"role_id", "conflicting_role_id", "approval_id"}, map[string]any{"role_id": uuidProperty(), "conflicting_role_id": uuidProperty(), "approval_id": stringProperty()})
}
func explainAccessSchema() json.RawMessage {
	return schema([]string{"principal_id", "object_id", "record_id", "action"}, map[string]any{"principal_id": stringProperty(), "object_id": uuidProperty(), "record_id": uuidProperty(), "action": enumProperty("read", "update", "delete")})
}
func schema(required []string, properties map[string]any) json.RawMessage {
	encoded, err := json.Marshal(map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "additionalProperties": false, "required": required, "properties": properties})
	if err != nil {
		panic(err)
	}
	return encoded
}
func uuidProperty() map[string]any { return map[string]any{"type": "string", "format": "uuid"} }
func stringProperty() map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "maxLength": 200}
}
func enumProperty(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}
