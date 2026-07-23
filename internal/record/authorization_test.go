package record

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	authz "github.com/OlivierZEN/ai-native-platform/internal/authorization"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/metadata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope(t *testing.T) {
	_, control, runtime := recordTestPools(t)
	metadataService := metadata.NewService(runtime, control)
	recordService := NewService(runtime, control)
	tenantID := "11111111-1111-4111-8111-111111111111"
	companyID := "orgaaaaaaaaaaaaaaaaa"
	owner := recordPrincipal(tenantID, companyID, "rbac-owner")
	owner.Actor.Scopes = append(owner.Actor.Scopes, "authorization.manage", "authorization.read", "record.share.manage", "organization.manage")
	reader := recordPrincipal(tenantID, companyID, "rbac-reader")
	denied := recordPrincipal(tenantID, companyID, "rbac-denied")
	authorizationService := authz.NewService(runtime, control)
	definitions := append(metadata.CapabilityDefinitions(metadataService), CapabilityDefinitions(recordService)...)
	definitions = append(definitions, authz.CapabilityDefinitions(authorizationService)...)
	invoker := capability.NewInvoker(capability.NewRegistry(definitions), 8)
	ids := publishRecordModel(t, invoker, owner)

	organizationA := uuid.New()
	organizationB := uuid.New()
	seedEnforcedRecordAuthorization(t, runtime, owner, reader, ids, organizationA, organizationB)
	seedAuthorizationManagerPermissions(t, runtime, owner)
	approvalID := "approval-record-model"
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.object-policy.set", map[string]any{"object_id": ids.CustomerID.String(), "enforcement_state": "enforced", "default_record_access": "private", "approval_id": approvalID}))

	created := requireRecord(t, invokeRecord(t, invoker, owner, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Scoped customer"},
	}))
	if created.DataOrganizationID != organizationA.String() {
		t.Fatalf("record data organization=%q want=%s", created.DataOrganizationID, organizationA)
	}
	if _, exists := created.Data["amount"]; exists {
		t.Fatalf("field without read permission leaked: %#v", created.Data)
	}
	explanation := invokeRecord(t, invoker, owner, "authorization.access.explain", map[string]any{"principal_id": owner.Actor.ID, "object_id": ids.CustomerID.String(), "record_id": created.RecordID, "action": "read"})
	if explanation.Status != capability.StatusSucceeded {
		t.Fatalf("access explanation=%#v", explanation)
	}
	var explained authz.AccessExplanation
	if err := json.Unmarshal(explanation.Result, &explained); err != nil {
		t.Fatal(err)
	}
	if !explained.Allowed || len(explained.Sources) == 0 {
		t.Fatalf("explanation=%#v", explained)
	}
	assertAdapterResultsEqual(t, invoker, owner, "authorization.access.explain", "", map[string]any{"principal_id": owner.Actor.ID, "object_id": ids.CustomerID.String(), "record_id": created.RecordID, "action": "read"})

	blockedField := invokeRecord(t, invoker, owner, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "No note", "note": "must fail"},
	})
	assertRecordError(t, blockedField, capability.CodeUnauthorized)
	assertAdapterErrorsEqual(t, invoker, owner, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "No note", "note": "must fail"},
	}, capability.CodeUnauthorized)
	assertRecordError(t, invokeRecord(t, invoker, denied, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}), capability.CodeUnauthorized)
	assertAdapterErrorsEqual(t, invoker, denied, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}, capability.CodeUnauthorized)
	expired := recordPrincipal(tenantID, companyID, "rbac-expired")
	grantExpiredRecordRole(t, runtime, expired, ids.CustomerID, ids.CustomerNameID)
	assertRecordError(t, invokeRecord(t, invoker, expired, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}), capability.CodeUnauthorized)
	conditional := recordPrincipal(tenantID, companyID, "rbac-conditional")
	conditionalRoleID := grantConditionalRecordRole(t, runtime, conditional, ids.CustomerID, ids.CustomerNameID)
	conditionalScopeInput := map[string]any{"role_id": conditionalRoleID.String(), "object_id": ids.CustomerID.String(), "action": "read", "scope_type": "conditional", "condition": map[string]any{"equals": map[string]any{"name": "Scoped customer"}}, "approval_id": approvalID}
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.role.set-data-scope", conditionalScopeInput))
	assertAdapterResultsEqual(t, invoker, owner, "authorization.role.set-data-scope", "", conditionalScopeInput)
	conditionalResponse := invokeRecord(t, invoker, conditional, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	})
	if conditionalResponse.Status != capability.StatusSucceeded {
		t.Fatalf("conditional scope get=%#v", conditionalResponse.Error)
	}
	conditionalRecord := requireRecord(t, conditionalResponse)
	if conditionalRecord.RecordID != created.RecordID {
		t.Fatalf("conditional scope did not grant matching record: %#v", conditionalRecord)
	}
	assertAdapterResultsEqual(t, invoker, conditional, "runtime.record.get", "", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	})
	conditionalQuery := invokeRecord(t, invoker, conditional, "runtime.record.query", map[string]any{"object_api_name": "customer", "limit": 10})
	if conditionalQuery.Status != capability.StatusSucceeded {
		t.Fatalf("conditional scope query=%#v", conditionalQuery)
	}
	var conditionalQueryResult QueryResult
	if err := json.Unmarshal(conditionalQuery.Result, &conditionalQueryResult); err != nil {
		t.Fatal(err)
	}
	if len(conditionalQueryResult.Records) != 1 || conditionalQueryResult.Records[0].RecordID != created.RecordID {
		t.Fatalf("conditional scope query result=%#v", conditionalQueryResult)
	}
	conditionalExplanation := invokeRecord(t, invoker, owner, "authorization.access.explain", map[string]any{"principal_id": conditional.Actor.ID, "object_id": ids.CustomerID.String(), "record_id": created.RecordID, "action": "read"})
	if conditionalExplanation.Status != capability.StatusSucceeded {
		t.Fatalf("conditional access explanation=%#v", conditionalExplanation)
	}
	if err := json.Unmarshal(conditionalExplanation.Result, &explained); err != nil {
		t.Fatal(err)
	}
	if !explained.Allowed || !containsAccessSource(explained.Sources, "conditional_scope") {
		t.Fatalf("conditional explanation=%#v", explained)
	}
	conditionalDenied := requireRecord(t, invokeRecord(t, invoker, owner, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Not conditionally shared"},
	}))
	assertRecordError(t, invokeRecord(t, invoker, conditional, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": conditionalDenied.RecordID,
	}), capability.CodeResourceNotFound)
	assertAdapterErrorsEqual(t, invoker, conditional, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": conditionalDenied.RecordID,
	}, capability.CodeResourceNotFound)

	createdRole := invokeRecord(t, invoker, owner, "authorization.role.create", map[string]any{"name": "temporary reviewer"})
	if createdRole.Status != capability.StatusSucceeded {
		t.Fatalf("create RBAC role=%#v", createdRole)
	}
	var role authz.Role
	if err := json.Unmarshal(createdRole.Result, &role); err != nil {
		t.Fatal(err)
	}
	assertAuthorizationAuditExists(t, runtime, owner, "authorization.role.create", "succeeded")
	createdSet := invokeRecord(t, invoker, owner, "authorization.permission-set.create", map[string]any{"name": "temporary reviewer permissions"})
	if createdSet.Status != capability.StatusSucceeded {
		t.Fatalf("create permission set=%#v", createdSet)
	}
	var permissionSet authz.PermissionSet
	if err := json.Unmarshal(createdSet.Result, &permissionSet); err != nil {
		t.Fatal(err)
	}
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.permission-set.grant", map[string]any{"permission_set_id": permissionSet.PermissionSetID, "resource_type": "platform", "resource_ref": "audit", "action": "read", "approval_id": approvalID}))
	assertRecordError(t, invokeRecord(t, invoker, owner, "authorization.permission-set.grant", map[string]any{"permission_set_id": permissionSet.PermissionSetID, "resource_type": "platform", "resource_ref": "unowned.permission", "action": "read", "approval_id": approvalID}), capability.CodeUnauthorized)
	assertAuthorizationAuditExists(t, runtime, owner, "authorization.permission-set.grant", "failed")
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.role.attach-permission-set", map[string]any{"role_id": role.RoleID, "permission_set_id": permissionSet.PermissionSetID, "approval_id": approvalID}))
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.role.assign", map[string]any{"principal_id": "rbac-reader", "role_id": role.RoleID, "expires_at": "2099-01-01T00:00:00Z", "approval_id": approvalID}))
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.role.set-data-scope", map[string]any{"role_id": role.RoleID, "object_id": ids.CustomerID.String(), "action": "read", "scope_type": "organization_descendants", "organization_id": organizationA.String(), "approval_id": approvalID}))
	assertAdapterResultsEqual(t, invoker, owner, "authorization.role.set-data-scope", "", map[string]any{"role_id": role.RoleID, "object_id": ids.CustomerID.String(), "action": "read", "scope_type": "organization_descendants", "organization_id": organizationA.String(), "approval_id": approvalID})
	assertAdapterErrorsEqual(t, invoker, owner, "authorization.role.set-data-scope", map[string]any{"role_id": role.RoleID, "object_id": ids.CustomerID.String(), "action": "read", "scope_type": "conditional", "condition": map[string]any{"equals": map[string]any{"name": map[string]any{"nested": true}}}, "approval_id": approvalID}, capability.CodeValidationFailed)
	secondRoleResponse := invokeRecord(t, invoker, owner, "authorization.role.create", map[string]any{"name": "conflicting reviewer"})
	if secondRoleResponse.Status != capability.StatusSucceeded {
		t.Fatalf("create conflict role=%#v", secondRoleResponse)
	}
	var secondRole authz.Role
	if err := json.Unmarshal(secondRoleResponse.Result, &secondRole); err != nil {
		t.Fatal(err)
	}
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.role.set-conflict", map[string]any{"role_id": role.RoleID, "conflicting_role_id": secondRole.RoleID, "approval_id": approvalID}))
	assertRecordError(t, invokeRecord(t, invoker, owner, "authorization.role.assign", map[string]any{"principal_id": "rbac-reader", "role_id": secondRole.RoleID, "approval_id": approvalID}), capability.CodeUnauthorized)
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.role.revoke", map[string]any{"principal_id": "rbac-reader", "role_id": role.RoleID, "approval_id": approvalID}))
	groupResponse := invokeRecord(t, invoker, owner, "authorization.group.create", map[string]any{"name": "temporary legal group"})
	if groupResponse.Status != capability.StatusSucceeded {
		t.Fatalf("create access group=%#v", groupResponse)
	}
	var group authz.AccessGroup
	if err := json.Unmarshal(groupResponse.Result, &group); err != nil {
		t.Fatal(err)
	}
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.group.set-membership", map[string]any{"group_id": group.GroupID, "principal_id": "rbac-reader", "active": true, "approval_id": approvalID}))
	ruleResponse := invokeRecord(t, invoker, owner, "record.sharing-rule.upsert", map[string]any{"object_id": ids.CustomerID.String(), "name": "sales-readers", "data_organization_id": organizationA.String(), "grantee_group_id": group.GroupID, "access_level": "read", "approval_id": approvalID})
	if ruleResponse.Status != capability.StatusSucceeded {
		t.Fatalf("upsert sharing rule=%#v", ruleResponse)
	}
	var rule authz.SharingRule
	if err := json.Unmarshal(ruleResponse.Result, &rule); err != nil {
		t.Fatal(err)
	}

	visibleToReader := requireRecord(t, invokeRecord(t, invoker, reader, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}))
	if visibleToReader.RecordID != created.RecordID || visibleToReader.Data["name"] != "Scoped customer" {
		t.Fatalf("organization descendant reader result=%#v", visibleToReader)
	}
	refreshResponse := invokeRecord(t, invoker, owner, "record.sharing-rule.refresh", map[string]any{"rule_id": rule.RuleID, "batch_size": 10, "approval_id": approvalID})
	if refreshResponse.Status != capability.StatusSucceeded {
		t.Fatalf("refresh sharing rule=%#v", refreshResponse)
	}
	if err := json.Unmarshal(refreshResponse.Result, &rule); err != nil {
		t.Fatal(err)
	}
	if rule.ProjectionState != "ready" {
		t.Fatalf("rule projection=%#v", rule)
	}
	shared := recordPrincipal(tenantID, companyID, "rbac-shared")
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.group.set-membership", map[string]any{"group_id": group.GroupID, "principal_id": shared.Actor.ID, "active": true, "approval_id": approvalID}))
	sharedByRule := requireRecord(t, invokeRecord(t, invoker, shared, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}))
	if sharedByRule.RecordID != created.RecordID {
		t.Fatalf("group rule did not grant record access: %#v", sharedByRule)
	}
	setAccessGroupLifecycle(t, runtime, owner, uuid.MustParse(group.GroupID), "disabled")
	assertRecordError(t, invokeRecord(t, invoker, shared, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}), capability.CodeResourceNotFound)
	setAccessGroupLifecycle(t, runtime, owner, uuid.MustParse(group.GroupID), "active")
	requireRecord(t, invokeRecord(t, invoker, shared, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}))
	requireSuccess(t, invokeRecord(t, invoker, owner, "authorization.group.set-membership", map[string]any{"group_id": group.GroupID, "principal_id": shared.Actor.ID, "active": false, "approval_id": approvalID}))
	assertRecordError(t, invokeRecord(t, invoker, shared, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}), capability.CodeResourceNotFound)
	createdAfterRule := requireRecord(t, invokeRecord(t, invoker, owner, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Projected after rule ready"},
	}))
	assertRuleProjectionExists(t, runtime, owner, ids.CustomerID, uuid.MustParse(createdAfterRule.RecordID), uuid.MustParse(rule.RuleID))
	// Rebuild with a one-record batch, then alter a record already behind the
	// projection cursor. The final completeness check must keep the rule in
	// building rather than exposing a partially refreshed projection.
	ruleResponse = invokeRecord(t, invoker, owner, "record.sharing-rule.upsert", map[string]any{"rule_id": rule.RuleID, "object_id": ids.CustomerID.String(), "name": "sales-readers", "data_organization_id": organizationA.String(), "grantee_group_id": group.GroupID, "access_level": "read", "approval_id": approvalID})
	if ruleResponse.Status != capability.StatusSucceeded {
		t.Fatalf("restart sharing rule=%#v", ruleResponse)
	}
	firstRefresh := invokeRecord(t, invoker, owner, "record.sharing-rule.refresh", map[string]any{"rule_id": rule.RuleID, "batch_size": 1, "approval_id": approvalID})
	if firstRefresh.Status != capability.StatusSucceeded {
		t.Fatalf("first refresh sharing rule=%#v", firstRefresh)
	}
	if err := json.Unmarshal(firstRefresh.Result, &rule); err != nil {
		t.Fatal(err)
	}
	if rule.ProjectionState != "building" {
		t.Fatalf("first projection batch=%#v", rule)
	}
	projectedRecordID := projectedRuleRecordID(t, runtime, owner, ids.CustomerID, uuid.MustParse(rule.RuleID))
	moveRecordDataOrganization(t, runtime, owner, ids.CustomerID, projectedRecordID, organizationB)
	moveRecordDataOrganization(t, runtime, owner, ids.CustomerID, projectedRecordID, organizationA)
	catchupRefresh := invokeRecord(t, invoker, owner, "record.sharing-rule.refresh", map[string]any{"rule_id": rule.RuleID, "batch_size": 10, "approval_id": approvalID})
	if catchupRefresh.Status != capability.StatusSucceeded {
		t.Fatalf("catch-up refresh sharing rule=%#v", catchupRefresh)
	}
	if err := json.Unmarshal(catchupRefresh.Result, &rule); err != nil {
		t.Fatal(err)
	}
	if rule.ProjectionState != "building" {
		t.Fatalf("incomplete catch-up was marked ready: %#v", rule)
	}
	finalRefresh := invokeRecord(t, invoker, owner, "record.sharing-rule.refresh", map[string]any{"rule_id": rule.RuleID, "batch_size": 10, "approval_id": approvalID})
	if finalRefresh.Status != capability.StatusSucceeded {
		t.Fatalf("final refresh sharing rule=%#v", finalRefresh)
	}
	if err := json.Unmarshal(finalRefresh.Result, &rule); err != nil {
		t.Fatal(err)
	}
	if rule.ProjectionState != "ready" {
		t.Fatalf("final projection=%#v", rule)
	}
	assertRuleProjectionExists(t, runtime, owner, ids.CustomerID, projectedRecordID, uuid.MustParse(rule.RuleID))
	setRuleProjectionState(t, runtime, owner, uuid.MustParse(rule.RuleID), "failed")
	retryResponse := invokeRecord(t, invoker, owner, "record.sharing-rule.retry", map[string]any{"rule_id": rule.RuleID, "approval_id": approvalID})
	if retryResponse.Status != capability.StatusSucceeded {
		t.Fatalf("retry sharing rule=%#v", retryResponse)
	}
	if err := json.Unmarshal(retryResponse.Result, &rule); err != nil {
		t.Fatal(err)
	}
	if rule.ProjectionState != "building" {
		t.Fatalf("retry rule projection=%#v", rule)
	}

	assertRecordError(t, invokeRecord(t, invoker, shared, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}), capability.CodeResourceNotFound)
	shareResponse := invokeRecord(t, invoker, owner, "record.share.grant", map[string]any{
		"object_id": ids.CustomerID.String(), "record_id": created.RecordID, "grantee_type": "principal", "grantee_ref": shared.Actor.ID, "access_level": "read", "approval_id": approvalID,
	})
	if shareResponse.Status != capability.StatusSucceeded {
		t.Fatalf("grant record share=%#v", shareResponse)
	}
	sharedRecord := requireRecord(t, invokeRecord(t, invoker, shared, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}))
	if sharedRecord.RecordID != created.RecordID {
		t.Fatalf("direct share did not grant record access: %#v", sharedRecord)
	}
	team := recordPrincipal(tenantID, companyID, "rbac-team")
	assertRecordError(t, invokeRecord(t, invoker, team, "runtime.record.get", map[string]any{"object_api_name": "customer", "record_id": created.RecordID}), capability.CodeResourceNotFound)
	requireSuccess(t, invokeRecord(t, invoker, owner, "record.team.add-member", map[string]any{"object_id": ids.CustomerID.String(), "record_id": created.RecordID, "principal_id": team.Actor.ID, "access_level": "read", "approval_id": approvalID}))
	teamRecord := requireRecord(t, invokeRecord(t, invoker, team, "runtime.record.get", map[string]any{"object_api_name": "customer", "record_id": created.RecordID}))
	if teamRecord.RecordID != created.RecordID {
		t.Fatalf("team grant did not grant record access: %#v", teamRecord)
	}

	movePrincipalToOrganization(t, runtime, owner, organizationA, organizationB)
	stillOwned := requireRecord(t, invokeRecord(t, invoker, owner, "runtime.record.get", map[string]any{
		"object_api_name": "customer", "record_id": created.RecordID,
	}))
	if stillOwned.RecordID != created.RecordID || stillOwned.DataOrganizationID != organizationA.String() {
		t.Fatalf("owner move rewrote or hid record: %#v", stillOwned)
	}
	mergeStart := invokeRecord(t, invoker, owner, "organization.merge.start", map[string]any{
		"source_organization_id": organizationB.String(), "target_organization_id": organizationA.String(), "approval_id": approvalID,
	})
	if mergeStart.Status != capability.StatusSucceeded {
		t.Fatalf("start organization merge=%#v", mergeStart)
	}
	var merge authz.OrganizationMergeOperation
	if err := json.Unmarshal(mergeStart.Result, &merge); err != nil {
		t.Fatal(err)
	}
	cancelResponse := invokeRecord(t, invoker, owner, "organization.merge.cancel", map[string]any{"operation_id": merge.OperationID, "approval_id": approvalID})
	if cancelResponse.Status != capability.StatusSucceeded {
		t.Fatalf("cancel organization merge=%#v", cancelResponse)
	}
	if err := json.Unmarshal(cancelResponse.Result, &merge); err != nil {
		t.Fatal(err)
	}
	if merge.State != "cancelled" {
		t.Fatalf("cancelled merge=%#v", merge)
	}
	mergeRecordOne := requireRecord(t, invokeRecord(t, invoker, owner, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Merge batch one"},
	}))
	mergeRecordTwo := requireRecord(t, invokeRecord(t, invoker, owner, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Merge batch two"},
	}))
	if mergeRecordOne.DataOrganizationID != organizationB.String() || mergeRecordTwo.DataOrganizationID != organizationB.String() {
		t.Fatalf("merge fixture records were not anchored to source organization: %#v %#v", mergeRecordOne, mergeRecordTwo)
	}
	mergeStart = invokeRecord(t, invoker, owner, "organization.merge.start", map[string]any{
		"source_organization_id": organizationB.String(), "target_organization_id": organizationA.String(), "approval_id": approvalID,
	})
	if mergeStart.Status != capability.StatusSucceeded {
		t.Fatalf("restart organization merge=%#v", mergeStart)
	}
	if err := json.Unmarshal(mergeStart.Result, &merge); err != nil {
		t.Fatal(err)
	}
	assertRecordError(t, invokeRecord(t, invoker, owner, "runtime.record.create", map[string]any{
		"object_api_name": "customer", "data": map[string]any{"name": "Must not be created during merge"},
	}), capability.CodeFailedPrecondition)
	mergeExecute := invokeRecord(t, invoker, owner, "organization.merge.execute", map[string]any{
		"operation_id": merge.OperationID, "batch_size": 1, "approval_id": approvalID,
	})
	if mergeExecute.Status != capability.StatusSucceeded {
		t.Fatalf("execute organization merge=%#v", mergeExecute)
	}
	if err := json.Unmarshal(mergeExecute.Result, &merge); err != nil {
		t.Fatal(err)
	}
	if merge.Completed || merge.State != "running" || merge.RecordsMigrated != 1 {
		t.Fatalf("first merge batch=%#v", merge)
	}
	mergeExecute = invokeRecord(t, invoker, owner, "organization.merge.execute", map[string]any{
		"operation_id": merge.OperationID, "batch_size": 1, "approval_id": approvalID,
	})
	if mergeExecute.Status != capability.StatusSucceeded {
		t.Fatalf("execute final organization merge=%#v", mergeExecute)
	}
	if err := json.Unmarshal(mergeExecute.Result, &merge); err != nil {
		t.Fatal(err)
	}
	if !merge.Completed || merge.State != "completed" || merge.RecordsMigrated != 2 {
		t.Fatalf("merge result=%#v", merge)
	}
	for _, recordID := range []string{mergeRecordOne.RecordID, mergeRecordTwo.RecordID} {
		migrated := requireRecord(t, invokeRecord(t, invoker, owner, "runtime.record.get", map[string]any{"object_api_name": "customer", "record_id": recordID}))
		if migrated.DataOrganizationID != organizationA.String() {
			t.Fatalf("merged record data organization=%s want=%s", migrated.DataOrganizationID, organizationA)
		}
	}
}

