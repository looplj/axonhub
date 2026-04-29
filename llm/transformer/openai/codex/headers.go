package codex

import (
	"encoding/json"
	"net/http"
	"strings"
)

const (
	SessionHeader         = "Session_id"
	TurnMetadataHeader    = "X-Codex-Turn-Metadata"
	WindowIDHeader        = "X-Codex-Window-Id"
	ClientRequestIDHeader = "X-Client-Request-Id"
	BetaFeaturesHeader    = "X-Codex-Beta-Features"
)

type TurnMetadata struct {
	SessionID string `json:"session_id"`
}

var PassthroughHeaders = []string{
	TurnMetadataHeader,
	WindowIDHeader,
	ClientRequestIDHeader,
	BetaFeaturesHeader,
}

func ExtractSessionIDFromTurnMetadata(raw string) string {
	if raw == "" {
		return ""
	}

	var payload TurnMetadata
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}

	return strings.TrimSpace(payload.SessionID)
}

func GetSessionIDFromHeaders(headers http.Header) string {
	if headers == nil {
		return ""
	}

	sessionID := strings.TrimSpace(headers.Get(SessionHeader))
	if sessionID != "" {
		return sessionID
	}

	return ExtractSessionIDFromTurnMetadata(strings.TrimSpace(headers.Get(TurnMetadataHeader)))
}
