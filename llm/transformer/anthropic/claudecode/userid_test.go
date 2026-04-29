package claudecode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestParseUserID_Legacy(t *testing.T) {
	raw := "user_" +
		"aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd" +
		"_account__session_7581b58b-1234-5678-9abc-def012345678"

	uid := ParseUserID(raw)
	require.NotNil(t, uid)
	assert.Equal(t, "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd", uid.DeviceID)
	assert.Equal(t, "", uid.AccountUUID)
	assert.Equal(t, "7581b58b-1234-5678-9abc-def012345678", uid.SessionID)
}

func TestParseUserID_V2JSON(t *testing.T) {
	raw := `{"device_id":"67bad5aabbccdd1122334455667788990011223344556677889900aabbccddee","account_uuid":"acc-uuid-123","session_id":"7581b58b-1234-5678-9abc-def012345678"}`

	uid := ParseUserID(raw)
	require.NotNil(t, uid)
	assert.Equal(t, "67bad5aabbccdd1122334455667788990011223344556677889900aabbccddee", uid.DeviceID)
	assert.Equal(t, "acc-uuid-123", uid.AccountUUID)
	assert.Equal(t, "7581b58b-1234-5678-9abc-def012345678", uid.SessionID)
}

func TestParseUserID_V2EmptySessionID(t *testing.T) {
	raw := `{"device_id":"abc","account_uuid":"","session_id":""}`
	assert.Nil(t, ParseUserID(raw))
}

func TestParseUserID_InvalidInputs(t *testing.T) {
	assert.Nil(t, ParseUserID(""))
	assert.Nil(t, ParseUserID("   "))
	assert.Nil(t, ParseUserID("random-string"))
	assert.Nil(t, ParseUserID("{invalid json"))
	assert.Nil(t, ParseUserID("user_tooshort_account__session_bad-uuid"))
}

func TestBuildUserID(t *testing.T) {
	uid := UserID{
		DeviceID:    "deadbeef",
		AccountUUID: "acc-123",
		SessionID:   "sess-456",
	}
	result := BuildUserID(uid)
	assert.Contains(t, result, `"device_id":"deadbeef"`)
	assert.Contains(t, result, `"session_id":"sess-456"`)

	parsed := ParseUserID(result)
	require.NotNil(t, parsed)
	assert.Equal(t, uid, *parsed)
}

func TestGenerateUserID_Random(t *testing.T) {
	raw := GenerateUserID(context.Background(), "")
	uid := ParseUserID(raw)
	require.NotNil(t, uid)
	assert.Len(t, uid.DeviceID, 64)
	assert.NotEmpty(t, uid.SessionID)
	assert.Equal(t, "", uid.AccountUUID)
}

func TestGenerateUserID_Stable(t *testing.T) {
	raw1 := GenerateUserID(context.Background(), "42")
	raw2 := GenerateUserID(context.Background(), "42")

	uid1 := ParseUserID(raw1)
	uid2 := ParseUserID(raw2)

	require.NotNil(t, uid1)
	require.NotNil(t, uid2)

	assert.Equal(t, uid1.DeviceID, uid2.DeviceID, "DeviceID should be deterministic")
	assert.Equal(t, uid1.AccountUUID, uid2.AccountUUID, "AccountUUID should be deterministic")
	assert.Len(t, uid1.DeviceID, 64)
	assert.NotEmpty(t, uid1.AccountUUID)
}

func TestGenerateUserID_DifferentIdentities(t *testing.T) {
	uid1 := ParseUserID(GenerateUserID(context.Background(), "1"))
	uid2 := ParseUserID(GenerateUserID(context.Background(), "2"))

	require.NotNil(t, uid1)
	require.NotNil(t, uid2)

	assert.NotEqual(t, uid1.DeviceID, uid2.DeviceID)
	assert.NotEqual(t, uid1.AccountUUID, uid2.AccountUUID)
}

func TestGenerateUserID_UsesSharedSessionID(t *testing.T) {
	ctx := shared.WithSessionID(context.Background(), "shared-session-id")

	raw := GenerateUserID(ctx, "")
	uid := ParseUserID(raw)
	require.NotNil(t, uid)
	assert.Equal(t, "shared-session-id", uid.SessionID)
}
