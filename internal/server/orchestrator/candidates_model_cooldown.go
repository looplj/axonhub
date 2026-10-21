package orchestrator

import (
	"time"
)

// filterModelCooldownCandidates drops candidates whose channel is currently in
// a model-scoped cooldown for the requested model (e.g. client-cancellation
// cooldown). The channel stays enabled for its other models, so only the
// candidate for this exact request model is skipped.
func filterModelCooldownCandidates(candidates []*ChannelModelsCandidate, requestModel string, now time.Time) []*ChannelModelsCandidate {
	if requestModel == "" || len(candidates) == 0 {
		return candidates
	}

	filtered := make([]*ChannelModelsCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Channel.HasActiveModelCooldown(requestModel, now) {
			continue
		}
		filtered = append(filtered, candidate)
	}

	return filtered
}
