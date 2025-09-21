package xjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSafeJSONRawMessage_Empty(t *testing.T) {
	got := SafeJSONRawMessage("")
	require.Equal(t, json.RawMessage("{}"), got)
}

func TestSafeJSONRawMessage_ValidPassThrough(t *testing.T) {
	in := `{"a":1,"b":"x"}`
	got := SafeJSONRawMessage(in)
	require.True(t, json.Valid(got))
	// should be identical for valid JSON input
	require.Equal(t, json.RawMessage(in), got)
}

func TestSafeJSONRawMessage_Repairable(t *testing.T) {
	// Missing quotes around key and single quotes for string
	in := "{a:1,'b':'x',}"
	got := SafeJSONRawMessage(in)
	// Should become valid JSON
	require.True(t, json.Valid(got))

	var m map[string]any
	require.NoError(t, json.Unmarshal(got, &m))
	require.Equal(t, float64(1), m["a"]) // numbers decode as float64 in map[string]any
	require.Equal(t, "x", m["b"])
}

func TestSafeJSONRawMessage_Unrepairable(t *testing.T) {
	in := "@@@ not json @@@"
	got := SafeJSONRawMessage(in)
	// The repairer may wrap invalid input as a JSON string; ensure it's valid JSON
	require.True(t, json.Valid(got))

	var s string
	require.NoError(t, json.Unmarshal(got, &s))
	require.Equal(t, in, s)
}
