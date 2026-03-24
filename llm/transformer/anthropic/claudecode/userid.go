package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/llm/transformer/shared"
)

// UserID represents parsed Claude Code user_id fields.
type UserID struct {
	ClientIDHex string `json:"client_id_hex"`
	AccountUUID string `json:"account_uuid"`
	SessionUUID string `json:"session_uuid"`
}

// legacyPattern matches the old Claude Code user_id format:
// user_<64hex>_account__session_<uuid-v4>
var legacyPattern = regexp.MustCompile(
	`^user_([a-fA-F0-9]{64})_account__session_([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$`,
)

// newPattern matches the new format:
// user_<hex>_account_<uuid>_session_<uuid>
var newPattern = regexp.MustCompile(
	`^user_([a-fA-F0-9]+)_account_([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})_session_([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$`,
)

// ParseUserID parses a Claude Code user_id string, supporting legacy, new, and v2 JSON formats.
//
// Legacy format: "user_<64hex>_account__session_<uuid>"
// New format: "user_<hex>_account_<uuid>_session_<uuid>"
// V2 format (>=2.1.78): '{"client_id_hex":"...","account_uuid":"...","session_uuid":"..."}'
//
// Returns nil if the input doesn't match any format.
func ParseUserID(raw string) *UserID {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Try v2 JSON format first
	if strings.HasPrefix(raw, "{") {
		var uid UserID
		if err := json.Unmarshal([]byte(raw), &uid); err != nil {
			return nil
		}

		if uid.SessionUUID == "" {
			return nil
		}

		return &uid
	}

	// Try new format
	matches := newPattern.FindStringSubmatch(raw)
	if matches != nil {
		return &UserID{
			ClientIDHex: matches[1],
			AccountUUID: matches[2],
			SessionUUID: matches[3],
		}
	}

	// Try legacy format
	matches = legacyPattern.FindStringSubmatch(raw)
	if matches == nil {
		return nil
	}

	return &UserID{
		ClientIDHex: matches[1],
		AccountUUID: "",
		SessionUUID: matches[2],
	}
}

// BuildUserID generates a user_id in the new format.
func BuildUserID(uid UserID) string {
	return fmt.Sprintf("user_%s_account_%s_session_%s", uid.ClientIDHex, uid.AccountUUID, uid.SessionUUID)
}

// GenerateUserID creates a user_id in the new format.
// Account UUID is deterministic based on channel ID (from context).
// Session UUID is deterministic based on session context hash.
func GenerateUserID(ctx context.Context, clientIDHex string) string {
	sessionID, ok := shared.GetSessionID(ctx)
	if !ok || strings.TrimSpace(sessionID) == "" {
		sessionID = uuid.New().String()
	}

	accountSeed := "axonhub_channel_account"
	if channelID, ok := shared.GetChannelID(ctx); ok {
		accountSeed = fmt.Sprintf("axonhub_channel_account:%d", channelID)
	}
	accountUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(accountSeed)).String()

	return BuildUserID(UserID{
		ClientIDHex: clientIDHex,
		AccountUUID: accountUUID,
		SessionUUID: sessionID,
	})
}
