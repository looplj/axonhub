package orchestrator

import (
	"errors"
	"slices"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	return httpclient.IsHTTPStatusCodeRetryable(ExtractStatusCodeFromError(err))
}

func isRetryableErrorForChannel(err error, ch *biz.Channel) bool {
	if err == nil {
		return false
	}

	statusCode := ExtractStatusCodeFromError(err)
	if httpclient.IsHTTPStatusCodeRetryable(statusCode) {
		return true
	}

	if ch == nil || ch.Channel == nil || ch.Settings == nil {
		return false
	}

	return slices.Contains(ch.Settings.RetryableStatusCodes, statusCode)
}

// ExtractStatusCodeFromError attempts to extract HTTP status code from various error types.
func ExtractStatusCodeFromError(err error) int {
	if err == nil {
		return 0
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}

	var llmErr *llm.ResponseError
	if errors.As(err, &llmErr) {
		return llmErr.StatusCode
	}

	return 0
}

func deriveLoadBalancerStrategy(retryPolicy *biz.RetryPolicy, apiKey *ent.APIKey) string {
	strategy := retryPolicy.LoadBalancerStrategy
	if apiKey == nil {
		return strategy
	}

	activeProfile := apiKey.GetActiveProfile()
	if activeProfile == nil {
		return strategy
	}

	if activeProfile.LoadBalanceStrategy == nil ||
		*activeProfile.LoadBalanceStrategy == "" ||
		*activeProfile.LoadBalanceStrategy == "system_default" {
		return strategy
	}

	return *activeProfile.LoadBalanceStrategy
}
