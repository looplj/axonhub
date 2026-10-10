package server

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func task6Whoami(t *testing.T, destination, apiKey, sourceName string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, destination+"/codex/v1/user-auth-credential/whoami", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, http.StatusOK, response.StatusCode)
	for _, field := range []string{
		"email",
		"chatgpt_user_id",
		"chatgpt_account_id",
		"chatgpt_plan_type",
		"chatgpt_account_is_fedramp",
	} {
		_, ok := body[field]
		require.True(t, ok, "whoami response missing field %q", field)
	}
	require.Equal(t, "display name", gjson.GetBytes(raw, "email").String())
	t.Logf("GATEWAY WHOAMI source_context=%s status=%d schema=email,chatgpt_user_id,chatgpt_account_id,chatgpt_plan_type,chatgpt_account_is_fedramp stable_name=%s authentication=local_api_key_mapping source_selection=not_involved", sourceName, response.StatusCode, gjson.GetBytes(raw, "email").String())
}
