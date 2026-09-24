package biz

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

// newKeyChannel builds a channel snapshot with the given keys plus the keys that
// carry an active disable record. Both the entity (the source of truth) and the
// precomputed caches the providers read are populated, mirroring buildChannel.
func newKeyChannel(id int, keys []string, disabled ...string) *Channel {
	disabledList := make([]objects.DisabledAPIKey, 0, len(disabled))
	disabledSet := make(map[string]struct{}, len(disabled))

	for _, key := range disabled {
		disabledList = append(disabledList, objects.DisabledAPIKey{Key: key})
		disabledSet[key] = struct{}{}
	}

	creds := objects.ChannelCredentials{APIKeys: keys}

	return &Channel{
		Channel: &ent.Channel{
			ID:              id,
			Credentials:     creds,
			DisabledAPIKeys: disabledList,
		},
		cachedDisabledKeySet: disabledSet,
		cachedEnabledAPIKeys: creds.GetEnabledAPIKeys(disabledList),
	}
}

// withStrategy configures the multi-key strategy (and optionally the reuse
// count) on a channel snapshot.
func withStrategy(ch *Channel, strategy string, switchAfter ...int) *Channel {
	settings := &objects.ChannelSettings{APIKeyStrategy: &strategy}
	if len(switchAfter) > 0 {
		count := switchAfter[0]
		settings.APIKeyRoundRobinSwitchAfter = &count
	}

	ch.Settings = settings

	return ch
}

// setFixedCursor forces the remembered fixed-strategy position, so a test can
// start from "the channel has been using key X" without walking there.
func setFixedCursor(state *apiKeySelectionState, key string, idx int) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.fixedCursorKey = key
	state.fixedCursorIdx = idx
}

// setRoundRobinSuccessCursor forces the round_robin_success cursor.
func setRoundRobinSuccessCursor(state *apiKeySelectionState, key string, idx, count int) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.rrSuccessCursor = key
	state.rrSuccessIdx = idx
	state.rrSuccessCount = count
}

func newFixedProvider(t *testing.T, svc *ChannelService, ch *Channel) (*FixedKeyProvider, *apiKeySelectionState) {
	t.Helper()

	state := svc.apiKeySelectionStateFor(ch)
	require.NotNil(t, state, "the fixed strategy must register shared state")

	ch.apiKeyState = state
	svc.publishAPIKeySelectionSnapshot(ch)

	return NewFixedKeyProvider(ch), state
}

func newRoundRobinSuccessProvider(
	t *testing.T,
	svc *ChannelService,
	ch *Channel,
) (*RoundRobinSuccessKeyProvider, *apiKeySelectionState) {
	t.Helper()

	state := svc.apiKeySelectionStateFor(ch)
	require.NotNil(t, state, "the round_robin_success strategy must register shared state")

	ch.apiKeyState = state
	svc.publishAPIKeySelectionSnapshot(ch)

	return NewRoundRobinSuccessKeyProvider(ch), state
}

// --- A. dispatch ---

func TestNewMultiKeyProvider_Dispatch(t *testing.T) {
	ch := newKeyChannel(1, []string{"k1", "k2"})

	// nil settings -> sticky (default).
	require.IsType(t, &sharedStickyKeyProvider{}, newMultiKeyProvider(ch))

	cases := []struct {
		strategy string
		want     any
	}{
		{"", &sharedStickyKeyProvider{}},
		{objects.APIKeyStrategySticky, &sharedStickyKeyProvider{}},
		{objects.APIKeyStrategyRandom, &RandomKeyProvider{}},
		{objects.APIKeyStrategyRoundRobin, &RoundRobinKeyProvider{}},
		{objects.APIKeyStrategyRoundRobinSuccess, &RoundRobinSuccessKeyProvider{}},
		{objects.APIKeyStrategyPriority, &PriorityKeyProvider{}},
		{objects.APIKeyStrategyFixed, &FixedKeyProvider{}},
		{"unknown_value", &sharedStickyKeyProvider{}},
	}
	for _, tc := range cases {
		strategy := tc.strategy
		ch.Settings = &objects.ChannelSettings{APIKeyStrategy: &strategy}
		require.IsTypef(t, tc.want, newMultiKeyProvider(ch), "strategy %q", tc.strategy)
	}
}

