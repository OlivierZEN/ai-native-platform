package tenant

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/operations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool       *pgxpool.Pool
	operations operations.Port
}

type ProvisionInput struct {
	OperationID           string          `json:"operation_id"`
	TenantID              string          `json:"tenant_id"`
	CompanyID             string          `json:"company_id"`
	TenantRevision        int64           `json:"tenant_revision"`
	ProductRevision       int64           `json:"product_revision"`
	DisplayName           string          `json:"display_name"`
	ServiceTier           string          `json:"service_tier"`
	GlobalLifecycleStatus string          `json:"global_lifecycle_status"`
	Entitlements          json.RawMessage `json:"entitlements,omitempty"`
}

type LifecycleInput struct {
	OperationID     string `json:"operation_id"`
	ProductRevision int64  `json:"product_revision"`
}

type EntitlementInput struct {
	OperationID     string          `json:"operation_id"`
	ProductRevision int64           `json:"product_revision"`
	Entitlements    json.RawMessage `json:"entitlements"`
}

type TenantStatus struct {
	TenantID              string          `json:"tenant_id"`
	CompanyID             string          `json:"company_id"`
	ShardID               string          `json:"shard_id"`
	TenantBucket          int16           `json:"tenant_bucket"`
	ServiceTier           string          `json:"service_tier"`
	GlobalLifecycleStatus string          `json:"global_lifecycle_status"`
	NativeStatus          string          `json:"native_status"`
	TenantRevision        int64           `json:"tenant_revision"`
	ProductRevision       int64           `json:"product_revision"`
	RouteRevision         int64           `json:"route_revision"`
	Entitlements          json.RawMessage `json:"entitlements"`
	LastOperationID       string          `json:"last_operation_id,omitempty"`
	OperationStatus       string          `json:"operation_status,omitempty"`
}

func NewService(pool *pgxpool.Pool, operationsPort operations.Port) *Service {
	if pool == nil || operationsPort == nil {
		panic("tenant service requires database pool and operations port")
	}
	return &Service{pool: pool, operations: operationsPort}
}

