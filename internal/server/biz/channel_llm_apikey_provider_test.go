package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

// innerProvider unwraps the context provider that getAPIKeyProvider always
// returns, so a test can tell which strategy provider was selected.
func innerProvider(t *testing.T, ch *Channel) any {
	t.Helper()

	wrapped, ok := getAPIKeyProvider(ch).(*channelAPIKeyContextProvider)
	require.True(t, ok, "getAPIKeyProvider must wrap the selected provider")

	return wrapped.inner
}

// A multi-key channel that temporarily drops to a single selectable key must
// keep its configured policy: a static provider would lose the fixed cursor and
// the round-robin success counter, so a re-enabled key would take traffic back
// and successes reported in the meantime would be dropped.
func TestGetAPIKeyProvider_KeepsStrategyWhenOneKeyIsSelectable(t *testing.T) {
	t.Run("fixed keeps its provider when one of two keys is disabled", func(t *testing.T) {
		ch := withStrategy(newKeyChannel(1, []string{"k1", "k2"}, "k1"), objects.APIKeyStrategyFixed)
		require.Len(t, ch.cachedEnabledAPIKeys, 1, "precondition: exactly one key is selectable")

		_, ok := innerProvider(t, ch).(*FixedKeyProvider)
		require.True(t, ok, "the fixed provider must stay active so the cursor is not lost")
	})

	t.Run("round robin success keeps its provider when one of two keys is disabled", func(t *testing.T) {
		ch := withStrategy(newKeyChannel(1, []string{"k1", "k2"}, "k1"), objects.APIKeyStrategyRoundRobinSuccess)
		require.Len(t, ch.cachedEnabledAPIKeys, 1, "precondition: exactly one key is selectable")

		_, ok := innerProvider(t, ch).(*RoundRobinSuccessKeyProvider)
		require.True(t, ok, "the success counter must stay active so successes are not dropped")
	})

	t.Run("a single-key channel keeps the static provider", func(t *testing.T) {
		ch := withStrategy(newKeyChannel(1, []string{"k1"}), objects.APIKeyStrategyFixed)

		_, ok := innerProvider(t, ch).(*FixedKeyProvider)
		require.False(t, ok, "a channel that only has one key needs no stateful provider")
	})
}
