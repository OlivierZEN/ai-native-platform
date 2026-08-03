package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/governance"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var apiNamePattern = regexp.MustCompile("^[a-z][a-z0-9_]{0,95}$")

type Service struct {
	pool   *pgxpool.Pool
	router database.RowQuerier
}

func NewService(pool *pgxpool.Pool, router database.RowQuerier) *Service {
	if pool == nil || router == nil {
		panic("metadata service requires data pool and tenant router")
	}
	return &Service{pool: pool, router: router}
}

func (service *Service) CreateVersion(ctx context.Context, request capability.Request) (Version, *capability.StableError) {
	tenantContext, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Version{}, stableErr
	}
	versionID, err := uuid.NewV7()
	if err != nil {
		return Version{}, internalError()
	}
	var result Version
	err = database.WithTenant(ctx, service.pool, tenantContext, func(tx pgx.Tx) error {
		var sequence int64
		if err := tx.QueryRow(ctx, "select coalesce(max(sequence),0)+1 from metadata_version").Scan(&sequence); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			"insert into metadata_version(tenant_bucket,tenant_id,metadata_version_id,sequence,status,created_by) values ($1,$2,$3,$4,'draft',$5)",
			tenantContext.Bucket, tenantContext.TenantID, versionID, sequence, request.Actor.ID); err != nil {
			return err
		}
		return scanVersion(tx.QueryRow(ctx, "select "+versionColumns+" from metadata_version where metadata_version_id=$1", versionID), &result)
	})
	if err != nil {
		return Version{}, mapDatabaseError(err)
	}
	return result, nil
}

func (service *Service) UpsertObject(ctx context.Context, request capability.Request, input ObjectUpsertInput) (ObjectDefinition, *capability.StableError) {
	versionID, stableErr := parseMetadataID(input.MetadataVersionID, "metadata_version_id")
	if stableErr != nil {
		return ObjectDefinition{}, stableErr
	}
	objectID, stableErr := optionalMetadataID(input.ObjectID, "object_id")
	if stableErr != nil {
		return ObjectDefinition{}, stableErr
	}
	if objectID == uuid.Nil {
		var err error
		objectID, err = uuid.NewV7()
		if err != nil {
			return ObjectDefinition{}, internalError()
		}
	}
	if !apiNamePattern.MatchString(input.APIName) || input.Label == "" {
		return ObjectDefinition{}, validationError("object api_name or label is invalid")
	}
	semantic, stableErr := canonicalObject(input.Semantic, "semantic")
	if stableErr != nil {
		return ObjectDefinition{}, stableErr
	}
	tenantContext, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return ObjectDefinition{}, stableErr
	}
	var result ObjectDefinition
	err := database.WithTenant(ctx, service.pool, tenantContext, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			"insert into object_definition(tenant_bucket,tenant_id,metadata_version_id,object_id,api_name,label,description,semantic) values ($1,$2,$3,$4,$5,$6,$7,$8) "+
				"on conflict (tenant_bucket,tenant_id,metadata_version_id,object_id) do update set api_name=excluded.api_name,label=excluded.label,description=excluded.description,semantic=excluded.semantic,updated_at=clock_timestamp()",
			tenantContext.Bucket, tenantContext.TenantID, versionID, objectID, input.APIName, input.Label, input.Description, semantic)
		if err != nil {
			return err
		}
		return scanObject(tx.QueryRow(ctx, "select "+objectColumns+" from object_definition where metadata_version_id=$1 and object_id=$2", versionID, objectID), &result)
	})
	if err != nil {
		return ObjectDefinition{}, mapDatabaseError(err)
	}
	return result, nil
}

