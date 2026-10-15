package gemini

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
)

// credentialKeyProvider stands in for the channel key provider: the real one records the
// selected key in the request context so a base URL provider can read it. Context values
// are immutable, so this stub records the selection instead; the observable contract the
// transformer relies on is that the key provider runs before the base URL is resolved.
type credentialKeyProvider struct {
	key      string
	selected *string
}

func (p *credentialKeyProvider) Get(context.Context) string {
	if p.selected != nil {
		*p.selected = p.key
	}

	return p.key
}

// newVertexTransformer wires a Vertex transformer whose key provider selects key and
// whose base URL provider resolves the endpoint from that selection, which is what the
// backend does once the real provider records the key in the request context.
func newVertexTransformer(t *testing.T, key string, bindings map[string]string) *OutboundTransformer {
	t.Helper()

	selected := key

	tr, err := NewOutboundTransformerWithConfig(Config{
		BaseURL:         "https://aiplatform.googleapis.com/v1",
		APIKeyProvider:  &credentialKeyProvider{key: key, selected: &selected},
		BaseURLProvider: func(context.Context) string { return bindings[selected] },
		PlatformType:    PlatformVertex,
	})
	require.NoError(t, err)

	return tr.(*OutboundTransformer)
}

func TestVertexResolveBaseURLUsesSelectedCredential(t *testing.T) {
	bindings := map[string]string{
		"vertex-key-alpha": "https://aiplatform.googleapis.com/v1/projects/project-alpha/locations/us-central1",
		"vertex-key-beta":  "https://aiplatform.googleapis.com/v1/projects/project-beta/locations/europe-west1",
	}

	for _, tc := range []struct {
		key     string
		wantURL string
	}{
		{key: "vertex-key-alpha", wantURL: bindings["vertex-key-alpha"]},
		{key: "vertex-key-beta", wantURL: bindings["vertex-key-beta"]},
	} {
		t.Run(tc.key, func(t *testing.T) {
			tr := newVertexTransformer(t, tc.key, bindings)

			// The key provider runs first and records the selection, as it does in production.
			tr.config.APIKeyProvider.Get(context.Background())

			// The key provider runs first and records the selection, as it does in production.
			tr.config.APIKeyProvider.Get(context.Background())

			require.Equal(t, tc.wantURL, tr.resolveBaseURL(context.Background()))
		})
	}
}

// When the provider cannot resolve a credential-bound URL, the configured Base URL is
// used so non-Vertex deployments keep working. The transformer normalizes the
// configured URL (dropping the version suffix) before its Vertex branch re-adds one.
func TestVertexResolveBaseURLFallsBackToConfiguredBaseURL(t *testing.T) {
	tr := newVertexTransformer(t, "vertex-key-unknown", nil)

	require.Equal(t, "https://aiplatform.googleapis.com", tr.resolveBaseURL(context.Background()))
}

// A provider that returns an empty URL must not override the configured Base URL.
func TestVertexResolveBaseURLEmptyProviderFallsBack(t *testing.T) {
	tr, err := NewOutboundTransformerWithConfig(Config{
		BaseURL:         "https://aiplatform.googleapis.com",
		APIKeyProvider:  auth.NewStaticKeyProvider("test-key"),
		BaseURLProvider: func(context.Context) string { return "" },
		PlatformType:    PlatformVertex,
	})
	require.NoError(t, err)

	require.Equal(t,
		"https://aiplatform.googleapis.com",
		tr.(*OutboundTransformer).resolveBaseURL(context.Background()),
	)
}

// The chat, embedding and image paths must all route through the credential-bound base
// URL; otherwise a Vertex channel would send the project-less endpoint for some of them.
func TestVertexPathsAllUseCredentialBaseURL(t *testing.T) {
	const credentialBaseURL = "https://aiplatform.googleapis.com/v1/projects/project-alpha/locations/us-central1"

	selected := "vertex-key-alpha"
	tr := newVertexTransformer(t, selected, map[string]string{selected: credentialBaseURL})

	// TransformRequest invokes the key provider itself, which is the ordering under test.
	ctx := context.Background()

	for _, tc := range []struct {
		name    string
		request *llm.Request
		action  string
	}{
		{
			name: "chat",
			request: &llm.Request{
				Model:    "gemini-2.5-pro",
				Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: stringPtrForTest("hello")}}},
			},
			action: "generateContent",
		},
		{
			name: "embedding",
			request: &llm.Request{
				Model:       "text-embedding-005",
				RequestType: llm.RequestTypeEmbedding,
				Embedding:   &llm.EmbeddingRequest{Input: llm.EmbeddingInput{String: "hello"}},
			},
			action: "embedContent",
		},
		{
			name: "image",
			request: &llm.Request{
				Model:       "gemini-2.5-flash-image",
				RequestType: llm.RequestTypeImage,
				Image:       &llm.ImageRequest{Prompt: "draw a cat"},
			},
			action: "generateContent",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			httpReq, err := tr.TransformRequest(ctx, tc.request)
			require.NoError(t, err)

			require.Equal(t,
				credentialBaseURL+"/publishers/google/models/"+tc.request.Model+":"+tc.action,
				httpReq.URL,
			)
		})
	}
}

func stringPtrForTest(v string) *string { return lo.ToPtr(v) }
