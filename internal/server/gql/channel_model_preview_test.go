package gql

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestPreviewChannelModelEntries_GraphQL(t *testing.T) {
	// No services or database are needed: the preview must not persist settings.
	server := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: &Resolver{}}))
	body := `{"query":"query Preview($input: ChannelModelPreviewInput!) { previewChannelModelEntries(input: $input) { requestModel actualModel source } }","variables":{"input":{"supportedModels":["Vendor/GPT","vendor-other"],"settings":{"extraModelPrefix":"public","autoTrimedModelPrefixes":["Vendor"],"modelMappings":[{"from":"chat","to":"Vendor/GPT"}],"hideMappedModels":true,"lowercaseModelId":true}}}}`

	t.Run("resolves aliases using routing rules", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(authz.WithTestBypass(t.Context()))
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		var result struct {
			Data struct {
				Entries []biz.ChannelModelEntry `json:"previewChannelModelEntries"`
			} `json:"data"`
			Errors []json.RawMessage `json:"errors"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.Empty(t, result.Errors)
		require.Equal(t, []biz.ChannelModelEntry{
			{RequestModel: "chat", ActualModel: "Vendor/GPT", Source: "mapping"},
			{RequestModel: "public/vendor-other", ActualModel: "vendor-other", Source: "prefix"},
			{RequestModel: "vendor-other", ActualModel: "vendor-other", Source: "direct"},
		}, result.Data.Entries)
	})

	t.Run("requires channel read permission", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		require.Contains(t, response.Body.String(), "read_channels")
	})
}

func TestPreviewChannelModelEntries_DoesNotMutateInput(t *testing.T) {
	settings := &objects.ChannelSettings{
		ModelMappings:      []objects.ModelMapping{{From: "chat", To: "Vendor/GPT"}},
		HideOriginalModels: true,
		LowercaseModelID:   true,
	}
	input := ChannelModelPreviewInput{SupportedModels: []string{"Vendor/GPT"}, Settings: settings}
	before, err := json.Marshal(input)
	require.NoError(t, err)
	resolver := &queryResolver{&Resolver{}}
	entries, err := resolver.PreviewChannelModelEntries(authz.WithTestBypass(t.Context()), input)
	require.NoError(t, err)
	require.Equal(t, []*biz.ChannelModelEntry{{RequestModel: "chat", ActualModel: "Vendor/GPT", Source: "mapping"}}, entries)
	after, err := json.Marshal(input)
	require.NoError(t, err)
	require.Equal(t, before, after)

	entries, err = resolver.PreviewChannelModelEntries(authz.WithTestBypass(t.Context()), ChannelModelPreviewInput{})
	require.NoError(t, err)
	require.Empty(t, entries)
}