func (service *Service) Provision(ctx context.Context, request capability.Request, input ProvisionInput) (TenantStatus, *capability.StableError) {
	tenantID, stableErr := validateProvisionRequest(request, input)
	if stableErr != nil {
		return TenantStatus{}, stableErr
	}
	if err := service.operations.VerifyGlobalTenant(ctx, operations.GlobalTenant{
		TenantID: tenantID, CompanyID: input.CompanyID, TenantRevision: input.TenantRevision, ProductRevision: input.ProductRevision,
	}); err != nil {
		return TenantStatus{}, validationError(err.Error())
	}
	hash, err := canonicalHash(request.Input)
	if err != nil {
		return TenantStatus{}, validationError("input must be valid JSON")
	}
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return TenantStatus{}, internalError()
	}
	defer tx.Rollback(ctx)
	if replay, found, replayErr := loadOperation(ctx, tx, input.OperationID, tenantID, request.CapabilityID, hash); replayErr != nil {
		return TenantStatus{}, replayErr
	} else if found {
		return replay, nil
	}

	status, found, queryErr := queryStatusByIdentity(ctx, tx, tenantID, input.CompanyID, true)
	if queryErr != nil {
		return TenantStatus{}, internalError()
	}
	entitlements := normalizedEntitlements(input.Entitlements)
	if !found {
		bucket, selectErr := database.SelectLeastUsedBucket(ctx, tx)
		if selectErr != nil {
			return TenantStatus{}, internalError()
		}
		_, err = tx.Exec(ctx,
			"insert into tenant_registry(tenant_id,company_id,display_name,shard_id,tenant_bucket,service_tier,global_lifecycle_status,native_status,tenant_revision,product_revision,route_revision,entitlements,last_operation_id) "+
				"values ($1,$2,$3,'shard-001',$4,$5,$6,'provisioning',$7,$8,1,$9,$10)",
			tenantID, input.CompanyID, input.DisplayName, bucket, input.ServiceTier, input.GlobalLifecycleStatus, input.TenantRevision, input.ProductRevision, entitlements, input.OperationID)
		if err != nil {
			return TenantStatus{}, conflictError("tenant or company identity already exists")
		}
		status, _, err = queryStatusByIdentity(ctx, tx, tenantID, input.CompanyID, true)
		if err != nil {
			return TenantStatus{}, internalError()
		}
	} else {
		if status.TenantID != tenantID.String() || status.CompanyID != input.CompanyID {
			return TenantStatus{}, conflictError("tenant_id and company_id are already bound to a different identity")
		}
		if input.TenantRevision < status.TenantRevision || input.ProductRevision <= status.ProductRevision {
			return TenantStatus{}, preconditionError("tenant or product revision is out of order")
		}
	}
	if err := insertOperation(ctx, tx, input.OperationID, tenantID, status.TenantBucket, request.CapabilityID, hash, input.ProductRevision, "running"); err != nil {
		return TenantStatus{}, conflictError("operation_id is already in use")
	}
	_, err = tx.Exec(ctx,
		"update tenant_registry set display_name=$2,service_tier=$3,global_lifecycle_status=$4,native_status='active',tenant_revision=$5,product_revision=$6,entitlements=$7,last_operation_id=$8,updated_at=clock_timestamp() where tenant_id=$1",
		tenantID, input.DisplayName, input.ServiceTier, input.GlobalLifecycleStatus, input.TenantRevision, input.ProductRevision, entitlements, input.OperationID)
	if err != nil {
		return TenantStatus{}, internalError()
	}
	status, _, err = queryStatusByIdentity(ctx, tx, tenantID, input.CompanyID, false)
	if err != nil {
		return TenantStatus{}, internalError()
	}
	status.OperationStatus = "succeeded"
	if err := completeOperation(ctx, tx, input.OperationID, "succeeded", status); err != nil {
		return TenantStatus{}, internalError()
	}
	if err := insertAudit(ctx, tx, request, status.TenantBucket, input.OperationID, "succeeded", ""); err != nil {
		return TenantStatus{}, internalError()
	}
	if err := tx.Commit(ctx); err != nil {
		return TenantStatus{}, internalError()
	}
	return status, nil
}

func (service *Service) GetStatus(ctx context.Context, request capability.Request) (TenantStatus, *capability.StableError) {
	tenantID, stableErr := trustedTenant(request)
	if stableErr != nil {
		return TenantStatus{}, stableErr
	}
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return TenantStatus{}, internalError()
	}
	defer tx.Rollback(ctx)
	status, found, err := queryStatusByIdentity(ctx, tx, tenantID, request.Principal.CompanyID, false)
	if err != nil {
		return TenantStatus{}, internalError()
	}
	if !found {
		return TenantStatus{}, &capability.StableError{Code: capability.CodeResourceNotFound, Message: "Native tenant projection was not found"}
	}
	if err := insertAudit(ctx, tx, request, status.TenantBucket, "", "succeeded", ""); err != nil {
		return TenantStatus{}, internalError()
	}
	if err := tx.Commit(ctx); err != nil {
		return TenantStatus{}, internalError()
	}
	return status, nil
}

func (service *Service) Suspend(ctx context.Context, request capability.Request, input LifecycleInput) (TenantStatus, *capability.StableError) {
	return service.mutate(ctx, request, input.OperationID, input.ProductRevision, func(ctx context.Context, tx pgx.Tx, status TenantStatus) *capability.StableError {
		if status.NativeStatus == "decommissioned" {
			return preconditionError("decommissioned tenant cannot be suspended")
		}
		_, err := tx.Exec(ctx, "update tenant_registry set native_status='suspended',product_revision=$2,last_operation_id=$3,updated_at=clock_timestamp() where tenant_id=$1", status.TenantID, input.ProductRevision, input.OperationID)
		if err != nil {
			return internalError()
		}
		return nil
	})
}

