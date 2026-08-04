package principal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool   *pgxpool.Pool
	router database.RowQuerier
}

type SyncInput struct {
	DisplayName string `json:"display_name,omitempty"`
	PublicID    string `json:"public_id,omitempty"`
}

type ListInput struct {
	PrincipalType string `json:"principal_type,omitempty"`
	Status        string `json:"status,omitempty"`
}

type SetStatusInput struct {
	PrincipalID string `json:"principal_id"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	ApprovalID  string `json:"approval_id"`
}

type SetOrganizationMembershipInput struct {
	PrincipalID    string `json:"principal_id"`
	OrganizationID string `json:"organization_id"`
	Active         bool   `json:"active"`
	Primary        bool   `json:"primary"`
	ApprovalID     string `json:"approval_id"`
}

type OrganizationMembership struct {
	MembershipID   string `json:"membership_id"`
	PrincipalID    string `json:"principal_id"`
	OrganizationID string `json:"organization_id"`
	Status         string `json:"status"`
	Primary        bool   `json:"primary"`
}

type Projection struct {
	PrincipalID      string     `json:"principal_id"`
	PrincipalType    string     `json:"principal_type"`
	Status           string     `json:"status"`
	DisplayName      string     `json:"display_name,omitempty"`
	PublicID         string     `json:"public_id,omitempty"`
	OwnerPrincipalID string     `json:"owner_principal_id,omitempty"`
	ClientID         string     `json:"client_id,omitempty"`
	AuthoritySource  string     `json:"authority_source"`
	IdentityVersion  int64      `json:"identity_version"`
	LastSyncedAt     *time.Time `json:"last_synced_at,omitempty"`
}

type ListResult struct {
	Principals []Projection `json:"principals"`
}

func NewService(pool *pgxpool.Pool, router database.RowQuerier) *Service {
	if pool == nil || router == nil {
		panic("principal projection service requires runtime pool and tenant router")
	}
	return &Service{pool: pool, router: router}
}

func (service *Service) Sync(ctx context.Context, request capability.Request, input SyncInput) (Projection, *capability.StableError) {
	trusted := request.Principal
	if trusted == nil || trusted.PrincipalID == "" || trusted.PrincipalID != request.Actor.ID {
		return Projection{}, unauthenticated("trusted principal is required")
	}
	displayName, stableErr := optionalText(input.DisplayName, 200, "display_name")
	if stableErr != nil {
		return Projection{}, stableErr
	}
	publicID, stableErr := optionalText(input.PublicID, 64, "public_id")
	if stableErr != nil {
		return Projection{}, stableErr
	}
	principalType, stableErr := physicalType(trusted.PrincipalType)
	if stableErr != nil {
		return Projection{}, stableErr
	}
	if principalType == "service" && (trusted.OwnerPrincipalID == "" || trusted.ClientID == "") {
		return Projection{}, unauthenticated("service principal owner and client claims are required")
	}
	tenant, stableErr := service.route(ctx, request)
	if stableErr != nil {
		return Projection{}, stableErr
	}
	var result Projection
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if principalType == "service" {
			var ownerType, ownerStatus string
			if err := tx.QueryRow(ctx, `select principal_type,status from principal_projection
				where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3`, tenant.Bucket, tenant.TenantID, trusted.OwnerPrincipalID).
				Scan(&ownerType, &ownerStatus); err != nil {
				return err
			}
			if ownerType != "user" || ownerStatus != "active" {
				return precondition("service principal owner must be an active HUMAN projection")
			}
		}
		var currentType, currentStatus, currentOwner, currentClient string
		err := tx.QueryRow(ctx, `select principal_type,status,coalesce(owner_principal_id,''),coalesce(client_id,'')
			from principal_projection where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3`,
			tenant.Bucket, tenant.TenantID, trusted.PrincipalID).
			Scan(&currentType, &currentStatus, &currentOwner, &currentClient)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			_, err = tx.Exec(ctx, `insert into principal_projection(
				tenant_bucket,tenant_id,principal_id,principal_type,status,display_name,public_id,
				owner_principal_id,client_id,authority_source,last_synced_at)
				values($1,$2,$3,$4,'active',nullif($5,''),nullif($6,''),nullif($7,''),nullif($8,''),'agentcici',clock_timestamp())`,
				tenant.Bucket, tenant.TenantID, trusted.PrincipalID, principalType, displayName, publicID,
				trusted.OwnerPrincipalID, trusted.ClientID)
			if err != nil {
				return err
			}
		case err != nil:
			return err
		case currentType != principalType || currentOwner != trusted.OwnerPrincipalID || currentClient != trusted.ClientID:
			return conflict("verified principal claims conflict with the existing projection")
		case currentStatus != "active":
			return precondition("principal projection is not active and cannot self-reactivate")
		default:
			_, err = tx.Exec(ctx, `update principal_projection set
				display_name=coalesce(nullif($4,''),display_name),public_id=coalesce(nullif($5,''),public_id),
				identity_version=identity_version+1,last_synced_at=clock_timestamp(),updated_at=clock_timestamp()
				where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3`,
				tenant.Bucket, tenant.TenantID, trusted.PrincipalID, displayName, publicID)
			if err != nil {
				return err
			}
		}
		if err := scanProjection(tx.QueryRow(ctx, projectionSelect+` where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3`,
			tenant.Bucket, tenant.TenantID, trusted.PrincipalID), &result); err != nil {
			return err
		}
		return insertAudit(ctx, tx, request, tenant, map[string]any{"principal_type": principalType, "status": result.Status})
	})
	if err != nil {
		return Projection{}, mapError(err)
	}
	return result, nil
}

func (service *Service) List(ctx context.Context, request capability.Request, input ListInput) (ListResult, *capability.StableError) {
	if stableErr := requireHumanManager(request); stableErr != nil {
		return ListResult{}, stableErr
	}
	physical := ""
	if input.PrincipalType != "" {
		var stableErr *capability.StableError
		physical, stableErr = physicalType(input.PrincipalType)
		if stableErr != nil {
			return ListResult{}, stableErr
		}
	}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status != "" && status != "active" && status != "suspended" && status != "disabled" {
		return ListResult{}, validation("status must be active, suspended or disabled")
	}
	tenant, stableErr := service.route(ctx, request)
	if stableErr != nil {
		return ListResult{}, stableErr
	}
	result := ListResult{Principals: []Projection{}}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, projectionSelect+`
			where tenant_bucket=$1 and tenant_id=$2
			  and ($3='' or principal_type=$3) and ($4='' or status=$4)
			order by created_at,principal_id`, tenant.Bucket, tenant.TenantID, physical, status)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Projection
			if err := scanProjection(rows, &item); err != nil {
				return err
			}
			result.Principals = append(result.Principals, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return insertAudit(ctx, tx, request, tenant, map[string]any{"count": len(result.Principals)})
	})
	if err != nil {
		return ListResult{}, mapError(err)
	}
	return result, nil
}

func (service *Service) SetStatus(ctx context.Context, request capability.Request, input SetStatusInput) (Projection, *capability.StableError) {
	if stableErr := requireHumanManager(request); stableErr != nil {
		return Projection{}, stableErr
	}
	principalID := strings.TrimSpace(input.PrincipalID)
	if principalID == "" || len(principalID) > 200 {
		return Projection{}, validation("principal_id is required")
	}
	if principalID == request.Actor.ID {
		return Projection{}, precondition("a manager cannot change their own projection status")
	}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status != "active" && status != "suspended" && status != "disabled" {
		return Projection{}, validation("status must be active, suspended or disabled")
	}
	reason, stableErr := requiredText(input.Reason, 500, "reason")
	if stableErr != nil {
		return Projection{}, stableErr
	}
	approvalID, stableErr := requiredText(input.ApprovalID, 200, "approval_id")
	if stableErr != nil {
		return Projection{}, stableErr
	}
	if stableErr := requireVerifiedApproval(request, approvalID); stableErr != nil {
		return Projection{}, stableErr
	}
	tenant, stableErr := service.route(ctx, request)
	if stableErr != nil {
		return Projection{}, stableErr
	}
	var result Projection
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `update principal_projection set status=$4,identity_version=identity_version+1,updated_at=clock_timestamp()
			where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3`, tenant.Bucket, tenant.TenantID, principalID, status)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return pgx.ErrNoRows
		}
		if err := scanProjection(tx.QueryRow(ctx, projectionSelect+` where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3`,
			tenant.Bucket, tenant.TenantID, principalID), &result); err != nil {
			return err
		}
		return insertAudit(ctx, tx, request, tenant, map[string]any{"target_principal_id": principalID, "status": status, "reason": reason})
	})
	if err != nil {
		return Projection{}, mapError(err)
	}
	return result, nil
}

func (service *Service) SetOrganizationMembership(ctx context.Context, request capability.Request, input SetOrganizationMembershipInput) (OrganizationMembership, *capability.StableError) {
	if stableErr := requireHumanManager(request); stableErr != nil {
		return OrganizationMembership{}, stableErr
	}
	principalID := strings.TrimSpace(input.PrincipalID)
	if principalID == "" || len(principalID) > 200 {
		return OrganizationMembership{}, validation("principal_id is required")
	}
	organizationID, err := uuid.Parse(strings.TrimSpace(input.OrganizationID))
	if err != nil || organizationID == uuid.Nil {
		return OrganizationMembership{}, validation("organization_id must be a UUID")
	}
	approvalID, stableErr := requiredText(input.ApprovalID, 200, "approval_id")
	if stableErr != nil {
		return OrganizationMembership{}, stableErr
	}
	if stableErr := requireVerifiedApproval(request, approvalID); stableErr != nil {
		return OrganizationMembership{}, stableErr
	}
	tenant, stableErr := service.route(ctx, request)
	if stableErr != nil {
		return OrganizationMembership{}, stableErr
	}
	var result OrganizationMembership
	err = database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		var principalStatus string
		if err := tx.QueryRow(ctx, `select status from principal_projection
			where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 for update`, tenant.Bucket, tenant.TenantID, principalID).Scan(&principalStatus); err != nil {
			return err
		}
		var organizationStatus string
		if err := tx.QueryRow(ctx, `select lifecycle_state from organization_node
			where tenant_bucket=$1 and tenant_id=$2 and organization_id=$3`, tenant.Bucket, tenant.TenantID, organizationID).Scan(&organizationStatus); err != nil {
			return err
		}
		if input.Active && organizationStatus != "active" {
			return precondition("organization must be active")
		}
		if input.Active {
			if input.Primary {
				if _, err := tx.Exec(ctx, `update principal_org_membership
					set membership_state='ended',effective_to=clock_timestamp(),is_primary=false,updated_at=clock_timestamp()
					where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and organization_id<>$4
					  and membership_state='active' and is_primary`, tenant.Bucket, tenant.TenantID, principalID, organizationID); err != nil {
					return err
				}
			}
			var membershipID uuid.UUID
			err := tx.QueryRow(ctx, `select membership_id from principal_org_membership
				where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and organization_id=$4 and membership_state='active'
				order by created_at,membership_id limit 1 for update`, tenant.Bucket, tenant.TenantID, principalID, organizationID).Scan(&membershipID)
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				membershipID = uuid.New()
				if _, err := tx.Exec(ctx, `insert into principal_org_membership(
					tenant_bucket,tenant_id,membership_id,principal_id,organization_id,is_primary)
					values($1,$2,$3,$4,$5,$6)`, tenant.Bucket, tenant.TenantID, membershipID, principalID, organizationID, input.Primary); err != nil {
					return err
				}
			case err != nil:
				return err
			default:
				if _, err := tx.Exec(ctx, `update principal_org_membership set is_primary=$5,updated_at=clock_timestamp()
					where tenant_bucket=$1 and tenant_id=$2 and membership_id=$3 and principal_id=$4`, tenant.Bucket, tenant.TenantID, membershipID, principalID, input.Primary); err != nil {
					return err
				}
			}
		} else {
			tag, err := tx.Exec(ctx, `update principal_org_membership
				set membership_state='ended',effective_to=clock_timestamp(),is_primary=false,updated_at=clock_timestamp()
				where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and organization_id=$4 and membership_state='active'`, tenant.Bucket, tenant.TenantID, principalID, organizationID)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return pgx.ErrNoRows
			}
		}
		if err := scanOrganizationMembership(tx.QueryRow(ctx, `select membership_id,principal_id,organization_id,membership_state,is_primary
			from principal_org_membership where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3 and organization_id=$4
			order by created_at desc,membership_id desc limit 1`, tenant.Bucket, tenant.TenantID, principalID, organizationID), &result); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `delete from permission_snapshot where tenant_bucket=$1 and tenant_id=$2 and principal_id=$3`, tenant.Bucket, tenant.TenantID, principalID); err != nil {
			return err
		}
		return insertAudit(ctx, tx, request, tenant, map[string]any{
			"target_principal_id": principalID,
			"organization_id":     organizationID.String(),
			"membership_status":   result.Status,
			"primary":             result.Primary,
		})
	})
	if err != nil {
		return OrganizationMembership{}, mapError(err)
	}
	return result, nil
}

func (service *Service) route(ctx context.Context, request capability.Request) (database.TenantContext, *capability.StableError) {
	if request.Principal == nil || request.Principal.TenantID != request.TenantID || request.Principal.Actor.ID != request.Actor.ID {
		return database.TenantContext{}, unauthenticated("trusted tenant identity is required")
	}
	tenantID, err := uuid.Parse(request.TenantID)
	if err != nil || tenantID == uuid.Nil {
		return database.TenantContext{}, unauthenticated("trusted tenant identity is invalid")
	}
	var bucket int16
	err = service.router.QueryRow(ctx, `select tenant_bucket from tenant_registry
		where tenant_id=$1 and company_id=$2 and native_status='active' and global_lifecycle_status='active'`,
		tenantID, request.Principal.CompanyID).Scan(&bucket)
	if errors.Is(err, pgx.ErrNoRows) {
		return database.TenantContext{}, &capability.StableError{Code: capability.CodeResourceNotFound, Message: "active Native tenant route was not found"}
	}
	if err != nil {
		return database.TenantContext{}, internal()
	}
	return database.TenantContext{TenantID: tenantID, Bucket: bucket, ActorID: request.Actor.ID}, nil
}

const projectionSelect = `select principal_id,principal_type,status,coalesce(display_name,''),coalesce(public_id,''),
	coalesce(owner_principal_id,''),coalesce(client_id,''),authority_source,identity_version,last_synced_at from principal_projection`

func scanProjection(row interface{ Scan(...any) error }, target *Projection) error {
	var physical string
	if err := row.Scan(&target.PrincipalID, &physical, &target.Status, &target.DisplayName, &target.PublicID,
		&target.OwnerPrincipalID, &target.ClientID, &target.AuthoritySource, &target.IdentityVersion, &target.LastSyncedAt); err != nil {
		return err
	}
	target.PrincipalType = apiType(physical)
	return nil
}

func scanOrganizationMembership(row interface{ Scan(...any) error }, target *OrganizationMembership) error {
	return row.Scan(&target.MembershipID, &target.PrincipalID, &target.OrganizationID, &target.Status, &target.Primary)
}

func requireHumanManager(request capability.Request) *capability.StableError {
	if request.Principal == nil || strings.ToUpper(request.Principal.PrincipalType) != "HUMAN" {
		return &capability.StableError{Code: capability.CodeUnauthorized, Message: "a HUMAN management principal is required"}
	}
	return nil
}

func requireVerifiedApproval(request capability.Request, approvalID string) *capability.StableError {
	if request.Principal == nil {
		return &capability.StableError{Code: capability.CodeUnauthorized, Message: "verified approval is required"}
	}
	for _, candidate := range request.Principal.Approvals {
		if candidate == approvalID {
			return nil
		}
	}
	return &capability.StableError{Code: capability.CodeUnauthorized, Message: "verified approval is required"}
}

func physicalType(value string) (string, *capability.StableError) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "HUMAN", "USER":
		return "user", nil
	case "SERVICE":
		return "service", nil
	case "GROUP":
		return "group", nil
	default:
		return "", validation("principal_type must be HUMAN, SERVICE or GROUP")
	}
}

func apiType(value string) string {
	switch value {
	case "user":
		return "HUMAN"
	case "service":
		return "SERVICE"
	case "group":
		return "GROUP"
	default:
		return strings.ToUpper(value)
	}
}

func optionalText(value string, max int, field string) (string, *capability.StableError) {
	normalized := strings.TrimSpace(value)
	if len([]rune(normalized)) > max {
		return "", validation(field + " is too long")
	}
	return normalized, nil
}

func requiredText(value string, max int, field string) (string, *capability.StableError) {
	normalized, stableErr := optionalText(value, max, field)
	if stableErr != nil {
		return "", stableErr
	}
	if normalized == "" {
		return "", validation(field + " is required")
	}
	return normalized, nil
}

func insertAudit(ctx context.Context, tx pgx.Tx, request capability.Request, tenant database.TenantContext, data map[string]any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into audit_event(audit_id,request_id,tenant_bucket,tenant_id,actor_id,capability_id,status,event_data)
		values($1,$2,$3,$4,$5,$6,'succeeded',$7)`, uuid.New(), request.RequestID, tenant.Bucket, tenant.TenantID,
		request.Actor.ID, request.CapabilityID, encoded)
	return err
}

func mapError(err error) *capability.StableError {
	var stableErr *capability.StableError
	if errors.As(err, &stableErr) {
		return stableErr
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return &capability.StableError{Code: capability.CodeResourceNotFound, Message: "principal projection was not found"}
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return conflict("principal client or public identity already exists")
		case "23503", "23514", "23502", "22001":
			return validation("principal projection violates identity constraints")
		}
	}
	return internal()
}

func validation(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeValidationFailed, Message: message}
}

func unauthenticated(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeUnauthenticated, Message: message}
}

func conflict(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeConflict, Message: message}
}

func precondition(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeFailedPrecondition, Message: message}
}

func internal() *capability.StableError {
	return &capability.StableError{Code: capability.CodeInternal, Message: "principal projection operation failed"}
}
