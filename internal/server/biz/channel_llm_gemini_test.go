package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	geminitransformer "github.com/looplj/axonhub/llm/transformer/gemini"
)

func TestGeminiChannel_BuildChannelWithOutbounds(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Gemini Multi Endpoint Channel").
		SetType(channel.TypeGemini).
		SetBaseURL("https://generativelanguage.googleapis.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.5-pro").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)
	require.Len(t, built.Outbounds, 2)

	require.Equal(t, llm.APIFormatGeminiContents, built.Outbound.APIFormat())

	embeddingOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatGeminiEmbedding.String())
	require.NoError(t, err)
	require.NotNil(t, embeddingOutbound)
	_, ok := embeddingOutbound.(*geminitransformer.OutboundTransformer)
	require.True(t, ok)
}

func TestGeminiVertexChannel_BuildRejectsEmptyCredentials(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file=ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	entChannel := client.Channel.Create().
		SetName(t.Name()).
		SetType(channel.TypeGeminiVertex).
		SetBaseURL("https://aiplatform.googleapis.com/v1").
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"gemini-3.8-flash"}).
		SetDefaultTestModel("gemini-3.8-flash").
		SaveX(ctx)

	_, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entChannel)
	require.ErrorContains(t, err, "missing api key")
}

func TestValidateGeminiVertexCredentials(t *testing.T) {
	t.Run("accepts service account credentials with ADC fallback", func(t *testing.T) {
		err := validateGeminiVertexCredentials(objects.ChannelCredentials{
			GCP: &objects.GCPCredential{ProjectID: "miguo-vertex-prod-1", Region: "global"},
		})
		require.NoError(t, err)
	})

	t.Run("rejects mixed API key and service account credentials", func(t *testing.T) {
		err := validateGeminiVertexCredentials(objects.ChannelCredentials{
			VertexAPIKeys: []objects.VertexAPIKey{{ProjectID: "project", APIKey: "key", Region: "global"}},
			GCP:           &objects.GCPCredential{ProjectID: "project", Region: "global"},
		})
		require.EqualError(t, err, "GCP credentials cannot be combined with API keys")
	})
}

func TestVertexProjectBaseURL(t *testing.T) {
	require.Equal(t,
		"https://aiplatform.googleapis.com/v1/projects/project/locations/global",
		vertexProjectBaseURL("https://aiplatform.googleapis.com", &objects.GCPCredential{
			ProjectID: "project",
			Region:    "global",
		}),
	)
}
