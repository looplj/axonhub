package biz

import (
	"context"
	"math/rand/v2"
	"slices"
	"sync/atomic"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/auth"
)

// channelAPIKeyStrategy returns the multi-key strategy configured on a channel.
// An empty result means the field was never set, which falls back to sticky.
func channelAPIKeyStrategy(ch *Channel) string {
	if ch == nil || ch.Settings == nil || ch.Settings.APIKeyStrategy == nil {
		return ""
	}

	return *ch.Settings.APIKeyStrategy
}

// channelAPIKeySwitchAfter returns how many times a key is reused by the two
// round-robin strategies before the cursor moves on. Values below 1 mean "move
// on immediately", matching the provider-level normalization.
func channelAPIKeySwitchAfter(ch *Channel) int {
	if ch == nil || ch.Settings == nil || ch.Settings.APIKeyRoundRobinSwitchAfter == nil {
		return 1
	}

	if n := *ch.Settings.APIKeyRoundRobinSwitchAfter; n >= 1 {
		return n
	}

	return 1
}

// strategyKeepsSelectionState reports whether a strategy remembers which key is
// in use across channel snapshots, and therefore needs the shared per-channel
// state owned by ChannelService.
func strategyKeepsSelectionState(strategy string) bool {
	return strategy == objects.APIKeyStrategyFixed ||
		strategy == objects.APIKeyStrategyRoundRobinSuccess
}

// newMultiKeyProvider builds the key provider for a channel with more than one
// enabled key, dispatching on ChannelSettings.APIKeyStrategy. Unknown or empty
// strategies fall back to the historical sticky behavior.
func newMultiKeyProvider(ch *Channel) auth.APIKeyProvider {
	switch channelAPIKeyStrategy(ch) {
	case objects.APIKeyStrategyRandom:
		return NewRandomKeyProvider(ch)
	case objects.APIKeyStrategyRoundRobin:
		return NewRoundRobinKeyProvider(ch, channelAPIKeySwitchAfter(ch))
	case objects.APIKeyStrategyRoundRobinSuccess:
		return NewRoundRobinSuccessKeyProvider(ch)
	case objects.APIKeyStrategyPriority:
		return NewPriorityKeyProvider(ch)
	case objects.APIKeyStrategyFixed:
		return NewFixedKeyProvider(ch)
	case objects.APIKeyStrategySticky, "":
		return NewTraceStickyKeyProvider(ch)
	default:
		return NewTraceStickyKeyProvider(ch)
	}
}

// selectableKeys returns the enabled keys snapshot plus the fallback used when
// no key is enabled (mirrors TraceStickyKeyProvider's edge handling).
func selectableKeys(ch *Channel) []string {
	enabled := ch.cachedEnabledAPIKeys
	if len(enabled) == 0 {
		return []string{ch.Credentials.APIKeys[0]}
	}
	return enabled
}

// currentChannel returns the freshest snapshot known for a channel. The
// strategies that keep shared state read through it so a key disabled while the
// request is in flight (by the auto-disable rules, for example) is honoured by
// the next selection instead of only by a rebuilt request.
func currentChannel(built *Channel, state *apiKeySelectionState) *Channel {
	if state != nil {
		if latest := state.snapshot.Load(); latest != nil {
			return latest
		}
	}

	return built
}

// positionOf returns the index of cursorKey inside all. When the key is absent
// (removed from the channel) it returns cursorIdx, so a walk continues from
// where the key used to sit instead of restarting at the head of the array.
func positionOf(all []string, cursorKey string, cursorIdx int) int {
	if idx := slices.Index(all, cursorKey); idx >= 0 {
		return idx
	}

	return cursorIdx
}

// firstSelectableFrom walks the full key array forward from start (wrapping
// around) and returns the first key without an active disable record, together
// with its index. When every key is disabled it returns the first key as a
// defensive fallback: the channel itself is taken out of service in that case,
// so this selection is never actually used.
func firstSelectableFrom(all []string, disabled map[string]struct{}, start int) (string, int) {
	n := len(all)
	if n == 0 {
		return "", -1
	}

	if start < 0 || start >= n {
		start = 0
	}

	for i := 0; i < n; i++ {
		idx := (start + i) % n
		if _, isDisabled := disabled[all[idx]]; !isDisabled {
			return all[idx], idx
		}
	}

	return all[0], 0
}

// RandomKeyProvider selects an API key uniformly at random on every call,
// regardless of whether the request carries a trace.
type RandomKeyProvider struct {
	channel *Channel
}

