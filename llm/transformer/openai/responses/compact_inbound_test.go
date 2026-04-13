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
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestCompactInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewCompactInboundTransformer()
	ctx := context.Background()

	t.Run("valid request with input", func(t *testing.T) {
		reqBody, _ := json.Marshal(CompactAPIRequest{
			Model: "gpt-4o",
			Input: Input{Text: lo.ToPtr("Hello")},
		})

		httpReq := &httpclient.Request{
			Body:    reqBody,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		}

		result, err := transformer.TransformRequest(ctx, httpReq)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "gpt-4o", result.Model)
		assert.Equal(t, llm.RequestTypeCompact, result.RequestType)
		assert.Equal(t, llm.APIFormatOpenAIResponseCompact, result.APIFormat)
		require.NotNil(t, result.Compact)
		require.Len(t, result.Compact.Input, 1)
		assert.Equal(t, "user", result.Compact.Input[0].Role)
		require.NotNil(t, result.Compact.Input[0].Content.Content)
		assert.Equal(t, "Hello", *result.Compact.Input[0].Content.Content)
	})

	t.Run("valid request with instructions", func(t *testing.T) {
		reqBody, _ := json.Marshal(CompactAPIRequest{
			Model:        "gpt-4o",
			Instructions: "Be concise",
		})

		httpReq := &httpclient.Request{
			Body:    reqBody,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		}

		result, err := transformer.TransformRequest(ctx, httpReq)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "gpt-4o", result.Model)
		require.NotNil(t, result.Compact)
		assert.Equal(t, "Be concise", result.Compact.Instructions)
	})

	t.Run("valid request with all fields", func(t *testing.T) {
		reqBody, _ := json.Marshal(CompactAPIRequest{
			Model:          "gpt-4o",
			Input:          Input{Text: lo.ToPtr("Hello")},
			Instructions:   "Be concise",
			PromptCacheKey: "cache_key_1",
		})

		httpReq := &httpclient.Request{
			Body:    reqBody,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		}

		result, err := transformer.TransformRequest(ctx, httpReq)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "gpt-4o", result.Model)
		require.NotNil(t, result.Compact)
		assert.Equal(t, "Be concise", result.Compact.Instructions)
		assert.Equal(t, "cache_key_1", result.Compact.PromptCacheKey)
	})

	t.Run("missing model", func(t *testing.T) {
		reqBody, _ := json.Marshal(CompactAPIRequest{
			Instructions: "Be concise",
		})

		httpReq := &httpclient.Request{
			Body:    reqBody,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		}

		_, err := transformer.TransformRequest(ctx, httpReq)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model is required")
	})

	t.Run("nil request", func(t *testing.T) {
		_, err := transformer.TransformRequest(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "http request is nil")
	})

	t.Run("empty body", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Body:    []byte{},
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		}

		_, err := transformer.TransformRequest(ctx, httpReq)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request body is empty")
	})
}

func TestCompactInboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewCompactInboundTransformer()
	ctx := context.Background()

	t.Run("valid compact response", func(t *testing.T) {
		llmResp := &llm.Response{
			RequestType: llm.RequestTypeCompact,
			APIFormat:   llm.APIFormatOpenAIResponseCompact,
			Compact: &llm.CompactResponse{
				ID:           "resp_001",
				CreatedAt:    1764967971,
				Object:       "response.compaction",
				Instructions: "Be concise",
				Output: []llm.Message{{
					ID:                 "msg_001",
					Role:               "assistant",
					Content:            llm.MessageContent{Content: lo.ToPtr("Hello")},
					ReasoningSignature: lo.ToPtr("gAAAAAB..."),
				}},
			},
			Model: "gpt-4o",
			Usage: &llm.Usage{
				PromptTokens:     139,
				CompletionTokens: 438,
				TotalTokens:      577,
			},
		}

		httpResp, err := transformer.TransformResponse(ctx, llmResp)
		require.NoError(t, err)
		require.NotNil(t, httpResp)

		assert.Equal(t, http.StatusOK, httpResp.StatusCode)

		var compactResp CompactAPIResponse
		err = json.Unmarshal(httpResp.Body, &compactResp)
		require.NoError(t, err)

		assert.Equal(t, "resp_001", compactResp.ID)
		assert.Equal(t, int64(1764967971), compactResp.CreatedAt)
		assert.Equal(t, "response.compaction", compactResp.Object)
		assert.Equal(t, "gpt-4o", compactResp.Model)
		assert.Equal(t, "Be concise", compactResp.Instructions)
		require.Len(t, compactResp.Output, 2)
		assert.Equal(t, "reasoning", compactResp.Output[0].Type)
		require.NotNil(t, compactResp.Output[0].EncryptedContent)
		assert.Equal(t, "gAAAAAB...", *compactResp.Output[0].EncryptedContent)
		assert.Equal(t, "message", compactResp.Output[1].Type)
		assert.Equal(t, "msg_001", compactResp.Output[1].ID)
		require.NotNil(t, compactResp.Output[1].Content)
		require.Len(t, compactResp.Output[1].Content.Items, 1)
		assert.Equal(t, "output_text", compactResp.Output[1].Content.Items[0].Type)
		require.NotNil(t, compactResp.Output[1].Content.Items[0].Text)
		assert.Equal(t, "Hello", *compactResp.Output[1].Content.Items[0].Text)
		require.NotNil(t, compactResp.Usage)
		assert.Equal(t, int64(139), compactResp.Usage.InputTokens)
		assert.Equal(t, int64(438), compactResp.Usage.OutputTokens)
		assert.Equal(t, int64(577), compactResp.Usage.TotalTokens)
	})

	t.Run("compact summary response", func(t *testing.T) {
		llmResp := &llm.Response{
			RequestType: llm.RequestTypeCompact,
			APIFormat:   llm.APIFormatOpenAIResponseCompact,
			Compact: &llm.CompactResponse{
				ID:        "resp_002",
				CreatedAt: 1764968000,
				Object:    "response.compaction",
				Output: []llm.Message{{
					ID:   "cmp_msg_001",
					Role: "assistant",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{{
							ID:   "cmp_001",
							Type: "compaction_summary",
							Compact: &llm.CompactContent{
								ID:               "cmp_001",
								EncryptedContent: "encrypted_summary",
							},
						}},
					},
				}},
			},
		}

		httpResp, err := transformer.TransformResponse(ctx, llmResp)
		require.NoError(t, err)

		var compactResp CompactAPIResponse
		err = json.Unmarshal(httpResp.Body, &compactResp)
		require.NoError(t, err)

		require.Len(t, compactResp.Output, 1)
		assert.Equal(t, "compaction_summary", compactResp.Output[0].Type)
		assert.Equal(t, "cmp_001", compactResp.Output[0].ID)
		require.NotNil(t, compactResp.Output[0].EncryptedContent)
		assert.Equal(t, "encrypted_summary", *compactResp.Output[0].EncryptedContent)
	})

	t.Run("nil response", func(t *testing.T) {
		_, err := transformer.TransformResponse(ctx, nil)
		require.Error(t, err)
	})

	t.Run("missing compact data", func(t *testing.T) {
		llmResp := &llm.Response{}
		_, err := transformer.TransformResponse(ctx, llmResp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "compact response missing compact data")
	})
}

func TestCompactInboundTransformer_TransformStream(t *testing.T) {
	transformer := NewCompactInboundTransformer()
	ctx := context.Background()

	_, err := transformer.TransformStream(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compact does not support streaming")
}