func TestStrategyKeepsSelectionState(t *testing.T) {
	for _, strategy := range []string{
		"", objects.APIKeyStrategySticky, objects.APIKeyStrategyRoundRobin,
		objects.APIKeyStrategyRoundRobinSuccess, objects.APIKeyStrategyFixed,
	} {
		require.Truef(t, strategyKeepsSelectionState(strategy), "strategy %q", strategy)
	}

	for _, strategy := range []string{objects.APIKeyStrategyRandom, objects.APIKeyStrategyPriority} {
		require.Falsef(t, strategyKeepsSelectionState(strategy), "strategy %q", strategy)
	}
}

func TestAPIKeySelectionStateFor_OnlyRegistersRememberingStrategies(t *testing.T) {
	svc := &ChannelService{}

	for i, strategy := range []string{
		"", objects.APIKeyStrategySticky, objects.APIKeyStrategyRoundRobin,
		objects.APIKeyStrategyRoundRobinSuccess, objects.APIKeyStrategyFixed,
	} {
		ch := withStrategy(newKeyChannel(i+1, []string{"k1", "k2"}), strategy)
		require.NotNilf(t, svc.apiKeySelectionStateFor(ch), "strategy %q", strategy)
	}

	for i, strategy := range []string{objects.APIKeyStrategyRandom, objects.APIKeyStrategyPriority} {
		ch := withStrategy(newKeyChannel(100+i, []string{"k1", "k2"}), strategy)
		require.Nilf(t, svc.apiKeySelectionStateFor(ch), "strategy %q", strategy)
		require.Nil(t, svc.apiKeySelectionState(100+i))
	}
}

// --- B. priority ---

func TestPriorityKeyProvider_AlwaysFirstSelectable(t *testing.T) {
	ch := newKeyChannel(1, []string{"first", "second", "third"})
	p := NewPriorityKeyProvider(ch)

	ctx := context.Background()
	for range 10 {
		require.Equal(t, "first", p.Get(ctx))
	}
}

func TestPriorityKeyProvider_SkipsDisabledLeadingKeys(t *testing.T) {
	ch := newKeyChannel(1, []string{"first", "second", "third"}, "first", "second")
	p := NewPriorityKeyProvider(ch)

	require.Equal(t, "third", p.Get(context.Background()))
}

func TestPriorityKeyProvider_AllDisabledFallsBackToFirst(t *testing.T) {
	ch := newKeyChannel(1, []string{"first", "second"}, "first", "second")
	p := NewPriorityKeyProvider(ch)

	// Every key is disabled, so the enabled snapshot is empty and the provider
	// falls back to the channel's first key. A channel in this state is taken out
	// of service, so this selection is never actually used.
	require.Equal(t, "first", p.Get(context.Background()))
}

// --- C. fixed ---

func TestFixedKeyProvider_StartsAtFirstSelectable(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	p, _ := newFixedProvider(t, svc, ch)

	require.Equal(t, "k1", p.Get(context.Background()))
}

func TestFixedKeyProvider_StartsAtFirstSelectableSkippingDisabled(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}, "k1"), objects.APIKeyStrategyFixed)
	p, _ := newFixedProvider(t, svc, ch)

	require.Equal(t, "k2", p.Get(context.Background()))
}

func TestFixedKeyProvider_KeepsCurrentKeyWhileSelectable(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	p, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k2", 1)

	ctx := context.Background()
	for i := range 20 {
		require.Equal(t, "k2", p.Get(ctx), "call %d", i)
	}
}

func TestFixedKeyProvider_MovesForwardWhenCurrentKeyIsDisabled(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyFixed)
	_, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k2", 1)

	// k2 gets disabled and the channel is rebuilt. A provider built from the new
	// snapshot must move on to k3, never back to k1.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}, "k2"), objects.APIKeyStrategyFixed)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k3", NewFixedKeyProvider(reloaded).Get(context.Background()))
}

func TestFixedKeyProvider_SkipsSeveralDisabledKeysOnTheWay(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4", "k5"}), objects.APIKeyStrategyFixed)
	_, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k2", 1)

	// k2, k3 and k4 are disabled in one go (a retry loop can disable several).
	reloaded := withStrategy(
		newKeyChannel(1, []string{"k1", "k2", "k3", "k4", "k5"}, "k2", "k3", "k4"),
		objects.APIKeyStrategyFixed,
	)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k5", NewFixedKeyProvider(reloaded).Get(context.Background()))
}

