package metadata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/governance"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (service *Service) ValidateChangeset(ctx context.Context, request capability.Request, input ChangesetValidateInput) (Changeset, *capability.StableError) {
	candidateID, stableErr := parseMetadataID(input.CandidateMetadataVersionID, "candidate_metadata_version_id")
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	var result Changeset
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		return service.validateChangesetTx(ctx, tx, request, tenant, candidateID, &result)
	})
	if err != nil {
		return Changeset{}, mapChangesetError(err)
	}
	return result, nil
}

func (service *Service) validateChangesetTx(ctx context.Context, tx pgx.Tx, request capability.Request, tenant database.TenantContext, candidateID uuid.UUID, result *Changeset) error {
	var candidateStatus string
	if err := tx.QueryRow(ctx, "select status from metadata_version where metadata_version_id=$1 for update", candidateID).Scan(&candidateStatus); err != nil {
		return err
	}
	if candidateStatus != "draft" {
		return errChangesetCandidateNotDraft
	}
	var baseID uuid.UUID
	if err := tx.QueryRow(ctx,
		"select coalesce(metadata_version_id,'00000000-0000-0000-0000-000000000000'::uuid) from tenant_registry where tenant_id=$1",
		tenant.TenantID,
	).Scan(&baseID); err != nil {
		return err
	}
	if baseID == candidateID {
		return errChangesetCandidateIsCurrent
	}
	var concurrent bool
	if err := tx.QueryRow(ctx,
		"select exists(select 1 from metadata_changeset where candidate_metadata_version_id<>$1 and state in ('validated','approved','backfilling','ready'))",
		candidateID,
	).Scan(&concurrent); err != nil {
		return err
	}
	if concurrent {
		return errChangesetConcurrent
	}
	var serviceTier string
	if err := tx.QueryRow(ctx, "select service_tier from tenant_registry where tenant_id=$1", tenant.TenantID).Scan(&serviceTier); err != nil {
		return err
	}
	policy, err := governance.LoadPolicy(ctx, tx, serviceTier)
	if err != nil {
		return err
	}

	candidateObjects, err := loadObjectDefinitionsTx(ctx, tx, candidateID)
	if err != nil {
		return err
	}
	if len(candidateObjects) == 0 {
		return errEmptyMetadata
	}
	baseObjects := []ObjectDefinition{}
	if baseID != uuid.Nil {
		baseObjects, err = loadObjectDefinitionsTx(ctx, tx, baseID)
		if err != nil {
			return err
		}
	}
	removedObjects, err := validateObjectIdentityEvolution(baseObjects, candidateObjects)
	if err != nil {
		return err
	}
	removedObjectIDs := make(map[string]struct{}, len(removedObjects))
	for _, object := range removedObjects {
		removedObjectIDs[object.ObjectID] = struct{}{}
	}
	candidateRelations, err := loadRelationDefinitionsTx(ctx, tx, candidateID)
	if err != nil {
		return err
	}
	baseRelations := []RelationDefinition{}
	if baseID != uuid.Nil {
		baseRelations, err = loadRelationDefinitionsTx(ctx, tx, baseID)
		if err != nil {
			return err
		}
	}
	if err := validateRelationIdentityEvolution(baseRelations, candidateRelations); err != nil {
		return err
	}

	candidateFields, err := loadFieldDefinitionsTx(ctx, tx, candidateID)
	if err != nil {
		return err
	}
	baseFields := []FieldDefinition{}
	if baseID != uuid.Nil {
		baseFields, err = loadFieldDefinitionsTx(ctx, tx, baseID)
		if err != nil {
			return err
		}
	}
	if err := validateFieldQuotas(candidateFields, policy); err != nil {
		return err
	}
	if err := validateDefinitionRemovalsTx(ctx, tx, removedObjects, removedObjectIDs, baseFields, candidateFields); err != nil {
		return err
	}

	recordCounts, simulations, err := simulateObjectsTx(ctx, tx, candidateObjects, candidateFields)
	if err != nil {
		return err
	}
	changes, requiresBackfill, risk, buildingFields, err := planFieldEvolution(baseFields, candidateFields, recordCounts, removedObjectIDs)
	if err != nil {
		return err
	}
	objectChanges, objectRisk := planObjectEvolution(baseObjects, candidateObjects, removedObjects, recordCounts)
	changes = append(changes, objectChanges...)
	risk = mergeMetadataRisk(risk, objectRisk)
	for _, fieldID := range buildingFields {
		if _, err := tx.Exec(ctx,
			"update field_definition set index_state='building',updated_at=clock_timestamp() where metadata_version_id=$1 and field_id=$2",
			candidateID, fieldID,
		); err != nil {
			return err
		}
	}

	plan := ChangesetPlan{Changes: changes}
	simulation := ChangesetSimulation{Objects: simulations}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	simulationJSON, err := json.Marshal(simulation)
	if err != nil {
		return err
	}
	quotaJSON, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	digestInput, err := json.Marshal(struct {
		Base      string          `json:"base"`
		Candidate string          `json:"candidate"`
		Quota     json.RawMessage `json:"quota"`
		Plan      json.RawMessage `json:"plan"`
	}{baseID.String(), candidateID.String(), quotaJSON, planJSON})
	if err != nil {
		return err
	}
	digestBytes := sha256.Sum256(digestInput)
	digest := hex.EncodeToString(digestBytes[:])

	changesetID, err := existingOrNewChangesetID(ctx, tx, candidateID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"insert into metadata_changeset(tenant_bucket,tenant_id,changeset_id,base_metadata_version_id,candidate_metadata_version_id,state,risk_level,requires_backfill,operation_digest,quota_snapshot,plan,simulation,created_by) "+
			"values ($1,$2,$3,$4,$5,'validated',$6,$7,$8,$9,$10,$11,$12) "+
			"on conflict (tenant_bucket,tenant_id,candidate_metadata_version_id) do update set base_metadata_version_id=excluded.base_metadata_version_id,state='validated',risk_level=excluded.risk_level,requires_backfill=excluded.requires_backfill,operation_digest=excluded.operation_digest,quota_snapshot=excluded.quota_snapshot,plan=excluded.plan,simulation=excluded.simulation,coverage='{}'::jsonb,approval_id=null,approved_by=null,approved_at=null,last_error_code=null,last_error_message=null,updated_at=clock_timestamp()",
		tenant.Bucket, tenant.TenantID, changesetID, nullableUUID(baseID), candidateID, risk, requiresBackfill, digest, quotaJSON, planJSON, simulationJSON, request.Actor.ID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "delete from metadata_changeset_object where changeset_id=$1", changesetID); err != nil {
		return err
	}
	for _, object := range simulations {
		defaults := backfillDefaults(candidateFields, object.ObjectID)
		encodedDefaults, err := json.Marshal(defaults)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			"insert into metadata_changeset_object(tenant_bucket,tenant_id,changeset_id,candidate_metadata_version_id,object_id,state,eligible_records,defaults) values ($1,$2,$3,$4,$5,'pending',$6,$7)",
			tenant.Bucket, tenant.TenantID, changesetID, candidateID, object.ObjectID, object.RecordCount, encodedDefaults,
		); err != nil {
			return err
		}
	}
	if err := insertMetadataAudit(ctx, tx, request, tenant, changesetID.String(), "validated", map[string]any{"digest": digest, "requires_backfill": requiresBackfill}); err != nil {
		return err
	}
	return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), result)
}

