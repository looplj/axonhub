package requesty

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func newTestTransformer(t *testing.T) *OutboundTransformer {
	t.Helper()

	out, err := NewOutboundTransformer("", "test-key")
	require.NoError(t, err)

	return out.(*OutboundTransformer)
}

func chatResponse(body string) *httpclient.Response {
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(body),
		Request:    &httpclient.Request{APIFormat: string(llm.APIFormatOpenAIChatCompletion)},
	}
}

func TestTransformRequest_UsesDefaultBaseURL(t *testing.T) {
	tr := newTestTransformer(t)

	req, err := tr.TransformRequest(context.Background(), &llm.Request{
		Model:    "openai/gpt-4o-mini",
		Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: new("hi")}}},
	})
	require.NoError(t, err)
	assert.Equal(t, DefaultBaseURL+"/chat/completions", req.URL)
}

func TestTransformResponse_ReasoningSignature(t *testing.T) {
	tr := newTestTransformer(t)

	// Shape returned live by anthropic/claude-sonnet-4-5 with reasoning_effort=low.
	resp, err := tr.TransformResponse(context.Background(), chatResponse(`{
		"id":"rqsty-cmpl-1","object":"chat.completion","created":1,"model":"claude-sonnet-4-5-20250929",
		"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"17 x 23 = 391",
			"reasoning_content":"17 * 20 + 17 * 3 = 391","reasoning_signature":"EuwCCroBCBIYAipA"}}],
		"usage":{"prompt_tokens":46,"completion_tokens":68,"total_tokens":114}}`))
	require.NoError(t, err)
	require.Len(t, resp.Choices, 1)

	msg := resp.Choices[0].Message
	require.NotNil(t, msg)
	assert.Equal(t, "17 x 23 = 391", *msg.Content.Content)
	assert.Equal(t, "17 * 20 + 17 * 3 = 391", *msg.ReasoningContent)
	require.NotNil(t, msg.ReasoningSignature)
	assert.Equal(t, "EuwCCroBCBIYAipA", *msg.ReasoningSignature)
}

func TestTransformResponse_MessageImages(t *testing.T) {
	tr := newTestTransformer(t)

	t.Run("images only", func(t *testing.T) {
		// Shape returned live by google/gemini-3.1-flash-image with modalities ["image","text"].
		resp, err := tr.TransformResponse(context.Background(), chatResponse(`{
			"id":"rqsty-cmpl-2","object":"chat.completion","created":1,"model":"gemini-3.1-flash-image",
			"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","reasoning_signature":"AY89a19",
				"images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgo="}}]}}]}`))
		require.NoError(t, err)

		msg := resp.Choices[0].Message
		require.NotNil(t, msg)
		assert.Nil(t, msg.Content.Content)
		require.Len(t, msg.Content.MultipleContent, 1)
		assert.Equal(t, "image_url", msg.Content.MultipleContent[0].Type)
		assert.Equal(t, "data:image/png;base64,iVBORw0KGgo=", msg.Content.MultipleContent[0].ImageURL.URL)
		assert.Equal(t, "AY89a19", *msg.ReasoningSignature)
	})

	t.Run("text and images", func(t *testing.T) {
		resp, err := tr.TransformResponse(context.Background(), chatResponse(`{
			"id":"rqsty-cmpl-3","object":"chat.completion","created":1,"model":"gemini-3.1-flash-image",
			"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"Here it is",
				"images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}}]}`))
		require.NoError(t, err)

		parts := resp.Choices[0].Message.Content.MultipleContent
		require.Len(t, parts, 2)
		assert.Equal(t, "text", parts[0].Type)
		assert.Equal(t, "Here it is", *parts[0].Text)
		assert.Equal(t, "data:image/png;base64,AAAA", parts[1].ImageURL.URL)
	})
}

func TestTransformStream_ReasoningSignature(t *testing.T) {
	tr := newTestTransformer(t)

	// Live stream order: signature first, then reasoning_content deltas, then content.
	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"c","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","reasoning_signature":"EsgCCroB"}}]}`)},
		{Data: []byte(`{"id":"c","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"reasoning_content":"thinking"}}]}`)},
		{Data: []byte(`{"id":"c","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"391"},"finish_reason":"stop"}]}`)},
		{Data: []byte(`[DONE]`)},
	}

	req := &httpclient.Request{APIFormat: string(llm.APIFormatOpenAIChatCompletion)}
	stream, err := tr.TransformStream(context.Background(), req, streams.SliceStream(events))
	require.NoError(t, err)

	var got []*llm.Response
	for stream.Next() {
		got = append(got, stream.Current())
	}

	require.NoError(t, stream.Err())
	require.Len(t, got, 4)
	assert.Equal(t, "EsgCCroB", *got[0].Choices[0].Delta.ReasoningSignature)
	assert.Equal(t, "thinking", *got[1].Choices[0].Delta.ReasoningContent)
	assert.Nil(t, got[1].Choices[0].Delta.ReasoningSignature)
	assert.Equal(t, "391", *got[2].Choices[0].Delta.Content.Content)
	assert.Equal(t, llm.DoneResponse, got[3])
}

func TestTransformError_Envelope(t *testing.T) {
	tr := newTestTransformer(t)

	tests := []struct {
		name    string
		status  int
		body    string
		message string
		code    string
		errType string
	}{
		{
			name:    "router error",
			status:  http.StatusForbidden,
			body:    `{"error":{"origin":"router","message":"Invalid authorization token"}}`,
			message: "Invalid authorization token",
			code:    "router",
			errType: "api_error",
		},
		{
			name:    "provider error",
			status:  http.StatusBadRequest,
			body:    `{"error":{"origin":"provider","message":"Invalid 'max_output_tokens'"}}`,
			message: "Invalid 'max_output_tokens'",
			code:    "provider",
			errType: "api_error",
		},
		{
			name:    "non Requesty body",
			status:  http.StatusBadGateway,
			body:    `<html>bad gateway</html>`,
			message: http.StatusText(http.StatusBadGateway),
			errType: "api_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respErr := tr.TransformError(context.Background(), &httpclient.Error{StatusCode: tt.status, Body: []byte(tt.body)})
			require.NotNil(t, respErr)
			assert.Equal(t, tt.status, respErr.StatusCode)
			assert.Equal(t, tt.message, respErr.Detail.Message)
			assert.Equal(t, tt.code, respErr.Detail.Code)
			assert.Equal(t, tt.errType, respErr.Detail.Type)
		})
	}
}

func TestTransformStreamChunk_ErrorEnvelope(t *testing.T) {
	tr := newTestTransformer(t)

	_, err := tr.TransformStreamChunk(context.Background(), &httpclient.StreamEvent{
		Data: []byte(`{"error":{"origin":"provider","message":"overloaded"}}`),
	})

	var respErr *llm.ResponseError
	require.ErrorAs(t, err, &respErr)
	assert.Equal(t, "overloaded", respErr.Detail.Message)
	assert.Equal(t, "provider", respErr.Detail.Code)
	assert.Equal(t, "api_error", respErr.Detail.Type)
}
