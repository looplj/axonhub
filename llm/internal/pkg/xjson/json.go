package xjson

import (
	"bytes"
	"encoding/json"
)

func MustMarshalString(v any) string {
	return string(MustMarshal(v))
}

func MustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return b
}

func MustTo[T any](v []byte) T {
	t, err := To[T](v)
	if err != nil {
		panic(err)
	}

	return t
}

func To[T any](v []byte) (T, error) {
	var t T

	err := json.Unmarshal(v, &t)
	if err != nil {
		return t, err
	}

	return t, nil
}

func IsNull(v json.RawMessage) bool {
	return len(v) == 0 || bytes.Equal(v, NullJSON)
}
