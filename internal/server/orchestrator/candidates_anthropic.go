package orchestrator

import (
	"context"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
)

// AnthropicNativeToolsSelector is a decorator that prioritizes candidates supporting Anthropic native tools.
// When a request contains Anthropic native tools (web_search -> web_search_20250305),
// this selector filters out candidates whose channels don't support these tools (e.g., deepseek_anthropic).
// If no compatible candidates are found, it falls back to all candidates (allowing downstream fallback logic).
type AnthropicNativeToolsSelector struct {
	wrapped CandidateSelector
}

// WithAnthropicNativeToolsSelector creates a selector that prioritizes Anthropic native tool compatible candidates.
func WithAnthropicNativeToolsSelector(wrapped CandidateSelector) *AnthropicNativeToolsSelector {
	return &AnthropicNativeToolsSelector{
		wrapped: wrapped,
	}
}

func (s *AnthropicNativeToolsSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	// If request doesn't contain Anthropic native tools, return all candidates
	if !llm.ContainsAnthropicNativeTools(req.Tools) {
		return candidates, nil
	}

	// Filter: keep only candidates whose channels support Anthropic native tools
	compatible := lo.Filter(candidates, func(c *ChannelModelCandidate, _ int) bool {
		return c.Channel.Type.SupportsAnthropicNativeTools()
	})

	if len(compatible) > 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "Filtered candidates for Anthropic native tools",
				log.Int("total_candidates", len(candidates)),
				log.Int("compatible_candidates", len(compatible)))
		}

		return compatible, nil
	}

	// No compatible candidates, return all (let downstream handle fallback)
	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "No candidates support Anthropic native tools, falling back to all",
			log.Int("total_candidates", len(candidates)))
	}

	return candidates, nil
}
