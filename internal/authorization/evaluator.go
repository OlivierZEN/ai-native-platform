// Package authorization evaluates FEAT-016 business authorization inside an
// already tenant-bound transaction. It deliberately does not authenticate a
// caller: capability.Request.Principal remains the trusted identity boundary.
package authorization

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	// ErrDenied is returned for an enforced object where the principal lacks a
	// required object or field permission.
	ErrDenied = errors.New("authorization denied")
	// ErrNoPrimaryOrganization prevents an enforced create from producing a
	// record without its immutable business data anchor.
	ErrNoPrimaryOrganization = errors.New("principal has no primary organization")
)

const (
	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionWrite  = "write"
)

// ObjectDecision describes whether FEAT-016 is enabled for an object and, if
// enabled, whether the current principal holds the requested object action.
// A disabled policy is intentionally a migration compatibility mode, not an
// authorization grant to be used for newly protected objects.
type ObjectDecision struct {
	Enforced            bool
	DefaultRecordAccess string
}

// RecordScope is converted to SQL by the record runtime. It contains only
// compact organization roots and never materializes record-to-user ACL rows.
type RecordScope struct {
	AllowAll         bool
	AllowOwner       bool
	Organizations    []uuid.UUID
	DescendantRoots  []uuid.UUID
	Conditions       []json.RawMessage
	IncludeTeamShare bool
}

// Evaluator is stateless and safe to share. All reads occur through the
// caller's pgx transaction, where TenantContext has already set RLS GUCs.
type Evaluator struct{}

func NewEvaluator() *Evaluator { return &Evaluator{} }

// RequirePermission enforces a non-record atomic permission. Management
// capabilities use resource_type=platform and a stable resource_ref such as
// "authorization.role" or "record.share". It is intentionally independent
// from an object's gradual enforcement switch: administration is never in
// legacy-allow mode.
func (e *Evaluator) RequirePermission(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID, resourceType, resourceRef, action string) error {
	var allowed bool
	err := tx.QueryRow(ctx, `
		select exists(
			select 1
			from principal_role_assignment assignment
			join principal_projection principal on principal.tenant_bucket=assignment.tenant_bucket and principal.tenant_id=assignment.tenant_id and principal.principal_id=assignment.principal_id
			join authorization_role role on role.tenant_bucket=assignment.tenant_bucket and role.tenant_id=assignment.tenant_id and role.role_id=assignment.role_id
			join role_permission_set role_set on role_set.tenant_bucket=role.tenant_bucket and role_set.tenant_id=role.tenant_id and role_set.role_id=role.role_id
			join permission_set permission_set on permission_set.tenant_bucket=role_set.tenant_bucket and permission_set.tenant_id=role_set.tenant_id and permission_set.permission_set_id=role_set.permission_set_id
			join permission_set_permission set_permission on set_permission.tenant_bucket=permission_set.tenant_bucket and set_permission.tenant_id=permission_set.tenant_id and set_permission.permission_set_id=permission_set.permission_set_id
			join authorization_permission permission on permission.tenant_bucket=set_permission.tenant_bucket and permission.tenant_id=set_permission.tenant_id and permission.permission_id=set_permission.permission_id
			where assignment.tenant_bucket=$1 and assignment.tenant_id=$2 and assignment.principal_id=$3 and assignment.assignment_state='active' and (assignment.effective_to is null or assignment.effective_to>clock_timestamp())
			  and principal.status='active' and role.lifecycle_state='active' and permission_set.lifecycle_state='active'
			  and permission.resource_type=$4 and permission.resource_ref in ($5,'*') and permission.action=$6 and permission.effect='allow'
		)`, tenant.Bucket, tenant.TenantID, principalID, resourceType, resourceRef, action).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	return nil
}

