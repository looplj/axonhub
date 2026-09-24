package orchestrator

import (
	"context"
	"slices"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/objects"
)

// refreshChannelBeforeRetry re-reads the channel state before a same-channel retry.
//
// A request keeps the channel snapshot taken when its candidates were selected.
// Credential auto-disable updates the stored channel and reloads the shared
// cache, but that snapshot, and the key provider built from it, stays stale: the
// retries keep picking the credential that was just disabled and spend the whole
// same-channel retry budget on it.
//
// When the credential in use is no longer part of the channel's enabled set, the
// candidate is swapped onto the latest snapshot so that the rebuilt key provider
// selects from the current enabled credentials.
//
// Every other case leaves the candidate untouched, so retry behavior stays
// identical to upstream.
func (p *PersistentOutboundTransformer) refreshChannelBeforeRetry(ctx context.Context) {
	if p == nil || p.state == nil {
		return
	}

	candidate := p.state.CurrentCandidate
	if candidate == nil || candidate.Channel == nil || p.state.ChannelService == nil {
		return
	}

	fresh := p.state.ChannelService.GetEnabledChannel(candidate.Channel.ID)
	if fresh == nil {
		// Nothing to compare against. The channel may have left the enabled set,
		// but it may equally be one that never entered the shared cache - a
		// channel selected by ID, or a test-only channel. Treating that as
		// "disabled" would skip legitimate same-channel retries, so leave the
		// candidate untouched instead.
		return
	}

	if fresh == candidate.Channel {
		return
	}

	// OAuth channels authenticate from their credentials and never pass through
	// the key provider, so there is no credential to compare.
	currentKey, ok := contexts.GetChannelAPIKey(ctx)
	if !ok || currentKey == "" || currentKey == objects.OAuthCredentialRef {
		return
	}

	// Compare against the set the key provider actually uses: credentials minus
	// disable records that have not expired. The snapshot's O(1) disabled set is
	// deliberately not used here, because it also keeps expired temporary
	// disables and would trigger a needless refresh.
	if slices.Contains(fresh.Credentials.GetEnabledAPIKeys(fresh.DisabledAPIKeys), currentKey) {
		return
	}

	previous := candidate.Channel
	candidate.Channel = fresh

	rebuilt := selectOutboundForCandidate(candidate)
	if rebuilt == nil {
		// The new snapshot offers no transformer for this protocol; keep the
		// previous one rather than leaving the request without a transformer.
		candidate.Channel = previous

		return
	}

	p.wrapped = rebuilt
}
