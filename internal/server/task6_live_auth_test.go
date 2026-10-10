package server

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/stretchr/testify/require"
)

const task6Home = "/root/.local/share/axonhub-codex-testing/"

func task6Sources(t *testing.T, h *codexRouteHarness) []*ent.Channel {
	t.Helper()
	raw, err := os.ReadFile(task6Home + "codex-home/auth.json")
	require.NoError(t, err)
	creds, err := codex.DecodeAuthJSON(string(raw))
	require.NoError(t, err)
	var relay struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
	}
	relayRaw, err := os.ReadFile(task6Home + "krill.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(relayRaw, &relay))
	// The Codex transformer uses the trailing raw-base marker to avoid adding /v1
	// to the official Responses endpoint; the catalog request strips it.
	official := h.db.Channel.Create().SetName("official-inference").SetType(channel.TypeCodex).SetStatus(channel.StatusDisabled).SetBaseURL("https://chatgpt.com/backend-api/codex#").SetDefaultTestModel("gpt-5.5").SetSupportedModels([]string{"gpt-5.5"}).SetCredentials(objects.ChannelCredentials{OAuth: creds}).SaveX(h.ctx)
	krillBase := strings.TrimRight(relay.BaseURL, "/#") + "#"
	krill := h.db.Channel.Create().SetName("krill-inference").SetType(channel.TypeCodex).SetStatus(channel.StatusDisabled).SetBaseURL(krillBase).SetDefaultTestModel("gpt-5.5").SetSupportedModels([]string{"gpt-5.5"}).SetCredentials(objects.ChannelCredentials{APIKey: relay.APIKey}).SaveX(h.ctx)
	t.Cleanup(func() {
		updated := h.db.Channel.GetX(context.WithoutCancel(h.ctx), official.ID).Credentials.OAuth
		if updated.AccessToken == creds.AccessToken && updated.RefreshToken == creds.RefreshToken {
			t.Log("OAuth export: unchanged; original preserved")
			return
		}
		var document map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &document))
		var tokens map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(document["tokens"], &tokens))
		tokens["access_token"], err = json.Marshal(updated.AccessToken)
		require.NoError(t, err)
		tokens["refresh_token"], err = json.Marshal(updated.RefreshToken)
		require.NoError(t, err)
		if updated.IDToken != "" {
			tokens["id_token"], err = json.Marshal(updated.IDToken)
			require.NoError(t, err)
		}
		document["tokens"], err = json.Marshal(tokens)
		require.NoError(t, err)
		document["last_refresh"], err = json.Marshal(time.Now().UTC().Format(time.RFC3339Nano))
		require.NoError(t, err)
		encoded, err := json.MarshalIndent(document, "", "  ")
		require.NoError(t, err)
		temp, err := os.CreateTemp(task6Home+"codex-home", ".task6-auth-*")
		require.NoError(t, err)
		defer os.Remove(temp.Name())
		require.NoError(t, temp.Chmod(0600))
		_, err = temp.Write(encoded)
		require.NoError(t, err)
		require.NoError(t, temp.Sync())
		require.NoError(t, temp.Close())
		require.NoError(t, os.Rename(temp.Name(), task6Home+"codex-home/auth.json"))
		t.Log("OAuth export: latest DB rotation atomically persisted mode600")
	})
	return []*ent.Channel{official, krill}
}
