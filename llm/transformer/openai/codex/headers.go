package codex

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const (
	SessionHeader         = "Session_id"
	SessionHeaderHyphen   = "Session-Id"
	TurnMetadataHeader    = "X-Codex-Turn-Metadata"
	TurnStateHeader       = "X-Codex-Turn-State"
	WindowIDHeader        = "X-Codex-Window-Id"
	ClientRequestIDHeader = "X-Client-Request-Id"
	BetaFeaturesHeader    = "X-Codex-Beta-Features"
	ThreadIDHeader        = "Thread-Id"
	// ResponsesLiteHeader uses the canonical spelling ("Openai"): Go's
	// http.Header canonicalizes keys, so lookups match regardless of case, and
	// the wire name is case-insensitive per RFC 9110.
	ResponsesLiteHeader = "X-Openai-Internal-Codex-Responses-Lite"
)

type TurnMetadata struct {
	InstallationID      string         `json:"installation_id"`
	SessionID           string         `json:"session_id"`
	ThreadID            string         `json:"thread_id"`
	TurnID              string         `json:"turn_id"`
	WindowID            string         `json:"window_id"`
	RequestKind         string         `json:"request_kind"`
	ThreadSource        string         `json:"thread_source"`
	Sandbox             string         `json:"sandbox"`
	TurnStartedAtUnixMS int64          `json:"turn_started_at_unix_ms"`
	Workspaces          map[string]any `json:"workspaces,omitempty"`
}

// PassthroughHeaders lists client metadata that Codex-compatible upstreams use
// to select protocol behavior. Headers a Codex client actually sends are copied
// verbatim; headers missing from non-Codex inbound requests are fabricated in
// OutboundTransformer.TransformRequest so the upstream always sees a complete
// Codex session shape.
var PassthroughHeaders = []string{
	TurnMetadataHeader,
	TurnStateHeader,
	WindowIDHeader,
	ClientRequestIDHeader,
	BetaFeaturesHeader,
	ThreadIDHeader,
	ResponsesLiteHeader,
}

// clientFingerprintHeaders lists client-private attribution headers that no
// Codex upstream consumes. They are exact-name matches.
//
// Http-Referer is the non-standard spelling of Referer used by several coding
// clients (the canonical Referer is already blocked globally in httpclient).
// X-Title and X-Platform are OpenRouter-style attribution headers: legitimate on
// the channels that consume them, but pure client fingerprint on the ChatGPT
// Codex backend, which is why they are scrubbed here rather than blocked
// globally for every channel.
var clientFingerprintHeaders = []string{
	"Http-Referer",
	"X-Title",
	"X-Platform",
}

// clientFingerprintHeaderPrefixes lists client-private header families that no
// Codex upstream consumes.
var clientFingerprintHeaderPrefixes = []string{
	"X-Zcode-",
	"X-Os-",
}

// ScrubClientFingerprintHeaders deletes client-private attribution headers from
// an inbound request so that the later MergeInboundRequest step cannot copy them
// onto the outbound request and hand the upstream the client's full fingerprint.
//
// This deliberately mirrors the existing Session_id handling in
// OutboundTransformer.TransformRequest: MergeInboundRequest runs after the
// transformer, so the only way for a transformer to keep a header off the wire
// is to remove it from the inbound request it was given.
//
// Header names are canonicalized by http.Header, so exact-name lookups match
// regardless of the case the client used.
func ScrubClientFingerprintHeaders(headers http.Header) {
	if headers == nil {
		return
	}

	for _, name := range clientFingerprintHeaders {
		headers.Del(name)
	}

	// Deleting during range is safe in Go.
	for name := range headers {
		for _, prefix := range clientFingerprintHeaderPrefixes {
			if strings.HasPrefix(name, prefix) {
				headers.Del(name)

				break
			}
		}
	}
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

// turnStartedAtUnixMS returns a deterministic per-session timestamp inside the
// current 10-minute bucket: the bucket start plus an offset derived from the
// session id. Fabricated turn metadata is therefore stable across requests of
// the same session and rolls over roughly every 10 minutes.
func turnStartedAtUnixMS(sessionID string) int64 {
	const bucketMS = int64(10 * time.Minute / time.Millisecond)
	now := time.Now().UnixMilli()
	return now - now%bucketMS + int64(hashUint64(sessionID)%uint64(bucketMS))
}

func hashUint64(value string) uint64 {
	if value == "" {
		return 0
	}

	sum := sha256.Sum256([]byte(value))
	return binary.BigEndian.Uint64(sum[:8])
}

func GetSessionIDFromHeaders(headers http.Header) string {
	if headers == nil {
		return ""
	}

	sessionID := strings.TrimSpace(headers.Get(SessionHeader))
	if sessionID == "" {
		sessionID = strings.TrimSpace(headers.Get(SessionHeaderHyphen))
	}
	if sessionID != "" {
		return sessionID
	}

	return ExtractSessionIDFromTurnMetadata(strings.TrimSpace(headers.Get(TurnMetadataHeader)))
}
