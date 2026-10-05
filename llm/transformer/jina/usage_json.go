package jina

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
)

func (u *RerankUsage) UnmarshalJSON(data []byte) error {
	type alias RerankUsage
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

func (u *EmbeddingUsage) UnmarshalJSON(data []byte) error {
	type alias EmbeddingUsage
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
