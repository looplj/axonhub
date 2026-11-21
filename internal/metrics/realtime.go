// 职责: 复用现有 OpenTelemetry Provider 记录实时会话与分模态用量。
// 关联文件:
//
//	internal/metrics/metrics.go
//	internal/server/api/realtime.go
//	internal/server/orchestrator/realtime.go
//
// 注意事项:
//
//	标签不包含会话标识、API Key、对话或音视频内容，避免泄露与高基数。

// Package metrics records application metrics.
package metrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/looplj/axonhub/llm"
)

type realtimeMetrics struct {
	active      metric.Int64UpDownCounter
	events      metric.Int64Counter
	duration    metric.Float64Histogram
	firstOutput metric.Float64Histogram
	tokens      metric.Int64Counter
}

func newRealtimeMetrics(meter metric.Meter) (*realtimeMetrics, error) {
	r := &realtimeMetrics{}
	var err error
	if r.active, err = meter.Int64UpDownCounter("axonhub_realtime_active_sessions"); err != nil {
		return nil, err
	}
	if r.events, err = meter.Int64Counter("axonhub_realtime_events_total"); err != nil {
		return nil, err
	}
	if r.duration, err = meter.Float64Histogram("axonhub_realtime_session_duration_seconds"); err != nil {
		return nil, err
	}
	if r.firstOutput, err = meter.Float64Histogram("axonhub_realtime_first_output_seconds"); err != nil {
		return nil, err
	}
	if r.tokens, err = meter.Int64Counter("axonhub_realtime_tokens_total"); err != nil {
		return nil, err
	}
	return r, nil
}

func (sm *_Metrics) RealtimeEvent(ctx context.Context, event string) {
	sm.realtime.events.Add(ctx, 1, metric.WithAttributes(attribute.String("event", event)))
}

func (sm *_Metrics) RealtimeActive(ctx context.Context, delta int64) {
	sm.realtime.active.Add(ctx, delta)
}

func (sm *_Metrics) RealtimeDuration(ctx context.Context, seconds float64) {
	sm.realtime.duration.Record(ctx, seconds)
}

func (sm *_Metrics) RealtimeUsage(ctx context.Context, usage *llm.Usage, firstOutput *int64) {
	if usage == nil {
		return
	}
	inputText, inputAudio, inputImage := int64(0), int64(0), int64(0)
	outputText, outputAudio := int64(0), int64(0)
	if details := usage.Realtime; details != nil {
		inputText = details.InputTextTokens
		inputAudio = details.InputAudioTokens
		inputImage = details.InputImageTokens
		outputText = details.OutputTextTokens
		outputAudio = details.OutputAudioTokens
	} else {
		if usage.PromptTokensDetails != nil {
			inputAudio = usage.PromptTokensDetails.AudioTokens
			inputImage = usage.PromptTokensDetails.ImageTokens
		}
		if usage.CompletionTokensDetails != nil {
			outputAudio = usage.CompletionTokensDetails.AudioTokens
		}
		inputText = max(usage.PromptTokens-inputAudio-inputImage, 0)
		outputText = max(usage.CompletionTokens-outputAudio, 0)
	}
	for kind, count := range map[string]int64{"input_text": inputText, "input_audio": inputAudio, "input_image": inputImage, "output_text": outputText, "output_audio": outputAudio} {
		if count > 0 {
			sm.realtime.tokens.Add(ctx, count, metric.WithAttributes(attribute.String("modality", kind)))
		}
	}
	if firstOutput != nil {
		sm.realtime.firstOutput.Record(ctx, float64(*firstOutput)/1000)
	}
}
