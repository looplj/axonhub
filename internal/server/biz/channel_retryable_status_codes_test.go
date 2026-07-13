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

func TestNormalizeRetryableErrorPatterns(t *testing.T) {
	t.Run("trims and deduplicates retryable error patterns", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			RetryableErrorPatterns: []objects.RetryableErrorPattern{
				{Pattern: " Console API returned 403 "},
				{Pattern: "Console API returned 403"},
				{Pattern: `Console API returned \d+`, Regex: true},
			},
		}

		err := NormalizeRetryableErrorPatterns(settings)

		require.NoError(t, err)
		require.Equal(t, []objects.RetryableErrorPattern{
			{Pattern: "Console API returned 403"},
			{Pattern: `Console API returned \d+`, Regex: true},
		}, settings.RetryableErrorPatterns)
	})

	t.Run("allows empty settings", func(t *testing.T) {
		require.NoError(t, NormalizeRetryableErrorPatterns(nil))
		require.NoError(t, NormalizeRetryableErrorPatterns(&objects.ChannelSettings{}))
	})

	t.Run("rejects invalid regex patterns", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			RetryableErrorPatterns: []objects.RetryableErrorPattern{
				{Pattern: "Console API returned [", Regex: true},
			},
		}

		err := NormalizeRetryableErrorPatterns(settings)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid retryable error regex")
	})
}

func TestNormalizeAPIKeyFailover(t *testing.T) {
	settings := &objects.ChannelSettings{
		APIKeyFailover: &objects.ChannelAPIKeyFailover{
			Enabled:     true,
			StatusCodes: []int{403, 402, 403},
			ErrorPatterns: []objects.RetryableErrorPattern{
				{Pattern: " insufficient balance "},
				{Pattern: "insufficient balance"},
				{Pattern: `quota\s+exhausted`, Regex: true},
			},
		},
	}

	err := NormalizeAPIKeyFailover(settings)

	require.NoError(t, err)
	require.Equal(t, []int{402, 403}, settings.APIKeyFailover.StatusCodes)
	require.Equal(t, []objects.RetryableErrorPattern{
		{Pattern: "insufficient balance"},
		{Pattern: `quota\s+exhausted`, Regex: true},
	}, settings.APIKeyFailover.ErrorPatterns)
}

func TestNormalizeAPIKeyFailoverRejectsInvalidValues(t *testing.T) {
	t.Run("status code", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			APIKeyFailover: &objects.ChannelAPIKeyFailover{StatusCodes: []int{200}},
		}
		require.Error(t, NormalizeAPIKeyFailover(settings))
	})

	t.Run("regex", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			APIKeyFailover: &objects.ChannelAPIKeyFailover{
				ErrorPatterns: []objects.RetryableErrorPattern{{Pattern: "[", Regex: true}},
			},
		}
		require.Error(t, NormalizeAPIKeyFailover(settings))
	})
}
