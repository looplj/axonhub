package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/objects"
)

// PublicError is an error whose message is safe to expose to API clients.
// Errors that implement this interface will have their PublicMessage() used
// in JSON responses instead of the raw Error() string.
type PublicError interface {
	PublicMessage() string
}

// NewPublicError wraps an error message as a PublicError.
func NewPublicError(msg string) error {
	return publicError(msg)
}

type publicError string

func (e publicError) Error() string       { return string(e) }
func (e publicError) PublicMessage() string { return string(e) }

// JSONError returns a JSON error response and adds the error to gin context for access logging.
// For 5xx (server) errors, the internal error message is never exposed directly.
// Instead, we check whether the error (or any wrapped error) implements PublicError.
// If not, a generic "internal server error" message is returned to avoid leaking
// stack traces, file paths, or internal state.
// For 4xx (client) errors, the message is preserved as these are typically
// validation or user-input errors that are safe to return.
func JSONError(c *gin.Context, status int, err error) {
	_ = c.Error(err)

	msg := publicErrorMessage(status, err)

	c.JSON(status, objects.ErrorResponse{
		Error: objects.Error{
			Type:    http.StatusText(status),
			Message: msg,
		},
	})
}

// publicErrorMessage extracts a safe error message for API responses.
func publicErrorMessage(status int, err error) string {
	if err == nil {
		return ""
	}

	// Check if any error in the chain implements PublicError.
	var pubErr PublicError
	if errors.As(err, &pubErr) {
		return pubErr.PublicMessage()
	}

	// 4xx client errors: preserve the message (usually validation / input errors).
	if status < http.StatusInternalServerError {
		return err.Error()
	}

	// 5xx server errors: never expose raw internal messages.
	return "internal server error"
}
