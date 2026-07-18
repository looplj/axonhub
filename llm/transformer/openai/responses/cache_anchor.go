package responses

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/looplj/axonhub/llm"
)

// anchorMaxBytes bounds how much of the conversation head is hashed.
const anchorMaxBytes = 8192

// conversationAnchor derives a stable fingerprint for the conversation a
// request belongs to: the contiguous leading system/developer messages plus
// the first user message. Later turns of the same conversation keep this head
// unchanged, while sibling conversations multiplexed over one client session
// (e.g. Claude Code subagents sharing a session_id) diverge in their first
// user message. Combining this anchor with the session ID therefore yields a
// per-conversation prompt_cache_key instead of a per-session one.
func conversationAnchor(messages []llm.Message) string {
	h := sha256.New()
	written := 0

	write := func(s string) {
		if written >= anchorMaxBytes {
			return
		}

		if remaining := anchorMaxBytes - written; len(s) > remaining {
			s = s[:remaining]
		}

		h.Write([]byte(s))
		written += len(s)
	}

	hashed := false

	for _, msg := range messages {
		if msg.Role != "system" && msg.Role != "developer" && msg.Role != "user" {
			break
		}

		write(msg.Role)
		write("\x00")

		if msg.Content.Content != nil {
			write(*msg.Content.Content)
		}

		for _, part := range msg.Content.MultipleContent {
			if part.Text != nil {
				write(*part.Text)
			}
		}

		write("\x1e")

		hashed = true

		// The first user message ends the stable conversation head.
		if msg.Role == "user" || written >= anchorMaxBytes {
			break
		}
	}

	if !hashed {
		return ""
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}
