package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/OlivierZEN/ai-native-platform/internal/api"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/cli"
	mcpserver "github.com/OlivierZEN/ai-native-platform/internal/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func testRequest() capability.Request {
	return capability.Request{
		CapabilityID:   "system.capability.list",
		RequestID:      "req-adapter-parity",
		TenantID:       "tenant-poc",
		Actor:          capability.Actor{ID: "agent-poc", Scopes: []string{"system.capability.read"}},
		IdempotencyKey: "idem-adapter-parity",
		Input:          json.RawMessage(`{}`),
	}
}

func TestAPICLIAndMCPProduceEquivalentResult(t *testing.T) {
	requestBody, err := json.Marshal(testRequest())
	if err != nil {
		t.Fatal(err)
	}

	apiInvoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 2)
	apiRequest := httptest.NewRequest(http.MethodPost, "/v1/capabilities/system.capability.list/invoke", bytes.NewReader(requestBody))
	apiRecorder := httptest.NewRecorder()
	api.NewHandler(apiInvoker).ServeHTTP(apiRecorder, apiRequest)
	if apiRecorder.Code != http.StatusOK {
		t.Fatalf("API status = %d: %s", apiRecorder.Code, apiRecorder.Body.String())
	}
	var apiResult capability.Response
	if err := json.Unmarshal(apiRecorder.Body.Bytes(), &apiResult); err != nil {
		t.Fatalf("decode API result: %v", err)
	}

	cliInvoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 2)
	var stdout, stderr bytes.Buffer
	exitCode := cli.Run(context.Background(), cliInvoker, []string{"capability", "invoke", "--id", "system.capability.list"}, bytes.NewReader(requestBody), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("CLI exit = %d, stdout = %s, stderr = %s", exitCode, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("CLI stderr = %q, want no diagnostics", stderr.String())
	}
	var cliResult capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &cliResult); err != nil {
		t.Fatalf("decode CLI result: %v; stdout = %q", err, stdout.String())
	}

	if !equivalent(apiResult, cliResult) {
		t.Fatalf("adapter drift:\nAPI: %#v\nCLI: %#v", apiResult, cliResult)
	}

	mcpResult := invokeMCP(t, testRequest())
	if !equivalent(apiResult, mcpResult) {
		t.Fatalf("adapter drift:\nAPI: %#v\nMCP: %#v", apiResult, mcpResult)
	}
}

func TestAPIMapsStableErrorsAndCLIEmitsJSONError(t *testing.T) {
	request := testRequest()
	request.Actor.Scopes = nil
	requestBody, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	apiInvoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 1)
	apiRecorder := httptest.NewRecorder()
	api.NewHandler(apiInvoker).ServeHTTP(apiRecorder, httptest.NewRequest(http.MethodPost, "/v1/capabilities/system.capability.list/invoke", bytes.NewReader(requestBody)))
	if apiRecorder.Code != http.StatusForbidden {
		t.Fatalf("API status = %d, want %d", apiRecorder.Code, http.StatusForbidden)
	}

	cliInvoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 1)
	var stdout bytes.Buffer
	exitCode := cli.Run(context.Background(), cliInvoker, []string{"capability", "invoke", "--id", "system.capability.list"}, bytes.NewReader(requestBody), &stdout, io.Discard)
	if exitCode == 0 {
		t.Fatal("CLI exit = 0, want non-zero")
	}
	var result capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("CLI error is not JSON: %v", err)
	}
	if result.Error == nil || result.Error.Code != capability.CodeUnauthorized {
		t.Fatalf("CLI response = %#v", result)
	}
	mcpResult := invokeMCP(t, request)
	if mcpResult.Error == nil || mcpResult.Error.Code != capability.CodeUnauthorized {
		t.Fatalf("MCP response = %#v", mcpResult)
	}
}

func TestAPIRejectsMultipleJSONDocuments(t *testing.T) {
	invoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 1)
	requestBody, err := json.Marshal(testRequest())
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	api.NewHandler(invoker).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/capabilities/system.capability.list/invoke", bytes.NewReader(append(requestBody, []byte("\n{}")...))))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("API status = %d, want %d; body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	var response capability.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode API response: %v", err)
	}
	if response.Error == nil || response.Error.Code != capability.CodeValidationFailed {
		t.Fatalf("response = %#v, want validation failure", response)
	}
}

