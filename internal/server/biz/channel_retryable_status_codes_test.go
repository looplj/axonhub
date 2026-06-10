package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestNormalizeRetryableStatusCodes(t *testing.T) {
	t.Run("sorts and deduplicates error status codes", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			RetryableStatusCodes: []int{403, 400, 403, 500},
		}

		err := NormalizeRetryableStatusCodes(settings)

		require.NoError(t, err)
		require.Equal(t, []int{400, 403, 500}, settings.RetryableStatusCodes)
	})

	t.Run("allows empty settings", func(t *testing.T) {
		require.NoError(t, NormalizeRetryableStatusCodes(nil))
		require.NoError(t, NormalizeRetryableStatusCodes(&objects.ChannelSettings{}))
	})

	t.Run("rejects non error status codes", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			RetryableStatusCodes: []int{200},
		}

		err := NormalizeRetryableStatusCodes(settings)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid retryable status code 200")
	})
}
