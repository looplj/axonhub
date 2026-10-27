package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// Listing 429 in RetryableStatusCodes is the only way a channel asks for a
// same-channel retry on a rate limit; every other channel keeps switching
// channels, which is the default behavior.
func TestChannelRetriesOn429(t *testing.T) {
	t.Run("a channel without settings does not opt in", func(t *testing.T) {
		require.False(t, channelRetriesOn429(nil))
		require.False(t, channelRetriesOn429(&biz.Channel{}))
		require.False(t, channelRetriesOn429(&biz.Channel{Channel: &ent.Channel{}}))
	})

	t.Run("codes other than 429 do not opt in", func(t *testing.T) {
		ch := &biz.Channel{Channel: &ent.Channel{
			Settings: &objects.ChannelSettings{RetryableStatusCodes: []int{500, 502}},
		}}

		require.False(t, channelRetriesOn429(ch))
	})

	t.Run("listing 429 opts the channel in", func(t *testing.T) {
		ch := &biz.Channel{Channel: &ent.Channel{
			Settings: &objects.ChannelSettings{RetryableStatusCodes: []int{500, 429}},
		}}

		require.True(t, channelRetriesOn429(ch))
	})
}
