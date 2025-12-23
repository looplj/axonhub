package orchestrator

import (
	"context"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/log"
)

// GoogleNativeToolsSelector is a decorator that prioritizes candidates supporting Google native tools.
// When a request contains Google native tools (google_search, google_url_context, google_code_execution),
// this selector filters out candidates whose channels don't support these tools (e.g., gemini_openai).
// If no compatible candidates are found, it falls back to all candidates (allowing downstream fallback logic).
type GoogleNativeToolsSelector struct {
	wrapped CandidateSelector
}

// NewGoogleNativeToolsSelector creates a selector that prioritizes Google native tool compatible candidates.
func NewGoogleNativeToolsSelector(wrapped CandidateSelector) *GoogleNativeToolsSelector {
	return &GoogleNativeToolsSelector{
		wrapped: wrapped,
	}
}

func (s *GoogleNativeToolsSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	// If request doesn't contain Google native tools, return all candidates
	if !llm.ContainsGoogleNativeTools(req.Tools) {
		return candidates, nil
	}

	// Filter: keep only candidates whose channels support Google native tools
	compatible := lo.Filter(candidates, func(c *ChannelModelCandidate, _ int) bool {
		return c.Channel.Type.SupportsGoogleNativeTools()
	})

	if len(compatible) > 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "Filtered candidates for Google native tools",
				log.Int("total_candidates", len(candidates)),
				log.Int("compatible_candidates", len(compatible)))
		}

		return compatible, nil
	}

	// No compatible candidates, return all (let downstream handle fallback)
	log.Warn(ctx, "No candidates support Google native tools, falling back to all",
		log.Int("total_candidates", len(candidates)))

	return candidates, nil
}
