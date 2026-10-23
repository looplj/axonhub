package biz

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestResponsesEndpointLegacyNormalizationExplicitModelProtocolsUpdate(t *testing.T) {
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	for _, tt := range []struct {
		name      string
		endpoints []objects.ChannelEndpoint
		formats   []string
		want      []string
	}{
		{"canonical dual explicit H", []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}, []string{h}, []string{h}},
		{"canonical HTTP only explicit H", []objects.ChannelEndpoint{{APIFormat: h}}, []string{h}, []string{h}},
		{"canonical dual explicit W", []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}, []string{w}, []string{w}},
		{"legacy client explicit H websocket", []objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket"}}, []string{h}, []string{w}},
		{"omitted endpoints legacy settings", nil, []string{h}, []string{w}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Given: independent SQLite fixture with old WS-only H restriction.
			client := enttest.NewEntClient(t, "sqlite3", "file:"+filepath.Join(t.TempDir(), "update.db")+"?_fk=0")
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			svc := NewChannelServiceForTest(client)
			t.Cleanup(svc.Stop)
			original, err := client.Channel.Create().SetType(channel.TypeCodex).SetName("synthetic-explicit-update").
				SetBaseURL("https://chatgpt.com/backend-api/codex#").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
				SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
				SetEndpoints([]objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket"}}).
				SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{h}}}}).Save(ctx)
			require.NoError(t, err)
			// When: explicit replacement settings accompany the submitted endpoints.
			_, err = svc.UpdateChannel(ctx, original.ID, &ent.UpdateChannelInput{
				Endpoints: tt.endpoints,
				Settings:  &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: tt.formats}}},
			})
			require.NoError(t, err)
			// Then: the real persisted restriction retains the submitted identity.
			stored, err := client.Channel.Get(ctx, original.ID)
			require.NoError(t, err)
			require.Equal(t, tt.want, stored.Settings.ModelProtocols[0].APIFormats)
			require.Equal(t, tt.want, (&Channel{Channel: stored}).ForcedAPIFormats("test-model"))
			require.Equal(t, "https://chatgpt.com/backend-api/codex#", stored.BaseURL)
		})
	}
}
