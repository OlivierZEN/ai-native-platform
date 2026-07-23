package metadata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/governance"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type executionModel struct {
	VersionID  uuid.UUID
	ObjectID   uuid.UUID
	APIName    string
	Fields     map[string]FieldDefinition
	FieldsByID map[string]FieldDefinition
	Relations  map[string]RelationDefinition
}

type executionRecord struct {
	RecordID uuid.UUID
	Revision int64
	State    string
	Data     map[string]any
}

func (service *Service) BackfillChangeset(ctx context.Context, request capability.Request, input ChangesetBatchInput) (BatchResult, *capability.StableError) {
	return service.executeChangesetBatch(ctx, request, input.ChangesetID, input.BatchSize, false)
}

func (service *Service) PurgeChangeset(ctx context.Context, request capability.Request, input ChangesetPurgeInput) (BatchResult, *capability.StableError) {
	if input.ApprovalID == "" || request.Principal == nil || !contains(request.Principal.Approvals, input.ApprovalID) {
		return BatchResult{}, preconditionError("a verified destructive purge approval is required")
	}
	return service.executeChangesetBatch(ctx, request, input.ChangesetID, input.BatchSize, true)
}

func (service *Service) executeChangesetBatch(ctx context.Context, request capability.Request, rawChangesetID string, batchSize int, destructive bool) (BatchResult, *capability.StableError) {
	changesetID, stableErr := parseMetadataID(rawChangesetID, "changeset_id")
	if stableErr != nil {
		return BatchResult{}, stableErr
	}
	if batchSize == 0 {
		batchSize = 200
	}
	if batchSize < 1 || batchSize > 1000 {
		return BatchResult{}, validationError("batch_size must be between 1 and 1000")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return BatchResult{}, stableErr
	}
	result := BatchResult{ChangesetID: changesetID.String()}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		var state string
		var candidateID uuid.UUID
		var requiresBackfill bool
		var planJSON []byte
		if err := tx.QueryRow(ctx,
			"select state,candidate_metadata_version_id,requires_backfill,plan from metadata_changeset where changeset_id=$1 for update",
			changesetID,
		).Scan(&state, &candidateID, &requiresBackfill, &planJSON); err != nil {
			return err
		}
		if state == "ready" || state == "active" {
			result.State = state
			var coverage []byte
			if err := tx.QueryRow(ctx, "select coverage from metadata_changeset where changeset_id=$1", changesetID).Scan(&coverage); err != nil {
				return err
			}
			result.Coverage = append(json.RawMessage(nil), coverage...)
			return nil
		}
		if !requiresBackfill || (state != "approved" && state != "backfilling") {
			return errChangesetExecutionUnavailable
		}
		var plan ChangesetPlan
		if err := json.Unmarshal(planJSON, &plan); err != nil {
			return err
		}
		requiresPurge := planRequiresPurge(plan)
		if requiresPurge != destructive {
			if requiresPurge {
				return errChangesetRequiresPurge
			}
			return errChangesetNotDestructive
		}
		if _, err := tx.Exec(ctx, "update metadata_changeset set state='backfilling',updated_at=clock_timestamp() where changeset_id=$1", changesetID); err != nil {
			return err
		}

		var objectID uuid.UUID
		err := tx.QueryRow(ctx,
			"select object_id from metadata_changeset_object where changeset_id=$1 and state<>'ready' order by object_id limit 1 for update",
			changesetID,
		).Scan(&objectID)
		if errors.Is(err, pgx.ErrNoRows) {
			coverage, _, err := coverageTx(ctx, tx, changesetID, candidateID)
			result.State, result.Coverage = "backfilling", coverage
			return err
		}
		if err != nil {
			return err
		}
		model, err := loadExecutionModel(ctx, tx, candidateID, objectID)
		if err != nil {
			return err
		}
		policy, err := changesetPolicy(ctx, tx, changesetID)
		if err != nil {
			return err
		}
		records, err := selectExecutionRecords(ctx, tx, candidateID, objectID, batchSize)
		if err != nil {
			return err
		}
		result.ObjectID = objectID.String()
		var lastRecordID *uuid.UUID
		failedSamples := []map[string]any{}
		for _, record := range records {
			result.AttemptedRecords++
			last := record.RecordID
			lastRecordID = &last
			nested, err := tx.Begin(ctx)
			if err != nil {
				return err
			}
			outcome, executionErr := evolveRecord(ctx, nested, tenant, changesetID, model, policy, record, destructive)
			if executionErr != nil || outcome != "succeeded" {
				_ = nested.Rollback(ctx)
				if outcome == "conflict" {
					result.ConflictRecords++
				} else {
					result.FailedRecords++
					if len(failedSamples) < 20 {
						failedSamples = append(failedSamples, map[string]any{"record_id": record.RecordID.String(), "error": boundedError(executionErr)})
					}
				}
				continue
			}
			if err := nested.Commit(ctx); err != nil {
				return err
			}
			result.SucceededRecords++
		}
		var remaining int64
		if err := tx.QueryRow(ctx,
			"select count(*) from object_record where object_id=$1 and metadata_version_id is distinct from $2",
			objectID, candidateID,
		).Scan(&remaining); err != nil {
			return err
		}
		result.RemainingRecords = remaining
		objectState := "backfilling"
		if remaining == 0 && result.FailedRecords == 0 {
			objectState = "ready"
		}
		samplesJSON, err := json.Marshal(failedSamples)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			"update metadata_changeset_object set state=$2::varchar,cursor_record_id=coalesce($3,cursor_record_id),processed_records=processed_records+$4,succeeded_records=succeeded_records+$5,failed_records=failed_records+$6,skipped_conflicts=skipped_conflicts+$7,retry_records=retry_records+$7,failed_samples=case when jsonb_array_length($8::jsonb)>0 then $8::jsonb else failed_samples end,completed_at=case when $2::varchar='ready' then clock_timestamp() else null end,updated_at=clock_timestamp() where changeset_id=$1 and object_id=$9",
			changesetID, objectState, lastRecordID, result.AttemptedRecords, result.SucceededRecords, result.FailedRecords, result.ConflictRecords, samplesJSON, objectID,
		); err != nil {
			return err
		}
		coverage, _, err := coverageTx(ctx, tx, changesetID, candidateID)
		if err != nil {
			return err
		}
		result.State, result.Coverage = "backfilling", coverage
		return insertMetadataAudit(ctx, tx, request, tenant, changesetID.String(), "backfill_batch", map[string]any{
			"object_id": result.ObjectID, "attempted": result.AttemptedRecords, "succeeded": result.SucceededRecords,
			"conflicts": result.ConflictRecords, "failed": result.FailedRecords, "remaining": result.RemainingRecords, "destructive": destructive,
		})
	})
	if err != nil {
		return BatchResult{}, mapChangesetError(err)
	}
	return result, nil
}

