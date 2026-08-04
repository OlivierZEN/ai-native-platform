package authorization

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	resourceAuthorizationRole          = "authorization.role"
	resourceAuthorizationPermissionSet = "authorization.permission-set"
	resourceRecordShare                = "record.share"
	resourceOrganization               = "organization"
	resourceAuthorizationGroup         = "authorization.group"
)

type Service struct {
	pool      *pgxpool.Pool
	router    database.RowQuerier
	evaluator *Evaluator
}

func NewService(pool *pgxpool.Pool, router database.RowQuerier) *Service {
	if pool == nil || router == nil {
		panic("authorization service requires runtime pool and tenant router")
	}
	return &Service{pool: pool, router: router, evaluator: NewEvaluator()}
}

type Role struct {
	RoleID         string `json:"role_id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	LifecycleState string `json:"lifecycle_state"`
}

type PermissionSet struct {
	PermissionSetID string `json:"permission_set_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	LifecycleState  string `json:"lifecycle_state"`
}

type ShareGrant struct {
	ShareGrantID   string `json:"share_grant_id"`
	ObjectID       string `json:"object_id"`
	RecordID       string `json:"record_id"`
	GranteeType    string `json:"grantee_type"`
	GranteeRef     string `json:"grantee_ref"`
	AccessLevel    string `json:"access_level"`
	GrantSource    string `json:"grant_source"`
	LifecycleState string `json:"lifecycle_state"`
}

type OrganizationMergeOperation struct {
	OperationID          string `json:"operation_id"`
	SourceOrganizationID string `json:"source_organization_id"`
	TargetOrganizationID string `json:"target_organization_id"`
	State                string `json:"state"`
	RecordsMigrated      int64  `json:"records_migrated"`
	Completed            bool   `json:"completed"`
}

type AccessGroup struct {
	GroupID        string `json:"group_id"`
	Name           string `json:"name"`
	GroupType      string `json:"group_type"`
	LifecycleState string `json:"lifecycle_state"`
}

type SharingRule struct {
	RuleID          string `json:"rule_id"`
	ObjectID        string `json:"object_id"`
	Name            string `json:"name"`
	GranteeGroupID  string `json:"grantee_group_id"`
	AccessLevel     string `json:"access_level"`
	LifecycleState  string `json:"lifecycle_state"`
	ProjectionState string `json:"projection_state"`
}

type AccessExplanation struct {
	PrincipalID string   `json:"principal_id"`
	ObjectID    string   `json:"object_id"`
	RecordID    string   `json:"record_id"`
	Action      string   `json:"action"`
	Allowed     bool     `json:"allowed"`
	Sources     []string `json:"sources"`
}

type CreateRoleInput struct {
	RoleID      string `json:"role_id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type CreatePermissionSetInput struct {
	PermissionSetID string `json:"permission_set_id,omitempty"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
}

type GrantPermissionInput struct {
	PermissionSetID string `json:"permission_set_id"`
	ResourceType    string `json:"resource_type"`
	ResourceRef     string `json:"resource_ref"`
	Action          string `json:"action"`
	ApprovalID      string `json:"approval_id"`
}

type AttachPermissionSetInput struct {
	RoleID          string `json:"role_id"`
	PermissionSetID string `json:"permission_set_id"`
	ApprovalID      string `json:"approval_id"`
}

