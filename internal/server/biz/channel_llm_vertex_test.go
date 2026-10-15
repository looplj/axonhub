package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// Vertex credentials pair an API key with the GCP project and region it belongs to.
// The tests below lock that binding: the request URL must carry the project and region
// of the key actually selected for the request, not the first credential in the list.

const (
	vertexTestBaseURL       = "https://aiplatform.googleapis.com/v1"
	vertexTestDefaultRegion = "global"
)

func vertexChatRequest(model string) *llm.Request {
	return &llm.Request{
		Model: model,
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: new("hello")}},
		},
	}
}

func TestVertexChannelURLBindsSelectedKeyProjectAndRegion(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := contexts.EnsureContainer(authz.WithTestBypass(t.Context()))

	entChannel := client.Channel.Create().
		SetName("Vertex Multi Project Channel").
		SetType(channel.TypeGeminiVertex).
		SetBaseURL(vertexTestBaseURL).
		SetCredentials(objects.ChannelCredentials{
			VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "project-alpha", APIKey: "vertex-key-alpha", Region: "us-central1"},
				{ProjectID: "project-beta", APIKey: "vertex-key-beta", Region: "europe-west1"},
			},
		}).
		SetSupportedModels([]string{"gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.5-pro").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	for _, tc := range []struct {
		name     string
		apiKey   string
		project  string
		region   string
		selected string
	}{
		{
			name:    "first credential",
			apiKey:  "vertex-key-alpha",
			project: "project-alpha",
			region:  "us-central1",
		},
		{
			name:    "second credential",
			apiKey:  "vertex-key-beta",
			project: "project-beta",
			region:  "europe-west1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The override stands in for the selected key so each credential can be
			// exercised deterministically; it flows through the same context provider.
			built, err := channelSvc.buildChannelWithOutbounds(entChannel, tc.apiKey)
			require.NoError(t, err)

			outbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatGeminiContents.String())
			require.NoError(t, err)

			httpReq, err := outbound.TransformRequest(ctx, vertexChatRequest("gemini-2.5-pro"))
			require.NoError(t, err)

			require.Equal(t,
				"https://aiplatform.googleapis.com/v1/projects/"+tc.project+"/locations/"+tc.region+
					"/publishers/google/models/gemini-2.5-pro:generateContent",
				httpReq.URL,
			)
			require.Equal(t, tc.apiKey, httpReq.Auth.APIKey)
		})
	}
}

func TestVertexChannelEmbeddingURLCarriesProjectAndRegion(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := contexts.EnsureContainer(authz.WithTestBypass(t.Context()))

	entChannel := client.Channel.Create().
		SetName("Vertex Embedding Channel").
		SetType(channel.TypeGeminiVertex).
		SetBaseURL(vertexTestBaseURL).
		SetCredentials(objects.ChannelCredentials{
			VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "project-embed", APIKey: "vertex-key-embed", Region: "asia-northeast1"},
			},
		}).
		SetSupportedModels([]string{"text-embedding-005"}).
		SetDefaultTestModel("text-embedding-005").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)

	outbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatGeminiEmbedding.String())
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(ctx, &llm.Request{
		Model:       "text-embedding-005",
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatGeminiEmbedding,
		Embedding:   &llm.EmbeddingRequest{Input: llm.EmbeddingInput{StringArray: []string{"hello", "world"}}},
	})
	require.NoError(t, err)

	require.Equal(t,
		"https://aiplatform.googleapis.com/v1/projects/project-embed/locations/asia-northeast1"+
			"/publishers/google/models/text-embedding-005:batchEmbedContents",
		httpReq.URL,
	)
}

// Build-time validation is the contract for the triple: an empty region is rejected
// rather than silently defaulted, so a malformed credential surfaces as a channel
// error instead of a request to an unintended endpoint.
func TestVertexChannelEmptyRegionIsRejected(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := contexts.EnsureContainer(authz.WithTestBypass(t.Context()))

	entChannel := client.Channel.Create().
		SetName("Vertex Default Region Channel").
		SetType(channel.TypeGeminiVertex).
		SetBaseURL(vertexTestBaseURL).
		SetCredentials(objects.ChannelCredentials{
			VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "project-default", APIKey: "vertex-key-default", Region: ""},
			},
		}).
		SetSupportedModels([]string{"gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.5-pro").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	_, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.ErrorContains(t, err, "vertex api key, project ID, and region are all required")
}