func (service *Service) SimulateChangeset(ctx context.Context, request capability.Request, input ChangesetIDInput) (Changeset, *capability.StableError) {
	return service.GetChangeset(ctx, request, input)
}

func (service *Service) GetChangeset(ctx context.Context, request capability.Request, input ChangesetIDInput) (Changeset, *capability.StableError) {
	changesetID, stableErr := parseMetadataID(input.ChangesetID, "changeset_id")
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	var result Changeset
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
	})
	if err != nil {
		return Changeset{}, mapChangesetError(err)
	}
	return result, nil
}

func (service *Service) ApproveChangeset(ctx context.Context, request capability.Request, input ChangesetApproveInput) (Changeset, *capability.StableError) {
	changesetID, stableErr := parseMetadataID(input.ChangesetID, "changeset_id")
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	approvalID := strings.TrimSpace(input.ApprovalID)
	if approvalID == "" {
		return Changeset{}, preconditionError("a manual approval id is required")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	var result Changeset
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		var state string
		var storedApprovalID string
		if err := tx.QueryRow(ctx, "select state,coalesce(approval_id,'') from metadata_changeset where changeset_id=$1 for update", changesetID).Scan(&state, &storedApprovalID); err != nil {
			return err
		}
		if (state == "approved" || state == "backfilling" || state == "ready" || state == "active") && storedApprovalID == approvalID {
			return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
		}
		if state != "validated" {
			return errChangesetNotValidated
		}
		if _, err := tx.Exec(ctx,
			"update metadata_changeset set state='approved',approval_id=$2,approved_by=$3,approved_at=clock_timestamp(),updated_at=clock_timestamp() where changeset_id=$1",
			changesetID, approvalID, request.Actor.ID,
		); err != nil {
			return err
		}
		if err := insertMetadataAudit(ctx, tx, request, tenant, changesetID.String(), "approved", map[string]any{
			"approval_id": approvalID, "approval_mode": "manual",
		}); err != nil {
			return err
		}
		return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
	})
	if err != nil {
		return Changeset{}, mapChangesetError(err)
	}
	return result, nil
}

