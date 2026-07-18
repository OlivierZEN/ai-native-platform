package capability

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func authorizedRequest() Request {
	return Request{
		CapabilityID:   "system.capability.list",
		RequestID:      "req-parity-001",
		TenantID:       "tenant-poc",
		Actor:          Actor{ID: "agent-poc", Scopes: []string{"system.capability.read"}},
		IdempotencyKey: "idem-parity-001",
		Input:          json.RawMessage(`{}`),
	}
}

func testDescriptor(id, description string) Descriptor {
	return Descriptor{
		ID:            id,
		Version:       "v1",
		Description:   description,
		RiskLevel:     "low",
		State:         PublicationPublished,
		RequiredScope: "system.capability.read",
		InputSchema:   json.RawMessage(`{"type":"object"}`),
		OutputSchema:  json.RawMessage(`{"type":"object"}`),
		Idempotency:   IdempotencyPolicy{Enabled: true},
		Execution:     ExecutionPolicy{Mode: ExecutionSynchronous},
	}
}

func TestInvokeSystemCapabilityListReturnsStableEnvelope(t *testing.T) {
	invoker := NewInvoker(NewRegistry(SystemCapabilityDefinitions()), 2)

	got := invoker.Invoke(context.Background(), authorizedRequest())
	if got.Status != StatusSucceeded {
		t.Fatalf("status = %q, want %q; error = %#v", got.Status, StatusSucceeded, got.Error)
	}
	if got.CapabilityID != "system.capability.list" || got.RequestID != "req-parity-001" {
		t.Fatalf("unexpected identity envelope: %#v", got)
	}
	if got.AuditID != "audit:req-parity-001" {
		t.Fatalf("audit ID = %q, want deterministic audit ID", got.AuditID)
	}

	var payload ListResult
	if err := json.Unmarshal(got.Result, &payload); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(payload.Capabilities) != 1 || payload.Capabilities[0].ID != "system.capability.list" {
		t.Fatalf("unexpected capabilities: %#v", payload.Capabilities)
	}
	audits := invoker.Audits()
	if len(audits) != 1 || audits[0].TenantID != "tenant-poc" || audits[0].ActorID != "agent-poc" {
		t.Fatalf("audit event does not preserve caller context: %#v", audits)
	}
}

func TestInvokeRejectsInvalidRequestsWithStableErrorCodes(t *testing.T) {
	invoker := NewInvoker(NewRegistry(SystemCapabilityDefinitions()), 1)

	tests := []struct {
		name    string
		request Request
		code    ErrorCode
	}{
		{
			name:    "unknown capability",
			request: Request{CapabilityID: "system.unknown", RequestID: "req-unknown", TenantID: "tenant-poc", Actor: Actor{ID: "agent-poc", Scopes: []string{"system.capability.read"}}, Input: json.RawMessage(`{}`)},
			code:    CodeCapabilityNotFound,
		},
		{
			name:    "missing scope",
			request: Request{CapabilityID: "system.capability.list", RequestID: "req-denied", TenantID: "tenant-poc", Actor: Actor{ID: "agent-poc"}, Input: json.RawMessage(`{}`)},
			code:    CodeUnauthorized,
		},
		{
			name:    "non object input",
			request: Request{CapabilityID: "system.capability.list", RequestID: "req-invalid", TenantID: "tenant-poc", Actor: Actor{ID: "agent-poc", Scopes: []string{"system.capability.read"}}, Input: json.RawMessage(`[]`)},
			code:    CodeValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := invoker.Invoke(context.Background(), tt.request)
			if got.Status != StatusFailed || got.Error == nil || got.Error.Code != tt.code {
				t.Fatalf("response = %#v, want error %q", got, tt.code)
			}
		})
	}
}

