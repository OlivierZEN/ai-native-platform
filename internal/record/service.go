package record

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/authorization"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/metering"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool       *pgxpool.Pool
	router     database.RowQuerier
	authorizer *authorization.Evaluator
	meter      *metering.Service
}

func NewService(pool *pgxpool.Pool, router database.RowQuerier, meters ...*metering.Service) *Service {
	if pool == nil || router == nil {
		panic("record service requires runtime pool and tenant router")
	}
	service := &Service{pool: pool, router: router, authorizer: authorization.NewEvaluator()}
	if len(meters) > 0 {
		service.meter = meters[0]
	}
	return service
}

func (service *Service) applyUsageDelta(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, request capability.Request, objectID string, recordID uuid.UUID, records, bytes int64) error {
	if service.meter == nil {
		return nil
	}
	parsedObjectID, err := uuid.Parse(objectID)
	if err != nil || parsedObjectID == uuid.Nil {
		return fmt.Errorf("invalid object ID for usage metering")
	}
	return service.meter.ApplyRecordDelta(ctx, tx, tenant, request, parsedObjectID, recordID, records, bytes)
}

func (service *Service) Create(ctx context.Context, request capability.Request, input CreateInput) (Record, *capability.StableError) {
	if stableErr := validateObjectAPIName(input.ObjectAPIName); stableErr != nil {
		return Record{}, stableErr
	}
	recordID, stableErr := optionalRecordID(input.RecordID)
	if stableErr != nil {
		return Record{}, stableErr
	}
	if recordID == uuid.Nil {
		var err error
		recordID, err = uuid.NewV7()
		if err != nil {
			return Record{}, internalError()
		}
	}
	changes, stableErr := decodeJSONObject(input.Data, "data")
	if stableErr != nil {
		return Record{}, stableErr
	}
	tenant, metadataVersionID, stableErr := service.route(ctx, request)
	if stableErr != nil {
		return Record{}, stableErr
	}
	ownerID := strings.TrimSpace(input.OwnerPrincipalID)
	if ownerID == "" {
		ownerID = request.Actor.ID
	}
	var result Record
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := lockMetadataRoute(ctx, tx, tenant, metadataVersionID); err != nil {
			return err
		}
		model, err := loadObjectModel(ctx, tx, metadataVersionID, input.ObjectAPIName)
		if err != nil {
			return err
		}
		decision, err := service.authorizer.RequireObject(ctx, tx, tenant, request.Actor.ID, model.ObjectID, authorization.ActionCreate)
		if err != nil {
			return mapAuthorizationError(err)
		}
		// An explicit owner is a create-time assignment, not an ownership
		// transfer.  The assignee must already be an active principal with both
		// read and update entitlement on the object; the normal record and field
		// policies still govern every later operation.  This lets a product
		// manager create a task directly for a developer without granting broad
		// tenant update access or bypassing the PDP.
		if ownerID != request.Actor.ID {
			if !decision.Enforced {
				return mapAuthorizationError(authorization.ErrDenied)
			}
			if _, err := service.authorizer.RequireObject(ctx, tx, tenant, ownerID, model.ObjectID, authorization.ActionRead); err != nil {
				return mapAuthorizationError(err)
			}
			if _, err := service.authorizer.RequireObject(ctx, tx, tenant, ownerID, model.ObjectID, authorization.ActionUpdate); err != nil {
				return mapAuthorizationError(err)
			}
		}
		if err := service.requireFields(ctx, tx, tenant, request.Actor.ID, model, authorization.ActionWrite, changes); err != nil {
			return err
		}
		replay, found, err := reserveOperation(ctx, tx, request, tenant)
		if err != nil {
			return err
		}
		if found {
			if err := json.Unmarshal(replay, &result); err != nil {
				return err
			}
			return insertAudit(ctx, tx, request, tenant, "succeeded")
		}
		data, domainErr := normalizeRecordData(model, nil, changes, true)
		if domainErr != nil {
			return domainErr
		}
		if domainErr := enforcePolicyData(ctx, tx, tenant, model, data); domainErr != nil {
			return domainErr
		}
		targetModel, targetData, projected, err := pendingEvolutionProjection(ctx, tx, tenant, metadataVersionID, input.ObjectAPIName, data)
		if err != nil {
			return err
		}
		if !projected {
			targetModel, targetData = model, data
		}
		encoded, err := json.Marshal(targetData)
		if err != nil {
			return err
		}
		dataOrganizationID, err := service.authorizer.PrimaryOrganization(ctx, tx, tenant, ownerID, model.ObjectID)
		if err != nil {
			return mapAuthorizationError(err)
		}
		if !decision.Enforced {
			dataOrganizationID = nil
		}
		if _, err := tx.Exec(ctx,
			"insert into object_record(tenant_bucket,tenant_id,metadata_version_id,object_id,record_id,owner_id,data_organization_id,lifecycle_state,data,revision,created_by,updated_by) "+
				"values ($1,$2,$3,$4,$5,$6,$7,'active',$8,1,$9,$9)",
			tenant.Bucket, tenant.TenantID, targetModel.MetadataVersionID, targetModel.ObjectID, recordID, ownerID, dataOrganizationID, encoded, request.Actor.ID,
		); err != nil {
			return err
		}
		if projected {
			err = rebuildDerivedStateForEvolution(ctx, tx, tenant, targetModel, recordID, targetData)
		} else {
			err = rebuildDerivedState(ctx, tx, tenant, targetModel, recordID, targetData)
		}
		if err != nil {
			return err
		}
		if err := scanRecord(tx.QueryRow(ctx, "select "+recordColumns+" from object_record where object_id=$1 and record_id=$2", model.ObjectID, recordID), model.APIName, &result); err != nil {
			return err
		}
		if err := service.filterAuthorizedRecordData(ctx, tx, tenant, request.Actor.ID, model, &result); err != nil {
			return err
		}
		if err := completeOperation(ctx, tx, request, result); err != nil {
			return err
		}
		if err := service.applyUsageDelta(ctx, tx, tenant, request, targetModel.ObjectID, recordID, 1, int64(len(encoded))); err != nil {
			return err
		}
		return insertAudit(ctx, tx, request, tenant, "succeeded")
	})
	if err != nil {
		return Record{}, mapError(err)
	}
	return result, nil
}

