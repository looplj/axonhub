// 职责: 验证 Gemini cachedContentTokenCount 显式零值的可信度和序列化。
// 关联文件:
//
//	llm/transformer/gemini/convert.go
//	llm/transformer/gemini/usage_json.go
package gemini

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/stretchr/testify/require"
)

func TestCacheReadStatusGemini(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		status llm.CacheReadStatus
	}{
		{`{"promptTokenCount":10,"cachedContentTokenCount":0}`, llm.CacheReadReported},
		{`{"promptTokenCount":10,"cachedContentTokenCount":3}`, llm.CacheReadReported},
		{`{"promptTokenCount":10,"cachedContentTokenCount":null}`, llm.CacheReadUnknown},
		{`{"promptTokenCount":10}`, llm.CacheReadUnknown},
	} {
		var u UsageMetadata
		require.NoError(t, json.Unmarshal([]byte(tc.raw), &u))
		converted := convertToLLMUsage(&u)
		require.Equal(t, tc.status, converted.CacheReadReporting())
		data, err := json.Marshal(convertToGeminiUsage(converted))
		require.NoError(t, err)
		var again UsageMetadata
		require.NoError(t, json.Unmarshal(data, &again))
		require.Equal(t, tc.status, convertToLLMUsage(&again).CacheReadReporting())
	}
}

func TestCacheReadStatusGeminiAggregate(t *testing.T) {
	for _, final := range []string{
		`"promptTokenCount":10,"candidatesTokenCount":2`,
		`"promptTokenCount":10,"candidatesTokenCount":2,"cachedContentTokenCount":0`,
	} {
		chunks := []*httpclient.StreamEvent{
			{Data: []byte(`{"responseId":"s","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"x"}]}}],"usageMetadata":{"promptTokenCount":10,"cachedContentTokenCount":0}}`)},
			{Data: []byte(`{"responseId":"s","candidates":[{"index":0,"finishReason":"STOP"}],"usageMetadata":{` + final + `}}`)},
		}
		body, meta, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)
		require.Equal(t, llm.CacheReadReported, meta.Usage.CacheReadReporting())
		var response GenerateContentResponse
		require.NoError(t, json.Unmarshal(body, &response))
		require.Equal(t, llm.CacheReadReported, convertToLLMUsage(response.UsageMetadata).CacheReadReporting())
	}
}

func TestCacheReadStatusGeminiNegative(t *testing.T) {
	var usage UsageMetadata
	require.NoError(t, json.Unmarshal([]byte(`{"promptTokenCount":10,"cachedContentTokenCount":-1}`), &usage))
	converted := convertToLLMUsage(&usage)
	require.Equal(t, llm.CacheReadReported, converted.CacheReadReporting())
	require.NotNil(t, converted.PromptTokensDetails)
	require.Equal(t, int64(-1), converted.PromptTokensDetails.CachedTokens)
	wire, err := json.Marshal(convertToGeminiUsage(converted))
	require.NoError(t, err)
	var restored UsageMetadata
	require.NoError(t, json.Unmarshal(wire, &restored))
	again := convertToLLMUsage(&restored)
	require.Equal(t, llm.CacheReadReported, again.CacheReadReporting())
	require.Equal(t, int64(-1), again.PromptTokensDetails.CachedTokens)
}