type AssignRoleInput struct {
	PrincipalID string `json:"principal_id"`
	RoleID      string `json:"role_id"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	ApprovalID  string `json:"approval_id"`
}

type RevokeRoleInput struct {
	PrincipalID string `json:"principal_id"`
	RoleID      string `json:"role_id"`
	ApprovalID  string `json:"approval_id"`
}

type SetRoleDataScopeInput struct {
	RoleID         string          `json:"role_id"`
	ObjectID       string          `json:"object_id"`
	Action         string          `json:"action"`
	ScopeType      string          `json:"scope_type"`
	OrganizationID string          `json:"organization_id,omitempty"`
	Condition      json.RawMessage `json:"condition,omitempty"`
	ApprovalID     string          `json:"approval_id"`
}

type GrantShareInput struct {
	ShareGrantID string `json:"share_grant_id,omitempty"`
	ObjectID     string `json:"object_id"`
	RecordID     string `json:"record_id"`
	GranteeType  string `json:"grantee_type"`
	GranteeRef   string `json:"grantee_ref"`
	AccessLevel  string `json:"access_level"`
	ApprovalID   string `json:"approval_id"`
}

type RevokeShareInput struct {
	ShareGrantID string `json:"share_grant_id"`
	ApprovalID   string `json:"approval_id"`
}

type StartOrganizationMergeInput struct {
	SourceOrganizationID string `json:"source_organization_id"`
	TargetOrganizationID string `json:"target_organization_id"`
	ApprovalID           string `json:"approval_id"`
}

type ExecuteOrganizationMergeInput struct {
	OperationID string `json:"operation_id"`
	BatchSize   int    `json:"batch_size,omitempty"`
	ApprovalID  string `json:"approval_id"`
}

type CancelOrganizationMergeInput struct {
	OperationID string `json:"operation_id"`
	ApprovalID  string `json:"approval_id"`
}

type CreateGroupInput struct {
	GroupID   string `json:"group_id,omitempty"`
	Name      string `json:"name"`
	GroupType string `json:"group_type,omitempty"`
}

type SetGroupMembershipInput struct {
	GroupID     string `json:"group_id"`
	PrincipalID string `json:"principal_id"`
	Active      bool   `json:"active"`
	ApprovalID  string `json:"approval_id"`
}

type AddTeamMemberInput struct {
	ObjectID    string `json:"object_id"`
	RecordID    string `json:"record_id"`
	PrincipalID string `json:"principal_id"`
	AccessLevel string `json:"access_level"`
	ApprovalID  string `json:"approval_id"`
}

type UpsertSharingRuleInput struct {
	RuleID             string `json:"rule_id,omitempty"`
	ObjectID           string `json:"object_id"`
	Name               string `json:"name"`
	DataOrganizationID string `json:"data_organization_id"`
	GranteeGroupID     string `json:"grantee_group_id"`
	AccessLevel        string `json:"access_level"`
	ApprovalID         string `json:"approval_id"`
}

type RefreshSharingRuleInput struct {
	RuleID     string `json:"rule_id"`
	BatchSize  int    `json:"batch_size,omitempty"`
	ApprovalID string `json:"approval_id"`
}

type RetrySharingRuleInput struct {
	RuleID     string `json:"rule_id"`
	ApprovalID string `json:"approval_id"`
}

type SetRoleConflictInput struct {
	RoleID            string `json:"role_id"`
	ConflictingRoleID string `json:"conflicting_role_id"`
	ApprovalID        string `json:"approval_id"`
}

type ExplainAccessInput struct {
	PrincipalID string `json:"principal_id"`
	ObjectID    string `json:"object_id"`
	RecordID    string `json:"record_id"`
	Action      string `json:"action"`
}

// SetObjectPolicyInput controls whether an object's object, field, and record
// PDP checks are enforced. Enabling an object is intentionally a high-risk,
// approved operation because it changes the default deny boundary.
type SetObjectPolicyInput struct {
	ObjectID            string `json:"object_id"`
	EnforcementState    string `json:"enforcement_state"`
	DefaultRecordAccess string `json:"default_record_access"`
	ApprovalID          string `json:"approval_id"`
}

func (service *Service) CreateRole(ctx context.Context, request capability.Request, input CreateRoleInput) (Role, *capability.StableError) {
	roleID, stableErr := optionalID(input.RoleID, "role_id")
	if stableErr != nil {
		return Role{}, stableErr
	}
	if roleID == uuid.Nil {
		roleID = uuid.New()
	}
	if !validName(input.Name) {
		return Role{}, validationError("role name is required and must be at most 200 characters")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Role{}, stableErr
	}
	var result Role
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationRole, "create"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "insert into authorization_role(tenant_bucket,tenant_id,role_id,name,description) values ($1,$2,$3,$4,$5)", tenant.Bucket, tenant.TenantID, roleID, input.Name, input.Description); err != nil {
			return err
		}
		return scanRole(tx.QueryRow(ctx, "select role_id,name,description,lifecycle_state from authorization_role where role_id=$1", roleID), &result)
	})
	if err != nil {
		return Role{}, mapError(err)
	}
	return result, nil
}

func (service *Service) SetObjectPolicy(ctx context.Context, request capability.Request, input SetObjectPolicyInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	objectID, stableErr := requiredID(input.ObjectID, "object_id")
	if stableErr != nil {
		return stableErr
	}
	if (input.EnforcementState != "disabled" && input.EnforcementState != "enforced") || (input.DefaultRecordAccess != "private" && input.DefaultRecordAccess != "read_all") {
		return validationError("enforcement_state or default_record_access is invalid")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, "authorization.policy", "update"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `insert into object_authorization_policy(tenant_bucket,tenant_id,object_id,enforcement_state,default_record_access)
			values ($1,$2,$3,$4,$5)
			on conflict (tenant_bucket,tenant_id,object_id) do update set enforcement_state=excluded.enforcement_state,default_record_access=excluded.default_record_access,updated_at=clock_timestamp()`, tenant.Bucket, tenant.TenantID, objectID, input.EnforcementState, input.DefaultRecordAccess)
		return err
	})
	return mapStableError(err)
}

func (service *Service) CreatePermissionSet(ctx context.Context, request capability.Request, input CreatePermissionSetInput) (PermissionSet, *capability.StableError) {
	permissionSetID, stableErr := optionalID(input.PermissionSetID, "permission_set_id")
	if stableErr != nil {
		return PermissionSet{}, stableErr
	}
	if permissionSetID == uuid.Nil {
		permissionSetID = uuid.New()
	}
	if !validName(input.Name) {
		return PermissionSet{}, validationError("permission set name is required and must be at most 200 characters")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return PermissionSet{}, stableErr
	}
	var result PermissionSet
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationPermissionSet, "create"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "insert into permission_set(tenant_bucket,tenant_id,permission_set_id,name,description) values ($1,$2,$3,$4,$5)", tenant.Bucket, tenant.TenantID, permissionSetID, input.Name, input.Description); err != nil {
			return err
		}
		return scanPermissionSet(tx.QueryRow(ctx, "select permission_set_id,name,description,lifecycle_state from permission_set where permission_set_id=$1", permissionSetID), &result)
	})
	if err != nil {
		return PermissionSet{}, mapError(err)
	}
	return result, nil
}

func (service *Service) GrantPermission(ctx context.Context, request capability.Request, input GrantPermissionInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	permissionSetID, stableErr := requiredID(input.PermissionSetID, "permission_set_id")
	if stableErr != nil {
		return stableErr
	}
	if !validPermission(input.ResourceType, input.ResourceRef, input.Action) {
		return validationError("permission resource_type, resource_ref, or action is invalid")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationPermissionSet, "update"); err != nil {
			return err
		}
		if err := service.requireDelegablePermission(ctx, tx, tenant, request.Actor.ID, input.ResourceType, input.ResourceRef, input.Action); err != nil {
			return err
		}
		permissionID := uuid.New()
		if err := tx.QueryRow(ctx, "insert into authorization_permission(tenant_bucket,tenant_id,permission_id,resource_type,resource_ref,action) values ($1,$2,$3,$4,$5,$6) on conflict (tenant_bucket,tenant_id,resource_type,resource_ref,action,effect) do update set resource_ref=excluded.resource_ref returning permission_id", tenant.Bucket, tenant.TenantID, permissionID, input.ResourceType, input.ResourceRef, input.Action).Scan(&permissionID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "insert into permission_set_permission(tenant_bucket,tenant_id,permission_set_id,permission_id) values ($1,$2,$3,$4) on conflict do nothing", tenant.Bucket, tenant.TenantID, permissionSetID, permissionID)
		return err
	})
	return mapStableError(err)
}

// RevokePermission removes a permission edge without deleting the shared
// atomic permission definition. Revocation is privilege-reducing, but still
// requires the permission-set management authority and an independently
// verified approval so cleanup remains governed and auditable.
func (service *Service) RevokePermission(ctx context.Context, request capability.Request, input GrantPermissionInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	permissionSetID, stableErr := requiredID(input.PermissionSetID, "permission_set_id")
	if stableErr != nil {
		return stableErr
	}
	if !validPermission(input.ResourceType, input.ResourceRef, input.Action) {
		return validationError("permission resource_type, resource_ref, or action is invalid")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationPermissionSet, "update"); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `delete from permission_set_permission edge
			using authorization_permission permission
			where edge.tenant_bucket=$1 and edge.tenant_id=$2 and edge.permission_set_id=$3
			  and permission.tenant_bucket=edge.tenant_bucket and permission.tenant_id=edge.tenant_id and permission.permission_id=edge.permission_id
			  and permission.resource_type=$4 and permission.resource_ref=$5 and permission.action=$6 and permission.effect='allow'`,
			tenant.Bucket, tenant.TenantID, permissionSetID, input.ResourceType, input.ResourceRef, input.Action)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	return mapStableError(err)
}

// requireDelegablePermission preserves the ordinary no-privilege-escalation
// rule while making a newly activated metadata resource governable. Before a
// new object or field has any grants, no actor can hold its exact permission;
// requiring exact ownership would therefore create a permanent bootstrap
// deadlock. A principal that already holds the separate
// authorization.policy:update authority may seed permissions, but only for an
// object or field in the tenant's currently active metadata version. Platform
// permissions and stale/unknown metadata identifiers still require an exact
// permission and fail closed.
func (service *Service) requireDelegablePermission(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, actorID, resourceType, resourceRef, action string) error {
	if resourceType != "object" && resourceType != "field" {
		return service.evaluator.RequirePermission(ctx, tx, tenant, actorID, resourceType, resourceRef, action)
	}
	var exact bool
	err := tx.QueryRow(ctx, `select exists(
		select 1 from principal_role_assignment assignment
		join principal_projection principal on principal.tenant_bucket=assignment.tenant_bucket and principal.tenant_id=assignment.tenant_id and principal.principal_id=assignment.principal_id
		join authorization_role role on role.tenant_bucket=assignment.tenant_bucket and role.tenant_id=assignment.tenant_id and role.role_id=assignment.role_id
		join role_permission_set role_set on role_set.tenant_bucket=role.tenant_bucket and role_set.tenant_id=role.tenant_id and role_set.role_id=role.role_id
		join permission_set permission_set on permission_set.tenant_bucket=role_set.tenant_bucket and permission_set.tenant_id=role_set.tenant_id and permission_set.permission_set_id=role_set.permission_set_id
		join permission_set_permission edge on edge.tenant_bucket=permission_set.tenant_bucket and edge.tenant_id=permission_set.tenant_id and edge.permission_set_id=permission_set.permission_set_id
		join authorization_permission permission on permission.tenant_bucket=edge.tenant_bucket and permission.tenant_id=edge.tenant_id and permission.permission_id=edge.permission_id
		where assignment.tenant_bucket=$1 and assignment.tenant_id=$2 and assignment.principal_id=$3 and assignment.assignment_state='active'
		  and (assignment.effective_to is null or assignment.effective_to>clock_timestamp()) and principal.status='active'
		  and role.lifecycle_state='active' and permission_set.lifecycle_state='active'
		  and permission.resource_type=$4 and permission.resource_ref=$5 and permission.action=$6 and permission.effect='allow'
	)`, tenant.Bucket, tenant.TenantID, actorID, resourceType, resourceRef, action).Scan(&exact)
	if err != nil {
		return err
	}
	if exact {
		return nil
	}
	if err := service.evaluator.RequirePermission(ctx, tx, tenant, actorID, "platform", "authorization.policy", "update"); err != nil {
		return ErrDenied
	}
	resourceID, parseErr := uuid.Parse(resourceRef)
	if parseErr != nil || resourceID == uuid.Nil {
		return ErrDenied
	}
	var active bool
	if resourceType == "object" {
		parseErr = tx.QueryRow(ctx, `select exists(
			select 1 from tenant_registry tenant
			join object_definition object on object.tenant_bucket=tenant.tenant_bucket and object.tenant_id=tenant.tenant_id and object.metadata_version_id=tenant.metadata_version_id
			where tenant.tenant_bucket=$1 and tenant.tenant_id=$2 and object.object_id=$3
		)`, tenant.Bucket, tenant.TenantID, resourceID).Scan(&active)
	} else {
		parseErr = tx.QueryRow(ctx, `select exists(
			select 1 from tenant_registry tenant
			join field_definition field on field.tenant_bucket=tenant.tenant_bucket and field.tenant_id=tenant.tenant_id and field.metadata_version_id=tenant.metadata_version_id
			where tenant.tenant_bucket=$1 and tenant.tenant_id=$2 and field.field_id=$3 and field.lifecycle_state in ('active','deprecated_read_write','deprecated_read_only')
		)`, tenant.Bucket, tenant.TenantID, resourceID).Scan(&active)
	}
	if parseErr != nil {
		return parseErr
	}
	if !active {
		return ErrDenied
	}
	return nil
}

func (service *Service) AttachPermissionSet(ctx context.Context, request capability.Request, input AttachPermissionSetInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	roleID, stableErr := requiredID(input.RoleID, "role_id")
	if stableErr != nil {
		return stableErr
	}
	permissionSetID, stableErr := requiredID(input.PermissionSetID, "permission_set_id")
	if stableErr != nil {
		return stableErr
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationRole, "update"); err != nil {
			return err
		}
		canDelegate, err := service.canDelegatePermissionSet(ctx, tx, tenant, request.Actor.ID, permissionSetID)
		if err != nil {
			return err
		}
		if !canDelegate {
			canDelegate, err = service.canPolicyAdministratorDelegateMetadataPermissionSet(ctx, tx, tenant, request.Actor.ID, permissionSetID)
			if err != nil {
				return err
			}
		}
		if !canDelegate {
			return ErrDenied
		}
		_, err = tx.Exec(ctx, "insert into role_permission_set(tenant_bucket,tenant_id,role_id,permission_set_id) values ($1,$2,$3,$4) on conflict do nothing", tenant.Bucket, tenant.TenantID, roleID, permissionSetID)
		return err
	})
	return mapStableError(err)
}

func (service *Service) AssignRole(ctx context.Context, request capability.Request, input AssignRoleInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	roleID, stableErr := requiredID(input.RoleID, "role_id")
	if stableErr != nil {
		return stableErr
	}
	if strings.TrimSpace(input.PrincipalID) == "" || len(input.PrincipalID) > 200 {
		return validationError("principal_id is invalid")
	}
	var expiresAt *time.Time
	if input.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, input.ExpiresAt)
		if err != nil || !parsed.After(time.Now()) {
			return validationError("expires_at must be a future RFC3339 timestamp")
		}
		expiresAt = &parsed
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationRole, "assign"); err != nil {
			return err
		}
		canDelegate, err := service.canDelegateRole(ctx, tx, tenant, request.Actor.ID, roleID)
		if err != nil {
			return err
		}
		if !canDelegate {
			canDelegate, err = service.canPolicyAdministratorDelegateMetadataRole(ctx, tx, tenant, request.Actor.ID, roleID)
			if err != nil {
				return err
			}
		}
		if !canDelegate {
			return ErrDenied
		}
		var conflict bool
		if err := tx.QueryRow(ctx, `select exists(
			select 1 from principal_role_assignment assignment
			join role_conflict conflict on conflict.tenant_bucket=assignment.tenant_bucket and conflict.tenant_id=assignment.tenant_id
			where assignment.tenant_bucket=$1 and assignment.tenant_id=$2 and assignment.principal_id=$3 and assignment.assignment_state='active' and (assignment.effective_to is null or assignment.effective_to>clock_timestamp())
			  and ((conflict.role_id=$4 and conflict.conflicting_role_id=assignment.role_id) or (conflict.conflicting_role_id=$4 and conflict.role_id=assignment.role_id))
		)`, tenant.Bucket, tenant.TenantID, input.PrincipalID, roleID).Scan(&conflict); err != nil {
			return err
		}
		if conflict {
			return ErrDenied
		}
		if _, err := tx.Exec(ctx, "insert into principal_role_assignment(tenant_bucket,tenant_id,assignment_id,principal_id,role_id,effective_to) values ($1,$2,$3,$4,$5,$6)", tenant.Bucket, tenant.TenantID, uuid.New(), input.PrincipalID, roleID, expiresAt); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "delete from permission_snapshot where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3", tenant.Bucket, tenant.TenantID, input.PrincipalID)
		return err
	})
	return mapStableError(err)
}

func (service *Service) RevokeRole(ctx context.Context, request capability.Request, input RevokeRoleInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	roleID, stableErr := requiredID(input.RoleID, "role_id")
	if stableErr != nil {
		return stableErr
	}
	if strings.TrimSpace(input.PrincipalID) == "" || len(input.PrincipalID) > 200 {
		return validationError("principal_id is invalid")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationRole, "assign"); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `update principal_role_assignment set assignment_state='ended',effective_to=clock_timestamp()
			where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and role_id=$4 and assignment_state='active' and (effective_to is null or effective_to>clock_timestamp())`, tenant.Bucket, tenant.TenantID, input.PrincipalID, roleID)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		_, err = tx.Exec(ctx, "delete from permission_snapshot where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3", tenant.Bucket, tenant.TenantID, input.PrincipalID)
		return err
	})
	return mapStableError(err)
}

func (service *Service) SetRoleDataScope(ctx context.Context, request capability.Request, input SetRoleDataScopeInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	roleID, stableErr := requiredID(input.RoleID, "role_id")
	if stableErr != nil {
		return stableErr
	}
	objectID, stableErr := requiredID(input.ObjectID, "object_id")
	if stableErr != nil {
		return stableErr
	}
	if !validDataScope(input.ScopeType) || !validAccessAction(input.Action) {
		return validationError("scope_type or action is invalid")
	}
	var organizationID *uuid.UUID
	var condition json.RawMessage
	if input.ScopeType == "organization" || input.ScopeType == "organization_descendants" {
		parsed, stableErr := requiredID(input.OrganizationID, "organization_id")
		if stableErr != nil {
			return stableErr
		}
		organizationID = &parsed
	} else if input.OrganizationID != "" {
		return validationError("organization_id is only valid for organization scopes")
	}
	if input.ScopeType == "conditional" {
		var stableErr *capability.StableError
		condition, stableErr = normalizeConditionalDataScope(input.Condition)
		if stableErr != nil {
			return stableErr
		}
	} else if len(input.Condition) != 0 {
		return validationError("condition is only valid for conditional scopes")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationRole, "update"); err != nil {
			return err
		}
		if organizationID != nil {
			var active bool
			if err := tx.QueryRow(ctx, "select exists(select 1 from organization_node where tenant_bucket=$1 and tenant_id=$2 and organization_id=$3 and lifecycle_state='active')", tenant.Bucket, tenant.TenantID, *organizationID).Scan(&active); err != nil {
				return err
			}
			if !active {
				return pgx.ErrNoRows
			}
		}
		if organizationID == nil {
			_, err := tx.Exec(ctx, `insert into role_data_scope(tenant_bucket,tenant_id,scope_id,role_id,object_id,action,scope_type,organization_id,condition_expression)
				values ($1,$2,$3,$4,$5,$6,$7,null,$8)
				on conflict (tenant_bucket,tenant_id,role_id,object_id,action,scope_type) where organization_id is null
				do update set condition_expression=excluded.condition_expression`, tenant.Bucket, tenant.TenantID, uuid.New(), roleID, objectID, input.Action, input.ScopeType, condition)
			return err
		}
		_, err := tx.Exec(ctx, `insert into role_data_scope(tenant_bucket,tenant_id,scope_id,role_id,object_id,action,scope_type,organization_id)
			values ($1,$2,$3,$4,$5,$6,$7,$8)
			on conflict do nothing`, tenant.Bucket, tenant.TenantID, uuid.New(), roleID, objectID, input.Action, input.ScopeType, organizationID)
		return err
	})
	return mapStableError(err)
}

// canDelegateRole prevents an access administrator from turning management
// permission into arbitrary privilege escalation. Every atomic permission on
// the target role must already be held by the assigning principal, with the
// usual wildcard resource semantics. Data-scope delegation is intentionally
// configured only by separate, approved policy changes.
func (service *Service) canDelegateRole(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, actorID string, roleID uuid.UUID) (bool, error) {
	var exceedsActor bool
	err := tx.QueryRow(ctx, `select exists(
		select 1
		from role_permission_set target_role_set
		join permission_set target_set on target_set.tenant_bucket=target_role_set.tenant_bucket and target_set.tenant_id=target_role_set.tenant_id and target_set.permission_set_id=target_role_set.permission_set_id
		join permission_set_permission target_set_permission on target_set_permission.tenant_bucket=target_set.tenant_bucket and target_set_permission.tenant_id=target_set.tenant_id and target_set_permission.permission_set_id=target_set.permission_set_id
		join authorization_permission target_permission on target_permission.tenant_bucket=target_set_permission.tenant_bucket and target_permission.tenant_id=target_set_permission.tenant_id and target_permission.permission_id=target_set_permission.permission_id
		where target_role_set.tenant_bucket=$1 and target_role_set.tenant_id=$2 and target_role_set.role_id=$4
		  and target_set.lifecycle_state='active'
		  and not exists(
			select 1
			from principal_role_assignment actor_assignment
			join authorization_role actor_role on actor_role.tenant_bucket=actor_assignment.tenant_bucket and actor_role.tenant_id=actor_assignment.tenant_id and actor_role.role_id=actor_assignment.role_id
			join role_permission_set actor_role_set on actor_role_set.tenant_bucket=actor_role.tenant_bucket and actor_role_set.tenant_id=actor_role.tenant_id and actor_role_set.role_id=actor_role.role_id
			join permission_set actor_set on actor_set.tenant_bucket=actor_role_set.tenant_bucket and actor_set.tenant_id=actor_role_set.tenant_id and actor_set.permission_set_id=actor_role_set.permission_set_id
			join permission_set_permission actor_set_permission on actor_set_permission.tenant_bucket=actor_set.tenant_bucket and actor_set_permission.tenant_id=actor_set.tenant_id and actor_set_permission.permission_set_id=actor_set.permission_set_id
			join authorization_permission actor_permission on actor_permission.tenant_bucket=actor_set_permission.tenant_bucket and actor_permission.tenant_id=actor_set_permission.tenant_id and actor_permission.permission_id=actor_set_permission.permission_id
			where actor_assignment.tenant_bucket=$1 and actor_assignment.tenant_id=$2 and actor_assignment.principal_id=$3 and actor_assignment.assignment_state='active' and (actor_assignment.effective_to is null or actor_assignment.effective_to>clock_timestamp())
			  and actor_role.lifecycle_state='active' and actor_set.lifecycle_state='active'
			  and actor_permission.resource_type=target_permission.resource_type
			  and actor_permission.resource_ref in (target_permission.resource_ref,'*')
			  and actor_permission.action=target_permission.action and actor_permission.effect='allow'
		  )
	)`, tenant.Bucket, tenant.TenantID, actorID, roleID).Scan(&exceedsActor)
	if err != nil {
		return false, err
	}
	return !exceedsActor, nil
}

func (service *Service) canPolicyAdministratorDelegateMetadataRole(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, actorID string, roleID uuid.UUID) (bool, error) {
	rows, err := tx.Query(ctx, `select permission_set_id from role_permission_set
		where tenant_bucket=$1 and tenant_id=$2 and role_id=$3`, tenant.Bucket, tenant.TenantID, roleID)
	if err != nil {
		return false, err
	}
	permissionSetIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var permissionSetID uuid.UUID
		if err := rows.Scan(&permissionSetID); err != nil {
			rows.Close()
			return false, err
		}
		permissionSetIDs = append(permissionSetIDs, permissionSetID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	for _, permissionSetID := range permissionSetIDs {
		allowed, err := service.canPolicyAdministratorDelegateMetadataPermissionSet(ctx, tx, tenant, actorID, permissionSetID)
		if err != nil || !allowed {
			return false, err
		}
	}
	return true, nil
}

func (service *Service) canDelegatePermissionSet(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, actorID string, permissionSetID uuid.UUID) (bool, error) {
	var exceedsActor bool
	err := tx.QueryRow(ctx, `select exists(
		select 1
		from permission_set_permission target_set_permission
		join permission_set target_set on target_set.tenant_bucket=target_set_permission.tenant_bucket and target_set.tenant_id=target_set_permission.tenant_id and target_set.permission_set_id=target_set_permission.permission_set_id
		join authorization_permission target_permission on target_permission.tenant_bucket=target_set_permission.tenant_bucket and target_permission.tenant_id=target_set_permission.tenant_id and target_permission.permission_id=target_set_permission.permission_id
		where target_set_permission.tenant_bucket=$1 and target_set_permission.tenant_id=$2 and target_set_permission.permission_set_id=$4 and target_set.lifecycle_state='active'
		  and not exists(
			select 1
			from principal_role_assignment actor_assignment
			join authorization_role actor_role on actor_role.tenant_bucket=actor_assignment.tenant_bucket and actor_role.tenant_id=actor_assignment.tenant_id and actor_role.role_id=actor_assignment.role_id
			join role_permission_set actor_role_set on actor_role_set.tenant_bucket=actor_role.tenant_bucket and actor_role_set.tenant_id=actor_role_set.tenant_id and actor_role_set.role_id=actor_role.role_id
			join permission_set actor_set on actor_set.tenant_bucket=actor_role_set.tenant_bucket and actor_set.tenant_id=actor_role_set.tenant_id and actor_set.permission_set_id=actor_role_set.permission_set_id
			join permission_set_permission actor_set_permission on actor_set_permission.tenant_bucket=actor_set.tenant_bucket and actor_set_permission.tenant_id=actor_set.tenant_id and actor_set_permission.permission_set_id=actor_set.permission_set_id
			join authorization_permission actor_permission on actor_permission.tenant_bucket=actor_set_permission.tenant_bucket and actor_permission.tenant_id=actor_set_permission.tenant_id and actor_permission.permission_id=actor_set_permission.permission_id
			where actor_assignment.tenant_bucket=$1 and actor_assignment.tenant_id=$2 and actor_assignment.principal_id=$3 and actor_assignment.assignment_state='active' and (actor_assignment.effective_to is null or actor_assignment.effective_to>clock_timestamp())
			  and actor_role.lifecycle_state='active' and actor_set.lifecycle_state='active'
			  and actor_permission.resource_type=target_permission.resource_type
			  and actor_permission.resource_ref in (target_permission.resource_ref,'*')
			  and actor_permission.action=target_permission.action and actor_permission.effect='allow'
		  )
	)`, tenant.Bucket, tenant.TenantID, actorID, permissionSetID).Scan(&exceedsActor)
	if err != nil {
		return false, err
	}
	return !exceedsActor, nil
}

// canPolicyAdministratorDelegateMetadataPermissionSet is the attachment half of the
// active-metadata bootstrap rule in requireDelegablePermission. It never
// relaxes platform permissions: every permission the actor does not already
// hold must reference an object or field in the current active metadata
// version, and the actor must separately hold authorization.policy:update.
func (service *Service) canPolicyAdministratorDelegateMetadataPermissionSet(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, actorID string, permissionSetID uuid.UUID) (bool, error) {
	if err := service.evaluator.RequirePermission(ctx, tx, tenant, actorID, "platform", "authorization.policy", "update"); err != nil {
		return false, nil
	}
	rows, err := tx.Query(ctx, `select permission.resource_type,permission.resource_ref,permission.action
		from permission_set_permission set_permission
		join permission_set target_set on target_set.tenant_bucket=set_permission.tenant_bucket and target_set.tenant_id=set_permission.tenant_id and target_set.permission_set_id=set_permission.permission_set_id
		join authorization_permission permission on permission.tenant_bucket=set_permission.tenant_bucket and permission.tenant_id=set_permission.tenant_id and permission.permission_id=set_permission.permission_id
		where set_permission.tenant_bucket=$1 and set_permission.tenant_id=$2 and set_permission.permission_set_id=$3 and target_set.lifecycle_state='active'`,
		tenant.Bucket, tenant.TenantID, permissionSetID)
	if err != nil {
		return false, err
	}
	type permissionRef struct{ resourceType, resourceRef, action string }
	permissions := make([]permissionRef, 0)
	for rows.Next() {
		var permission permissionRef
		if err := rows.Scan(&permission.resourceType, &permission.resourceRef, &permission.action); err != nil {
			rows.Close()
			return false, err
		}
		permissions = append(permissions, permission)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	for _, permission := range permissions {
		if err := service.requireDelegablePermission(ctx, tx, tenant, actorID, permission.resourceType, permission.resourceRef, permission.action); err != nil {
			return false, nil
		}
	}
	return true, nil
}

func (service *Service) GrantShare(ctx context.Context, request capability.Request, input GrantShareInput) (ShareGrant, *capability.StableError) {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return ShareGrant{}, stableErr
	}
	shareGrantID, stableErr := optionalID(input.ShareGrantID, "share_grant_id")
	if stableErr != nil {
		return ShareGrant{}, stableErr
	}
	if shareGrantID == uuid.Nil {
		shareGrantID = uuid.New()
	}
	objectID, stableErr := requiredID(input.ObjectID, "object_id")
	if stableErr != nil {
		return ShareGrant{}, stableErr
	}
	recordID, stableErr := requiredID(input.RecordID, "record_id")
	if stableErr != nil {
		return ShareGrant{}, stableErr
	}
	if (input.GranteeType != "principal" && input.GranteeType != "group") || strings.TrimSpace(input.GranteeRef) == "" || !validAccessLevel(input.AccessLevel) {
		return ShareGrant{}, validationError("share grantee or access_level is invalid")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return ShareGrant{}, stableErr
	}
	var result ShareGrant
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceRecordShare, "grant"); err != nil {
			return err
		}
		if err := validateGrantee(ctx, tx, tenant, input.GranteeType, input.GranteeRef); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "insert into share_grant(tenant_bucket,tenant_id,share_grant_id,object_id,record_id,grantee_type,grantee_ref,access_level,grant_source) values ($1,$2,$3,$4,$5,$6,$7,$8,'manual')", tenant.Bucket, tenant.TenantID, shareGrantID, objectID, recordID, input.GranteeType, input.GranteeRef, input.AccessLevel); err != nil {
			return err
		}
		return scanShareGrant(tx.QueryRow(ctx, "select share_grant_id,object_id,record_id,grantee_type,grantee_ref,access_level,grant_source,lifecycle_state from share_grant where share_grant_id=$1", shareGrantID), &result)
	})
	if err != nil {
		return ShareGrant{}, mapError(err)
	}
	return result, nil
}

func (service *Service) RevokeShare(ctx context.Context, request capability.Request, input RevokeShareInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	shareGrantID, stableErr := requiredID(input.ShareGrantID, "share_grant_id")
	if stableErr != nil {
		return stableErr
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceRecordShare, "revoke"); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, "update share_grant set lifecycle_state='revoked' where tenant_bucket=$1 and tenant_id=$2 and share_grant_id=$3 and lifecycle_state='active'", tenant.Bucket, tenant.TenantID, shareGrantID)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return pgx.ErrNoRows
		}
		return nil
	})
	return mapStableError(err)
}

func (service *Service) CreateGroup(ctx context.Context, request capability.Request, input CreateGroupInput) (AccessGroup, *capability.StableError) {
	groupID, stableErr := optionalID(input.GroupID, "group_id")
	if stableErr != nil {
		return AccessGroup{}, stableErr
	}
	if groupID == uuid.Nil {
		groupID = uuid.New()
	}
	groupType := input.GroupType
	if groupType == "" {
		groupType = "manual"
	}
	if !validName(input.Name) || (groupType != "manual" && groupType != "rule") {
		return AccessGroup{}, validationError("group name or group_type is invalid")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return AccessGroup{}, stableErr
	}
	var result AccessGroup
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationGroup, "create"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "insert into access_group(tenant_bucket,tenant_id,group_id,name,group_type) values ($1,$2,$3,$4,$5)", tenant.Bucket, tenant.TenantID, groupID, input.Name, groupType); err != nil {
			return err
		}
		return tx.QueryRow(ctx, "select group_id,name,group_type,lifecycle_state from access_group where group_id=$1", groupID).Scan(&result.GroupID, &result.Name, &result.GroupType, &result.LifecycleState)
	})
	if err != nil {
		return AccessGroup{}, mapError(err)
	}
	return result, nil
}

func (service *Service) SetGroupMembership(ctx context.Context, request capability.Request, input SetGroupMembershipInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	groupID, stableErr := requiredID(input.GroupID, "group_id")
	if stableErr != nil {
		return stableErr
	}
	if strings.TrimSpace(input.PrincipalID) == "" || len(input.PrincipalID) > 200 {
		return validationError("principal_id is invalid")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationGroup, "update"); err != nil {
			return err
		}
		if err := validateGrantee(ctx, tx, tenant, "principal", input.PrincipalID); err != nil {
			return err
		}
		if input.Active {
			_, err := tx.Exec(ctx, "insert into group_membership(tenant_bucket,tenant_id,group_id,principal_id,membership_state) values ($1,$2,$3,$4,'active') on conflict (tenant_bucket,tenant_id,group_id,principal_id) do update set membership_state='active',ended_at=null", tenant.Bucket, tenant.TenantID, groupID, input.PrincipalID)
			return err
		}
		command, err := tx.Exec(ctx, "update group_membership set membership_state='ended',ended_at=clock_timestamp() where tenant_bucket=$1 and tenant_id=$2 and group_id=$3 and principal_id=$4 and membership_state='active'", tenant.Bucket, tenant.TenantID, groupID, input.PrincipalID)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return pgx.ErrNoRows
		}
		return nil
	})
	return mapStableError(err)
}

func (service *Service) AddTeamMember(ctx context.Context, request capability.Request, input AddTeamMemberInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	objectID, stableErr := requiredID(input.ObjectID, "object_id")
	if stableErr != nil {
		return stableErr
	}
	recordID, stableErr := requiredID(input.RecordID, "record_id")
	if stableErr != nil {
		return stableErr
	}
	if strings.TrimSpace(input.PrincipalID) == "" || !validAccessLevel(input.AccessLevel) {
		return validationError("team principal_id or access_level is invalid")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceRecordShare, "grant"); err != nil {
			return err
		}
		if err := validateGrantee(ctx, tx, tenant, "principal", input.PrincipalID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "insert into record_team_member(tenant_bucket,tenant_id,object_id,record_id,principal_id,access_level) values ($1,$2,$3,$4,$5,$6) on conflict (tenant_bucket,tenant_id,object_id,record_id,principal_id,access_level) do update set lifecycle_state='active'", tenant.Bucket, tenant.TenantID, objectID, recordID, input.PrincipalID, input.AccessLevel)
		return err
	})
	return mapStableError(err)
}

func (service *Service) UpsertSharingRule(ctx context.Context, request capability.Request, input UpsertSharingRuleInput) (SharingRule, *capability.StableError) {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return SharingRule{}, stableErr
	}
	ruleID, stableErr := optionalID(input.RuleID, "rule_id")
	if stableErr != nil {
		return SharingRule{}, stableErr
	}
	if ruleID == uuid.Nil {
		ruleID = uuid.New()
	}
	objectID, stableErr := requiredID(input.ObjectID, "object_id")
	if stableErr != nil {
		return SharingRule{}, stableErr
	}
	organizationID, stableErr := requiredID(input.DataOrganizationID, "data_organization_id")
	if stableErr != nil {
		return SharingRule{}, stableErr
	}
	groupID, stableErr := requiredID(input.GranteeGroupID, "grantee_group_id")
	if stableErr != nil {
		return SharingRule{}, stableErr
	}
	if !validName(input.Name) || !validAccessLevel(input.AccessLevel) {
		return SharingRule{}, validationError("sharing rule name or access_level is invalid")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return SharingRule{}, stableErr
	}
	var result SharingRule
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceRecordShare, "rule"); err != nil {
			return err
		}
		if err := validateGrantee(ctx, tx, tenant, "group", groupID.String()); err != nil {
			return err
		}
		condition := map[string]string{"data_organization_id": organizationID.String()}
		if _, err := tx.Exec(ctx, `insert into sharing_rule_def(tenant_bucket,tenant_id,rule_id,object_id,name,condition_expression,grantee_group_id,access_level,lifecycle_state,projection_state,projection_cursor,projection_revision,projection_error)
			values ($1,$2,$3,$4,$5,$6,$7,$8,'active','building',null,1,null)
			on conflict (tenant_bucket,tenant_id,rule_id) do update set object_id=excluded.object_id,name=excluded.name,condition_expression=excluded.condition_expression,grantee_group_id=excluded.grantee_group_id,access_level=excluded.access_level,lifecycle_state='active',projection_state='building',projection_cursor=null,projection_revision=sharing_rule_def.projection_revision+1,projection_error=null,updated_at=clock_timestamp()`, tenant.Bucket, tenant.TenantID, ruleID, objectID, input.Name, condition, groupID, input.AccessLevel); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "delete from share_projection where tenant_bucket=$1 and tenant_id=$2 and rule_id=$3", tenant.Bucket, tenant.TenantID, ruleID); err != nil {
			return err
		}
		return scanSharingRule(tx.QueryRow(ctx, "select rule_id,object_id,name,grantee_group_id,access_level,lifecycle_state,projection_state from sharing_rule_def where rule_id=$1", ruleID), &result)
	})
	if err != nil {
		return SharingRule{}, mapError(err)
	}
	return result, nil
}

func (service *Service) RefreshSharingRule(ctx context.Context, request capability.Request, input RefreshSharingRuleInput) (SharingRule, *capability.StableError) {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return SharingRule{}, stableErr
	}
	ruleID, stableErr := requiredID(input.RuleID, "rule_id")
	if stableErr != nil {
		return SharingRule{}, stableErr
	}
	batch := input.BatchSize
	if batch == 0 {
		batch = 500
	}
	if batch < 1 || batch > 1000 {
		return SharingRule{}, validationError("batch_size must be between 1 and 1000")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return SharingRule{}, stableErr
	}
	var result SharingRule
	projectionStarted := false
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceRecordShare, "rule"); err != nil {
			return err
		}
		var objectID, groupID, organizationID uuid.UUID
		var access, state string
		var cursor *uuid.UUID
		if err := tx.QueryRow(ctx, "select object_id,grantee_group_id,(condition_expression->>'data_organization_id')::uuid,access_level,projection_state,projection_cursor from sharing_rule_def where tenant_bucket=$1 and tenant_id=$2 and rule_id=$3 for update", tenant.Bucket, tenant.TenantID, ruleID).Scan(&objectID, &groupID, &organizationID, &access, &state, &cursor); err != nil {
			return err
		}
		if state != "building" {
			return preconditionError("sharing rule is not awaiting projection refresh")
		}
		projectionStarted = true
		after := uuid.Nil
		if cursor != nil {
			after = *cursor
		}
		rows, err := tx.Query(ctx, "select record_id from object_record where object_id=$1 and data_organization_id=$2 and lifecycle_state='active' and ($3::uuid='00000000-0000-0000-0000-000000000000'::uuid or record_id>$3) order by record_id limit $4", objectID, organizationID, after, batch)
		if err != nil {
			return err
		}
		var ids []uuid.UUID
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, id := range ids {
			if _, err := tx.Exec(ctx, "insert into share_projection(tenant_bucket,tenant_id,object_id,record_id,group_id,access_level,rule_id) values ($1,$2,$3,$4,$5,$6,$7) on conflict (tenant_bucket,tenant_id,object_id,record_id,group_id,rule_id) do update set access_level=excluded.access_level,projected_at=clock_timestamp()", tenant.Bucket, tenant.TenantID, objectID, id, groupID, access, ruleID); err != nil {
				return err
			}
		}
		if len(ids) < batch {
			// A record can change organization while this rule is building. The
			// record trigger intentionally withholds new edges until the rule is
			// ready, so a final completeness check prevents a prematurely-ready
			// rule from silently missing that earlier record. Restarting from the
			// beginning remains bounded by the caller's batch size and fail-closed.
			var missing bool
			if err := tx.QueryRow(ctx, `select exists(
				select 1 from object_record record
				where record.tenant_bucket=$1 and record.tenant_id=$2 and record.object_id=$3
				  and record.data_organization_id=$4 and record.lifecycle_state='active'
				  and not exists(
					select 1 from share_projection projection
					where projection.tenant_bucket=record.tenant_bucket and projection.tenant_id=record.tenant_id
					  and projection.object_id=record.object_id and projection.record_id=record.record_id and projection.rule_id=$5
				  )
			)`, tenant.Bucket, tenant.TenantID, objectID, organizationID, ruleID).Scan(&missing); err != nil {
				return err
			}
			if missing {
				if _, err := tx.Exec(ctx, "update sharing_rule_def set projection_cursor=null,updated_at=clock_timestamp() where rule_id=$1", ruleID); err != nil {
					return err
				}
			} else if _, err := tx.Exec(ctx, "update sharing_rule_def set projection_state='ready',projection_cursor=null,updated_at=clock_timestamp() where rule_id=$1", ruleID); err != nil {
				return err
			}
		} else if _, err := tx.Exec(ctx, "update sharing_rule_def set projection_cursor=$2,updated_at=clock_timestamp() where rule_id=$1", ruleID, ids[len(ids)-1]); err != nil {
			return err
		}
		return scanSharingRule(tx.QueryRow(ctx, "select rule_id,object_id,name,grantee_group_id,access_level,lifecycle_state,projection_state from sharing_rule_def where rule_id=$1", ruleID), &result)
	})
	if err != nil {
		if projectionStarted {
			service.markSharingRuleFailed(ctx, tenant, ruleID, err)
		}
		return SharingRule{}, mapError(err)
	}
	return result, nil
}

func (service *Service) markSharingRuleFailed(ctx context.Context, tenant database.TenantContext, ruleID uuid.UUID, cause error) {
	message := cause.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	// The projection can fail because its caller timed out or was cancelled.
	// Use a short independent recovery context so that cancellation does not
	// leave the rule indefinitely in building with no actionable error state.
	recoveryContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = database.WithTenant(recoveryContext, service.pool, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(recoveryContext, `update sharing_rule_def set projection_state='failed',projection_error=$4,updated_at=clock_timestamp()
			where tenant_bucket=$1 and tenant_id=$2 and rule_id=$3 and projection_state='building'`, tenant.Bucket, tenant.TenantID, ruleID, message)
		return err
	})
}

// RetrySharingRule makes a failed asynchronous projection explicit and
// recoverable. It never exposes stale partial rows: old projections are
// removed and the rule remains fail-closed until a refresh reaches ready.
func (service *Service) RetrySharingRule(ctx context.Context, request capability.Request, input RetrySharingRuleInput) (SharingRule, *capability.StableError) {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return SharingRule{}, stableErr
	}
	ruleID, stableErr := requiredID(input.RuleID, "rule_id")
	if stableErr != nil {
		return SharingRule{}, stableErr
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return SharingRule{}, stableErr
	}
	var result SharingRule
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceRecordShare, "rule"); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `update sharing_rule_def set projection_state='building',projection_cursor=null,projection_error=null,projection_revision=projection_revision+1,updated_at=clock_timestamp()
			where tenant_bucket=$1 and tenant_id=$2 and rule_id=$3 and projection_state='failed'`, tenant.Bucket, tenant.TenantID, ruleID)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return preconditionError("sharing rule is not failed")
		}
		if _, err := tx.Exec(ctx, "delete from share_projection where tenant_bucket=$1 and tenant_id=$2 and rule_id=$3", tenant.Bucket, tenant.TenantID, ruleID); err != nil {
			return err
		}
		return scanSharingRule(tx.QueryRow(ctx, "select rule_id,object_id,name,grantee_group_id,access_level,lifecycle_state,projection_state from sharing_rule_def where rule_id=$1", ruleID), &result)
	})
	if err != nil {
		return SharingRule{}, mapError(err)
	}
	return result, nil
}

func (service *Service) SetRoleConflict(ctx context.Context, request capability.Request, input SetRoleConflictInput) *capability.StableError {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return stableErr
	}
	roleID, stableErr := requiredID(input.RoleID, "role_id")
	if stableErr != nil {
		return stableErr
	}
	conflictID, stableErr := requiredID(input.ConflictingRoleID, "conflicting_role_id")
	if stableErr != nil {
		return stableErr
	}
	if roleID == conflictID {
		return validationError("role_id and conflicting_role_id must differ")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return stableErr
	}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceAuthorizationRole, "update"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "insert into role_conflict(tenant_bucket,tenant_id,role_id,conflicting_role_id) values ($1,$2,$3,$4),($1,$2,$4,$3) on conflict do nothing", tenant.Bucket, tenant.TenantID, roleID, conflictID); err != nil {
			return err
		}
		return nil
	})
	return mapStableError(err)
}

func (service *Service) ExplainAccess(ctx context.Context, request capability.Request, input ExplainAccessInput) (AccessExplanation, *capability.StableError) {
	objectID, stableErr := requiredID(input.ObjectID, "object_id")
	if stableErr != nil {
		return AccessExplanation{}, stableErr
	}
	recordID, stableErr := requiredID(input.RecordID, "record_id")
	if stableErr != nil {
		return AccessExplanation{}, stableErr
	}
	if strings.TrimSpace(input.PrincipalID) == "" || !(input.Action == ActionRead || input.Action == ActionUpdate || input.Action == ActionDelete) {
		return AccessExplanation{}, validationError("principal_id or action is invalid")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return AccessExplanation{}, stableErr
	}
	result := AccessExplanation{PrincipalID: input.PrincipalID, ObjectID: objectID.String(), RecordID: recordID.String(), Action: input.Action, Sources: []string{}}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, "audit", "read"); err != nil {
			return err
		}
		if _, err := service.evaluator.RequireObject(ctx, tx, tenant, input.PrincipalID, objectID.String(), input.Action); err != nil {
			return err
		}
		scope, err := service.evaluator.RecordScope(ctx, tx, tenant, input.PrincipalID, objectID.String(), input.Action)
		if err != nil {
			return err
		}
		var owner string
		var dataOrganization *uuid.UUID
		var recordData json.RawMessage
		if err := tx.QueryRow(ctx, "select coalesce(owner_id,''),data_organization_id,data from object_record where tenant_bucket=$1 and tenant_id=$2 and object_id=$3 and record_id=$4", tenant.Bucket, tenant.TenantID, objectID, recordID).Scan(&owner, &dataOrganization, &recordData); err != nil {
			return err
		}
		if scope.AllowAll {
			result.Sources = append(result.Sources, "object_default")
		}
		if scope.AllowOwner && owner == input.PrincipalID {
			result.Sources = append(result.Sources, "owner")
		}
		if dataOrganization != nil {
			for _, id := range scope.Organizations {
				if id == *dataOrganization {
					result.Sources = append(result.Sources, "organization_scope")
					break
				}
			}
			if len(scope.DescendantRoots) > 0 {
				var matched bool
				if err := tx.QueryRow(ctx, "select exists(select 1 from organization_closure where tenant_bucket=$1 and tenant_id=$2 and descendant_organization_id=$3 and ancestor_organization_id=any($4::uuid[]))", tenant.Bucket, tenant.TenantID, *dataOrganization, scope.DescendantRoots).Scan(&matched); err != nil {
					return err
				}
				if matched {
					result.Sources = append(result.Sources, "organization_descendants")
				}
			}
		}
		for _, condition := range scope.Conditions {
			var matched bool
			if err := tx.QueryRow(ctx, "select $1::jsonb @> $2::jsonb", string(recordData), string(condition)).Scan(&matched); err != nil {
				return err
			}
			if matched {
				result.Sources = append(result.Sources, "conditional_scope")
				break
			}
		}
		levels := accessLevels(input.Action)
		var team, direct, rule bool
		if err := tx.QueryRow(ctx, "select exists(select 1 from record_team_member where tenant_bucket=$1 and tenant_id=$2 and object_id=$3 and record_id=$4 and principal_id=$5 and lifecycle_state='active' and access_level=any($6::varchar[]))", tenant.Bucket, tenant.TenantID, objectID, recordID, input.PrincipalID, levels).Scan(&team); err != nil {
			return err
		}
		if team {
			result.Sources = append(result.Sources, "team")
		}
		if err := tx.QueryRow(ctx, `select exists(select 1 from share_grant shared where shared.tenant_bucket=$1 and shared.tenant_id=$2 and shared.object_id=$3 and shared.record_id=$4 and shared.lifecycle_state='active' and shared.access_level=any($6::varchar[]) and (shared.grantee_type='principal' and shared.grantee_ref=$5 or shared.grantee_type='group' and exists(select 1 from group_membership membership join access_group access_group on access_group.tenant_bucket=membership.tenant_bucket and access_group.tenant_id=membership.tenant_id and access_group.group_id=membership.group_id where membership.tenant_bucket=shared.tenant_bucket and membership.tenant_id=shared.tenant_id and membership.group_id::text=shared.grantee_ref and membership.principal_id=$5 and membership.membership_state='active' and access_group.lifecycle_state='active')))`, tenant.Bucket, tenant.TenantID, objectID, recordID, input.PrincipalID, levels).Scan(&direct); err != nil {
			return err
		}
		if direct {
			result.Sources = append(result.Sources, "share_grant")
		}
		if err := tx.QueryRow(ctx, `select exists(select 1 from share_projection projection join sharing_rule_def rule on rule.tenant_bucket=projection.tenant_bucket and rule.tenant_id=projection.tenant_id and rule.rule_id=projection.rule_id join group_membership membership on membership.tenant_bucket=projection.tenant_bucket and membership.tenant_id=projection.tenant_id and membership.group_id=projection.group_id join access_group access_group on access_group.tenant_bucket=projection.tenant_bucket and access_group.tenant_id=projection.tenant_id and access_group.group_id=projection.group_id where projection.tenant_bucket=$1 and projection.tenant_id=$2 and projection.object_id=$3 and projection.record_id=$4 and projection.access_level=any($6::varchar[]) and rule.lifecycle_state='active' and rule.projection_state='ready' and access_group.lifecycle_state='active' and membership.principal_id=$5 and membership.membership_state='active')`, tenant.Bucket, tenant.TenantID, objectID, recordID, input.PrincipalID, levels).Scan(&rule); err != nil {
			return err
		}
		if rule {
			result.Sources = append(result.Sources, "sharing_rule")
		}
		result.Allowed = len(result.Sources) > 0
		eventData, err := json.Marshal(map[string]any{"decision": result.Allowed, "sources": result.Sources, "subject_principal_id": input.PrincipalID, "object_id": objectID.String(), "record_id": recordID.String(), "action": input.Action})
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "insert into audit_event(audit_id,request_id,tenant_bucket,tenant_id,actor_id,capability_id,status,event_data) values ($1,$2,$3,$4,$5,'authorization.access.explain','succeeded',$6)", uuid.New(), request.RequestID, tenant.Bucket, tenant.TenantID, request.Actor.ID, eventData)
		return err
	})
	if err != nil {
		return AccessExplanation{}, mapError(err)
	}
	return result, nil
}

func (service *Service) StartOrganizationMerge(ctx context.Context, request capability.Request, input StartOrganizationMergeInput) (OrganizationMergeOperation, *capability.StableError) {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return OrganizationMergeOperation{}, stableErr
	}
	sourceID, stableErr := requiredID(input.SourceOrganizationID, "source_organization_id")
	if stableErr != nil {
		return OrganizationMergeOperation{}, stableErr
	}
	targetID, stableErr := requiredID(input.TargetOrganizationID, "target_organization_id")
	if stableErr != nil {
		return OrganizationMergeOperation{}, stableErr
	}
	if sourceID == targetID {
		return OrganizationMergeOperation{}, validationError("source and target organizations must differ")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return OrganizationMergeOperation{}, stableErr
	}
	var result OrganizationMergeOperation
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceOrganization, "merge"); err != nil {
			return err
		}
		var sourceState, targetState string
		if err := tx.QueryRow(ctx, "select lifecycle_state from organization_node where tenant_bucket=$1 and tenant_id=$2 and organization_id=$3 for update", tenant.Bucket, tenant.TenantID, sourceID).Scan(&sourceState); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, "select lifecycle_state from organization_node where tenant_bucket=$1 and tenant_id=$2 and organization_id=$3 for update", tenant.Bucket, tenant.TenantID, targetID).Scan(&targetState); err != nil {
			return err
		}
		if sourceState != "active" || targetState != "active" {
			return preconditionError("source and target organizations must be active")
		}
		var targetIsDescendant bool
		if err := tx.QueryRow(ctx, "select exists(select 1 from organization_closure where tenant_bucket=$1 and tenant_id=$2 and ancestor_organization_id=$3 and descendant_organization_id=$4 and depth>0)", tenant.Bucket, tenant.TenantID, sourceID, targetID).Scan(&targetIsDescendant); err != nil {
			return err
		}
		if targetIsDescendant {
			return validationError("target organization cannot be a descendant of source")
		}
		operationID := uuid.New()
		if _, err := tx.Exec(ctx, "update organization_node set lifecycle_state='migrating',merged_into_organization_id=$4,updated_at=clock_timestamp() where tenant_bucket=$1 and tenant_id=$2 and organization_id=$3", tenant.Bucket, tenant.TenantID, sourceID, targetID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "insert into organization_merge_operation(tenant_bucket,tenant_id,operation_id,source_organization_id,target_organization_id,state,approval_id,initiated_by) values ($1,$2,$3,$4,$5,'running',$6,$7)", tenant.Bucket, tenant.TenantID, operationID, sourceID, targetID, input.ApprovalID, request.Actor.ID); err != nil {
			return err
		}
		return scanMergeOperation(tx.QueryRow(ctx, "select operation_id,source_organization_id,target_organization_id,state,records_migrated,state='completed' from organization_merge_operation where operation_id=$1", operationID), &result)
	})
	if err != nil {
		return OrganizationMergeOperation{}, mapError(err)
	}
	return result, nil
}

func (service *Service) ExecuteOrganizationMerge(ctx context.Context, request capability.Request, input ExecuteOrganizationMergeInput) (OrganizationMergeOperation, *capability.StableError) {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return OrganizationMergeOperation{}, stableErr
	}
	operationID, stableErr := requiredID(input.OperationID, "operation_id")
	if stableErr != nil {
		return OrganizationMergeOperation{}, stableErr
	}
	batchSize := input.BatchSize
	if batchSize == 0 {
		batchSize = 500
	}
	if batchSize < 1 || batchSize > 1000 {
		return OrganizationMergeOperation{}, validationError("batch_size must be between 1 and 1000")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return OrganizationMergeOperation{}, stableErr
	}
	var result OrganizationMergeOperation
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceOrganization, "merge"); err != nil {
			return err
		}
		var sourceID, targetID uuid.UUID
		var state string
		if err := tx.QueryRow(ctx, "select source_organization_id,target_organization_id,state from organization_merge_operation where tenant_bucket=$1 and tenant_id=$2 and operation_id=$3 for update", tenant.Bucket, tenant.TenantID, operationID).Scan(&sourceID, &targetID, &state); err != nil {
			return err
		}
		if state != "running" {
			return preconditionError("organization merge is not running")
		}
		rows, err := tx.Query(ctx, "select object_id,record_id from object_record where data_organization_id=$1 and lifecycle_state='active' order by record_id limit $2 for update", sourceID, batchSize)
		if err != nil {
			return err
		}
		var records [][2]uuid.UUID
		for rows.Next() {
			var objectID, recordID uuid.UUID
			if err := rows.Scan(&objectID, &recordID); err != nil {
				rows.Close()
				return err
			}
			records = append(records, [2]uuid.UUID{objectID, recordID})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, record := range records {
			if _, err := tx.Exec(ctx, "update object_record set data_organization_id=$4,revision=revision+1,updated_by=$5,updated_at=clock_timestamp() where tenant_bucket=$1 and tenant_id=$2 and object_id=$3 and record_id=$6", tenant.Bucket, tenant.TenantID, record[0], targetID, request.Actor.ID, record[1]); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, "insert into record_organization_history(tenant_bucket,tenant_id,history_id,operation_id,object_id,record_id,from_organization_id,to_organization_id,changed_by) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)", tenant.Bucket, tenant.TenantID, uuid.New(), operationID, record[0], record[1], sourceID, targetID, request.Actor.ID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, "update organization_merge_operation set records_migrated=records_migrated+$4,last_record_id=$5,updated_at=clock_timestamp() where tenant_bucket=$1 and tenant_id=$2 and operation_id=$3", tenant.Bucket, tenant.TenantID, operationID, len(records), nullableMergeRecordID(records)); err != nil {
			return err
		}
		var remaining bool
		if err := tx.QueryRow(ctx, "select exists(select 1 from object_record where data_organization_id=$1 and lifecycle_state='active')", sourceID).Scan(&remaining); err != nil {
			return err
		}
		if !remaining {
			if err := finalizeOrganizationMerge(ctx, tx, tenant, sourceID, targetID, operationID); err != nil {
				return err
			}
		}
		return scanMergeOperation(tx.QueryRow(ctx, "select operation_id,source_organization_id,target_organization_id,state,records_migrated,state='completed' from organization_merge_operation where operation_id=$1", operationID), &result)
	})
	if err != nil {
		return OrganizationMergeOperation{}, mapError(err)
	}
	return result, nil
}

func (service *Service) CancelOrganizationMerge(ctx context.Context, request capability.Request, input CancelOrganizationMergeInput) (OrganizationMergeOperation, *capability.StableError) {
	if stableErr := requireVerifiedApproval(request, input.ApprovalID); stableErr != nil {
		return OrganizationMergeOperation{}, stableErr
	}
	operationID, stableErr := requiredID(input.OperationID, "operation_id")
	if stableErr != nil {
		return OrganizationMergeOperation{}, stableErr
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return OrganizationMergeOperation{}, stableErr
	}
	var result OrganizationMergeOperation
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := service.requirePlatform(ctx, tx, tenant, request.Actor.ID, resourceOrganization, "merge"); err != nil {
			return err
		}
		var sourceID uuid.UUID
		var state string
		var migrated int64
		if err := tx.QueryRow(ctx, "select source_organization_id,state,records_migrated from organization_merge_operation where tenant_bucket=$1 and tenant_id=$2 and operation_id=$3 for update", tenant.Bucket, tenant.TenantID, operationID).Scan(&sourceID, &state, &migrated); err != nil {
			return err
		}
		if state != "running" || migrated != 0 {
			return preconditionError("only a merge with no migrated records can be cancelled")
		}
		if _, err := tx.Exec(ctx, "update organization_node set lifecycle_state='active',merged_into_organization_id=null,updated_at=clock_timestamp() where tenant_bucket=$1 and tenant_id=$2 and organization_id=$3 and lifecycle_state='migrating'", tenant.Bucket, tenant.TenantID, sourceID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "update organization_merge_operation set state='cancelled',updated_at=clock_timestamp() where tenant_bucket=$1 and tenant_id=$2 and operation_id=$3", tenant.Bucket, tenant.TenantID, operationID); err != nil {
			return err
		}
		return scanMergeOperation(tx.QueryRow(ctx, "select operation_id,source_organization_id,target_organization_id,state,records_migrated,state='completed' from organization_merge_operation where operation_id=$1", operationID), &result)
	})
	if err != nil {
		return OrganizationMergeOperation{}, mapError(err)
	}
	return result, nil
}

func (service *Service) requirePlatform(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID, resourceRef, action string) error {
	return service.evaluator.RequirePermission(ctx, tx, tenant, principalID, "platform", resourceRef, action)
}

func (service *Service) tenantContext(ctx context.Context, request capability.Request) (database.TenantContext, *capability.StableError) {
	if request.Principal == nil || request.Principal.TenantID != request.TenantID || request.Principal.CompanyID == "" || request.Principal.Actor.ID != request.Actor.ID {
		return database.TenantContext{}, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted tenant identity is required"}
	}
	tenantID, err := uuid.Parse(request.TenantID)
	if err != nil || tenantID == uuid.Nil {
		return database.TenantContext{}, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted tenant identity is invalid"}
	}
	var bucket int16
	var status string
	err = service.router.QueryRow(ctx, "select tenant_bucket,native_status from tenant_registry where tenant_id=$1 and company_id=$2", tenantID, request.Principal.CompanyID).Scan(&bucket, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return database.TenantContext{}, notFoundError("active Native tenant projection was not found")
	}
	if err != nil {
		return database.TenantContext{}, internalError()
	}
	if status != "active" {
		return database.TenantContext{}, preconditionError("Native tenant is not active")
	}
	return database.TenantContext{TenantID: tenantID, Bucket: bucket, ActorID: request.Actor.ID}, nil
}

// persistCapabilityAudit is deliberately best-effort because the business
// mutation may already have committed when a generic Capability handler
// returns. Per-operation transactional audit remains in the mutating domain
// transaction where available (for example access.explain). This wrapper
// ensures management attempts, including rejections, are never silently
// absent from the durable tenant audit ledger.
func (service *Service) persistCapabilityAudit(ctx context.Context, request capability.Request, capabilityID string, stableErr *capability.StableError) {
	tenant, tenantErr := service.tenantContext(ctx, request)
	if tenantErr != nil {
		return
	}
	var input map[string]any
	if err := json.Unmarshal(request.Input, &input); err != nil {
		input = map[string]any{"input_decode": "failed"}
	}
	delete(input, "approval_id")
	status := "succeeded"
	eventData := map[string]any{"input": input}
	if stableErr != nil {
		status = "failed"
		eventData["error_code"] = stableErr.Code
	}
	encoded, err := json.Marshal(eventData)
	if err != nil {
		return
	}
	_ = database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "insert into audit_event(audit_id,request_id,tenant_bucket,tenant_id,actor_id,capability_id,status,event_data) values ($1,$2,$3,$4,$5,$6,$7,$8)", uuid.New(), request.RequestID, tenant.Bucket, tenant.TenantID, request.Actor.ID, capabilityID, status, encoded)
		return err
	})
}

func optionalID(raw, field string) (uuid.UUID, *capability.StableError) {
	if raw == "" {
		return uuid.Nil, nil
	}
	return requiredID(raw, field)
}

func requiredID(raw, field string) (uuid.UUID, *capability.StableError) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, validationError(field + " must be a UUID")
	}
	return id, nil
}

func validName(value string) bool { return strings.TrimSpace(value) != "" && len(value) <= 200 }
func requireVerifiedApproval(request capability.Request, approvalID string) *capability.StableError {
	if approvalID == "" || request.Principal == nil {
		return preconditionError("a verified approval is required")
	}
	for _, verified := range request.Principal.Approvals {
		if verified == approvalID {
			return nil
		}
	}
	return preconditionError("a verified approval is required")
}
func validAccessLevel(value string) bool {
	return value == "read" || value == "update" || value == "delete"
}
func validAccessAction(value string) bool {
	return value == ActionRead || value == ActionUpdate || value == ActionDelete
}
func validDataScope(value string) bool {
	return value == "own" || value == "organization" || value == "organization_descendants" || value == "assigned_organizations" || value == "all_tenant" || value == "conditional"
}

var conditionalDataScopeField = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,127}$`)