func (service *Service) Get(ctx context.Context, request capability.Request, input GetInput) (Record, *capability.StableError) {
	if stableErr := validateObjectAPIName(input.ObjectAPIName); stableErr != nil {
		return Record{}, stableErr
	}
	recordID, stableErr := parseRecordID(input.RecordID)
	if stableErr != nil {
		return Record{}, stableErr
	}
	tenant, metadataVersionID, stableErr := service.route(ctx, request)
	if stableErr != nil {
		return Record{}, stableErr
	}
	var result Record
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		model, err := loadObjectModel(ctx, tx, metadataVersionID, input.ObjectAPIName)
		if err != nil {
			return err
		}
		if _, err := service.authorizer.RequireObject(ctx, tx, tenant, request.Actor.ID, model.ObjectID, authorization.ActionRead); err != nil {
			return mapAuthorizationError(err)
		}
		scope, err := service.authorizer.RecordScope(ctx, tx, tenant, request.Actor.ID, model.ObjectID, authorization.ActionRead)
		if err != nil {
			return mapAuthorizationError(err)
		}
		statement := "select " + recordColumns + " from object_record where object_id=$1 and record_id=$2"
		arguments := []any{model.ObjectID, recordID}
		if !input.IncludeDeleted {
			statement += " and lifecycle_state='active'"
		}
		statement += recordAccessPredicate(scope, tenant, request.Actor.ID, authorization.ActionRead, "object_record", &arguments)
		if err := scanRecord(tx.QueryRow(ctx, statement, arguments...), model.APIName, &result); err != nil {
			return err
		}
		if err := service.filterAuthorizedRecordData(ctx, tx, tenant, request.Actor.ID, model, &result); err != nil {
			return err
		}
		return insertAudit(ctx, tx, request, tenant, "succeeded")
	})
	if err != nil {
		return Record{}, mapError(err)
	}
	return result, nil
}

