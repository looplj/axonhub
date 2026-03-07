package biz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_isPairCodeFormat(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		s    string
		want bool
	}{
		{
			name: "valid pair code",
			s:    "2E76-6A49",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPairCodeFormat(tt.s)
			require.Equal(t, tt.want, got)
		})
	}
}