func TestAPICLIAndMCPUseSameStableFailureCodes(t *testing.T) {
	tests := []struct {
		name      string
		request   capability.Request
		preInvoke bool
		wantCode  capability.ErrorCode
	}{
		{
			name:     "semantic input validation",
			request:  requestWithInput(json.RawMessage(`[]`)),
			wantCode: capability.CodeValidationFailed,
		},
		{
			name: "authorization denial",
			request: func() capability.Request {
				request := testRequest()
				request.Actor.Scopes = nil
				return request
			}(),
			wantCode: capability.CodeUnauthorized,
		},
		{
			name:      "idempotency input conflict",
			request:   requestWithInput(json.RawMessage(`{"include_deprecated":true}`)),
			preInvoke: true,
			wantCode:  capability.CodeIdempotencyConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiInvoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 2)
			cliInvoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 2)
			mcpInvoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 2)
			if tt.preInvoke {
				seed := testRequest()
				if got := invokeAPI(t, apiInvoker, seed); got.Status != capability.StatusSucceeded {
					t.Fatalf("API seed response = %#v", got)
				}
				if got := invokeCLI(t, cliInvoker, seed); got.Status != capability.StatusSucceeded {
					t.Fatalf("CLI seed response = %#v", got)
				}
				if got := invokeMCPWithInvoker(t, mcpInvoker, seed); got.Status != capability.StatusSucceeded {
					t.Fatalf("MCP seed response = %#v", got)
				}
			}

			apiResult := invokeAPI(t, apiInvoker, tt.request)
			cliResult := invokeCLI(t, cliInvoker, tt.request)
			mcpResult := invokeMCPWithInvoker(t, mcpInvoker, tt.request)
			for adapter, result := range map[string]capability.Response{"API": apiResult, "CLI": cliResult, "MCP": mcpResult} {
				if result.Status != capability.StatusFailed || result.Error == nil || result.Error.Code != tt.wantCode {
					t.Fatalf("%s response = %#v, want stable error %q", adapter, result, tt.wantCode)
				}
			}
			if !equivalent(apiResult, cliResult) || !equivalent(apiResult, mcpResult) {
				t.Fatalf("failure adapter drift:\nAPI: %#v\nCLI: %#v\nMCP: %#v", apiResult, cliResult, mcpResult)
			}
		})
	}
}

func TestAPICLIAndMCPKeepReplayAuditContext(t *testing.T) {
	first := testRequest()
	first.RequestID = "req-replay-first"
	first.IdempotencyKey = "idem-adapter-replay"
	replay := first
	replay.RequestID = "req-replay-second"
	replay.Actor.ID = "agent-replay"

	apiInvoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 2)
	cliInvoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 2)
	mcpInvoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 2)

	if response := invokeAPI(t, apiInvoker, first); response.Status != capability.StatusSucceeded {
		t.Fatalf("API first response = %#v", response)
	}
	if response := invokeCLI(t, cliInvoker, first); response.Status != capability.StatusSucceeded {
		t.Fatalf("CLI first response = %#v", response)
	}
	if response := invokeMCPWithInvoker(t, mcpInvoker, first); response.Status != capability.StatusSucceeded {
		t.Fatalf("MCP first response = %#v", response)
	}

	apiReplay := invokeAPI(t, apiInvoker, replay)
	cliReplay := invokeCLI(t, cliInvoker, replay)
	mcpReplay := invokeMCPWithInvoker(t, mcpInvoker, replay)
	for adapter, response := range map[string]capability.Response{"API": apiReplay, "CLI": cliReplay, "MCP": mcpReplay} {
		if response.Status != capability.StatusSucceeded || response.RequestID != replay.RequestID || response.AuditID != "audit:"+replay.RequestID {
			t.Fatalf("%s replay response = %#v, want current request identity", adapter, response)
		}
	}
	if !equivalent(apiReplay, cliReplay) || !equivalent(apiReplay, mcpReplay) {
		t.Fatalf("replay adapter drift:\nAPI: %#v\nCLI: %#v\nMCP: %#v", apiReplay, cliReplay, mcpReplay)
	}
	for adapter, invoker := range map[string]*capability.Invoker{"API": apiInvoker, "CLI": cliInvoker, "MCP": mcpInvoker} {
		audits := invoker.Audits()
		if len(audits) != 2 {
			t.Fatalf("%s audit events = %#v, want two", adapter, audits)
		}
		if replayAudit := audits[1]; replayAudit.RequestID != replay.RequestID || replayAudit.AuditID != "audit:"+replay.RequestID || replayAudit.ActorID != replay.Actor.ID || replayAudit.TenantID != replay.TenantID || replayAudit.IdempotencyKey != replay.IdempotencyKey {
			t.Fatalf("%s replay audit = %#v, want current caller context", adapter, replayAudit)
		}
	}
}

