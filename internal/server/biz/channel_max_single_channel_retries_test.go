package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestValidateMaxSingleChannelRetries(t *testing.T) {
	ptr := func(v int) *int { return &v }

	t.Run("accepts an unset override", func(t *testing.T) {
		require.NoError(t, ValidateMaxSingleChannelRetries(nil))
		require.NoError(t, ValidateMaxSingleChannelRetries(&objects.ChannelSettings{}))
	})

	t.Run("accepts the inclusive range", func(t *testing.T) {
		require.NoError(t, ValidateMaxSingleChannelRetries(&objects.ChannelSettings{MaxSingleChannelRetries: ptr(0)}))
		require.NoError(t, ValidateMaxSingleChannelRetries(&objects.ChannelSettings{MaxSingleChannelRetries: ptr(65535)}))
	})

	t.Run("rejects values outside the range", func(t *testing.T) {
		require.Error(t, ValidateMaxSingleChannelRetries(&objects.ChannelSettings{MaxSingleChannelRetries: ptr(-1)}))
		require.Error(t, ValidateMaxSingleChannelRetries(&objects.ChannelSettings{MaxSingleChannelRetries: ptr(65536)}))
	})
}