func (service *Service) Update(ctx context.Context, request capability.Request, input UpdateInput) (Record, *capability.StableError) {
	if stableErr := validateObjectAPIName(input.ObjectAPIName); stableErr != nil {
		return Record{}, stableErr
	}
	recordID, stableErr := parseRecordID(input.RecordID)
	if stableErr != nil {
		return Record{}, stableErr
	}
	if input.ExpectedRevision < 1 {
		return Record{}, validationError("expected_revision must be positive")
	}
	patch, stableErr := decodeJSONObject(input.Patch, "patch")
	if stableErr != nil {
		return Record{}, stableErr
	}
	tenant, metadataVersionID, stableErr := service.route(ctx, request)
	if stableErr != nil {
		return Record{}, stableErr
	}
	var result Record
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := lockMetadataRoute(ctx, tx, tenant, metadataVersionID); err != nil {
			return err
		}
		model, err := loadObjectModel(ctx, tx, metadataVersionID, input.ObjectAPIName)
		if err != nil {
			return err
		}
		if _, err := service.authorizer.RequireObject(ctx, tx, tenant, request.Actor.ID, model.ObjectID, authorization.ActionUpdate); err != nil {
			return mapAuthorizationError(err)
		}
		if err := service.requireFields(ctx, tx, tenant, request.Actor.ID, model, authorization.ActionWrite, patch); err != nil {
			return err
		}
		scope, err := service.authorizer.RecordScope(ctx, tx, tenant, request.Actor.ID, model.ObjectID, authorization.ActionUpdate)
		if err != nil {
			return mapAuthorizationError(err)
		}
		replay, found, err := reserveOperation(ctx, tx, request, tenant)
		if err != nil {
			return err
		}
		if found {
			if err := json.Unmarshal(replay, &result); err != nil {
				return err
			}
			return insertAudit(ctx, tx, request, tenant, "succeeded")
		}
		arguments := []any{model.ObjectID, recordID}
		statement := "select " + recordColumns + " from object_record where object_id=$1 and record_id=$2 and lifecycle_state='active'"
		statement += recordAccessPredicate(scope, tenant, request.Actor.ID, authorization.ActionUpdate, "object_record", &arguments)
		statement += " for update"
		var current Record
		if err := scanRecord(tx.QueryRow(ctx, statement, arguments...), model.APIName, &current); err != nil {
			return err
		}
		if current.Revision != input.ExpectedRevision {
			return conflictError("record revision does not match expected_revision")
		}
		activeCurrent := activeDataOnly(model, current.Data)
		activeData, domainErr := normalizeRecordData(model, activeCurrent, patch, false)
		if domainErr != nil {
			return domainErr
		}
		if domainErr := enforcePolicyData(ctx, tx, tenant, model, activeData); domainErr != nil {
			return domainErr
		}
		merged := mergeActiveData(model, current.Data, activeData)
		targetModel, targetData, projected, err := pendingEvolutionProjection(ctx, tx, tenant, metadataVersionID, input.ObjectAPIName, merged)
		if err != nil {
			return err
		}
		if !projected {
			targetModel, targetData = model, activeData
		}
		encoded, err := json.Marshal(targetData)
		if err != nil {
			return err
		}
		if current.MetadataVersionID != targetModel.MetadataVersionID {
			if err := deleteDerivedState(ctx, tx, tenant, model.ObjectID, recordID); err != nil {
				return err
			}
		}
		ownerID := current.OwnerID
		command, err := tx.Exec(ctx,
			"update object_record set metadata_version_id=$3,data=$4,owner_id=$5,revision=revision+1,updated_by=$6,updated_at=clock_timestamp() "+
				"where object_id=$1 and record_id=$2 and lifecycle_state='active' and revision=$7",
			model.ObjectID, recordID, targetModel.MetadataVersionID, encoded, ownerID, request.Actor.ID, input.ExpectedRevision,
		)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return conflictError("record revision changed during update")
		}
		if projected {
			err = rebuildDerivedStateForEvolution(ctx, tx, tenant, targetModel, recordID, targetData)
		} else {
			err = rebuildDerivedState(ctx, tx, tenant, targetModel, recordID, targetData)
		}
		if err != nil {
			return err
		}
		if err := scanRecord(tx.QueryRow(ctx, "select "+recordColumns+" from object_record where object_id=$1 and record_id=$2", model.ObjectID, recordID), model.APIName, &result); err != nil {
			return err
		}
		if err := service.filterAuthorizedRecordData(ctx, tx, tenant, request.Actor.ID, model, &result); err != nil {
			return err
		}
		if err := completeOperation(ctx, tx, request, result); err != nil {
			return err
		}
		previous, err := json.Marshal(current.Data)
		if err != nil {
			return err
		}
		if err := service.applyUsageDelta(ctx, tx, tenant, request, targetModel.ObjectID, recordID, 0, int64(len(encoded)-len(previous))); err != nil {
			return err
		}
		return insertAudit(ctx, tx, request, tenant, "succeeded")
	})
	if err != nil {
		return Record{}, mapError(err)
	}
	return result, nil
}

