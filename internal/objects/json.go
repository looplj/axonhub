package objects

import (
	"encoding/json"
	"errors"
	"io"
)

type JSONRawMessage []byte

// MarshalJSON returns m as the JSON encoding of m.
func (m JSONRawMessage) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}

	return m, nil
}

// UnmarshalJSON sets *m to a copy of data.
func (m *JSONRawMessage) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.New("json.RawMessage: UnmarshalJSON on nil pointer")
	}

	*m = append((*m)[0:0], data...)

	return nil
}

// MarshalJSON returns m as the JSON encoding of m.
func (m JSONRawMessage) MarshalGQL(w io.Writer) {
	if m == nil {
		_, _ = w.Write([]byte("null"))
		return
	}

	_, _ = w.Write(m)
}

// UnmarshalJSON sets *m to a copy of data.
func (m *JSONRawMessage) UnmarshalGQL(v any) error {
	if m == nil {
		return errors.New("json.RawMessage: UnmarshalGQL on nil pointer")
	}

	switch v := v.(type) {
	case *JSONRawMessage:
		*m = append((*m)[0:0], *v...)
		return nil
	case *string:
		*v = string(*m)
		return nil
	case *[]byte:
		*v = append((*v)[0:0], *m...)
		return nil
	case *map[string]any:
		return json.Unmarshal(*m, v)
	}

	return nil
}
