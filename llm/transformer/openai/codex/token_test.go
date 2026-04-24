package codex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDecodeAuthJSON(t *testing.T) {
	creds, err := DecodeAuthJSON(`{
		"auth_mode":"chatgpt",
		"last_refresh":"2026-04-17T08:58:36.389Z",
		"tokens":{
			"access_token":"access",
			"refresh_token":"refresh",
			"id_token":"id"
		}
	}`)
	require.NoError(t, err)
	require.Equal(t, ClientID, creds.ClientID)
	require.Equal(t, "access", creds.AccessToken)
	require.Equal(t, "refresh", creds.RefreshToken)
	require.Equal(t, "id", creds.IDToken)
	require.Equal(t, "bearer", creds.TokenType)
	require.Equal(t, []string{"openid", "profile", "email", "offline_access"}, creds.Scopes)
	require.Equal(t, time.Date(2026, 4, 17, 9, 58, 36, 389000000, time.UTC), creds.ExpiresAt.UTC())
}