func (service *Service) ValidateCoverage(ctx context.Context, request capability.Request, input ChangesetIDInput) (Changeset, *capability.StableError) {
	changesetID, stableErr := parseMetadataID(input.ChangesetID, "changeset_id")
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	var result Changeset
	ready := false
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		var state string
		var baseID *uuid.UUID
		var candidateID uuid.UUID
		var requiresBackfill bool
		if err := tx.QueryRow(ctx,
			"select state,base_metadata_version_id,candidate_metadata_version_id,requires_backfill from metadata_changeset where changeset_id=$1 for update",
			changesetID,
		).Scan(&state, &baseID, &candidateID, &requiresBackfill); err != nil {
			return err
		}
		if state == "ready" || state == "active" {
			ready = true
			return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
		}
		if !requiresBackfill || state != "backfilling" {
			return errChangesetExecutionUnavailable
		}
		var currentID *uuid.UUID
		if err := tx.QueryRow(ctx, "select metadata_version_id from tenant_registry where tenant_id=$1 for update", tenant.TenantID).Scan(&currentID); err != nil {
			return err
		}
		if !sameOptionalUUID(currentID, baseID) {
			return errChangesetBaseChanged
		}
		coverage, complete, err := coverageTx(ctx, tx, changesetID, candidateID)
		if err != nil {
			return err
		}
		ready = complete
		targetState := "backfilling"
		if complete {
			targetState = "ready"
		}
		if _, err := tx.Exec(ctx,
			"update metadata_changeset set state=$2::varchar,coverage=$3,updated_at=clock_timestamp(),last_error_code=case when $2::varchar='ready' then null else 'COVERAGE_INCOMPLETE' end,last_error_message=case when $2::varchar='ready' then null else 'record or derived-state coverage is incomplete' end where changeset_id=$1",
			changesetID, targetState, coverage,
		); err != nil {
			return err
		}
		if err := insertMetadataAudit(ctx, tx, request, tenant, changesetID.String(), "coverage_validated", map[string]any{"ready": complete}); err != nil {
			return err
		}
		return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
	})
	if err != nil {
		return Changeset{}, mapChangesetError(err)
	}
	if !ready {
		return result, preconditionError("changeset coverage is incomplete")
	}
	return result, nil
}

