package claudecode

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
)

// IdentityConfig holds the canonical identity configuration.
type IdentityConfig struct {
	DeviceID    string // 64-char hex device ID
	AccountUUID string // UUID v4 format
}

// GenerateIdentityFromAccount generates a deterministic identity based on account identifier.
// This ensures all requests from the same account appear as a single device.
func GenerateIdentityFromAccount(accountIdentity string) *IdentityConfig {
	// Parse account identity to get numeric ID
	accountID, err := strconv.Atoi(accountIdentity)
	if err != nil {
		// Fallback: use string hash if not numeric
		accountID = int(sha256.Sum256([]byte(accountIdentity))[0])
	}

	accountStr := strconv.Itoa(accountID)

	// Generate device_id (64 hex chars = 32 bytes)
	deviceHash := sha256.Sum256([]byte("device:" + accountStr))
	deviceID := hex.EncodeToString(deviceHash[:])

	// Generate account_uuid using a different algorithm
	accountHash := sha256.Sum256([]byte("account:" + accountStr))
	accountUUID := formatUUID(accountHash[:])

	return &IdentityConfig{
		DeviceID:    deviceID,
		AccountUUID: accountUUID,
	}
}

// formatUUID formats a 16-byte hash into UUID v4 format.
func formatUUID(bytes []byte) string {
	if len(bytes) < 16 {
		return ""
	}

	// Set version (4) and variant (2) bits according to RFC 4122
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // Version 4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // Variant 2

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		bytes[0:4],
		bytes[4:6],
		bytes[6:8],
		bytes[8:10],
		bytes[10:16],
	)
}