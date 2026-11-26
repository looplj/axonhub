// 职责: 验证 Responses API 的缓存零值和缺失不会在转换时混淆。
// 关联文件:
//
//	llm/transformer/openai/responses/usage.go
//	llm/transformer/openai/responses/usage_json.go
package responses

import (
	"encoding/json"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/stretchr/testify/require"
)

func TestCacheReadStatusResponses(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		status llm.CacheReadStatus
	}{
		{`{"input_tokens":10,"input_tokens_details":{"cached_tokens":0}}`, llm.CacheReadReported},
		{`{"input_tokens":10,"input_tokens_details":{"cache_write_tokens":2}}`, llm.CacheReadUnknown},
		{`{"input_tokens":10}`, llm.CacheReadUnknown},
	} {
		var u Usage
		require.NoError(t, json.Unmarshal([]byte(tc.raw), &u))
		converted := u.ToUsage()
		require.Equal(t, tc.status, converted.CacheReadReporting())
		data, err := json.Marshal(ConvertLLMUsageToResponsesUsage(converted))
		require.NoError(t, err)
		var again Usage
		require.NoError(t, json.Unmarshal(data, &again))
		require.Equal(t, tc.status, again.ToUsage().CacheReadReporting())
	}
}

func TestCacheReadStatusResponsesNegative(t *testing.T) {
	var usage Usage
	require.NoError(t, json.Unmarshal([]byte(`{"input_tokens":10,"input_tokens_details":{"cached_tokens":-1}}`), &usage))
	converted := usage.ToUsage()
	require.Equal(t, llm.CacheReadReported, converted.CacheReadReporting())
	require.Equal(t, int64(-1), converted.PromptTokensDetails.CachedTokens)
	wire, err := json.Marshal(ConvertLLMUsageToResponsesUsage(converted))
	require.NoError(t, err)
	var restored Usage
	require.NoError(t, json.Unmarshal(wire, &restored))
	require.Equal(t, llm.CacheReadReported, restored.ToUsage().CacheReadReporting())
	require.Equal(t, int64(-1), restored.ToUsage().PromptTokensDetails.CachedTokens)
}