func TestFixedKeyProvider_WrapsAroundToFirstSelectable(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	_, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k3", 2)

	// The tail is disabled, so the walk wraps around and lands on k1.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}, "k3"), objects.APIKeyStrategyFixed)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k1", NewFixedKeyProvider(reloaded).Get(context.Background()))
}

func TestFixedKeyProvider_RemovedKeyContinuesForward(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyFixed)
	_, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k3", 2)

	// k3 is deleted from the channel: the walk continues from where it used to
	// sit, so k4 is next rather than restarting at k1.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k4"}), objects.APIKeyStrategyFixed)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k4", NewFixedKeyProvider(reloaded).Get(context.Background()))
}

func TestFixedKeyProvider_RemovedKeyOutOfRangeWrapsAround(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyFixed)
	_, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k4", 3)

	// The array shrank below the remembered position, so the walk restarts at the
	// head of the array instead of reading past the end.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyFixed)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k1", NewFixedKeyProvider(reloaded).Get(context.Background()))
}

func TestFixedKeyProvider_RemovedKeyThenSkipsDisabledTail(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyFixed)
	_, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k3", 2)

	// k3 was removed and the key that took its place is disabled: the walk keeps
	// going and wraps around to k1.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k4"}, "k4"), objects.APIKeyStrategyFixed)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k1", NewFixedKeyProvider(reloaded).Get(context.Background()))
}

func TestFixedKeyProvider_AllDisabledFallsBackToFirst(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyFixed)
	_, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k2", 1)

	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2"}, "k1", "k2"), objects.APIKeyStrategyFixed)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k1", NewFixedKeyProvider(reloaded).Get(context.Background()))
}

func TestFixedKeyProvider_UsesItsOwnSnapshot(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	p, _ := newFixedProvider(t, svc, ch)

	require.Equal(t, "k1", p.Get(context.Background()))

	// The provider keeps the snapshot it was built with. A rebuild must not make
	// an in-flight provider pick a key that belongs to a different channel
	// generation than the endpoint it is about to call.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}, "k1"), objects.APIKeyStrategyFixed)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)
	svc.publishAPIKeySelectionSnapshot(reloaded)

	require.Equal(t, "k1", p.Get(context.Background()), "the in-flight provider keeps its own snapshot")
	require.Equal(t, "k2", NewFixedKeyProvider(reloaded).Get(context.Background()), "a rebuilt provider moves on")
}

func TestFixedKeyProvider_KeepsKeyWhenTheArrayShifts(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	_, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k2", 1)

	// A new key is prepended: positions shift, but the remembered key is still
	// selectable, so it keeps being used.
	reloaded := withStrategy(newKeyChannel(1, []string{"k0", "k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k2", NewFixedKeyProvider(reloaded).Get(context.Background()))
}

func TestFixedKeyProvider_ForgetsStateOnChannelDeletion(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyFixed)
	_, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k2", 1)

	svc.forgetAPIKeySelectionState(1)

	require.Nil(t, svc.apiKeySelectionState(1))
}

// --- D. round_robin_success ---

func TestRoundRobinSuccessKeyProvider_GetDoesNotAdvance(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyRoundRobinSuccess, 2)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	ctx := context.Background()
	for i := range 10 {
		require.Equal(t, "k1", p.Get(ctx), "call %d", i)
	}
}

func TestRoundRobinSuccessKeyProvider_AdvancesAfterSuccessfulCalls(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyRoundRobinSuccess, 2)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	ctx := context.Background()
	require.Equal(t, "k1", p.Get(ctx))

	// One success is not enough for switchAfter=2.
	svc.onAPIKeySuccess(1, "k1")
	require.Equal(t, "k1", p.Get(ctx))

	// The second success moves the cursor on.
	svc.onAPIKeySuccess(1, "k1")
	require.Equal(t, "k2", p.Get(ctx))

	svc.onAPIKeySuccess(1, "k2")
	require.Equal(t, "k2", p.Get(ctx))
	svc.onAPIKeySuccess(1, "k2")
	require.Equal(t, "k3", p.Get(ctx))
}

func TestRoundRobinSuccessKeyProvider_SwitchAfterOne(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	ctx := context.Background()
	require.Equal(t, "k1", p.Get(ctx))

	svc.onAPIKeySuccess(1, "k1")
	require.Equal(t, "k2", p.Get(ctx))

	svc.onAPIKeySuccess(1, "k2")
	require.Equal(t, "k1", p.Get(ctx))
}

