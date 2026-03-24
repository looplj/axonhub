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
	assert.Equal(t, "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd", uid.ClientIDHex)
	assert.Equal(t, "", uid.AccountUUID)
	assert.Equal(t, "7581b58b-1234-5678-9abc-def012345678", uid.SessionUUID)
}

func TestParseUserID_V2JSON(t *testing.T) {
	raw := `{"client_id_hex":"67bad5aabbccdd1122334455667788990011223344556677889900aabbccddee","account_uuid":"acc-uuid-123","session_uuid":"7581b58b-1234-5678-9abc-def012345678"}`

	uid := ParseUserID(raw)
	require.NotNil(t, uid)
	assert.Equal(t, "67bad5aabbccdd1122334455667788990011223344556677889900aabbccddee", uid.ClientIDHex)
	assert.Equal(t, "acc-uuid-123", uid.AccountUUID)
	assert.Equal(t, "7581b58b-1234-5678-9abc-def012345678", uid.SessionUUID)
}

func TestParseUserID_V2EmptySessionUUID(t *testing.T) {
	raw := `{"client_id_hex":"abc","account_uuid":"","session_uuid":""}`
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
		ClientIDHex: "deadbeef0011223344556677",
		AccountUUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		SessionUUID: "11111111-2222-3333-4444-555555555555",
	}
	result := BuildUserID(uid)
	assert.Equal(t, "user_deadbeef0011223344556677_account_a1b2c3d4-e5f6-7890-abcd-ef1234567890_session_11111111-2222-3333-4444-555555555555", result)

	parsed := ParseUserID(result)
	require.NotNil(t, parsed)
	assert.Equal(t, uid, *parsed)
}

func TestGenerateUserID(t *testing.T) {
	clientIDHex := "aabbccdd11223344556677880099aabb"
	raw := GenerateUserID(context.Background(), clientIDHex)
	uid := ParseUserID(raw)
	require.NotNil(t, uid)
	assert.Equal(t, clientIDHex, uid.ClientIDHex)
	assert.NotEmpty(t, uid.SessionUUID)
	assert.NotEmpty(t, uid.AccountUUID)
}

func TestGenerateUserID_UsesSharedSessionID(t *testing.T) {
	sessionUUID := "f25958b8-e75c-455d-8b40-f006d87cc2a4"
	ctx := shared.WithSessionID(context.Background(), sessionUUID)

	clientIDHex := "aabb112233445566778899aabbccddeeff"
	raw := GenerateUserID(ctx, clientIDHex)
	uid := ParseUserID(raw)
	require.NotNil(t, uid)
	assert.Equal(t, sessionUUID, uid.SessionUUID)
}

func TestGenerateUserID_AccountUUIDScopedByChannelID(t *testing.T) {
	clientIDHex := "aabb112233445566778899aabbccddeeff"

	ctxCh1 := shared.WithChannelID(context.Background(), 1)
	ctxCh2 := shared.WithChannelID(context.Background(), 2)

	uidCh1A := ParseUserID(GenerateUserID(ctxCh1, clientIDHex))
	uidCh1B := ParseUserID(GenerateUserID(ctxCh1, clientIDHex))
	uidCh2 := ParseUserID(GenerateUserID(ctxCh2, clientIDHex))

	require.NotNil(t, uidCh1A)
	require.NotNil(t, uidCh1B)
	require.NotNil(t, uidCh2)

	assert.Equal(t, uidCh1A.AccountUUID, uidCh1B.AccountUUID)
	assert.NotEqual(t, uidCh1A.AccountUUID, uidCh2.AccountUUID)
}