func (e *Evaluator) RequireObject(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID, objectID, action string) (ObjectDecision, error) {
	decision, err := e.objectDecision(ctx, tx, tenant, objectID)
	if err != nil || !decision.Enforced {
		return decision, err
	}
	var allowed bool
	err = tx.QueryRow(ctx, `
		select exists(
			select 1
			from principal_role_assignment assignment
			join principal_projection principal
			  on principal.tenant_bucket=assignment.tenant_bucket and principal.tenant_id=assignment.tenant_id and principal.principal_id=assignment.principal_id
			join authorization_role role
			  on role.tenant_bucket=assignment.tenant_bucket and role.tenant_id=assignment.tenant_id and role.role_id=assignment.role_id
			join role_permission_set role_set
			  on role_set.tenant_bucket=role.tenant_bucket and role_set.tenant_id=role.tenant_id and role_set.role_id=role.role_id
			join permission_set permission_set
			  on permission_set.tenant_bucket=role_set.tenant_bucket and permission_set.tenant_id=role_set.tenant_id and permission_set.permission_set_id=role_set.permission_set_id
			join permission_set_permission set_permission
			  on set_permission.tenant_bucket=permission_set.tenant_bucket and set_permission.tenant_id=permission_set.tenant_id and set_permission.permission_set_id=permission_set.permission_set_id
			join authorization_permission permission
			  on permission.tenant_bucket=set_permission.tenant_bucket and permission.tenant_id=set_permission.tenant_id and permission.permission_id=set_permission.permission_id
			where assignment.tenant_bucket=$1 and assignment.tenant_id=$2
			  and assignment.principal_id=$3 and assignment.assignment_state='active' and (assignment.effective_to is null or assignment.effective_to>clock_timestamp())
			  and principal.status='active' and role.lifecycle_state='active' and permission_set.lifecycle_state='active'
			  and permission.resource_type='object' and permission.resource_ref in ($4::text,'*')
			  and permission.action=$5 and permission.effect='allow'
		)`, tenant.Bucket, tenant.TenantID, principalID, objectID, action).Scan(&allowed)
	if err != nil {
		return ObjectDecision{}, err
	}
	if !allowed {
		return decision, ErrDenied
	}
	return decision, nil
}

// RequireFields verifies every specified field. The caller passes field UUIDs;
// API names never become authorization resource identifiers.
func (e *Evaluator) RequireFields(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID, objectID, action string, fieldIDs []string) error {
	if len(fieldIDs) == 0 {
		return nil
	}
	decision, err := e.objectDecision(ctx, tx, tenant, objectID)
	if err != nil || !decision.Enforced {
		return err
	}
	fieldIDs = uniqueStrings(fieldIDs)
	var granted []string
	err = tx.QueryRow(ctx, `
		select coalesce(array_agg(distinct permission.resource_ref), '{}'::text[])
		from principal_role_assignment assignment
		join principal_projection principal
		  on principal.tenant_bucket=assignment.tenant_bucket and principal.tenant_id=assignment.tenant_id and principal.principal_id=assignment.principal_id
		join authorization_role role
		  on role.tenant_bucket=assignment.tenant_bucket and role.tenant_id=assignment.tenant_id and role.role_id=assignment.role_id
		join role_permission_set role_set
		  on role_set.tenant_bucket=role.tenant_bucket and role_set.tenant_id=role.tenant_id and role_set.role_id=role.role_id
		join permission_set permission_set
		  on permission_set.tenant_bucket=role_set.tenant_bucket and permission_set.tenant_id=role_set.tenant_id and permission_set.permission_set_id=role_set.permission_set_id
		join permission_set_permission set_permission
		  on set_permission.tenant_bucket=permission_set.tenant_bucket and set_permission.tenant_id=permission_set.tenant_id and set_permission.permission_set_id=permission_set.permission_set_id
		join authorization_permission permission
		  on permission.tenant_bucket=set_permission.tenant_bucket and permission.tenant_id=set_permission.tenant_id and permission.permission_id=set_permission.permission_id
		where assignment.tenant_bucket=$1 and assignment.tenant_id=$2
		  and assignment.principal_id=$3 and assignment.assignment_state='active' and (assignment.effective_to is null or assignment.effective_to>clock_timestamp())
		  and principal.status='active' and role.lifecycle_state='active' and permission_set.lifecycle_state='active'
		  and permission.resource_type='field' and permission.resource_ref = any($4::text[])
		  and permission.action=$5 and permission.effect='allow'`,
		tenant.Bucket, tenant.TenantID, principalID, fieldIDs, action).Scan(&granted)
	if err != nil {
		return err
	}
	if len(granted) != len(fieldIDs) {
		return ErrDenied
	}
	return nil
}