func (service *Service) UpsertField(ctx context.Context, request capability.Request, input FieldUpsertInput) (FieldDefinition, *capability.StableError) {
	versionID, stableErr := parseMetadataID(input.MetadataVersionID, "metadata_version_id")
	if stableErr != nil {
		return FieldDefinition{}, stableErr
	}
	objectID, stableErr := parseMetadataID(input.ObjectID, "object_id")
	if stableErr != nil {
		return FieldDefinition{}, stableErr
	}
	fieldID, stableErr := optionalMetadataID(input.FieldID, "field_id")
	if stableErr != nil {
		return FieldDefinition{}, stableErr
	}
	if fieldID == uuid.Nil {
		var err error
		fieldID, err = uuid.NewV7()
		if err != nil {
			return FieldDefinition{}, internalError()
		}
	}
	if !apiNamePattern.MatchString(input.APIName) || input.Label == "" || !validDataType(input.DataType) {
		return FieldDefinition{}, validationError("field api_name, label or data_type is invalid")
	}
	lifecycleState := input.LifecycleState
	if lifecycleState == "" {
		lifecycleState = "active"
	}
	if !validFieldLifecycle(lifecycleState) {
		return FieldDefinition{}, validationError("field lifecycle_state is invalid")
	}
	indexState := input.IndexState
	if input.Indexed && indexState == "" {
		indexState = "active"
	}
	if !input.Indexed {
		indexState = "none"
	}
	if !validIndexState(indexState) || (input.Indexed && indexState == "none") {
		return FieldDefinition{}, validationError("field index_state is incompatible with indexed")
	}
	defaultSemantics := input.DefaultSemantics
	if defaultSemantics == "" {
		defaultSemantics = "on_create"
	}
	if !contains([]string{"on_create", "backfill_required"}, defaultSemantics) {
		return FieldDefinition{}, validationError("field default_semantics is invalid")
	}
	predecessorID, stableErr := optionalMetadataID(input.PredecessorFieldID, "predecessor_field_id")
	if stableErr != nil {
		return FieldDefinition{}, stableErr
	}
	if lifecycleState == "tombstone" && (input.Required || input.Indexed) {
		return FieldDefinition{}, validationError("tombstone fields cannot be required or indexed")
	}
	if input.UniqueValue && (!input.Indexed || input.DataType == "json" || lifecycleState == "tombstone") {
		return FieldDefinition{}, validationError("unique_value requires a non-JSON indexed online field")
	}
	semantic, stableErr := canonicalObject(input.Semantic, "semantic")
	if stableErr != nil {
		return FieldDefinition{}, stableErr
	}
	constraints, stableErr := canonicalObject(input.Constraints, "constraints")
	if stableErr != nil {
		return FieldDefinition{}, stableErr
	}
	defaultValue, stableErr := canonicalValue(input.DefaultValue, "default_value")
	if stableErr != nil {
		return FieldDefinition{}, stableErr
	}
	tenantContext, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return FieldDefinition{}, stableErr
	}
	var result FieldDefinition
	err := database.WithTenant(ctx, service.pool, tenantContext, func(tx pgx.Tx) error {
		var serviceTier string
		if err := tx.QueryRow(ctx, "select service_tier from tenant_registry where tenant_id=$1", tenantContext.TenantID).Scan(&serviceTier); err != nil {
			return err
		}
		policy, err := governance.LoadPolicy(ctx, tx, serviceTier)
		if err != nil {
			return err
		}
		var existing bool
		if err := tx.QueryRow(ctx,
			"select exists(select 1 from field_definition where metadata_version_id=$1 and field_id=$2)",
			versionID, fieldID,
		).Scan(&existing); err != nil {
			return err
		}
		if !existing && lifecycleState != "tombstone" {
			var count int
			if err := tx.QueryRow(ctx,
				"select count(*) from field_definition where metadata_version_id=$1 and object_id=$2 and lifecycle_state<>'tombstone'",
				versionID, objectID,
			).Scan(&count); err != nil {
				return err
			}
			if count >= policy.MaxFieldsPerObject {
				return errFieldQuota
			}
		}
		if !existing {
			var reserved bool
			if err := tx.QueryRow(ctx,
				"select exists(select 1 from field_tombstone where object_id=$1 and api_name=$2 and field_id<>$3)",
				objectID, input.APIName, fieldID,
			).Scan(&reserved); err != nil {
				return err
			}
			if reserved {
				return errFieldNameReserved
			}
		}
		if input.Indexed && indexState != "retiring" {
			var count int
			if err := tx.QueryRow(ctx,
				"select count(*) from field_definition where metadata_version_id=$1 and object_id=$2 and indexed and index_state<>'retiring' and field_id<>$3",
				versionID, objectID, fieldID,
			).Scan(&count); err != nil {
				return err
			}
			if count >= policy.MaxActiveIndexedFields {
				return errIndexedFieldQuota
			}
		}
		_, err = tx.Exec(ctx,
			"insert into field_definition(tenant_bucket,tenant_id,metadata_version_id,field_id,object_id,api_name,label,description,data_type,required,indexed,unique_value,lifecycle_state,index_state,default_semantics,predecessor_field_id,default_value,constraints,semantic) "+
				"values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19) "+
				"on conflict (tenant_bucket,tenant_id,metadata_version_id,field_id) do update set object_id=excluded.object_id,api_name=excluded.api_name,label=excluded.label,description=excluded.description,data_type=excluded.data_type,required=excluded.required,indexed=excluded.indexed,unique_value=excluded.unique_value,lifecycle_state=excluded.lifecycle_state,index_state=excluded.index_state,default_semantics=excluded.default_semantics,predecessor_field_id=excluded.predecessor_field_id,default_value=excluded.default_value,constraints=excluded.constraints,semantic=excluded.semantic,updated_at=clock_timestamp()",
			tenantContext.Bucket, tenantContext.TenantID, versionID, fieldID, objectID, input.APIName, input.Label, input.Description, input.DataType, input.Required, input.Indexed, input.UniqueValue, lifecycleState, indexState, defaultSemantics, nullableUUID(predecessorID), nullableJSON(defaultValue), constraints, semantic)
		if err != nil {
			return err
		}
		return scanField(tx.QueryRow(ctx, "select "+fieldColumns+" from field_definition where metadata_version_id=$1 and field_id=$2", versionID, fieldID), &result)
	})
	if err != nil {
		return FieldDefinition{}, mapDatabaseError(err)
	}
	return result, nil
}