func (service *Service) PublishChangeset(ctx context.Context, request capability.Request, input ChangesetIDInput) (Changeset, *capability.StableError) {
	changesetID, stableErr := parseMetadataID(input.ChangesetID, "changeset_id")
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	var result Changeset
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
		if state == "rolled_back" {
			return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
		}
		if state == "active" {
			return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
		}
		if state != "approved" && state != "ready" {
			return errChangesetNotApproved
		}
		if requiresBackfill && state != "ready" {
			return errChangesetBackfillRequired
		}
		var currentID *uuid.UUID
		if err := tx.QueryRow(ctx, "select metadata_version_id from tenant_registry where tenant_id=$1 for update", tenant.TenantID).Scan(&currentID); err != nil {
			return err
		}
		if !sameOptionalUUID(currentID, baseID) {
			return errChangesetBaseChanged
		}
		if requiresBackfill {
			_, complete, err := coverageTx(ctx, tx, changesetID, candidateID)
			if err != nil {
				return err
			}
			if !complete {
				return errChangesetBackfillRequired
			}
		}
		if err := registerTombstonesTx(ctx, tx, tenant, candidateID); err != nil {
			return err
		}
		if err := publishVersionTx(ctx, tx, tenant, candidateID, request.Actor.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			"update metadata_changeset set state='active',activated_at=clock_timestamp(),updated_at=clock_timestamp() where changeset_id=$1",
			changesetID,
		); err != nil {
			return err
		}
		if err := insertMetadataAudit(ctx, tx, request, tenant, changesetID.String(), "active", map[string]any{"candidate_metadata_version_id": candidateID.String()}); err != nil {
			return err
		}
		return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
	})
	if err != nil {
		return Changeset{}, mapChangesetError(err)
	}
	return result, nil
}

func registerTombstonesTx(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, candidateID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		"insert into field_tombstone(tenant_bucket,tenant_id,object_id,field_id,api_name,metadata_version_id) "+
			"select tenant_bucket,tenant_id,object_id,field_id,api_name,metadata_version_id from field_definition where metadata_version_id=$1 and lifecycle_state='tombstone' "+
			"on conflict (tenant_bucket,tenant_id,object_id,field_id) do nothing",
		candidateID,
	)
	return err
}

func (service *Service) CancelChangeset(ctx context.Context, request capability.Request, input ChangesetIDInput) (Changeset, *capability.StableError) {
	return service.transitionChangeset(ctx, request, input, []string{"validated", "approved"}, "canceled")
}

func (service *Service) RollbackChangeset(ctx context.Context, request capability.Request, input ChangesetRollbackInput) (Changeset, *capability.StableError) {
	changesetID, stableErr := parseMetadataID(input.ChangesetID, "changeset_id")
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	if input.ApprovalID == "" || request.Principal == nil || !contains(request.Principal.Approvals, input.ApprovalID) {
		return Changeset{}, preconditionError("a verified independent rollback approval is required")
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	var result Changeset
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
		if state != "active" || baseID == nil {
			return errChangesetRollbackUnavailable
		}
		if requiresBackfill {
			return errChangesetDestructiveRollback
		}
		var current uuid.UUID
		if err := tx.QueryRow(ctx, "select metadata_version_id from tenant_registry where tenant_id=$1 for update", tenant.TenantID).Scan(&current); err != nil {
			return err
		}
		if current != candidateID {
			return errChangesetRollbackUnavailable
		}
		if _, err := tx.Exec(ctx, "update tenant_registry set metadata_version_id=$2,updated_at=clock_timestamp() where tenant_id=$1", tenant.TenantID, *baseID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "update metadata_changeset set state='rolled_back',updated_at=clock_timestamp() where changeset_id=$1", changesetID); err != nil {
			return err
		}
		if err := insertMetadataAudit(ctx, tx, request, tenant, changesetID.String(), "rolled_back", map[string]any{"base_metadata_version_id": baseID.String(), "approval_id": input.ApprovalID}); err != nil {
			return err
		}
		return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
	})
	if err != nil {
		return Changeset{}, mapChangesetError(err)
	}
	return result, nil
}

