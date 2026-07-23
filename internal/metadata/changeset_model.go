package metadata

import (
	"encoding/json"
	"time"
)

type Changeset struct {
	ChangesetID                string          `json:"changeset_id"`
	BaseMetadataVersionID      string          `json:"base_metadata_version_id,omitempty"`
	CandidateMetadataVersionID string          `json:"candidate_metadata_version_id"`
	State                      string          `json:"state"`
	RiskLevel                  string          `json:"risk_level"`
	RequiresBackfill           bool            `json:"requires_backfill"`
	OperationDigest            string          `json:"operation_digest"`
	QuotaSnapshot              json.RawMessage `json:"quota_snapshot"`
	Plan                       json.RawMessage `json:"plan"`
	Simulation                 json.RawMessage `json:"simulation"`
	Coverage                   json.RawMessage `json:"coverage"`
	ApprovalID                 string          `json:"approval_id,omitempty"`
	ApprovedBy                 string          `json:"approved_by,omitempty"`
	CreatedBy                  string          `json:"created_by"`
	LastErrorCode              string          `json:"last_error_code,omitempty"`
	LastErrorMessage           string          `json:"last_error_message,omitempty"`
	CreatedAt                  time.Time       `json:"created_at"`
	UpdatedAt                  time.Time       `json:"updated_at"`
	ApprovedAt                 *time.Time      `json:"approved_at,omitempty"`
	ActivatedAt                *time.Time      `json:"activated_at,omitempty"`
}

type ChangesetChange struct {
	ObjectID         string `json:"object_id"`
	FieldID          string `json:"field_id,omitempty"`
	APIName          string `json:"api_name,omitempty"`
	Kind             string `json:"kind"`
	From             string `json:"from,omitempty"`
	To               string `json:"to,omitempty"`
	EligibleRecords  int64  `json:"eligible_records"`
	RequiresBackfill bool   `json:"requires_backfill"`
	CoreSupported    bool   `json:"core_supported"`
}

type ObjectSimulation struct {
	ObjectID           string  `json:"object_id"`
	RecordCount        int64   `json:"record_count"`
	AverageJSONBytes   float64 `json:"average_json_bytes"`
	MaximumJSONBytes   int64   `json:"maximum_json_bytes"`
	ProjectedTypedRows int64   `json:"projected_typed_rows"`
}

type ChangesetPlan struct {
	Changes []ChangesetChange `json:"changes"`
}

type ChangesetSimulation struct {
	Objects []ObjectSimulation `json:"objects"`
}

type ChangesetValidateInput struct {
	CandidateMetadataVersionID string `json:"candidate_metadata_version_id"`
}

type ChangesetIDInput struct {
	ChangesetID string `json:"changeset_id"`
}

type ChangesetApproveInput struct {
	ChangesetID string `json:"changeset_id"`
	ApprovalID  string `json:"approval_id"`
}

type ChangesetRollbackInput struct {
	ChangesetID string `json:"changeset_id"`
	ApprovalID  string `json:"approval_id"`
}

type ChangesetBatchInput struct {
	ChangesetID string `json:"changeset_id"`
	BatchSize   int    `json:"batch_size,omitempty"`
}

type ChangesetPurgeInput struct {
	ChangesetID string `json:"changeset_id"`
	BatchSize   int    `json:"batch_size,omitempty"`
	ApprovalID  string `json:"approval_id"`
}

type BatchResult struct {
	ChangesetID      string          `json:"changeset_id"`
	State            string          `json:"state"`
	ObjectID         string          `json:"object_id,omitempty"`
	AttemptedRecords int64           `json:"attempted_records"`
	SucceededRecords int64           `json:"succeeded_records"`
	ConflictRecords  int64           `json:"conflict_records"`
	FailedRecords    int64           `json:"failed_records"`
	RemainingRecords int64           `json:"remaining_records"`
	Coverage         json.RawMessage `json:"coverage"`
}