func requestWithInput(input json.RawMessage) capability.Request {
	request := testRequest()
	request.Input = input
	return request
}

func invokeAPI(t *testing.T, invoker *capability.Invoker, request capability.Request) capability.Response {
	t.Helper()
	requestBody, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("encode API request: %v", err)
	}
	recorder := httptest.NewRecorder()
	api.NewHandler(invoker).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/capabilities/"+request.CapabilityID+"/invoke", bytes.NewReader(requestBody)))
	var response capability.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode API response: %v", err)
	}
	return response
}

func invokeCLI(t *testing.T, invoker *capability.Invoker, request capability.Request) capability.Response {
	t.Helper()
	requestBody, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("encode CLI request: %v", err)
	}
	var stdout bytes.Buffer
	_ = cli.Run(context.Background(), invoker, []string{"capability", "invoke", "--id", request.CapabilityID}, bytes.NewReader(requestBody), &stdout, io.Discard)
	var response capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode CLI response: %v", err)
	}
	return response
}

func invokeMCP(t *testing.T, request capability.Request) capability.Response {
	t.Helper()
	invoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 2)
	return invokeMCPWithInvoker(t, invoker, request)
}

func invokeMCPWithInvoker(t *testing.T, invoker *capability.Invoker, request capability.Request) capability.Response {
	t.Helper()
	ctx := context.Background()
	server := mcpserver.NewServer(invoker)
	client := mcp.NewClient(&mcp.Implementation{Name: "adapter-parity-client", Version: "v1"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect MCP server: %v", err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	defer clientSession.Close()

	var input any
	if err := json.Unmarshal(request.Input, &input); err != nil {
		t.Fatalf("decode test input: %v", err)
	}
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: capability.MCPToolName(request.CapabilityID),
		Arguments: map[string]any{
			"request_id":      request.RequestID,
			"tenant_id":       request.TenantID,
			"actor_id":        request.Actor.ID,
			"scopes":          request.Actor.Scopes,
			"idempotency_key": request.IdempotencyKey,
			"input":           input,
		},
	})
	if err != nil {
		t.Fatalf("call MCP tool: %v", err)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("encode MCP response: %v", err)
	}
	var response capability.Response
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	return response
}

func equivalent(left, right capability.Response) bool {
	if !equalJSON(left.Result, right.Result) {
		return false
	}
	if (left.Error == nil) != (right.Error == nil) {
		return false
	}
	if left.Error != nil && (left.Error.Code != right.Error.Code || left.Error.Message != right.Error.Message) {
		return false
	}
	return left.CapabilityID == right.CapabilityID &&
		left.RequestID == right.RequestID &&
		left.AuditID == right.AuditID &&
		left.Status == right.Status
}

func equalJSON(left, right json.RawMessage) bool {
	if len(left) == 0 || len(right) == 0 {
		return len(left) == len(right)
	}
	var leftValue, rightValue any
	return json.Unmarshal(left, &leftValue) == nil && json.Unmarshal(right, &rightValue) == nil && reflect.DeepEqual(leftValue, rightValue)
}
