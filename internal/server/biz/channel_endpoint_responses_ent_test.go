package biz

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestResponsesEndpointLegacyNormalizationEnt(t *testing.T) {
	// Given: a real isolated SQLite Ent driver and legacy persisted configuration.
	client := enttest.NewEntClient(t, "sqlite3", "file:"+filepath.Join(t.TempDir(), "endpoints.db")+"?_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewChannelServiceForTest(client)
	t.Cleanup(svc.Stop)
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	legacy := []objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket", Path: "/custom"}}
	created, err := client.Channel.Create().SetType(channel.TypeCodex).SetName("synthetic-legacy").
		SetBaseURL("https://chatgpt.com/backend-api/codex#").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
		SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetEndpoints(legacy).
		SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{h}}}}).Save(ctx)
	require.NoError(t, err)

	t.Run("read preserves legacy storage", func(t *testing.T) {
		// When
		read, err := client.Channel.Get(ctx, created.ID)
		require.NoError(t, err)
		resolved := (&Channel{Channel: read}).ResolveEndpoints()
		// Then
		require.Equal(t, objects.ChannelEndpoint{APIFormat: w, Path: "/custom"}, resolved[0])
		require.Equal(t, legacy, read.Endpoints)
		require.Equal(t, []string{h}, read.Settings.ModelProtocols[0].APIFormats)
		require.Equal(t, "https://chatgpt.com/backend-api/codex#", read.BaseURL)
	})
	t.Run("collision rejected before persistence", func(t *testing.T) {
		// When
		_, err := svc.SaveChannelEndpoints(ctx, SaveChannelEndpointsInput{ChannelID: objects.GUID{ID: created.ID}, Endpoints: []objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket"}, {APIFormat: w}}})
		// Then
		require.ErrorContains(t, err, "duplicate api_format")
		read, err := client.Channel.Get(ctx, created.ID)
		require.NoError(t, err)
		require.Equal(t, legacy, read.Endpoints)
	})
	t.Run("explicit dual save translates old model restriction", func(t *testing.T) {
		// When
		_, err := svc.SaveChannelEndpoints(ctx, SaveChannelEndpointsInput{ChannelID: objects.GUID{ID: created.ID}, Endpoints: []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}})
		require.NoError(t, err)
		read, err := client.Channel.Get(ctx, created.ID)
		// Then
		require.NoError(t, err)
		require.Equal(t, []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}, read.Endpoints)
		require.Equal(t, []string{w}, read.Settings.ModelProtocols[0].APIFormats)
		require.Equal(t, []string{w}, (&Channel{Channel: read}).ForcedAPIFormats("test-model"))
		require.Equal(t, "https://chatgpt.com/backend-api/codex#", read.BaseURL)
	})
	t.Run("disabling W rejects surviving W model restriction", func(t *testing.T) {
		// When
		_, err := svc.UpdateChannel(ctx, created.ID, &ent.UpdateChannelInput{Endpoints: []objects.ChannelEndpoint{{APIFormat: h}}})
		// Then
		require.ErrorContains(t, err, "not configured on this channel")
	})
	t.Run("create canonicalizes legacy input", func(t *testing.T) {
		// When
		created, err := svc.createChannel(ctx, ent.CreateChannelInput{Type: channel.TypeCodex, Name: "synthetic-new", BaseURL: lo.ToPtr("https://chatgpt.com/backend-api/codex#"), Credentials: objects.ChannelCredentials{APIKey: "synthetic-key"}, SupportedModels: []string{"test-model"}, DefaultTestModel: "test-model", Endpoints: legacy, Settings: &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{h}}}}})
		require.NoError(t, err)
		read, err := client.Channel.Get(ctx, created.ID)
		// Then
		require.NoError(t, err)
		require.Equal(t, []objects.ChannelEndpoint{{APIFormat: w, Path: "/custom"}}, read.Endpoints)
		require.Equal(t, []string{w}, read.Settings.ModelProtocols[0].APIFormats)
	})
	t.Run("update uses old H restriction identity before replacing endpoints", func(t *testing.T) {
		// Given
		original, err := client.Channel.Create().SetType(channel.TypeCodex).SetName("synthetic-http").
			SetBaseURL("https://example.test#").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
			SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetEndpoints([]objects.ChannelEndpoint{{APIFormat: h}}).
			SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{h}}}}).Save(ctx)
		require.NoError(t, err)
		// When
		_, err = svc.UpdateChannel(ctx, original.ID, &ent.UpdateChannelInput{Endpoints: legacy})
		// Then
		require.ErrorContains(t, err, "not configured on this channel")
	})
	t.Run("general update persists canonical dual and old W identity", func(t *testing.T) {
		// Given
		original, err := client.Channel.Create().SetType(channel.TypeCodex).SetName("synthetic-update").
			SetBaseURL("https://chatgpt.com/backend-api/codex#").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
			SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetEndpoints(legacy).
			SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{h}}}}).Save(ctx)
		require.NoError(t, err)
		// When
		_, err = svc.UpdateChannel(ctx, original.ID, &ent.UpdateChannelInput{Endpoints: []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}})
		require.NoError(t, err)
		read, err := client.Channel.Get(ctx, original.ID)
		// Then
		require.NoError(t, err)
		require.Equal(t, []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}, read.Endpoints)
		require.Equal(t, []string{w}, read.Settings.ModelProtocols[0].APIFormats)
		require.Equal(t, "https://chatgpt.com/backend-api/codex#", read.BaseURL)
	})
	t.Run("inherited WSS collision rejected on create", func(t *testing.T) {
		// Given
		input := ent.CreateChannelInput{Type: channel.TypeCodex, Name: "synthetic-collision", BaseURL: lo.ToPtr("wss://example.test#"), Credentials: objects.ChannelCredentials{APIKey: "synthetic-key"}, SupportedModels: []string{"test-model"}, DefaultTestModel: "test-model", Endpoints: []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}}
		// When
		_, err := svc.createChannel(ctx, input)
		// Then
		require.ErrorContains(t, err, "duplicate api_format")
	})
}

func TestResponsesEndpointLegacyNormalizationStoredJSON(t *testing.T) {
	// Given
	dsn := "file:" + filepath.Join(t.TempDir(), "stored.db") + "?_fk=0"
	client := enttest.NewEntClient(t, "sqlite3", dsn)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	created, err := client.Channel.Create().SetType(channel.TypeCodex).SetName("synthetic-json").
		SetBaseURL("https://chatgpt.com/backend-api/codex#").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
		SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket", BaseURL: "wss://other.test/custom#"}}).
		SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{h}}}}).Save(ctx)
	require.NoError(t, err)
	var beforeEndpoints, beforeSettings string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT endpoints, settings FROM channels WHERE id = ?", created.ID).Scan(&beforeEndpoints, &beforeSettings))
	// When
	read, err := client.Channel.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, w, (&Channel{Channel: read}).ResolveEndpoints()[0].APIFormat)
	require.Equal(t, []string{w}, (&Channel{Channel: read}).ForcedAPIFormats("test-model"))
	var afterEndpoints, afterSettings string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT endpoints, settings FROM channels WHERE id = ?", created.ID).Scan(&afterEndpoints, &afterSettings))
	// Then
	require.Equal(t, beforeEndpoints, afterEndpoints)
	require.Equal(t, beforeSettings, afterSettings)
	require.JSONEq(t, `[{"api_format":"openai/responses","transport":"websocket","base_url":"wss://other.test/custom#"}]`, afterEndpoints)
}