func seedEnforcedRecordAuthorization(t *testing.T, runtime *pgxpool.Pool, owner, reader capability.TrustedPrincipal, ids recordModelIDs, organizationA, organizationB uuid.UUID) {
	t.Helper()
	tenantID := uuid.MustParse(owner.TenantID)
	tenant := database.TenantContext{TenantID: tenantID, Bucket: 7, ActorID: owner.Actor.ID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		for _, principalID := range []string{owner.Actor.ID, reader.Actor.ID, "rbac-denied", "rbac-expired", "rbac-conditional", "rbac-shared", "rbac-team"} {
			if _, err := tx.Exec(context.Background(), "insert into principal_projection(tenant_bucket,tenant_id,principal_id,principal_type) values ($1,$2,$3,'user')", tenant.Bucket, tenant.TenantID, principalID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(context.Background(), "insert into organization_node(tenant_bucket,tenant_id,organization_id,parent_organization_id,name) values ($1,$2,$3,null,'Sales'),($1,$2,$4,$3,'Sales East')", tenant.Bucket, tenant.TenantID, organizationA, organizationB); err != nil {
			return err
		}
		if _, err := tx.Exec(context.Background(), "insert into organization_closure(tenant_bucket,tenant_id,ancestor_organization_id,descendant_organization_id,depth) values ($1,$2,$3,$3,0),($1,$2,$4,$4,0),($1,$2,$3,$4,1)", tenant.Bucket, tenant.TenantID, organizationA, organizationB); err != nil {
			return err
		}
		if _, err := tx.Exec(context.Background(), "insert into principal_org_membership(tenant_bucket,tenant_id,membership_id,principal_id,organization_id,is_primary) values ($1,$2,$3,$4,$5,true)", tenant.Bucket, tenant.TenantID, uuid.New(), owner.Actor.ID, organizationA); err != nil {
			return err
		}
		if err := grantRecordRole(context.Background(), tx, tenant, owner.Actor.ID, ids.CustomerID, []string{"create", "read", "update", "delete"}, []fieldGrant{{fieldID: ids.CustomerNameID, action: "read"}, {fieldID: ids.CustomerNameID, action: "write"}}, ""); err != nil {
			return err
		}
		if err := grantRecordRole(context.Background(), tx, tenant, reader.Actor.ID, ids.CustomerID, []string{"read"}, []fieldGrant{{fieldID: ids.CustomerNameID, action: "read"}}, "organization_descendants:"+organizationA.String()); err != nil {
			return err
		}
		if err := grantRecordRole(context.Background(), tx, tenant, "rbac-shared", ids.CustomerID, []string{"read"}, []fieldGrant{{fieldID: ids.CustomerNameID, action: "read"}}, ""); err != nil {
			return err
		}
		if err := grantRecordRole(context.Background(), tx, tenant, "rbac-team", ids.CustomerID, []string{"read"}, []fieldGrant{{fieldID: ids.CustomerNameID, action: "read"}}, ""); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(), "insert into object_authorization_policy(tenant_bucket,tenant_id,object_id,enforcement_state,default_record_access) values ($1,$2,$3,'enforced','private')", tenant.Bucket, tenant.TenantID, ids.CustomerID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func seedAuthorizationManagerPermissions(t *testing.T, runtime *pgxpool.Pool, principal capability.TrustedPrincipal) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		roleID, setID := uuid.New(), uuid.New()
		if _, err := tx.Exec(context.Background(), "insert into authorization_role(tenant_bucket,tenant_id,role_id,name) values ($1,$2,$3,'authorization-manager')", tenant.Bucket, tenant.TenantID, roleID); err != nil {
			return err
		}
		if _, err := tx.Exec(context.Background(), "insert into permission_set(tenant_bucket,tenant_id,permission_set_id,name) values ($1,$2,$3,'authorization-manager-permissions')", tenant.Bucket, tenant.TenantID, setID); err != nil {
			return err
		}
		if _, err := tx.Exec(context.Background(), "insert into role_permission_set(tenant_bucket,tenant_id,role_id,permission_set_id) values ($1,$2,$3,$4)", tenant.Bucket, tenant.TenantID, roleID, setID); err != nil {
			return err
		}
		if _, err := tx.Exec(context.Background(), "insert into principal_role_assignment(tenant_bucket,tenant_id,assignment_id,principal_id,role_id) values ($1,$2,$3,$4,$5)", tenant.Bucket, tenant.TenantID, uuid.New(), principal.Actor.ID, roleID); err != nil {
			return err
		}
		for _, permission := range [][2]string{{"authorization.role", "create"}, {"authorization.role", "update"}, {"authorization.role", "assign"}, {"authorization.policy", "update"}, {"authorization.permission-set", "create"}, {"authorization.permission-set", "update"}, {"authorization.group", "create"}, {"authorization.group", "update"}, {"record.share", "grant"}, {"record.share", "revoke"}, {"record.share", "rule"}, {"organization", "merge"}, {"audit", "read"}} {
			permissionID := uuid.New()
			if _, err := tx.Exec(context.Background(), "insert into authorization_permission(tenant_bucket,tenant_id,permission_id,resource_type,resource_ref,action) values ($1,$2,$3,'platform',$4,$5)", tenant.Bucket, tenant.TenantID, permissionID, permission[0], permission[1]); err != nil {
				return err
			}
			if _, err := tx.Exec(context.Background(), "insert into permission_set_permission(tenant_bucket,tenant_id,permission_set_id,permission_id) values ($1,$2,$3,$4)", tenant.Bucket, tenant.TenantID, setID, permissionID); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

type fieldGrant struct {
	fieldID uuid.UUID
	action  string
}

func grantRecordRole(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID string, objectID uuid.UUID, objectActions []string, fields []fieldGrant, scope string) error {
	roleID, setID := uuid.New(), uuid.New()
	if _, err := tx.Exec(ctx, "insert into authorization_role(tenant_bucket,tenant_id,role_id,name) values ($1,$2,$3,$4)", tenant.Bucket, tenant.TenantID, roleID, "role-"+principalID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "insert into permission_set(tenant_bucket,tenant_id,permission_set_id,name) values ($1,$2,$3,$4)", tenant.Bucket, tenant.TenantID, setID, "set-"+principalID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "insert into role_permission_set(tenant_bucket,tenant_id,role_id,permission_set_id) values ($1,$2,$3,$4)", tenant.Bucket, tenant.TenantID, roleID, setID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "insert into principal_role_assignment(tenant_bucket,tenant_id,assignment_id,principal_id,role_id) values ($1,$2,$3,$4,$5)", tenant.Bucket, tenant.TenantID, uuid.New(), principalID, roleID); err != nil {
		return err
	}
	grant := func(resourceType, resourceRef, action string) error {
		permissionID := uuid.New()
		if err := tx.QueryRow(ctx, "insert into authorization_permission(tenant_bucket,tenant_id,permission_id,resource_type,resource_ref,action) values ($1,$2,$3,$4,$5,$6) on conflict (tenant_bucket,tenant_id,resource_type,resource_ref,action,effect) do update set resource_ref=excluded.resource_ref returning permission_id", tenant.Bucket, tenant.TenantID, permissionID, resourceType, resourceRef, action).Scan(&permissionID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "insert into permission_set_permission(tenant_bucket,tenant_id,permission_set_id,permission_id) values ($1,$2,$3,$4)", tenant.Bucket, tenant.TenantID, setID, permissionID); err != nil {
			return err
		}
		return nil
	}
	for _, action := range objectActions {
		if err := grant("object", objectID.String(), action); err != nil {
			return err
		}
	}
	for _, field := range fields {
		if err := grant("field", field.fieldID.String(), field.action); err != nil {
			return err
		}
	}
	if scope == "" {
		return nil
	}
	parsed, err := uuid.Parse(scope[len("organization_descendants:"):])
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "insert into role_data_scope(tenant_bucket,tenant_id,scope_id,role_id,object_id,action,scope_type,organization_id) values ($1,$2,$3,$4,$5,'read','organization_descendants',$6)", tenant.Bucket, tenant.TenantID, uuid.New(), roleID, objectID, parsed); err != nil {
		return err
	}
	return nil
}

func grantExpiredRecordRole(t *testing.T, runtime *pgxpool.Pool, principal capability.TrustedPrincipal, objectID, fieldID uuid.UUID) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		if err := grantRecordRole(context.Background(), tx, tenant, principal.Actor.ID, objectID, []string{"read"}, []fieldGrant{{fieldID: fieldID, action: "read"}}, ""); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(), `update principal_role_assignment
			set effective_from=clock_timestamp()-interval '2 hours',effective_to=clock_timestamp()-interval '1 hour'
			where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and assignment_state='active'`, tenant.Bucket, tenant.TenantID, principal.Actor.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func grantConditionalRecordRole(t *testing.T, runtime *pgxpool.Pool, principal capability.TrustedPrincipal, objectID, fieldID uuid.UUID) uuid.UUID {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	var roleID uuid.UUID
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		if err := grantRecordRole(context.Background(), tx, tenant, principal.Actor.ID, objectID, []string{"read"}, []fieldGrant{{fieldID: fieldID, action: "read"}}, ""); err != nil {
			return err
		}
		return tx.QueryRow(context.Background(), "select role_id from principal_role_assignment where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and assignment_state='active' order by created_at desc limit 1", tenant.Bucket, tenant.TenantID, principal.Actor.ID).Scan(&roleID)
	}); err != nil {
		t.Fatal(err)
	}
	return roleID
}

func movePrincipalToOrganization(t *testing.T, runtime *pgxpool.Pool, principal capability.TrustedPrincipal, oldOrganizationID, newOrganizationID uuid.UUID) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(), "update principal_org_membership set membership_state='ended',effective_to=clock_timestamp(),is_primary=false where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and organization_id=$4 and membership_state='active'", tenant.Bucket, tenant.TenantID, principal.Actor.ID, oldOrganizationID); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(), "insert into principal_org_membership(tenant_bucket,tenant_id,membership_id,principal_id,organization_id,is_primary) values ($1,$2,$3,$4,$5,true)", tenant.Bucket, tenant.TenantID, uuid.New(), principal.Actor.ID, newOrganizationID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func moveRecordDataOrganization(t *testing.T, runtime *pgxpool.Pool, principal capability.TrustedPrincipal, objectID, recordID, organizationID uuid.UUID) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		command, err := tx.Exec(context.Background(), "update object_record set data_organization_id=$5 where tenant_bucket=$1 and tenant_id=$2 and object_id=$3 and record_id=$4", tenant.Bucket, tenant.TenantID, objectID, recordID, organizationID)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return fmt.Errorf("record %s was not moved", recordID)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func setAccessGroupLifecycle(t *testing.T, runtime *pgxpool.Pool, principal capability.TrustedPrincipal, groupID uuid.UUID, lifecycleState string) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		command, err := tx.Exec(context.Background(), "update access_group set lifecycle_state=$4 where tenant_bucket=$1 and tenant_id=$2 and group_id=$3", tenant.Bucket, tenant.TenantID, groupID, lifecycleState)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return fmt.Errorf("access group %s was not updated", groupID)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func projectedRuleRecordID(t *testing.T, runtime *pgxpool.Pool, principal capability.TrustedPrincipal, objectID, ruleID uuid.UUID) uuid.UUID {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	var recordID uuid.UUID
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), "select record_id from share_projection where tenant_bucket=$1 and tenant_id=$2 and object_id=$3 and rule_id=$4 order by record_id limit 1", tenant.Bucket, tenant.TenantID, objectID, ruleID).Scan(&recordID)
	}); err != nil {
		t.Fatal(err)
	}
	return recordID
}