func (service *Service) Delete(ctx context.Context, request capability.Request, input DeleteInput) (Record, *capability.StableError) {
	if stableErr := validateObjectAPIName(input.ObjectAPIName); stableErr != nil {
		return Record{}, stableErr
	}
	recordID, stableErr := parseRecordID(input.RecordID)
	if stableErr != nil {
		return Record{}, stableErr
	}
	if input.ExpectedRevision < 1 {
		return Record{}, validationError("expected_revision must be positive")
	}
	tenant, metadataVersionID, stableErr := service.route(ctx, request)
	if stableErr != nil {
		return Record{}, stableErr
	}
	var result Record
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := lockMetadataRoute(ctx, tx, tenant, metadataVersionID); err != nil {
			return err
		}
		model, err := loadObjectModel(ctx, tx, metadataVersionID, input.ObjectAPIName)
		if err != nil {
			return err
		}
		if _, err := service.authorizer.RequireObject(ctx, tx, tenant, request.Actor.ID, model.ObjectID, authorization.ActionDelete); err != nil {
			return mapAuthorizationError(err)
		}
		scope, err := service.authorizer.RecordScope(ctx, tx, tenant, request.Actor.ID, model.ObjectID, authorization.ActionDelete)
		if err != nil {
			return mapAuthorizationError(err)
		}
		replay, found, err := reserveOperation(ctx, tx, request, tenant)
		if err != nil {
			return err
		}
		if found {
			if err := json.Unmarshal(replay, &result); err != nil {
				return err
			}
			return insertAudit(ctx, tx, request, tenant, "succeeded")
		}
		arguments := []any{model.ObjectID, recordID}
		statement := "select " + recordColumns + " from object_record where object_id=$1 and record_id=$2 and lifecycle_state='active'"
		statement += recordAccessPredicate(scope, tenant, request.Actor.ID, authorization.ActionDelete, "object_record", &arguments)
		statement += " for update"
		var current Record
		if err := scanRecord(tx.QueryRow(ctx, statement, arguments...), model.APIName, &current); err != nil {
			return err
		}
		if current.Revision != input.ExpectedRevision {
			return conflictError("record revision does not match expected_revision")
		}
		var referenced bool
		if err := tx.QueryRow(ctx,
			"select exists(select 1 from record_relation where target_object_id=$1 and target_record_id=$2)", model.ObjectID, recordID,
		).Scan(&referenced); err != nil {
			return err
		}
		if referenced {
			return preconditionError("record is referenced and cannot be deleted in the bounded runtime")
		}
		command, err := tx.Exec(ctx,
			"update object_record set lifecycle_state='deleted',deleted_at=clock_timestamp(),revision=revision+1,updated_by=$3,updated_at=clock_timestamp() "+
				"where object_id=$1 and record_id=$2 and lifecycle_state='active' and revision=$4",
			model.ObjectID, recordID, request.Actor.ID, input.ExpectedRevision,
		)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return conflictError("record revision changed during delete")
		}
		if err := deleteDerivedState(ctx, tx, tenant, model.ObjectID, recordID); err != nil {
			return err
		}
		if err := scanRecord(tx.QueryRow(ctx, "select "+recordColumns+" from object_record where object_id=$1 and record_id=$2", model.ObjectID, recordID), model.APIName, &result); err != nil {
			return err
		}
		if err := service.filterAuthorizedRecordData(ctx, tx, tenant, request.Actor.ID, model, &result); err != nil {
			return err
		}
		if err := completeOperation(ctx, tx, request, result); err != nil {
			return err
		}
		previous, err := json.Marshal(current.Data)
		if err != nil {
			return err
		}
		if err := service.applyUsageDelta(ctx, tx, tenant, request, model.ObjectID, recordID, -1, -int64(len(previous))); err != nil {
			return err
		}
		return insertAudit(ctx, tx, request, tenant, "succeeded")
	})
	if err != nil {
		return Record{}, mapError(err)
	}
	return result, nil
}

func (service *Service) Query(ctx context.Context, request capability.Request, input QueryInput) (QueryResult, *capability.StableError) {
	if stableErr := validateObjectAPIName(input.ObjectAPIName); stableErr != nil {
		return QueryResult{}, stableErr
	}
	if len(input.Filters) > 8 {
		return QueryResult{}, validationError("at most 8 filters are allowed")
	}
	limit := input.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return QueryResult{}, validationError("limit must be between 1 and 100")
	}
	after := uuid.Nil
	var stableErr *capability.StableError
	if input.After != "" {
		after, stableErr = parseRecordID(input.After)
		if stableErr != nil {
			return QueryResult{}, stableErr
		}
	}
	tenant, metadataVersionID, stableErr := service.route(ctx, request)
	if stableErr != nil {
		return QueryResult{}, stableErr
	}
	result := QueryResult{Records: []Record{}, Plan: QueryPlan{Strategy: "bounded_object_scan", Limit: limit}}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		model, err := loadObjectModel(ctx, tx, metadataVersionID, input.ObjectAPIName)
		if err != nil {
			return err
		}
		if _, err := service.authorizer.RequireObject(ctx, tx, tenant, request.Actor.ID, model.ObjectID, authorization.ActionRead); err != nil {
			return mapAuthorizationError(err)
		}
		scope, err := service.authorizer.RecordScope(ctx, tx, tenant, request.Actor.ID, model.ObjectID, authorization.ActionRead)
		if err != nil {
			return mapAuthorizationError(err)
		}
		filters := make([]normalizedFilter, 0, len(input.Filters))
		for _, rawFilter := range input.Filters {
			field, ok := model.Fields[rawFilter.Field]
			if !ok {
				return validationError("unknown query field: " + rawFilter.Field)
			}
			if err := service.requireFields(ctx, tx, tenant, request.Actor.ID, model, authorization.ActionRead, map[string]any{rawFilter.Field: nil}); err != nil {
				return err
			}
			filter, domainErr := normalizeFilter(field, rawFilter)
			if domainErr != nil {
				return domainErr
			}
			filters = append(filters, filter)
			result.Plan.IndexedFields = append(result.Plan.IndexedFields, field.APIName)
		}
		if len(filters) > 0 {
			result.Plan.Strategy = "typed_index"
		}
		fieldIDs := make([]string, 0, len(model.Fields))
		for _, field := range model.Fields {
			fieldIDs = append(fieldIDs, field.FieldID)
		}
		readableFieldIDs, err := service.authorizer.ReadableFieldIDs(ctx, tx, tenant, request.Actor.ID, model.ObjectID, fieldIDs)
		if err != nil {
			return mapAuthorizationError(err)
		}
		statement, arguments := buildQuery(tenant, model, filters, after, limit+1, scope, request.Actor.ID)
		rows, err := tx.Query(ctx, statement, arguments...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var record Record
			if err := scanRecord(rows, model.APIName, &record); err != nil {
				return err
			}
			filterRecordData(model, &record, readableFieldIDs)
			result.Records = append(result.Records, record)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(result.Records) > limit {
			result.NextCursor = result.Records[limit-1].RecordID
			result.Records = result.Records[:limit]
		}
		return insertAudit(ctx, tx, request, tenant, "succeeded")
	})
	if err != nil {
		return QueryResult{}, mapError(err)
	}
	return result, nil
}

