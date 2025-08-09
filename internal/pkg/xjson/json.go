package xjson

import (
	"encoding/json"

	"github.com/looplj/axonhub/internal/objects"
)

func To[T any](v []byte) (T, error) {
	var t T

	err := json.Unmarshal(v, &t)
	if err != nil {
		return t, err
	}

	return t, nil
}

func Marshal(v any) (objects.JSONRawMessage, error) {
	switch v := v.(type) {
	case string:
		return objects.JSONRawMessage(v), nil
	case []byte:
		return objects.JSONRawMessage(v), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		return objects.JSONRawMessage(b), nil
	}
}
