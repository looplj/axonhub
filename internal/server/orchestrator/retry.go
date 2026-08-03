package orchestrator

import (
	"errors"
	"io"
	"net"
	"regexp"
	"slices"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	return isRetryableTransportError(err) ||
		httpclient.IsHTTPStatusCodeRetryable(ExtractStatusCodeFromError(err))
}

func isRetryableErrorForChannel(err error, ch *biz.Channel) bool {
	if err == nil {
		return false
	}
	if isRetryableTransportError(err) {
		return true
	}

	statusCode := ExtractStatusCodeFromError(err)
	if httpclient.IsHTTPStatusCodeRetryable(statusCode) {
		return true
	}

	if ch == nil || ch.Channel == nil || ch.Settings == nil {
		return false
	}

	return slices.Contains(ch.Settings.RetryableStatusCodes, statusCode) ||
		matchesRetryableErrorPattern(err, ch.Settings.RetryableErrorPatterns)
}

// isRetryableTransportError identifies failures where the upstream connection
// ended before a usable response was received. The streaming pipeline only
// invokes retry selection before response content is committed, so retrying
// these transport failures cannot duplicate already-delivered output.
func isRetryableTransportError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var netErr net.Error

	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}

func matchesRetryableErrorPattern(err error, patterns []objects.RetryableErrorPattern) bool {
	if err == nil || len(patterns) == 0 {
		return false
	}

	message := err.Error()
	if message == "" {
		return false
	}

	for _, pattern := range patterns {
		if pattern.Pattern == "" {
			continue
		}

		if pattern.Regex {
			matched, regexErr := regexp.MatchString(pattern.Pattern, message)
			if regexErr == nil && matched {
				return true
			}

			continue
		}

		if strings.Contains(message, pattern.Pattern) {
			return true
		}
	}

	return false
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

type EffectiveRoutingPolicy struct {
	LoadBalancerStrategy string
	TraceStickyMode      biz.TraceStickyMode
}

func deriveRoutingPolicy(
	retryPolicy *biz.RetryPolicy,
	apiKey *ent.APIKey,
	modelPolicy *ModelRoutingPolicy,
) EffectiveRoutingPolicy {
	policy := EffectiveRoutingPolicy{
		LoadBalancerStrategy: retryPolicy.LoadBalancerStrategy,
		TraceStickyMode:      retryPolicy.TraceStickyMode,
	}

	if modelPolicy != nil {
		if strategy := objects.NormalizeRoutingPolicyValue(modelPolicy.LoadBalancerStrategy); strategy != objects.RoutingPolicyDefault && objects.IsValidLoadBalancerStrategy(strategy) {
			policy.LoadBalancerStrategy = strategy
		}
		if mode := objects.NormalizeRoutingPolicyValue(modelPolicy.TraceStickyMode); mode != objects.RoutingPolicyDefault && objects.IsValidTraceStickyMode(mode) {
			policy.TraceStickyMode = biz.TraceStickyMode(mode)
		}
	}

	if apiKey == nil {
		return policy
	}

	activeProfile := apiKey.GetActiveProfile()
	if activeProfile == nil {
		return policy
	}

	if activeProfile.LoadBalanceStrategy != nil {
		if strategy := objects.NormalizeRoutingPolicyValue(*activeProfile.LoadBalanceStrategy); strategy != objects.RoutingPolicyDefault && objects.IsValidLoadBalancerStrategy(strategy) {
			policy.LoadBalancerStrategy = strategy
		}
	}
	if activeProfile.TraceStickyMode != nil {
		if mode := objects.NormalizeRoutingPolicyValue(*activeProfile.TraceStickyMode); mode != objects.RoutingPolicyDefault && objects.IsValidTraceStickyMode(mode) {
			policy.TraceStickyMode = biz.TraceStickyMode(mode)
		}
	}

	return policy
}

func deriveLoadBalancerStrategy(retryPolicy *biz.RetryPolicy, apiKey *ent.APIKey) string {
	return deriveRoutingPolicy(retryPolicy, apiKey, nil).LoadBalancerStrategy
}
