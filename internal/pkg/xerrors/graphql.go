package xerrors

import (
	"errors"
	"fmt"
	"maps"
)

// ErrorCode represents application error codes
// These codes can be mapped to GraphQL extensions.code.
type ErrorCode string

const (
	// ErrCodeInvalidInput indicates a generic invalid input.
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	// ErrCodeValidationFailed indicates validation failure.
	ErrCodeValidationFailed ErrorCode = "VALIDATION_FAILED"

	// ErrCodeDuplicateName indicates duplicate name conflict.
	ErrCodeDuplicateName ErrorCode = "DUPLICATE_NAME"
	// ErrCodeAlreadyExists indicates resource already exists.
	ErrCodeAlreadyExists ErrorCode = "ALREADY_EXISTS"

	// ErrCodeNotFound indicates requested resource was not found.
	ErrCodeNotFound ErrorCode = "NOT_FOUND"

	// ErrCodeUnauthenticated indicates authentication is required.
	ErrCodeUnauthenticated ErrorCode = "UNAUTHENTICATED"
	// ErrCodeForbidden indicates insufficient permissions.
	ErrCodeForbidden ErrorCode = "FORBIDDEN"

	// ErrCodeInternalServerError indicates an unexpected server error.
	ErrCodeInternalServerError ErrorCode = "INTERNAL_SERVER_ERROR"
)

// CodedError is an error with a machine-readable code.
type CodedError struct {
	Code       ErrorCode
	Message    string
	Extensions map[string]any
	Cause      error
}

// Error implements the error interface.
func (e *CodedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}

	return e.Message
}

// Unwrap returns the underlying error.
func (e *CodedError) Unwrap() error {
	return e.Cause
}

// IsCodedError checks if an error is a CodedError.
func IsCodedError(err error) (*CodedError, bool) {
	var codedErr *CodedError
	if errors.As(err, &codedErr) {
		return codedErr, true
	}

	return nil, false
}

// NewCodedError creates a new coded error.
func NewCodedError(code ErrorCode, message string) *CodedError {
	return &CodedError{
		Code:       code,
		Message:    message,
		Extensions: make(map[string]any),
	}
}

// NewCodedErrorWithExtensions creates a coded error with extensions.
func NewCodedErrorWithExtensions(code ErrorCode, message string, extensions map[string]any) *CodedError {
	ext := make(map[string]any)
	maps.Copy(ext, extensions)

	return &CodedError{
		Code:       code,
		Message:    message,
		Extensions: ext,
	}
}

// WithCause adds a cause error.
func (e *CodedError) WithCause(err error) *CodedError {
	e.Cause = err
	return e
}

// WithExtension adds a single extension.
func (e *CodedError) WithExtension(key string, value any) *CodedError {
	e.Extensions[key] = value
	return e
}

// Common error constructors

// DuplicateNameError creates an error for duplicate name conflicts.
func DuplicateNameError(resource, name string) *CodedError {
	return NewCodedErrorWithExtensions(
		ErrCodeDuplicateName,
		fmt.Sprintf("%s name '%s' already exists", resource, name),
		map[string]any{
			"resource": resource,
			"field":    "name",
			"value":    name,
		},
	)
}

// AlreadyExistsError creates a generic already exists error.
func AlreadyExistsError(resource string) *CodedError {
	return NewCodedErrorWithExtensions(
		ErrCodeAlreadyExists,
		fmt.Sprintf("%s already exists", resource),
		map[string]any{
			"resource": resource,
		},
	)
}

// NotFoundError creates a not found error.
func NotFoundError(resource string) *CodedError {
	return NewCodedErrorWithExtensions(
		ErrCodeNotFound,
		fmt.Sprintf("%s not found", resource),
		map[string]any{
			"resource": resource,
		},
	)
}

// ValidationError creates a validation error.
func ValidationError(message string) *CodedError {
	return NewCodedError(ErrCodeValidationFailed, message)
}

// UnauthorizedError creates an unauthenticated error.
func UnauthorizedError(message string) *CodedError {
	return NewCodedError(ErrCodeUnauthenticated, message)
}

// ForbiddenError creates a forbidden error.
func ForbiddenError(message string) *CodedError {
	return NewCodedError(ErrCodeForbidden, message)
}