func (service *Service) UpsertRelation(ctx context.Context, request capability.Request, input RelationUpsertInput) (RelationDefinition, *capability.StableError) {
	versionID, stableErr := parseMetadataID(input.MetadataVersionID, "metadata_version_id")
	if stableErr != nil {
		return RelationDefinition{}, stableErr
	}
	sourceID, stableErr := parseMetadataID(input.SourceObjectID, "source_object_id")
	if stableErr != nil {
		return RelationDefinition{}, stableErr
	}
	targetID, stableErr := parseMetadataID(input.TargetObjectID, "target_object_id")
	if stableErr != nil {
		return RelationDefinition{}, stableErr
	}
	relationID, stableErr := optionalMetadataID(input.RelationID, "relation_id")
	if stableErr != nil {
		return RelationDefinition{}, stableErr
	}
	if relationID == uuid.Nil {
		var err error
		relationID, err = uuid.NewV7()
		if err != nil {
			return RelationDefinition{}, internalError()
		}
	}
	if !apiNamePattern.MatchString(input.APIName) || !validRelationType(input.RelationType) || !validDeleteBehavior(input.DeleteBehavior) {
		return RelationDefinition{}, validationError("relation api_name, type or delete_behavior is invalid")
	}
	semantic, stableErr := canonicalObject(input.Semantic, "semantic")
	if stableErr != nil {
		return RelationDefinition{}, stableErr
	}
	tenantContext, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return RelationDefinition{}, stableErr
	}
	var result RelationDefinition
	err := database.WithTenant(ctx, service.pool, tenantContext, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			"insert into relation_definition(tenant_bucket,tenant_id,metadata_version_id,relation_id,api_name,source_object_id,target_object_id,relation_type,delete_behavior,description,semantic) "+
				"values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) "+
				"on conflict (tenant_bucket,tenant_id,metadata_version_id,relation_id) do update set api_name=excluded.api_name,source_object_id=excluded.source_object_id,target_object_id=excluded.target_object_id,relation_type=excluded.relation_type,delete_behavior=excluded.delete_behavior,description=excluded.description,semantic=excluded.semantic,updated_at=clock_timestamp()",
			tenantContext.Bucket, tenantContext.TenantID, versionID, relationID, input.APIName, sourceID, targetID, input.RelationType, input.DeleteBehavior, input.Description, semantic)
		if err != nil {
			return err
		}
		return scanRelation(tx.QueryRow(ctx, "select "+relationColumns+" from relation_definition where metadata_version_id=$1 and relation_id=$2", versionID, relationID), &result)
	})
	if err != nil {
		return RelationDefinition{}, mapDatabaseError(err)
	}
	return result, nil
}

