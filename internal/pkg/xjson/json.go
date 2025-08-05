package xjson

import (
	"encoding/json"

	"github.com/looplj/axonhub/internal/objects"
)

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