func (service *Service) transitionChangeset(ctx context.Context, request capability.Request, input ChangesetIDInput, allowed []string, target string) (Changeset, *capability.StableError) {
	changesetID, stableErr := parseMetadataID(input.ChangesetID, "changeset_id")
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	tenant, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Changeset{}, stableErr
	}
	var result Changeset
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		var state string
		if err := tx.QueryRow(ctx, "select state from metadata_changeset where changeset_id=$1 for update", changesetID).Scan(&state); err != nil {
			return err
		}
		if state == target {
			return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
		}
		if !contains(allowed, state) {
			return errChangesetTransition
		}
		if _, err := tx.Exec(ctx, "update metadata_changeset set state=$2,updated_at=clock_timestamp() where changeset_id=$1", changesetID, target); err != nil {
			return err
		}
		if err := insertMetadataAudit(ctx, tx, request, tenant, changesetID.String(), target, nil); err != nil {
			return err
		}
		return scanChangeset(tx.QueryRow(ctx, "select "+changesetColumns+" from metadata_changeset where changeset_id=$1", changesetID), &result)
	})
	if err != nil {
		return Changeset{}, mapChangesetError(err)
	}
	return result, nil
}

func publishVersionTx(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, versionID uuid.UUID, actorID string) error {
	var status string
	if err := tx.QueryRow(ctx, "select status from metadata_version where metadata_version_id=$1 for update", versionID).Scan(&status); err != nil {
		return err
	}
	if status != "draft" {
		return errPublishedVersion
	}
	if _, err := tx.Exec(ctx,
		"update field_definition set index_state='active',updated_at=clock_timestamp() where metadata_version_id=$1 and indexed and index_state in ('building','validating')",
		versionID,
	); err != nil {
		return err
	}
	snapshot, digest, err := compileTx(ctx, tx, versionID)
	if err != nil {
		return err
	}
	if len(snapshot.Objects) == 0 {
		return errEmptyMetadata
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"update metadata_version set status='published',snapshot=$2,snapshot_digest=$3,published_by=$4,published_at=clock_timestamp() where metadata_version_id=$1",
		versionID, encoded, digest, actorID,
	); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "update tenant_registry set metadata_version_id=$2,updated_at=clock_timestamp() where tenant_id=$1", tenant.TenantID, versionID)
	return err
}

