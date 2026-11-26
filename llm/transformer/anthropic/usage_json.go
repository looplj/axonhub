package anthropic

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
)

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
	u.CacheReadStatus = llm.ParseCacheReadStatus(data, []string{"cache_read_input_tokens"}, []string{"cached_tokens"})
	return nil
}