func (service *Service) route(ctx context.Context, request capability.Request) (database.TenantContext, uuid.UUID, *capability.StableError) {
	if request.Principal == nil || request.Principal.TenantID != request.TenantID || request.Principal.Actor.ID != request.Actor.ID {
		return database.TenantContext{}, uuid.Nil, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted tenant identity is required"}
	}
	tenantID, err := uuid.Parse(request.TenantID)
	if err != nil || tenantID == uuid.Nil {
		return database.TenantContext{}, uuid.Nil, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted tenant identity is invalid"}
	}
	var bucket int16
	var metadataVersionID uuid.UUID
	err = service.router.QueryRow(ctx,
		"select tenant_bucket,coalesce(metadata_version_id,'00000000-0000-0000-0000-000000000000'::uuid) from tenant_registry "+
			"where tenant_id=$1 and company_id=$2 and native_status='active' and global_lifecycle_status='active'",
		tenantID, request.Principal.CompanyID,
	).Scan(&bucket, &metadataVersionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return database.TenantContext{}, uuid.Nil, notFoundError("active Native tenant route was not found")
	}
	if err != nil {
		return database.TenantContext{}, uuid.Nil, internalError()
	}
	if metadataVersionID == uuid.Nil {
		return database.TenantContext{}, uuid.Nil, preconditionError("tenant has no published metadata version")
	}
	return database.TenantContext{TenantID: tenantID, Bucket: bucket, ActorID: request.Actor.ID}, metadataVersionID, nil
}

func lockMetadataRoute(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, expected uuid.UUID) error {
	var current uuid.UUID
	if err := tx.QueryRow(ctx,
		"select coalesce(metadata_version_id,'00000000-0000-0000-0000-000000000000'::uuid) from tenant_registry where tenant_id=$1 for share",
		tenant.TenantID,
	).Scan(&current); err != nil {
		return err
	}
	if current != expected {
		return conflictError("metadata route changed; retry the record operation")
	}
	return nil
}

func loadObjectModel(ctx context.Context, tx pgx.Tx, metadataVersionID uuid.UUID, apiName string) (objectModel, error) {
	return loadObjectModelVersion(ctx, tx, metadataVersionID, apiName, true)
}

func loadEvolutionModel(ctx context.Context, tx pgx.Tx, metadataVersionID uuid.UUID, apiName string) (objectModel, error) {
	return loadObjectModelVersion(ctx, tx, metadataVersionID, apiName, false)
}

