package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"

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

	// Chat metrics
	ChatRequestCount    metric.Int64Counter
	ChatRequestDuration metric.Float64Histogram
	ChatTokenCount      metric.Int64Counter
	ChatSuccessCount    metric.Int64Counter
	ChatFailureCount    metric.Int64Counter
}

var Metrics *_Metrics

func init() {
	// Default to a no-op implementation so Metrics is always safe to use even
	// when SetupMetrics has not run, for example in tests or when metrics are
	// disabled. No-op meters never fail instrument creation, so the error
	// below is unreachable in practice.
	m, err := newMetrics(noop.NewMeterProvider().Meter("axonhub"))
	if err != nil {
		panic(err)
	}

	Metrics = m
}

// SetupMetrics creates a new ServerMetrics instance.
func SetupMetrics(provider *sdk.MeterProvider, name string) error {
	m, err := newMetrics(provider.Meter(name))
	if err != nil {
		return err
	}

	Metrics = m

	return nil
}

func newMetrics(meter metric.Meter) (*_Metrics, error) {
	m := &_Metrics{}

	// HTTP metrics
	httpRequestCount, err := meter.Int64Counter(
		"http_request_count",
		metric.WithDescription("Number of HTTP requests"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_request_count counter: %w", err)
	}

	m.HTTPRequestCount = httpRequestCount

	httpRequestDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_request_duration_seconds histogram: %w", err)
	}

	m.HTTPRequestDuration = httpRequestDuration

	// GraphQL metrics
	graphQLRequestCount, err := meter.Int64Counter(
		"graphql_request_count",
		metric.WithDescription("Number of GraphQL requests"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create graphql_request_count counter: %w", err)
	}

	m.GraphQLRequestCount = graphQLRequestCount

	graphQLRequestDuration, err := meter.Float64Histogram(
		"graphql_request_duration_seconds",
		metric.WithDescription("GraphQL request duration in seconds"),
		metric.WithUnit("seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create graphql_request_duration_seconds histogram: %w", err)
	}

	m.GraphQLRequestDuration = graphQLRequestDuration

	// Chat metrics
	chatRequestCount, err := meter.Int64Counter(
		"chat_request_count",
		metric.WithDescription("Number of chat requests"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat_request_count counter: %w", err)
	}

	m.ChatRequestCount = chatRequestCount

	chatRequestDuration, err := meter.Float64Histogram(
		"chat_request_duration_seconds",
		metric.WithDescription("Chat request duration in seconds"),
		metric.WithUnit("seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat_request_duration_seconds histogram: %w", err)
	}

	m.ChatRequestDuration = chatRequestDuration

	chatTokenCount, err := meter.Int64Counter(
		"chat_token_count",
		metric.WithDescription("Number of tokens in chat requests"),
		metric.WithUnit("tokens"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat_token_count counter: %w", err)
	}

	m.ChatTokenCount = chatTokenCount

	chatSuccessCount, err := meter.Int64Counter(
		"chat_success_count",
		metric.WithDescription("Number of successful chat requests"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat_success_count counter: %w", err)
	}

	m.ChatSuccessCount = chatSuccessCount

	chatFailureCount, err := meter.Int64Counter(
		"chat_failure_count",
		metric.WithDescription("Number of failed chat requests"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat_failure_count counter: %w", err)
	}

	m.ChatFailureCount = chatFailureCount

	return m, nil
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

// RecordChatRequest records the outcome of a chat request: request count,
// duration and success/failure, attributed to the channel that served it.
func (sm *_Metrics) RecordChatRequest(ctx context.Context, channelID, channelName string, duration float64, success bool) {
	labels := chatChannelLabels(channelID, channelName)

	sm.ChatRequestCount.Add(ctx, 1, metric.WithAttributes(labels...))
	sm.ChatRequestDuration.Record(ctx, duration, metric.WithAttributes(labels...))

	if success {
		sm.ChatSuccessCount.Add(ctx, 1, metric.WithAttributes(labels...))
	} else {
		sm.ChatFailureCount.Add(ctx, 1, metric.WithAttributes(labels...))
	}
}

// RecordChatTokens records the token usage of a chat request, attributed to
// the channel that served it.
func (sm *_Metrics) RecordChatTokens(ctx context.Context, channelID, channelName string, tokenCount int64) {
	if tokenCount <= 0 {
		return
	}

	sm.ChatTokenCount.Add(ctx, tokenCount, metric.WithAttributes(chatChannelLabels(channelID, channelName)...))
}

func chatChannelLabels(channelID, channelName string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("channel_id", channelID),
		attribute.String("channel_name", channelName),
	}
}
