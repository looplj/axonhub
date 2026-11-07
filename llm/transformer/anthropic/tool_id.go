package anthropic

import (
	"crypto/sha256"
	"encoding/hex"
)

// normalizeToolUseID preserves valid Anthropic IDs and deterministically maps
// invalid IDs to a bounded, legal value in responses to Anthropic clients.
// The mapping is intentionally one-way; it does not restore upstream IDs.
func normalizeToolUseID(id string) string {
	valid := id != ""
	for i := range len(id) {
		c := id[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			valid = false
			break
		}
	}
	if valid {
		return id
	}

	sum := sha256.Sum256([]byte(id))
	// Keep 128 bits of the digest for a compact, deterministic 38-character ID.
	return "toolu_" + hex.EncodeToString(sum[:16])
}

// Special Anthropic tool blocks have their own identities and references.
func anthropicToolBlockID(blockType, id string) string {
	if blockType == "tool_use" {
		return normalizeToolUseID(id)
	}
	return id
}
