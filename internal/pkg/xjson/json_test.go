package xjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type sample struct {
	A int    `json:"a"`
	B string `json:"b"`
}

func TestMustMarshalString_Success(t *testing.T) {
	v := sample{A: 1, B: "x"}
	got := MustMarshalString(v)

	var decoded sample
	require.NoError(t, json.Unmarshal([]byte(got), &decoded))
	require.Equal(t, v, decoded)
}

func TestMustMarshalString_Panic(t *testing.T) {
	// channel cannot be marshaled by encoding/json, should panic
	ch := make(chan int)

	require.Panics(t, func() { _ = MustMarshalString(ch) })
}

func TestMustMarshal_Success(t *testing.T) {
	v := sample{A: 2, B: "y"}
	got := MustMarshal(v)

	var decoded sample
	require.NoError(t, json.Unmarshal(got, &decoded))
	require.Equal(t, v, decoded)
}

func TestMustMarshal_Panic(t *testing.T) {
	ch := make(chan int)

	require.Panics(t, func() { _ = MustMarshal(ch) })
}

func TestTo_Success(t *testing.T) {
	in := []byte(`{"a":3,"b":"z"}`)
	got, err := To[sample](in)
	require.NoError(t, err)
	require.Equal(t, sample{A: 3, B: "z"}, got)
}

func TestTo_Error(t *testing.T) {
	in := []byte(`{"a":`)
	_, err := To[sample](in)
	require.Error(t, err)
}

func TestMustTo_Success(t *testing.T) {
	in := []byte(`{"a":4,"b":"w"}`)
	got := MustTo[sample](in)
	require.Equal(t, sample{A: 4, B: "w"}, got)
}

func TestMustTo_Panic(t *testing.T) {
	in := []byte(`{"a":`)

	require.Panics(t, func() { _ = MustTo[sample](in) })
}

func TestMarshal_StringInput(t *testing.T) {
	raw := `{"k":"v"}`
	got, err := Marshal(raw)
	require.NoError(t, err)
	require.Equal(t, []byte(raw), []byte(got))
}

func TestMarshal_BytesInput(t *testing.T) {
	raw := []byte(`{"n":123}`)
	got, err := Marshal(raw)
	require.NoError(t, err)
	require.Equal(t, raw, []byte(got))
}

func TestMarshal_OtherTypeInput(t *testing.T) {
	v := sample{A: 5, B: "q"}
	got, err := Marshal(v)
	require.NoError(t, err)

	var decoded sample
	require.NoError(t, json.Unmarshal(got, &decoded))
	require.Equal(t, v, decoded)
}
