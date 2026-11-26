package gemini

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
)

func (u UsageMetadata) MarshalJSON() ([]byte, error) {
	type alias UsageMetadata
	return json.Marshal(struct {
		alias
		CachedContentTokenCount *int64 `json:"cachedContentTokenCount,omitempty"`
	}{alias: alias(u), CachedContentTokenCount: u.CacheReadStatus.ReportedCount(u.CachedContentTokenCount)})
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
