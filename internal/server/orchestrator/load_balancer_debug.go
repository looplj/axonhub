package orchestrator

import (
	"context"
	"time"
)

// Context key for storing debug information in context.
type debugContextKey struct{}

// DebugOptions holds the configuration for debug mode.
type DebugOptions struct {
	// Enabled indicates whether debug mode is active
	Enabled bool

	// RecordDecisionDetails records detailed decision information
	RecordDecisionDetails bool

	// RecordStrategyDetails records detailed strategy calculation info
	RecordStrategyDetails bool

	// MaxRecordsPerMinute limits debug records to prevent log flooding
	MaxRecordsPerMinute int
}

// DefaultDebugOptions returns default debug options.
func DefaultDebugOptions() *DebugOptions {
	return &DebugOptions{
		Enabled:               false,
		RecordDecisionDetails: true,
		RecordStrategyDetails: true,
		MaxRecordsPerMinute:   100,
	}
}

// EnableDebugMode enables debug mode for the load balancer in the given context.
func EnableDebugMode(ctx context.Context, opts *DebugOptions) context.Context {
	if opts == nil {
		opts = DefaultDebugOptions()
	}

	return context.WithValue(ctx, debugContextKey{}, opts)
}

// GetDebugOptions retrieves debug options from the context.
func GetDebugOptions(ctx context.Context) *DebugOptions {
	if opts, ok := ctx.Value(debugContextKey{}).(*DebugOptions); ok {
		return opts
	}
	// Return disabled options if not found
	return &DebugOptions{Enabled: false}
}

// IsDebugEnabled checks if debug mode is enabled in the context.
func IsDebugEnabled(ctx context.Context) bool {
	opts := GetDebugOptions(ctx)
	return opts.Enabled
}

// Debug holds detailed debug information about a load balancing decision.
type Debug struct {
	// RequestID is the unique identifier for the request
	RequestID string
	// Timestamp when the decision was made
	Timestamp time.Time
	// Model being requested
	Model string
	// InputChannels are channels before sorting
	InputChannels []ChannelDebug
	// OutputChannels are channels after sorting
	OutputChannels []ChannelDebug
	// TotalDuration is the time spent on load balancing
	TotalDuration time.Duration
}

// ChannelDebug holds debug information for a single channel.
type ChannelDebug struct {
	// ChannelID is the channel identifier
	ChannelID int
	// ChannelName is the channel name
	ChannelName string
	// TotalScore is the sum of all strategy scores
	TotalScore float64
	// StrategyScores contains detailed scores from each strategy
	StrategyScores []StrategyDebug
	// Rank is the final ranking (1 = highest priority)
	Rank int
}

// StrategyDebug holds debug information for a single strategy's score.
type StrategyDebug struct {
	// StrategyName is the name of the strategy
	StrategyName string
	// Score is the score calculated
	Score float64
	// Duration is the time spent scoring
	Duration time.Duration
	// Details contains strategy-specific information
	Details map[string]any
}

type debugInfoKey struct{}

// WithDebugInfo stores debug information in the context.
func WithDebugInfo(ctx context.Context, info *Debug) context.Context {
	return context.WithValue(ctx, debugInfoKey{}, info)
}

// GetDebugInfo retrieves debug information from the context if available.
func GetDebugInfo(ctx context.Context) *Debug {
	if info, ok := ctx.Value(debugInfoKey{}).(*Debug); ok {
		return info
	}

	return nil
}
