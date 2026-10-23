package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/internal/server/safeguards"
	"github.com/looplj/axonhub/internal/tracing"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// newSafeguardsClassifier uses the ordinary routing, model access, quota and
// persistence pipeline. It does not reenter the Anthropic HTTP handler, so a
// classifier cannot recursively trigger review. The caller supplies its deadline.
func newSafeguardsClassifier(parent *orchestrator.ChatCompletionOrchestrator) safeguards.ClassifierFunc {
	classifier := *parent
	classifier.Inbound = openai.NewInboundTransformer()
	return safeguardsClassifierWithProcess(classifier.Process)
}

// safeguardsClassifierWithProcess adapts the pipeline to a tool-free, non-streaming
// reviewer call with isolated request bookkeeping. Only one nonempty, unrefused
// choice ending with stop is accepted; verdict JSON validation happens later.
func safeguardsClassifierWithProcess(process func(context.Context, *httpclient.Request) (orchestrator.ChatCompletionResult, error)) safeguards.ClassifierFunc {
	return func(ctx context.Context, req safeguards.ClassifierRequest) (string, error) {
		ctx = contexts.ForkRequest(ctx)
		id := tracing.GenerateRequestID()
		ctx = contexts.WithRequestID(ctx, id)
		ctx = contexts.WithOperationName(ctx, "safeguards-classifier")
		body, err := json.Marshal(map[string]any{
			"model":      req.Model,
			"messages":   []map[string]string{{"role": "system", "content": req.System}, {"role": "user", "content": req.User}},
			"max_tokens": req.MaxTokens,
			"stream":     false,
		})
		if err != nil {
			return "", fmt.Errorf("encode safeguards classifier request: %w", err)
		}
		result, err := process(ctx, &httpclient.Request{
			Method: http.MethodPost, Path: "/v1/chat/completions",
			Headers:     http.Header{"Content-Type": {"application/json"}},
			ContentType: "application/json", Body: body, RequestID: id,
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		})
		if err != nil {
			return "", fmt.Errorf("safeguards classifier request: %w", err)
		}
		if result.ChatCompletionStream != nil {
			_ = result.ChatCompletionStream.Close()
			return "", fmt.Errorf("safeguards classifier returned an unexpected stream")
		}
		if result.ChatCompletion == nil || result.ChatCompletion.StatusCode < 200 || result.ChatCompletion.StatusCode >= 300 {
			return "", fmt.Errorf("safeguards classifier did not return a successful response")
		}
		var response struct {
			Choices []struct {
				FinishReason string `json:"finish_reason"`
				Message      struct {
					Content string `json:"content"`
					Refusal string `json:"refusal"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(result.ChatCompletion.Body, &response); err != nil {
			return "", fmt.Errorf("decode safeguards classifier response: %w", err)
		}
		if len(response.Choices) != 1 || response.Choices[0].FinishReason != "stop" || response.Choices[0].Message.Refusal != "" || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
			return "", fmt.Errorf("safeguards classifier returned incomplete or refused output")
		}
		return response.Choices[0].Message.Content, nil
	}
}