func assertRuleProjectionExists(t *testing.T, runtime *pgxpool.Pool, principal capability.TrustedPrincipal, objectID, recordID, ruleID uuid.UUID) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		var count int
		if err := tx.QueryRow(context.Background(), "select count(*) from share_projection where tenant_bucket=$1 and tenant_id=$2 and object_id=$3 and record_id=$4 and rule_id=$5", tenant.Bucket, tenant.TenantID, objectID, recordID, ruleID).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("share projection count=%d", count)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func setRuleProjectionState(t *testing.T, runtime *pgxpool.Pool, principal capability.TrustedPrincipal, ruleID uuid.UUID, state string) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), "update sharing_rule_def set projection_state=$4,projection_error='simulated worker failure' where tenant_bucket=$1 and tenant_id=$2 and rule_id=$3", tenant.Bucket, tenant.TenantID, ruleID, state)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func assertAuthorizationAuditExists(t *testing.T, runtime *pgxpool.Pool, principal capability.TrustedPrincipal, capabilityID, status string) {
	t.Helper()
	tenant := database.TenantContext{TenantID: uuid.MustParse(principal.TenantID), Bucket: 7, ActorID: principal.Actor.ID}
	if err := database.WithTenant(context.Background(), runtime, tenant, func(tx pgx.Tx) error {
		var found bool
		if err := tx.QueryRow(context.Background(), "select exists(select 1 from audit_event where tenant_bucket=$1 and tenant_id=$2 and actor_id=$3 and capability_id=$4 and status=$5)", tenant.Bucket, tenant.TenantID, principal.Actor.ID, capabilityID, status).Scan(&found); err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("missing audit event for %s status %s", capabilityID, status)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func containsAccessSource(sources []string, wanted string) bool {
	for _, source := range sources {
		if source == wanted {
			return true
		}
	}
	return false
}
