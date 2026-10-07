package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
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

		var apiKeyID int
		if apiKey, ok := contexts.GetAPIKey(ctx); ok {
			apiKeyID = apiKey.ID
		}

		matchingPrompts := matcher.FilterMatchingPrompts(enabledPrompts, llmRequest.Model, apiKeyID)
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

		if llmRequest.APIFormat == llm.APIFormatOpenAIDecisions {
			if err := injectDecisionsPrompts(llmRequest, matchingPrompts); err != nil {
				return nil, fmt.Errorf("inject decisions prompts: %w", err)
			}
		} else {
			llmRequest = matcher.ApplyPrompts(llmRequest, matchingPrompts)
		}

		return llmRequest, nil
	})
}

func injectDecisionsPrompts(request *llm.Request, prompts []*ent.Prompt) error {
	if request.Decisions == nil || len(request.Decisions.Body) == 0 {
		return nil
	}
	body := append([]byte(nil), request.Decisions.Body...)
	input := gjson.GetBytes(body, "input")
	if input.Type == gjson.String {
		text := input.String()
		for _, prompt := range prompts {
			if prompt == nil {
				continue
			}
			if prompt.Settings.Action.Type == objects.PromptActionTypeAppend {
				text += "\n" + prompt.Content
			} else {
				text = prompt.Content + "\n" + text
			}
		}
		updated, err := sjson.SetBytes(body, "input", text)
		if err != nil {
			return err
		}
		request.Decisions.Body = updated
		return nil
	}

	for i := range jsonArrayLength(body, "input") {
		itemPath := fmt.Sprintf("input.%d", i)
		role := gjson.GetBytes(body, itemPath+".role").String()
		if role == "" {
			role = "user"
		}
		for j := range jsonArrayLength(body, itemPath+".content") {
			partPath := fmt.Sprintf("%s.content.%d", itemPath, j)
			if strings.EqualFold(gjson.GetBytes(body, partPath+".type").String(), "input_text") {
				text := gjson.GetBytes(body, partPath+".text").String()
				for _, prompt := range prompts {
					if prompt == nil || !promptScopeMatches(prompt.Role, role) {
						continue
					}
					if prompt.Settings.Action.Type == objects.PromptActionTypeAppend {
						text += "\n" + prompt.Content
					} else {
						text = prompt.Content + "\n" + text
					}
				}
				updated, err := sjson.SetBytes(body, partPath+".text", text)
				if err != nil {
					return err
				}
				request.Decisions.Body = updated
				return nil
			}
		}
	}

	return nil
}

func promptScopeMatches(promptRole, inputRole string) bool {
	return promptRole == "" || promptRole == inputRole || promptRole == "system"
}
