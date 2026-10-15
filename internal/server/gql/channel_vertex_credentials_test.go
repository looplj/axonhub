package gql

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
)

// vertexCredentialsContext grants the write-channels scope the credentials resolver
// requires, so the read boundary is exercised the way a real client reaches it.
func vertexCredentialsContext(t *testing.T, ctx context.Context, client *ent.Client) context.Context {
	t.Helper()

	owner, err := client.User.Create().
		SetEmail(fmt.Sprintf("vertex-creds-%d@example.com", time.Now().UnixNano())).
		SetPassword("test-password").
		SetFirstName("Vertex").
		SetLastName("Owner").
		SetStatus(user.StatusActivated).
		SetIsOwner(true).
		Save(ctx)
	require.NoError(t, err)

	return contexts.WithUser(ctx, owner)
}

// Vertex credentials are project-bound triples. The credentials resolver is the read
// boundary the edit form echoes from, so it must return the triples intact and must
// not also fold their keys into the flat apiKeys list, which would present them as
// project-less keys and lose the binding on the next save.
func TestChannelResolver_Credentials_KeepsVertexKeysAsTriples(t *testing.T) {
	_, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	vertexChannel, err := client.Channel.Create().
		SetType(channel.TypeGeminiVertex).
		SetName("Vertex Channel").
		SetBaseURL("https://aiplatform.googleapis.com/v1").
		SetCredentials(objects.ChannelCredentials{
			VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "project-alpha", APIKey: "vertex-key-alpha", Region: "us-central1"},
				{ProjectID: "project-beta", APIKey: "vertex-key-beta", Region: "global"},
			},
		}).
		SetSupportedModels([]string{"gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.5-pro").
		Save(ctx)
	require.NoError(t, err)

	ctx = vertexCredentialsContext(t, ctx, client)
	channelResolver := &channelResolver{&Resolver{client: client}}

	creds, err := channelResolver.Credentials(ctx, vertexChannel)
	require.NoError(t, err)
	require.NotNil(t, creds)

	require.Equal(t, []objects.VertexAPIKey{
		{ProjectID: "project-alpha", APIKey: "vertex-key-alpha", Region: "us-central1"},
		{ProjectID: "project-beta", APIKey: "vertex-key-beta", Region: "global"},
	}, creds.VertexAPIKeys)

	// The Vertex keys are returned only as triples, never duplicated into the flat list.
	require.Empty(t, creds.APIKeys)
	require.Empty(t, creds.APIKey)
}

// An ordinary channel keeps the flat apiKeys path and carries no Vertex triples.
func TestChannelResolver_Credentials_OrdinaryChannelHasNoVertexKeys(t *testing.T) {
	_, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	ordinaryChannel, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Ordinary Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"sk-one", "sk-two"}}).
		SetSupportedModels([]string{"gpt-4o"}).
		SetDefaultTestModel("gpt-4o").
		Save(ctx)
	require.NoError(t, err)

	ctx = vertexCredentialsContext(t, ctx, client)
	channelResolver := &channelResolver{&Resolver{client: client}}

	creds, err := channelResolver.Credentials(ctx, ordinaryChannel)
	require.NoError(t, err)
	require.NotNil(t, creds)

	require.Equal(t, []string{"sk-one", "sk-two"}, creds.APIKeys)
	require.Empty(t, creds.VertexAPIKeys)
	require.Empty(t, creds.APIKey)
}

// The GraphQL input mapping is generated, so a regression there would silently drop
// the field again. This guards the wiring through the generated unmarshaler, which is
// the exact seam that previously discarded vertexApiKeys.
func TestChannelCredentialsInput_CarriesVertexKeys(t *testing.T) {
	ec := &executionContext{}

	creds, err := ec.unmarshalInputChannelCredentialsInput(t.Context(), map[string]any{
		"vertexApiKeys": []any{
			map[string]any{"projectID": "project-alpha", "apiKey": "vertex-key-alpha", "region": "us-central1"},
		},
		"managementApiKey": "sk-mg",
	})
	require.NoError(t, err)

	require.Equal(t, []objects.VertexAPIKey{
		{ProjectID: "project-alpha", APIKey: "vertex-key-alpha", Region: "us-central1"},
	}, creds.VertexAPIKeys)
	require.Equal(t, "sk-mg", creds.ManagementAPIKey)
}

// A non-Vertex channel must never expose a Vertex key. Legacy rows can still carry
// one (credentials were accepted before the type rule existed), and returning it
// would offer a Google-project key in the UI as if it belonged to that provider.
func TestChannelResolver_Credentials_DropsVertexKeysOnOtherChannelTypes(t *testing.T) {
	_, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	openaiChannel, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("OpenAI Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{
			APIKeys: []string{"sk-plain"},
			VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "project-alpha", APIKey: "vertex-key-alpha", Region: "global"},
			},
		}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	ctx = vertexCredentialsContext(t, ctx, client)
	channelResolver := &channelResolver{&Resolver{client: client}}

	creds, err := channelResolver.Credentials(ctx, openaiChannel)
	require.NoError(t, err)
	require.NotNil(t, creds)

	require.Empty(t, creds.VertexAPIKeys, "an ordinary channel must not surface Vertex credentials")
	require.Equal(t, []string{"sk-plain"}, creds.APIKeys)
}
