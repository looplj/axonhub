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

// ObjectHasOnlyFields reports whether object is non-nil and contains only the
// allowed JSON fields. It does not require every allowed field to be present.
func ObjectHasOnlyFields(object map[string]json.RawMessage, allowed ...string) bool {
	if object == nil {
		return false
	}
	allowedFields := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedFields[field] = struct{}{}
	}
	for field := range object {
		if _, ok := allowedFields[field]; !ok {
			return false
		}
	}
	return true
}

func IsNull(v json.RawMessage) bool {
	return len(v) == 0 || bytes.Equal(v, NullJSON)
}
