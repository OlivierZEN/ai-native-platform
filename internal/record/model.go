package record

import (
	"encoding/json"
	"time"
)

type Record struct {
	MetadataVersionID  string         `json:"metadata_version_id"`
	ObjectID           string         `json:"object_id"`
	ObjectAPIName      string         `json:"object_api_name"`
	RecordID           string         `json:"record_id"`
	OwnerID            string         `json:"owner_principal_id,omitempty"`
	DataOrganizationID string         `json:"data_organization_id,omitempty"`
	LifecycleState     string         `json:"lifecycle_state"`
	Data               map[string]any `json:"data"`
	Revision           int64          `json:"revision"`
	CreatedBy          string         `json:"created_by"`
	UpdatedBy          string         `json:"updated_by"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          *time.Time     `json:"deleted_at,omitempty"`
}

type CreateInput struct {
	ObjectAPIName string          `json:"object_api_name"`
	RecordID      string          `json:"record_id,omitempty"`
	Data          json.RawMessage `json:"data"`
}

type GetInput struct {
	ObjectAPIName  string `json:"object_api_name"`
	RecordID       string `json:"record_id"`
	IncludeDeleted bool   `json:"include_deleted,omitempty"`
}

type UpdateInput struct {
	ObjectAPIName    string          `json:"object_api_name"`
	RecordID         string          `json:"record_id"`
	ExpectedRevision int64           `json:"expected_revision"`
	Patch            json.RawMessage `json:"patch"`
}

type DeleteInput struct {
	ObjectAPIName    string `json:"object_api_name"`
	RecordID         string `json:"record_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type FilterInput struct {
	Field    string          `json:"field"`
	Operator string          `json:"op"`
	Value    json.RawMessage `json:"value"`
}

type QueryInput struct {
	ObjectAPIName string        `json:"object_api_name"`
	Filters       []FilterInput `json:"filters,omitempty"`
	After         string        `json:"after,omitempty"`
	Limit         int           `json:"limit,omitempty"`
}

type QueryResult struct {
	Records    []Record  `json:"records"`
	NextCursor string    `json:"next_cursor,omitempty"`
	Plan       QueryPlan `json:"plan"`
}

type QueryPlan struct {
	Strategy      string   `json:"strategy"`
	IndexedFields []string `json:"indexed_fields,omitempty"`
	Limit         int      `json:"limit"`
}

type fieldSpec struct {
	FieldID            string
	APIName            string
	DataType           string
	Required           bool
	Indexed            bool
	UniqueValue        bool
	LifecycleState     string
	IndexState         string
	DefaultSemantics   string
	PredecessorFieldID string
	DefaultValue       json.RawMessage
	Constraints        map[string]any
}

type relationSpec struct {
	RelationID     string
	APIName        string
	TargetObjectID string
	RelationType   string
	DeleteBehavior string
}

type objectModel struct {
	MetadataVersionID string
	ObjectID          string
	APIName           string
	Fields            map[string]fieldSpec
	Relations         map[string]relationSpec
}

type normalizedFilter struct {
	Field       fieldSpec
	Table       string
	ValueColumn string
	OperatorSQL string
	Value       any
}
