package anthropic

import (
	"context"
	"errors"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/looplj/axonhub/axon/agent"
	axoncontext "github.com/looplj/axonhub/axon/context"
)

func (p *Provider) Chat(ctx context.Context, model string, tools []agent.ToolDefinition, messages []agent.Message) (agent.Response, error) {
	system, msgParams := convertMessages(messages)
	toolParams := convertTools(tools)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: defaultMaxTokens,
		Messages:  msgParams,
	}
	if len(system) > 0 {
		params.System = system
	}
	if len(toolParams) > 0 {
		params.Tools = toolParams
	}

	if budget := reasoningEffortToBudget(p.reasoningEffort); budget > 0 {
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: budget,
			},
		}
	}

	var reqOpts []option.RequestOption
	if threadID := axoncontext.ThreadID(ctx); threadID != "" {
		reqOpts = append(reqOpts, option.WithHeader(p.threadHeader, threadID))
	}
	if traceID := axoncontext.TraceID(ctx); traceID != "" {
		reqOpts = append(reqOpts, option.WithHeader(p.traceHeader, traceID))
	}

	var lastErr error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return agent.Response{}, ctx.Err()
			case <-time.After(time.Second * time.Duration(attempt)):
			}
		}

		resp, err := p.client.Messages.New(ctx, params, reqOpts...)
		if err == nil {
			return convertResponse(resp), nil
		}

		lastErr = wrapAPIError(err)

		providerErr := &agent.ProviderError{}
		if errors.As(lastErr, &providerErr) {
			if providerErr.IsClientError() || providerErr.StatusCode < 500 {
				break
			}
		}
	}

	return agent.Response{}, lastErr
}
