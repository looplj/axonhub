package gemini

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
)

func (u UsageMetadata) MarshalJSON() ([]byte, error) {
	type alias UsageMetadata
	if u.CacheReadStatus == llm.CacheReadReported && u.CachedContentTokenCount == 0 {
		return json.Marshal(struct {
			alias
			CachedContentTokenCount int64 `json:"cachedContentTokenCount"`
		}{alias: alias(u)})
	}
	return json.Marshal(alias(u))
}

func (u *UsageMetadata) UnmarshalJSON(data []byte) error {
	type alias UsageMetadata
	aux := struct {
		*alias

		Cost json.RawMessage `json:"cost"`
	}{alias: (*alias)(u)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	u.Cost = xjson.ParseOptionalFloat64(aux.Cost)
	u.CacheReadStatus = llm.ParseCacheReadStatus(data, []string{"cachedContentTokenCount"})
	return nil
}
