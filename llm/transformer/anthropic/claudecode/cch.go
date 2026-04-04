package claudecode

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// CCH constants (reverse-engineered from Claude CLI)
const (
	CCHSalt = "59cf53e54c78"
)

var CCHPositions = []int{4, 7, 20}

// ComputeCCH computes the billing header hash from the first user message.
// This hash is used in the x-anthropic-billing-header to attribute requests.
func ComputeCCH(firstUserMessageText string, version string) string {
	if version == "" {
		version = CCHVersion
	}

	// Extract characters at specific positions
	var chars strings.Builder
	for _, pos := range CCHPositions {
		if pos < len(firstUserMessageText) {
			chars.WriteString(string(firstUserMessageText[pos]))
		} else {
			chars.WriteString("0")
		}
	}

	// Compute SHA-256 hash
	data := CCHSalt + chars.String() + version
	hash := sha256.Sum256([]byte(data))

	// Return first 3 hex chars
	return hex.EncodeToString(hash[:])[:3]
}
