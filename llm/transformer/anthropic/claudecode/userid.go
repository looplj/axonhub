package claudecode

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/llm/transformer/shared"
)

// UserID represents parsed Claude Code user_id fields.
type UserID struct {
	DeviceID    string `json:"device_id"`
	AccountUUID string `json:"account_uuid"`
	SessionID   string `json:"session_id"`
}

// legacyPattern matches the old Claude Code user_id format:
// user_<64hex>_account__session_<uuid-v4>.
var legacyPattern = regexp.MustCompile(
	`^user_([a-fA-F0-9]{64})_account__session_([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$`,
)

// ParseUserID parses a Claude Code user_id string, supporting both legacy and v2 JSON formats.
//
// Legacy format: "user_<64hex>_account__session_<uuid>"
// V2 format (>=2.1.78): '{"device_id":"...","account_uuid":"...","session_id":"..."}'
//
// Returns nil if the input doesn't match either format.
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

		if uid.SessionID == "" {
			return nil
		}

		return &uid
	}

	// Try legacy format
	matches := legacyPattern.FindStringSubmatch(raw)
	if matches == nil {
		return nil
	}

	return &UserID{
		DeviceID:    matches[1],
		AccountUUID: "",
		SessionID:   matches[2],
	}
}

// BuildUserID generates a new user_id in v2 JSON format.
func BuildUserID(uid UserID) string {
	data, _ := json.Marshal(uid)
	return string(data)
}

// GenerateUserID creates a user_id in v2 JSON format.
// When accountIdentity is provided, DeviceID and AccountUUID are derived
// deterministically so the same channel always produces the same identity.
// When accountIdentity is empty, random values are used as a fallback.
func GenerateUserID(ctx context.Context, accountIdentity string) string {
	var (
		deviceID    string
		accountUUID string
	)

	if accountIdentity != "" {
		h := sha256.Sum256([]byte(accountIdentity))
		deviceID = hex.EncodeToString(h[:])
		accountUUID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(accountIdentity)).String()
	} else {
		hexBytes := make([]byte, 32)
		_, _ = rand.Read(hexBytes)
		deviceID = hex.EncodeToString(hexBytes)
	}

	sessionID, ok := shared.GetSessionID(ctx)
	if !ok || strings.TrimSpace(sessionID) == "" {
		sessionID = uuid.New().String()
	}

	return BuildUserID(UserID{
		DeviceID:    deviceID,
		AccountUUID: accountUUID,
		SessionID:   sessionID,
	})
}
