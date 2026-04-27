package orchestrator

import (
	"context"
	"strings"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

var supportedAutoReasoningEfforts = map[string]struct{}{
	"max":    {},
	"xhigh":  {},
	"high":   {},
	"medium": {},
	"low":    {},
}

func applyAutoReasoningEffort(systemService *biz.SystemService) pipeline.Middleware {
	return &autoReasoningEffortMiddleware{
		systemService: systemService,
	}
}

type autoReasoningEffortMiddleware struct {
	pipeline.DummyMiddleware

	systemService *biz.SystemService
}

func (m *autoReasoningEffortMiddleware) Name() string {
	return "auto-reasoning-effort"
}

func (m *autoReasoningEffortMiddleware) OnInboundLlmRequest(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
	if llmRequest == nil || llmRequest.Model == "" {
		return llmRequest, nil
	}

	settings := m.systemService.ModelSettingsOrDefault(ctx)
	if !settings.AutoReasoningEffort {
		return llmRequest, nil
	}

	baseModel, reasoningEffort, ok := splitAutoReasoningEffortModel(llmRequest.Model)
	if !ok {
		return llmRequest, nil
	}

	originalModel := llmRequest.Model
	llmRequest.Model = baseModel
	llmRequest.ReasoningEffort = reasoningEffort

	log.Debug(ctx, "applied auto reasoning effort",
		log.String("original_model", originalModel),
		log.String("normalized_model", baseModel),
		log.String("reasoning_effort", reasoningEffort),
	)

	return llmRequest, nil
}

func splitAutoReasoningEffortModel(model string) (baseModel string, reasoningEffort string, ok bool) {
	lastDash := strings.LastIndex(model, "-")
	if lastDash <= 0 || lastDash == len(model)-1 {
		return "", "", false
	}

	effort := strings.ToLower(model[lastDash+1:])
	if _, supported := supportedAutoReasoningEfforts[effort]; !supported {
		return "", "", false
	}

	base := model[:lastDash]
	if base == "" {
		return "", "", false
	}

	return base, effort, true
}
