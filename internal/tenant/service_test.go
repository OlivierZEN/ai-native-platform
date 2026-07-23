package tenant

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/api"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/cli"
	"github.com/OlivierZEN/ai-native-platform/internal/database/migrate"
	mcpserver "github.com/OlivierZEN/ai-native-platform/internal/mcp"
	"github.com/OlivierZEN/ai-native-platform/internal/operations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTenantLifecyclePersistentOperationsAndRevisions(t *testing.T) {
	pool := tenantTestPool(t)
	service := NewService(pool, operations.ClaimBoundPort{})
	invoker := capability.NewInvoker(capability.NewRegistry(CapabilityDefinitions(service)), 4)
	principal := tenantPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa")

	provision := map[string]any{
		"operation_id": "op-provision-a", "tenant_id": principal.TenantID, "company_id": principal.CompanyID,
		"tenant_revision": 1, "product_revision": 1, "display_name": "Tenant A",
		"service_tier": "standard", "global_lifecycle_status": "active",
		"entitlements": map[string]any{"record_limit": 1000000},
	}
	first := invokeTenant(t, invoker, principal, "tenant.provision", provision)
	assertSucceededStatus(t, first, "active", 1, "succeeded")
	legacyProvision := cloneMap(provision)
	delete(legacyProvision, "company_id")
	legacyProvision["org_id"] = principal.CompanyID
	assertErrorCode(t, invokeTenant(t, invoker, principal, "tenant.provision", legacyProvision), capability.CodeValidationFailed)

	replay := invokeTenant(t, invoker, principal, "tenant.provision", provision)
	if !equalResult(first.Result, replay.Result) {
		t.Fatalf("persistent replay drift: first=%s replay=%s", first.Result, replay.Result)
	}
	conflicting := cloneMap(provision)
	conflicting["display_name"] = "Changed"
	conflict := invokeTenant(t, invoker, principal, "tenant.provision", conflicting)
	assertErrorCode(t, conflict, capability.CodeIdempotencyConflict)

	outOfOrder := invokeTenant(t, invoker, principal, "tenant.suspend", map[string]any{"operation_id": "op-suspend-old", "product_revision": 1})
	assertErrorCode(t, outOfOrder, capability.CodeFailedPrecondition)
	suspended := invokeTenant(t, invoker, principal, "tenant.suspend", map[string]any{"operation_id": "op-suspend", "product_revision": 2})
	assertSucceededStatus(t, suspended, "suspended", 2, "succeeded")
	resumed := invokeTenant(t, invoker, principal, "tenant.resume", map[string]any{"operation_id": "op-resume", "product_revision": 3})
	assertSucceededStatus(t, resumed, "active", 3, "succeeded")
	entitled := invokeTenant(t, invoker, principal, "tenant.update-entitlement", map[string]any{
		"operation_id": "op-entitlement", "product_revision": 4, "entitlements": map[string]any{"record_limit": 2000000},
	})
	assertSucceededStatus(t, entitled, "active", 4, "succeeded")
	pending := invokeTenant(t, invoker, principal, "tenant.decommission", map[string]any{"operation_id": "op-decommission", "product_revision": 5})
	assertSucceededStatus(t, pending, "active", 4, "pending_approval")

	status := invokeTenant(t, invoker, principal, "tenant.get-status", map[string]any{})
	assertSucceededStatus(t, status, "active", 4, "")

	var operationCount, auditCount int
	if err := pool.QueryRow(context.Background(), "select count(*) from tenant_operation").Scan(&operationCount); err != nil {
		t.Fatalf("count operations: %v", err)
	}
	if err := pool.QueryRow(context.Background(), "select count(*) from audit_event").Scan(&auditCount); err != nil {
		t.Fatalf("count audit events: %v", err)
	}
	if operationCount != 5 {
		t.Fatalf("operation count=%d, want 5", operationCount)
	}
	if auditCount < 6 {
		t.Fatalf("audit count=%d, want at least 6", auditCount)
	}

	tenantB := tenantPrincipal("22222222-2222-4222-8222-222222222222", principal.CompanyID)
	failedProvision := cloneMap(provision)
	failedProvision["operation_id"] = "op-provision-b"
	failedProvision["tenant_id"] = tenantB.TenantID
	failedProvision["display_name"] = "Tenant B"
	assertErrorCode(t, invokeTenant(t, invoker, tenantB, "tenant.provision", failedProvision), capability.CodeConflict)
	tenantB.CompanyID = "orgbbbbbbbbbbbbbbbbb"
	failedProvision["company_id"] = tenantB.CompanyID
	recovered := invokeTenant(t, invoker, tenantB, "tenant.provision", failedProvision)
	assertSucceededStatus(t, recovered, "active", 1, "succeeded")
}

