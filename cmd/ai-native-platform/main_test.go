package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/database/migrate"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBinaryCapabilityInvokeIsNonInteractiveJSON(t *testing.T) {
	binary := buildBinary(t)
	input := `{"capability_id":"system.capability.list","request_id":"req-child-cli","tenant_id":"tenant-poc","actor":{"id":"agent-poc","scopes":["system.capability.read"]},"input":{}}`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "capability", "invoke", "--id", "system.capability.list")
	command.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("run binary: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no diagnostics", stderr.String())
	}
	var response map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("stdout is not a single JSON result: %v; stdout=%q", err, stdout.String())
	}
	if response["status"] != "succeeded" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestBinaryMCPStdioWritesOnlyJSONRPCToStdout(t *testing.T) {
	binary := buildBinary(t)
	const identityKey = "0123456789abcdef0123456789abcdef"
	claims := jwt.MapClaims{
		"iss": "https://identity.example.test", "aud": "native-platform", "sub": "agent-poc",
		"iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		"tenant_id": "11111111-1111-4111-8111-111111111111", "company_id": "orgaaaaaaaaaaaaaaaaa",
		"scopes": []string{"system.capability.read"},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(identityKey))
	if err != nil {
		t.Fatalf("sign identity token: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "mcp", "stdio")
	command.Env = append(os.Environ(),
		"AI_NATIVE_IDENTITY_ISSUER=https://identity.example.test",
		"AI_NATIVE_IDENTITY_AUDIENCE=native-platform",
		"AI_NATIVE_IDENTITY_ALGORITHM=HS256",
		"AI_NATIVE_IDENTITY_HMAC_KEY="+identityKey,
		"AI_NATIVE_IDENTITY_TOKEN="+token,
	)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		t.Fatalf("start MCP server: %v", err)
	}
	waited := false
	defer func() {
		if waited {
			return
		}
		_ = stdin.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
	}()

	scanner := bufio.NewScanner(stdout)
	writeMessage(t, stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"stdout-test","version":"v1"}}}`)
	initializeLine := nextJSONRPCLine(t, scanner)
	writeMessage(t, stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	writeMessage(t, stdin, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"system_capability_list","arguments":{"request_id":"req-child-mcp","input":{}}}}`)
	toolLine := nextJSONRPCLine(t, scanner)
	_ = stdin.Close()
	waitErr := command.Wait()
	waited = true
	if waitErr != nil {
		t.Fatalf("MCP process exit: %v", waitErr)
	}

	for _, line := range []string{initializeLine, toolLine} {
		var message map[string]any
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			t.Fatalf("stdout has protocol-external data %q: %v", line, err)
		}
		if message["jsonrpc"] != "2.0" {
			t.Fatalf("not JSON-RPC: %#v", message)
		}
	}
}

func TestDatabaseMigrateCommandIsExplicitAndRepeatable(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	t.Setenv("AI_NATIVE_DATABASE_URL", databaseURL)
	for attempt := range 2 {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), []string{"db", "migrate"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("attempt %d exit=%d stdout=%q stderr=%q", attempt, code, stdout.String(), stderr.String())
		}
		var response map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &response); err != nil || response["status"] != "succeeded" {
			t.Fatalf("attempt %d response=%#v err=%v", attempt, response, err)
		}
		if stderr.Len() != 0 {
			t.Fatalf("attempt %d stderr=%q", attempt, stderr.String())
		}
	}
}

