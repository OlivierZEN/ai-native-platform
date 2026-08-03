package migrate

import _ "embed"

//go:embed sql/0001_shard_and_tenant_isolation.sql
var shardAndTenantIsolationSQL string

//go:embed sql/0002_metadata_core.sql
var metadataCoreSQL string

//go:embed sql/0003_tenant_control_role.sql
var tenantControlRoleSQL string

//go:embed sql/0004_record_runtime.sql
var recordRuntimeSQL string

//go:embed sql/0005_changeset_field_evolution.sql
var changesetFieldEvolutionSQL string

//go:embed sql/0006_changeset_execution.sql
var changesetExecutionSQL string

//go:embed sql/0007_authorization_rbac.sql
var authorizationRBACSQL string

//go:embed sql/0008_organization_merge.sql
var organizationMergeSQL string

//go:embed sql/0009_sharing_rule_projection.sql
var sharingRuleProjectionSQL string

//go:embed sql/0010_role_data_scope_null_idempotency.sql
var roleDataScopeNullIdempotencySQL string

//go:embed sql/0011_share_projection_record_trigger.sql
var shareProjectionRecordTriggerSQL string

//go:embed sql/0012_timed_role_assignments.sql
var timedRoleAssignmentsSQL string

//go:embed sql/0013_company_identity_rename.sql
var companyIdentityRenameSQL string

//go:embed sql/0014_usage_metering.sql
var usageMeteringSQL string

//go:embed sql/0015_usage_rollups.sql
var usageRollupsSQL string

//go:embed sql/0016_physical_storage_samples.sql
var physicalStorageSamplesSQL string

//go:embed sql/0017_governed_principal_projection.sql
var governedPrincipalProjectionSQL string

// Builtin returns the ordered migrations compiled into this binary. Domain
// migrations are added here as their owning tasks are implemented.
func Builtin() []Migration {
	return []Migration{
		{Version: 1, Name: "shard_and_tenant_isolation", SQL: shardAndTenantIsolationSQL},
		{Version: 2, Name: "metadata_core", SQL: metadataCoreSQL},
		{Version: 3, Name: "tenant_control_role", SQL: tenantControlRoleSQL},
		{Version: 4, Name: "record_runtime", SQL: recordRuntimeSQL},
		{Version: 5, Name: "changeset_field_evolution", SQL: changesetFieldEvolutionSQL},
		{Version: 6, Name: "changeset_execution", SQL: changesetExecutionSQL},
		{Version: 7, Name: "authorization_rbac", SQL: authorizationRBACSQL},
		{Version: 8, Name: "organization_merge", SQL: organizationMergeSQL},
		{Version: 9, Name: "sharing_rule_projection", SQL: sharingRuleProjectionSQL},
		{Version: 10, Name: "role_data_scope_null_idempotency", SQL: roleDataScopeNullIdempotencySQL},
		{Version: 11, Name: "share_projection_record_trigger", SQL: shareProjectionRecordTriggerSQL},
		{Version: 12, Name: "timed_role_assignments", SQL: timedRoleAssignmentsSQL},
		{Version: 13, Name: "company_identity_rename", SQL: companyIdentityRenameSQL},
		{Version: 14, Name: "usage_metering", SQL: usageMeteringSQL},
		{Version: 15, Name: "usage_rollups", SQL: usageRollupsSQL},
		{Version: 16, Name: "physical_storage_samples", SQL: physicalStorageSamplesSQL},
		// Migration 17 was published with one trailing blank line. Preserve its
		// exact runtime bytes even though the source file is formatting-clean.
		{Version: 17, Name: "governed_principal_projection", SQL: governedPrincipalProjectionSQL + "\n"},
	}
}
