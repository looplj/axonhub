package cacheidentity

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/llm"
)

const (
	anchorVersion   = "ah:anchor:v1"
	defaultMaxBytes = 32768
)

// DeriveAnchor computes a deterministic conversation-anchor digest from
// the leading stable portion of the message list.
//
// The stable prefix is defined as: all contiguous system/developer messages
// followed by the first user message. The hash stops before the first
// assistant, tool-call, or tool-result message.
//
// The hash is namespaced by project and API key to prevent cross-tenant
// collisions.
func DeriveAnchor(messages []llm.Message, projectID int, apiKeyID int, maxBytes int) string {
	if len(messages) == 0 {
		return ""
	}

	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	// Extract leading stable prefix.
	var parts []string
	seenUser := false

	for _, msg := range messages {
		role := strings.ToLower(msg.Role)

		switch role {
		case "system", "developer":
			if seenUser {
				// system/developer after first user → stop
				goto done
			}

			parts = append(parts, canonicalizeMessage(msg))
		case "user":
			if seenUser {
				// second user message → stop
				goto done
			}

			parts = append(parts, canonicalizeMessage(msg))
			seenUser = true
		default:
			// assistant, tool, function, etc. → stop
			goto done
		}
	}

done:
	if len(parts) == 0 {
		return ""
	}

	canonical := strings.Join(parts, "\n---\n")

	// Apply byte limit.
	if len(canonical) > maxBytes {
		canonical = canonical[:maxBytes]
	}

	// Build namespaced hash input.
	input := fmt.Sprintf("%s:%d:%d:%s", anchorVersion, projectID, apiKeyID, canonical)

	hash := sha256.Sum256([]byte(input))

	return fmt.Sprintf("%s:%x", anchorVersion, hash)
}

// canonicalizeMessage produces a stable string representation of a message
// for hashing. It normalizes whitespace (CRLF → LF) and emits stable
// descriptors for binary/inline parts so multimodal identity is preserved.
func canonicalizeMessage(msg llm.Message) string {
	var sb strings.Builder
	sb.WriteString(msg.Role)
	sb.WriteByte(':')

	// Plain text content.
	if msg.Content.Content != nil {
		text := normalizeLineEndings(*msg.Content.Content)
		sb.WriteString(text)

		return sb.String()
	}

	// Multi-part content.
	for i, part := range msg.Content.MultipleContent {
		if i > 0 {
			sb.WriteByte('\n')
		}

		switch part.Type {
		case "text":
			if part.Text != nil {
				sb.WriteString(normalizeLineEndings(*part.Text))
			}
		case "image_url":
			if part.ImageURL != nil {
				sb.WriteString("[image_url:")
				sb.WriteString(canonicalizeURL(part.ImageURL.URL))
				sb.WriteByte(']')
			}
		case "video_url":
			if part.VideoURL != nil {
				sb.WriteString("[video_url:")
				sb.WriteString(canonicalizeURL(part.VideoURL.URL))
				sb.WriteByte(']')
			}
		case "document":
			if part.Document != nil {
				sb.WriteString("[document:")
				sb.WriteString(part.Document.MIMEType)
				sb.WriteByte(':')
				sb.WriteString(canonicalizeURL(part.Document.URL))
				sb.WriteByte(']')
			}
		case "input_audio":
			if part.InputAudio != nil {
				// Compute a stable digest of the audio data so two
				// requests with different audio payloads but the same
				// text produce distinct anchors.
				sb.WriteString("[input_audio:")
				sb.WriteString(part.InputAudio.Format)
				sb.WriteByte(':')
				sb.WriteString(hashInlineData(part.InputAudio.Data))
				sb.WriteByte(']')
			}
		case "compaction", "compaction_summary":
			// Skip entirely — these are encrypted/ephemeral.
			continue
		default:
			sb.WriteString("[")
			sb.WriteString(part.Type)
			sb.WriteString("]")
		}
	}

	return sb.String()
}

// canonicalizeURL produces a stable identifier for a URL. For remote URLs
// the URL itself is the stable identity. For data: URLs (inline base64),
// the binary content is hashed to produce a stable, short descriptor
// that distinguishes different inline payloads.
func canonicalizeURL(u string) string {
	if !strings.HasPrefix(u, "data:") {
		// Remote URL — already a stable external identifier.
		return u
	}

	// data:<mediatype>;base64,<data>
	// Hash the binary content for a stable, comparable descriptor.
	idx := strings.Index(u, ",")
	if idx < 0 {
		return hashInlineData(u)
	}

	mediaType := u[5:idx] // between "data:" and ","
	b64Data := u[idx+1:]

	return fmt.Sprintf("%s:%s", strings.TrimSuffix(mediaType, ";base64"), hashInlineData(b64Data))
}

// hashInlineData computes a truncated SHA-256 hex digest of base64-encoded
// binary data. It decodes first to produce a stable hash regardless of
// whitespace or line-break differences in the base64 encoding.
func hashInlineData(b64 string) string {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		// Fallback: hash the raw base64 string if decode fails.
		h := sha256.Sum256([]byte(b64))
		return fmt.Sprintf("sha256:%x", h[:16])
	}

	h := sha256.Sum256(decoded)

	return fmt.Sprintf("sha256:%x", h[:16])
}

// normalizeLineEndings replaces CRLF with LF for stable hashing.
func normalizeLineEndings(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