func (service *Service) Resume(ctx context.Context, request capability.Request, input LifecycleInput) (TenantStatus, *capability.StableError) {
	return service.mutate(ctx, request, input.OperationID, input.ProductRevision, func(ctx context.Context, tx pgx.Tx, status TenantStatus) *capability.StableError {
		if status.GlobalLifecycleStatus != "active" || status.NativeStatus == "decommissioned" {
			return preconditionError("tenant lifecycle does not allow resume")
		}
		_, err := tx.Exec(ctx, "update tenant_registry set native_status='active',product_revision=$2,last_operation_id=$3,updated_at=clock_timestamp() where tenant_id=$1", status.TenantID, input.ProductRevision, input.OperationID)
		if err != nil {
			return internalError()
		}
		return nil
	})
}

func (service *Service) UpdateEntitlement(ctx context.Context, request capability.Request, input EntitlementInput) (TenantStatus, *capability.StableError) {
	if !validJSONObject(input.Entitlements) {
		return TenantStatus{}, validationError("entitlements must be a JSON object")
	}
	return service.mutate(ctx, request, input.OperationID, input.ProductRevision, func(ctx context.Context, tx pgx.Tx, status TenantStatus) *capability.StableError {
		_, err := tx.Exec(ctx, "update tenant_registry set entitlements=$2,product_revision=$3,last_operation_id=$4,updated_at=clock_timestamp() where tenant_id=$1", status.TenantID, input.Entitlements, input.ProductRevision, input.OperationID)
		if err != nil {
			return internalError()
		}
		return nil
	})
}

func (service *Service) RequestDecommission(ctx context.Context, request capability.Request, input LifecycleInput) (TenantStatus, *capability.StableError) {
	tenantID, stableErr := trustedTenant(request)
	if stableErr != nil {
		return TenantStatus{}, stableErr
	}
	hash, err := canonicalHash(request.Input)
	if err != nil {
		return TenantStatus{}, validationError("input must be valid JSON")
	}
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return TenantStatus{}, internalError()
	}
	defer tx.Rollback(ctx)
	if replay, found, replayErr := loadOperation(ctx, tx, input.OperationID, tenantID, request.CapabilityID, hash); replayErr != nil {
		return TenantStatus{}, replayErr
	} else if found {
		return replay, nil
	}
	status, found, err := queryStatusByIdentity(ctx, tx, tenantID, request.Principal.CompanyID, true)
	if err != nil {
		return TenantStatus{}, internalError()
	}
	if !found {
		return TenantStatus{}, &capability.StableError{Code: capability.CodeResourceNotFound, Message: "Native tenant projection was not found"}
	}
	if input.ProductRevision <= status.ProductRevision {
		return TenantStatus{}, preconditionError("product revision is out of order")
	}
	if err := insertOperation(ctx, tx, input.OperationID, tenantID, status.TenantBucket, request.CapabilityID, hash, input.ProductRevision, "pending_approval"); err != nil {
		return TenantStatus{}, conflictError("operation_id is already in use")
	}
	status.OperationStatus = "pending_approval"
	if err := completeOperation(ctx, tx, input.OperationID, "pending_approval", status); err != nil {
		return TenantStatus{}, internalError()
	}
	if err := insertAudit(ctx, tx, request, status.TenantBucket, input.OperationID, "pending_approval", ""); err != nil {
		return TenantStatus{}, internalError()
	}
	if err := tx.Commit(ctx); err != nil {
		return TenantStatus{}, internalError()
	}
	return status, nil
}

