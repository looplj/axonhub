package orchestrator

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

// OpenAICustomToolsSelector excludes candidates that cannot carry an OpenAI
// freeform custom-tool declaration and its complete call history.
type OpenAICustomToolsSelector struct {
	wrapped CandidateSelector
}

// WithOpenAICustomToolsSelector creates a selector that filters candidates for
// OpenAI custom-tool compatibility.
func WithOpenAICustomToolsSelector(wrapped CandidateSelector) *OpenAICustomToolsSelector {
	return &OpenAICustomToolsSelector{wrapped: wrapped}
}

func (s *OpenAICustomToolsSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	if !hasOpenAICustomTools(req) {
		return candidates, nil
	}

	compatible := lo.Filter(candidates, func(candidate *ChannelModelsCandidate, _ int) bool {
		return candidateSupportsOpenAICustomTools(candidate)
	})
	if len(compatible) == 0 && len(candidates) > 0 {
		return nil, fmt.Errorf("%w: no candidate supports OpenAI custom tools", transformer.ErrInvalidRequest)
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "filtered candidates for OpenAI custom tools",
			log.Int("total_candidates", len(candidates)),
			log.Int("compatible_candidates", len(compatible)))
	}

	return compatible, nil
}

func hasOpenAICustomTools(req *llm.Request) bool {
	if req == nil {
		return false
	}

	for _, tool := range req.Tools {
		if tool.Type == llm.ToolTypeResponsesCustomTool || tool.ResponseCustomTool != nil ||
			(tool.Type == "custom" && tool.OpenAIChatCustomTool != nil) {
			return true
		}
	}

	for _, message := range req.Messages {
		for _, toolCall := range message.ToolCalls {
			if toolCall.Type == llm.ToolTypeResponsesCustomTool || toolCall.ResponseCustomToolCall != nil ||
				(toolCall.Type == "custom" && toolCall.OpenAIChatCustomToolCall != nil) {
				return true
			}
		}
	}

	return false
}

func candidateSupportsOpenAICustomTools(candidate *ChannelModelsCandidate) bool {
	if candidate == nil || candidate.Channel == nil {
		return false
	}

	switch llm.APIFormat(candidate.APIFormat) {
	case llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseCompact:
		return true
	case llm.APIFormatOpenAIChatCompletion:
		for _, endpoint := range candidate.Channel.ResolveEndpoints() {
			if endpoint.APIFormat == candidate.APIFormat {
				return endpoint.SupportsOpenAIChatCustomTools
			}
		}
		return false
	default:
		return false
	}
}