func planRequiresPurge(plan ChangesetPlan) bool {
	for _, change := range plan.Changes {
		if change.Kind == "lifecycle_changed" && (change.To == "purging" || change.To == "tombstone") {
			return true
		}
	}
	return false
}

func loadExecutionModel(ctx context.Context, tx pgx.Tx, candidateID, objectID uuid.UUID) (executionModel, error) {
	model := executionModel{VersionID: candidateID, ObjectID: objectID, Fields: map[string]FieldDefinition{}, FieldsByID: map[string]FieldDefinition{}, Relations: map[string]RelationDefinition{}}
	if err := tx.QueryRow(ctx, "select api_name from object_definition where metadata_version_id=$1 and object_id=$2", candidateID, objectID).Scan(&model.APIName); err != nil {
		return executionModel{}, err
	}
	fields, err := loadFieldDefinitionsTx(ctx, tx, candidateID)
	if err != nil {
		return executionModel{}, err
	}
	for _, field := range fields {
		if field.ObjectID == objectID.String() {
			model.Fields[field.APIName] = field
			model.FieldsByID[field.FieldID] = field
		}
	}
	relations, err := loadRelationDefinitionsTx(ctx, tx, candidateID)
	if err != nil {
		return executionModel{}, err
	}
	for _, relation := range relations {
		if relation.SourceObjectID == objectID.String() {
			model.Relations[relation.APIName] = relation
		}
	}
	return model, nil
}

