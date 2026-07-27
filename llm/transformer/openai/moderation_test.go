package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestModerationInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewModerationInboundTransformer()

	t.Run("valid string input with model", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"model": "omni-moderation-latest",
			"input": "hello world",
		})
		require.NoError(t, err)

		llmReq, err := transformer.TransformRequest(context.Background(), &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "omni-moderation-latest", llmReq.Model)
		require.Equal(t, llm.RequestTypeModeration, llmReq.RequestType)
		require.Equal(t, llm.APIFormatOpenAIModeration, llmReq.APIFormat)
		require.NotNil(t, llmReq.Moderation)
		require.Equal(t, "hello world", llmReq.Moderation.Input.String)
	})

	t.Run("defaults model when omitted", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"input": "I want to kill them.",
		})
		require.NoError(t, err)

		llmReq, err := transformer.TransformRequest(context.Background(), &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "omni-moderation-latest", llmReq.Model)
	})

	t.Run("string array input", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"model": "omni-moderation-latest",
			"input": []string{"one", "two"},
		})
		require.NoError(t, err)

		llmReq, err := transformer.TransformRequest(context.Background(), &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"one", "two"}, llmReq.Moderation.Input.StringArray)
	})

	t.Run("multimodal input", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"model": "omni-moderation-latest",
			"input": []map[string]any{
				{"type": "text", "text": "classify this"},
				{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/a.png"}},
			},
		})
		require.NoError(t, err)

		llmReq, err := transformer.TransformRequest(context.Background(), &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		})
		require.NoError(t, err)
		require.Len(t, llmReq.Moderation.Input.Parts, 2)
		require.Equal(t, "text", llmReq.Moderation.Input.Parts[0].Type)
		require.Equal(t, "image_url", llmReq.Moderation.Input.Parts[1].Type)
		require.Equal(t, "https://example.com/a.png", llmReq.Moderation.Input.Parts[1].ImageURL.URL)
	})

	t.Run("empty input rejected", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"model": "omni-moderation-latest",
			"input": "",
		})
		require.NoError(t, err)

		_, err = transformer.TransformRequest(context.Background(), &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "input cannot be empty string")
	})

	t.Run("empty array input rejected", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"model": "omni-moderation-latest",
			"input": []string{},
		})
		require.NoError(t, err)

		_, err = transformer.TransformRequest(context.Background(), &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "input cannot be empty array")
	})

	t.Run("multimodal missing image url rejected", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"model": "omni-moderation-latest",
			"input": []map[string]any{
				{"type": "image_url", "image_url": map[string]any{"url": ""}},
			},
		})
		require.NoError(t, err)

		_, err = transformer.TransformRequest(context.Background(), &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "image_url.url is required")
	})

	t.Run("unsupported content type rejected", func(t *testing.T) {
		_, err := transformer.TransformRequest(context.Background(), &httpclient.Request{
			Body: []byte(`{"input":"x"}`),
			Headers: http.Header{
				"Content-Type": []string{"text/plain"},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported content type")
	})

	t.Run("empty body rejected", func(t *testing.T) {
		_, err := transformer.TransformRequest(context.Background(), &httpclient.Request{
			Body: []byte{},
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "request body is empty")
	})
}

func TestModerationInboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewModerationInboundTransformer()

	httpResp, err := transformer.TransformResponse(context.Background(), &llm.Response{
		ID:    "modr-test",
		Model: "omni-moderation-latest",
		Moderation: &llm.ModerationResponse{
			Results: []llm.ModerationClassification{
				{
					Flagged: true,
					Categories: map[string]bool{
						"violence": true,
					},
					CategoryScores: map[string]float64{
						"violence": 0.9,
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResp.StatusCode)

	var got ModerationCreateResponse
	require.NoError(t, json.Unmarshal(httpResp.Body, &got))
	require.Equal(t, "modr-test", got.ID)
	require.Equal(t, "omni-moderation-latest", got.Model)
	require.True(t, got.Results[0].Flagged)
}

func TestOutboundTransformer_ModerationRoundTrip(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		PlatformType:   PlatformOpenAI,
		BaseURL:        "https://api.openai.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	ot := outbound.(*OutboundTransformer)

	httpReq, err := ot.TransformRequest(context.Background(), &llm.Request{
		Model:       "omni-moderation-latest",
		RequestType: llm.RequestTypeModeration,
		Moderation: &llm.ModerationRequest{
			Input: llm.ModerationInput{String: "hello"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.openai.com/v1/moderations", httpReq.URL)
	require.Equal(t, string(llm.RequestTypeModeration), httpReq.RequestType)
	require.Equal(t, string(llm.APIFormatOpenAIModeration), httpReq.APIFormat)

	var sent ModerationCreateRequest
	require.NoError(t, json.Unmarshal(httpReq.Body, &sent))
	require.Equal(t, "omni-moderation-latest", sent.Model)
	require.Equal(t, "hello", sent.Input.String)

	upstreamBody, err := json.Marshal(ModerationCreateResponse{
		ID:    "modr-1",
		Model: "omni-moderation-latest",
		Results: []llm.ModerationClassification{
			{Flagged: false, Categories: map[string]bool{"violence": false}, CategoryScores: map[string]float64{"violence": 0.01}},
		},
	})
	require.NoError(t, err)

	llmResp, err := ot.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       upstreamBody,
		Request:    httpReq,
	})
	require.NoError(t, err)
	require.Equal(t, "modr-1", llmResp.ID)
	require.NotNil(t, llmResp.Moderation)
	require.False(t, llmResp.Moderation.Results[0].Flagged)
}

func TestOutboundTransformer_ModerationCustomEndpointPath(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		PlatformType:   PlatformOpenAI,
		BaseURL:        "https://gateway.example.com/v1",
		EndpointPath:   "/custom/moderations",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	httpReq, err := outbound.TransformRequest(context.Background(), &llm.Request{
		Model:       "omni-moderation-latest",
		RequestType: llm.RequestTypeModeration,
		Moderation: &llm.ModerationRequest{
			Input: llm.ModerationInput{String: "hello"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://gateway.example.com/v1/custom/moderations", httpReq.URL)
}

func TestOutboundTransformer_ModerationMissingPayload(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		PlatformType:   PlatformOpenAI,
		BaseURL:        "https://api.openai.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	_, err = outbound.TransformRequest(context.Background(), &llm.Request{
		Model:       "omni-moderation-latest",
		RequestType: llm.RequestTypeModeration,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "moderation request is nil")
}

func TestModerationInput_JSON(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		var input llm.ModerationInput
		require.NoError(t, json.Unmarshal([]byte(`"hi"`), &input))
		require.Equal(t, "hi", input.String)

		raw, err := json.Marshal(input)
		require.NoError(t, err)
		require.JSONEq(t, `"hi"`, string(raw))
	})

	t.Run("string array", func(t *testing.T) {
		var input llm.ModerationInput
		require.NoError(t, json.Unmarshal([]byte(`["a","b"]`), &input))
		require.Equal(t, []string{"a", "b"}, input.StringArray)
	})

	t.Run("parts", func(t *testing.T) {
		var input llm.ModerationInput
		require.NoError(t, json.Unmarshal([]byte(`[{"type":"text","text":"x"}]`), &input))
		require.Len(t, input.Parts, 1)
		require.Equal(t, "text", input.Parts[0].Type)
	})
}
