package orchestrator

import (
	"net/url"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
)

// inboundRequestDebugFields returns debug log fields for an inbound request.
// The raw request body and headers are intentionally omitted: the body can carry
// user content and extra_body provider hints, and the headers include the
// Authorization credential. Only routing diagnostics that cannot leak secrets
// are kept: method, url, body size, request type, and the retry policy.
func inboundRequestDebugFields(request *httpclient.Request, retryPolicy *biz.RetryPolicy) []log.Field {
	fields := []log.Field{
		log.String("method", request.Method),
		log.String("url", urlForInboundLog(request.URL)),
		log.Int("body_size", len(request.Body)),
		log.String("request_type", request.RequestType),
		log.String("api_format", request.APIFormat),
	}

	if retryPolicy != nil {
		fields = append(fields,
			log.Any("retry_policy", retryPolicy),
			log.String("system_load_balance_strategy", retryPolicy.LoadBalancerStrategy),
			log.String("system_trace_sticky_mode", string(retryPolicy.TraceStickyMode)),
		)
	}

	return fields
}

// urlForInboundLog strips user info, query parameters and fragments so sensitive
// values (e.g. API keys in query strings) never reach the log.
func urlForInboundLog(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""

	return parsed.String()
}
