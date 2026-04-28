package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestOutboundTransformer_TransformCompactRequest(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("valid compact request with input", func(t *testing.T) {
		llmReq := &llm.Request{
			Model:       "gpt-4o",
			RequestType: llm.RequestTypeCompact,
			Compact: &llm.CompactRequest{
				Input: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, llmReq)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		assert.Equal(t, http.MethodPost, httpReq.Method)
		assert.Contains(t, httpReq.URL, "/responses/compact")
		assert.Equal(t, string(llm.RequestTypeCompact), httpReq.RequestType)
		assert.Equal(t, string(llm.APIFormatOpenAIResponseCompact), httpReq.APIFormat)

		var payload CompactAPIRequest

		err = json.Unmarshal(httpReq.Body, &payload)
		require.NoError(t, err)
		assert.Equal(t, "gpt-4o", payload.Model)
		require.Len(t, payload.Input.Items, 1)
		assert.Equal(t, "message", payload.Input.Items[0].Type)
		assert.Equal(t, "user", payload.Input.Items[0].Role)
	})

	t.Run("valid compact request with instructions", func(t *testing.T) {
		llmReq := &llm.Request{
			Model:       "gpt-4o",
			RequestType: llm.RequestTypeCompact,
			Compact: &llm.CompactRequest{
				Instructions: "Be concise",
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, llmReq)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		var payload CompactAPIRequest

		err = json.Unmarshal(httpReq.Body, &payload)
		require.NoError(t, err)
		assert.Equal(t, "gpt-4o", payload.Model)
		assert.Equal(t, "Be concise", payload.Instructions)
	})

	t.Run("compact request with all fields", func(t *testing.T) {
		llmReq := &llm.Request{
			Model:       "gpt-4o",
			RequestType: llm.RequestTypeCompact,
			Compact: &llm.CompactRequest{
				Input: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
				Instructions:   "Be concise",
				PromptCacheKey: "cache_key_1",
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, llmReq)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		var payload CompactAPIRequest

		err = json.Unmarshal(httpReq.Body, &payload)
		require.NoError(t, err)
		assert.Equal(t, "gpt-4o", payload.Model)
		assert.Equal(t, "Be concise", payload.Instructions)
		assert.Equal(t, "cache_key_1", payload.PromptCacheKey)
	})

	t.Run("nil compact request", func(t *testing.T) {
		llmReq := &llm.Request{
			Model:       "gpt-4o",
			RequestType: llm.RequestTypeCompact,
		}

		_, err := transformer.TransformRequest(ctx, llmReq)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "compact request is nil")
	})
}

func TestOutboundTransformer_TransformCompactRequest_AccountIdentityFootprint(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:         "https://api.openai.com/v1",
		APIKeyProvider:  auth.NewStaticKeyProvider("test-key"),
		AccountIdentity: "channel-1",
	})
	require.NoError(t, err)

	llmReq := &llm.Request{
		Model:       "gpt-4o",
		RequestType: llm.RequestTypeCompact,
		Compact: &llm.CompactRequest{
			Instructions: "Be concise",
		},
	}

	httpReq, err := transformer.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.NotNil(t, httpReq)
	require.NotNil(t, httpReq.Metadata)
	require.Equal(t, transformer.config.BaseURL, httpReq.Metadata[shared.MetadataKeyBaseURL])
	require.Equal(t, "channel-1", httpReq.Metadata[shared.MetadataKeyAccountIdentity])
}