func (service *Service) mutate(ctx context.Context, request capability.Request, operationID string, productRevision int64, apply func(context.Context, pgx.Tx, TenantStatus) *capability.StableError) (TenantStatus, *capability.StableError) {
	tenantID, stableErr := trustedTenant(request)
	if stableErr != nil {
		return TenantStatus{}, stableErr
	}
	if operationID == "" || productRevision < 1 {
		return TenantStatus{}, validationError("operation_id and positive product_revision are required")
	}
	hash, err := canonicalHash(request.Input)
	if err != nil {
		return TenantStatus{}, validationError("input must be valid JSON")
	}
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return TenantStatus{}, internalError()
	}
	defer tx.Rollback(ctx)
	if replay, found, replayErr := loadOperation(ctx, tx, operationID, tenantID, request.CapabilityID, hash); replayErr != nil {
		return TenantStatus{}, replayErr
	} else if found {
		return replay, nil
	}
	status, found, err := queryStatusByIdentity(ctx, tx, tenantID, request.Principal.CompanyID, true)
	if err != nil {
		return TenantStatus{}, internalError()
	}
	if !found {
		return TenantStatus{}, &capability.StableError{Code: capability.CodeResourceNotFound, Message: "Native tenant projection was not found"}
	}
	if productRevision <= status.ProductRevision {
		return TenantStatus{}, preconditionError("product revision is out of order")
	}
	if err := insertOperation(ctx, tx, operationID, tenantID, status.TenantBucket, request.CapabilityID, hash, productRevision, "running"); err != nil {
		return TenantStatus{}, conflictError("operation_id is already in use")
	}
	if stableErr := apply(ctx, tx, status); stableErr != nil {
		return TenantStatus{}, stableErr
	}
	status, _, err = queryStatusByIdentity(ctx, tx, tenantID, request.Principal.CompanyID, false)
	if err != nil {
		return TenantStatus{}, internalError()
	}
	status.OperationStatus = "succeeded"
	if err := completeOperation(ctx, tx, operationID, "succeeded", status); err != nil {
		return TenantStatus{}, internalError()
	}
	if err := insertAudit(ctx, tx, request, status.TenantBucket, operationID, "succeeded", ""); err != nil {
		return TenantStatus{}, internalError()
	}
	if err := tx.Commit(ctx); err != nil {
		return TenantStatus{}, internalError()
	}
	return status, nil
}

const statusColumns = "tenant_id,company_id,shard_id,tenant_bucket,service_tier,global_lifecycle_status,native_status,tenant_revision,product_revision,route_revision,entitlements,coalesce(last_operation_id,'')"

// ResolveActiveCompany maps a Keycloak Organization alias to an existing
// Semattice tenant. It never provisions or mutates a tenant.
func (service *Service) ResolveActiveCompany(ctx context.Context, companyID string) (TenantStatus, bool, error) {
	if companyID == "" {
		return TenantStatus{}, false, nil
	}
	return queryStatusByIdentity(ctx, service.pool, uuid.Nil, companyID, false)
}

func queryStatusByIdentity(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenantID uuid.UUID, companyID string, forUpdate bool) (TenantStatus, bool, error) {
	statement := "select " + statusColumns + " from tenant_registry where tenant_id=$1 or company_id=$2"
	if forUpdate {
		statement += " for update"
	}
	var status TenantStatus
	err := query.QueryRow(ctx, statement, tenantID, companyID).Scan(
		&status.TenantID, &status.CompanyID, &status.ShardID, &status.TenantBucket, &status.ServiceTier,
		&status.GlobalLifecycleStatus, &status.NativeStatus, &status.TenantRevision, &status.ProductRevision,
		&status.RouteRevision, &status.Entitlements, &status.LastOperationID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantStatus{}, false, nil
	}
	return status, err == nil, err
}

func loadOperation(ctx context.Context, tx pgx.Tx, operationID string, tenantID uuid.UUID, capabilityID, requestHash string) (TenantStatus, bool, *capability.StableError) {
	var storedTenant uuid.UUID
	var storedCapability, storedHash, status string
	var result []byte
	err := tx.QueryRow(ctx, "select tenant_id,capability_id,request_hash,status,coalesce(result,'{}'::jsonb) from tenant_operation where operation_id=$1", operationID).
		Scan(&storedTenant, &storedCapability, &storedHash, &status, &result)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantStatus{}, false, nil
	}
	if err != nil {
		return TenantStatus{}, false, internalError()
	}
	if storedTenant != tenantID || storedCapability != capabilityID || storedHash != requestHash {
		return TenantStatus{}, false, &capability.StableError{Code: capability.CodeIdempotencyConflict, Message: "operation_id was already used with different input"}
	}
	var replay TenantStatus
	if err := json.Unmarshal(result, &replay); err != nil {
		return TenantStatus{}, false, internalError()
	}
	replay.OperationStatus = status
	return replay, true, nil
}

