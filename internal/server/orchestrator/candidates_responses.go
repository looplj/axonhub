package orchestrator

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

type responsesOnlyDataSelector struct {
	wrapped CandidateSelector
	policy  biz.ResponsesOnlyDataPolicy
}

func WithResponsesOnlyDataSelector(selector CandidateSelector, policy biz.ResponsesOnlyDataPolicy) CandidateSelector {
	return &responsesOnlyDataSelector{
		wrapped: selector,
		policy:  biz.NormalizeResponsesOnlyDataPolicy(policy),
	}
}

func (s *responsesOnlyDataSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil || len(candidates) == 0 || !requiresResponsesOutbound(req) {
		return candidates, err
	}

	compatible := make([]*ChannelModelsCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		apiFormat, ok := responsesOutboundFormatForCandidate(req, candidate)
		if !ok {
			continue
		}

		cloned := *candidate
		cloned.APIFormat = string(apiFormat)
		compatible = append(compatible, &cloned)
	}

	if len(compatible) == 0 {
		if s.policy == biz.ResponsesOnlyDataPolicyDiscard {
			return candidates, nil
		}

		return nil, fmt.Errorf(
			"%w: no OpenAI Responses outbound candidates for model %q",
			errResponsesOnlyDataRequiresResponsesOutbound,
			req.Model,
		)
	}

	return compatible, nil
}

func responsesOutboundFormatForCandidate(req *llm.Request, candidate *ChannelModelsCandidate) (llm.APIFormat, bool) {
	currentFormat := selectedOutboundFormat(candidate)
	if isResponsesFormat(currentFormat) {
		return currentFormat, true
	}

	if req != nil && isResponsesFormat(req.APIFormat) && candidateHasOutboundFormat(candidate, req.APIFormat) {
		return req.APIFormat, true
	}

	for _, apiFormat := range []llm.APIFormat{llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseCompact} {
		if candidateHasOutboundFormat(candidate, apiFormat) {
			return apiFormat, true
		}
	}

	return "", false
}

func candidateHasOutboundFormat(candidate *ChannelModelsCandidate, apiFormat llm.APIFormat) bool {
	if candidate == nil || candidate.Channel == nil {
		return false
	}

	if candidate.Channel.Outbounds != nil {
		_, ok := candidate.Channel.Outbounds[string(apiFormat)]
		return ok
	}

	return selectedOutboundFormat(candidate) == apiFormat
}

func selectedOutboundFormat(candidate *ChannelModelsCandidate) llm.APIFormat {
	if candidate == nil || candidate.Channel == nil {
		return ""
	}

	if candidate.APIFormat != "" && candidate.Channel.Outbounds != nil {
		if out, ok := candidate.Channel.Outbounds[candidate.APIFormat]; ok && out != nil {
			return out.APIFormat()
		}
	}

	if candidate.Channel.Outbound != nil {
		return candidate.Channel.Outbound.APIFormat()
	}

	return llm.APIFormat(candidate.APIFormat)
}
