package record

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var derivedTables = []string{
	"record_index_text",
	"record_index_number",
	"record_index_boolean",
	"record_index_datetime",
	"record_index_uuid",
}

func rebuildDerivedState(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, model objectModel, recordID uuid.UUID, data map[string]any) error {
	return rebuildDerivedStateWithIndexStates(ctx, tx, tenant, model, recordID, data, false)
}

func rebuildDerivedStateForEvolution(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, model objectModel, recordID uuid.UUID, data map[string]any) error {
	return rebuildDerivedStateWithIndexStates(ctx, tx, tenant, model, recordID, data, true)
}

func rebuildDerivedStateWithIndexStates(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, model objectModel, recordID uuid.UUID, data map[string]any, includeBuilding bool) error {
	if err := deleteDerivedState(ctx, tx, tenant, model.ObjectID, recordID); err != nil {
		return err
	}
	for name, field := range model.Fields {
		if !field.Indexed || (field.IndexState != "active" && !(includeBuilding && (field.IndexState == "building" || field.IndexState == "validating"))) {
			continue
		}
		value, exists := data[name]
		if !exists || value == nil {
			continue
		}
		common := []any{tenant.Bucket, tenant.TenantID, model.MetadataVersionID, model.ObjectID, field.FieldID, recordID}
		var err error
		switch field.DataType {
		case "text":
			_, err = tx.Exec(ctx,
				"insert into record_index_text(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_text) values ($1,$2,$3,$4,$5,$6,$7)",
				append(common, value.(string))...,
			)
		case "number":
			_, err = tx.Exec(ctx,
				"insert into record_index_number(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_number) values ($1,$2,$3,$4,$5,$6,$7)",
				append(common, value.(json.Number).String())...,
			)
		case "boolean":
			_, err = tx.Exec(ctx,
				"insert into record_index_boolean(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_boolean) values ($1,$2,$3,$4,$5,$6,$7)",
				append(common, value.(bool))...,
			)
		case "date", "datetime":
			parsed, parseErr := parseDateTime(field.DataType, value)
			if parseErr != nil {
				return parseErr
			}
			_, err = tx.Exec(ctx,
				"insert into record_index_datetime(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_kind,value_datetime) values ($1,$2,$3,$4,$5,$6,$7,$8)",
				append(common, field.DataType, parsed.UTC())...,
			)
		case "uuid":
			_, err = tx.Exec(ctx,
				"insert into record_index_uuid(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_uuid) values ($1,$2,$3,$4,$5,$6,$7)",
				append(common, value.(string))...,
			)
		case "json":
			return preconditionError("indexed JSON fields are not supported by the bounded runtime")
		default:
			return preconditionError("indexed field type is not supported: " + field.DataType)
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
				return validationError("unique field canonical value exceeds 1024 bytes: " + field.APIName)
			}
			hash := sha256.Sum256(canonical)
			if _, err := tx.Exec(ctx,
				"insert into record_unique_value(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id,record_id,value_hash,value_canonical) values ($1,$2,$3,$4,$5,$6,$7,$8)",
				tenant.Bucket, tenant.TenantID, model.MetadataVersionID, model.ObjectID, field.FieldID, recordID, hash[:], string(canonical),
			); err != nil {
				return err
			}
		}
	}
	return rebuildRelations(ctx, tx, tenant, model, recordID, data)
}

func deleteDerivedState(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, objectID string, recordID uuid.UUID) error {
	for _, table := range derivedTables {
		if _, err := tx.Exec(ctx, "delete from "+table+" where tenant_bucket=$1 and tenant_id=$2 and object_id=$3 and record_id=$4", tenant.Bucket, tenant.TenantID, objectID, recordID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx,
		"delete from record_unique_value where tenant_bucket=$1 and tenant_id=$2 and object_id=$3 and record_id=$4",
		tenant.Bucket, tenant.TenantID, objectID, recordID,
	); err != nil {
		return err
	}
	_, err := tx.Exec(ctx,
		"delete from record_relation where tenant_bucket=$1 and tenant_id=$2 and source_object_id=$3 and source_record_id=$4",
		tenant.Bucket, tenant.TenantID, objectID, recordID,
	)
	return err
}

func rebuildRelations(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, model objectModel, sourceRecordID uuid.UUID, data map[string]any) error {
	for name, relation := range model.Relations {
		value, exists := data[name]
		if !exists || value == nil {
			continue
		}
		targets, err := relationTargets(value)
		if err != nil {
			return err
		}
		for _, target := range targets {
			var active bool
			if err := tx.QueryRow(ctx,
				"select exists(select 1 from object_record where object_id=$1 and record_id=$2 and lifecycle_state='active')",
				relation.TargetObjectID, target,
			).Scan(&active); err != nil {
				return err
			}
			if !active {
				return validationError(fmt.Sprintf("relation %s target is not an active record", relation.APIName))
			}
			relationID, err := uuid.NewV7()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				"insert into record_relation(tenant_bucket,tenant_id,metadata_version_id,relation_id,relation_definition_id,source_object_id,source_record_id,target_object_id,target_record_id) "+
					"values ($1,$2,$3,$4,$5,$6,$7,$8,$9)",
				tenant.Bucket, tenant.TenantID, model.MetadataVersionID, relationID, relation.RelationID, model.ObjectID, sourceRecordID, relation.TargetObjectID, target,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func relationTargets(value any) ([]uuid.UUID, error) {
	parse := func(raw any) (uuid.UUID, error) {
		text, ok := raw.(string)
		if !ok {
			return uuid.Nil, fmt.Errorf("relation target is not a UUID string")
		}
		return uuid.Parse(text)
	}
	switch typed := value.(type) {
	case string:
		value, err := uuid.Parse(typed)
		if err != nil {
			return nil, err
		}
		return []uuid.UUID{value}, nil
	case []string:
		result := make([]uuid.UUID, 0, len(typed))
		for _, raw := range typed {
			value, err := uuid.Parse(raw)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		return result, nil
	case []any:
		result := make([]uuid.UUID, 0, len(typed))
		for _, raw := range typed {
			value, err := parse(raw)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("relation value has unsupported type %T", value)
	}
}
