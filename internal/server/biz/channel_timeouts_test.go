package biz

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestNormalizeChannelTimeouts(t *testing.T) {
	t.Run("allows empty settings", func(t *testing.T) {
		require.NoError(t, NormalizeChannelTimeouts(nil))
		require.NoError(t, NormalizeChannelTimeouts(&objects.ChannelSettings{}))
	})

	t.Run("keeps valid overrides", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			StreamFirstEventTimeoutSeconds: lo.ToPtr(150),
			NonStreamResponseTimeoutSeconds: lo.ToPtr(300),
		}

		require.NoError(t, NormalizeChannelTimeouts(settings))
		require.Equal(t, 150, *settings.StreamFirstEventTimeoutSeconds)
		require.Equal(t, 300, *settings.NonStreamResponseTimeoutSeconds)
	})

	t.Run("normalizes non-positive values to inherit", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			StreamFirstEventTimeoutSeconds: lo.ToPtr(0),
			NonStreamResponseTimeoutSeconds: lo.ToPtr(-5),
		}

		require.NoError(t, NormalizeChannelTimeouts(settings))
		require.Nil(t, settings.StreamFirstEventTimeoutSeconds)
		require.Nil(t, settings.NonStreamResponseTimeoutSeconds)
	})

	t.Run("rejects values above the global cap", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			StreamFirstEventTimeoutSeconds: lo.ToPtr(maxRetryResponseTimeoutSeconds + 1),
		}

		require.Error(t, NormalizeChannelTimeouts(settings))
	})
}
