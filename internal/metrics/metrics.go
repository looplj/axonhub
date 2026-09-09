package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"

	metric "go.opentelemetry.io/otel/metric"
	sdk "go.opentelemetry.io/otel/sdk/metric"
)

// _Metrics holds all the metrics for the server.
type _Metrics struct {
	// HTTP metrics
	HTTPRequestCount    metric.Int64Counter
	HTTPRequestDuration metric.Float64Histogram

	// GraphQL metrics
	GraphQLRequestCount    metric.Int64Counter
	GraphQLRequestDuration metric.Float64Histogram

	// LLM request lifecycle metrics. A downstream request is one logical
	// request received from a client; an upstream request is one provider
	// execution attempt (retries therefore create additional upstream events).
	DownstreamRequestsTotal  metric.Int64Counter
	DownstreamActiveRequests metric.Int64UpDownCounter
	UpstreamRequestsTotal    metric.Int64Counter
	UpstreamActiveRequests   metric.Int64UpDownCounter

	// LLM usage and performance metrics.
	LLMTokens          metric.Int64Counter
	LLMCost            metric.Float64Counter
	UpstreamDuration   metric.Float64Histogram
	UpstreamTTFT       metric.Float64Histogram
	DownstreamDuration metric.Float64Histogram
	DownstreamTTFT     metric.Float64Histogram
}

// RequestAttributes contains the dimensions shared by request, usage, and
// performance metrics. Empty IDs are used for dimensions that are not known at
// a lifecycle point (for example, the selected channel is not known when a
// downstream request is first created).
type RequestAttributes struct {
	ProjectID      int
	ChannelID      int
	RequestModelID string
	ModelID        string
	APIKeyID       int
	UserID         int
	Source         string
	Format         string
	Stream         bool
	Status         string
}

var Metrics *_Metrics

