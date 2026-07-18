package capability

import "encoding/json"

type Status string

const (
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type ErrorCode string

const (
	CodeValidationFailed    ErrorCode = "VALIDATION_FAILED"
	CodeCapabilityNotFound  ErrorCode = "CAPABILITY_NOT_FOUND"
	CodeUnauthorized        ErrorCode = "UNAUTHORIZED"
	CodeIdempotencyConflict ErrorCode = "IDEMPOTENCY_CONFLICT"
	CodeOverloaded          ErrorCode = "OVERLOADED"
	CodeInternal            ErrorCode = "INTERNAL"
)

type StableError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type Actor struct {
	ID     string   `json:"id"`
	Scopes []string `json:"scopes"`
}

type Request struct {
	CapabilityID   string          `json:"capability_id"`
	RequestID      string          `json:"request_id"`
	TenantID       string          `json:"tenant_id"`
	Actor          Actor           `json:"actor"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Input          json.RawMessage `json:"input"`
}

type Response struct {
	CapabilityID string          `json:"capability_id"`
	RequestID    string          `json:"request_id"`
	AuditID      string          `json:"audit_id"`
	Status       Status          `json:"status"`
	Result       json.RawMessage `json:"result,omitempty"`
	Error        *StableError    `json:"error,omitempty"`
}

type Descriptor struct {
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	Description   string            `json:"description"`
	RiskLevel     string            `json:"risk_level"`
	State         PublicationState  `json:"state"`
	RequiredScope string            `json:"required_scope"`
	InputSchema   json.RawMessage   `json:"input_schema"`
	OutputSchema  json.RawMessage   `json:"output_schema"`
	Idempotency   IdempotencyPolicy `json:"idempotency"`
	Execution     ExecutionPolicy   `json:"execution"`
}

type PublicationState string

const (
	PublicationDraft     PublicationState = "draft"
	PublicationPublished PublicationState = "published"
)

type IdempotencyPolicy struct {
	Enabled bool `json:"enabled"`
}

type ExecutionMode string

const (
	ExecutionSynchronous  ExecutionMode = "synchronous"
	ExecutionAsynchronous ExecutionMode = "asynchronous"
)

type ExecutionPolicy struct {
	Mode             ExecutionMode `json:"mode"`
	ApprovalRequired bool          `json:"approval_required"`
}

type ListResult struct {
	Capabilities []Descriptor `json:"capabilities"`
}

type AuditEvent struct {
	AuditID        string    `json:"audit_id"`
	RequestID      string    `json:"request_id"`
	CapabilityID   string    `json:"capability_id"`
	TenantID       string    `json:"tenant_id"`
	ActorID        string    `json:"actor_id"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	Status         Status    `json:"status"`
	ErrorCode      ErrorCode `json:"error_code,omitempty"`
}
