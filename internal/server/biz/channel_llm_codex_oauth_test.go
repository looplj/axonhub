package biz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

func TestCodexRefreshPersistsChannelCredentials(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(tokenServer.Close)

	prevURLs := codex.DefaultTokenURLs
	codex.DefaultTokenURLs = oauth.OAuthUrls{AuthorizeUrl: prevURLs.AuthorizeUrl, TokenUrl: tokenServer.URL}

	t.Cleanup(func() { codex.DefaultTokenURLs = prevURLs })

	db := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = db.Close() })

	ctx := ent.NewContext(context.Background(), db)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	created, err := db.Channel.Create().
		SetType(channel.TypeCodex).
		SetName("codex").
		SetStatus(channel.StatusEnabled).
		SetSupportedModels([]string{"gpt-4o-mini"}).
		SetDefaultTestModel("gpt-4o-mini").
		SetCredentials(objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{
				AccessToken:  "old-access",
				RefreshToken: "old-refresh",
				ClientID:     codex.ClientID,
				ExpiresAt:    time.Now().Add(-1 * time.Hour),
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	svc := &ChannelService{AbstractService: &AbstractService{db: db}}

	ch, err := svc.buildChannel(created)
	require.NoError(t, err)

	req := &llm.Request{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: loPtr("hi")}},
		},
	}

	_, err = ch.Outbound.TransformRequest(ctx, req)
	require.NoError(t, err)

	reloaded, err := db.Channel.Get(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.Credentials)
	require.NotNil(t, reloaded.Credentials.OAuth)
	require.Equal(t, "new-access", reloaded.Credentials.OAuth.AccessToken)
	require.Equal(t, "new-refresh", reloaded.Credentials.OAuth.RefreshToken)
	require.False(t, reloaded.Credentials.OAuth.ExpiresAt.IsZero())
	require.Equal(t, "new-access", extractAccessTokenFromAPIKeyJSON(t, reloaded.Credentials.APIKey))
}

func loPtr(s string) *string {
	return &s
}

func extractAccessTokenFromAPIKeyJSON(t *testing.T, raw string) string {
	t.Helper()

	creds, err := oauth.ParseCredentialsJSON(raw)
	require.NoError(t, err)

	return creds.AccessToken
}
