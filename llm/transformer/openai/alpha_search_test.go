package openai

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestAlphaSearchInboundAndOutbound(t *testing.T) {
	ctx := context.Background()
	inbound := NewAlphaSearchInboundTransformer()
	raw := []byte(`{"id":"session-123","model":"client-model","commands":{"search_query":[{"q":"golang"}]}}`)

	request, err := inbound.TransformRequest(ctx, &httpclient.Request{
		Body:    raw,
		Headers: http.Header{"Content-Type": []string{"application/json"}},
	})
	require.NoError(t, err)
	require.Equal(t, llm.RequestTypeAlphaSearch, request.RequestType)
	require.Equal(t, raw, request.AlphaSearch.Body)

	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://cpa.example/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)
	providerRequest, err := outbound.TransformRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "POST", providerRequest.Method)
	require.Equal(t, "https://cpa.example/v1/alpha/search", providerRequest.URL)
	require.JSONEq(t, `{"id":"session-123","model":"client-model","commands":{"search_query":[{"q":"golang"}]}}`, string(providerRequest.Body))

	providerResponse, err := outbound.TransformResponse(ctx, &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"results":[{"title":"Go"}]}`),
		Request:    providerRequest,
	})
	require.NoError(t, err)
	clientResponse, err := inbound.TransformResponse(ctx, providerResponse)
	require.NoError(t, err)
	require.JSONEq(t, `{"results":[{"title":"Go"}]}`, string(clientResponse.Body))
}

func TestAlphaSearchInboundRequiresModel(t *testing.T) {
	_, err := NewAlphaSearchInboundTransformer().TransformRequest(context.Background(), &httpclient.Request{
		Body: []byte(`{"commands":{"search_query":[]}}`),
	})
	require.Error(t, err)
}