// ReadableFieldIDs returns the field UUIDs the principal is permitted to read.
// With a disabled policy, callers preserve legacy field lifecycle filtering.
func (e *Evaluator) ReadableFieldIDs(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID, objectID string, fieldIDs []string) (map[string]bool, error) {
	decision, err := e.objectDecision(ctx, tx, tenant, objectID)
	if err != nil {
		return nil, err
	}
	if !decision.Enforced {
		return nil, nil
	}
	fieldIDs = uniqueStrings(fieldIDs)
	if len(fieldIDs) == 0 {
		return map[string]bool{}, nil
	}
	var granted []string
	err = tx.QueryRow(ctx, `
		select coalesce(array_agg(distinct permission.resource_ref), '{}'::text[])
		from principal_role_assignment assignment
		join principal_projection principal on principal.tenant_bucket=assignment.tenant_bucket and principal.tenant_id=assignment.tenant_id and principal.principal_id=assignment.principal_id
		join authorization_role role on role.tenant_bucket=assignment.tenant_bucket and role.tenant_id=assignment.tenant_id and role.role_id=assignment.role_id
		join role_permission_set role_set on role_set.tenant_bucket=role.tenant_bucket and role_set.tenant_id=role.tenant_id and role_set.role_id=role.role_id
		join permission_set permission_set on permission_set.tenant_bucket=role_set.tenant_bucket and permission_set.tenant_id=role_set.tenant_id and permission_set.permission_set_id=role_set.permission_set_id
		join permission_set_permission set_permission on set_permission.tenant_bucket=permission_set.tenant_bucket and set_permission.tenant_id=permission_set.tenant_id and set_permission.permission_set_id=permission_set.permission_set_id
		join authorization_permission permission on permission.tenant_bucket=set_permission.tenant_bucket and permission.tenant_id=set_permission.tenant_id and permission.permission_id=set_permission.permission_id
		where assignment.tenant_bucket=$1 and assignment.tenant_id=$2 and assignment.principal_id=$3 and assignment.assignment_state='active' and (assignment.effective_to is null or assignment.effective_to>clock_timestamp())
		  and principal.status='active' and role.lifecycle_state='active' and permission_set.lifecycle_state='active'
		  and permission.resource_type='field' and permission.resource_ref = any($4::text[]) and permission.action='read' and permission.effect='allow'`,
		tenant.Bucket, tenant.TenantID, principalID, fieldIDs).Scan(&granted)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(granted))
	for _, id := range granted {
		result[id] = true
	}
	return result, nil
}

