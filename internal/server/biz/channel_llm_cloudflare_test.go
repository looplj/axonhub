package biz

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/cloudflare"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

const cloudflareTestBaseURL = "https://api.cloudflare.com/client/v4/accounts/<account_id>/ai"

func createCloudflareTestChannel(t *testing.T, client *ent.Client, endpoints []objects.ChannelEndpoint) *ent.Channel {
	t.Helper()

	ctx := authz.WithTestBypass(context.Background())

	create := client.Channel.Create().
		SetName("Cloudflare Channel").
		SetType(channel.TypeCloudflare).
		SetBaseURL(cloudflareTestBaseURL).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"@cf/meta/llama-3.1-8b-instruct", "@cf/baai/bge-m3", "@cf/cloudflare/clef"}).
		SetDefaultTestModel("@cf/meta/llama-3.1-8b-instruct")
	if endpoints != nil {
		create.SetEndpoints(endpoints)
	}

	return create.SaveX(ctx)
}

func TestCloudflareChannel_DefaultEndpoints(t *testing.T) {
	defaults := DefaultEndpointsForChannelType(channel.TypeCloudflare)
	require.Equal(t, []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatOpenAIEmbedding.String()},
	}, defaults)
}

func TestCloudflareChannel_ValidatesSystemOneEndpoint(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatTypeSafeSystemOne.String()},
	}
	require.NoError(t, validateEndpointsForChannelType(channel.TypeCloudflare, endpoints))

	// Channel types without a System One surface (e.g. Anthropic) keep rejecting it.
	err := validateEndpointsForChannelType(channel.TypeAnthropic, endpoints)
	require.Error(t, err)
	require.Contains(t, err.Error(), "only supported by TypeSafe, OpenAI and Cloudflare channel types")
}

func TestCloudflareChannel_BuildsOpenAIOutbounds(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	entChannel := createCloudflareTestChannel(t, client, nil)

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)

	_, ok := built.Outbound.(*openai.OutboundTransformer)
	require.True(t, ok, "TypeCloudflare should create openai.OutboundTransformer")
	require.Len(t, built.Outbounds, 2)

	chatOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIChatCompletion.String())
	require.NoError(t, err)
	require.Same(t, built.Outbound, chatOutbound)

	content := "hello"
	chatReq, err := chatOutbound.TransformRequest(ctx, &llm.Request{
		Model: "@cf/meta/llama-3.1-8b-instruct",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: &content},
		}},
	})
	require.NoError(t, err)
	require.Equal(t, cloudflareTestBaseURL+"/v1/chat/completions", chatReq.URL)

	embeddingOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIEmbedding.String())
	require.NoError(t, err)
	require.Same(t, built.Outbound, embeddingOutbound)

	embeddingReq, err := embeddingOutbound.TransformRequest(ctx, &llm.Request{
		Model:       "@cf/baai/bge-m3",
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatOpenAIEmbedding,
		Embedding: &llm.EmbeddingRequest{
			Input: llm.EmbeddingInput{String: "hello"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, cloudflareTestBaseURL+"/v1/embeddings", embeddingReq.URL)

	_, err = BuildOutboundByAPIFormat(built, llm.APIFormatTypeSafeSystemOne.String())
	require.Error(t, err, "System One requires an explicit endpoint on cloudflare channels")
}

func TestCloudflareChannel_BuildsSystemOneOutbound(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	entChannel := createCloudflareTestChannel(t, client, []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatTypeSafeSystemOne.String()},
	})

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)

	resolved := built.ResolveEndpoints()
	require.Equal(t, []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatOpenAIEmbedding.String()},
		{APIFormat: llm.APIFormatTypeSafeSystemOne.String()},
	}, resolved)
	require.Len(t, built.Outbounds, 3)

	systemOneOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatTypeSafeSystemOne.String())
	require.NoError(t, err)
	_, ok := systemOneOutbound.(*cloudflare.SystemOneOutboundTransformer)
	require.True(t, ok, "cloudflare System One endpoint should create cloudflare.SystemOneOutboundTransformer")

	req, err := systemOneOutbound.TransformRequest(ctx, &llm.Request{
		Model:       "@cf/cloudflare/clef",
		RequestType: llm.RequestTypeSystemOne,
		APIFormat:   llm.APIFormatTypeSafeSystemOne,
		SystemOne: &llm.SystemOneRequest{
			State: "Sample state",
			Questions: map[string]llm.SystemOneQuestion{
				"check": {Type: "noul", Instructions: "True?"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, cloudflareTestBaseURL+"/run/@cf/cloudflare/clef", req.URL)
	require.Equal(t, "test-key", req.Auth.APIKey)

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.Body, &body))
	require.Equal(t, "clef", body["model"])

	// Chat and embeddings keep using the primary OpenAI outbound.
	chatOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIChatCompletion.String())
	require.NoError(t, err)
	require.Same(t, built.Outbound, chatOutbound)
}
