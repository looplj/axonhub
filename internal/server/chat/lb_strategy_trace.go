package chat

import (
	"context"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// ChannelTraceProvider provides trace-related channel information.
type ChannelTraceProvider interface {
	GetLastSuccessfulChannelID(ctx context.Context, traceID int) (int, error)
}

// TraceAwareStrategy prioritizes the last successful channel from the trace context.
// If a trace ID exists and has a last successful channel, that channel gets maximum score.
type TraceAwareStrategy struct {
	traceProvider ChannelTraceProvider
	// Score boost for the last successful channel (default: 1000)
	boostScore float64
}

// NewTraceAwareStrategy creates a new trace-aware strategy.
func NewTraceAwareStrategy(traceProvider ChannelTraceProvider) *TraceAwareStrategy {
	return &TraceAwareStrategy{
		traceProvider: traceProvider,
		boostScore:    1000.0,
	}
}

// Score returns maximum score if this channel was the last successful one in the trace.
// Production path without debug logging.
func (s *TraceAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	trace, hasTrace := contexts.GetTrace(ctx)
	if !hasTrace {
		return 0
	}

	lastChannelID, err := s.traceProvider.GetLastSuccessfulChannelID(ctx, trace.ID)
	if err != nil {
		return 0
	}

	if lastChannelID == 0 {
		return 0
	}

	if channel.ID == lastChannelID {
		return s.boostScore
	}

	return 0
}

// ScoreWithDebug returns maximum score with detailed debug information.
// Debug path with comprehensive logging.
func (s *TraceAwareStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	trace, hasTrace := contexts.GetTrace(ctx)
	if !hasTrace {
		log.Info(ctx, "TraceAwareStrategy: no trace in context, returning 0 score")

		return 0, StrategyScore{
			StrategyName: s.Name(),
			Score:        0,
			Details: map[string]interface{}{
				"reason": "no_trace_in_context",
			},
		}
	}

	lastChannelID, err := s.traceProvider.GetLastSuccessfulChannelID(ctx, trace.ID)
	if err != nil {
		log.Info(ctx, "TraceAwareStrategy: failed to get last successful channel ID",
			log.Int("trace_id", trace.ID),
			log.Cause(err),
		)

		return 0, StrategyScore{
			StrategyName: s.Name(),
			Score:        0,
			Details: map[string]interface{}{
				"reason":   "error_getting_last_channel",
				"trace_id": trace.ID,
				"error":    err.Error(),
			},
		}
	}

	if lastChannelID == 0 {
		log.Info(ctx, "TraceAwareStrategy: no last successful channel for trace",
			log.Int("trace_id", trace.ID),
		)

		return 0, StrategyScore{
			StrategyName: s.Name(),
			Score:        0,
			Details: map[string]interface{}{
				"reason":   "no_last_successful_channel",
				"trace_id": trace.ID,
			},
		}
	}

	isLastChannel := channel.ID == lastChannelID
	score := 0.0
	details := map[string]interface{}{
		"trace_id":        trace.ID,
		"last_channel_id": lastChannelID,
		"is_last_channel": isLastChannel,
	}

	if isLastChannel {
		score = s.boostScore
		details["reason"] = "last_successful_channel_in_trace"

		log.Info(ctx, "TraceAwareStrategy: boosting channel",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int("trace_id", trace.ID),
			log.Float64("score", score),
			log.String("reason", "last_successful_channel_in_trace"),
		)
	} else {
		details["reason"] = "not_last_successful_channel"

		log.Info(ctx, "TraceAwareStrategy: channel not in trace",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int("trace_id", trace.ID),
		)
	}

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
	}
}

// Name returns the strategy name.
func (s *TraceAwareStrategy) Name() string {
	return "TraceAware"
}
