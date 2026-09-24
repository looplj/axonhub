package biz

import (
	"context"
	"math/rand/v2"
	"sync/atomic"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/auth"
)

// newMultiKeyProvider builds the key provider for a channel with more than one
// enabled key, dispatching on ChannelSettings.APIKeyStrategy. Unknown or empty
// strategies fall back to the historical sticky behavior.
func newMultiKeyProvider(ch *Channel) auth.APIKeyProvider {
	var strategy string
	var per int
	if ch.Settings != nil {
		if s := ch.Settings.APIKeyStrategy; s != nil {
			strategy = *s
		}
		if n := ch.Settings.APIKeyRoundRobinSwitchAfter; n != nil {
			per = *n
		}
	}

	switch strategy {
	case objects.APIKeyStrategyRandom:
		return NewRandomKeyProvider(ch)
	case objects.APIKeyStrategyRoundRobin:
		return NewRoundRobinKeyProvider(ch, per)
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
// key for `per` consecutive requests before moving to the next. The counter is
// in-process and resets whenever the channel (and thus this provider) is rebuilt.
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

// FixedKeyProvider always returns the first enabled key. Failover to the next key
// is delegated to the channel's per-key auto-disable rules: once the leading key
// is disabled and the channel reloads, the next key becomes the first enabled one.
type FixedKeyProvider struct {
	channel *Channel
}

func NewFixedKeyProvider(channel *Channel) *FixedKeyProvider {
	return &FixedKeyProvider{channel: channel}
}

func (p *FixedKeyProvider) Get(ctx context.Context) string {
	enabled := selectableKeys(p.channel)
	selectedKey := enabled[0]

	contexts.WithChannelAPIKey(ctx, selectedKey)

	return selectedKey
}
