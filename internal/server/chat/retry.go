package chat

import (
	"errors"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	return httpclient.IsHTTPStatusCodeRetryable(ExtractStatusCodeFromError(err))
}

// ExtractStatusCodeFromError attempts to extract HTTP status code from various error types.
func ExtractStatusCodeFromError(err error) int {
	if err == nil {
		return 0
	}

	// Try to extract from httpclient.Error
	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}

	return 0
}