// normalizeConditionalDataScope deliberately supports a small, data-only
// subset rather than arbitrary predicates. The stored JSON object is passed
// to PostgreSQL's jsonb containment operator, so it cannot become SQL text.
// A conditional scope grants a record only when every supplied data field is
// equal to the corresponding scalar value.
func normalizeConditionalDataScope(raw json.RawMessage) (json.RawMessage, *capability.StableError) {
	if len(raw) == 0 {
		return nil, validationError("conditional scope requires condition.equals")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope) != 1 || envelope["equals"] == nil {
		return nil, validationError("conditional scope condition must contain only equals")
	}
	var equals map[string]json.RawMessage
	if err := json.Unmarshal(envelope["equals"], &equals); err != nil || len(equals) == 0 || len(equals) > 5 {
		return nil, validationError("conditional scope equals must contain one to five fields")
	}
	return normalizeConditionalDataFields(equals)
}

// normalizeStoredConditionalDataScope validates the compact expression stored
// in role_data_scope. The public input has an `equals` envelope; the stored
// expression is only the JSONB containment object used by the record PDP.
func normalizeStoredConditionalDataScope(raw json.RawMessage) (json.RawMessage, *capability.StableError) {
	var equals map[string]json.RawMessage
	if err := json.Unmarshal(raw, &equals); err != nil || len(equals) == 0 || len(equals) > 5 {
		return nil, validationError("stored conditional scope is invalid")
	}
	return normalizeConditionalDataFields(equals)
}

