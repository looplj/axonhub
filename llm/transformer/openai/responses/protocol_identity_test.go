package responses_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestResponsesProtocolCharacterization(t *testing.T) {
	// Given: the existing HTTP transformer and a synthetic client body.
	inbound := responses.NewInboundTransformer()
	outbound, err := responses.NewOutboundTransformer("https://example.invalid", "synthetic")
	require.NoError(t, err)
	request, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{"model":"synthetic","input":"canary"}`)})
	require.NoError(t, err)
	// When: the real module surface transforms the request.
	wire, err := outbound.TransformRequest(context.Background(), request)
	// Then: HTTP identity, input and native Images rejection remain intact.
	require.NoError(t, err)
	require.Equal(t, llm.APIFormatOpenAIResponse, request.APIFormat)
	require.Equal(t, llm.APIFormatOpenAIResponse.String(), wire.APIFormat)
	require.Contains(t, string(wire.Body), "canary")
	_, err = outbound.TransformRequest(context.Background(), &llm.Request{RequestType: llm.RequestTypeImage})
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
}

func TestResponsesProtocolIdentity(t *testing.T) {
	t.Run("websocket inbound", func(t *testing.T) {
		// Given
		inbound := responses.NewWebSocketInboundTransformer()
		// When
		request, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{"type":"response.create","stream_id":"lane","model":"synthetic","input":"canary"}`)})
		// Then
		require.NoError(t, err)
		require.Equal(t, llm.APIFormatOpenAIResponseWebSocket, inbound.APIFormat())
		require.Equal(t, inbound.APIFormat(), request.APIFormat)
	})
	for _, tc := range []struct {
		transport string
		format    llm.APIFormat
	}{
		{responses.TransportHTTP, llm.APIFormatOpenAIResponse},
		{responses.TransportWebSocket, llm.APIFormatOpenAIResponseWebSocket},
	} {
		t.Run(tc.transport, func(t *testing.T) {
			// Given: a transformer configured for the actual upstream transport.
			outbound, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
				BaseURL: "https://example.invalid", APIKeyProvider: auth.NewStaticKeyProvider("synthetic"), Transport: tc.transport,
			})
			require.NoError(t, err)
			// When: the transformer emits an upstream request.
			wire, err := outbound.TransformRequest(context.Background(), &llm.Request{Model: "synthetic", RequestType: llm.RequestTypeChat})
			// Then: both the transformer and emitted request expose the actual protocol.
			require.NoError(t, err)
			require.Equal(t, tc.format, outbound.APIFormat())
			require.Equal(t, tc.format.String(), wire.APIFormat)
			_, err = outbound.TransformRequest(context.Background(), &llm.Request{Model: "synthetic", RequestType: llm.RequestTypeImage})
			if tc.format == llm.APIFormatOpenAIResponseWebSocket {
				require.ErrorIs(t, err, transformer.ErrUnsupportedConversion)
			} else {
				require.ErrorIs(t, err, transformer.ErrInvalidRequest)
			}
		})
	}
}

func TestResponsesProtocolIdentityRejectsWebSocketCompact(t *testing.T) {
	// Given
	outbound, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
		BaseURL: "https://example.invalid", APIKeyProvider: auth.NewStaticKeyProvider("synthetic"), Transport: responses.TransportWebSocket,
	})
	require.NoError(t, err)
	// When
	wire, err := outbound.TransformRequest(context.Background(), &llm.Request{Model: "synthetic", RequestType: llm.RequestTypeCompact, Compact: &llm.CompactRequest{}})
	// Then
	require.ErrorIs(t, err, transformer.ErrUnsupportedConversion)
	require.Nil(t, wire)
}

func TestResponsesProtocolIdentityInboundStateIsolation(t *testing.T) {
	// Given: one WS transformer reused for two unrelated synthetic requests.
	inbound := responses.NewWebSocketInboundTransformer()
	first, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{"type":"response.create","model":"synthetic","previous_response_id":"resp_old","input":"old"}`)})
	require.NoError(t, err)
	require.NotNil(t, first.PreviousResponseID)
	require.Equal(t, "resp_old", *first.PreviousResponseID)
	// When: the next normalized frame has no continuation ID.
	second, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{"model":"synthetic","input":"new"}`)})
	// Then: stale state is not inherited by the new request.
	require.NoError(t, err)
	require.Nil(t, second.PreviousResponseID)
	require.Equal(t, llm.APIFormatOpenAIResponseWebSocket, second.APIFormat)
}
