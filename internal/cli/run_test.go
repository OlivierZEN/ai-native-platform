package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

func TestDescribeReturnsOnlyPublishedCapabilityMetadata(t *testing.T) {
	invoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 1)
	var stdout, stderr bytes.Buffer
	exitCode := Run(context.Background(), invoker, []string{"capability", "describe", "--id", "system.capability.list"}, nil, &stdout, &stderr)
	if exitCode != 0 || stderr.Len() != 0 {
		t.Fatalf("describe exit = %d stdout = %q stderr = %q", exitCode, stdout.String(), stderr.String())
	}
	var descriptor capability.Descriptor
	if err := json.Unmarshal(stdout.Bytes(), &descriptor); err != nil {
		t.Fatalf("describe stdout is not descriptor JSON: %v", err)
	}
	if descriptor.ID != "system.capability.list" || descriptor.State != capability.PublicationPublished || descriptor.RequiredScope != "system.capability.read" {
		t.Fatalf("unexpected published descriptor: %#v", descriptor)
	}
}

func TestDescribeReturnsStructuredNotFoundError(t *testing.T) {
	invoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 1)
	var stdout bytes.Buffer
	exitCode := Run(context.Background(), invoker, []string{"capability", "describe", "--id", "system.not-published"}, nil, &stdout, &bytes.Buffer{})
	if exitCode == 0 {
		t.Fatal("describe exit = 0, want non-zero")
	}
	var response capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("describe stdout is not structured JSON: %v", err)
	}
	if response.Error == nil || response.Error.Code != capability.CodeCapabilityNotFound {
		t.Fatalf("describe response = %#v", response)
	}
}

func TestInvokeRejectsMultipleJSONDocuments(t *testing.T) {
	invoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 1)
	request := capability.Request{
		CapabilityID: "system.capability.list",
		RequestID:    "req-cli-multiple-json",
		TenantID:     "tenant-poc",
		Actor:        capability.Actor{ID: "agent-poc", Scopes: []string{"system.capability.read"}},
		Input:        json.RawMessage(`{}`),
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	exitCode := Run(context.Background(), invoker, []string{"capability", "invoke", "--id", request.CapabilityID}, bytes.NewReader(append(body, []byte("\n{}")...)), &stdout, &bytes.Buffer{})
	if exitCode == 0 {
		t.Fatal("invoke exit = 0, want non-zero")
	}
	var response capability.Response
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode CLI response: %v", err)
	}
	if response.Error == nil || response.Error.Code != capability.CodeValidationFailed {
		t.Fatalf("response = %#v, want validation failure", response)
	}
}
