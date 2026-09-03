package opencode

import (
	"net/http"
	"strings"
)

const (
	// SessionHeader is the current OpenCode conversation identifier header.
	SessionHeader = "X-Opencode-Session"
	// SessionIDHeader is used by OpenCode clients that expose their session ID directly.
	SessionIDHeader = "X-Session-Id"
	// SessionAffinityHeader is the legacy OpenCode session affinity header.
	SessionAffinityHeader = "X-Session-Affinity"
)

// GetSessionIDFromHeaders returns the first non-empty OpenCode session identifier.
func GetSessionIDFromHeaders(headers http.Header) string {
	for _, header := range []string{SessionHeader, SessionIDHeader, SessionAffinityHeader} {
		if sessionID := strings.TrimSpace(headers.Get(header)); sessionID != "" {
			return sessionID
		}
	}

	return ""
}