func loadObjectDefinitionsTx(ctx context.Context, tx pgx.Tx, versionID uuid.UUID) ([]ObjectDefinition, error) {
	rows, err := tx.Query(ctx, "select "+objectColumns+" from object_definition where metadata_version_id=$1 order by object_id,api_name", versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ObjectDefinition{}
	for rows.Next() {
		var object ObjectDefinition
		if err := scanObject(rows, &object); err != nil {
			return nil, err
		}
		result = append(result, object)
	}
	return result, rows.Err()
}

func loadFieldDefinitionsTx(ctx context.Context, tx pgx.Tx, versionID uuid.UUID) ([]FieldDefinition, error) {
	rows, err := tx.Query(ctx, "select "+fieldColumns+" from field_definition where metadata_version_id=$1 order by object_id,api_name,field_id", versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []FieldDefinition{}
	for rows.Next() {
		var field FieldDefinition
		if err := scanField(rows, &field); err != nil {
			return nil, err
		}
		result = append(result, field)
	}
	return result, rows.Err()
}

func loadRelationDefinitionsTx(ctx context.Context, tx pgx.Tx, versionID uuid.UUID) ([]RelationDefinition, error) {
	rows, err := tx.Query(ctx, "select "+relationColumns+" from relation_definition where metadata_version_id=$1 order by source_object_id,api_name,relation_id", versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []RelationDefinition{}
	for rows.Next() {
		var relation RelationDefinition
		if err := scanRelation(rows, &relation); err != nil {
			return nil, err
		}
		result = append(result, relation)
	}
	return result, rows.Err()
}

func validateObjectIdentityEvolution(base, candidate []ObjectDefinition) ([]ObjectDefinition, error) {
	candidateByID := make(map[string]ObjectDefinition, len(candidate))
	for _, object := range candidate {
		candidateByID[object.ObjectID] = object
	}
	removed := []ObjectDefinition{}
	for _, previous := range base {
		next, exists := candidateByID[previous.ObjectID]
		if !exists {
			removed = append(removed, previous)
			continue
		}
		if previous.APIName != next.APIName {
			return nil, fmt.Errorf("%w: object api_name is immutable for %s", errUnsafeFieldEvolution, previous.ObjectID)
		}
	}
	return removed, nil
}

func validateRelationIdentityEvolution(base, candidate []RelationDefinition) error {
	candidateByID := make(map[string]RelationDefinition, len(candidate))
	for _, relation := range candidate {
		candidateByID[relation.RelationID] = relation
	}
	for _, previous := range base {
		next, exists := candidateByID[previous.RelationID]
		if !exists {
			return fmt.Errorf("%w: relation %s cannot disappear from a candidate version", errUnsafeFieldEvolution, previous.APIName)
		}
		if previous.APIName != next.APIName || previous.SourceObjectID != next.SourceObjectID || previous.TargetObjectID != next.TargetObjectID || previous.RelationType != next.RelationType {
			return fmt.Errorf("%w: relation identity and endpoints are immutable for %s", errUnsafeFieldEvolution, previous.RelationID)
		}
	}
	return nil
}

func validateFieldQuotas(fields []FieldDefinition, policy governance.Policy) error {
	counts := map[string]int{}
	indexed := map[string]int{}
	for _, field := range fields {
		if field.LifecycleState != "tombstone" {
			counts[field.ObjectID]++
		}
		if field.Indexed && field.IndexState != "retiring" {
			indexed[field.ObjectID]++
		}
	}
	for objectID, count := range counts {
		if count > policy.MaxFieldsPerObject {
			return fmt.Errorf("%w: object %s has %d online fields", errFieldQuota, objectID, count)
		}
	}
	for objectID, count := range indexed {
		if count > policy.MaxActiveIndexedFields {
			return fmt.Errorf("%w: object %s has %d indexed fields", errIndexedFieldQuota, objectID, count)
		}
	}
	return nil
}

func simulateObjectsTx(ctx context.Context, tx pgx.Tx, objects []ObjectDefinition, fields []FieldDefinition) (map[string]int64, []ObjectSimulation, error) {
	indexedCounts := map[string]int64{}
	for _, field := range fields {
		if field.Indexed && field.IndexState != "retiring" && field.LifecycleState != "tombstone" {
			indexedCounts[field.ObjectID]++
		}
	}
	counts := make(map[string]int64, len(objects))
	result := make([]ObjectSimulation, 0, len(objects))
	for _, object := range objects {
		var count int64
		var average float64
		var maximum int64
		if err := tx.QueryRow(ctx,
			"select count(*),coalesce(avg(pg_column_size(data)),0)::float8,coalesce(max(pg_column_size(data)),0)::bigint from object_record where object_id=$1",
			object.ObjectID,
		).Scan(&count, &average, &maximum); err != nil {
			return nil, nil, err
		}
		counts[object.ObjectID] = count
		result = append(result, ObjectSimulation{
			ObjectID: object.ObjectID, RecordCount: count, AverageJSONBytes: average, MaximumJSONBytes: maximum,
			ProjectedTypedRows: count * indexedCounts[object.ObjectID],
		})
	}
	return counts, result, nil
}

func validateDefinitionRemovalsTx(ctx context.Context, tx pgx.Tx, removedObjects []ObjectDefinition, removedObjectIDs map[string]struct{}, baseFields, candidateFields []FieldDefinition) error {
	for _, object := range removedObjects {
		var count int64
		if err := tx.QueryRow(ctx, "select count(*) from object_record where object_id=$1", object.ObjectID).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("%w: object %s still has %d records", errDefinitionRemovalBlocked, object.APIName, count)
		}
	}
	candidateByID := make(map[string]struct{}, len(candidateFields))
	for _, field := range candidateFields {
		candidateByID[field.FieldID] = struct{}{}
	}
	for _, field := range baseFields {
		if _, exists := candidateByID[field.FieldID]; exists {
			continue
		}
		if _, objectRemoved := removedObjectIDs[field.ObjectID]; objectRemoved {
			continue
		}
		var count int64
		if err := tx.QueryRow(ctx, "select count(*) from object_record where object_id=$1 and data ? $2", field.ObjectID, field.APIName).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("%w: field %s still has values in %d records", errDefinitionRemovalBlocked, field.APIName, count)
		}
	}
	return nil
}

func planObjectEvolution(base, candidate, removed []ObjectDefinition, recordCounts map[string]int64) ([]ChangesetChange, string) {
	baseByID := make(map[string]ObjectDefinition, len(base))
	for _, object := range base {
		baseByID[object.ObjectID] = object
	}
	changes := make([]ChangesetChange, 0)
	risk := "low"
	for _, object := range candidate {
		previous, exists := baseByID[object.ObjectID]
		if !exists {
			changes = append(changes, ChangesetChange{ObjectID: object.ObjectID, APIName: object.APIName, Kind: "object_added", EligibleRecords: recordCounts[object.ObjectID], CoreSupported: true})
			risk = mergeMetadataRisk(risk, "medium")
			continue
		}
		if previous.Label != object.Label || previous.Description != object.Description || !bytes.Equal(previous.Semantic, object.Semantic) {
			changes = append(changes, ChangesetChange{ObjectID: object.ObjectID, APIName: object.APIName, Kind: "object_updated", EligibleRecords: recordCounts[object.ObjectID], CoreSupported: true})
			risk = mergeMetadataRisk(risk, "medium")
		}
	}
	for _, object := range removed {
		changes = append(changes, ChangesetChange{ObjectID: object.ObjectID, APIName: object.APIName, Kind: "object_removed", EligibleRecords: 0, CoreSupported: true})
		risk = "high"
	}
	return changes, risk
}

func mergeMetadataRisk(left, right string) string {
	rank := map[string]int{"low": 0, "medium": 1, "high": 2}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func planFieldEvolution(base, candidate []FieldDefinition, recordCounts map[string]int64, removedObjectIDs map[string]struct{}) ([]ChangesetChange, bool, string, []string, error) {
	baseByID := make(map[string]FieldDefinition, len(base))
	candidateByID := make(map[string]FieldDefinition, len(candidate))
	for _, field := range base {
		baseByID[field.FieldID] = field
	}
	for _, field := range candidate {
		candidateByID[field.FieldID] = field
	}
	changes := []ChangesetChange{}
	risk := "low"
	for _, previous := range base {
		if _, exists := candidateByID[previous.FieldID]; !exists {
			if _, objectRemoved := removedObjectIDs[previous.ObjectID]; objectRemoved {
				continue
			}
			changes = append(changes, ChangesetChange{ObjectID: previous.ObjectID, FieldID: previous.FieldID, APIName: previous.APIName, Kind: "field_removed", EligibleRecords: 0, CoreSupported: true})
			risk = "high"
		}
	}
	requiresBackfill := false
	building := []string{}
	for _, next := range candidate {
		previous, exists := baseByID[next.FieldID]
		recordCount := recordCounts[next.ObjectID]
		if !exists {
			if next.LifecycleState != "active" {
				return nil, false, "", nil, fmt.Errorf("%w: new field %s must start active", errUnsafeFieldEvolution, next.APIName)
			}
			needsBackfill := recordCount > 0 && (next.Required || next.Indexed || next.DefaultSemantics == "backfill_required" || next.PredecessorFieldID != "")
			if needsBackfill && (next.Required || next.DefaultSemantics == "backfill_required") && len(next.DefaultValue) == 0 && next.PredecessorFieldID == "" {
				return nil, false, "", nil, fmt.Errorf("%w: field %s requires a backfill default or predecessor", errUnsafeFieldEvolution, next.APIName)
			}
			if next.PredecessorFieldID != "" {
				predecessor, predecessorExists := baseByID[next.PredecessorFieldID]
				if !predecessorExists || predecessor.ObjectID != next.ObjectID {
					return nil, false, "", nil, fmt.Errorf("%w: predecessor for %s must be an existing field on the same object", errUnsafeFieldEvolution, next.APIName)
				}
			}
			if needsBackfill && next.Indexed {
				building = append(building, next.FieldID)
			}
			changes = append(changes, ChangesetChange{ObjectID: next.ObjectID, FieldID: next.FieldID, APIName: next.APIName, Kind: "field_added", To: next.LifecycleState, EligibleRecords: recordCount, RequiresBackfill: needsBackfill, CoreSupported: true})
			if needsBackfill {
				requiresBackfill, risk = true, "high"
			} else if risk == "low" {
				risk = "medium"
			}
			continue
		}
		if previous.ObjectID != next.ObjectID || previous.APIName != next.APIName || previous.DataType != next.DataType {
			return nil, false, "", nil, fmt.Errorf("%w: field_id %s cannot change object, api_name, or data_type", errUnsafeFieldEvolution, next.FieldID)
		}
		if !validLifecycleTransition(previous.LifecycleState, next.LifecycleState) {
			return nil, false, "", nil, fmt.Errorf("%w: invalid lifecycle transition %s -> %s for %s", errUnsafeFieldEvolution, previous.LifecycleState, next.LifecycleState, next.APIName)
		}
		if previous.LifecycleState != next.LifecycleState {
			needsBackfill := next.LifecycleState == "purging" || next.LifecycleState == "tombstone"
			changes = append(changes, ChangesetChange{ObjectID: next.ObjectID, FieldID: next.FieldID, APIName: next.APIName, Kind: "lifecycle_changed", From: previous.LifecycleState, To: next.LifecycleState, EligibleRecords: recordCount, RequiresBackfill: needsBackfill, CoreSupported: true})
			if needsBackfill {
				requiresBackfill, risk = true, "high"
			} else if risk == "low" {
				risk = "medium"
			}
		}
		if !previous.Required && next.Required {
			needsBackfill := recordCount > 0
			if needsBackfill && len(next.DefaultValue) == 0 {
				return nil, false, "", nil, fmt.Errorf("%w: required field %s needs a backfill default", errUnsafeFieldEvolution, next.APIName)
			}
			changes = append(changes, ChangesetChange{ObjectID: next.ObjectID, FieldID: next.FieldID, APIName: next.APIName, Kind: "required_enabled", From: "optional", To: "required", EligibleRecords: recordCount, RequiresBackfill: needsBackfill, CoreSupported: true})
			if needsBackfill {
				requiresBackfill, risk = true, "high"
			}
		}
		if !previous.Indexed && next.Indexed {
			needsBackfill := recordCount > 0
			if needsBackfill {
				building = append(building, next.FieldID)
			}
			changes = append(changes, ChangesetChange{ObjectID: next.ObjectID, FieldID: next.FieldID, APIName: next.APIName, Kind: "index_enabled", From: previous.IndexState, To: next.IndexState, EligibleRecords: recordCount, RequiresBackfill: needsBackfill, CoreSupported: true})
			if needsBackfill {
				requiresBackfill, risk = true, "high"
			}
		}
		if previous.Indexed && !next.Indexed {
			changes = append(changes, ChangesetChange{ObjectID: next.ObjectID, FieldID: next.FieldID, APIName: next.APIName, Kind: "index_retired", From: previous.IndexState, To: next.IndexState, EligibleRecords: recordCount, RequiresBackfill: true, CoreSupported: true})
			requiresBackfill, risk = true, "high"
		}
		if !previous.UniqueValue && next.UniqueValue {
			needsBackfill := recordCount > 0
			if needsBackfill {
				building = append(building, next.FieldID)
			}
			changes = append(changes, ChangesetChange{ObjectID: next.ObjectID, FieldID: next.FieldID, APIName: next.APIName, Kind: "unique_enabled", From: "non_unique", To: "unique", EligibleRecords: recordCount, RequiresBackfill: needsBackfill, CoreSupported: true})
			if needsBackfill {
				requiresBackfill, risk = true, "high"
			}
		}
		if previous.UniqueValue && !next.UniqueValue {
			changes = append(changes, ChangesetChange{ObjectID: next.ObjectID, FieldID: next.FieldID, APIName: next.APIName, Kind: "unique_retired", From: "unique", To: "non_unique", EligibleRecords: recordCount, RequiresBackfill: true, CoreSupported: true})
			requiresBackfill, risk = true, "high"
		}
		if !bytes.Equal(previous.Constraints, next.Constraints) {
			changes = append(changes, ChangesetChange{ObjectID: next.ObjectID, FieldID: next.FieldID, APIName: next.APIName, Kind: "constraints_changed", EligibleRecords: recordCount, RequiresBackfill: recordCount > 0, CoreSupported: true})
			if recordCount > 0 {
				requiresBackfill, risk = true, "high"
			} else if risk == "low" {
				risk = "medium"
			}
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].ObjectID != changes[j].ObjectID {
			return changes[i].ObjectID < changes[j].ObjectID
		}
		if changes[i].APIName != changes[j].APIName {
			return changes[i].APIName < changes[j].APIName
		}
		return changes[i].Kind < changes[j].Kind
	})
	return changes, requiresBackfill, risk, building, nil
}

func validLifecycleTransition(from, to string) bool {
	if from == to {
		return true
	}
	allowed := map[string]string{
		"active":                "deprecated_read_write",
		"deprecated_read_write": "deprecated_read_only",
		"deprecated_read_only":  "hidden",
		"hidden":                "purging",
		"purging":               "tombstone",
	}
	return allowed[from] == to
}

func sameOptionalUUID(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func backfillDefaults(fields []FieldDefinition, objectID string) map[string]json.RawMessage {
	result := map[string]json.RawMessage{}
	for _, field := range fields {
		if field.ObjectID == objectID && field.DefaultSemantics == "backfill_required" && len(field.DefaultValue) > 0 {
			result[field.APIName] = append(json.RawMessage(nil), field.DefaultValue...)
		}
	}
	return result
}

func existingOrNewChangesetID(ctx context.Context, tx pgx.Tx, candidateID uuid.UUID) (uuid.UUID, error) {
	var changesetID uuid.UUID
	err := tx.QueryRow(ctx, "select changeset_id from metadata_changeset where candidate_metadata_version_id=$1", candidateID).Scan(&changesetID)
	if err == nil {
		return changesetID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	return uuid.NewV7()
}

func insertMetadataAudit(ctx context.Context, tx pgx.Tx, request capability.Request, tenant database.TenantContext, operationID, status string, eventData any) error {
	encoded := json.RawMessage("{}")
	if eventData != nil {
		var err error
		encoded, err = json.Marshal(eventData)
		if err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx,
		"insert into audit_event(audit_id,request_id,operation_id,tenant_bucket,tenant_id,actor_id,capability_id,status,event_data) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)",
		uuid.New(), request.RequestID, operationID, tenant.Bucket, tenant.TenantID, request.Actor.ID, request.CapabilityID, status, encoded,
	)
	return err
}

const changesetColumns = "changeset_id,coalesce(base_metadata_version_id::text,''),candidate_metadata_version_id,state,risk_level,requires_backfill,operation_digest,quota_snapshot,plan,simulation,coverage,coalesce(approval_id,''),coalesce(approved_by,''),created_by,coalesce(last_error_code,''),coalesce(last_error_message,''),created_at,updated_at,approved_at,activated_at"

func scanChangeset(row rowScanner, result *Changeset) error {
	return row.Scan(
		&result.ChangesetID, &result.BaseMetadataVersionID, &result.CandidateMetadataVersionID, &result.State,
		&result.RiskLevel, &result.RequiresBackfill, &result.OperationDigest, &result.QuotaSnapshot, &result.Plan,
		&result.Simulation, &result.Coverage, &result.ApprovalID, &result.ApprovedBy, &result.CreatedBy,
		&result.LastErrorCode, &result.LastErrorMessage, &result.CreatedAt, &result.UpdatedAt, &result.ApprovedAt, &result.ActivatedAt,
	)
}

var (
	errChangesetCandidateNotDraft    = errors.New("changeset candidate metadata version is not draft")
	errChangesetCandidateIsCurrent   = errors.New("changeset candidate is already the current metadata version")
	errChangesetNotValidated         = errors.New("changeset is not validated")
	errChangesetNotApproved          = errors.New("changeset is not approved or ready")
	errChangesetBackfillRequired     = errors.New("changeset requires backfill before activation")
	errChangesetTransition           = errors.New("changeset transition is not allowed")
	errChangesetRollbackUnavailable  = errors.New("changeset rollback is not available")
	errChangesetDestructiveRollback  = errors.New("changeset with data migration requires forward recovery")
	errChangesetBaseChanged          = errors.New("changeset base metadata version is no longer current")
	errChangesetConcurrent           = errors.New("another non-terminal metadata changeset already exists")
	errChangesetExecutionUnavailable = errors.New("changeset is not available for backfill execution")
	errChangesetRequiresPurge        = errors.New("changeset contains destructive field evolution and requires purge capability")
	errChangesetNotDestructive       = errors.New("changeset does not contain destructive field evolution")
	errUnsafeFieldEvolution          = errors.New("unsafe metadata evolution")
	errDefinitionRemovalBlocked      = errors.New("metadata definition removal is blocked")
)

func mapChangesetError(err error) *capability.StableError {
	if errors.Is(err, pgx.ErrNoRows) {
		return &capability.StableError{Code: capability.CodeResourceNotFound, Message: "changeset or metadata version was not found"}
	}
	for _, expected := range []error{
		errChangesetCandidateNotDraft, errChangesetCandidateIsCurrent, errChangesetNotValidated, errChangesetNotApproved,
		errChangesetBackfillRequired, errChangesetTransition, errChangesetRollbackUnavailable, errChangesetDestructiveRollback,
		errChangesetBaseChanged, errChangesetConcurrent, errChangesetExecutionUnavailable, errChangesetRequiresPurge,
		errChangesetNotDestructive, errUnsafeFieldEvolution, errEmptyMetadata, errFieldQuota, errIndexedFieldQuota,
		errDefinitionRemovalBlocked,
	} {
		if errors.Is(err, expected) {
			return preconditionError(err.Error())
		}
	}
	return mapDatabaseError(err)
}
