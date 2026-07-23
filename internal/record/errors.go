package record

import (
	"errors"

	"github.com/OlivierZEN/ai-native-platform/internal/authorization"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

func validationError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeValidationFailed, Message: message}
}

func conflictError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeConflict, Message: message}
}

func idempotencyConflictError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeIdempotencyConflict, Message: message}
}

func preconditionError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeFailedPrecondition, Message: message}
}

func notFoundError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeResourceNotFound, Message: message}
}

func unauthorizedError(message string) *capability.StableError {
	return &capability.StableError{Code: capability.CodeUnauthorized, Message: message}
}

func mapAuthorizationError(err error) error {
	switch {
	case errors.Is(err, authorization.ErrDenied):
		return unauthorizedError("actor lacks the required object, field, or record permission")
	case errors.Is(err, authorization.ErrNoPrimaryOrganization):
		return preconditionError("actor needs an active primary organization before creating a protected record")
	default:
		return err
	}
}

func internalError() *capability.StableError {
	return &capability.StableError{Code: capability.CodeInternal, Message: "record operation failed"}
}
