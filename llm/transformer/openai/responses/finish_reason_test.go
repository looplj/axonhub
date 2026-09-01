package responses

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAbnormalChatFinishReason(t *testing.T) {
	for _, reason := range []string{"length", "content_filter", "error", "cancelled", "canceled"} {
		require.True(t, IsAbnormalChatFinishReason(reason), reason)
	}
	for _, reason := range []string{"stop", "tool_calls", ""} {
		require.False(t, IsAbnormalChatFinishReason(reason), reason)
	}
}
