package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const trustedPrincipalTokenInfoKey = "cloudcc_semattice.trusted_principal"

// BearerIdentityVerifier validates the identity attached to every Streamable
// HTTP request. The expiration is propagated to the MCP SDK so it can enforce
// the bearer-token lifecycle and bind sessions to the verified principal.
type BearerIdentityVerifier func(context.Context, string) (capability.TrustedPrincipal, time.Time, error)

type toolInput struct {
	RequestID      string   `json:"request_id"`
	TenantID       string   `json:"tenant_id"`
	ActorID        string   `json:"actor_id"`
	Scopes         []string `json:"scopes"`
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
	Input          any      `json:"input"`
}

func NewServer(invoker *capability.Invoker) *mcp.Server {
	return newServer(invoker, nil, false)
}

func NewServerAs(invoker *capability.Invoker, principal capability.TrustedPrincipal) *mcp.Server {
	return newServer(invoker, &principal, false)
}

// NewAuthenticatedStreamableHTTPHandler exposes the MCP Streamable HTTP
// transport. Each HTTP request must have a verified bearer identity. The
// SDK's token context retains a tenant-qualified subject so subsequent requests
// for a stateful MCP session cannot be replayed by a different principal.
func NewAuthenticatedStreamableHTTPHandler(invoker *capability.Invoker, verify BearerIdentityVerifier) http.Handler {
	if verify == nil {
		panic("authenticated streamable MCP handler requires an identity verifier")
	}

	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		// An unauthenticated session is permitted for MCP discovery only. Tool
		// handlers independently require a verified principal from RequestExtra,
		// so possession of a discovery session ID grants no invocation rights.
		return newServer(invoker, nil, true)
	}, &mcp.StreamableHTTPOptions{SessionTimeout: 5 * time.Minute})

	// Streamable HTTP must reject cross-origin browser requests. The Go standard
	// library guard validates Origin/Fetch Metadata while the SDK additionally
	// rejects localhost DNS rebinding through an invalid Host header.
	protected := http.NewCrossOriginProtection().Handler(streamable)
	return methodScopedBearer(protected, func(ctx context.Context, rawToken string, _ *http.Request) (*auth.TokenInfo, error) {
		principal, expiresAt, err := verify(ctx, rawToken)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
		}
		if expiresAt.IsZero() {
			return nil, fmt.Errorf("%w: identity token is missing expiry", auth.ErrInvalidToken)
		}
		return &auth.TokenInfo{
			Scopes:     append([]string(nil), principal.Actor.Scopes...),
			Expiration: expiresAt,
			UserID:     principal.TenantID + "\x00" + principal.Actor.ID,
			Extra:      map[string]any{trustedPrincipalTokenInfoKey: principal},
		}, nil
	})
}

// methodScopedBearer permits only the protocol discovery calls without a
// bearer token. Any other MCP method, including tools/call, must carry a token
// and is verified on that individual HTTP request. If a caller supplies a
// token for discovery it is still verified, preserving authenticated clients'
// session-binding protections.
func methodScopedBearer(next http.Handler, verify auth.TokenVerifier) http.Handler {
	authenticated := auth.RequireBearerToken(verify, nil)(next)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		required, err := bearerRequiredForMCPRequest(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		if required || strings.TrimSpace(request.Header.Get("Authorization")) != "" {
			authenticated.ServeHTTP(writer, request)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// bearerRequiredForMCPRequest allowlists only metadata discovery. It restores
// the body after inspecting it because the Streamable HTTP SDK consumes the
// same JSON-RPC request body later.
func bearerRequiredForMCPRequest(request *http.Request) (bool, error) {
	if request.Method != http.MethodPost {
		return true, nil
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, (1<<20)+1))
	if err != nil {
		return false, err
	}
	if len(body) > 1<<20 {
		return false, fmt.Errorf("MCP request body exceeds 1 MiB")
	}
	_ = request.Body.Close()
	request.Body = io.NopCloser(bytes.NewReader(body))

	var batch []struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(body, &batch); err == nil {
		for _, message := range batch {
			if !anonymousDiscoveryMethod(message.Method) {
				return true, nil
			}
		}
		return false, nil
	}
	var message struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(body, &message); err != nil {
		// Fail closed: an unparseable request must not accidentally be treated as
		// a discovery request.
		return true, nil
	}
	return !anonymousDiscoveryMethod(message.Method), nil
}

func anonymousDiscoveryMethod(method string) bool {
	return method == "initialize" || method == "notifications/initialized" || method == "tools/list"
}

func newServer(invoker *capability.Invoker, principal *capability.TrustedPrincipal, requireRequestPrincipal bool) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "ai-native-platform", Version: "v0.1.0"}, nil)
	for _, definition := range invoker.RegistryDefinitions() {
		addCapabilityTool(server, invoker, definition, principal, requireRequestPrincipal)
	}
	return server
}

func addCapabilityTool(server *mcp.Server, invoker *capability.Invoker, definition capability.Definition, principal *capability.TrustedPrincipal, requireRequestPrincipal bool) {
	descriptor := definition.Descriptor
	server.AddTool(&mcp.Tool{
		Name:         capability.MCPToolName(descriptor.ID),
		Description:  descriptor.Description,
		InputSchema:  invocationInputSchema(descriptor.InputSchema, principal != nil || requireRequestPrincipal),
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

		invocation := capability.Request{
			CapabilityID:   descriptor.ID,
			RequestID:      input.RequestID,
			TenantID:       input.TenantID,
			Actor:          capability.Actor{ID: input.ActorID, Scopes: input.Scopes},
			IdempotencyKey: input.IdempotencyKey,
			Input:          rawInput,
		}
		verifiedPrincipal := principal
		if requireRequestPrincipal {
			var stableErr *capability.StableError
			verifiedPrincipal, stableErr = principalFromToolRequest(request)
			if stableErr != nil {
				return responseResult(failedToolResponse(descriptor.ID, input.RequestID, stableErr.Code, stableErr.Message)), nil
			}
		}
		if verifiedPrincipal != nil {
			bound, stableErr := capability.BindTrustedPrincipal(invocation, *verifiedPrincipal)
			if stableErr != nil {
				return responseResult(failedToolResponse(descriptor.ID, input.RequestID, stableErr.Code, stableErr.Message)), nil
			}
			invocation = bound
		}
		invocation.Entrypoint = "mcp"
		result := invoker.Invoke(ctx, invocation)
		return responseResult(result), nil
	})
}

func principalFromToolRequest(request *mcp.CallToolRequest) (*capability.TrustedPrincipal, *capability.StableError) {
	if request == nil || request.Extra == nil || request.Extra.TokenInfo == nil {
		return nil, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "valid bearer authentication is required for tool calls"}
	}
	principal, ok := request.Extra.TokenInfo.Extra[trustedPrincipalTokenInfoKey].(capability.TrustedPrincipal)
	if !ok {
		return nil, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "verified bearer identity is incomplete"}
	}
	return &principal, nil
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

func invocationInputSchema(inputSchema json.RawMessage, authenticated bool) json.RawMessage {
	required := []string{"request_id", "tenant_id", "actor_id", "scopes", "input"}
	if authenticated {
		required = []string{"request_id", "input"}
	}
	return mustJSON(map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"additionalProperties": false,
		"required":             required,
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

func RunStdioAs(ctx context.Context, invoker *capability.Invoker, principal capability.TrustedPrincipal) error {
	return NewServerAs(invoker, principal).Run(ctx, &mcp.StdioTransport{})
}