// SetupMetrics creates a new ServerMetrics instance.
func SetupMetrics(provider *sdk.MeterProvider, name string) error {
	meter := provider.Meter(name)
	Metrics = &_Metrics{}

	// HTTP metrics
	httpRequestCount, err := meter.Int64Counter(
		"http_request_count",
		metric.WithDescription("Number of HTTP requests"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_request_count counter: %w", err)
	}

	Metrics.HTTPRequestCount = httpRequestCount

	httpRequestDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("seconds"),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_request_duration_seconds histogram: %w", err)
	}

	Metrics.HTTPRequestDuration = httpRequestDuration

	// GraphQL metrics
	graphQLRequestCount, err := meter.Int64Counter(
		"graphql_request_count",
		metric.WithDescription("Number of GraphQL requests"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create graphql_request_count counter: %w", err)
	}

	Metrics.GraphQLRequestCount = graphQLRequestCount

	graphQLRequestDuration, err := meter.Float64Histogram(
		"graphql_request_duration_seconds",
		metric.WithDescription("GraphQL request duration in seconds"),
		metric.WithUnit("seconds"),
	)
	if err != nil {
		return fmt.Errorf("failed to create graphql_request_duration_seconds histogram: %w", err)
	}

	Metrics.GraphQLRequestDuration = graphQLRequestDuration

	// LLM request lifecycle metrics
	downstreamRequestsTotal, err := meter.Int64Counter(
		"axonhub_downstream_requests_total",
		metric.WithDescription("Logical downstream requests by terminal status"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create axonhub_downstream_requests_total counter: %w", err)
	}
	Metrics.DownstreamRequestsTotal = downstreamRequestsTotal

	downstreamActiveRequests, err := meter.Int64UpDownCounter(
		"axonhub_downstream_active_requests",
		metric.WithDescription("Logical downstream requests currently in progress"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create axonhub_downstream_active_requests gauge: %w", err)
	}
	Metrics.DownstreamActiveRequests = downstreamActiveRequests

	upstreamRequestsTotal, err := meter.Int64Counter(
		"axonhub_upstream_requests_total",
		metric.WithDescription("Upstream provider execution attempts by terminal status"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create axonhub_upstream_requests_total counter: %w", err)
	}
	Metrics.UpstreamRequestsTotal = upstreamRequestsTotal

	upstreamActiveRequests, err := meter.Int64UpDownCounter(
		"axonhub_upstream_active_requests",
		metric.WithDescription("Upstream provider execution attempts currently in progress"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create axonhub_upstream_requests gauge: %w", err)
	}
	Metrics.UpstreamActiveRequests = upstreamActiveRequests

	llmTokens, err := meter.Int64Counter(
		"axonhub_llm_tokens_total",
		metric.WithDescription("LLM tokens recorded from completed usage data"),
		metric.WithUnit("tokens"),
	)
	if err != nil {
		return fmt.Errorf("failed to create axonhub_llm_tokens_total counter: %w", err)
	}
	Metrics.LLMTokens = llmTokens

	llmCost, err := meter.Float64Counter(
		"axonhub_llm_cost_total",
		metric.WithDescription("AxonHub-calculated LLM cost recorded from usage data"),
		metric.WithUnit("currency"),
	)
	if err != nil {
		return fmt.Errorf("failed to create axonhub_llm_cost_total counter: %w", err)
	}
	Metrics.LLMCost = llmCost

	upstreamDuration, err := meter.Float64Histogram(
		"axonhub_upstream_request_duration_seconds",
		metric.WithDescription("Duration of an upstream provider execution attempt"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create axonhub_upstream_request_duration_seconds histogram: %w", err)
	}
	Metrics.UpstreamDuration = upstreamDuration

	upstreamTTFT, err := meter.Float64Histogram(
		"axonhub_upstream_ttft_seconds",
		metric.WithDescription("Time from upstream execution start to the first generated token"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create axonhub_upstream_ttft_seconds histogram: %w", err)
	}
	Metrics.UpstreamTTFT = upstreamTTFT

	downstreamDuration, err := meter.Float64Histogram("axonhub_downstream_request_duration_seconds", metric.WithDescription("End-to-end duration of a logical downstream request, including retries"), metric.WithUnit("s"))
	if err != nil {
		return fmt.Errorf("failed to create axonhub_downstream_request_duration_seconds histogram: %w", err)
	}
	Metrics.DownstreamDuration = downstreamDuration

	downstreamTTFT, err := meter.Float64Histogram("axonhub_downstream_ttft_seconds", metric.WithDescription("Time from logical downstream request creation to the first client-visible output"), metric.WithUnit("s"))
	if err != nil {
		return fmt.Errorf("failed to create axonhub_downstream_ttft_seconds histogram: %w", err)
	}
	Metrics.DownstreamTTFT = downstreamTTFT

	return nil
}

// RecordHTTPRequest records HTTP request metrics.
func (sm *_Metrics) RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration float64) {
	labels := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("path", path),
		attribute.Int("status_code", statusCode),
	}

	sm.HTTPRequestCount.Add(ctx, 1, metric.WithAttributes(labels...))
	sm.HTTPRequestDuration.Record(ctx, duration, metric.WithAttributes(labels...))
}

// RecordGraphQLRequest records GraphQL request metrics.
func (sm *_Metrics) RecordGraphQLRequest(ctx context.Context, operation string, duration float64) {
	labels := []attribute.KeyValue{
		attribute.String("operation", operation),
	}

	sm.GraphQLRequestCount.Add(ctx, 1, metric.WithAttributes(labels...))
	sm.GraphQLRequestDuration.Record(ctx, duration, metric.WithAttributes(labels...))
}

// RecordDownstreamRequestCreated records creation of one logical client request.
func (sm *_Metrics) RecordDownstreamRequestCreated(ctx context.Context, attrs RequestAttributes) {
	if sm == nil || sm.DownstreamActiveRequests == nil {
		return
	}

	sm.DownstreamActiveRequests.Add(ctx, 1, metric.WithAttributes(requestAttrs(attrs)...))
}

// RecordDownstreamRequestCompleted records one logical client request entering
// a terminal status. Callers must only invoke this on a real state transition.
func (sm *_Metrics) RecordDownstreamRequestCompleted(ctx context.Context, attrs RequestAttributes, status string) {
	if sm == nil || sm.DownstreamRequestsTotal == nil {
		return
	}

	attrs.Status = status
	attrsWithoutStatus := attrs
	attrsWithoutStatus.Status = ""
	sm.DownstreamRequestsTotal.Add(ctx, 1, metric.WithAttributes(requestAttrs(attrs)...))
	if sm.DownstreamActiveRequests != nil {
		sm.DownstreamActiveRequests.Add(ctx, -1, metric.WithAttributes(requestAttrs(attrsWithoutStatus)...))
	}
}

// RecordUpstreamRequestCreated records one provider execution attempt.
func (sm *_Metrics) RecordUpstreamRequestCreated(ctx context.Context, attrs RequestAttributes) {
	if sm == nil || sm.UpstreamActiveRequests == nil {
		return
	}

	sm.UpstreamActiveRequests.Add(ctx, 1, metric.WithAttributes(requestAttrs(attrs)...))
}

// RecordUpstreamRequestCompleted records one provider execution attempt entering
// a terminal status. Callers must only invoke this on a real state transition.
func (sm *_Metrics) RecordUpstreamRequestCompleted(ctx context.Context, attrs RequestAttributes, status string) {
	if sm == nil || sm.UpstreamRequestsTotal == nil {
		return
	}

	attrs.Status = status
	sm.UpstreamRequestsTotal.Add(ctx, 1, metric.WithAttributes(requestAttrs(attrs)...))
	attrsWithoutStatus := attrs
	attrsWithoutStatus.Status = ""
	if sm.UpstreamActiveRequests != nil {
		sm.UpstreamActiveRequests.Add(ctx, -1, metric.WithAttributes(requestAttrs(attrsWithoutStatus)...))
	}
}

// RecordLLMUsage records token usage and optional cost after the usage record
// has been persisted successfully. Cached and reasoning tokens are breakdowns
// of input and output respectively; consumers should not sum all token_type
// values together.
func (sm *_Metrics) RecordLLMUsage(ctx context.Context, attrs RequestAttributes, input, output, cached, reasoning int64, cost *float64) {
	if sm == nil {
		return
	}

	baseAttrs := requestAttrs(attrs)
	tokenValues := []struct {
		tokenType string
		value     int64
	}{
		{tokenType: "input", value: input},
		{tokenType: "output", value: output},
		{tokenType: "cached", value: cached},
		{tokenType: "reasoning", value: reasoning},
	}
	for _, token := range tokenValues {
		tokenType, value := token.tokenType, token.value
		if value > 0 && sm.LLMTokens != nil {
			attrsWithType := append(append([]attribute.KeyValue{}, baseAttrs...), attribute.String("token_type", tokenType))
			sm.LLMTokens.Add(ctx, value, metric.WithAttributes(attrsWithType...))
		}
	}

	if cost != nil && *cost > 0 && sm.LLMCost != nil {
		sm.LLMCost.Add(ctx, *cost, metric.WithAttributes(baseAttrs...))
	}
}

// RecordUpstreamPerformance records duration and, when available, TTFT for an
// upstream execution. The status is attached to both histograms so dashboards
// can select completed requests while failure latency remains observable.
func (sm *_Metrics) RecordUpstreamPerformance(ctx context.Context, attrs RequestAttributes, status string, durationSeconds float64, ttftSeconds *float64) {
	if sm == nil {
		return
	}

	attrs.Status = status
	metricAttrs := metric.WithAttributes(requestAttrs(attrs)...)
	if durationSeconds >= 0 && sm.UpstreamDuration != nil {
		sm.UpstreamDuration.Record(ctx, durationSeconds, metricAttrs)
	}
	if ttftSeconds != nil && *ttftSeconds >= 0 && sm.UpstreamTTFT != nil {
		sm.UpstreamTTFT.Record(ctx, *ttftSeconds, metricAttrs)
	}
}

// RecordDownstreamPerformance records end-to-end logical-request performance.
func (sm *_Metrics) RecordDownstreamPerformance(ctx context.Context, attrs RequestAttributes, status string, durationSeconds float64, ttftSeconds *float64) {
	if sm == nil {
		return
	}

	attrs.Status = status
	metricAttrs := metric.WithAttributes(requestAttrs(attrs)...)
	if durationSeconds >= 0 && sm.DownstreamDuration != nil {
		sm.DownstreamDuration.Record(ctx, durationSeconds, metricAttrs)
	}
	if ttftSeconds != nil && *ttftSeconds >= 0 && sm.DownstreamTTFT != nil {
		sm.DownstreamTTFT.Record(ctx, *ttftSeconds, metricAttrs)
	}
}

func requestAttrs(attrs RequestAttributes) []attribute.KeyValue {
	result := []attribute.KeyValue{
		attribute.Int("project_id", attrs.ProjectID),
		attribute.Int("channel_id", attrs.ChannelID),
		attribute.String("request_model_id", attrs.RequestModelID),
		attribute.String("model_id", attrs.ModelID),
		attribute.Int("api_key_id", attrs.APIKeyID),
		attribute.Int("user_id", attrs.UserID),
		attribute.String("source", attrs.Source),
		attribute.String("format", attrs.Format),
		attribute.Bool("stream", attrs.Stream),
	}
	if attrs.Status != "" {
		result = append(result, attribute.String("status", attrs.Status))
	}

	return result
}
