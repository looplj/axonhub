package biz

import (
	"slices"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/looplj/axonhub/internal/objects"
)

// apiKeySelectionState is the per-channel selection state that has to outlive a
// channel snapshot.
//
// Any channel change (disabling a key, editing a channel, the periodic cleanup
// of expired disables, even a change to a *different* channel) rebuilds every
// enabled *Channel, and with it every outbound transformer and key provider.
// A strategy that remembers anything between requests would lose it on each of
// those rebuilds: the round-robin rotation would restart at the first key and an
// in-flight session would be scattered across keys. The state therefore lives on
// ChannelService, keyed by channel id; only the snapshot pointer is refreshed
// when a channel is rebuilt.
type apiKeySelectionState struct {
	mu sync.Mutex

	// strategy is the strategy the channel was last built with. The success
	// report uses it to decide whether this channel counts successes at all.
	strategy string

	// snapshot points at the freshest *Channel of this channel. Selections read
	// through it so a key disabled mid-request is honoured by the next call.
	snapshot atomic.Pointer[Channel]

	// rrCounter is the round_robin request counter. It is kept here, not in the
	// provider, so that rebuilding a snapshot does not restart the rotation at
	// the first key.
	rrCounter uint64

	// stickyCache remembers traceID → key for the sticky strategy, for the same
	// reason: a rebuild must not scatter an ongoing session across keys.
	stickyCache *lru.Cache[string, string]

	// fixedCursorKey / fixedCursorIdx remember the key the fixed strategy is
	// using and where it sits in the channel's key array.
	fixedCursorKey string
	fixedCursorIdx int

	// rrSuccessCursor / rrSuccessIdx point at the key the round_robin_success
	// strategy is currently using, rrSuccessCount is how many successful
	// requests that key has already served, and rrSuccessPer is the threshold
	// after which the cursor moves on.
	rrSuccessCursor string
	rrSuccessIdx    int
	rrSuccessCount  int
	rrSuccessPer    int
}

// apiKeySelectionStateFor returns the shared state of a channel, creating it on
// first use. Strategies that need no memory between requests get nil, so their
// channels never allocate one.
func (svc *ChannelService) apiKeySelectionStateFor(ch *Channel) *apiKeySelectionState {
	if ch == nil || !strategyKeepsSelectionState(channelAPIKeyStrategy(ch)) {
		return nil
	}

	svc.apiKeyStatesLock.Lock()

	if svc.apiKeyStates == nil {
		svc.apiKeyStates = make(map[int]*apiKeySelectionState)
	}

	state, ok := svc.apiKeyStates[ch.ID]
	if !ok {
		state = &apiKeySelectionState{}
		svc.apiKeyStates[ch.ID] = state
	}

	svc.apiKeyStatesLock.Unlock()

	state.mu.Lock()
	state.strategy = channelAPIKeyStrategy(ch)

	per := channelAPIKeySwitchAfter(ch)
	if per != state.rrSuccessPer {
		// The reuse threshold changed: the successes counted under the previous
		// threshold no longer mean anything.
		state.rrSuccessCount = 0
	}
	state.rrSuccessPer = per
	state.mu.Unlock()

	state.snapshot.Store(ch)

	return state
}

// apiKeySelectionState returns the shared state of a channel, or nil when the
// channel never registered one.
func (svc *ChannelService) apiKeySelectionState(channelID int) *apiKeySelectionState {
	svc.apiKeyStatesLock.Lock()
	defer svc.apiKeyStatesLock.Unlock()

	return svc.apiKeyStates[channelID]
}

// forgetAPIKeySelectionState drops the shared state of a channel, so a deleted
// channel does not leave state behind for the lifetime of the process.
func (svc *ChannelService) forgetAPIKeySelectionState(channelID int) {
	svc.apiKeyStatesLock.Lock()
	defer svc.apiKeyStatesLock.Unlock()

	delete(svc.apiKeyStates, channelID)
}

