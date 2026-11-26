package responses

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
)

func (u Usage) MarshalJSON() ([]byte, error) {
	type usageAlias Usage
	type detailsAlias inputTokenDetails
	type details struct {
		detailsAlias
		CachedTokens *int64 `json:"cached_tokens,omitempty"`
	}
	return json.Marshal(struct {
		usageAlias
		InputTokenDetails details `json:"input_tokens_details"`
	}{
		usageAlias: usageAlias(u),
		InputTokenDetails: details{
			detailsAlias: detailsAlias(u.InputTokenDetails),
			CachedTokens: u.CacheReadStatus.ReportedCount(u.InputTokenDetails.CachedTokens),
		},
	})
}

func (u *Usage) UnmarshalJSON(data []byte) error {
	type alias Usage
	aux := struct {
		*alias

		Cost json.RawMessage `json:"cost"`
	}{alias: (*alias)(u)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	u.Cost = xjson.ParseOptionalFloat64(aux.Cost)
	u.CacheReadStatus = llm.ParseCacheReadStatus(data, []string{"input_tokens_details", "cached_tokens"})
	return nil
}