func loadObjectModelVersion(ctx context.Context, tx pgx.Tx, metadataVersionID uuid.UUID, apiName string, requirePublished bool) (objectModel, error) {
	var model objectModel
	model.MetadataVersionID = metadataVersionID.String()
	statement := "select od.object_id,od.api_name from object_definition od join metadata_version mv using (tenant_bucket,tenant_id,metadata_version_id) where od.metadata_version_id=$1 and od.api_name=$2"
	if requirePublished {
		statement += " and mv.status='published'"
	} else {
		statement += " and mv.status in ('draft','published')"
	}
	err := tx.QueryRow(ctx, statement, metadataVersionID, apiName).Scan(&model.ObjectID, &model.APIName)
	if err != nil {
		return objectModel{}, err
	}
	model.Fields = make(map[string]fieldSpec)
	rows, err := tx.Query(ctx,
		"select field_id,api_name,data_type,required,indexed,unique_value,lifecycle_state,index_state,default_semantics,coalesce(predecessor_field_id::text,''),default_value,constraints from field_definition where metadata_version_id=$1 and object_id=$2 order by api_name",
		metadataVersionID, model.ObjectID,
	)
	if err != nil {
		return objectModel{}, err
	}
	for rows.Next() {
		var field fieldSpec
		var defaultValue, constraints []byte
		if err := rows.Scan(
			&field.FieldID, &field.APIName, &field.DataType, &field.Required, &field.Indexed, &field.UniqueValue,
			&field.LifecycleState, &field.IndexState, &field.DefaultSemantics, &field.PredecessorFieldID,
			&defaultValue, &constraints,
		); err != nil {
			rows.Close()
			return objectModel{}, err
		}
		field.DefaultValue = append(json.RawMessage(nil), defaultValue...)
		if len(constraints) > 0 {
			decoded, domainErr := decodeJSONObject(constraints, "constraints")
			if domainErr != nil {
				rows.Close()
				return objectModel{}, domainErr
			}
			field.Constraints = decoded
		}
		model.Fields[field.APIName] = field
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return objectModel{}, err
	}
	rows.Close()
	model.Relations = make(map[string]relationSpec)
	rows, err = tx.Query(ctx,
		"select relation_id,api_name,target_object_id,relation_type,delete_behavior from relation_definition where metadata_version_id=$1 and source_object_id=$2 order by api_name",
		metadataVersionID, model.ObjectID,
	)
	if err != nil {
		return objectModel{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var relation relationSpec
		if err := rows.Scan(&relation.RelationID, &relation.APIName, &relation.TargetObjectID, &relation.RelationType, &relation.DeleteBehavior); err != nil {
			return objectModel{}, err
		}
		if _, collision := model.Fields[relation.APIName]; collision {
			return objectModel{}, preconditionError("field and relation API names collide: " + relation.APIName)
		}
		model.Relations[relation.APIName] = relation
	}
	return model, rows.Err()
}

func buildQuery(tenant database.TenantContext, model objectModel, filters []normalizedFilter, after uuid.UUID, limit int, scope authorization.RecordScope, principalID string) (string, []any) {
	arguments := []any{tenant.Bucket, tenant.TenantID, model.ObjectID}
	statement := ""
	afterParameter := 0
	groups := groupQueryFilters(filters)
	if len(groups) == 0 {
		statement = "select " + qualifiedRecordColumns + " from object_record r where r.tenant_bucket=$1 and r.tenant_id=$2 and r.object_id=$3 and r.lifecycle_state='active'"
	} else {
		driver := groups[0]
		arguments = append(arguments, driver.FieldID)
		statement = fmt.Sprintf(
			"with candidates as materialized (select i0.metadata_version_id,i0.record_id from %s i0 where i0.tenant_bucket=$1 and i0.tenant_id=$2 and i0.object_id=$3 and i0.field_id=$%d",
			driver.Table, len(arguments),
		)
		statement = appendFilterPredicates(statement, "i0", driver.Filters, &arguments)
		if after != uuid.Nil {
			arguments = append(arguments, after)
			afterParameter = len(arguments)
			statement += fmt.Sprintf(" and i0.record_id>$%d", afterParameter)
		}
		statement += ") select " + qualifiedRecordColumns + " from candidates c join object_record r on r.tenant_bucket=$1 and r.tenant_id=$2 and r.object_id=$3 and r.metadata_version_id=c.metadata_version_id and r.record_id=c.record_id where r.lifecycle_state='active'"
		for index, group := range groups[1:] {
			arguments = append(arguments, group.FieldID)
			alias := fmt.Sprintf("i%d", index+1)
			statement += fmt.Sprintf(
				" and exists(select 1 from %s %s where %s.tenant_bucket=r.tenant_bucket and %s.tenant_id=r.tenant_id and %s.metadata_version_id=r.metadata_version_id and %s.object_id=r.object_id and %s.record_id=r.record_id and %s.field_id=$%d",
				group.Table, alias, alias, alias, alias, alias, alias, alias, len(arguments),
			)
			statement = appendFilterPredicates(statement, alias, group.Filters, &arguments)
			statement += ")"
		}
	}
	if after != uuid.Nil {
		if len(groups) == 0 {
			arguments = append(arguments, after)
			afterParameter = len(arguments)
		}
		statement += fmt.Sprintf(" and r.record_id>$%d", afterParameter)
	}
	statement += recordAccessPredicate(scope, tenant, principalID, authorization.ActionRead, "r", &arguments)
	arguments = append(arguments, limit)
	statement += fmt.Sprintf(" order by r.record_id limit $%d", len(arguments))
	return statement, arguments
}

// recordAccessPredicate keeps authorization in the same bounded query as the
// record read. Team and share checks are correlated lookups, while hierarchy
// scopes use organization_closure. It deliberately never joins a generated
// record-to-user ACL table.
func recordAccessPredicate(scope authorization.RecordScope, tenant database.TenantContext, principalID, action, alias string, arguments *[]any) string {
	if scope.AllowAll {
		return ""
	}
	conditions := make([]string, 0, 5)
	if scope.AllowOwner {
		*arguments = append(*arguments, principalID)
		conditions = append(conditions, fmt.Sprintf("%s.owner_id=$%d", alias, len(*arguments)))
	}
	if len(scope.Organizations) > 0 {
		*arguments = append(*arguments, scope.Organizations)
		conditions = append(conditions, fmt.Sprintf("%s.data_organization_id = any($%d::uuid[])", alias, len(*arguments)))
	}
	if len(scope.DescendantRoots) > 0 {
		*arguments = append(*arguments, scope.DescendantRoots)
		conditions = append(conditions, fmt.Sprintf(`exists(
			select 1 from organization_closure closure
			where closure.tenant_bucket=%s.tenant_bucket and closure.tenant_id=%s.tenant_id
			  and closure.descendant_organization_id=%s.data_organization_id
			  and closure.ancestor_organization_id = any($%d::uuid[])
		)`, alias, alias, alias, len(*arguments)))
	}
	for _, condition := range scope.Conditions {
		*arguments = append(*arguments, string(condition))
		conditions = append(conditions, fmt.Sprintf("%s.data @> $%d::jsonb", alias, len(*arguments)))
	}
	if scope.IncludeTeamShare {
		*arguments = append(*arguments, principalID)
		principalParameter := len(*arguments)
		*arguments = append(*arguments, accessLevelsFor(action))
		accessParameter := len(*arguments)
		conditions = append(conditions, fmt.Sprintf(`exists(
			select 1 from record_team_member team
			where team.tenant_bucket=%s.tenant_bucket and team.tenant_id=%s.tenant_id
			  and team.object_id=%s.object_id and team.record_id=%s.record_id
			  and team.principal_id=$%d and team.lifecycle_state='active'
			  and team.access_level = any($%d::varchar[])
		)`, alias, alias, alias, alias, principalParameter, accessParameter))
		conditions = append(conditions, fmt.Sprintf(`exists(
			select 1 from share_grant shared
			where shared.tenant_bucket=%s.tenant_bucket and shared.tenant_id=%s.tenant_id
			  and shared.object_id=%s.object_id and shared.record_id=%s.record_id
			  and shared.lifecycle_state='active' and (shared.expires_at is null or shared.expires_at > clock_timestamp())
			  and shared.access_level = any($%d::varchar[])
			  and (
				(shared.grantee_type='principal' and shared.grantee_ref=$%d)
				or (shared.grantee_type='group' and exists(
					select 1 from group_membership membership
					join access_group access_group
					  on access_group.tenant_bucket=membership.tenant_bucket and access_group.tenant_id=membership.tenant_id and access_group.group_id=membership.group_id
					where membership.tenant_bucket=shared.tenant_bucket and membership.tenant_id=shared.tenant_id
					  and membership.group_id::text=shared.grantee_ref and membership.principal_id=$%d and membership.membership_state='active'
					  and access_group.lifecycle_state='active'
				))
			  )
		)`, alias, alias, alias, alias, accessParameter, principalParameter, principalParameter))
		conditions = append(conditions, fmt.Sprintf(`exists(
			select 1 from share_projection projection
			join sharing_rule_def rule
			  on rule.tenant_bucket=projection.tenant_bucket and rule.tenant_id=projection.tenant_id and rule.rule_id=projection.rule_id
			join group_membership membership
			  on membership.tenant_bucket=projection.tenant_bucket and membership.tenant_id=projection.tenant_id and membership.group_id=projection.group_id
			join access_group access_group
			  on access_group.tenant_bucket=projection.tenant_bucket and access_group.tenant_id=projection.tenant_id and access_group.group_id=projection.group_id
			where projection.tenant_bucket=%s.tenant_bucket and projection.tenant_id=%s.tenant_id
			  and projection.object_id=%s.object_id and projection.record_id=%s.record_id
			  and rule.lifecycle_state='active' and rule.projection_state='ready'
			  and access_group.lifecycle_state='active'
			  and projection.access_level = any($%d::varchar[])
			  and membership.principal_id=$%d and membership.membership_state='active'
		)`, alias, alias, alias, alias, accessParameter, principalParameter))
	}
	if len(conditions) == 0 {
		return " and false"
	}
	return " and (" + strings.Join(conditions, " or ") + ")"
}

func accessLevelsFor(action string) []string {
	switch action {
	case authorization.ActionDelete:
		return []string{"delete"}
	case authorization.ActionUpdate:
		return []string{"update", "delete"}
	default:
		return []string{"read", "update", "delete"}
	}
}

type queryFilterGroup struct {
	Table   string
	FieldID string
	Filters []normalizedFilter
}

func groupQueryFilters(filters []normalizedFilter) []queryFilterGroup {
	groups := make([]queryFilterGroup, 0, len(filters))
	positions := make(map[string]int, len(filters))
	for _, filter := range filters {
		key := filter.Table + "\x00" + filter.Field.FieldID
		position, exists := positions[key]
		if !exists {
			position = len(groups)
			positions[key] = position
			groups = append(groups, queryFilterGroup{Table: filter.Table, FieldID: filter.Field.FieldID})
		}
		groups[position].Filters = append(groups[position].Filters, filter)
	}
	return groups
}

func appendFilterPredicates(statement, alias string, filters []normalizedFilter, arguments *[]any) string {
	for _, filter := range filters {
		*arguments = append(*arguments, filter.Value)
		statement += fmt.Sprintf(" and %s.%s %s $%d", alias, filter.ValueColumn, filter.OperatorSQL, len(*arguments))
		if filter.OperatorSQL == "like" {
			statement += " escape '\\'"
		}
	}
	return statement
}

const recordColumns = "metadata_version_id,object_id,record_id,coalesce(owner_id,''),coalesce(data_organization_id::text,''),lifecycle_state,data,revision,created_by,updated_by,created_at,updated_at,deleted_at"
const qualifiedRecordColumns = "r.metadata_version_id,r.object_id,r.record_id,coalesce(r.owner_id,''),coalesce(r.data_organization_id::text,''),r.lifecycle_state,r.data,r.revision,r.created_by,r.updated_by,r.created_at,r.updated_at,r.deleted_at"

type rowScanner interface {
	Scan(...any) error
}

func scanRecord(row rowScanner, objectAPIName string, target *Record) error {
	var encoded []byte
	if err := row.Scan(
		&target.MetadataVersionID, &target.ObjectID, &target.RecordID, &target.OwnerID, &target.DataOrganizationID, &target.LifecycleState,
		&encoded, &target.Revision, &target.CreatedBy, &target.UpdatedBy, &target.CreatedAt, &target.UpdatedAt, &target.DeletedAt,
	); err != nil {
		return err
	}
	data, stableErr := decodeJSONObject(encoded, "stored data")
	if stableErr != nil {
		return fmt.Errorf("decode stored record: %s", stableErr.Message)
	}
	target.ObjectAPIName = objectAPIName
	target.Data = data
	return nil
}

func (service *Service) requireFields(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID string, model objectModel, action string, data map[string]any) error {
	fieldIDs := make([]string, 0, len(data))
	for apiName := range data {
		if field, exists := model.Fields[apiName]; exists {
			fieldIDs = append(fieldIDs, field.FieldID)
		}
	}
	if err := service.authorizer.RequireFields(ctx, tx, tenant, principalID, model.ObjectID, action, fieldIDs); err != nil {
		return mapAuthorizationError(err)
	}
	return nil
}

func (service *Service) filterAuthorizedRecordData(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, principalID string, model objectModel, target *Record) error {
	fieldIDs := make([]string, 0, len(model.Fields))
	for _, field := range model.Fields {
		fieldIDs = append(fieldIDs, field.FieldID)
	}
	readable, err := service.authorizer.ReadableFieldIDs(ctx, tx, tenant, principalID, model.ObjectID, fieldIDs)
	if err != nil {
		return mapAuthorizationError(err)
	}
	filterRecordData(model, target, readable)
	return nil
}

func filterRecordData(model objectModel, target *Record, readableFieldIDs map[string]bool) {
	target.MetadataVersionID = model.MetadataVersionID
	for name := range target.Data {
		if field, exists := model.Fields[name]; exists {
			if !fieldIsVisible(field) || (readableFieldIDs != nil && !readableFieldIDs[field.FieldID]) {
				delete(target.Data, name)
			}
			continue
		}
		if _, exists := model.Relations[name]; !exists {
			delete(target.Data, name)
		}
	}
}

func optionalRecordID(raw string) (uuid.UUID, *capability.StableError) {
	if raw == "" {
		return uuid.Nil, nil
	}
	return parseRecordID(raw)
}

func parseRecordID(raw string) (uuid.UUID, *capability.StableError) {
	value, err := uuid.Parse(raw)
	if err != nil || value == uuid.Nil || value.Version() != 7 {
		return uuid.Nil, validationError("record_id must be a UUIDv7")
	}
	return value, nil
}

var objectAPINamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,95}$`)

func validateObjectAPIName(value string) *capability.StableError {
	if !objectAPINamePattern.MatchString(value) {
		return validationError("object_api_name must match ^[a-z][a-z0-9_]{0,95}$")
	}
	return nil
}

func insertAudit(ctx context.Context, tx pgx.Tx, request capability.Request, tenant database.TenantContext, status string) error {
	_, err := tx.Exec(ctx,
		"insert into audit_event(audit_id,request_id,tenant_bucket,tenant_id,actor_id,capability_id,status) values ($1,$2,$3,$4,$5,$6,$7)",
		uuid.New(), request.RequestID, tenant.Bucket, tenant.TenantID, request.Actor.ID, request.CapabilityID, status,
	)
	return err
}

func mapError(err error) *capability.StableError {
	var stableErr *capability.StableError
	if errors.As(err, &stableErr) {
		return stableErr
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return notFoundError("record or published object was not found")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return conflictError("record identity already exists")
		case "23503", "23514", "23502", "22P02", "22003":
			return validationError("record violates metadata or relation constraints")
		}
	}
	return internalError()
}

func parseDateTime(kind string, value any) (time.Time, error) {
	text, ok := value.(string)
	if !ok {
		return time.Time{}, fmt.Errorf("not a string")
	}
	if kind == "date" {
		return time.Parse("2006-01-02", text)
	}
	return time.Parse(time.RFC3339Nano, text)
}