func TestRoundRobinSuccessKeyProvider_SwitchAfterBelowOneIsTreatedAsOne(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyRoundRobinSuccess, 0)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	require.Equal(t, "k1", p.Get(context.Background()))

	svc.onAPIKeySuccess(1, "k1")

	require.Equal(t, "k2", p.Get(context.Background()))
}

func TestRoundRobinSuccessKeyProvider_IgnoresReportsForOtherKeys(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	require.Equal(t, "k1", p.Get(context.Background()))

	// A late report for a key the cursor never pointed at must not advance it.
	svc.onAPIKeySuccess(1, "k2")
	svc.onAPIKeySuccess(1, "")

	require.Equal(t, "k1", p.Get(context.Background()))
}

func TestRoundRobinSuccessKeyProvider_IgnoresOtherStrategies(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	_, state := newRoundRobinSuccessProvider(t, svc, ch)

	// Flip the channel to a strategy that does not count successes.
	other := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyFixed)
	other.apiKeyState = svc.apiKeySelectionStateFor(other)
	svc.publishAPIKeySelectionSnapshot(other)

	svc.onAPIKeySuccess(1, "k1")

	state.mu.Lock()
	count := state.rrSuccessCount
	state.mu.Unlock()

	require.Zero(t, count)
}

func TestRoundRobinSuccessKeyProvider_UnknownChannelIsIgnored(t *testing.T) {
	svc := &ChannelService{}

	// No state registered for this channel: the report must be a silent no-op.
	require.NotPanics(t, func() { svc.onAPIKeySuccess(42, "k1") })
}

func TestRoundRobinSuccessKeyProvider_ResetsWhenCursorKeyIsDisabled(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyRoundRobinSuccess, 5)
	_, state := newRoundRobinSuccessProvider(t, svc, ch)

	setRoundRobinSuccessCursor(state, "k1", 0, 3)

	// k1 is disabled: the cursor moves on and the successes counted for k1 are
	// discarded.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}, "k1"), objects.APIKeyStrategyRoundRobinSuccess, 5)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k2", NewRoundRobinSuccessKeyProvider(reloaded).Get(context.Background()))

	state.mu.Lock()
	count := state.rrSuccessCount
	state.mu.Unlock()

	require.Zero(t, count)
}

func TestRoundRobinSuccessKeyProvider_RemovedCursorKeyContinuesForward(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	_, state := newRoundRobinSuccessProvider(t, svc, ch)

	setRoundRobinSuccessCursor(state, "k3", 2, 0)

	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k4"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k4", NewRoundRobinSuccessKeyProvider(reloaded).Get(context.Background()))
}

func TestRoundRobinSuccessKeyProvider_UsesItsOwnSnapshot(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	require.Equal(t, "k1", p.Get(context.Background()))

	// Same as the fixed strategy: an in-flight provider must not be steered onto
	// another channel generation by a rebuild.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}, "k1"), objects.APIKeyStrategyRoundRobinSuccess, 1)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)
	svc.publishAPIKeySelectionSnapshot(reloaded)

	require.Equal(t, "k1", p.Get(context.Background()), "the in-flight provider keeps its own snapshot")
	require.Equal(t, "k2", NewRoundRobinSuccessKeyProvider(reloaded).Get(context.Background()), "a rebuilt provider moves on")
}

func TestRoundRobinSuccessKeyProvider_AdvancesPastDisabledKeys(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	require.Equal(t, "k1", p.Get(context.Background()))

	// k2 and k3 are disabled before k1 finishes its turn: the cursor must land on
	// k4 rather than on a disabled key. The success report reads the published
	// snapshot, so the rebuild has to be published.
	reloaded := withStrategy(
		newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}, "k2", "k3"),
		objects.APIKeyStrategyRoundRobinSuccess,
		1,
	)
	reloaded.apiKeyState = svc.apiKeySelectionStateFor(reloaded)
	svc.publishAPIKeySelectionSnapshot(reloaded)

	svc.onAPIKeySuccess(1, "k1")

	require.Equal(t, "k4", NewRoundRobinSuccessKeyProvider(reloaded).Get(context.Background()))
}

// --- F. concurrency ---

