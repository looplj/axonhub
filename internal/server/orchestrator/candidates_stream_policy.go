package orchestrator

import (
	"context"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

type StreamPolicySelector struct {
	wrapped CandidateSelector
}

func WithStreamPolicySelector(wrapped CandidateSelector) *StreamPolicySelector {
	return &StreamPolicySelector{wrapped: wrapped}
}

func (s *StreamPolicySelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return candidates, nil
	}

	wantStream := req.Stream != nil && *req.Stream

	filtered := lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		policy := objects.CapabilityPolicyUnlimited
		if c.Channel.Policies.Stream != "" {
			policy = c.Channel.Policies.Stream
		}

		//nolint:exhaustive // Checked.
		switch policy {
		case objects.CapabilityPolicyRequire:
			return wantStream
		case objects.CapabilityPolicyForbid:
			return !wantStream
		default:
			return true
		}
	})

	return filtered, nil
}