func TestInvokeRejectsIdempotencyKeyReuseWithDifferentInput(t *testing.T) {
	invoker := NewInvoker(NewRegistry(SystemCapabilityDefinitions()), 1)
	first := authorizedRequest()
	if got := invoker.Invoke(context.Background(), first); got.Status != StatusSucceeded {
		t.Fatalf("first response = %#v", got)
	}

	second := first
	second.RequestID = "req-parity-002"
	second.Input = json.RawMessage(`{"include_deprecated":true}`)
	got := invoker.Invoke(context.Background(), second)
	if got.Status != StatusFailed || got.Error == nil || got.Error.Code != CodeIdempotencyConflict {
		t.Fatalf("response = %#v, want idempotency conflict", got)
	}
}

func TestInvokeReplayUsesCurrentRequestAndAuditIdentity(t *testing.T) {
	invoker := NewInvoker(NewRegistry(SystemCapabilityDefinitions()), 1)
	first := authorizedRequest()
	first.RequestID = "req-idempotency-first"
	first.IdempotencyKey = "idem-audit-replay"
	if got := invoker.Invoke(context.Background(), first); got.Status != StatusSucceeded {
		t.Fatalf("first response = %#v", got)
	}

	second := first
	second.RequestID = "req-idempotency-replay"
	second.Actor.ID = "agent-replay"
	got := invoker.Invoke(context.Background(), second)
	if got.Status != StatusSucceeded || got.RequestID != second.RequestID || got.AuditID != "audit:"+second.RequestID {
		t.Fatalf("replayed response does not identify this invocation: %#v", got)
	}

	audits := invoker.Audits()
	if len(audits) != 2 {
		t.Fatalf("audit events = %#v, want one event per invocation", audits)
	}
	if replay := audits[1]; replay.RequestID != second.RequestID || replay.AuditID != "audit:"+second.RequestID || replay.ActorID != second.Actor.ID || replay.TenantID != second.TenantID {
		t.Fatalf("replay audit is not internally consistent: %#v", replay)
	}
}

func TestInvokeCollapsesConcurrentIdenticalIdempotencyRequests(t *testing.T) {
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var calls atomic.Int32
	definition := Definition{
		Descriptor:    testDescriptor("system.idempotent", "test-only idempotent capability"),
		ValidateInput: func(json.RawMessage) *StableError { return nil },
		Handler: func(context.Context, Request, RegistryView) (any, *StableError) {
			calls.Add(1)
			started <- struct{}{}
			<-release
			return map[string]bool{"ok": true}, nil
		},
	}
	invoker := NewInvoker(NewRegistry([]Definition{definition}), 4)
	request := authorizedRequest()
	request.CapabilityID = "system.idempotent"
	request.IdempotencyKey = "idem-concurrent"

	var group sync.WaitGroup
	responses := make(chan Response, 4)
	gate := make(chan struct{})
	for index := 0; index < 4; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-gate
			invocation := request
			invocation.RequestID = "req-concurrent-" + string(rune('a'+index))
			responses <- invoker.Invoke(context.Background(), invocation)
		}(index)
	}
	close(gate)
	<-started
	select {
	case <-started:
		close(release)
		t.Fatal("identical idempotency requests executed the handler more than once")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	group.Wait()
	close(responses)

	if calls.Load() != 1 {
		t.Fatalf("handler calls = %d, want 1", calls.Load())
	}
	for response := range responses {
		if response.Status != StatusSucceeded || response.RequestID == "" || response.AuditID != "audit:"+response.RequestID {
			t.Fatalf("unexpected concurrent response: %#v", response)
		}
	}
	if audits := invoker.Audits(); len(audits) != 4 {
		t.Fatalf("audit events = %#v, want one event per invocation", audits)
	}
}