// nextRoundRobinKey advances the shared round-robin counter and returns the key
// for this request. Every request counts, whatever its outcome.
func (st *apiKeySelectionState) nextRoundRobinKey(ch *Channel, per int) string {
	enabled := selectableKeys(ch)
	if len(enabled) == 0 {
		return ""
	}

	if per < 1 {
		per = 1
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	index := st.rrCounter / uint64(per) % uint64(len(enabled))
	st.rrCounter++

	return enabled[index]
}

// stickyKeyFor returns the sticky key of a trace, remembering the choice on the
// shared state so a channel rebuild keeps the session on the same key. The
// remembered key wins even if the enabled set changed, mirroring the upstream
// provider's behaviour.
func (st *apiKeySelectionState) stickyKeyFor(ch *Channel, traceID string) string {
	enabled := ch.cachedEnabledAPIKeys
	if len(enabled) == 0 {
		return ch.Credentials.APIKeys[0]
	}

	if len(enabled) == 1 {
		return enabled[0]
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	if st.stickyCache == nil {
		st.stickyCache, _ = lru.New[string, string](traceStickyLRUSize)
	}

	if cached, ok := st.stickyCache.Get(traceID); ok {
		return cached
	}

	selected := rendezvousSelect(enabled, traceID)
	st.stickyCache.Add(traceID, selected)

	return selected
}

// selectFixedKey returns the key the fixed strategy should use and records it.
//
// The key in use wins as long as it is still selectable. Otherwise the walk
// starts at that key's current position, or at its remembered position when the
// key has been removed from the channel, and moves forward through the full key
// array (wrapping around) skipping disabled keys.
func (st *apiKeySelectionState) selectFixedKey(ch *Channel) string {
	all := ch.Credentials.GetAllAPIKeys()
	if len(all) == 0 {
		return ""
	}

	disabled := ch.cachedDisabledKeySet

	st.mu.Lock()
	defer st.mu.Unlock()

	if idx := slices.Index(all, st.fixedCursorKey); idx >= 0 {
		if _, isDisabled := disabled[st.fixedCursorKey]; !isDisabled {
			st.fixedCursorIdx = idx

			return st.fixedCursorKey
		}
	}

	selectedKey, idx := firstSelectableFrom(all, disabled, positionOf(all, st.fixedCursorKey, st.fixedCursorIdx))
	st.fixedCursorKey = selectedKey
	st.fixedCursorIdx = idx

	return selectedKey
}

// selectRoundRobinSuccessKey returns the key the round_robin_success strategy
// should use. It counts nothing: the counter is advanced by the success report
// (see onAPIKeySuccess), so only successful calls move the cursor.
//
// The cursor key is reused while it stays selectable. When it is disabled or
// removed, the cursor moves forward through the full key array (wrapping around)
// and the success counter restarts.
func (st *apiKeySelectionState) selectRoundRobinSuccessKey(ch *Channel) string {
	all := ch.Credentials.GetAllAPIKeys()
	if len(all) == 0 {
		return ""
	}

	disabled := ch.cachedDisabledKeySet

	st.mu.Lock()
	defer st.mu.Unlock()

	if idx := slices.Index(all, st.rrSuccessCursor); idx >= 0 {
		if _, isDisabled := disabled[st.rrSuccessCursor]; !isDisabled {
			st.rrSuccessIdx = idx

			return st.rrSuccessCursor
		}
	}

	selectedKey, idx := firstSelectableFrom(all, disabled, positionOf(all, st.rrSuccessCursor, st.rrSuccessIdx))
	st.rrSuccessCursor = selectedKey
	st.rrSuccessIdx = idx
	st.rrSuccessCount = 0

	return selectedKey
}

// onAPIKeySuccess advances the round_robin_success cursor after a successful
// request. It is a no-op for every other strategy, and for reports that do not
// belong to the key currently in use: a late success report for a key the cursor
// has already moved past must not be counted.
func (svc *ChannelService) onAPIKeySuccess(channelID int, apiKey string) {
	if apiKey == "" {
		return
	}

	state := svc.apiKeySelectionState(channelID)
	if state == nil {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.strategy != objects.APIKeyStrategyRoundRobinSuccess {
		return
	}

	if state.rrSuccessCursor == "" || state.rrSuccessCursor != apiKey {
		return
	}

	state.rrSuccessCount++

	per := state.rrSuccessPer
	if per < 1 {
		per = 1
	}

	if state.rrSuccessCount < per {
		return
	}

	ch := state.snapshot.Load()
	if ch == nil {
		state.rrSuccessCount = 0

		return
	}

	all := ch.Credentials.GetAllAPIKeys()
	if len(all) == 0 {
		state.rrSuccessCount = 0

		return
	}

	// Move past the key that just finished its turn, not back onto it.
	selectedKey, idx := firstSelectableFrom(
		all,
		ch.cachedDisabledKeySet,
		positionOf(all, state.rrSuccessCursor, state.rrSuccessIdx)+1,
	)

	state.rrSuccessCursor = selectedKey
	state.rrSuccessIdx = idx
	state.rrSuccessCount = 0
}
