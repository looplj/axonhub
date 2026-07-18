package httpclient

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForwardResponseHeaders(t *testing.T) {
	tests := []struct {
		name       string
		input      http.Header
		wantHeader string
	}{
		{
			name:       "forwards verified true value",
			input:      http.Header{"x-reasoning-included": []string{" TRUE "}},
			wantHeader: "true",
		},
		{
			name: "does not synthesize false value",
			input: http.Header{
				ReasoningIncludedHeader: []string{"false"},
			},
		},
		{
			name: "does not forward unrelated headers",
			input: http.Header{
				ReasoningIncludedHeader: []string{"true"},
				"Set-Cookie":            []string{"secret=1"},
			},
			wantHeader: "true",
		},
		{
			name: "does not forward conflicting duplicate names",
			input: http.Header{
				ReasoningIncludedHeader: []string{"true"},
				"x-reasoning-included":  []string{"false"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ForwardResponseHeaders(tt.input)

			require.Equal(t, tt.wantHeader, got.Get(ReasoningIncludedHeader))
			require.Empty(t, got.Get("Set-Cookie"))
		})
	}
}
