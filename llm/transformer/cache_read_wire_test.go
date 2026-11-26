// 职责: 验证公开协议响应和流事件通过原生字段表达缓存读数。
// 关联文件:
//
//	llm/transformer/openai/usage.go
//	llm/transformer/openai/responses/usage_json.go
//	llm/transformer/anthropic/usage_json.go
//	llm/transformer/gemini/usage_json.go
package transformer_test

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestPublicCacheReadUsageJSON(t *testing.T) {
	for _, protocol := range []struct {
		name                        string
		inbound                     transformer.Inbound
		usage, streamUsage, counter string
	}{
		{"chat", openai.NewInboundTransformer(), "usage", "usage", "prompt_tokens_details.cached_tokens"},
		{"responses", responses.NewInboundTransformer(), "usage", "response.usage", "input_tokens_details.cached_tokens"},
		{"responses_websocket", responses.NewWebSocketInboundTransformer(), "usage", "response.usage", "input_tokens_details.cached_tokens"},
		{"anthropic", anthropic.NewInboundTransformer(), "usage", "usage", "cache_read_input_tokens"},
		{"gemini", gemini.NewInboundTransformer(), "usageMetadata", "usageMetadata", "cachedContentTokenCount"},
	} {
		t.Run(protocol.name, func(t *testing.T) {
			for _, tt := range []struct {
				name   string
				status llm.CacheReadStatus
				count  int64
			}{
				{"reported_zero", llm.CacheReadReported, 0},
				{"reported_positive", llm.CacheReadReported, 3},
				{"unknown", llm.CacheReadUnknown, 0},
				{"unknown_with_legacy_count", "", 3},
			} {
				t.Run(tt.name, func(t *testing.T) {
					usage := &llm.Usage{
						CacheReadStatus:  tt.status,
						PromptTokens:     10,
						CompletionTokens: 2,
						TotalTokens:      12,
						PromptTokensDetails: &llm.PromptTokensDetails{
							CachedTokens: tt.count,
						},
					}
					assertCount := func(wireUsage gjson.Result) {
						t.Helper()
						require.True(t, wireUsage.IsObject())
						count := wireUsage.Get(protocol.counter)
						if tt.status == llm.CacheReadReported {
							require.Equal(t, gjson.Number, count.Type)
							require.Equal(t, tt.count, count.Int())
						} else {
							require.False(t, count.Exists())
						}
					}
					message := &llm.Message{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}
					resp, err := protocol.inbound.TransformResponse(t.Context(), &llm.Response{
						ID: "response", Model: "model", Usage: usage,
						Choices: []llm.Choice{{Message: message, FinishReason: lo.ToPtr("stop")}},
					})
					require.NoError(t, err)
					require.NotContains(t, string(resp.Body), `"cache_read_status"`)
					assertCount(gjson.GetBytes(resp.Body, protocol.usage))

					stream, err := protocol.inbound.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
						{ID: "response", Model: "model", Choices: []llm.Choice{{Delta: message}}},
						{ID: "response", Model: "model", Choices: []llm.Choice{{Delta: &llm.Message{}, FinishReason: lo.ToPtr("stop")}}},
						{ID: "response", Model: "model", Choices: []llm.Choice{}, Usage: usage},
					}))
					require.NoError(t, err)
					events, err := streams.All(stream)
					require.NoError(t, err)
					var final gjson.Result
					for _, event := range events {
						require.NotContains(t, string(event.Data), `"cache_read_status"`)
						if seen := gjson.GetBytes(event.Data, protocol.streamUsage); seen.IsObject() {
							final = seen
						}
					}
					assertCount(final)
					body, meta, err := protocol.inbound.AggregateStreamChunks(t.Context(), events)
					require.NoError(t, err)
					require.NotContains(t, string(body), `"cache_read_status"`)
					assertCount(gjson.GetBytes(body, protocol.usage))
					require.Equal(t, usage.CacheReadReporting(), meta.Usage.CacheReadReporting())
				})
			}
		})
	}
}