func normalizeConditionalDataFields(equals map[string]json.RawMessage) (json.RawMessage, *capability.StableError) {
	for field, value := range equals {
		if !conditionalDataScopeField.MatchString(field) {
			return nil, validationError("conditional scope field name is invalid")
		}
		var decoded any
		if err := json.Unmarshal(value, &decoded); err != nil {
			return nil, validationError("conditional scope value is invalid JSON")
		}
		switch decoded.(type) {
		case string, float64, bool:
		default:
			return nil, validationError("conditional scope values must be scalar string, number, or boolean")
		}
	}
	normalized, err := json.Marshal(equals)
	if err != nil || len(normalized) > 4096 {
		return nil, validationError("conditional scope expression is too large")
	}
	return normalized, nil
}
func accessLevels(action string) []string {
	if action == ActionDelete {
		return []string{"delete"}
	}
	if action == ActionUpdate {
		return []string{"update", "delete"}
	}
	return []string{"read", "update", "delete"}
}
func validPermission(resourceType, resourceRef, action string) bool {
	if resourceType != "platform" && resourceType != "object" && resourceType != "field" {
		return false
	}
	return strings.TrimSpace(resourceRef) != "" && len(resourceRef) <= 200 && strings.TrimSpace(action) != "" && len(action) <= 32
}