func (service *Service) Publish(ctx context.Context, request capability.Request, input PublishInput) (Version, *capability.StableError) {
	versionID, stableErr := parseMetadataID(input.MetadataVersionID, "metadata_version_id")
	if stableErr != nil {
		return Version{}, stableErr
	}
	approvalID := strings.TrimSpace(input.ApprovalID)
	if approvalID == "" {
		return Version{}, preconditionError("a manual approval id is required")
	}
	tenantContext, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Version{}, stableErr
	}
	var result Version
	err := database.WithTenant(ctx, service.pool, tenantContext, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, "select status from metadata_version where metadata_version_id=$1 for update", versionID).Scan(&status); err != nil {
			return err
		}
		if status != "draft" {
			return errPublishedVersion
		}
		var currentVersionID uuid.UUID
		if err := tx.QueryRow(ctx,
			"select coalesce(metadata_version_id,'00000000-0000-0000-0000-000000000000'::uuid) from tenant_registry where tenant_id=$1 for update",
			tenantContext.TenantID,
		).Scan(&currentVersionID); err != nil {
			return err
		}
		if currentVersionID != uuid.Nil {
			return errChangesetRequired
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
			versionID, encoded, digest, request.Actor.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "update tenant_registry set metadata_version_id=$2,updated_at=clock_timestamp() where tenant_id=$1", tenantContext.TenantID, versionID); err != nil {
			return err
		}
		if err := insertMetadataAudit(ctx, tx, request, tenantContext, versionID.String(), "published", map[string]any{
			"approval_id":         approvalID,
			"approval_mode":       "manual",
			"metadata_version_id": versionID.String(),
		}); err != nil {
			return err
		}
		return scanVersion(tx.QueryRow(ctx, "select "+versionColumns+" from metadata_version where metadata_version_id=$1", versionID), &result)
	})
	if err != nil {
		return Version{}, mapDatabaseError(err)
	}
	return result, nil
}

func (service *Service) Get(ctx context.Context, request capability.Request, input VersionInput) (Bundle, *capability.StableError) {
	versionID, stableErr := parseMetadataID(input.MetadataVersionID, "metadata_version_id")
	if stableErr != nil {
		return Bundle{}, stableErr
	}
	tenantContext, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return Bundle{}, stableErr
	}
	var bundle Bundle
	err := database.WithTenant(ctx, service.pool, tenantContext, func(tx pgx.Tx) error {
		if err := scanVersion(tx.QueryRow(ctx, "select "+versionColumns+" from metadata_version where metadata_version_id=$1", versionID), &bundle.Version); err != nil {
			return err
		}
		snapshot, _, err := compileTx(ctx, tx, versionID)
		if err != nil {
			return err
		}
		bundle.Objects, bundle.Fields, bundle.Relations = snapshot.Objects, snapshot.Fields, snapshot.Relations
		return nil
	})
	if err != nil {
		return Bundle{}, mapDatabaseError(err)
	}
	return bundle, nil
}

func (service *Service) Compile(ctx context.Context, request capability.Request, versionIDString string) ([]byte, string, *capability.StableError) {
	versionID, stableErr := parseMetadataID(versionIDString, "metadata_version_id")
	if stableErr != nil {
		return nil, "", stableErr
	}
	tenantContext, stableErr := service.tenantContext(ctx, request)
	if stableErr != nil {
		return nil, "", stableErr
	}
	var encoded []byte
	var digest string
	err := database.WithTenant(ctx, service.pool, tenantContext, func(tx pgx.Tx) error {
		snapshot, compiledDigest, err := compileTx(ctx, tx, versionID)
		if err != nil {
			return err
		}
		encoded, err = json.Marshal(snapshot)
		digest = compiledDigest
		return err
	})
	if err != nil {
		return nil, "", mapDatabaseError(err)
	}
	return encoded, digest, nil
}

type compiledSnapshot struct {
	Objects   []ObjectDefinition   `json:"objects"`
	Fields    []FieldDefinition    `json:"fields"`
	Relations []RelationDefinition `json:"relations"`
}

