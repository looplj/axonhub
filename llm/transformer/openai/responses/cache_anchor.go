package responses

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"github.com/looplj/axonhub/llm"
)

// anchorMaxBytesPerUnit bounds how much each content unit (a message's plain
// string content, or a single field of a content part) contributes to the
// hash. The budget is per unit rather than global so that a large instruction
// prefix can never starve the first user message out of the fingerprint.
const anchorMaxBytesPerUnit = 4096

// conversationAnchor derives a stable fingerprint for the conversation a
// request belongs to: the contiguous leading system/developer messages plus
// the first user message. Later turns of the same conversation keep this head
// unchanged, while sibling conversations multiplexed over one client session
// (e.g. Claude Code subagents sharing a session_id) diverge in their first
// user message. Combining this anchor with the session ID therefore yields a
// per-conversation prompt_cache_key instead of a per-session one.
func conversationAnchor(messages []llm.Message) string {
	h := sha256.New()

	// Each unit is hashed as <len>:<capped bytes>. The length prefix keeps
	// concatenated units unambiguous and still distinguishes oversized units
	// (e.g. two base64 images sharing their first 4 KiB) by their size.
	writeUnit := func(s string) {
		h.Write([]byte(strconv.Itoa(len(s))))
		h.Write([]byte{':'})

		if len(s) > anchorMaxBytesPerUnit {
			s = s[:anchorMaxBytesPerUnit]
		}

		h.Write([]byte(s))
	}

	hashed := false

	for _, msg := range messages {
		if msg.Role != "system" && msg.Role != "developer" && msg.Role != "user" {
			break
		}

		h.Write([]byte(msg.Role))
		h.Write([]byte{0x00})

		if msg.Content.Content != nil {
			writeUnit(*msg.Content.Content)
		}

		for _, part := range msg.Content.MultipleContent {
			h.Write([]byte(part.Type))
			h.Write([]byte{0x1f})

			switch {
			case part.Text != nil:
				writeUnit(*part.Text)
			case part.ImageURL != nil:
				writeUnit(part.ImageURL.URL)
			case part.VideoURL != nil:
				writeUnit(part.VideoURL.URL)
			case part.Document != nil:
				writeUnit(part.Document.MIMEType)
				writeUnit(part.Document.URL)
			case part.InputAudio != nil:
				writeUnit(part.InputAudio.Format)
				writeUnit(part.InputAudio.Data)
			case part.Compact != nil:
				writeUnit(part.Compact.ID)
				writeUnit(part.Compact.EncryptedContent)
			}

			h.Write([]byte{0x1f})
		}

		h.Write([]byte{0x1e})

		hashed = true

		// The first user message ends the stable conversation head.
		if msg.Role == "user" {
			break
		}
	}

	if !hashed {
		return ""
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}
