// 职责: 验证 OpenAI chat 缓存用量解析和流式聚合保留上报状态。
// 关联文件:
//
//	llm/transformer/openai/usage.go
//	llm/transformer/openai/aggregator.go
package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/stretchr/testify/require"
)

func TestCacheReadStatusChat(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		status llm.CacheReadStatus
	}{
		{`{"prompt_tokens":10,"prompt_tokens_details":{"cached_tokens":0}}`, llm.CacheReadReported},
		{`{"prompt_tokens":10,"cached_tokens":0}`, llm.CacheReadReported},
		{`{"prompt_tokens":10,"prompt_cache_hit_tokens":0}`, llm.CacheReadReported},
		{`{"prompt_tokens":10,"prompt_cache_hit_tokens":3}`, llm.CacheReadReported},
		{`{"prompt_tokens":10,"prompt_tokens_details":{"cached_tokens":null}}`, llm.CacheReadUnknown},
		{`{"prompt_tokens":10,"prompt_tokens_details":{"audio_tokens":2}}`, llm.CacheReadUnknown},
		{`{"prompt_tokens":10}`, llm.CacheReadUnknown},
	} {
		var u Usage
		require.NoError(t, json.Unmarshal([]byte(tc.raw), &u))
		converted := u.ToLLMUsage()
		require.Equal(t, tc.status, converted.CacheReadReporting())
		wire, err := json.Marshal(UsageFromLLM(converted))
		require.NoError(t, err)
		var again Usage
		require.NoError(t, json.Unmarshal(wire, &again))
		require.Equal(t, tc.status, again.ToLLMUsage().CacheReadReporting())
	}
}

func TestCacheReadStatusChatAggregate(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"s","choices":[{"index":0,"delta":{"content":"x"},"finish_reason":null}],"usage":{"prompt_tokens":10,"completion_tokens":1,"prompt_tokens_details":{"cached_tokens":0}}}`)},
		{Data: []byte(`{"id":"s","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)},
		{Data: []byte(`{"id":"s","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2}}`)},
	}
	body, meta, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)
	require.Equal(t, llm.CacheReadReported, meta.Usage.CacheReadReporting())
	var response Response
	require.NoError(t, json.Unmarshal(body, &response))
	require.Equal(t, llm.CacheReadReported, response.Usage.ToLLMUsage().CacheReadReporting())
}

func TestCacheReadStatusChatAggregatePreservesValues(t *testing.T) {
	for _, tc := range []struct {
		name, first, last string
		status            llm.CacheReadStatus
		cached            int64
	}{
		{"same prompt missing final", `"prompt_tokens_details":{"cached_tokens":3}`, `"prompt_tokens":10`, llm.CacheReadReported, 3},
		{"deepseek alias", `"prompt_cache_hit_tokens":3`, `"prompt_tokens":10`, llm.CacheReadReported, 3},
		{"explicit final zero", `"prompt_tokens_details":{"cached_tokens":3}`, `"prompt_tokens":10,"prompt_tokens_details":{"cached_tokens":0}`, llm.CacheReadReported, 0},
		{"changed prompt", `"prompt_tokens_details":{"cached_tokens":3}`, `"prompt_tokens":11`, llm.CacheReadUnknown, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chunks := []*httpclient.StreamEvent{
				{Data: []byte(`{"id":"s","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":1,` + tc.first + `}}`)},
				{Data: []byte(`{"id":"s","choices":[],"usage":{"completion_tokens":2,` + tc.last + `}}`)},
			}
			_, meta, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
			require.NoError(t, err)
			require.Equal(t, tc.status, meta.Usage.CacheReadReporting())
			var cached int64
			if meta.Usage.PromptTokensDetails != nil {
				cached = meta.Usage.PromptTokensDetails.CachedTokens
			}
			require.Equal(t, tc.cached, cached)
		})
	}
}

func TestCacheReadStatusChatNegative(t *testing.T) {
	for _, raw := range []string{
		`{"prompt_tokens":10,"prompt_tokens_details":{"cached_tokens":-1}}`,
		`{"prompt_tokens":10,"cached_tokens":-1}`,
		`{"prompt_tokens":10,"prompt_cache_hit_tokens":-1}`,
	} {
		var usage Usage
		require.NoError(t, json.Unmarshal([]byte(raw), &usage))
		converted := usage.ToLLMUsage()
		require.Equal(t, llm.CacheReadReported, converted.CacheReadReporting())
		require.NotNil(t, converted.PromptTokensDetails)
		require.Equal(t, int64(-1), converted.PromptTokensDetails.CachedTokens)
		wire, err := json.Marshal(UsageFromLLM(converted))
		require.NoError(t, err)
		var restored Usage
		require.NoError(t, json.Unmarshal(wire, &restored))
		again := restored.ToLLMUsage()
		require.Equal(t, llm.CacheReadReported, again.CacheReadReporting())
		require.Equal(t, int64(-1), again.PromptTokensDetails.CachedTokens)
	}
}
