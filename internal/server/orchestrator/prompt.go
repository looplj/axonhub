package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

type PromptProvider interface {
	GetEnabledPrompts(ctx context.Context, projectID int) ([]*ent.Prompt, error)
}

type stubPromptProvider struct {
	prompts []*ent.Prompt
	err     error
}

func (s *stubPromptProvider) GetEnabledPrompts(_ context.Context, _ int) ([]*ent.Prompt, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.prompts, nil
}

func injectPrompts(inbound *PersistentInboundTransformer) pipeline.Middleware {
	matcher := biz.NewPromptMatcher()

	return pipeline.OnLlmRequest("inject-prompts", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		projectID, ok := contexts.GetProjectID(ctx)
		if !ok {
			log.Debug(ctx, "no project ID in context, skipping prompt injection")
			return llmRequest, nil
		}

		enabledPrompts, err := inbound.state.PromptProvider.GetEnabledPrompts(ctx, projectID)
		if err != nil {
			log.Warn(ctx, "failed to get enabled prompts", log.Int("project_id", projectID), log.Cause(err))
			return llmRequest, nil
		}

		if len(enabledPrompts) == 0 {
			return llmRequest, nil
		}

		matchingPrompts := matcher.FilterMatchingPrompts(enabledPrompts, llmRequest.Model)
		if len(matchingPrompts) == 0 {
			log.Debug(ctx, "no matching prompts for model",
				log.String("model", llmRequest.Model),
				log.Int("enabled_count", len(enabledPrompts)),
			)

			return llmRequest, nil
		}

		log.Debug(ctx, "injecting prompts",
			log.String("model", llmRequest.Model),
			log.Int("matching_count", len(matchingPrompts)),
		)

		llmRequest = matcher.ApplyPrompts(llmRequest, matchingPrompts)

		return llmRequest, nil
	})
}
