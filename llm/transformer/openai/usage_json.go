package openai

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
)

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

func (u *CompletionUsage) UnmarshalJSON(data []byte) error {
	type alias CompletionUsage
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

func (u *ImagesResponseUsage) UnmarshalJSON(data []byte) error {
	type alias ImagesResponseUsage
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

func (u *OpenAIVideoUsage) UnmarshalJSON(data []byte) error {
	type alias OpenAIVideoUsage
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
