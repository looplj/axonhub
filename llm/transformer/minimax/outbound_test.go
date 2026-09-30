package minimax

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestImageGeneration_roundTrip_whenModelIsOmitted(t *testing.T) {
	// Given an OpenAI image request without a model and a MiniMax HTTP endpoint.
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/image_generation", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		var body struct {
			Model          string `json:"model"`
			ResponseFormat string `json:"response_format"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "image-01", body.Model)
		require.Equal(t, "base64", body.ResponseFormat)
		_, _ = w.Write([]byte(`{"id":"minimax-trace","base_resp":{"status_code":0},"data":{"image_base64":["YWJj"]}}`))
	}))
	defer server.Close()
	inbound := openai.NewImageGenerationInboundTransformer()
	req, err := inbound.TransformRequest(ctx, &httpclient.Request{Headers: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(`{"prompt":"cat","response_format":"b64_json"}`)})
	require.NoError(t, err)
	outbound, err := NewOutboundTransformer(server.URL, "test-key")
	require.NoError(t, err)
	request, err := outbound.TransformRequest(ctx, req)
	require.NoError(t, err)

	// When the actual outbound request is sent and decoded.
	response, err := httpclient.NewHttpClient().Do(ctx, request)
	require.NoError(t, err)
	image, err := outbound.TransformResponse(ctx, response)
	require.NoError(t, err)
	clientResponse, err := inbound.TransformResponse(ctx, image)
	require.NoError(t, err)

	// Then the provider ID and OpenAI base64 image survive the round trip.
	require.Equal(t, "minimax-trace", image.ID)
	var result struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(clientResponse.Body, &result))
	require.Equal(t, "YWJj", result.Data[0].B64JSON)
}

func TestImageGeneration_preservesModel_whenExplicitOrMapped(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		model string
	}{
		{"explicit model", `{"model":"dall-e-2","prompt":"cat"}`, "dall-e-2"},
		{"mapped model", `{"prompt":"cat"}`, "image-01-live"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given an inbound request with a selected channel model.
			ctx := context.Background()
			inbound := openai.NewImageGenerationInboundTransformer()
			req, err := inbound.TransformRequest(ctx, &httpclient.Request{Headers: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(tc.body)})
			require.NoError(t, err)
			req.Model = tc.model
			outbound, err := NewOutboundTransformer("https://api.minimax.io", "key")
			require.NoError(t, err)

			// When the MiniMax request is constructed.
			request, err := outbound.TransformRequest(ctx, req)
			require.NoError(t, err)

			// Then the selected model is forwarded intact.
			var payload struct {
				Model string `json:"model"`
			}
			require.NoError(t, json.Unmarshal(request.Body, &payload))
			require.Equal(t, tc.model, payload.Model)
		})
	}
}

func TestImageGeneration_mapsSize_whenDimensionsAreAbsent(t *testing.T) {
	// Given an OpenAI size and optional MiniMax dimension overrides.
	for _, tc := range []struct {
		name, body    string
		width, height int64
	}{
		{"size only", `{"prompt":"cat","size":"1536x1024"}`, 1536, 1024},
		{"explicit width", `{"prompt":"cat","size":"1536x1024","width":768}`, 768, 1024},
		{"explicit dimensions", `{"prompt":"cat","size":"1536x1024","width":768,"height":1280}`, 768, 1280},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			inbound := openai.NewImageGenerationInboundTransformer()
			req, err := inbound.TransformRequest(ctx, &httpclient.Request{Headers: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(tc.body)})
			require.NoError(t, err)
			outbound, err := NewOutboundTransformer("https://api.minimax.io", "key")
			require.NoError(t, err)

			// When the provider request is built.
			request, err := outbound.TransformRequest(ctx, req)

			// Then the provider receives the requested dimensions.
			require.NoError(t, err)
			var payload struct {
				Width  int64 `json:"width"`
				Height int64 `json:"height"`
			}
			require.NoError(t, json.Unmarshal(request.Body, &payload))
			require.Equal(t, tc.width, payload.Width)
			require.Equal(t, tc.height, payload.Height)
		})
	}
}

func TestImageGeneration_rejectsUnsupportedSize_whenDimensionsAreAbsent(t *testing.T) {
	// Given sizes that cannot be sent as MiniMax dimensions.
	for _, size := range []string{"auto", "bad", "511x1024", "1025x1024", "2048x2056"} {
		t.Run(size, func(t *testing.T) {
			outbound, err := NewOutboundTransformer("https://api.minimax.io", "key")
			require.NoError(t, err)

			// When the provider request is built.
			request, err := outbound.TransformRequest(context.Background(), &llm.Request{
				RequestType: llm.RequestTypeImage, Image: &llm.ImageRequest{Prompt: "cat", Size: size},
			})

			// Then unsupported sizes fail before sending the request.
			require.Nil(t, request)
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		})
	}
}

func TestImageGeneration_usesVersionedCustomPath_whenBaseIsHost(t *testing.T) {
	for _, tc := range []struct {
		name, baseURL, path, want string
	}{
		{"versioned path", "https://api.minimax.io", "/v1/image_generation", "https://api.minimax.io/v1/image_generation"},
		{"unversioned path", "https://api.minimax.io", "/image_generation", "https://api.minimax.io/v1/image_generation"},
		{"versioned base", "https://api.minimax.io/v1", "/image_generation", "https://api.minimax.io/v1/image_generation"},
		{"both versioned", "https://api.minimax.io/v1", "/v1/image_generation", "https://api.minimax.io/v1/image_generation"},
		{"default path", "https://api.minimax.io", "", "https://api.minimax.io/v1/image_generation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given a MiniMax base URL and optional endpoint path.
			outbound, err := NewOutboundTransformerWithConfig(&Config{
				BaseURL: tc.baseURL, EndpointPath: tc.path,
				APIKeyProvider: auth.NewStaticKeyProvider("key"),
			})
			require.NoError(t, err)

			// When the MiniMax request is constructed.
			request, err := outbound.TransformRequest(context.Background(), &llm.Request{Model: "image-01", RequestType: llm.RequestTypeImage, Image: &llm.ImageRequest{Prompt: "cat"}})
			require.NoError(t, err)

			// Then the endpoint path includes the version once.
			require.Equal(t, tc.want, request.URL)
		})
	}
}

func TestImageGeneration_rejectsBusinessFailure_whenHTTPIsOK(t *testing.T) {
	// Given an HTTP 200 response with a MiniMax error code.
	transformer, err := NewOutboundTransformer("https://api.minimax.io", "key")
	require.NoError(t, err)
	response := &httpclient.Response{StatusCode: http.StatusOK, Request: &httpclient.Request{RequestType: llm.RequestTypeImage.String()}, Body: []byte(`{"base_resp":{"status_code":1008,"status_msg":"insufficient balance"},"data":{"image_urls":["https://image.example/test.png"]}}`)}

	// When the provider response is decoded.
	image, err := transformer.TransformResponse(context.Background(), response)

	// Then the business error is reported instead of returning the image.
	require.Nil(t, image)
	require.ErrorContains(t, err, "1008")
}
