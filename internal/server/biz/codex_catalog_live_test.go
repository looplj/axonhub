package biz

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/stretchr/testify/require"
)

func TestCodexCatalog_RealSources(t *testing.T) {
	if os.Getenv("AXONHUB_CODEX_LIVE_QA") != "1" {
		t.Skip("requires authorized external credentials")
	}
	const home = "/root/.local/share/axonhub-codex-testing/"
	authPath := home + "codex-home/auth.json"
	raw, err := os.ReadFile(authPath)
	require.NoError(t, err)
	creds, err := codex.DecodeAuthJSON(string(raw))
	require.NoError(t, err)
	var relay struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
	}
	relayRaw, err := os.ReadFile(home + "krill.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(relayRaw, &relay))
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	oauthSource := db.Channel.Create().SetName("live-oauth").SetType(channel.TypeCodex).SetStatus(channel.StatusDisabled).SetBaseURL("https://chatgpt.com/backend-api/codex").SetDefaultTestModel("gpt-5.5").SetSupportedModels([]string{"gpt-5.5"}).SetCredentials(objects.ChannelCredentials{OAuth: creds}).SaveX(ctx)
	relaySource := db.Channel.Create().SetName("live-relay").SetType(channel.TypeCodex).SetStatus(channel.StatusArchived).SetBaseURL(relay.BaseURL).SetDefaultTestModel("gpt-5.5").SetSupportedModels([]string{"gpt-5.5"}).SetCredentials(objects.ChannelCredentials{APIKey: relay.APIKey}).SaveX(ctx)
	svc := NewChannelServiceForTest(db)
	defer svc.Stop()
	defer func() {
		updated := db.Channel.GetX(ctx, oauthSource.ID).Credentials.OAuth
		if updated.AccessToken == creds.AccessToken && updated.RefreshToken == creds.RefreshToken {
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
		temp, err := os.CreateTemp(home+"codex-home", ".rotated-auth-*")
		require.NoError(t, err)
		defer os.Remove(temp.Name())
		require.NoError(t, temp.Chmod(0600))
		_, err = temp.Write(encoded)
		require.NoError(t, err)
		require.NoError(t, temp.Close())
		require.NoError(t, os.Rename(temp.Name(), authPath))
	}()
	for _, source := range []*ent.Channel{oauthSource, relaySource} {
		t.Run(source.Name, func(t *testing.T) {
			catalog, err := svc.FetchCodexCatalog(ctx, source.ID, "0.162.1")
			require.NoError(t, err)
			require.Positive(t, catalog.Count())
			require.Equal(t, source.Status, db.Channel.GetX(ctx, source.ID).Status)
			t.Logf("real catalog success: count=%d, source state unchanged", catalog.Count())
		})
	}
}
