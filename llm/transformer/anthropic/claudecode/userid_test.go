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

func TestGenerateUserID(t *testing.T) {
	raw := GenerateUserID(context.Background())
	uid := ParseUserID(raw)
	require.NotNil(t, uid)
	assert.Len(t, uid.DeviceID, 64)
	assert.NotEmpty(t, uid.SessionID)
	assert.Equal(t, "", uid.AccountUUID)
}

func TestGenerateUserID_UsesSharedSessionID(t *testing.T) {
	ctx := shared.WithSessionID(context.Background(), "shared-session-id")

	raw := GenerateUserID(ctx)
	uid := ParseUserID(raw)
	require.NotNil(t, uid)
	assert.Equal(t, "shared-session-id", uid.SessionID)
}