func insertOperation(ctx context.Context, tx pgx.Tx, operationID string, tenantID uuid.UUID, bucket int16, capabilityID, requestHash string, revision int64, status string) error {
	_, err := tx.Exec(ctx,
		"insert into tenant_operation(operation_id,tenant_bucket,tenant_id,capability_id,request_hash,status,product_revision) values ($1,$2,$3,$4,$5,$6,$7)",
		operationID, bucket, tenantID, capabilityID, requestHash, status, revision)
	return err
}

func completeOperation(ctx context.Context, tx pgx.Tx, operationID, operationStatus string, result TenantStatus) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "update tenant_operation set status=$2,result=$3,updated_at=clock_timestamp() where operation_id=$1", operationID, operationStatus, encoded)
	return err
}

func insertAudit(ctx context.Context, tx pgx.Tx, request capability.Request, bucket int16, operationID, status, errorCode string) error {
	_, err := tx.Exec(ctx,
		"insert into audit_event(audit_id,request_id,operation_id,tenant_bucket,tenant_id,actor_id,capability_id,status,error_code) values ($1,$2,nullif($3,''),$4,$5,$6,$7,$8,nullif($9,''))",
		uuid.New(), request.RequestID, operationID, bucket, request.TenantID, request.Actor.ID, request.CapabilityID, status, errorCode)
	return err
}

func validateProvisionRequest(request capability.Request, input ProvisionInput) (uuid.UUID, *capability.StableError) {
	tenantID, stableErr := trustedTenant(request)
	if stableErr != nil {
		return uuid.Nil, stableErr
	}
	inputTenantID, err := uuid.Parse(input.TenantID)
	if err != nil || inputTenantID == uuid.Nil || inputTenantID != tenantID {
		return uuid.Nil, validationError("input tenant_id must match the verified identity")
	}
	if request.Principal.CompanyID != input.CompanyID {
		return uuid.Nil, validationError("company_id must match the verified identity")
	}
	if input.OperationID == "" || input.DisplayName == "" || input.ServiceTier == "" || input.GlobalLifecycleStatus != "active" || input.TenantRevision < 1 || input.ProductRevision < 1 {
		return uuid.Nil, validationError("provision input is incomplete or invalid")
	}
	if len(input.Entitlements) > 0 && !validJSONObject(input.Entitlements) {
		return uuid.Nil, validationError("entitlements must be a JSON object")
	}
	return tenantID, nil
}

func trustedTenant(request capability.Request) (uuid.UUID, *capability.StableError) {
	if request.Principal == nil {
		return uuid.Nil, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted identity is required"}
	}
	tenantID, err := uuid.Parse(request.TenantID)
	if err != nil || tenantID == uuid.Nil || request.Principal.TenantID != request.TenantID || request.Principal.Actor.ID != request.Actor.ID {
		return uuid.Nil, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted tenant identity is invalid"}
	}
	return tenantID, nil
}

func canonicalHash(raw json.RawMessage) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func normalizedEntitlements(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}

func validJSONObject(raw json.RawMessage) bool {
	var object map[string]any
	return json.Unmarshal(raw, &object) == nil && object != nil
}

func validationError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeValidationFailed, Message: message}
}

func conflictError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeConflict, Message: message}
}

func preconditionError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeFailedPrecondition, Message: message}
}

func internalError() *capability.StableError {
	return &capability.StableError{Code: capability.CodeInternal, Message: "tenant operation failed"}
}
