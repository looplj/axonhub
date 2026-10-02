package orchestrator

import (
	"context"
	"errors"
	"slices"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
)

// errChannelUnavailableForRetry tells the pipeline to stop retrying the current
// candidate because its channel has left service (deleted, disabled or
// archived). Spending the remaining same-channel budget on it can only fail, so
// the pipeline switches to the next candidate instead, or ends the request when
// there is none.
var errChannelUnavailableForRetry = errors.New("current channel is no longer available for retry")

// refreshChannelBeforeRetry re-reads the channel state from the database before a
// same-channel retry.
//
// A request pins the channel snapshot taken when its candidates were selected.
// When a credential is auto-disabled mid-request, the failure is recorded
// asynchronously: the performance worker evaluates the disable rule, commits the
// disable record, and notifies the shared cache afterwards. The pinned snapshot —
// and the key provider built from it — stays stale meanwhile, so the remaining
// same-channel attempts keep selecting the credential that was just disabled.
//
// The database is the authority here, not the shared cache, so the ordering this
// relies on is simply "once the invalidation is committed, the retry observes
// it". When the worker has not committed the disable record yet, nothing is
// refreshed and the retry behaves exactly like upstream.
//
// Outcomes:
//   - the candidate was not in service to begin with: leave it alone, only a
//     channel that was in service when the candidate was selected can leave it;
//   - the channel is gone from the database (including soft-deleted), is no longer
//     enabled, or has no usable credential left while the credential this attempt
//     used is gone: give up on this candidate so the pipeline switches or fails;
//   - the channel is still enabled, the credential this attempt used carries an
//     active disable record, and another usable credential remains: rebuild the
//     candidate from the stored channel, so that retries authenticate with the
//     currently enabled credentials;
//   - anything else — no database in the context, a transient lookup failure, a
//     channel served without credentials (local Ollama deployments) — leaves the
//     candidate untouched and keeps retry behavior identical to upstream.
func (p *PersistentOutboundTransformer) refreshChannelBeforeRetry(ctx context.Context) error {
	if p == nil || p.state == nil {
		return nil
	}

	candidate := p.state.CurrentCandidate
	if candidate == nil || candidate.Channel == nil || p.state.ChannelService == nil {
		return nil
	}

	// Only a channel that was in service when this candidate was selected can
	// have left it. A pinned channel that is not enabled is not "leaving"
	// anything, so the guard stays silent and the retry behaves as upstream.
	if candidate.Channel.Status != channel.StatusEnabled {
		return nil
	}

	client := ent.FromContext(ctx)
	if client == nil {
		return nil
	}

	entity, err := client.Channel.Get(ctx, candidate.Channel.ID)
	if err != nil {
		// A soft-deleted channel is filtered out of the normal query, so a
		// not-found is a confirmed removal: the channel cannot serve anymore.
		if ent.IsNotFound(err) {
			return errChannelUnavailableForRetry
		}

		// Transient lookup failure (permission denied, database hiccup): a degraded
		// read must never cancel an otherwise legitimate retry.
		return nil
	}

	if entity.Status != channel.StatusEnabled {
		return errChannelUnavailableForRetry
	}

	// Identify the credential this attempt authenticated with. OAuth channels are
	// tracked by the fixed OAuth credential reference, the same identity the
	// auto-disable bookkeeping uses; a channel served without credentials carries
	// none and has nothing to refresh.
	currentRef := currentRetryCredentialRef(ctx, p)
	if currentRef == "" {
		return nil
	}

	// Compare against the credential references the channel can be disabled on,
	// with the same expiry semantics the auto-disable bookkeeping uses. A channel
	// that still has a usable credential — including an OAuth one, which is a
	// single reference here — keeps retrying untouched.
	enabledRefs := entity.Credentials.GetEnabledCredentialRefs(entity.DisabledAPIKeys)
	if slices.Contains(enabledRefs, currentRef) {
		return nil
	}

	if len(enabledRefs) == 0 {
		// The candidate was credentialed (currentRef is set) and the channel has no
		// usable credential left — its credentials were replaced while the request
		// was in flight, for example. Continuing would keep sending a credential the
		// channel no longer accepts, so give up on this candidate.
		return errChannelUnavailableForRetry
	}

	fresh, err := p.state.ChannelService.GetChannel(ctx, entity.ID)
	if err != nil || fresh == nil {
		// Degraded rebuild: the channel changed under us, or it lost its remaining
		// credentials between the read and the rebuild. Keep the pinned candidate
		// instead of failing the whole retry, exactly as before this guard existed.
		return nil //nolint:nilerr // deliberate degradation: retry with the pinned snapshot
	}

	previous := candidate.Channel
	candidate.Channel = fresh

	rebuilt := selectOutboundForCandidate(candidate)
	if rebuilt == nil {
		// The refreshed channel offers no transformer for this protocol; keep the
		// previous one rather than leaving the request without a transformer.
		candidate.Channel = previous

		return nil
	}

	p.wrapped = rebuilt

	return nil
}

// currentRetryCredentialRef returns the credential identifier the current attempt
// authenticated with: the key the key provider selected for this request, or the
// credential recorded on the failed attempt. It returns "" for a channel that was
// served without any credential.
func currentRetryCredentialRef(ctx context.Context, p *PersistentOutboundTransformer) string {
	if key, ok := contexts.GetChannelAPIKey(ctx); ok && key != "" {
		return key
	}

	if p.state != nil && p.state.Perf != nil {
		return p.state.Perf.APIKey
	}

	return ""
}
