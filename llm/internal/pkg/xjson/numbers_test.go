package xjson

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalizeIntegralJSONNumbers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "float encoded int", in: `{"yield_time_ms":180000.0}`, want: `{"yield_time_ms":180000}`},
		{name: "negative float encoded int", in: `{"n":-180000.0}`, want: `{"n":-180000}`},
		{name: "scientific integer", in: `{"n":1e5}`, want: `{"n":100000}`},
		{name: "trailing zeros", in: `{"n":10.00}`, want: `{"n":10}`},
		{name: "non integral float", in: `{"n":1.5}`, want: `{"n":1.5}`},
		{name: "scientific fraction", in: `{"n":1e-1}`, want: `{"n":0.1}`},
		{name: "already integer", in: `{"n":180000}`, want: `{"n":180000}`},
		{name: "nested object and array", in: `{"a":[180000.0,{"b":1.0}]}`, want: `{"a":[180000,{"b":1}]}`},
		{name: "invalid json", in: `{"yield_time_ms":180000.0`, want: `{"yield_time_ms":180000.0`},
		{name: "fragment", in: `{"yield_time_ms":180`, want: `{"yield_time_ms":180`},
		{name: "empty", in: "", want: ""},
		{name: "whitespace", in: "   ", want: "   "},
		{name: "non object", in: `180000.0`, want: `180000`},
		{name: "string stays", in: `{"n":"180000.0"}`, want: `{"n":"180000.0"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CanonicalizeIntegralJSONNumbers(tt.in)
			switch tt.name {
			case "invalid json", "fragment", "empty", "whitespace":
				require.Equal(t, tt.want, got)
			case "non integral float", "scientific fraction", "string stays":
				require.JSONEq(t, tt.want, got)
			default:
				require.JSONEq(t, tt.want, got)
				require.False(t, strings.Contains(got, ".0"), "got %s", got)
			}
		})
	}
}

func TestCanonicalizeIntegralJSONNumbers_PreservesOversizedIntegerLexeme(t *testing.T) {
	t.Parallel()
	in := `{"n":9223372036854775808}`
	require.Equal(t, in, CanonicalizeIntegralJSONNumbers(in))
}
