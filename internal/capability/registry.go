package capability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type RegistryView interface {
	Descriptors() []Descriptor
}

type Handler func(context.Context, Request, RegistryView) (any, *StableError)

type Definition struct {
	Descriptor    Descriptor
	ValidateInput func(json.RawMessage) *StableError
	Handler       Handler
}

// Meter receives compact, payload-free invocation facts. Implementations must
// never make an already-completed domain operation fail merely because a
// dashboard write is unavailable.
type Meter interface {
	RecordInvocation(context.Context, Request, Response, bool, time.Duration)
}

type Registry struct {
	definitions map[string]Definition
}

func NewRegistry(definitions []Definition) *Registry {
	registry := &Registry{definitions: make(map[string]Definition, len(definitions))}
	toolNames := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		if definition.Handler == nil || definition.ValidateInput == nil {
			panic("capability definition requires validator and handler")
		}
		validateDefinition(definition)
		if _, exists := registry.definitions[definition.Descriptor.ID]; exists {
			panic("duplicate capability definition: " + definition.Descriptor.ID)
		}
		if definition.Descriptor.State == PublicationPublished {
			toolName := MCPToolName(definition.Descriptor.ID)
			if existingID, exists := toolNames[toolName]; exists {
				panic("published capability MCP projection collides: " + existingID + " and " + definition.Descriptor.ID)
			}
			toolNames[toolName] = definition.Descriptor.ID
		}
		registry.definitions[definition.Descriptor.ID] = definition
	}
	return registry
}

func (r *Registry) Lookup(id string) (Definition, bool) {
	definition, ok := r.definitions[id]
	return definition, ok
}

func (r *Registry) Descriptors() []Descriptor {
	definitions := r.PublishedDefinitions()
	descriptors := make([]Descriptor, 0, len(definitions))
	for _, definition := range definitions {
		descriptors = append(descriptors, definition.Descriptor)
	}
	return descriptors
}

func (r *Registry) PublishedDefinitions() []Definition {
	definitions := make([]Definition, 0, len(r.definitions))
	for _, definition := range r.definitions {
		if definition.Descriptor.State == PublicationPublished {
			definitions = append(definitions, definition)
		}
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Descriptor.ID < definitions[j].Descriptor.ID })
	return definitions
}

type idempotencyRecord struct {
	fingerprint string
	response    Response
	completed   bool
	done        chan struct{}
}

type Invoker struct {
	registry    *Registry
	concurrency chan struct{}
	mu          sync.Mutex
	idempotency map[string]idempotencyRecord
	audits      []AuditEvent
	meter       Meter
}

func NewInvoker(registry *Registry, maxConcurrency int) *Invoker {
	return NewMeteredInvoker(registry, maxConcurrency, nil)
}

func NewMeteredInvoker(registry *Registry, maxConcurrency int, meter Meter) *Invoker {
	if maxConcurrency < 1 {
		panic("max concurrency must be positive")
	}
	return &Invoker{
		registry:    registry,
		concurrency: make(chan struct{}, maxConcurrency),
		idempotency: make(map[string]idempotencyRecord),
		meter:       meter,
	}
}

func (i *Invoker) Invoke(ctx context.Context, request Request) Response {
	started := time.Now()
	executed := false
	response := i.invoke(ctx, request, &executed)
	if i.meter != nil {
		i.meter.RecordInvocation(ctx, request, response, executed, time.Since(started))
	}
	return response
}