func selectExecutionRecords(ctx context.Context, tx pgx.Tx, candidateID, objectID uuid.UUID, limit int) ([]executionRecord, error) {
	rows, err := tx.Query(ctx,
		"select record_id,revision,lifecycle_state,data from object_record where object_id=$1 and metadata_version_id is distinct from $2 order by record_id limit $3",
		objectID, candidateID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []executionRecord{}
	for rows.Next() {
		var record executionRecord
		var raw []byte
		if err := rows.Scan(&record.RecordID, &record.Revision, &record.State, &raw); err != nil {
			return nil, err
		}
		data, err := decodeExecutionObject(raw)
		if err != nil {
			return nil, err
		}
		record.Data = data
		result = append(result, record)
	}
	return result, rows.Err()
}

func evolveRecord(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, changesetID uuid.UUID, model executionModel, policy governance.Policy, record executionRecord, destructive bool) (string, error) {
	data, err := evolveData(model, policy, record.Data, destructive)
	if err != nil {
		return "failed", err
	}
	if err := deleteExecutionDerived(ctx, tx, tenant, model.ObjectID, record.RecordID); err != nil {
		return "failed", err
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return "failed", err
	}
	command, err := tx.Exec(ctx,
		"update object_record set metadata_version_id=$3,data=$4,revision=revision+1,updated_by=$5,updated_at=clock_timestamp() where object_id=$1 and record_id=$2 and revision=$6",
		model.ObjectID, record.RecordID, model.VersionID, encoded, "changeset:"+changesetID.String(), record.Revision,
	)
	if err != nil {
		return "failed", err
	}
	if command.RowsAffected() != 1 {
		return "conflict", nil
	}
	if record.State == "active" {
		if err := rebuildExecutionDerived(ctx, tx, tenant, model, record.RecordID, data); err != nil {
			return "failed", err
		}
	}
	return "succeeded", nil
}

func evolveData(model executionModel, policy governance.Policy, source map[string]any, destructive bool) (map[string]any, error) {
	result := make(map[string]any, len(source)+len(model.Fields))
	for key, value := range source {
		result[key] = value
	}
	for name, field := range model.Fields {
		if field.LifecycleState == "purging" || field.LifecycleState == "tombstone" {
			if !destructive {
				return nil, fmt.Errorf("field %s requires purge execution", name)
			}
			delete(result, name)
			continue
		}
		if _, exists := result[name]; exists {
			continue
		}
		if field.PredecessorFieldID != "" {
			if predecessor, exists := model.FieldsByID[field.PredecessorFieldID]; exists {
				if value, present := result[predecessor.APIName]; present {
					converted, err := normalizeExecutionField(field, value)
					if err != nil {
						return nil, err
					}
					result[name] = converted
					continue
				}
			}
		}
		if field.DefaultSemantics == "backfill_required" && len(field.DefaultValue) > 0 {
			value, err := decodeExecutionValue(field.DefaultValue)
			if err != nil {
				return nil, err
			}
			result[name] = value
		}
	}
	for name := range result {
		field, fieldExists := model.Fields[name]
		_, relationExists := model.Relations[name]
		if !fieldExists && !relationExists {
			return nil, fmt.Errorf("property %s is absent from candidate metadata", name)
		}
		if fieldExists {
			normalized, err := normalizeExecutionField(field, result[name])
			if err != nil {
				return nil, err
			}
			result[name] = normalized
		}
	}
	for name, field := range model.Fields {
		if field.Required {
			if value, exists := result[name]; !exists || value == nil {
				return nil, fmt.Errorf("required field is missing: %s", name)
			}
		}
	}
	if err := validateExecutionPolicy(result, model, policy); err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeExecutionField(field FieldDefinition, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	var normalized any
	switch field.DataType {
	case "text":
		switch typed := value.(type) {
		case string:
			normalized = typed
		case json.Number, bool:
			normalized = fmt.Sprint(typed)
		default:
			return nil, fmt.Errorf("%s cannot convert to text", field.APIName)
		}
	case "number":
		var raw string
		switch typed := value.(type) {
		case json.Number:
			raw = typed.String()
		case string:
			raw = typed
		case float64:
			raw = strconv.FormatFloat(typed, 'g', -1, 64)
		default:
			return nil, fmt.Errorf("%s cannot convert to number", field.APIName)
		}
		if _, ok := new(big.Rat).SetString(raw); !ok {
			return nil, fmt.Errorf("%s is not a number", field.APIName)
		}
		normalized = json.Number(raw)
	case "boolean":
		switch typed := value.(type) {
		case bool:
			normalized = typed
		case string:
			parsed, err := strconv.ParseBool(typed)
			if err != nil {
				return nil, fmt.Errorf("%s cannot convert to boolean", field.APIName)
			}
			normalized = parsed
		default:
			return nil, fmt.Errorf("%s cannot convert to boolean", field.APIName)
		}
	case "date":
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be an ISO date", field.APIName)
		}
		parsed, err := time.Parse("2006-01-02", text)
		if err != nil {
			return nil, fmt.Errorf("%s must be an ISO date", field.APIName)
		}
		normalized = parsed.Format("2006-01-02")
	case "datetime":
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be RFC3339", field.APIName)
		}
		parsed, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return nil, fmt.Errorf("%s must be RFC3339", field.APIName)
		}
		normalized = parsed.UTC().Format(time.RFC3339Nano)
	case "uuid":
		text, ok := value.(string)
		parsed, err := uuid.Parse(text)
		if !ok || err != nil || parsed == uuid.Nil {
			return nil, fmt.Errorf("%s must be UUID", field.APIName)
		}
		normalized = parsed.String()
	case "json":
		normalized = value
	default:
		return nil, fmt.Errorf("unsupported field type %s", field.DataType)
	}
	var constraints map[string]any
	if len(field.Constraints) > 0 {
		constraints, _ = decodeExecutionObject(field.Constraints)
	}
	if text, ok := normalized.(string); ok {
		if max, exists := executionIntegerConstraint(constraints, "max_length"); exists && len([]rune(text)) > max {
			return nil, fmt.Errorf("%s exceeds max_length", field.APIName)
		}
		if min, exists := executionIntegerConstraint(constraints, "min_length"); exists && len([]rune(text)) < min {
			return nil, fmt.Errorf("%s is shorter than min_length", field.APIName)
		}
	}
	return normalized, nil
}