func TestTenantCapabilitiesProjectAuthenticatedAPICLIAndMCP(t *testing.T) {
	pool := tenantTestPool(t)
	service := NewService(pool, operations.ClaimBoundPort{})
	definitions := CapabilityDefinitions(service)
	invoker := capability.NewInvoker(capability.NewRegistry(definitions), 4)
	principal := tenantPrincipal("11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa")
	provision := map[string]any{
		"operation_id": "op-projection", "tenant_id": principal.TenantID, "company_id": principal.CompanyID,
		"tenant_revision": 1, "product_revision": 1, "display_name": "Projection Tenant",
		"service_tier": "standard", "global_lifecycle_status": "active",
	}
	assertSucceededStatus(t, invokeTenant(t, invoker, principal, "tenant.provision", provision), "active", 1, "succeeded")

	if len(definitions) != 6 || len(invoker.RegistryDescriptors()) != 6 {
		t.Fatalf("tenant definitions=%d descriptors=%d", len(definitions), len(invoker.RegistryDescriptors()))
	}
	for _, definition := range definitions {
		if definition.Descriptor.Version != "v2" {
			t.Fatalf("tenant capability %s version=%s, want v2", definition.Descriptor.ID, definition.Descriptor.Version)
		}
		if definition.Descriptor.ID == "tenant.provision" && (bytes.Contains(definition.Descriptor.InputSchema, []byte(`"org_id"`)) || !bytes.Contains(definition.Descriptor.InputSchema, []byte(`"company_id"`))) {
			t.Fatalf("tenant.provision schema did not complete company_id rename: %s", definition.Descriptor.InputSchema)
		}
	}
	request := capability.Request{CapabilityID: "tenant.get-status", RequestID: "req-adapter-tenant", Input: json.RawMessage("{}")}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/v1/capabilities/tenant.get-status/invoke", bytes.NewReader(body))
	httpRequest.Header.Set("Authorization", "Bearer test-token")
	api.NewAuthenticatedHandler(invoker, staticVerifier{principal: principal}).ServeHTTP(recorder, httpRequest)
	if recorder.Code != http.StatusOK {
		t.Fatalf("API status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var apiResponse capability.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &apiResponse); err != nil {
		t.Fatalf("decode API response: %v", err)
	}

	var stdout bytes.Buffer
	if exit := cli.RunAs(context.Background(), invoker, principal, []string{"capability", "invoke", "--id", request.CapabilityID}, bytes.NewReader(body), &stdout, io.Discard); exit != 0 {
		t.Fatalf("CLI exit=%d output=%s", exit, stdout.String())
	}
	var cliResponse capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &cliResponse); err != nil {
		t.Fatalf("decode CLI response: %v", err)
	}
	mcpResponse := invokeAuthenticatedMCP(t, invoker, principal, request)
	for adapter, response := range map[string]capability.Response{"API": apiResponse, "CLI": cliResponse, "MCP": mcpResponse} {
		if response.Status != capability.StatusSucceeded {
			t.Fatalf("%s response=%#v", adapter, response)
		}
	}
	if !equalResult(apiResponse.Result, cliResponse.Result) || !equalResult(apiResponse.Result, mcpResponse.Result) {
		t.Fatalf("tenant adapter result drift: API=%s CLI=%s MCP=%s", apiResponse.Result, cliResponse.Result, mcpResponse.Result)
	}

	unauthenticated := httptest.NewRecorder()
	api.NewAuthenticatedHandler(invoker, staticVerifier{principal: principal}).ServeHTTP(unauthenticated,
		httptest.NewRequest(http.MethodPost, "/v1/capabilities/tenant.get-status/invoke", bytes.NewReader(body)))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated API status=%d body=%s", unauthenticated.Code, unauthenticated.Body.String())
	}
	mismatched := request
	mismatched.TenantID = "22222222-2222-4222-8222-222222222222"
	mismatchBody, _ := json.Marshal(mismatched)
	mismatchRequest := httptest.NewRequest(http.MethodPost, "/v1/capabilities/tenant.get-status/invoke", bytes.NewReader(mismatchBody))
	mismatchRequest.Header.Set("Authorization", "Bearer test-token")
	mismatchRecorder := httptest.NewRecorder()
	api.NewAuthenticatedHandler(invoker, staticVerifier{principal: principal}).ServeHTTP(mismatchRecorder, mismatchRequest)
	if mismatchRecorder.Code != http.StatusBadRequest {
		t.Fatalf("mismatched tenant API status=%d body=%s", mismatchRecorder.Code, mismatchRecorder.Body.String())
	}
}

