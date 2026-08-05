package metadata

import (
	"encoding/json"
)

type Version struct {
	MetadataVersionID string          `json:"metadata_version_id"`
	Sequence          int64           `json:"sequence"`
	Status            string          `json:"status"`
	Snapshot          json.RawMessage `json:"snapshot,omitempty"`
	SnapshotDigest    string          `json:"snapshot_digest,omitempty"`
	CreatedBy         string          `json:"created_by"`
	PublishedBy       string          `json:"published_by,omitempty"`
}

type ObjectDefinition struct {
	MetadataVersionID string          `json:"metadata_version_id"`
	ObjectID          string          `json:"object_id"`
	APIName           string          `json:"api_name"`
	Label             string          `json:"label"`
	Description       string          `json:"description"`
	Semantic          json.RawMessage `json:"semantic"`
}

type FieldDefinition struct {
	MetadataVersionID  string          `json:"metadata_version_id"`
	FieldID            string          `json:"field_id"`
	ObjectID           string          `json:"object_id"`
	APIName            string          `json:"api_name"`
	Label              string          `json:"label"`
	Description        string          `json:"description"`
	DataType           string          `json:"data_type"`
	Required           bool            `json:"required"`
	Indexed            bool            `json:"indexed"`
	UniqueValue        bool            `json:"unique_value"`
	LifecycleState     string          `json:"lifecycle_state"`
	IndexState         string          `json:"index_state"`
	DefaultSemantics   string          `json:"default_semantics"`
	PredecessorFieldID string          `json:"predecessor_field_id,omitempty"`
	DefaultValue       json.RawMessage `json:"default_value,omitempty"`
	Constraints        json.RawMessage `json:"constraints"`
	Semantic           json.RawMessage `json:"semantic"`
}

type RelationDefinition struct {
	MetadataVersionID string          `json:"metadata_version_id"`
	RelationID        string          `json:"relation_id"`
	APIName           string          `json:"api_name"`
	SourceObjectID    string          `json:"source_object_id"`
	TargetObjectID    string          `json:"target_object_id"`
	RelationType      string          `json:"relation_type"`
	DeleteBehavior    string          `json:"delete_behavior"`
	Description       string          `json:"description"`
	Semantic          json.RawMessage `json:"semantic"`
}

type Bundle struct {
	Version   Version              `json:"version"`
	Objects   []ObjectDefinition   `json:"objects"`
	Fields    []FieldDefinition    `json:"fields"`
	Relations []RelationDefinition `json:"relations"`
}

type CreateVersionInput struct{}

type ObjectUpsertInput struct {
	MetadataVersionID string          `json:"metadata_version_id"`
	ObjectID          string          `json:"object_id,omitempty"`
	APIName           string          `json:"api_name"`
	Label             string          `json:"label"`
	Description       string          `json:"description,omitempty"`
	Semantic          json.RawMessage `json:"semantic,omitempty"`
}

type ObjectGetInput struct {
	MetadataVersionID string `json:"metadata_version_id"`
	ObjectID          string `json:"object_id"`
}

type ObjectListInput struct {
	MetadataVersionID string `json:"metadata_version_id"`
}

type ObjectDeleteResult struct {
	Object            ObjectDefinition `json:"object"`
	DeletedFieldCount int64            `json:"deleted_field_count"`
	Deleted           bool             `json:"deleted"`
}

type FieldUpsertInput struct {
	MetadataVersionID  string          `json:"metadata_version_id"`
	FieldID            string          `json:"field_id,omitempty"`
	ObjectID           string          `json:"object_id"`
	APIName            string          `json:"api_name"`
	Label              string          `json:"label"`
	Description        string          `json:"description,omitempty"`
	DataType           string          `json:"data_type"`
	Required           bool            `json:"required,omitempty"`
	Indexed            bool            `json:"indexed,omitempty"`
	UniqueValue        bool            `json:"unique_value,omitempty"`
	LifecycleState     string          `json:"lifecycle_state,omitempty"`
	IndexState         string          `json:"index_state,omitempty"`
	DefaultSemantics   string          `json:"default_semantics,omitempty"`
	PredecessorFieldID string          `json:"predecessor_field_id,omitempty"`
	DefaultValue       json.RawMessage `json:"default_value,omitempty"`
	Constraints        json.RawMessage `json:"constraints,omitempty"`
	Semantic           json.RawMessage `json:"semantic,omitempty"`
}

type FieldGetInput struct {
	MetadataVersionID string `json:"metadata_version_id"`
	FieldID           string `json:"field_id"`
}

type FieldListInput struct {
	MetadataVersionID string `json:"metadata_version_id"`
	ObjectID          string `json:"object_id,omitempty"`
}

type FieldDeleteResult struct {
	Field   FieldDefinition `json:"field"`
	Deleted bool            `json:"deleted"`
}

type RelationUpsertInput struct {
	MetadataVersionID string          `json:"metadata_version_id"`
	RelationID        string          `json:"relation_id,omitempty"`
	APIName           string          `json:"api_name"`
	SourceObjectID    string          `json:"source_object_id"`
	TargetObjectID    string          `json:"target_object_id"`
	RelationType      string          `json:"relation_type"`
	DeleteBehavior    string          `json:"delete_behavior"`
	Description       string          `json:"description,omitempty"`
	Semantic          json.RawMessage `json:"semantic,omitempty"`
}

type VersionInput struct {
	MetadataVersionID string `json:"metadata_version_id"`
}

type PublishInput struct {
	MetadataVersionID string `json:"metadata_version_id"`
	ApprovalID        string `json:"approval_id"`
}
