package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type toolInput struct {
	RequestID      string   `json:"request_id"`
	TenantID       string   `json:"tenant_id"`
	ActorID        string   `json:"actor_id"`
	Scopes         []string `json:"scopes"`
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
	Input          any      `json:"input"`
}

func NewServer(invoker *capability.Invoker) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "ai-native-platform", Version: "v0.1.0"}, nil)
	for _, definition := range invoker.RegistryDefinitions() {
		addCapabilityTool(server, invoker, definition)
	}
	return server
}

func addCapabilityTool(server *mcp.Server, invoker *capability.Invoker, definition capability.Definition) {
	descriptor := definition.Descriptor
	server.AddTool(&mcp.Tool{
		Name:         capability.MCPToolName(descriptor.ID),
		Description:  descriptor.Description,
		InputSchema:  invocationInputSchema(descriptor.InputSchema),
		OutputSchema: invocationOutputSchema(descriptor.OutputSchema),
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		input, response := decodeToolInput(descriptor.ID, request.Params.Arguments)
		if response != nil {
			return responseResult(*response), nil
		}
		rawInput, err := json.Marshal(input.Input)
		if err != nil {
			return responseResult(failedToolResponse(descriptor.ID, input.RequestID, capability.CodeValidationFailed, "input cannot be encoded as JSON")), nil
		}

		result := invoker.Invoke(ctx, capability.Request{
			CapabilityID:   descriptor.ID,
			RequestID:      input.RequestID,
			TenantID:       input.TenantID,
			Actor:          capability.Actor{ID: input.ActorID, Scopes: input.Scopes},
			IdempotencyKey: input.IdempotencyKey,
			Input:          rawInput,
		})
		return responseResult(result), nil
	})
}

func decodeToolInput(capabilityID string, raw json.RawMessage) (toolInput, *capability.Response) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var input toolInput
	if err := decoder.Decode(&input); err != nil {
		response := failedToolResponse(capabilityID, "", capability.CodeValidationFailed, "invalid tool arguments: "+err.Error())
		return toolInput{}, &response
	}
	return input, nil
}

func failedToolResponse(capabilityID, requestID string, code capability.ErrorCode, message string) capability.Response {
	return capability.Response{
		CapabilityID: capabilityID,
		RequestID:    requestID,
		AuditID:      "audit:" + requestID,
		Status:       capability.StatusFailed,
		Error:        &capability.StableError{Code: code, Message: message},
	}
}

func invocationInputSchema(inputSchema json.RawMessage) json.RawMessage {
	return mustJSON(map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"request_id", "tenant_id", "actor_id", "scopes", "input"},
		"properties": map[string]any{
			"request_id":      map[string]any{"type": "string"},
			"tenant_id":       map[string]any{"type": "string"},
			"actor_id":        map[string]any{"type": "string"},
			"scopes":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"idempotency_key": map[string]any{"type": "string"},
			"input":           inputSchema,
		},
	})
}

func invocationOutputSchema(outputSchema json.RawMessage) json.RawMessage {
	return mustJSON(map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"capability_id", "request_id", "audit_id", "status"},
		"properties": map[string]any{
			"capability_id": map[string]any{"type": "string"},
			"request_id":    map[string]any{"type": "string"},
			"audit_id":      map[string]any{"type": "string"},
			"status":        map[string]any{"type": "string", "enum": []string{string(capability.StatusSucceeded), string(capability.StatusFailed)}},
			"result":        outputSchema,
			"error": map[string]any{"type": "object", "required": []string{"code", "message"}, "properties": map[string]any{
				"code":    map[string]any{"type": "string"},
				"message": map[string]any{"type": "string"},
			}},
		},
	})
}

func responseResult(response capability.Response) *mcp.CallToolResult {
	object := responseObject(response)
	encoded, err := json.Marshal(object)
	if err != nil {
		encoded = []byte(`{"status":"failed","error":{"code":"INTERNAL","message":"response cannot be encoded"}}`)
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
		StructuredContent: object,
		IsError:           response.Status == capability.StatusFailed,
	}
}

func responseObject(response capability.Response) map[string]any {
	encoded, err := json.Marshal(response)
	if err != nil {
		return map[string]any{"status": capability.StatusFailed, "error": map[string]any{"code": capability.CodeInternal, "message": "response is not JSON safe"}}
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		return map[string]any{"status": capability.StatusFailed, "error": map[string]any{"code": capability.CodeInternal, "message": "response cannot be converted to object"}}
	}
	return object
}

func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func RunStdio(ctx context.Context, invoker *capability.Invoker) error {
	return NewServer(invoker).Run(ctx, &mcp.StdioTransport{})
}