func compileTx(ctx context.Context, tx pgx.Tx, versionID uuid.UUID) (compiledSnapshot, string, error) {
	snapshot := compiledSnapshot{Objects: []ObjectDefinition{}, Fields: []FieldDefinition{}, Relations: []RelationDefinition{}}
	objectRows, err := tx.Query(ctx, "select "+objectColumns+" from object_definition where metadata_version_id=$1 order by api_name,object_id", versionID)
	if err != nil {
		return compiledSnapshot{}, "", err
	}
	for objectRows.Next() {
		var object ObjectDefinition
		if err := scanObject(objectRows, &object); err != nil {
			objectRows.Close()
			return compiledSnapshot{}, "", err
		}
		object.MetadataVersionID = ""
		snapshot.Objects = append(snapshot.Objects, object)
	}
	if err := objectRows.Err(); err != nil {
		objectRows.Close()
		return compiledSnapshot{}, "", err
	}
	objectRows.Close()
	fieldRows, err := tx.Query(ctx, "select "+fieldColumns+" from field_definition where metadata_version_id=$1 order by object_id,api_name,field_id", versionID)
	if err != nil {
		return compiledSnapshot{}, "", err
	}
	for fieldRows.Next() {
		var field FieldDefinition
		if err := scanField(fieldRows, &field); err != nil {
			fieldRows.Close()
			return compiledSnapshot{}, "", err
		}
		field.MetadataVersionID = ""
		snapshot.Fields = append(snapshot.Fields, field)
	}
	if err := fieldRows.Err(); err != nil {
		fieldRows.Close()
		return compiledSnapshot{}, "", err
	}
	fieldRows.Close()
	relationRows, err := tx.Query(ctx, "select "+relationColumns+" from relation_definition where metadata_version_id=$1 order by api_name,relation_id", versionID)
	if err != nil {
		return compiledSnapshot{}, "", err
	}
	for relationRows.Next() {
		var relation RelationDefinition
		if err := scanRelation(relationRows, &relation); err != nil {
			relationRows.Close()
			return compiledSnapshot{}, "", err
		}
		relation.MetadataVersionID = ""
		snapshot.Relations = append(snapshot.Relations, relation)
	}
	if err := relationRows.Err(); err != nil {
		relationRows.Close()
		return compiledSnapshot{}, "", err
	}
	relationRows.Close()
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return compiledSnapshot{}, "", err
	}
	sum := sha256.Sum256(encoded)
	return snapshot, hex.EncodeToString(sum[:]), nil
}

func (service *Service) tenantContext(ctx context.Context, request capability.Request) (database.TenantContext, *capability.StableError) {
	if request.Principal == nil || request.Principal.TenantID != request.TenantID || request.Principal.CompanyID == "" {
		return database.TenantContext{}, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted tenant identity is required"}
	}
	tenantID, err := uuid.Parse(request.TenantID)
	if err != nil || tenantID == uuid.Nil {
		return database.TenantContext{}, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted tenant identity is invalid"}
	}
	var bucket int16
	var nativeStatus string
	err = service.router.QueryRow(ctx, "select tenant_bucket,native_status from tenant_registry where tenant_id=$1 and company_id=$2", tenantID, request.Principal.CompanyID).Scan(&bucket, &nativeStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return database.TenantContext{}, &capability.StableError{Code: capability.CodeResourceNotFound, Message: "active Native tenant projection was not found"}
	}
	if err != nil {
		return database.TenantContext{}, internalError()
	}
	if nativeStatus != "active" {
		return database.TenantContext{}, preconditionError("Native tenant is not active")
	}
	return database.TenantContext{TenantID: tenantID, Bucket: bucket, ActorID: request.Actor.ID}, nil
}

const versionColumns = "metadata_version_id,sequence,status,coalesce(snapshot,'null'::jsonb),coalesce(snapshot_digest,''),created_by,coalesce(published_by,'')"
const objectColumns = "metadata_version_id,object_id,api_name,label,description,semantic"
const fieldColumns = "metadata_version_id,field_id,object_id,api_name,label,description,data_type,required,indexed,unique_value,lifecycle_state,index_state,default_semantics,coalesce(predecessor_field_id::text,''),default_value,constraints,semantic"
const relationColumns = "metadata_version_id,relation_id,api_name,source_object_id,target_object_id,relation_type,delete_behavior,description,semantic"

type rowScanner interface {
	Scan(...any) error
}

func scanVersion(row rowScanner, version *Version) error {
	return row.Scan(&version.MetadataVersionID, &version.Sequence, &version.Status, &version.Snapshot, &version.SnapshotDigest, &version.CreatedBy, &version.PublishedBy)
}