func validateGrantee(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, granteeType, granteeRef string) error {
	if granteeType == "principal" {
		var exists bool
		if err := tx.QueryRow(ctx, "select exists(select 1 from principal_projection where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and status='active')", tenant.Bucket, tenant.TenantID, granteeRef).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return pgx.ErrNoRows
		}
		return nil
	}
	groupID, err := uuid.Parse(granteeRef)
	if err != nil || groupID == uuid.Nil {
		return validationError("group grantee_ref must be a UUID")
	}
	var exists bool
	if err := tx.QueryRow(ctx, "select exists(select 1 from access_group where tenant_bucket=$1 and tenant_id=$2 and group_id=$3 and lifecycle_state='active')", tenant.Bucket, tenant.TenantID, groupID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	return nil
}

func scanRole(row interface{ Scan(...any) error }, target *Role) error {
	return row.Scan(&target.RoleID, &target.Name, &target.Description, &target.LifecycleState)
}
func scanPermissionSet(row interface{ Scan(...any) error }, target *PermissionSet) error {
	return row.Scan(&target.PermissionSetID, &target.Name, &target.Description, &target.LifecycleState)
}
func scanShareGrant(row interface{ Scan(...any) error }, target *ShareGrant) error {
	return row.Scan(&target.ShareGrantID, &target.ObjectID, &target.RecordID, &target.GranteeType, &target.GranteeRef, &target.AccessLevel, &target.GrantSource, &target.LifecycleState)
}
func scanMergeOperation(row interface{ Scan(...any) error }, target *OrganizationMergeOperation) error {
	return row.Scan(&target.OperationID, &target.SourceOrganizationID, &target.TargetOrganizationID, &target.State, &target.RecordsMigrated, &target.Completed)
}
func scanSharingRule(row interface{ Scan(...any) error }, target *SharingRule) error {
	return row.Scan(&target.RuleID, &target.ObjectID, &target.Name, &target.GranteeGroupID, &target.AccessLevel, &target.LifecycleState, &target.ProjectionState)
}

