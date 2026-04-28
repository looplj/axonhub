package orchestrator

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/looplj/axonhub/internal/server/biz"
)

// ChannelLimiterMetrics is the per-channel observability surface for the
// admission control system.
//
// Five instruments:
//   - axonhub_channel_inflight             (observable gauge) — current in-flight count
//   - axonhub_channel_queue_waiting        (observable gauge) — current queue depth
//   - axonhub_channel_queue_full_total     (counter)          — cumulative queue-full rejections
//   - axonhub_channel_queue_timeout_total  (counter)          — cumulative wait-timeout exits
//   - axonhub_channel_queue_wait_seconds   (histogram)        — wait time on successful acquires
//
// The gauges read live state from ChannelLimiterManager.Snapshot() at scrape
// time; counters/histogram are pushed by middleware on each event.
type ChannelLimiterMetrics struct {
	queueFull    metric.Int64Counter
	queueTimeout metric.Int64Counter
	queueWait    metric.Float64Histogram
}

// NewChannelLimiterMetrics registers the channel-limiter metric instruments and
// wires the gauges' callback against manager. Pass a nil meter to obtain a no-op
// metrics struct (useful for tests that do not initialize OTel).
func NewChannelLimiterMetrics(meter metric.Meter, manager *ChannelLimiterManager) (*ChannelLimiterMetrics, error) {
	if meter == nil {
		return &ChannelLimiterMetrics{}, nil
	}

	inFlight, err := meter.Int64ObservableGauge(
		"axonhub_channel_inflight",
		metric.WithDescription("In-flight requests currently holding a channel-limiter slot"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create axonhub_channel_inflight gauge: %w", err)
	}

	waiting, err := meter.Int64ObservableGauge(
		"axonhub_channel_queue_waiting",
		metric.WithDescription("Requests currently waiting in the channel-limiter FIFO queue"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create axonhub_channel_queue_waiting gauge: %w", err)
	}

	queueFull, err := meter.Int64Counter(
		"axonhub_channel_queue_full_total",
		metric.WithDescription("Total requests rejected because the channel queue was full"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create axonhub_channel_queue_full_total counter: %w", err)
	}

	queueTimeout, err := meter.Int64Counter(
		"axonhub_channel_queue_timeout_total",
		metric.WithDescription("Total requests that exited the channel queue via per-channel wait timeout"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create axonhub_channel_queue_timeout_total counter: %w", err)
	}

	queueWait, err := meter.Float64Histogram(
		"axonhub_channel_queue_wait_seconds",
		metric.WithDescription("Wait time from channel-limiter Acquire entry to slot grant, on successful acquisitions"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create axonhub_channel_queue_wait_seconds histogram: %w", err)
	}

	if manager != nil {
		_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
			for _, snap := range manager.Snapshot() {
				attrs := metric.WithAttributes(channelAttrs(snap.ChannelID, snap.ChannelName)...)
				observer.ObserveInt64(inFlight, int64(snap.InFlight), attrs)
				observer.ObserveInt64(waiting, int64(snap.Waiting), attrs)
			}

			return nil
		}, inFlight, waiting)
		if err != nil {
			return nil, fmt.Errorf("register channel-limiter gauge callback: %w", err)
		}
	}

	return &ChannelLimiterMetrics{
		queueFull:    queueFull,
		queueTimeout: queueTimeout,
		queueWait:    queueWait,
	}, nil
}

// IncQueueFull increments the queue-full rejection counter.
func (m *ChannelLimiterMetrics) IncQueueFull(ctx context.Context, ch *biz.Channel) {
	if m == nil || m.queueFull == nil || ch == nil {
		return
	}

	m.queueFull.Add(ctx, 1, metric.WithAttributes(channelAttrs(ch.ID, ch.Name)...))
}

// IncQueueTimeout increments the queue-timeout exit counter.
func (m *ChannelLimiterMetrics) IncQueueTimeout(ctx context.Context, ch *biz.Channel) {
	if m == nil || m.queueTimeout == nil || ch == nil {
		return
	}

	m.queueTimeout.Add(ctx, 1, metric.WithAttributes(channelAttrs(ch.ID, ch.Name)...))
}

// ObserveQueueWait records the wait duration for a successful Acquire.
func (m *ChannelLimiterMetrics) ObserveQueueWait(ctx context.Context, ch *biz.Channel, dur time.Duration) {
	if m == nil || m.queueWait == nil || ch == nil {
		return
	}

	m.queueWait.Record(ctx, dur.Seconds(), metric.WithAttributes(channelAttrs(ch.ID, ch.Name)...))
}

func channelAttrs(channelID int, channelName string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("channel_id", channelID),
		attribute.String("channel_name", channelName),
	}
}