func scanObject(row rowScanner, object *ObjectDefinition) error {
	return row.Scan(&object.MetadataVersionID, &object.ObjectID, &object.APIName, &object.Label, &object.Description, &object.Semantic)
}

func scanField(row rowScanner, field *FieldDefinition) error {
	return row.Scan(&field.MetadataVersionID, &field.FieldID, &field.ObjectID, &field.APIName, &field.Label, &field.Description, &field.DataType, &field.Required, &field.Indexed, &field.UniqueValue, &field.LifecycleState, &field.IndexState, &field.DefaultSemantics, &field.PredecessorFieldID, &field.DefaultValue, &field.Constraints, &field.Semantic)
}

func scanRelation(row rowScanner, relation *RelationDefinition) error {
	return row.Scan(&relation.MetadataVersionID, &relation.RelationID, &relation.APIName, &relation.SourceObjectID, &relation.TargetObjectID, &relation.RelationType, &relation.DeleteBehavior, &relation.Description, &relation.Semantic)
}

var errPublishedVersion = errors.New("metadata version is not draft")
var errEmptyMetadata = errors.New("metadata version has no objects")
var errFieldQuota = errors.New("object dynamic field quota exceeded")
var errIndexedFieldQuota = errors.New("object indexed field quota exceeded")
var errChangesetRequired = errors.New("subsequent metadata publication requires metadata.changeset.publish")
var errFieldNameReserved = errors.New("field api_name is reserved by a tombstone")

func mapDatabaseError(err error) *capability.StableError {
	if errors.Is(err, pgx.ErrNoRows) {
		return &capability.StableError{Code: capability.CodeResourceNotFound, Message: "metadata resource was not found"}
	}
	if errors.Is(err, errPublishedVersion) || errors.Is(err, errEmptyMetadata) || errors.Is(err, errFieldQuota) || errors.Is(err, errIndexedFieldQuota) || errors.Is(err, errChangesetRequired) || errors.Is(err, errFieldNameReserved) {
		return preconditionError(err.Error())
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505", "23503":
			return &capability.StableError{Code: capability.CodeConflict, Message: "metadata identity or reference conflicts with existing definitions"}
		case "23514", "22P02":
			return validationError("metadata value violates its contract")
		case "55000":
			return preconditionError("published metadata is immutable")
		}
	}
	return internalError()
}

func parseMetadataID(raw, field string) (uuid.UUID, *capability.StableError) {
	value, err := uuid.Parse(raw)
	if err != nil || value == uuid.Nil || value.Version() != 7 {
		return uuid.Nil, validationError(field + " must be a UUIDv7")
	}
	return value, nil
}

func optionalMetadataID(raw, field string) (uuid.UUID, *capability.StableError) {
	if raw == "" {
		return uuid.Nil, nil
	}
	return parseMetadataID(raw, field)
}

func canonicalObject(raw json.RawMessage, field string) (json.RawMessage, *capability.StableError) {
	if len(raw) == 0 {
		return json.RawMessage("{}"), nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, validationError(field + " must be a JSON object")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, internalError()
	}
	return encoded, nil
}

func canonicalValue(raw json.RawMessage, field string) (json.RawMessage, *capability.StableError) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, validationError(field + " must be valid JSON")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, internalError()
	}
	return encoded, nil
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func nullableUUID(value uuid.UUID) any {
	if value == uuid.Nil {
		return nil
	}
	return value
}

func validDataType(value string) bool {
	return contains([]string{"text", "number", "boolean", "date", "datetime", "uuid", "json"}, value)
}

func validFieldLifecycle(value string) bool {
	return contains([]string{"active", "deprecated_read_write", "deprecated_read_only", "hidden", "purging", "tombstone"}, value)
}

func validIndexState(value string) bool {
	return contains([]string{"none", "building", "validating", "active", "failed", "retiring"}, value)
}

func validRelationType(value string) bool {
	return contains([]string{"lookup", "master_detail", "many_to_many"}, value)
}

func validDeleteBehavior(value string) bool {
	return contains([]string{"restrict", "cascade", "set_null"}, value)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validationError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeValidationFailed, Message: message}
}

func preconditionError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeFailedPrecondition, Message: message}
}

func internalError() *capability.StableError {
	return &capability.StableError{Code: capability.CodeInternal, Message: "metadata operation failed"}
}
