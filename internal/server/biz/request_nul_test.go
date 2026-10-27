package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestStripJSONNul(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "no nul", in: `{"a":"hello"}`, want: `{"a":"hello"}`},
		{name: "nul inside string", in: `{"a":"hello\u0000world"}`, want: `{"a":"helloworld"}`},
		{name: "several", in: `{"a":"\u0000x\u0000","b":["\u0000"]}`, want: `{"a":"x","b":[""]}`},
		{name: "escaped backslash kept", in: `{"a":"\\u0000"}`, want: `{"a":"\\u0000"}`},
		{name: "escaped backslash then nul", in: `{"a":"\\\u0000"}`, want: `{"a":"\\"}`},
		{name: "other unicode escape kept", in: `{"a":"A\u0001"}`, want: `{"a":"A\u0001"}`},
		{name: "empty", in: ``, want: ``},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, string(stripJSONNul(objects.JSONRawMessage(tt.in))))
		})
	}
}
