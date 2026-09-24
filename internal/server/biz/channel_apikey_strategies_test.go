package biz

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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

	return NewRoundRobinSuccessKeyProvider(ch), state
}

// --- A. dispatch ---

func TestNewMultiKeyProvider_Dispatch(t *testing.T) {
	ch := newKeyChannel(1, []string{"k1", "k2"})

	// nil settings -> sticky (default).
	require.IsType(t, &TraceStickyKeyProvider{}, newMultiKeyProvider(ch))

	cases := []struct {
		strategy string
		want     any
	}{
		{"", &TraceStickyKeyProvider{}},
		{objects.APIKeyStrategySticky, &TraceStickyKeyProvider{}},
		{objects.APIKeyStrategyRandom, &RandomKeyProvider{}},
		{objects.APIKeyStrategyRoundRobin, &RoundRobinKeyProvider{}},
		{objects.APIKeyStrategyRoundRobinSuccess, &RoundRobinSuccessKeyProvider{}},
		{objects.APIKeyStrategyPriority, &PriorityKeyProvider{}},
		{objects.APIKeyStrategyFixed, &FixedKeyProvider{}},
		{"unknown_value", &TraceStickyKeyProvider{}},
	}
	for _, tc := range cases {
		strategy := tc.strategy
		ch.Settings = &objects.ChannelSettings{APIKeyStrategy: &strategy}
		require.IsTypef(t, tc.want, newMultiKeyProvider(ch), "strategy %q", tc.strategy)
	}
}

func TestStrategyKeepsSelectionState(t *testing.T) {
	require.True(t, strategyKeepsSelectionState(objects.APIKeyStrategyFixed))
	require.True(t, strategyKeepsSelectionState(objects.APIKeyStrategyRoundRobinSuccess))

	for _, strategy := range []string{
		"", objects.APIKeyStrategySticky, objects.APIKeyStrategyRandom,
		objects.APIKeyStrategyRoundRobin, objects.APIKeyStrategyPriority,
	} {
		require.Falsef(t, strategyKeepsSelectionState(strategy), "strategy %q", strategy)
	}
}

func TestAPIKeySelectionStateFor_OnlyRegistersRememberingStrategies(t *testing.T) {
	svc := &ChannelService{}

	fixed := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyFixed)
	require.NotNil(t, svc.apiKeySelectionStateFor(fixed))

	sticky := withStrategy(newKeyChannel(2, []string{"k1", "k2"}), objects.APIKeyStrategySticky)
	require.Nil(t, svc.apiKeySelectionStateFor(sticky))
	require.Nil(t, svc.apiKeySelectionState(2))
}

// --- B. priority ---

func TestPriorityKeyProvider_AlwaysFirstSelectable(t *testing.T) {
	ch := newKeyChannel(1, []string{"first", "second", "third"})
	p := NewPriorityKeyProvider(ch)

	ctx := context.Background()
	for i := 0; i < 10; i++ {
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
	for i := 0; i < 20; i++ {
		require.Equal(t, "k2", p.Get(ctx), "call %d", i)
	}
}

func TestFixedKeyProvider_MovesForwardWhenCurrentKeyIsDisabled(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyFixed)
	p, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k2", 1)

	// k2 gets disabled. The next selection must move on to k3, never back to k1.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}, "k2"), objects.APIKeyStrategyFixed)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k3", p.Get(context.Background()))
}

func TestFixedKeyProvider_SkipsSeveralDisabledKeysOnTheWay(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4", "k5"}), objects.APIKeyStrategyFixed)
	p, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k2", 1)

	// k2, k3 and k4 are disabled in one go (a retry loop can disable several).
	reloaded := withStrategy(
		newKeyChannel(1, []string{"k1", "k2", "k3", "k4", "k5"}, "k2", "k3", "k4"),
		objects.APIKeyStrategyFixed,
	)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k5", p.Get(context.Background()))
}

func TestFixedKeyProvider_WrapsAroundToFirstSelectable(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	p, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k3", 2)

	// The tail is disabled, so the walk wraps around and lands on k1.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}, "k3"), objects.APIKeyStrategyFixed)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k1", p.Get(context.Background()))
}

func TestFixedKeyProvider_RemovedKeyContinuesForward(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyFixed)
	p, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k3", 2)

	// k3 is deleted from the channel: the walk continues from where it used to
	// sit, so k4 is next rather than restarting at k1.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k4"}), objects.APIKeyStrategyFixed)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k4", p.Get(context.Background()))
}

func TestFixedKeyProvider_RemovedKeyOutOfRangeWrapsAround(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyFixed)
	p, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k4", 3)

	// The array shrank below the remembered position, so the walk restarts at the
	// head of the array instead of reading past the end.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyFixed)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k1", p.Get(context.Background()))
}