func TestFixedKeyProvider_ConcurrentGetIsConsistent(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	p, _ := newFixedProvider(t, svc, ch)

	var wg sync.WaitGroup

	for range 32 {
		wg.Go(func() {
			ctx := context.Background()
			for range 50 {
				require.Equal(t, "k1", p.Get(ctx))
			}
		})
	}

	wg.Wait()
}

func TestRoundRobinSuccessKeyProvider_ConcurrentGetAndSuccess(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyRoundRobinSuccess, 2)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	var wg sync.WaitGroup

	for range 16 {
		wg.Go(func() {
			ctx := context.Background()
			for range 50 {
				key := p.Get(ctx)
				svc.onAPIKeySuccess(1, key)
			}
		})
	}

	wg.Wait()

	// The cursor must still point at a selectable key of the channel.
	require.Contains(t, []string{"k1", "k2", "k3"}, p.Get(context.Background()))
}

// --- E. RecordPerformance wiring ---

func TestRecordPerformance_AdvancesRoundRobinSuccessCursor(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	svc := newTestChannelService(client)

	ch := withStrategy(newKeyChannel(7, []string{"k1", "k2"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	ch.apiKeyState = svc.apiKeySelectionStateFor(ch)
	svc.publishAPIKeySelectionSnapshot(ch)
	p := NewRoundRobinSuccessKeyProvider(ch)

	ctx := context.Background()
	require.Equal(t, "k1", p.Get(ctx))

	// A successful request reported through the normal metrics path is the only
	// thing that advances the cursor.
	svc.RecordPerformance(ctx, &PerformanceRecord{
		ChannelID:        7,
		APIKey:           "k1",
		EndTime:          time.Now(),
		Success:          true,
		RequestCompleted: true,
	})

	require.Equal(t, "k2", p.Get(ctx))
}

func TestRecordPerformance_FailedRequestDoesNotAdvanceRoundRobinSuccess(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	svc := newTestChannelService(client)

	ch := withStrategy(newKeyChannel(8, []string{"k1", "k2"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	ch.apiKeyState = svc.apiKeySelectionStateFor(ch)
	svc.publishAPIKeySelectionSnapshot(ch)
	p := NewRoundRobinSuccessKeyProvider(ch)

	ctx := context.Background()
	require.Equal(t, "k1", p.Get(ctx))

	svc.RecordPerformance(ctx, &PerformanceRecord{
		ChannelID:          8,
		APIKey:             "k1",
		EndTime:            time.Now(),
		Success:            false,
		RequestCompleted:   true,
		ResponseStatusCode: 500,
	})

	require.Equal(t, "k1", p.Get(ctx))
}

func TestRecordPerformance_SuccessDoesNotTouchOtherStrategies(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	svc := newTestChannelService(client)

	// A fixed channel has no success counter; a success report must be a no-op.
	ch := withStrategy(newKeyChannel(9, []string{"k1", "k2"}), objects.APIKeyStrategyFixed)
	ch.apiKeyState = svc.apiKeySelectionStateFor(ch)
	svc.publishAPIKeySelectionSnapshot(ch)

	ctx := context.Background()

	require.NotPanics(t, func() {
		svc.RecordPerformance(ctx, &PerformanceRecord{
			ChannelID:        9,
			APIKey:           "k1",
			EndTime:          time.Now(),
			Success:          true,
			RequestCompleted: true,
		})
	})

	require.Equal(t, "k1", NewFixedKeyProvider(ch).Get(ctx))
}

// --- D2. round_robin & sticky keep their state across snapshot rebuilds ---

// Any channel change rebuilds every channel snapshot, and with it every
// provider. A per-provider counter would restart the rotation at the first key
// each time any channel in the cluster changed, which is exactly the
// "everything goes to the first key" symptom this pins down.
func TestRoundRobinKeyProvider_RotatesAcrossRequests(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyRoundRobin, 1)
	ch.apiKeyState = svc.apiKeySelectionStateFor(ch)
	svc.publishAPIKeySelectionSnapshot(ch)
	p := NewRoundRobinKeyProvider(ch, 1)

	ctx := context.Background()
	require.Equal(t, "k1", p.Get(ctx))
	require.Equal(t, "k2", p.Get(ctx))
	require.Equal(t, "k3", p.Get(ctx))
	require.Equal(t, "k1", p.Get(ctx))
}

func TestRoundRobinKeyProvider_SwitchAfterN(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyRoundRobin, 2)
	ch.apiKeyState = svc.apiKeySelectionStateFor(ch)
	svc.publishAPIKeySelectionSnapshot(ch)
	p := NewRoundRobinKeyProvider(ch, 2)

	ctx := context.Background()
	require.Equal(t, "k1", p.Get(ctx))
	require.Equal(t, "k1", p.Get(ctx))
	require.Equal(t, "k2", p.Get(ctx))
	require.Equal(t, "k2", p.Get(ctx))
	require.Equal(t, "k1", p.Get(ctx))
}

func TestRoundRobinKeyProvider_KeepsRotatingAcrossSnapshotRebuild(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyRoundRobin, 1)
	ch.apiKeyState = svc.apiKeySelectionStateFor(ch)
	svc.publishAPIKeySelectionSnapshot(ch)
	p := NewRoundRobinKeyProvider(ch, 1)

	ctx := context.Background()
	require.Equal(t, "k1", p.Get(ctx))
	require.Equal(t, "k2", p.Get(ctx))

	// Rebuild the snapshot (any channel change does this) and build a fresh
	// provider from it: the rotation must continue with k3, not restart at k1.
	rebuilt := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyRoundRobin, 1)
	rebuilt.apiKeyState = svc.apiKeySelectionStateFor(rebuilt)
	svc.publishAPIKeySelectionSnapshot(rebuilt)

	rebuiltProvider := NewRoundRobinKeyProvider(rebuilt, 1)
	require.Equal(t, "k3", rebuiltProvider.Get(ctx))
	require.Equal(t, "k1", rebuiltProvider.Get(ctx))
}

func TestStickyKeyProvider_KeepsSessionAcrossSnapshotRebuild(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategySticky)
	ch.apiKeyState = svc.apiKeySelectionStateFor(ch)
	svc.publishAPIKeySelectionSnapshot(ch)
	p := newSharedStickyKeyProvider(ch)

	ctx := contexts.WithTrace(context.Background(), &ent.Trace{TraceID: "trace-1"})

	first := p.Get(ctx)

	// A rebuild must not move an ongoing session onto another key.
	rebuilt := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategySticky)
	rebuilt.apiKeyState = svc.apiKeySelectionStateFor(rebuilt)
	svc.publishAPIKeySelectionSnapshot(rebuilt)

	require.Equal(t, first, newSharedStickyKeyProvider(rebuilt).Get(ctx))
}