func (i *Invoker) invoke(ctx context.Context, request Request, executed *bool) Response {
	auditID := "audit:" + request.RequestID
	if err := validateRequest(request); err != nil {
		return i.record(request, failedResponse(request, auditID, err))
	}

	definition, ok := i.registry.Lookup(request.CapabilityID)
	if !ok || definition.Descriptor.State != PublicationPublished {
		return i.record(request, failedResponse(request, auditID, &StableError{Code: CodeCapabilityNotFound, Message: "capability is not published"}))
	}
	if !hasScope(request.Actor.Scopes, definition.Descriptor.RequiredScope) {
		return i.record(request, failedResponse(request, auditID, &StableError{Code: CodeUnauthorized, Message: "actor lacks the required capability scope"}))
	}
	if err := definition.ValidateInput(request.Input); err != nil {
		return i.record(request, failedResponse(request, auditID, err))
	}
	if request.IdempotencyKey != "" && !definition.Descriptor.Idempotency.Enabled {
		return i.record(request, failedResponse(request, auditID, &StableError{Code: CodeValidationFailed, Message: "capability does not accept an idempotency key"}))
	}

	key, fingerprint := idempotencyValues(request)
	if key != "" {
		existing, execute, err := i.reserveIdempotency(ctx, key, fingerprint)
		if err != nil {
			return i.record(request, failedResponse(request, auditID, err))
		}
		if !execute {
			return i.record(request, replayResponse(existing, request, auditID))
		}
	}
	*executed = true

	select {
	case i.concurrency <- struct{}{}:
		defer func() { <-i.concurrency }()
	case <-ctx.Done():
		return i.completeAndRecord(request, key, failedResponse(request, auditID, &StableError{Code: CodeOverloaded, Message: "request cancelled before execution"}))
	}

	payload, stableErr := definition.Handler(ctx, request, i.registry)
	if stableErr != nil {
		return i.completeAndRecord(request, key, failedResponse(request, auditID, stableErr))
	}
	result, err := json.Marshal(payload)
	if err != nil {
		return i.completeAndRecord(request, key, failedResponse(request, auditID, &StableError{Code: CodeInternal, Message: "capability result is not JSON safe"}))
	}
	response := Response{CapabilityID: request.CapabilityID, RequestID: request.RequestID, AuditID: auditID, Status: StatusSucceeded, Result: result}
	return i.completeAndRecord(request, key, response)
}

// reserveIdempotency atomically claims an unseen key or waits for the existing
// invocation to complete. Only the caller that receives execute=true may call
// the handler, which prevents duplicated side effects under concurrent retries.
func (i *Invoker) reserveIdempotency(ctx context.Context, key, fingerprint string) (response Response, execute bool, stableErr *StableError) {
	for {
		i.mu.Lock()
		existing, exists := i.idempotency[key]
		if !exists {
			i.idempotency[key] = idempotencyRecord{fingerprint: fingerprint, done: make(chan struct{})}
			i.mu.Unlock()
			return Response{}, true, nil
		}
		if existing.fingerprint != fingerprint {
			i.mu.Unlock()
			return Response{}, false, &StableError{Code: CodeIdempotencyConflict, Message: "idempotency key was already used with different input"}
		}
		if existing.completed {
			i.mu.Unlock()
			return existing.response, false, nil
		}
		done := existing.done
		i.mu.Unlock()

		select {
		case <-done:
			// The record is now complete. Re-read it while holding the mutex.
		case <-ctx.Done():
			return Response{}, false, &StableError{Code: CodeOverloaded, Message: "request cancelled while awaiting idempotent invocation"}
		}
	}
}

func (i *Invoker) completeAndRecord(request Request, key string, response Response) Response {
	if key != "" {
		i.mu.Lock()
		existing, ok := i.idempotency[key]
		if !ok || existing.completed {
			i.mu.Unlock()
			panic("idempotency completion without an active reservation")
		}
		existing.response = response
		existing.completed = true
		i.idempotency[key] = existing
		close(existing.done)
		i.mu.Unlock()
	}
	return i.record(request, response)
}

func replayResponse(original Response, request Request, auditID string) Response {
	replay := original
	replay.CapabilityID = request.CapabilityID
	replay.RequestID = request.RequestID
	replay.AuditID = auditID
	return replay
}

func (i *Invoker) Audits() []AuditEvent {
	i.mu.Lock()
	defer i.mu.Unlock()
	return append([]AuditEvent(nil), i.audits...)
}