func rebuildExecutionDerived(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, model executionModel, recordID uuid.UUID, data map[string]any) error {
	for name, field := range model.Fields {
		if !field.Indexed || field.LifecycleState == "purging" || field.LifecycleState == "tombstone" || (field.IndexState != "active" && field.IndexState != "building" && field.IndexState != "validating") {
			continue
		}
		value, exists := data[name]
		if !exists || value == nil {
			continue
		}
		common := []any{tenant.Bucket, tenant.TenantID, model.VersionID, model.ObjectID, field.FieldID, recordID}
		var err error
		switch field.DataType {
		case "text":
			_, err = tx.Exec(ctx, "insert into record_index_text(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_text) values ($1,$2,$3,$4,$5,$6,$7)", append(common, value.(string))...)
		case "number":
			_, err = tx.Exec(ctx, "insert into record_index_number(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_number) values ($1,$2,$3,$4,$5,$6,$7)", append(common, value.(json.Number).String())...)
		case "boolean":
			_, err = tx.Exec(ctx, "insert into record_index_boolean(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_boolean) values ($1,$2,$3,$4,$5,$6,$7)", append(common, value.(bool))...)
		case "date", "datetime":
			layout := time.RFC3339Nano
			if field.DataType == "date" {
				layout = "2006-01-02"
			}
			parsed, parseErr := time.Parse(layout, value.(string))
			if parseErr != nil {
				return parseErr
			}
			_, err = tx.Exec(ctx, "insert into record_index_datetime(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_kind,value_datetime) values ($1,$2,$3,$4,$5,$6,$7,$8)", append(common, field.DataType, parsed.UTC())...)
		case "uuid":
			_, err = tx.Exec(ctx, "insert into record_index_uuid(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_uuid) values ($1,$2,$3,$4,$5,$6,$7)", append(common, value.(string))...)
		default:
			return fmt.Errorf("indexed field type is unsupported: %s", field.DataType)
		}
		if err != nil {
			return err
		}
		if field.UniqueValue {
			canonical, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if len(canonical) > 1024 {
				return fmt.Errorf("unique field %s exceeds 1024 canonical bytes", field.APIName)
			}
			hash := sha256.Sum256(canonical)
			if _, err := tx.Exec(ctx,
				"insert into record_unique_value(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_hash,value_canonical) values ($1,$2,$3,$4,$5,$6,$7,$8)",
				tenant.Bucket, tenant.TenantID, model.VersionID, model.ObjectID, field.FieldID, recordID, hash[:], string(canonical),
			); err != nil {
				return err
			}
		}
	}
	for name, relation := range model.Relations {
		value, exists := data[name]
		if !exists || value == nil {
			continue
		}
		targets, err := executionTargets(value, relation.RelationType == "many_to_many")
		if err != nil {
			return err
		}
		for _, target := range targets {
			var active bool
			if err := tx.QueryRow(ctx, "select exists(select 1 from object_record where object_id=$1 and record_id=$2 and lifecycle_state='active')", relation.TargetObjectID, target).Scan(&active); err != nil {
				return err
			}
			if !active {
				return fmt.Errorf("relation %s target is not active", relation.APIName)
			}
			relationID, err := uuid.NewV7()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				"insert into record_relation(tenant_bucket,tenant_id,metadata_version_id,relation_id,relation_definition_id,source_object_id,source_record_id,target_object_id,target_record_id) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)",
				tenant.Bucket, tenant.TenantID, model.VersionID, relationID, relation.RelationID, model.ObjectID, recordID, relation.TargetObjectID, target,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func deleteExecutionDerived(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, objectID, recordID uuid.UUID) error {
	for _, table := range []string{"record_index_text", "record_index_number", "record_index_boolean", "record_index_datetime", "record_index_uuid"} {
		if _, err := tx.Exec(ctx, "delete from "+table+" where object_id=$1 and record_id=$2", objectID, recordID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, "delete from record_unique_value where object_id=$1 and record_id=$2", objectID, recordID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "delete from record_relation where source_object_id=$1 and source_record_id=$2", objectID, recordID)
	return err
}

func coverageTx(ctx context.Context, tx pgx.Tx, changesetID, candidateID uuid.UUID) (json.RawMessage, bool, error) {
	rows, err := tx.Query(ctx, "select object_id from metadata_changeset_object where changeset_id=$1 order by object_id", changesetID)
	if err != nil {
		return nil, false, err
	}
	objectIDs := []uuid.UUID{}
	for rows.Next() {
		var objectID uuid.UUID
		if err := rows.Scan(&objectID); err != nil {
			rows.Close()
			return nil, false, err
		}
		objectIDs = append(objectIDs, objectID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, false, err
	}
	rows.Close()
	type objectCoverage struct {
		ObjectID          string `json:"object_id"`
		EligibleRecords   int64  `json:"eligible_records"`
		MigratedRecords   int64  `json:"migrated_records"`
		RemainingRecords  int64  `json:"remaining_records"`
		ValidationFailure int64  `json:"validation_failures"`
	}
	objects := []objectCoverage{}
	var total, migrated, failures int64
	complete := true
	for _, objectID := range objectIDs {
		var eligible, migratedObject int64
		if err := tx.QueryRow(ctx,
			"select count(*),count(*) filter (where metadata_version_id=$2) from object_record where object_id=$1",
			objectID, candidateID,
		).Scan(&eligible, &migratedObject); err != nil {
			return nil, false, err
		}
		model, err := loadExecutionModel(ctx, tx, candidateID, objectID)
		if err != nil {
			return nil, false, err
		}
		validationFailures, err := validateDerivedCoverage(ctx, tx, model)
		if err != nil {
			return nil, false, err
		}
		remaining := eligible - migratedObject
		state := "ready"
		if remaining != 0 || validationFailures != 0 {
			complete = false
			state = "backfilling"
			if remaining == 0 && validationFailures != 0 {
				state = "failed"
			}
		}
		if _, err := tx.Exec(ctx,
			"update metadata_changeset_object set state=$3::varchar,eligible_records=$4,completed_at=case when $3::varchar='ready' then coalesce(completed_at,clock_timestamp()) else null end,updated_at=clock_timestamp() where changeset_id=$1 and object_id=$2",
			changesetID, objectID, state, eligible,
		); err != nil {
			return nil, false, err
		}
		objects = append(objects, objectCoverage{ObjectID: objectID.String(), EligibleRecords: eligible, MigratedRecords: migratedObject, RemainingRecords: remaining, ValidationFailure: validationFailures})
		total += eligible
		migrated += migratedObject
		failures += validationFailures
	}
	ratio := float64(1)
	if total > 0 {
		ratio = float64(migrated) / float64(total)
	}
	coverage, err := json.Marshal(map[string]any{
		"eligible_records": total, "migrated_records": migrated, "remaining_records": total - migrated,
		"validation_failures": failures, "coverage_ratio": ratio, "objects": objects,
	})
	if err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(ctx, "update metadata_changeset set coverage=$2,updated_at=clock_timestamp() where changeset_id=$1", changesetID, coverage); err != nil {
		return nil, false, err
	}
	return coverage, complete && failures == 0, nil
}

func validateDerivedCoverage(ctx context.Context, tx pgx.Tx, model executionModel) (int64, error) {
	var failures int64
	for _, field := range model.Fields {
		if field.LifecycleState == "purging" || field.LifecycleState == "tombstone" {
			var present int64
			if err := tx.QueryRow(ctx, "select count(*) from object_record where object_id=$1 and metadata_version_id=$2 and data ? $3", model.ObjectID, model.VersionID, field.APIName).Scan(&present); err != nil {
				return 0, err
			}
			failures += present
			continue
		}
		if field.Required {
			var missing int64
			if err := tx.QueryRow(ctx, "select count(*) from object_record where object_id=$1 and metadata_version_id=$2 and (not data ? $3 or data->$3='null'::jsonb)", model.ObjectID, model.VersionID, field.APIName).Scan(&missing); err != nil {
				return 0, err
			}
			failures += missing
		}
		if field.Indexed && field.IndexState != "none" && field.IndexState != "retiring" {
			var expected, actual int64
			if err := tx.QueryRow(ctx, "select count(*) from object_record where object_id=$1 and metadata_version_id=$2 and data ? $3 and data->$3<>'null'::jsonb and lifecycle_state='active'", model.ObjectID, model.VersionID, field.APIName).Scan(&expected); err != nil {
				return 0, err
			}
			table := executionIndexTable(field.DataType)
			if table == "" {
				failures += expected
				continue
			}
			if err := tx.QueryRow(ctx, "select count(*) from "+table+" where object_id=$1 and metadata_version_id=$2 and field_id=$3", model.ObjectID, model.VersionID, field.FieldID).Scan(&actual); err != nil {
				return 0, err
			}
			if expected != actual {
				failures += absoluteDifference(expected, actual)
			}
			if field.UniqueValue {
				var uniqueActual int64
				if err := tx.QueryRow(ctx, "select count(*) from record_unique_value where object_id=$1 and metadata_version_id=$2 and field_id=$3", model.ObjectID, model.VersionID, field.FieldID).Scan(&uniqueActual); err != nil {
					return 0, err
				}
				if expected != uniqueActual {
					failures += absoluteDifference(expected, uniqueActual)
				}
			}
		}
	}
	for _, relation := range model.Relations {
		var expected, actual int64
		if relation.RelationType == "many_to_many" {
			if err := tx.QueryRow(ctx,
				"select coalesce(sum(case when jsonb_typeof(data->$3)='array' then jsonb_array_length(data->$3) else 0 end),0)::bigint from object_record where object_id=$1 and metadata_version_id=$2 and data ? $3 and lifecycle_state='active'",
				model.ObjectID, model.VersionID, relation.APIName,
			).Scan(&expected); err != nil {
				return 0, err
			}
		} else if err := tx.QueryRow(ctx,
			"select count(*) from object_record where object_id=$1 and metadata_version_id=$2 and data ? $3 and data->$3<>'null'::jsonb and lifecycle_state='active'",
			model.ObjectID, model.VersionID, relation.APIName,
		).Scan(&expected); err != nil {
			return 0, err
		}
		if err := tx.QueryRow(ctx, "select count(*) from record_relation where source_object_id=$1 and metadata_version_id=$2 and relation_definition_id=$3", model.ObjectID, model.VersionID, relation.RelationID).Scan(&actual); err != nil {
			return 0, err
		}
		if expected != actual {
			failures += absoluteDifference(expected, actual)
		}
	}
	return failures, nil
}

func changesetPolicy(ctx context.Context, tx pgx.Tx, changesetID uuid.UUID) (governance.Policy, error) {
	var raw []byte
	if err := tx.QueryRow(ctx, "select quota_snapshot from metadata_changeset where changeset_id=$1", changesetID).Scan(&raw); err != nil {
		return governance.Policy{}, err
	}
	var policy governance.Policy
	return policy, json.Unmarshal(raw, &policy)
}

func validateExecutionPolicy(data map[string]any, model executionModel, policy governance.Policy) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if len(encoded) > policy.MaxRecordJSONBytes {
		return fmt.Errorf("record exceeds policy JSON limit")
	}
	for name, field := range model.Fields {
		if field.DataType != "json" {
			continue
		}
		value, exists := data[name]
		if !exists || value == nil {
			continue
		}
		raw, err := json.Marshal(value)
		if err != nil || len(raw) > policy.MaxJSONFieldBytes {
			return fmt.Errorf("JSON field %s exceeds policy limit", name)
		}
		if err := validateExecutionJSONShape(value, 1, policy); err != nil {
			return fmt.Errorf("JSON field %s: %w", name, err)
		}
	}
	return nil
}

func validateExecutionJSONShape(value any, depth int, policy governance.Policy) error {
	if depth > policy.MaxJSONDepth {
		return fmt.Errorf("depth exceeds %d", policy.MaxJSONDepth)
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			if err := validateExecutionJSONShape(child, depth+1, policy); err != nil {
				return err
			}
		}
	case []any:
		if len(typed) > policy.MaxJSONArrayElements {
			return fmt.Errorf("array elements exceed %d", policy.MaxJSONArrayElements)
		}
		for _, child := range typed {
			if err := validateExecutionJSONShape(child, depth+1, policy); err != nil {
				return err
			}
		}
	}
	return nil
}

func executionTargets(value any, many bool) ([]uuid.UUID, error) {
	if !many {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("relation value must be UUID")
		}
		parsed, err := uuid.Parse(text)
		if err != nil || parsed == uuid.Nil {
			return nil, fmt.Errorf("relation value must be UUID")
		}
		return []uuid.UUID{parsed}, nil
	}
	array, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("many-to-many relation value must be array")
	}
	result := make([]uuid.UUID, 0, len(array))
	for _, item := range array {
		text, ok := item.(string)
		parsed, err := uuid.Parse(text)
		if !ok || err != nil || parsed == uuid.Nil {
			return nil, fmt.Errorf("relation array contains invalid UUID")
		}
		result = append(result, parsed)
	}
	return result, nil
}

func decodeExecutionObject(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil || result == nil {
		return nil, fmt.Errorf("record data must be JSON object")
	}
	return result, nil
}

func decodeExecutionValue(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("default contains trailing JSON")
	}
	return value, nil
}

func executionIntegerConstraint(constraints map[string]any, name string) (int, bool) {
	value, exists := constraints[name]
	if !exists {
		return 0, false
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.Atoi(number.String())
	return parsed, err == nil && parsed >= 0
}

func executionIndexTable(dataType string) string {
	switch dataType {
	case "text":
		return "record_index_text"
	case "number":
		return "record_index_number"
	case "boolean":
		return "record_index_boolean"
	case "date", "datetime":
		return "record_index_datetime"
	case "uuid":
		return "record_index_uuid"
	default:
		return ""
	}
}

func absoluteDifference(left, right int64) int64 {
	if left > right {
		return left - right
	}
	return right - left
}

func boundedError(err error) string {
	if err == nil {
		return "record revision changed"
	}
	message := err.Error()
	if len(message) > 300 {
		return message[:300]
	}
	return message
}
