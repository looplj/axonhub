package gemini

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
)

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
	return nil
}