// Channels configured before per-key projects stored the project path in the Base URL.
// The per-key project replaces it, so the resulting URL must not carry two projects.
func TestVertexChannelLegacyBaseURLDoesNotDuplicateProject(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := contexts.EnsureContainer(authz.WithTestBypass(t.Context()))

	entChannel := client.Channel.Create().
		SetName("Vertex Legacy BaseURL Channel").
		SetType(channel.TypeGeminiVertex).
		SetBaseURL("https://aiplatform.googleapis.com/v1/projects/legacy-project/locations/us-central1").
		SetCredentials(objects.ChannelCredentials{
			VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "project-new", APIKey: "vertex-key-new", Region: "us-east5"},
			},
		}).
		SetSupportedModels([]string{"gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.5-pro").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)

	outbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatGeminiContents.String())
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(ctx, vertexChatRequest("gemini-2.5-pro"))
	require.NoError(t, err)

	require.Equal(t,
		"https://aiplatform.googleapis.com/v1/projects/project-new/locations/us-east5"+
			"/publishers/google/models/gemini-2.5-pro:generateContent",
		httpReq.URL,
	)
}

// Ordinary key-based channels keep the flat apiKeys credential path: the Vertex
// fields must not leak into their credentials or their request URL.
func TestOrdinaryChannelIgnoresVertexCredentials(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := contexts.EnsureContainer(authz.WithTestBypass(t.Context()))

	entChannel := client.Channel.Create().
		SetName("Ordinary OpenAI Channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"sk-ordinary"}}).
		SetSupportedModels([]string{"gpt-4o"}).
		SetDefaultTestModel("gpt-4o").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)

	outbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIChatCompletion.String())
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(ctx, vertexChatRequest("gpt-4o"))
	require.NoError(t, err)

	require.Equal(t, "https://api.openai.com/v1/chat/completions", httpReq.URL)
	require.Equal(t, "sk-ordinary", httpReq.Auth.APIKey)

	// The flat key list stays the single source of truth for ordinary channels.
	require.Equal(t, []string{"sk-ordinary"}, built.Credentials.GetAllAPIKeys())
	require.Empty(t, built.Credentials.VertexAPIKeys)
}

func TestValidVertexResourceSegment(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{value: "project-alpha", valid: true},
		{value: "my.project", valid: true},
		{value: "project_underscore", valid: true},
		{value: "us-central1", valid: true},
		{value: "global", valid: true},
		{value: "123456789", valid: true},
		{value: "", valid: false},
		{value: "   ", valid: false},
		{value: "-leading-dash", valid: false},
		{value: ".leading-dot", valid: false},
		{value: "has space", valid: false},
		{value: "has/slash", valid: false},
		{value: "has?query", valid: false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			require.Equal(t, tc.valid, validVertexResourceSegment(tc.value))
		})
	}
}

func TestVertexServiceBaseURL(t *testing.T) {
	for _, tc := range []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "service root with version",
			baseURL: "https://aiplatform.googleapis.com/v1",
			want:    "https://aiplatform.googleapis.com",
		},
		{
			name:    "service root with trailing slash",
			baseURL: "https://aiplatform.googleapis.com/v1/",
			want:    "https://aiplatform.googleapis.com",
		},
		{
			name:    "legacy project path",
			baseURL: "https://aiplatform.googleapis.com/v1/projects/legacy/locations/us-central1",
			want:    "https://aiplatform.googleapis.com",
		},
		{
			name:    "regional host",
			baseURL: "https://us-central1-aiplatform.googleapis.com/v1",
			want:    "https://us-central1-aiplatform.googleapis.com",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, vertexServiceBaseURL(tc.baseURL))
		})
	}
}