func (i *Invoker) RegistryDescriptors() []Descriptor {
	return i.registry.Descriptors()
}

func (i *Invoker) RegistryDefinitions() []Definition {
	return i.registry.PublishedDefinitions()
}

func (i *Invoker) record(request Request, response Response) Response {
	event := AuditEvent{
		AuditID:        response.AuditID,
		RequestID:      response.RequestID,
		CapabilityID:   response.CapabilityID,
		TenantID:       request.TenantID,
		ActorID:        request.Actor.ID,
		IdempotencyKey: request.IdempotencyKey,
		Status:         response.Status,
	}
	if response.Error != nil {
		event.ErrorCode = response.Error.Code
	}
	i.mu.Lock()
	i.audits = append(i.audits, event)
	i.mu.Unlock()
	return response
}

func MCPToolName(capabilityID string) string {
	return strings.ReplaceAll(capabilityID, ".", "_")
}

func validateDefinition(definition Definition) {
	descriptor := definition.Descriptor
	if !validCapabilityID(descriptor.ID) || descriptor.Version == "" || descriptor.Description == "" || descriptor.RequiredScope == "" {
		panic("capability definition requires a valid ID, version, description, and required scope")
	}
	if descriptor.State != PublicationDraft && descriptor.State != PublicationPublished {
		panic("capability definition requires a valid publication state")
	}
	if descriptor.RiskLevel != "low" && descriptor.RiskLevel != "medium" && descriptor.RiskLevel != "high" {
		panic("capability definition requires a valid risk level")
	}
	if descriptor.Execution.Mode != ExecutionSynchronous && descriptor.Execution.Mode != ExecutionAsynchronous {
		panic("capability definition requires a valid execution mode")
	}
	if descriptor.State == PublicationPublished && descriptor.RiskLevel == "high" && (descriptor.Execution.Mode != ExecutionAsynchronous || !descriptor.Execution.ApprovalRequired) {
		panic("published high-risk capability requires asynchronous approval")
	}
	if descriptor.State == PublicationPublished && (!validObjectSchema(descriptor.InputSchema) || !validObjectSchema(descriptor.OutputSchema)) {
		panic("published capability requires object input and output schemas")
	}
}

func validObjectSchema(raw json.RawMessage) bool {
	var schema struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(raw, &schema) == nil && schema.Type == "object"
}

func validCapabilityID(id string) bool {
	segments := strings.Split(id, ".")
	if len(segments) < 2 {
		return false
	}
	for _, segment := range segments {
		if segment == "" || segment[0] < 'a' || segment[0] > 'z' {
			return false
		}
		for _, character := range segment {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func failedResponse(request Request, auditID string, err *StableError) Response {
	return Response{CapabilityID: request.CapabilityID, RequestID: request.RequestID, AuditID: auditID, Status: StatusFailed, Error: err}
}

func validateRequest(request Request) *StableError {
	if request.CapabilityID == "" || request.RequestID == "" || request.TenantID == "" || request.Actor.ID == "" {
		return &StableError{Code: CodeValidationFailed, Message: "capability_id, request_id, tenant_id, and actor.id are required"}
	}
	if len(request.Input) == 0 {
		return &StableError{Code: CodeValidationFailed, Message: "input is required"}
	}
	return nil
}

func hasScope(scopes []string, required string) bool {
	for _, scope := range scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func idempotencyValues(request Request) (string, string) {
	if request.IdempotencyKey == "" {
		return "", ""
	}
	canonical := fmt.Sprintf("%s\x00%s\x00%s", request.CapabilityID, request.TenantID, strings.TrimSpace(string(request.Input)))
	sum := sha256.Sum256([]byte(canonical))
	return request.TenantID + "\x00" + request.CapabilityID + "\x00" + request.IdempotencyKey, hex.EncodeToString(sum[:])
}
