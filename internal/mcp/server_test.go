package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	mcpserver "github.com/OlivierZEN/ai-native-platform/internal/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSystemCapabilityListMCPToolUsesSharedInvocation(t *testing.T) {
	ctx := context.Background()
	invoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 1)
	server := mcpserver.NewServer(invoker)
	client := mcp.NewClient(&mcp.Implementation{Name: "parity-client", Version: "v1"}, nil)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "system_capability_list",
		Arguments: map[string]any{
			"request_id":      "req-mcp-parity",
			"tenant_id":       "tenant-poc",
			"actor_id":        "agent-poc",
			"scopes":          []string{"system.capability.read"},
			"idempotency_key": "idem-mcp-parity",
			"input":           map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("MCP result reported error: %#v", result)
	}

	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("encode structured response: %v", err)
	}
	var response capability.Response
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatalf("decode structured response: %v", err)
	}
	if response.Status != capability.StatusSucceeded || response.AuditID != "audit:req-mcp-parity" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if len(invoker.Audits()) != 1 {
		t.Fatalf("audit events = %#v, want one shared invocation", invoker.Audits())
	}
}

func TestServerProjectsOnlyPublishedRegistryDefinitions(t *testing.T) {
	ctx := context.Background()
	definitions := []capability.Definition{
		projectionDefinition("system.registry-projection", capability.PublicationPublished),
		projectionDefinition("system.draft-projection", capability.PublicationDraft),
	}
	invoker := capability.NewInvoker(capability.NewRegistry(definitions), 1)
	server := mcpserver.NewServer(invoker)
	client := mcp.NewClient(&mcp.Implementation{Name: "projection-client", Version: "v1"}, nil)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer clientSession.Close()

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools.Tools) != 1 {
		t.Fatalf("tools = %#v, want only the published registry entry", tools.Tools)
	}
	tool := tools.Tools[0]
	if tool.Name != capability.MCPToolName("system.registry-projection") || tool.Description != "projected test capability" {
		t.Fatalf("tool metadata was not projected from registry: %#v", tool)
	}
	schema, ok := tool.InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("input schema type = %T, want object", tool.InputSchema)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("input schema properties = %#v", schema)
	}
	inputSchema, ok := properties["input"].(map[string]any)
	if !ok || inputSchema["title"] != "RegistryProjectionInput" {
		t.Fatalf("capability input schema was not projected: %#v", inputSchema)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: tool.Name,
		Arguments: map[string]any{
			"request_id": "req-projected-tool",
			"tenant_id":  "tenant-poc",
			"actor_id":   "agent-poc",
			"scopes":     []string{"system.capability.read"},
			"input":      map[string]any{"marker": "ok"},
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("projected tool result = %#v, err = %v", result, err)
	}
}

func projectionDefinition(id string, state capability.PublicationState) capability.Definition {
	return capability.Definition{
		Descriptor: capability.Descriptor{
			ID:            id,
			Version:       "v1",
			Description:   "projected test capability",
			RiskLevel:     "low",
			State:         state,
			RequiredScope: "system.capability.read",
			InputSchema:   json.RawMessage(`{"type":"object","title":"RegistryProjectionInput","properties":{"marker":{"type":"string"}}}`),
			OutputSchema:  json.RawMessage(`{"type":"object","properties":{"projected":{"type":"boolean"}}}`),
			Idempotency:   capability.IdempotencyPolicy{Enabled: true},
			Execution:     capability.ExecutionPolicy{Mode: capability.ExecutionSynchronous},
		},
		ValidateInput: func(json.RawMessage) *capability.StableError { return nil },
		Handler: func(context.Context, capability.Request, capability.RegistryView) (any, *capability.StableError) {
			return map[string]bool{"projected": true}, nil
		},
	}
}