func nullableMergeRecordID(records [][2]uuid.UUID) any {
	if len(records) == 0 {
		return nil
	}
	return records[len(records)-1][1]
}

func finalizeOrganizationMerge(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, sourceID, targetID, operationID uuid.UUID) error {
	// A target membership wins. Other active source memberships move intact so
	// a user never ends the operation with two active primary memberships.
	if _, err := tx.Exec(ctx, `update principal_org_membership source set membership_state='ended',effective_to=clock_timestamp(),is_primary=false,updated_at=clock_timestamp()
		where source.tenant_bucket=$1 and source.tenant_id=$2 and source.organization_id=$3 and source.membership_state='active'
		  and exists(select 1 from principal_org_membership target where target.tenant_bucket=source.tenant_bucket and target.tenant_id=source.tenant_id and target.principal_id=source.principal_id and target.organization_id=$4 and target.membership_state='active')`, tenant.Bucket, tenant.TenantID, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "update principal_org_membership set organization_id=$4,updated_at=clock_timestamp() where tenant_bucket=$1 and tenant_id=$2 and organization_id=$3 and membership_state='active'", tenant.Bucket, tenant.TenantID, sourceID, targetID); err != nil {
		return err
	}
	// Collapse duplicate fixed organization scopes before moving their anchor.
	if _, err := tx.Exec(ctx, `delete from role_data_scope source using role_data_scope target
		where source.tenant_bucket=$1 and source.tenant_id=$2 and source.organization_id=$3
		  and target.tenant_bucket=source.tenant_bucket and target.tenant_id=source.tenant_id and target.organization_id=$4
		  and target.role_id=source.role_id and target.object_id=source.object_id and target.action=source.action and target.scope_type=source.scope_type`, tenant.Bucket, tenant.TenantID, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "update role_data_scope set organization_id=$4 where tenant_bucket=$1 and tenant_id=$2 and organization_id=$3", tenant.Bucket, tenant.TenantID, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "update organization_node set parent_organization_id=$4,updated_at=clock_timestamp() where tenant_bucket=$1 and tenant_id=$2 and parent_organization_id=$3", tenant.Bucket, tenant.TenantID, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "update organization_node set lifecycle_state='merged',parent_organization_id=null,merged_into_organization_id=$4,updated_at=clock_timestamp() where tenant_bucket=$1 and tenant_id=$2 and organization_id=$3", tenant.Bucket, tenant.TenantID, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "delete from organization_closure where tenant_bucket=$1 and tenant_id=$2", tenant.Bucket, tenant.TenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `with recursive tree as (
		select organization_id as ancestor_organization_id,organization_id as descendant_organization_id,0 as depth
		from organization_node where tenant_bucket=$1 and tenant_id=$2 and lifecycle_state <> 'merged'
		union all
		select tree.ancestor_organization_id,node.organization_id,tree.depth+1
		from tree join organization_node node on node.tenant_bucket=$1 and node.tenant_id=$2 and node.parent_organization_id=tree.descendant_organization_id
		where node.lifecycle_state <> 'merged'
	) insert into organization_closure(tenant_bucket,tenant_id,ancestor_organization_id,descendant_organization_id,depth)
	select $1,$2,ancestor_organization_id,descendant_organization_id,depth from tree`, tenant.Bucket, tenant.TenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "delete from permission_snapshot where tenant_bucket=$1 and tenant_id=$2", tenant.Bucket, tenant.TenantID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "update organization_merge_operation set state='completed',completed_at=clock_timestamp(),updated_at=clock_timestamp() where tenant_bucket=$1 and tenant_id=$2 and operation_id=$3", tenant.Bucket, tenant.TenantID, operationID)
	return err
}

func validationError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeValidationFailed, Message: message}
}
func preconditionError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeFailedPrecondition, Message: message}
}
func notFoundError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeResourceNotFound, Message: message}
}
func internalError() *capability.StableError {
	return &capability.StableError{Code: capability.CodeInternal, Message: "authorization operation failed"}
}
func mapStableError(err error) *capability.StableError {
	if err == nil {
		return nil
	}
	return mapError(err)
}
func mapError(err error) *capability.StableError {
	var stable *capability.StableError
	if errors.As(err, &stable) {
		return stable
	}
	if errors.Is(err, ErrDenied) {
		return &capability.StableError{Code: capability.CodeUnauthorized, Message: "actor lacks the required authorization permission"}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return notFoundError("authorization resource was not found")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23503") {
		return validationError("authorization relation already exists or is invalid")
	}
	return internalError()
}