func TestFixedKeyProvider_RemovedKeyThenSkipsDisabledTail(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyFixed)
	p, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k3", 2)

	// k3 was removed and the key that took its place is disabled: the walk keeps
	// going and wraps around to k1.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k4"}, "k4"), objects.APIKeyStrategyFixed)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k1", p.Get(context.Background()))
}

func TestFixedKeyProvider_AllDisabledFallsBackToFirst(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2"}), objects.APIKeyStrategyFixed)
	p, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k2", 1)

	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2"}, "k1", "k2"), objects.APIKeyStrategyFixed)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k1", p.Get(context.Background()))
}

func TestFixedKeyProvider_UsesFreshestSnapshot(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	p, _ := newFixedProvider(t, svc, ch)

	require.Equal(t, "k1", p.Get(context.Background()))

	// A key disabled while the request is in flight must be honoured by the next
	// selection even though this provider was built from the older snapshot.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}, "k1"), objects.APIKeyStrategyFixed)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k2", p.Get(context.Background()))
}

func TestFixedKeyProvider_KeepsKeyWhenTheArrayShifts(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	p, state := newFixedProvider(t, svc, ch)

	setFixedCursor(state, "k2", 1)

	// A new key is prepended: positions shift, but the remembered key is still
	// selectable, so it keeps being used.
	reloaded := withStrategy(newKeyChannel(1, []string{"k0", "k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k2", p.Get(context.Background()))
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
	for i := 0; i < 10; i++ {
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
	svc.apiKeySelectionStateFor(other)

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
	p, state := newRoundRobinSuccessProvider(t, svc, ch)

	setRoundRobinSuccessCursor(state, "k1", 0, 3)

	// k1 is disabled: the cursor moves on and the successes counted for k1 are
	// discarded.
	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}, "k1"), objects.APIKeyStrategyRoundRobinSuccess, 5)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k2", p.Get(context.Background()))

	state.mu.Lock()
	count := state.rrSuccessCount
	state.mu.Unlock()

	require.Zero(t, count)
}

func TestRoundRobinSuccessKeyProvider_RemovedCursorKeyContinuesForward(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	p, state := newRoundRobinSuccessProvider(t, svc, ch)

	setRoundRobinSuccessCursor(state, "k3", 2, 0)

	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k4"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k4", p.Get(context.Background()))
}

func TestRoundRobinSuccessKeyProvider_UsesFreshestSnapshot(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	require.Equal(t, "k1", p.Get(context.Background()))

	reloaded := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}, "k1"), objects.APIKeyStrategyRoundRobinSuccess, 1)
	svc.apiKeySelectionStateFor(reloaded)

	require.Equal(t, "k2", p.Get(context.Background()))
}

func TestRoundRobinSuccessKeyProvider_AdvancesPastDisabledKeys(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}), objects.APIKeyStrategyRoundRobinSuccess, 1)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	require.Equal(t, "k1", p.Get(context.Background()))

	// k2 and k3 are disabled before k1 finishes its turn: the cursor must land on
	// k4 rather than on a disabled key.
	reloaded := withStrategy(
		newKeyChannel(1, []string{"k1", "k2", "k3", "k4"}, "k2", "k3"),
		objects.APIKeyStrategyRoundRobinSuccess,
		1,
	)
	svc.apiKeySelectionStateFor(reloaded)

	svc.onAPIKeySuccess(1, "k1")

	require.Equal(t, "k4", p.Get(context.Background()))
}

// --- F. concurrency ---

func TestFixedKeyProvider_ConcurrentGetIsConsistent(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyFixed)
	p, _ := newFixedProvider(t, svc, ch)

	var wg sync.WaitGroup

	for i := 0; i < 32; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			ctx := context.Background()
			for j := 0; j < 50; j++ {
				require.Equal(t, "k1", p.Get(ctx))
			}
		}()
	}

	wg.Wait()
}

func TestRoundRobinSuccessKeyProvider_ConcurrentGetAndSuccess(t *testing.T) {
	svc := &ChannelService{}
	ch := withStrategy(newKeyChannel(1, []string{"k1", "k2", "k3"}), objects.APIKeyStrategyRoundRobinSuccess, 2)
	p, _ := newRoundRobinSuccessProvider(t, svc, ch)

	var wg sync.WaitGroup

	for i := 0; i < 16; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			ctx := context.Background()
			for j := 0; j < 50; j++ {
				key := p.Get(ctx)
				svc.onAPIKeySuccess(1, key)
			}
		}()
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