// A key that is disabled (or removed) after the trace picked it must not keep
// being returned from the shared cache. Upstream got this for free because the
// trace→key memory lived in the provider and died with the rebuild; here the
// memory outlives rebuilds, so a disabled key has to be re-checked.
func TestStickyKeyProvider_MovesOffDisabledKeyAfterRebuild(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategySticky)
	ch.apiKeyState = svc.apiKeySelectionStateFor(ch)
	svc.publishAPIKeySelectionSnapshot(ch)

	ctx := contexts.WithTrace(context.Background(), &ent.Trace{TraceID: "trace-pinned"})

	first := newSharedStickyKeyProvider(ch).Get(ctx)

	// Rebuild with the key the trace is pinned to now disabled, as the
	// auto-disable rules would.
	rebuilt := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}, first), objects.APIKeyStrategySticky)
	rebuilt.apiKeyState = svc.apiKeySelectionStateFor(rebuilt)
	svc.publishAPIKeySelectionSnapshot(rebuilt)

	after := newSharedStickyKeyProvider(rebuilt).Get(ctx)
	require.NotEqual(t, first, after, "a disabled sticky key must not be reused")
	require.Contains(t, rebuilt.cachedEnabledAPIKeys, after)
}

func TestStickyKeyProvider_SameTraceStaysOnItsKey(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategySticky)
	ch.apiKeyState = svc.apiKeySelectionStateFor(ch)
	svc.publishAPIKeySelectionSnapshot(ch)
	p := newSharedStickyKeyProvider(ch)

	ctx := context.Background()
	first := p.Get(contexts.WithTrace(ctx, &ent.Trace{TraceID: "trace-a"}))

	for range 10 {
		require.Equal(t, first, p.Get(contexts.WithTrace(ctx, &ent.Trace{TraceID: "trace-a"})))
	}
}
