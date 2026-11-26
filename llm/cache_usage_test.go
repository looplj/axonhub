// 职责: 验证缓存读取可信度显式区分零值、缺失和跨模块序列化。
// 关联文件:
//
//	llm/cache_usage.go
//	llm/model.go
//
// 注意事项:
//
//	旧数据含 details 或正缓存数也不能代替显式可信度元数据。
package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCacheReadStatus(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      CacheReadStatus
	}{
		{"missing", `{}`, CacheReadUnknown},
		{"details only", `{"prompt_tokens_details":{"audio_tokens":4}}`, CacheReadUnknown},
		{"zero", `{"prompt_tokens_details":{"cached_tokens":0}}`, CacheReadReported},
		{"positive", `{"prompt_tokens_details":{"cached_tokens":3}}`, CacheReadReported},
		{"null", `{"prompt_tokens_details":{"cached_tokens":null}}`, CacheReadUnknown},
		{"top level zero", `{"cached_tokens":0}`, CacheReadReported},
		{"wrong type", `{"cached_tokens":"0"}`, CacheReadUnknown},
		{"unknown roundtrip", `{"cache_read_status":"unknown","prompt_tokens_details":{"cached_tokens":0}}`, CacheReadUnknown},
		{"empty roundtrip", `{"cache_read_status":"","cached_tokens":0}`, CacheReadUnknown},
		{"reported roundtrip", `{"cache_read_status":"reported","cached_tokens":0}`, CacheReadReported},
		{"bare reported marker", `{"cache_read_status":"reported"}`, CacheReadUnknown},
		{"reported null", `{"cache_read_status":"reported","cached_tokens":null}`, CacheReadUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ParseCacheReadStatus([]byte(tc.raw), []string{"prompt_tokens_details", "cached_tokens"}, []string{"cached_tokens"}))
		})
	}
}

func TestUsageCacheReadStatusJSON(t *testing.T) {
	for _, status := range []CacheReadStatus{"", CacheReadUnknown, CacheReadReported} {
		u := Usage{CacheReadStatus: status, PromptTokens: 10, PromptTokensDetails: &PromptTokensDetails{CachedTokens: 0}}
		data, err := json.Marshal(u)
		require.NoError(t, err)
		var decoded Usage
		require.NoError(t, json.Unmarshal(data, &decoded))
		require.Equal(t, u.CacheReadReporting(), decoded.CacheReadReporting())
	}
	var old Usage
	require.NoError(t, json.Unmarshal([]byte(`{"prompt_tokens":10,"prompt_tokens_details":{"cached_tokens":4}}`), &old))
	require.Equal(t, CacheReadUnknown, old.CacheReadReporting())
	var nilUsage *Usage
	require.Equal(t, CacheReadUnknown, nilUsage.CacheReadReporting())
}

func TestParseCacheReadStatusDoesNotMergeParentFields(t *testing.T) {
	require.Equal(t, CacheReadUnknown, ParseCacheReadStatus(
		[]byte(`{"cached_tokens":7,"input_tokens_details":{"cache_write_tokens":2}}`),
		[]string{"input_tokens_details", "cached_tokens"}))
}
