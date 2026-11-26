// 职责: 验证 Anthropic cache_read_input_tokens 缺失、零值和流式聚合。
// 关联文件:
//
//	llm/transformer/anthropic/usage.go
//	llm/transformer/anthropic/aggregator.go
//	llm/transformer/anthropic/outbound_stream.go
package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/stretchr/testify/require"
)

func TestCacheReadStatusAnthropic(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		status llm.CacheReadStatus
	}{
		{`{"input_tokens":10,"cache_read_input_tokens":0}`, llm.CacheReadReported},
		{`{"input_tokens":10,"cached_tokens":0}`, llm.CacheReadReported},
		{`{"input_tokens":10,"cache_read_input_tokens":null}`, llm.CacheReadUnknown},
		{`{"input_tokens":10,"cache_creation_input_tokens":2}`, llm.CacheReadUnknown},
		{`{"input_tokens":10}`, llm.CacheReadUnknown},
	} {
		var u Usage
		require.NoError(t, json.Unmarshal([]byte(tc.raw), &u))
		converted := convertToLlmUsage(&u, PlatformDirect)
		require.Equal(t, tc.status, converted.CacheReadReporting())
		data, err := json.Marshal(convertToAnthropicUsage(converted))
		require.NoError(t, err)
		var again Usage
		require.NoError(t, json.Unmarshal(data, &again))
		require.Equal(t, tc.status, convertToLlmUsage(&again, PlatformDirect).CacheReadReporting())
	}
}

func TestCacheReadStatusAnthropicAggregate(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"message_start","message":{"id":"s","role":"assistant","content":[],"usage":{"input_tokens":10,"output_tokens":0,"cache_read_input_tokens":0}}}`)},
		{Data: []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`)},
		{Data: []byte(`{"type":"message_stop"}`)},
	}
	_, meta, err := AggregateStreamChunks(context.Background(), chunks, PlatformDirect)
	require.NoError(t, err)
	require.Equal(t, llm.CacheReadReported, meta.Usage.CacheReadReporting())
}

func TestCacheReadStatusAnthropicAggregateExplicitZero(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"message_start","message":{"id":"s","role":"assistant","content":[],"usage":{"input_tokens":10,"output_tokens":0,"cached_tokens":3}}}`)},
		{Data: []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2,"cache_read_input_tokens":0}}`)},
		{Data: []byte(`{"type":"message_stop"}`)},
	}
	_, meta, err := AggregateStreamChunks(context.Background(), chunks, PlatformDirect)
	require.NoError(t, err)
	require.Equal(t, llm.CacheReadReported, meta.Usage.CacheReadReporting())
	if meta.Usage.PromptTokensDetails != nil {
		require.Zero(t, meta.Usage.PromptTokensDetails.CachedTokens)
	}
}

func TestCacheReadStatusAnthropicNegative(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		platform PlatformType
	}{
		{`{"input_tokens":10,"cache_read_input_tokens":-1}`, PlatformDirect},
		{`{"input_tokens":10,"cached_tokens":-1}`, PlatformMoonshot},
		{`{"input_tokens":10,"cached_tokens":-1,"cache_creation_input_tokens":2}`, PlatformMoonshot},
	} {
		var usage Usage
		require.NoError(t, json.Unmarshal([]byte(tc.raw), &usage))
		converted := convertToLlmUsage(&usage, tc.platform)
		require.Equal(t, llm.CacheReadReported, converted.CacheReadReporting())
		require.NotNil(t, converted.PromptTokensDetails)
		require.Equal(t, int64(-1), converted.PromptTokensDetails.CachedTokens)
		wire, err := json.Marshal(convertToAnthropicUsage(converted))
		require.NoError(t, err)
		var restored Usage
		require.NoError(t, json.Unmarshal(wire, &restored))
		again := convertToLlmUsage(&restored, tc.platform)
		require.Equal(t, llm.CacheReadReported, again.CacheReadReporting())
		require.Equal(t, int64(-1), again.PromptTokensDetails.CachedTokens)
	}
}

func TestCacheReadStatusAnthropicOutboundStream(t *testing.T) {
	for _, tc := range []struct {
		name, first, final string
		status             llm.CacheReadStatus
		cached             int64
	}{
		{"zero then output only", `"cache_read_input_tokens":0`, `"output_tokens":2`, llm.CacheReadReported, 0},
		{"nonzero then output only", `"cache_read_input_tokens":3`, `"output_tokens":2`, llm.CacheReadReported, 3},
		{"explicit zero replaces nonzero", `"cache_read_input_tokens":3`, `"output_tokens":2,"cache_read_input_tokens":0`, llm.CacheReadReported, 0},
		{"alias then explicit zero", `"cached_tokens":3`, `"output_tokens":2,"cache_read_input_tokens":0`, llm.CacheReadReported, 0},
		{"unknown remains unknown", `"cache_creation_input_tokens":2`, `"output_tokens":2`, llm.CacheReadUnknown, 0},
		{"negative survives output only", `"cache_read_input_tokens":-1`, `"output_tokens":2`, llm.CacheReadReported, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformerWithConfig(&Config{
				Type: PlatformDirect, BaseURL: "https://example.invalid", APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
			})
			require.NoError(t, err)
			chunks := []*httpclient.StreamEvent{
				{Type: "message_start", Data: []byte(`{"type":"message_start","message":{"id":"s","model":"model","role":"assistant","content":[],"usage":{"input_tokens":10,"output_tokens":0,` + tc.first + `}}}`)},
				{Type: "message_delta", Data: []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{` + tc.final + `}}`)},
				{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
			}
			stream, err := transformer.TransformStream(t.Context(), nil, streams.SliceStream(chunks))
			require.NoError(t, err)
			responses, err := streams.All(stream)
			require.NoError(t, err)
			require.NotEmpty(t, responses)
			var final *llm.Usage
			for _, response := range responses {
				if response.Usage != nil {
					final = response.Usage
				}
			}
			require.NotNil(t, final)
			require.Equal(t, tc.status, final.CacheReadReporting())
			var cached int64
			if final.PromptTokensDetails != nil {
				cached = final.PromptTokensDetails.CachedTokens
			}
			require.Equal(t, tc.cached, cached)
			require.Equal(t, int64(2), final.CompletionTokens)
			require.Equal(t, final.PromptTokens+2, final.TotalTokens)
			// 后续覆盖不能反向修改已经发出的 message_start usage。
			if tc.name == "explicit zero replaces nonzero" {
				require.Equal(t, int64(3), responses[0].Usage.PromptTokensDetails.CachedTokens)
			}
		})
	}
}