type staticVerifier struct {
	principal capability.TrustedPrincipal
}

func (verifier staticVerifier) Verify(context.Context, string) (capability.TrustedPrincipal, error) {
	return verifier.principal, nil
}

func tenantPrincipal(tenantID, companyID string) capability.TrustedPrincipal {
	return capability.TrustedPrincipal{
		TenantID:  tenantID,
		CompanyID: companyID,
		Actor: capability.Actor{ID: "operations-agent", Scopes: []string{
			"tenant.provision", "tenant.status.read", "tenant.lifecycle.write", "tenant.entitlement.write", "tenant.decommission",
		}},
		Source: "test-jwt",
	}
}

func invokeTenant(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, capabilityID string, input any) capability.Response {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	request, stableErr := capability.BindTrustedPrincipal(capability.Request{
		CapabilityID: capabilityID,
		RequestID:    "req-" + capabilityID + "-" + time.Now().Format("150405.000000000"),
		Input:        raw,
	}, principal)
	if stableErr != nil {
		t.Fatalf("bind principal: %v", stableErr)
	}
	return invoker.Invoke(context.Background(), request)
}

func assertSucceededStatus(t *testing.T, response capability.Response, nativeStatus string, revision int64, operationStatus string) {
	t.Helper()
	if response.Status != capability.StatusSucceeded {
		t.Fatalf("response=%#v", response)
	}
	var status TenantStatus
	if err := json.Unmarshal(response.Result, &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.NativeStatus != nativeStatus || status.ProductRevision != revision || status.OperationStatus != operationStatus {
		t.Fatalf("status=%#v", status)
	}
}

func assertErrorCode(t *testing.T, response capability.Response, code capability.ErrorCode) {
	t.Helper()
	if response.Status != capability.StatusFailed || response.Error == nil || response.Error.Code != code {
		t.Fatalf("response=%#v, want %s", response, code)
	}
}

func tenantTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	lock, err := admin.Acquire(context.Background())
	if err != nil {
		admin.Close()
		t.Fatalf("acquire integration lock connection: %v", err)
	}
	if _, err := lock.Exec(context.Background(), "select pg_advisory_lock(7167614658367249410)"); err != nil {
		lock.Release()
		admin.Close()
		t.Fatalf("acquire integration lock: %v", err)
	}
	if err := migrate.Apply(context.Background(), admin, migrate.Builtin()); err != nil {
		admin.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "alter role ai_native_control login"); err != nil {
		admin.Close()
		t.Fatalf("enable local control login: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "truncate tenant_registry cascade"); err != nil {
		admin.Close()
		t.Fatalf("reset tenant data: %v", err)
	}
	controlConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		admin.Close()
		t.Fatalf("parse control pool: %v", err)
	}
	controlConfig.ConnConfig.User = "ai_native_control"
	controlConfig.MaxConns = 4
	control, err := pgxpool.NewWithConfig(context.Background(), controlConfig)
	if err != nil {
		admin.Close()
		t.Fatalf("open control pool: %v", err)
	}
	if err := control.Ping(context.Background()); err != nil {
		control.Close()
		admin.Close()
		t.Fatalf("ping control pool: %v", err)
	}
	t.Cleanup(func() {
		control.Close()
		_, _ = lock.Exec(context.Background(), "select pg_advisory_unlock(7167614658367249410)")
		lock.Release()
		admin.Close()
	})
	return control
}

func invokeAuthenticatedMCP(t *testing.T, invoker *capability.Invoker, principal capability.TrustedPrincipal, request capability.Request) capability.Response {
	t.Helper()
	ctx := context.Background()
	server := mcpserver.NewServerAs(invoker, principal)
	client := mcp.NewClient(&mcp.Implementation{Name: "tenant-parity", Version: "v1"}, nil)
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
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: capability.MCPToolName(request.CapabilityID),
		Arguments: map[string]any{
			"request_id": request.RequestID,
			"input":      map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("call MCP: %v", err)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	var response capability.Response
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	return response
}

func equalResult(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	return json.Unmarshal(left, &leftValue) == nil && json.Unmarshal(right, &rightValue) == nil && reflect.DeepEqual(leftValue, rightValue)
}

func cloneMap(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
