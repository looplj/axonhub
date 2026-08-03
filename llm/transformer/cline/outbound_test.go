package cline

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func newTestTransformer(t *testing.T) *OutboundTransformer {
	t.Helper()

	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.cline.bot/api/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	clineTransformer, ok := transformer.(*OutboundTransformer)
	require.True(t, ok)

	return clineTransformer
}

func TestOutboundTransformer_TransformRequest_UsesConfiguredEndpointPath(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.cline.bot/api/v1",
		EndpointPath:   "/custom/chat/completions",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	content := "hello"
	req, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model: "cline-pass/deepseek-v4-flash",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: &content},
		}},
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "https://api.cline.bot/api/v1/custom/chat/completions", req.URL)
}

func TestOutboundTransformer_TransformResponse_UnwrapsClineData(t *testing.T) {
	transformer := newTestTransformer(t)

	resp, err := transformer.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body: []byte(`{
			"success": true,
			"data": {
				"id": "chatcmpl-1",
				"object": "chat.completion",
				"created": 123,
				"model": "cline-pass/deepseek-v4-flash",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "ok"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 2,
					"total_tokens": 12
				}
			}
		}`),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "chatcmpl-1", resp.ID)
	assert.Equal(t, "chat.completion", resp.Object)
	assert.Equal(t, int64(123), resp.Created)
	assert.Equal(t, "cline-pass/deepseek-v4-flash", resp.Model)
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message)
	require.NotNil(t, resp.Choices[0].Message.Content.Content)
	assert.Equal(t, "ok", *resp.Choices[0].Message.Content.Content)
	require.NotNil(t, resp.Choices[0].FinishReason)
	assert.Equal(t, "stop", *resp.Choices[0].FinishReason)
	require.NotNil(t, resp.Usage)
	assert.Equal(t, int64(10), resp.Usage.PromptTokens)
	assert.Equal(t, int64(2), resp.Usage.CompletionTokens)
	assert.Equal(t, int64(12), resp.Usage.TotalTokens)
}

func TestOutboundTransformer_TransformResponse_ReturnsClineErrorEnvelope(t *testing.T) {
	transformer := newTestTransformer(t)

	_, err := transformer.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body: []byte(`{
			"success": false,
			"error": "model not found"
		}`),
	})

	require.Error(t, err)
	var respErr *llm.ResponseError
	require.True(t, errors.As(err, &respErr))
	assert.Equal(t, http.StatusBadGateway, respErr.StatusCode)
	assert.Equal(t, "model not found", respErr.Detail.Message)
	assert.Equal(t, "api_error", respErr.Detail.Type)
}

func TestOutboundTransformer_TransformResponse_UsesErrorsFallbackForClineFailure(t *testing.T) {
	transformer := newTestTransformer(t)

	_, err := transformer.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body: []byte(`{
			"success": false,
			"errors": [{"message": "quota exceeded"}]
		}`),
	})

	require.Error(t, err)
	var respErr *llm.ResponseError
	require.True(t, errors.As(err, &respErr))
	assert.Equal(t, http.StatusBadGateway, respErr.StatusCode)
	assert.Contains(t, respErr.Detail.Message, "quota exceeded")
	assert.Equal(t, "api_error", respErr.Detail.Type)
}

func TestOutboundTransformer_TransformError_ParsesStringError(t *testing.T) {
	transformer := newTestTransformer(t)

	respErr := transformer.TransformError(context.Background(), &httpclient.Error{
		StatusCode: http.StatusNotFound,
		Body:       []byte(`{"success":false,"error":"model not found"}`),
	})

	require.NotNil(t, respErr)
	assert.Equal(t, http.StatusNotFound, respErr.StatusCode)
	assert.Equal(t, "model not found", respErr.Detail.Message)
	assert.Equal(t, "api_error", respErr.Detail.Type)
}

func TestOutboundTransformer_TransformStreamChunk_ReturnsClineInBandError(t *testing.T) {
	transformer := newTestTransformer(t)

	_, err := transformer.TransformStreamChunk(context.Background(), &httpclient.StreamEvent{
		Data: []byte(`{"success":false,"error":"response_format json_object is not supported"}`),
	})

	require.Error(t, err)
	var respErr *llm.ResponseError
	require.True(t, errors.As(err, &respErr))
	assert.Equal(t, "response_format json_object is not supported", respErr.Detail.Message)
}

func TestOutboundTransformer_TransformStreamChunk_UsesErrorsFallbackForClineFailure(t *testing.T) {
	transformer := newTestTransformer(t)

	_, err := transformer.TransformStreamChunk(context.Background(), &httpclient.StreamEvent{
		Data: []byte(`{"success":false,"errors":[{"message":"quota exceeded"}]}`),
	})

	require.Error(t, err)
	var respErr *llm.ResponseError
	require.True(t, errors.As(err, &respErr))
	assert.Equal(t, http.StatusBadGateway, respErr.StatusCode)
	assert.Contains(t, respErr.Detail.Message, "quota exceeded")
}

func TestOutboundTransformer_AggregateStreamChunks_UsesClineReasoningFields(t *testing.T) {
	transformer := newTestTransformer(t)

	data, meta, err := transformer.AggregateStreamChunks(context.Background(), nil, []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":123,"model":"cline-pass/deepseek-v4-flash","choices":[{"index":0,"delta":{"role":"assistant"}}]}`)},
		{Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":123,"model":"cline-pass/deepseek-v4-flash","choices":[{"index":0,"delta":{"reasoning":"think ","reasoning_details":[{"type":"reasoning.text","text":"think "}]}}]}`)},
		{Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":123,"model":"cline-pass/deepseek-v4-flash","choices":[{"index":0,"delta":{"reasoning":"then answer","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`)},
	})

	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-1", meta.ID)
	require.NotNil(t, meta.Usage)
	assert.Equal(t, int64(7), meta.Usage.TotalTokens)

	var resp llm.Response
	require.NoError(t, json.Unmarshal(data, &resp))
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message)
	require.NotNil(t, resp.Choices[0].Message.ReasoningContent)
	assert.Equal(t, "think then answer", *resp.Choices[0].Message.ReasoningContent)
	require.NotNil(t, resp.Choices[0].Message.Content.Content)
	assert.Equal(t, "ok", *resp.Choices[0].Message.Content.Content)
}

func TestOutboundTransformer_AggregateStreamChunks_IgnoresDoneChunk(t *testing.T) {
	transformer := newTestTransformer(t)

	data, meta, err := transformer.AggregateStreamChunks(context.Background(), nil, []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":123,"model":"cline-pass/deepseek-v4-flash","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)},
		{Data: []byte(`[DONE]`)},
	})

	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-1", meta.ID)

	var resp llm.Response
	require.NoError(t, json.Unmarshal(data, &resp))
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message)
	require.NotNil(t, resp.Choices[0].Message.Content.Content)
	assert.Equal(t, "ok", *resp.Choices[0].Message.Content.Content)
}
