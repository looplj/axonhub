package biz

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/llm"
)

func TestIsModerationFormat(t *testing.T) {
	require.True(t, isModerationFormat(llm.APIFormatOpenAIModeration))
	require.False(t, isModerationFormat(llm.APIFormatOpenAIChatCompletion))
	require.False(t, isModerationFormat(llm.APIFormatOpenAIEmbedding))
}

func TestExtractSpansFromModerationRequestBody(t *testing.T) {
	t.Run("string input", func(t *testing.T) {
		body := []byte(`{"model":"omni-moderation-latest","input":"hello world"}`)
		spans := extractSpansFromModerationRequestBody(body, "request-1")
		require.Len(t, spans, 1)
		require.Equal(t, "user_query", spans[0].Type)
		require.NotNil(t, spans[0].Value.UserQuery)
		require.Contains(t, spans[0].Value.UserQuery.Text, "hello world")
		require.Contains(t, spans[0].Value.UserQuery.Text, "omni-moderation-latest")
	})

	t.Run("string array input", func(t *testing.T) {
		body := []byte(`{"input":["one","two"]}`)
		spans := extractSpansFromModerationRequestBody(body, "request-1")
		require.Len(t, spans, 1)
		require.Equal(t, "one | two", spans[0].Value.UserQuery.Text)
	})

	t.Run("multimodal input", func(t *testing.T) {
		body := []byte(`{"input":[{"type":"text","text":"check"},{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]}`)
		spans := extractSpansFromModerationRequestBody(body, "request-1")
		require.Len(t, spans, 1)
		require.Contains(t, spans[0].Value.UserQuery.Text, "check")
		require.Contains(t, spans[0].Value.UserQuery.Text, "image:https://example.com/a.png")
	})

	t.Run("invalid json", func(t *testing.T) {
		require.Nil(t, extractSpansFromModerationRequestBody([]byte(`{`), "request-1"))
	})
}

func TestExtractSpansFromModerationResponseBody(t *testing.T) {
	body := []byte(`{
		"id":"modr-1",
		"model":"omni-moderation-latest",
		"results":[{
			"flagged":true,
			"categories":{"violence":true,"hate":false},
			"category_scores":{"violence":0.9,"hate":0.01}
		}]
	}`)
	spans := extractSpansFromModerationResponseBody(body, "response-1")
	require.Len(t, spans, 1)
	require.Equal(t, "text", spans[0].Type)
	require.NotNil(t, spans[0].Value.Text)
	require.Contains(t, spans[0].Value.Text.Text, "flagged=true")
	require.Contains(t, spans[0].Value.Text.Text, "violence")
	require.Contains(t, spans[0].Value.Text.Text, "omni-moderation-latest")
}

func TestRequestToSegment_ModerationFormat(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	now := time.Now()

	reqBody, err := json.Marshal(map[string]any{
		"model": "omni-moderation-latest",
		"input": "I want to kill them.",
	})
	require.NoError(t, err)

	respBody, err := json.Marshal(map[string]any{
		"id":    "modr-1",
		"model": "omni-moderation-latest",
		"results": []map[string]any{{
			"flagged":         true,
			"categories":      map[string]bool{"violence": true, "hate": false},
			"category_scores": map[string]float64{"violence": 0.99},
		}},
	})
	require.NoError(t, err)

	req := &ent.Request{
		ID:           42,
		ModelID:      "omni-moderation-latest",
		Format:       string(llm.APIFormatOpenAIModeration),
		Status:       request.StatusCompleted,
		CreatedAt:    now,
		UpdatedAt:    now,
		RequestBody:  reqBody,
		ResponseBody: respBody,
	}

	segment, err := requestToSegment(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, segment)
	require.Len(t, segment.RequestSpans, 1)
	require.Equal(t, "user_query", segment.RequestSpans[0].Type)
	require.Contains(t, segment.RequestSpans[0].Value.UserQuery.Text, "I want to kill them.")
	require.Len(t, segment.ResponseSpans, 1)
	require.Equal(t, "text", segment.ResponseSpans[0].Type)
	require.Contains(t, segment.ResponseSpans[0].Value.Text.Text, "flagged=true")
}