func TestBinaryWiringUsesSeparateControlAndRuntimeRoles(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(context.Background(), databaseURL)
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
	defer func() {
		_, _ = lock.Exec(context.Background(), "select pg_advisory_unlock(7167614658367249410)")
		lock.Release()
		admin.Close()
	}()
	if err := migrate.Apply(context.Background(), admin, migrate.Builtin()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "alter role ai_native_control login"); err != nil {
		t.Fatalf("enable control login: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "alter role ai_native_runtime login"); err != nil {
		t.Fatalf("enable runtime login: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "truncate tenant_registry cascade"); err != nil {
		t.Fatalf("reset application data: %v", err)
	}

	const identityKey = "0123456789abcdef0123456789abcdef"
	claims := jwt.MapClaims{
		"iss": "https://identity.example.test", "aud": "native-platform", "sub": "agent-main",
		"iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		"tenant_id": "11111111-1111-4111-8111-111111111111", "company_id": "orgaaaaaaaaaaaaaaaaa",
		"scopes": []string{"metadata.version.write"},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(identityKey))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	t.Setenv("AI_NATIVE_DATABASE_URL", "")
	t.Setenv("AI_NATIVE_CONTROL_DATABASE_URL", databaseURLForUser(t, databaseURL, "ai_native_control"))
	t.Setenv("AI_NATIVE_RUNTIME_DATABASE_URL", databaseURLForUser(t, databaseURL, "ai_native_runtime"))
	t.Setenv("AI_NATIVE_IDENTITY_ISSUER", "https://identity.example.test")
	t.Setenv("AI_NATIVE_IDENTITY_AUDIENCE", "native-platform")
	t.Setenv("AI_NATIVE_IDENTITY_ALGORITHM", "HS256")
	t.Setenv("AI_NATIVE_IDENTITY_HMAC_KEY", identityKey)
	t.Setenv("AI_NATIVE_IDENTITY_TOKEN", token)

	// Public tenant.provision is deliberately unpublished. This fixture creates
	// an already verified company projection so the test can exercise the
	// separate control/runtime database role wiring without reopening a public
	// provisioning route.
	if _, err := admin.Exec(context.Background(),
		"insert into tenant_registry(tenant_id,company_id,display_name,shard_id,tenant_bucket,service_tier,global_lifecycle_status,native_status,tenant_revision,product_revision,route_revision) values ($1,$2,$3,'shard-001',$4,'standard','active','active',1,1,1)",
		"11111111-1111-4111-8111-111111111111", "orgaaaaaaaaaaaaaaaaa", "Main Tenant", 17,
	); err != nil {
		t.Fatalf("create verified tenant fixture: %v", err)
	}

	versionBody := `{"request_id":"req-main-version","input":{}}`
	var versionOutput bytes.Buffer
	if code := run(context.Background(), []string{"capability", "invoke", "--id", "metadata.version.create"}, strings.NewReader(versionBody), &versionOutput, io.Discard); code != 0 {
		t.Fatalf("metadata.version.create exit=%d output=%s", code, versionOutput.String())
	}
	var versionResponse map[string]any
	if err := json.Unmarshal(versionOutput.Bytes(), &versionResponse); err != nil || versionResponse["status"] != "succeeded" {
		t.Fatalf("metadata.version.create response=%#v err=%v", versionResponse, err)
	}
	var listOutput bytes.Buffer
	if code := run(context.Background(), []string{"capability", "list"}, strings.NewReader(""), &listOutput, io.Discard); code != 0 {
		t.Fatalf("capability list exit=%d output=%s", code, listOutput.String())
	}
	var listed struct {
		Capabilities []struct {
			ID string `json:"id"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(listOutput.Bytes(), &listed); err != nil {
		t.Fatalf("decode capability list: %v", err)
	}
	found := map[string]bool{}
	for _, descriptor := range listed.Capabilities {
		found[descriptor.ID] = true
	}
	if found["tenant.provision"] {
		t.Fatalf("main wiring must not publish tenant.provision: %s", listOutput.String())
	}
	if len(listed.Capabilities) != 56 {
		t.Fatalf("main wiring published %d capabilities, want 56: %s", len(listed.Capabilities), listOutput.String())
	}
	for _, required := range []string{
		"metadata.changeset.backfill", "metadata.changeset.validate-coverage", "metadata.changeset.purge",
		"runtime.record.create", "runtime.record.get", "runtime.record.update", "runtime.record.delete", "runtime.record.query",
		"authorization.role.create", "authorization.role.assign", "authorization.role.revoke", "authorization.role.set-data-scope", "authorization.role.set-conflict", "authorization.object-policy.set", "authorization.access.explain", "authorization.group.create", "record.team.add-member", "record.share.grant", "record.sharing-rule.upsert", "record.sharing-rule.refresh", "record.sharing-rule.retry", "organization.merge.start", "organization.merge.execute", "organization.merge.cancel",
		"identity.principal.sync", "identity.principal.list", "identity.principal.set-status", "identity.principal.set-organization-membership",
	} {
		if !found[required] {
			t.Fatalf("main wiring omitted %s: %s", required, listOutput.String())
		}
	}
}

func databaseURLForUser(t *testing.T, rawURL, user string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	parsed.User = url.User(user)
	return parsed.String()
}

func writeMessage(t *testing.T, stdin io.Writer, message string) {
	t.Helper()
	if _, err := fmt.Fprintln(stdin, message); err != nil {
		t.Fatalf("write protocol message: %v", err)
	}
}

func nextJSONRPCLine(t *testing.T, scanner *bufio.Scanner) string {
	t.Helper()
	if !scanner.Scan() {
		t.Fatalf("MCP stdout closed before response; err=%v", scanner.Err())
	}
	return scanner.Text()
}

func buildBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "ai-native-platform")
	command := exec.Command("go", "build", "-o", binary, ".")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.5")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build binary: %v\n%s", err, output)
	}
	return binary
}
