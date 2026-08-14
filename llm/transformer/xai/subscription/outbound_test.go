package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestOutboundTransformer_TransformRequest_uses_xAI_CLI_responses_identity(t *testing.T) {
	// Given
	const accessToken = "synthetic-access-token"
	tokens := oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{
		AccessToken: accessToken,
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	transformer, err := NewOutboundTransformer(tokens)
	require.NoError(t, err)
	request := &llm.Request{
		Model: "grok-4.5",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
		},
		Stream: lo.ToPtr(true),
	}

	// When
	httpRequest, err := transformer.TransformRequest(context.Background(), request)

	// Then
	require.NoError(t, err)
	require.Equal(t, DefaultBaseURL+"/responses", httpRequest.URL)
	require.Equal(t, accessToken, httpRequest.Auth.APIKey)
	require.Equal(t, CLITokenAuth, httpRequest.Headers.Get(CLITokenAuthHeader))
	require.Equal(t, CLIClientVersion, httpRequest.Headers.Get(CLIClientVersionHeader))
	require.Equal(t, CLIClientIdentifier, httpRequest.Headers.Get(CLIClientIdentifierHeader))
	require.Equal(t, CLIUserAgent, httpRequest.Headers.Get("User-Agent"))
}

func TestOutboundTransformer_APIFormat_returns_responses(t *testing.T) {
	// Given
	transformer, err := NewOutboundTransformer(oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{AccessToken: "synthetic-token"}))
	require.NoError(t, err)

	// When
	format := transformer.APIFormat()

	// Then
	require.Equal(t, llm.APIFormatOpenAIResponse, format)
}

func TestOutboundTransformer_TransformRequest_rejects_non_chat_requests(t *testing.T) {
	for _, requestType := range []llm.RequestType{llm.RequestTypeImage, llm.RequestTypeCompact} {
		t.Run(string(requestType), func(t *testing.T) {
			// Given
			outbound, err := NewOutboundTransformer(oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{AccessToken: "synthetic-token"}))
			require.NoError(t, err)

			// When
			request, err := outbound.TransformRequest(t.Context(), &llm.Request{RequestType: requestType, Model: "synthetic-model"})

			// Then
			require.Nil(t, request)
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		})
	}
}
