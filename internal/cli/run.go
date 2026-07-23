package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

func Run(ctx context.Context, invoker *capability.Invoker, args []string, in io.Reader, out, _ io.Writer) int {
	return run(ctx, invoker, nil, args, in, out)
}

func RunAs(ctx context.Context, invoker *capability.Invoker, principal capability.TrustedPrincipal, args []string, in io.Reader, out, _ io.Writer) int {
	return run(ctx, invoker, &principal, args, in, out)
}

func run(ctx context.Context, invoker *capability.Invoker, principal *capability.TrustedPrincipal, args []string, in io.Reader, out io.Writer) int {
	if len(args) == 2 && args[0] == "capability" && args[1] == "list" {
		return write(out, capability.ListResult{Capabilities: invoker.RegistryDescriptors()})
	}
	if len(args) == 4 && args[0] == "capability" && args[1] == "describe" && args[2] == "--id" && args[3] != "" {
		for _, descriptor := range invoker.RegistryDescriptors() {
			if descriptor.ID == args[3] {
				return write(out, descriptor)
			}
		}
		return writeFailure(out, capability.Response{CapabilityID: args[3], Status: capability.StatusFailed, Error: &capability.StableError{Code: capability.CodeCapabilityNotFound, Message: "capability is not published"}})
	}
	if len(args) != 4 || args[0] != "capability" || args[1] != "invoke" || args[2] != "--id" || args[3] == "" {
		return writeFailure(out, capability.Response{Status: capability.StatusFailed, Error: &capability.StableError{Code: capability.CodeValidationFailed, Message: "expected: capability list | capability describe --id <capability-id> | capability invoke --id <capability-id>"}})
	}

	var request capability.Request
	decoder := json.NewDecoder(in)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return writeFailure(out, capability.Response{CapabilityID: args[3], Status: capability.StatusFailed, Error: &capability.StableError{Code: capability.CodeValidationFailed, Message: fmt.Sprintf("invalid JSON input: %v", err)}})
	}
	if err := requireEndOfJSON(decoder); err != nil {
		return writeFailure(out, capability.Response{CapabilityID: args[3], Status: capability.StatusFailed, Error: &capability.StableError{Code: capability.CodeValidationFailed, Message: fmt.Sprintf("invalid JSON input: %v", err)}})
	}
	if request.CapabilityID == "" {
		request.CapabilityID = args[3]
	}
	if request.CapabilityID != args[3] {
		return writeFailure(out, capability.Response{CapabilityID: args[3], RequestID: request.RequestID, AuditID: "audit:" + request.RequestID, Status: capability.StatusFailed, Error: &capability.StableError{Code: capability.CodeValidationFailed, Message: "capability_id must match --id"}})
	}
	if principal != nil {
		bound, stableErr := capability.BindTrustedPrincipal(request, *principal)
		if stableErr != nil {
			return writeFailure(out, capability.Response{CapabilityID: args[3], RequestID: request.RequestID, AuditID: "audit:" + request.RequestID, Status: capability.StatusFailed, Error: stableErr})
		}
		request = bound
	}
	request.Entrypoint = "cli"
	response := invoker.Invoke(ctx, request)
	if exitCode := write(out, response); exitCode != 0 {
		return exitCode
	}
	if response.Status == capability.StatusFailed {
		return 1
	}
	return 0
}

func writeFailure(out io.Writer, response capability.Response) int {
	if exitCode := write(out, response); exitCode != 0 {
		return exitCode
	}
	return 1
}

func requireEndOfJSON(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("input must contain exactly one JSON value")
}

func write(out io.Writer, value any) int {
	if err := json.NewEncoder(out).Encode(value); err != nil {
		return 2
	}
	return 0
}