func TestOutboundTransformer_TransformCompactResponse(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("valid compact response", func(t *testing.T) {
		respBody := `{
			"id": "resp_001",
			"object": "response.compaction",
			"created_at": 1764967971,
			"model": "gpt-4o",
			"instructions": "Be concise",
			"output": [
				{"id":"rs_001","type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"Reasoning summary"}],"encrypted_content":"gAAAAAB..."},
				{"id":"msg_000","type":"message","status":"completed","content":[{"type":"output_text","text":"Hello"}],"role":"assistant"}
			],
			"usage": {
				"input_tokens": 139,
				"input_tokens_details": {"cached_tokens": 0},
				"output_tokens": 438,
				"output_tokens_details": {"reasoning_tokens": 64},
				"total_tokens": 577
			}
		}`

		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(respBody),
			Request: &httpclient.Request{
				RequestType: string(llm.RequestTypeCompact),
			},
		}

		llmResp, err := transformer.TransformResponse(ctx, httpResp)
		require.NoError(t, err)
		require.NotNil(t, llmResp)

		assert.Equal(t, llm.RequestTypeCompact, llmResp.RequestType)
		assert.Equal(t, llm.APIFormatOpenAIResponseCompact, llmResp.APIFormat)
		assert.Equal(t, "gpt-4o", llmResp.Model)
		assert.Empty(t, llmResp.Choices)

		require.NotNil(t, llmResp.Compact)
		assert.Equal(t, "resp_001", llmResp.Compact.ID)
		assert.Equal(t, int64(1764967971), llmResp.Compact.CreatedAt)
		assert.Equal(t, "response.compaction", llmResp.Compact.Object)
		assert.Equal(t, "Be concise", llmResp.Compact.Instructions)
		require.Len(t, llmResp.Compact.Output, 1)
		assert.Equal(t, "msg_000", llmResp.Compact.Output[0].ID)
		assert.Equal(t, "assistant", llmResp.Compact.Output[0].Role)
		require.NotNil(t, llmResp.Compact.Output[0].Content.Content)
		assert.Equal(t, "Hello", *llmResp.Compact.Output[0].Content.Content)
		require.NotNil(t, llmResp.Compact.Output[0].ReasoningContent)
		assert.Equal(t, "Reasoning summary", *llmResp.Compact.Output[0].ReasoningContent)
		require.NotNil(t, llmResp.Compact.Output[0].ReasoningSignature)
		assert.Equal(t, "gAAAAAB...", *llmResp.Compact.Output[0].ReasoningSignature)

		require.NotNil(t, llmResp.Usage)
		assert.Equal(t, int64(139), llmResp.Usage.PromptTokens)
		assert.Equal(t, int64(438), llmResp.Usage.CompletionTokens)
		assert.Equal(t, int64(577), llmResp.Usage.TotalTokens)
		require.NotNil(t, llmResp.Usage.CompletionTokensDetails)
		assert.Equal(t, int64(64), llmResp.Usage.CompletionTokensDetails.ReasoningTokens)
	})

	t.Run("error response", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusBadRequest,
			Body:       []byte(`{"error":{"message":"Invalid model","type":"invalid_request_error"}}`),
			Request: &httpclient.Request{
				RequestType: string(llm.RequestTypeCompact),
			},
		}

		_, err := transformer.TransformResponse(ctx, httpResp)
		require.Error(t, err)
	})

	t.Run("empty body", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte{},
			Request: &httpclient.Request{
				RequestType: string(llm.RequestTypeCompact),
			},
		}

		_, err := transformer.TransformResponse(ctx, httpResp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "response body is empty")
	})
}

func TestOutboundTransformer_BuildCompactURL(t *testing.T) {
	t.Run("standard URL", func(t *testing.T) {
		transformer, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
		require.NoError(t, err)

		url := transformer.buildCompactURL()
		assert.Equal(t, "https://api.openai.com/v1/responses/compact", url)
	})

	t.Run("raw URL", func(t *testing.T) {
		transformer, err := NewOutboundTransformerWithConfig(&Config{
			BaseURL:        "https://custom-api.example.com/v1##",
			APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
		})
		require.NoError(t, err)

		// Raw URL mode returns base URL as-is (the user controls the full URL)
		url := transformer.buildCompactURL()
		assert.Equal(t, "https://custom-api.example.com/v1", url)
	})
}
