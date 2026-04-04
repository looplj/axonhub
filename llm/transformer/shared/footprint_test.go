package shared

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeFootprint(t *testing.T) {
	t.Run("empty inputs return empty", func(t *testing.T) {
		require.Empty(t, ComputeFootprint("", "x"))
		require.Empty(t, ComputeFootprint("https://api.example.com", ""))
	})

	t.Run("produces 6-char hex string", func(t *testing.T) {
		fp := ComputeFootprint("https://api.anthropic.com/v1", "channel-1")
		require.Len(t, fp, 6)
	})

	t.Run("deterministic", func(t *testing.T) {
		fp1 := ComputeFootprint("https://api.anthropic.com/v1", "channel-1")
		fp2 := ComputeFootprint("https://api.anthropic.com/v1", "channel-1")
		require.Equal(t, fp1, fp2)
	})

	t.Run("different identities produce different footprints", func(t *testing.T) {
		fp1 := ComputeFootprint("https://api.anthropic.com/v1", "channel-1")
		fp2 := ComputeFootprint("https://api.anthropic.com/v1", "channel-2")
		require.NotEqual(t, fp1, fp2)
	})
}