func NewRandomKeyProvider(channel *Channel) *RandomKeyProvider {
	return &RandomKeyProvider{channel: channel}
}

func (p *RandomKeyProvider) Get(ctx context.Context) string {
	enabled := selectableKeys(p.channel)

	//nolint:gosec // not a security issue, just a random selection.
	selectedKey := enabled[rand.IntN(len(enabled))]

	contexts.WithChannelAPIKey(ctx, selectedKey)

	return selectedKey
}

// RoundRobinKeyProvider advances through the enabled keys in order, reusing each
// key for `per` consecutive requests before moving to the next. Every request
// counts, whatever its outcome. The counter is in-process and resets whenever
// the channel (and thus this provider) is rebuilt.
type RoundRobinKeyProvider struct {
	channel *Channel
	per     uint64
	counter atomic.Uint64
}

func NewRoundRobinKeyProvider(channel *Channel, per int) *RoundRobinKeyProvider {
	if per < 1 {
		per = 1
	}
	return &RoundRobinKeyProvider{
		channel: channel,
		per:     uint64(per),
	}
}

func (p *RoundRobinKeyProvider) Get(ctx context.Context) string {
	enabled := selectableKeys(p.channel)

	index := (p.counter.Add(1) - 1) / p.per % uint64(len(enabled))
	selectedKey := enabled[index]

	contexts.WithChannelAPIKey(ctx, selectedKey)

	return selectedKey
}

// RoundRobinSuccessKeyProvider is round-robin that only counts successful
// calls. It keeps returning the cursor's key; the cursor is advanced by the
// success report (ChannelService.onAPIKeySuccess), which runs after the response
// arrives, once the key has served its share of successful requests.
//
// The cursor and its success counter live in the shared apiKeySelectionState so
// they survive channel reloads and so the report can advance the very cursor
// this provider reads.
type RoundRobinSuccessKeyProvider struct {
	channel *Channel
	state   *apiKeySelectionState
}

func NewRoundRobinSuccessKeyProvider(channel *Channel) *RoundRobinSuccessKeyProvider {
	return &RoundRobinSuccessKeyProvider{channel: channel, state: channel.apiKeyState}
}

func (p *RoundRobinSuccessKeyProvider) Get(ctx context.Context) string {
	if p.state == nil {
		// Defensive: without shared state there is nothing to advance, so behave
		// like a static first-key selection.
		enabled := selectableKeys(p.channel)
		contexts.WithChannelAPIKey(ctx, enabled[0])

		return enabled[0]
	}

	selectedKey := p.state.selectRoundRobinSuccessKey(currentChannel(p.channel, p.state))

	contexts.WithChannelAPIKey(ctx, selectedKey)

	return selectedKey
}

// PriorityKeyProvider always returns the first selectable key. Failover to the
// next key is delegated to the channel's per-key auto-disable rules: once the
// leading key is disabled, the next key becomes the first selectable one.
type PriorityKeyProvider struct {
	channel *Channel
}

func NewPriorityKeyProvider(channel *Channel) *PriorityKeyProvider {
	return &PriorityKeyProvider{channel: channel}
}

func (p *PriorityKeyProvider) Get(ctx context.Context) string {
	enabled := selectableKeys(p.channel)
	selectedKey := enabled[0]

	contexts.WithChannelAPIKey(ctx, selectedKey)

	return selectedKey
}

// FixedKeyProvider keeps using the same key until that key stops being
// selectable. Unlike PriorityKeyProvider it does not fall back to the head of
// the list: when the key in use is disabled or removed, the selection walks the
// full key array forward from that key's position (wrapping around) and skips
// every disabled key on the way.
//
// The selection lives in the shared apiKeySelectionState rather than in this
// provider, because disabling a key rebuilds the channel snapshot and therefore
// this provider: a rebuilt provider must still know where it was.
type FixedKeyProvider struct {
	channel *Channel
	state   *apiKeySelectionState
}

func NewFixedKeyProvider(channel *Channel) *FixedKeyProvider {
	return &FixedKeyProvider{channel: channel, state: channel.apiKeyState}
}

func (p *FixedKeyProvider) Get(ctx context.Context) string {
	if p.state == nil {
		// Defensive: without shared state there is nothing to remember, so fall
		// back to the first selectable key.
		enabled := selectableKeys(p.channel)
		contexts.WithChannelAPIKey(ctx, enabled[0])

		return enabled[0]
	}

	selectedKey := p.state.selectFixedKey(currentChannel(p.channel, p.state))

	contexts.WithChannelAPIKey(ctx, selectedKey)

	return selectedKey
}