func TestInvokeReturnsOverloadedWhenConcurrencyIsExhausted(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	definition := Definition{
		Descriptor:    testDescriptor("system.block", "test-only blocking capability"),
		ValidateInput: func(json.RawMessage) *StableError { return nil },
		Handler: func(context.Context, Request, RegistryView) (any, *StableError) {
			close(started)
			<-release
			return map[string]bool{"ok": true}, nil
		},
	}
	invoker := NewInvoker(NewRegistry([]Definition{definition}), 1)
	first := authorizedRequest()
	first.CapabilityID, first.RequestID, first.IdempotencyKey = "system.block", "req-blocking", ""
	done := make(chan Response, 1)
	go func() { done <- invoker.Invoke(context.Background(), first) }()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	second := first
	second.RequestID = "req-overloaded"
	got := invoker.Invoke(ctx, second)
	if got.Status != StatusFailed || got.Error == nil || got.Error.Code != CodeOverloaded {
		t.Fatalf("response = %#v, want overloaded response", got)
	}
	close(release)
	if completed := <-done; completed.Status != StatusSucceeded {
		t.Fatalf("first response = %#v", completed)
	}
}

func TestRegistryRejectsPublishedDefinitionsWithoutRequiredContractMetadata(t *testing.T) {
	tests := []struct {
		name       string
		descriptor Descriptor
	}{
		{
			name: "missing output schema",
			descriptor: func() Descriptor {
				descriptor := testDescriptor("system.missing-output", "invalid published definition")
				descriptor.OutputSchema = nil
				return descriptor
			}(),
		},
		{
			name: "synchronous high risk",
			descriptor: func() Descriptor {
				descriptor := testDescriptor("system.high-risk", "invalid high-risk definition")
				descriptor.RiskLevel = "high"
				return descriptor
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("NewRegistry did not reject invalid published contract")
				}
			}()
			NewRegistry([]Definition{{
				Descriptor:    tt.descriptor,
				ValidateInput: func(json.RawMessage) *StableError { return nil },
				Handler: func(context.Context, Request, RegistryView) (any, *StableError) {
					return map[string]bool{"ok": true}, nil
				},
			}})
		})
	}
}

func TestInvokeRejectsIdempotencyKeyWhenContractDisablesIt(t *testing.T) {
	descriptor := testDescriptor("system.no-idempotency", "test-only non-idempotent capability")
	descriptor.Idempotency.Enabled = false
	invoker := NewInvoker(NewRegistry([]Definition{{
		Descriptor:    descriptor,
		ValidateInput: func(json.RawMessage) *StableError { return nil },
		Handler: func(context.Context, Request, RegistryView) (any, *StableError) {
			return map[string]bool{"ok": true}, nil
		},
	}}), 1)
	request := authorizedRequest()
	request.CapabilityID = descriptor.ID
	request.IdempotencyKey = "not-supported"
	got := invoker.Invoke(context.Background(), request)
	if got.Status != StatusFailed || got.Error == nil || got.Error.Code != CodeValidationFailed {
		t.Fatalf("response = %#v, want idempotency policy validation failure", got)
	}
}

func TestDraftCapabilityIsExcludedFromDiscoveryAndInvocation(t *testing.T) {
	published := testDescriptor("system.published", "published capability")
	draft := testDescriptor("system.draft", "draft capability")
	draft.State = PublicationDraft
	definitions := []Definition{
		{
			Descriptor:    published,
			ValidateInput: func(json.RawMessage) *StableError { return nil },
			Handler: func(context.Context, Request, RegistryView) (any, *StableError) {
				return map[string]bool{"published": true}, nil
			},
		},
		{
			Descriptor:    draft,
			ValidateInput: func(json.RawMessage) *StableError { return nil },
			Handler: func(context.Context, Request, RegistryView) (any, *StableError) {
				return map[string]bool{"draft": true}, nil
			},
		},
	}
	invoker := NewInvoker(NewRegistry(definitions), 1)
	descriptors := invoker.RegistryDescriptors()
	if len(descriptors) != 1 || descriptors[0].ID != published.ID {
		t.Fatalf("discovery exposed an unpublished capability: %#v", descriptors)
	}
	request := authorizedRequest()
	request.CapabilityID = draft.ID
	request.IdempotencyKey = ""
	got := invoker.Invoke(context.Background(), request)
	if got.Status != StatusFailed || got.Error == nil || got.Error.Code != CodeCapabilityNotFound {
		t.Fatalf("draft invocation = %#v, want unpublished capability rejection", got)
	}
}
