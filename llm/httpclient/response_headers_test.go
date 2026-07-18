package httpclient

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/streams"
)

func TestMergeForwardResponseHeaders(t *testing.T) {
	tests := []struct {
		name string
		src  http.Header
		want string
	}{
		{name: "true", src: http.Header{"x-reasoning-included": []string{" TRUE "}}, want: "true"},
		{name: "false", src: http.Header{ReasoningIncludedHeader: []string{"false"}}},
		{name: "missing", src: http.Header{"Set-Cookie": []string{"secret=1"}}},
		{name: "conflicting duplicates", src: http.Header{
			ReasoningIncludedHeader: []string{"true"},
			"x-reasoning-included":  []string{"false"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := http.Header{ReasoningIncludedHeader: []string{"stale"}}
			got := MergeForwardResponseHeaders(dst, tt.src)

			require.Equal(t, tt.want, got.Get(ReasoningIncludedHeader))
			require.Empty(t, got.Get("Set-Cookie"))
		})
	}
}

func TestResponseHeadersStreamCopiesHeaders(t *testing.T) {
	headers := http.Header{ReasoningIncludedHeader: []string{"true"}}
	stream := WithResponseHeaders(streams.SliceStream([]*StreamEvent{}), headers)
	headers.Set(ReasoningIncludedHeader, "false")

	got := GetResponseHeaders(stream)
	got.Set(ReasoningIncludedHeader, "false")

	require.Equal(t, "true", GetResponseHeaders(stream).Get(ReasoningIncludedHeader))
	require.Nil(t, GetResponseHeaders(streams.SliceStream([]*StreamEvent{})))
}
