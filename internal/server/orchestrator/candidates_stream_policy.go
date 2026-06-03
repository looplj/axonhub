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

	if req.Stream != nil && *req.Stream {
		filtered := lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
			return streamPolicyOf(c) != objects.CapabilityPolicyForbid
		})

		return filtered, nil
	}

	nativeCandidates := lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		return streamPolicyOf(c) != objects.CapabilityPolicyRequire
	})
	if len(nativeCandidates) > 0 {
		return nativeCandidates, nil
	}

	if supportsAutoAggregateRequest(req) {
		return candidates, nil
	}

	return nil, nil
}

func streamPolicyOf(candidate *ChannelModelsCandidate) objects.CapabilityPolicy {
	if candidate == nil || candidate.Channel == nil {
		return objects.CapabilityPolicyUnlimited
	}

	if candidate.Channel.Policies.Stream != "" {
		return candidate.Channel.Policies.Stream
	}

	return objects.CapabilityPolicyUnlimited
}

func supportsAutoAggregateRequest(req *llm.Request) bool {
	if req == nil {
		return false
	}

	switch req.RequestType {
	case "", llm.RequestTypeChat:
		switch req.APIFormat {
		case "",
			llm.APIFormatOpenAIChatCompletion,
			llm.APIFormatOpenAIResponse,
			llm.APIFormatAnthropicMessage,
			llm.APIFormatGeminiContents,
			llm.APIFormatOllamaChat:
			return true
		default:
			return false
		}
	case llm.RequestTypeCompletion:
		return req.APIFormat == "" || req.APIFormat == llm.APIFormatOpenAICompletion
	default:
		return false
	}
}
