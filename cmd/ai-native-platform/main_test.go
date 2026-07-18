package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "mcp", "stdio")
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

	scanner := bufio.NewScanner(stdout)
	writeMessage(t, stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"stdout-test","version":"v1"}}}`)
	initializeLine := nextJSONRPCLine(t, scanner)
	writeMessage(t, stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	writeMessage(t, stdin, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"system_capability_list","arguments":{"request_id":"req-child-mcp","tenant_id":"tenant-poc","actor_id":"agent-poc","scopes":["system.capability.read"],"input":{}}}}`)
	toolLine := nextJSONRPCLine(t, scanner)
	_ = stdin.Close()
	_ = command.Wait()

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
