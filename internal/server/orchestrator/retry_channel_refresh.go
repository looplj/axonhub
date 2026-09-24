package orchestrator

import (
	"context"
	"errors"
	"slices"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// errChannelUnavailableForRetry tells the pipeline to stop retrying the current
// candidate because the channel has left service (disabled, archived or deleted).
// Spending the remaining same-channel budget on it can only fail, so the pipeline
// switches to the next candidate instead, or ends the request when there is none.
var errChannelUnavailableForRetry = errors.New("current channel is no longer available for retry")

// refreshChannelBeforeRetry re-reads the channel state before a same-channel retry.
//
// A request keeps the channel snapshot taken when its candidates were selected.
// Credential auto-disable updates the stored channel and reloads the shared
// cache, but that snapshot, and the key provider built from it, stays stale: the
// retries keep picking the credential that was just disabled and spend the whole
// same-channel retry budget on it.
//
// Two outcomes:
//   - the channel is still in the enabled set but the credential in use was
//     disabled: swap the candidate onto the latest snapshot so the rebuilt key
//     provider selects from the current enabled credentials;
//   - the channel left the enabled set and the database confirms it went out of
//     service: give up on this candidate so the pipeline switches or fails.
//
// Every other case leaves the candidate untouched, so retry behavior stays
// identical to upstream.
func (p *PersistentOutboundTransformer) refreshChannelBeforeRetry(ctx context.Context) error {
	if p == nil || p.state == nil {
		return nil
	}

	candidate := p.state.CurrentCandidate
	if candidate == nil || candidate.Channel == nil || p.state.ChannelService == nil {
		return nil
	}

	fresh := p.state.ChannelService.GetEnabledChannel(candidate.Channel.ID)
	if fresh == nil {
		// Absence from the enabled set is not proof of a disable: the channel may
		// never have been in the shared cache (selected by ID, or a cache that has
		// not loaded yet). Only a channel that was in service when this candidate
		// was selected can leave it, so ask the database and stay silent unless it
		// says the channel really went out of service.
		if channelLeftService(ctx, candidate.Channel) {
			return errChannelUnavailableForRetry
		}

		return nil
	}

	if fresh == candidate.Channel {
		return nil
	}

	// OAuth channels authenticate from their credentials and never pass through
	// the key provider, so there is no credential to compare.
	currentKey, ok := contexts.GetChannelAPIKey(ctx)
	if !ok || currentKey == "" || currentKey == objects.OAuthCredentialRef {
		return nil
	}

	// Compare against the set the key provider actually uses: credentials minus
	// disable records that have not expired. The snapshot's O(1) disabled set is
	// deliberately not used here, because it also keeps expired temporary
	// disables and would trigger a needless refresh.
	if slices.Contains(fresh.Credentials.GetEnabledAPIKeys(fresh.DisabledAPIKeys), currentKey) {
		return nil
	}

	previous := candidate.Channel
	candidate.Channel = fresh

	rebuilt := selectOutboundForCandidate(candidate)
	if rebuilt == nil {
		// The new snapshot offers no transformer for this protocol; keep the
		// previous one rather than leaving the request without a transformer.
		candidate.Channel = previous

		return nil
	}

	p.wrapped = rebuilt

	return nil
}

// channelLeftService reports whether a channel that was in service when its
// candidate was selected has since gone out of service: not enabled anymore, or
// left with no usable credential.
//
// The snapshot check is what keeps this guard safe. A channel that was never
// enabled (a fixture, a channel picked by ID) is not "leaving" anything, so the
// guard stays silent and the retry behaves exactly as upstream.
//
// It reads the entity on purpose instead of calling ChannelService.GetChannel:
// that helper rebuilds the channel's outbound transformers, which panics when a
// channel has no enabled key, and a panic here would surface as a retryable 500
// and restart the very retry storm this guard exists to stop. Any lookup failure
// (no database in the context, permission denied) returns false, so a degraded
// read can never cancel a legitimate retry.
func channelLeftService(ctx context.Context, snapshot *biz.Channel) bool {
	if snapshot == nil || snapshot.Status != channel.StatusEnabled {
		return false
	}

	client := ent.FromContext(ctx)
	if client == nil {
		return false
	}

	entity, err := client.Channel.Get(ctx, snapshot.ID)
	if err != nil {
		return false
	}

	if entity.Status != channel.StatusEnabled {
		return true
	}

	return len(entity.Credentials.GetEnabledAPIKeys(entity.DisabledAPIKeys)) == 0
}