func (e *Evaluator) RecordScope(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID, objectID, action string) (RecordScope, error) {
	decision, err := e.RequireObject(ctx, tx, tenant, principalID, objectID, action)
	if err != nil {
		return RecordScope{}, err
	}
	if !decision.Enforced {
		return RecordScope{AllowAll: true}, nil
	}
	scope := RecordScope{AllowOwner: true, IncludeTeamShare: true}
	if action == ActionRead && decision.DefaultRecordAccess == "read_all" {
		scope.AllowAll = true
		return scope, nil
	}
	rows, err := tx.Query(ctx, `
		select distinct data_scope.scope_type, data_scope.organization_id,data_scope.condition_expression
		from principal_role_assignment assignment
		join authorization_role role on role.tenant_bucket=assignment.tenant_bucket and role.tenant_id=assignment.tenant_id and role.role_id=assignment.role_id
		join role_permission_set role_set on role_set.tenant_bucket=role.tenant_bucket and role_set.tenant_id=role.tenant_id and role_set.role_id=role.role_id
		join permission_set permission_set on permission_set.tenant_bucket=role_set.tenant_bucket and permission_set.tenant_id=role_set.tenant_id and permission_set.permission_set_id=role_set.permission_set_id
		join permission_set_permission set_permission on set_permission.tenant_bucket=permission_set.tenant_bucket and set_permission.tenant_id=permission_set.tenant_id and set_permission.permission_set_id=permission_set.permission_set_id
		join authorization_permission permission on permission.tenant_bucket=set_permission.tenant_bucket and permission.tenant_id=set_permission.tenant_id and permission.permission_id=set_permission.permission_id
		join role_data_scope data_scope on data_scope.tenant_bucket=role.tenant_bucket and data_scope.tenant_id=role.tenant_id and data_scope.role_id=role.role_id
		where assignment.tenant_bucket=$1 and assignment.tenant_id=$2 and assignment.principal_id=$3 and assignment.assignment_state='active' and (assignment.effective_to is null or assignment.effective_to>clock_timestamp())
		  and role.lifecycle_state='active' and permission_set.lifecycle_state='active'
		  and permission.resource_type='object' and permission.resource_ref in ($4::text,'*') and permission.action=$5 and permission.effect='allow'
		  and data_scope.object_id=$6::uuid and data_scope.action=$5`,
		tenant.Bucket, tenant.TenantID, principalID, objectID, action, objectID)
	if err != nil {
		return RecordScope{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var scopeType string
		var organizationID *uuid.UUID
		var condition json.RawMessage
		if err := rows.Scan(&scopeType, &organizationID, &condition); err != nil {
			return RecordScope{}, err
		}
		switch scopeType {
		case "all_tenant":
			scope.AllowAll = true
		case "organization":
			if organizationID != nil {
				scope.Organizations = append(scope.Organizations, *organizationID)
			}
		case "organization_descendants":
			if organizationID != nil {
				scope.DescendantRoots = append(scope.DescendantRoots, *organizationID)
			}
		case "assigned_organizations":
			ids, err := activeOrganizations(ctx, tx, tenant, principalID)
			if err != nil {
				return RecordScope{}, err
			}
			scope.DescendantRoots = append(scope.DescendantRoots, ids...)
		case "conditional":
			if normalized, stableErr := normalizeStoredConditionalDataScope(condition); stableErr == nil {
				scope.Conditions = append(scope.Conditions, normalized)
			} else {
				return RecordScope{}, errors.New(stableErr.Message)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return RecordScope{}, err
	}
	scope.Organizations = uniqueUUIDs(scope.Organizations)
	scope.DescendantRoots = uniqueUUIDs(scope.DescendantRoots)
	scope.Conditions = uniqueConditions(scope.Conditions)
	return scope, nil
}

func uniqueConditions(values []json.RawMessage) []json.RawMessage {
	if len(values) < 2 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]json.RawMessage, 0, len(values))
	for _, value := range values {
		key := string(value)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func (e *Evaluator) PrimaryOrganization(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID, objectID string) (*uuid.UUID, error) {
	decision, err := e.objectDecision(ctx, tx, tenant, objectID)
	if err != nil || !decision.Enforced {
		return nil, err
	}
	var organizationID uuid.UUID
	err = tx.QueryRow(ctx, `select membership.organization_id from principal_org_membership membership
		join organization_node organization on organization.tenant_bucket=membership.tenant_bucket and organization.tenant_id=membership.tenant_id and organization.organization_id=membership.organization_id
		where membership.tenant_bucket=$1 and membership.tenant_id=$2 and membership.principal_id=$3 and membership.membership_state='active' and membership.is_primary and organization.lifecycle_state='active'
		limit 1`, tenant.Bucket, tenant.TenantID, principalID).Scan(&organizationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoPrimaryOrganization
	}
	if err != nil {
		return nil, err
	}
	return &organizationID, nil
}

func (e *Evaluator) objectDecision(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, objectID string) (ObjectDecision, error) {
	var state, defaultAccess string
	err := tx.QueryRow(ctx, `select enforcement_state,default_record_access from object_authorization_policy
		where tenant_bucket=$1 and tenant_id=$2 and object_id=$3::uuid`, tenant.Bucket, tenant.TenantID, objectID).Scan(&state, &defaultAccess)
	if errors.Is(err, pgx.ErrNoRows) {
		return ObjectDecision{}, nil
	}
	if err != nil {
		return ObjectDecision{}, err
	}
	return ObjectDecision{Enforced: state == "enforced", DefaultRecordAccess: defaultAccess}, nil
}

func activeOrganizations(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID string) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `select organization_id from principal_org_membership
		where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and membership_state='active'`, tenant.Bucket, tenant.TenantID, principalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func uniqueStrings(values []string) []string {
	sort.Strings(values)
	return uniqueSorted(values)
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return values
	}
	position := 1
	for _, value := range values[1:] {
		if value != values[position-1] {
			values[position] = value
			position++
		}
	}
	return values[:position]
}

func uniqueUUIDs(values []uuid.UUID) []uuid.UUID {
	sort.Slice(values, func(i, j int) bool { return values[i].String() < values[j].String() })
	if len(values) == 0 {
		return values
	}
	position := 1
	for _, value := range values[1:] {
		if value != values[position-1] {
			values[position] = value
			position++
		}
	}
	return values[:position]
}
