// 职责: 验证统一图片用量回退路径保留缓存读数可信度和显式零值。
// 关联文件:
//
//	llm/image.go
//	llm/transformer/openai/image_inbound.go
//	llm/transformer/openai/image_outbound.go
//
// 注意事项: 原生图片 JSON 不新增可信度字段；统一用量回退必须显式携带 unknown。
package openai

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestImageFallbackCacheReadStatusRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  llm.CacheReadStatus
		details *llm.PromptTokensDetails
		cached  int64
	}{
		{"reported zero no details", llm.CacheReadReported, nil, 0},
		{"reported zero with details", llm.CacheReadReported, &llm.PromptTokensDetails{}, 0},
		{"reported positive", llm.CacheReadReported, &llm.PromptTokensDetails{CachedTokens: 3}, 3},
		{"reported negative", llm.CacheReadReported, &llm.PromptTokensDetails{CachedTokens: -1}, -1},
		{"unknown no details", llm.CacheReadUnknown, nil, 0},
		{"unknown zero details", llm.CacheReadUnknown, &llm.PromptTokensDetails{}, 0},
		{"unknown positive", llm.CacheReadUnknown, &llm.PromptTokensDetails{CachedTokens: 3}, 3},
		{"legacy unknown positive", "", &llm.PromptTokensDetails{CachedTokens: 3}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := &llm.Response{
				Image: &llm.ImageResponse{Created: 7},
				Usage: &llm.Usage{CacheReadStatus: tc.status, PromptTokens: 10, CompletionTokens: 2,
					TotalTokens: 12, PromptTokensDetails: tc.details},
			}
			encoded, err := NewImageGenerationInboundTransformer().TransformResponse(t.Context(), source)
			require.NoError(t, err)
			var wire ImagesResponse
			require.NoError(t, json.Unmarshal(encoded.Body, &wire))
			require.Equal(t, source.Usage.CacheReadReporting(), wire.Usage.CacheReadStatus)
			if tc.status == llm.CacheReadReported {
				require.NotNil(t, wire.Usage.InputTokensDetails)
				require.NotNil(t, wire.Usage.InputTokensDetails.CachedTokens)
				require.Equal(t, tc.cached, *wire.Usage.InputTokensDetails.CachedTokens)
			}
			outbound, err := NewOutboundTransformer("https://example.invalid/v1", "test-key")
			require.NoError(t, err)
			result, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
				StatusCode: http.StatusOK, Body: encoded.Body,
				Request: &httpclient.Request{APIFormat: string(llm.APIFormatOpenAIImageGeneration)},
			})
			require.NoError(t, err)
			require.Equal(t, source.Usage.CacheReadReporting(), result.Usage.CacheReadReporting())
			var cached int64
			if result.Usage.PromptTokensDetails != nil {
				cached = result.Usage.PromptTokensDetails.CachedTokens
			}
			require.Equal(t, tc.cached, cached)
			require.Equal(t, int64(10), result.Usage.PromptTokens)
			require.Equal(t, int64(12), result.Usage.TotalTokens)
		})
	}
}

func TestImageCacheReadExplicitUnknownOverridesNumericField(t *testing.T) {
	for _, status := range []string{`"unknown"`, `null`, `"unsupported"`} {
		outbound, err := NewOutboundTransformer("https://example.invalid/v1", "test-key")
		require.NoError(t, err)
		result, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
			StatusCode: http.StatusOK,
			Body: []byte(`{"created":7,"data":[],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12,` +
				`"cache_read_status":` + status + `,"input_tokens_details":{"cached_tokens":3}}}`),
			Request: &httpclient.Request{APIFormat: string(llm.APIFormatOpenAIImageGeneration)},
		})
		require.NoError(t, err)
		require.Equal(t, llm.CacheReadUnknown, result.Usage.CacheReadReporting())
		require.Equal(t, int64(3), result.Usage.PromptTokensDetails.CachedTokens)
	}
}
