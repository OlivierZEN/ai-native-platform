package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

func NewHandler(invoker *capability.Invoker) http.Handler {
	return newHandler(invoker, nil)
}

type IdentityVerifier interface {
	Verify(context.Context, string) (capability.TrustedPrincipal, error)
}

func NewAuthenticatedHandler(invoker *capability.Invoker, verifier IdentityVerifier) http.Handler {
	if verifier == nil {
		panic("authenticated API handler requires an identity verifier")
	}
	return newHandler(invoker, verifier)
}

func newHandler(invoker *capability.Invoker, verifier IdentityVerifier) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		capabilityID, ok := invokePath(request.URL.Path)
		if request.Method != http.MethodPost || !ok {
			writeJSON(writer, http.StatusNotFound, capability.Response{Status: capability.StatusFailed, Error: &capability.StableError{Code: capability.CodeCapabilityNotFound, Message: "route not found"}})
			return
		}

		var principal capability.TrustedPrincipal
		if verifier != nil {
			token, ok := bearerToken(request.Header.Get("Authorization"))
			if !ok {
				writeJSON(writer, http.StatusUnauthorized, capability.Response{CapabilityID: capabilityID, Status: capability.StatusFailed, Error: &capability.StableError{Code: capability.CodeUnauthenticated, Message: "valid bearer authentication is required"}})
				return
			}
			var err error
			principal, err = verifier.Verify(request.Context(), token)
			if err != nil {
				writeJSON(writer, http.StatusUnauthorized, capability.Response{CapabilityID: capabilityID, Status: capability.StatusFailed, Error: &capability.StableError{Code: capability.CodeUnauthenticated, Message: "valid bearer authentication is required"}})
				return
			}
		}
		invocation, err := decodeInvocation(writer, request.Context(), request, capabilityID)
		if err != nil {
			writeJSON(writer, http.StatusBadRequest, capability.Response{CapabilityID: capabilityID, Status: capability.StatusFailed, Error: &capability.StableError{Code: capability.CodeValidationFailed, Message: err.Error()}})
			return
		}
		if verifier != nil {
			bound, stableErr := capability.BindTrustedPrincipal(invocation, principal)
			if stableErr != nil {
				writeJSON(writer, httpStatus(capability.Response{Status: capability.StatusFailed, Error: stableErr}), capability.Response{CapabilityID: capabilityID, RequestID: invocation.RequestID, Status: capability.StatusFailed, Error: stableErr})
				return
			}
			invocation = bound
		}
		invocation.Entrypoint = "api"
		result := invoker.Invoke(request.Context(), invocation)
		writeJSON(writer, httpStatus(result), result)
	})
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	returnValue := ""
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] != "" {
		returnValue = parts[1]
	}
	return returnValue, returnValue != ""
}

func invokePath(path string) (string, bool) {
	const prefix = "/v1/capabilities/"
	const suffix = "/invoke"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return id, id != "" && !strings.Contains(id, "/")
}

func decodeInvocation(writer http.ResponseWriter, _ context.Context, request *http.Request, capabilityID string) (capability.Request, error) {
	defer request.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var invocation capability.Request
	if err := decoder.Decode(&invocation); err != nil {
		return capability.Request{}, err
	}
	if err := requireEndOfJSON(decoder); err != nil {
		return capability.Request{}, err
	}
	if invocation.CapabilityID == "" {
		invocation.CapabilityID = capabilityID
	}
	if invocation.CapabilityID != capabilityID {
		return capability.Request{}, &requestError{"capability_id must match the URL"}
	}
	return invocation, nil
}

func requireEndOfJSON(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("request body must contain exactly one JSON value")
}

type requestError struct{ message string }

func (e *requestError) Error() string { return e.message }

func httpStatus(response capability.Response) int {
	if response.Status == capability.StatusSucceeded || response.Error == nil {
		return http.StatusOK
	}
	switch response.Error.Code {
	case capability.CodeUnauthenticated:
		return http.StatusUnauthorized
	case capability.CodeValidationFailed:
		return http.StatusBadRequest
	case capability.CodeCapabilityNotFound, capability.CodeResourceNotFound:
		return http.StatusNotFound
	case capability.CodeUnauthorized:
		return http.StatusForbidden
	case capability.CodeIdempotencyConflict, capability.CodeConflict:
		return http.StatusConflict
	case capability.CodeFailedPrecondition:
		return http.StatusPreconditionFailed
	case capability.CodeOverloaded:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
