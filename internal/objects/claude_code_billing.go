package objects

import (
	"fmt"
	"io"
)

// ClaudeCodeBillingHeaderMode selects how a channel treats the Claude Code
// billing header system message (`x-anthropic-billing-header: ...`) that the
// Claude Code client sends as its first system block.
//
// The two requirements in the wild are opposed: some relays reject requests
// that still carry the block, others require a recognisable Claude Code
// identity marker because they forward to the official OAuth endpoint. No
// inference from the channel type can satisfy both, so the choice is left to
// the operator.
type ClaudeCodeBillingHeaderMode string

const (
	// ClaudeCodeBillingHeaderAuto is the zero value and keeps the historic
	// behaviour: the block survives only for an official Claude Code OAuth
	// channel and is removed for everything else.
	ClaudeCodeBillingHeaderAuto ClaudeCodeBillingHeaderMode = ""

	// ClaudeCodeBillingHeaderKeep always forwards the billing system message.
	ClaudeCodeBillingHeaderKeep ClaudeCodeBillingHeaderMode = "keep"

	// ClaudeCodeBillingHeaderStrip always removes the billing system message.
	ClaudeCodeBillingHeaderStrip ClaudeCodeBillingHeaderMode = "strip"
)

// MarshalGQL writes the GraphQL enum value. Unknown persisted values degrade to
// AUTO so an older or hand-edited setting can never break a channel read.
func (m ClaudeCodeBillingHeaderMode) MarshalGQL(w io.Writer) {
	var value string

	switch m {
	case ClaudeCodeBillingHeaderKeep:
		value = "KEEP"
	case ClaudeCodeBillingHeaderStrip:
		value = "STRIP"
	default:
		value = "AUTO"
	}

	_, _ = io.WriteString(w, `"`+value+`"`)
}

// UnmarshalGQL reads the GraphQL enum value. AUTO maps back to the zero value so
// that selecting the default clears the field instead of persisting a marker.
func (m *ClaudeCodeBillingHeaderMode) UnmarshalGQL(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("ClaudeCodeBillingHeaderMode must be a string")
	}

	switch str {
	case "AUTO":
		*m = ClaudeCodeBillingHeaderAuto
	case "KEEP":
		*m = ClaudeCodeBillingHeaderKeep
	case "STRIP":
		*m = ClaudeCodeBillingHeaderStrip
	default:
		return fmt.Errorf("invalid ClaudeCodeBillingHeaderMode: %s", str)
	}

	return nil
}
