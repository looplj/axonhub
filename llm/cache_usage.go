// 职责: 表达缓存读取用量是否由上游显式上报，并保留跨转换的可信度。
// 关联文件:
//
//	llm/model.go
//	llm/transformer/openai/usage.go
//	internal/server/biz/usage_log.go
//
// 注意事项:
//
//	零值和旧序列化数据均为 unknown；details 对象存在不能证明 cached_tokens 被上报。
package llm

import "encoding/json"

type CacheReadStatus string

const (
	CacheReadUnknown  CacheReadStatus = "unknown"
	CacheReadReported CacheReadStatus = "reported"
)

// MarshalJSON preserves unknown through provider-shaped JSON containing
// synthetic cached_tokens:0 fields, including when status is the old zero value.
func (status CacheReadStatus) MarshalJSON() ([]byte, error) {
	if status == CacheReadReported {
		return json.Marshal(string(CacheReadReported))
	}
	return json.Marshal(string(CacheReadUnknown))
}

// CacheReadReporting normalizes missing/unsupported metadata conservatively.
func (u *Usage) CacheReadReporting() CacheReadStatus {
	if u != nil && u.CacheReadStatus == CacheReadReported {
		return CacheReadReported
	}
	return CacheReadUnknown
}

// MarshalJSON emits an explicit zero when a provider reported it without any
// other prompt details, including in aggregated Chat Completion responses.
func (u Usage) MarshalJSON() ([]byte, error) {
	if u.CacheReadStatus == CacheReadReported && u.PromptTokensDetails == nil {
		u.PromptTokensDetails = &PromptTokensDetails{}
	}
	type usageAlias Usage
	return json.Marshal(usageAlias(u))
}

// ParseCacheReadStatus detects a non-null integer field, including explicit zero.
// Unknown metadata suppresses synthetic counts after a JSON round trip. Reported
// metadata still requires a provider count so a bare marker cannot create a zero.
func ParseCacheReadStatus(data []byte, paths ...[]string) CacheReadStatus {
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return CacheReadUnknown
	}
	if raw, ok := fields["cache_read_status"]; ok {
		var status CacheReadStatus
		if json.Unmarshal(raw, &status) != nil || status != CacheReadReported {
			return CacheReadUnknown
		}
	}
	for _, path := range paths {
		current := fields
		for i, key := range path {
			raw, ok := current[key]
			if !ok || string(raw) == "null" {
				break
			}
			if i == len(path)-1 {
				var value int64
				if json.Unmarshal(raw, &value) == nil {
					return CacheReadReported
				}
			} else {
				var nested map[string]json.RawMessage
				if json.Unmarshal(raw, &nested) != nil {
					break
				}
				current = nested
			}
		}
	}
	return CacheReadUnknown
}
