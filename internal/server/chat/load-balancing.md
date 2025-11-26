# Channel Load Balancing

This document describes the load balancing system for channel selection in AxonHub.

## Overview

After channels are selected based on model compatibility, they are sorted using a load balancing system that considers multiple factors to determine the optimal order for attempting requests.

## Architecture

### Core Components

1. **LoadBalanceStrategy Interface** - Defines how strategies score channels
2. **LoadBalancer** - Orchestrates multiple strategies to sort channels
3. **Strategy Implementations** - Concrete implementations for different prioritization criteria

### Strategy Pattern

The load balancing system uses the Strategy pattern to make the prioritization logic extensible and composable. Each strategy independently scores channels, and the LoadBalancer combines these scores to produce a final ordering.

## Built-in Strategies

### 1. TraceAwareStrategy (Priority: 1000 points)

**Purpose**: Prioritizes the last successful channel from the current trace context.

**Behavior**:
- If a trace ID exists in the context, queries for the most recent successful request in that trace
- If found, gives that channel a maximum score boost (1000 points)
- Other channels receive 0 points from this strategy

**Use Case**: Ensures consistency within a conversation/trace by preferring the channel that successfully handled previous requests.

**Implementation**:
```go
NewTraceAwareStrategy(channelService)
```

### 2. ErrorAwareStrategy (Priority: 0-200 points)

**Purpose**: Deprioritizes channels with recent errors and health issues.

**Scoring Factors**:
- **Consecutive Failures**: -50 points per consecutive failure
- **Recent Failure (within 5 min)**: -100 points (decreases linearly over time)
- **Recent Success (within 1 min)**: +20 points
- **Low Success Rate (<50%)**: -50 points (requires 10+ requests)
- **High Success Rate (>90%)**: +30 points (requires 10+ requests)
- **Base Score**: 200 points (for healthy channels)

**Use Case**: Avoids channels experiencing issues and promotes reliable channels.

**Implementation**:
```go
NewErrorAwareStrategy(channelService)
```

### 3. WeightStrategy (Priority: 0-100 points)

**Purpose**: Respects admin-configured channel priorities.

**Behavior**:
- Uses the `OrderingWeight` field from channel configuration
- Normalizes weight (0-100 range) to score (0-100 points)
- Allows administrators to manually set channel preferences

**Use Case**: Enables cost optimization, geographic routing, or business logic preferences.

**Implementation**:
```go
NewWeightStrategy()
```

### 4. ConnectionAwareStrategy (Priority: 0-50 points)

**Purpose**: Load balances based on current connection utilization.

**Behavior**:
- Requires a `ConnectionTracker` implementation
- Scores channels inversely proportional to their utilization
- 0% utilization = 50 points, 100% utilization = 0 points

**Use Case**: Distributes load across channels to prevent overloading.

**Status**: Interface defined but requires connection tracking implementation.

**Implementation**:
```go
NewConnectionAwareStrategy(channelService, connectionTracker)
```

## Default Configuration

The `DefaultChannelSelector` uses these strategies in order:

```go
loadBalancer := NewLoadBalancer(
    NewTraceAwareStrategy(channelService),   // Priority 1: Trace consistency
    NewErrorAwareStrategy(channelService),   // Priority 2: Health
    NewWeightStrategy(),                     // Priority 3: Admin weight
)
```

**Total Score Range**: 0-1300 points per channel

## Scoring Example

Given 3 channels for a request in an existing trace:

| Channel | Last Success in Trace | Consecutive Failures | Weight | Total Score | Rank |
|---------|----------------------|---------------------|--------|-------------|------|
| A       | Yes                  | 0                   | 80     | 1280        | 1    |
| C       | No                   | 0                   | 50     | 250         | 2    |
| B       | No                   | 1                   | 100    | 250         | 3    |

**Calculation**:
- Channel A: 1000 (trace) + 200 (health) + 80 (weight) = 1280
- Channel C: 0 (trace) + 200 (health) + 50 (weight) = 250
- Channel B: 0 (trace) + 150 (health, -50 for failure) + 100 (weight) = 250

## Extension Points

### Provider Interfaces

Strategies depend on provider interfaces for better testability:

**ChannelMetricsProvider** - Provides performance metrics:
```go
type ChannelMetricsProvider interface {
    GetChannelMetrics(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error)
}
```

**ChannelTraceProvider** - Provides trace information:
```go
type ChannelTraceProvider interface {
    GetLastSuccessfulChannelID(ctx context.Context, traceID int) (int, error)
}
```

**ConnectionTracker** - Provides connection tracking:
```go
type ConnectionTracker interface {
    GetActiveConnections(channelID int) int
    GetMaxConnections(channelID int) int
}
```

### Creating Custom Strategies

Implement the `LoadBalanceStrategy` interface:

```go
type CustomStrategy struct {
    provider SomeProvider // Use interface dependencies
}

func (s *CustomStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
    // Your scoring logic using provider
    return score
}

func (s *CustomStrategy) Name() string {
    return "Custom"
}
```

### Testing with Mocks

Mock provider implementations for testing:

```go
type mockMetricsProvider struct {
    metrics map[int]*biz.AggregatedMetrics
}

func (m *mockMetricsProvider) GetChannelMetrics(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error) {
    return m.metrics[channelID], nil
}

// Use in tests
mockProvider := &mockMetricsProvider{
    metrics: map[int]*biz.AggregatedMetrics{
        1: {ConsecutiveFailures: 3},
    },
}
strategy := NewErrorAwareStrategy(mockProvider)
```

### Composing Strategies

Use `CompositeStrategy` to combine strategies with custom weights:

```go
composite := NewCompositeStrategy(
    strategy1,
    strategy2,
    strategy3,
).WithWeights(2.0, 1.5, 1.0)
```

### Custom Load Balancer

Create a custom load balancer with your strategy combination:

```go
customLoadBalancer := NewLoadBalancer(
    NewTraceAwareStrategy(channelService),
    myCustomStrategy,
    NewWeightStrategy(),
)

selector := &DefaultChannelSelector{
    ChannelService: channelService,
    LoadBalancer:   customLoadBalancer,
}
```

## Observability

### Structured Decision Logging

The load balancer provides comprehensive structured logging for debugging and monitoring:

**Decision Summary Log**:
```json
{
  "level": "debug",
  "timestamp": "2025-11-22T10:30:15Z",
  "msg": "Load balancing decision completed",
  "channel_count": 3,
  "duration_ms": 12.5,
  "top_channel_id": 1,
  "top_channel_name": "openai-us",
  "top_channel_score": 1280.0,
  "model": "gpt-4"
}
```

**Channel Details Log** (one per channel):
```json
{
  "level": "debug",
  "timestamp": "2025-11-22T10:30:15Z",
  "msg": "Channel load balancing details",
  "channel_id": 1,
  "channel_name": "openai-us",
  "total_score": 1280.0,
  "final_rank": 1,
  "strategy_breakdown": {
    "TraceAware": {
      "score": 1000.0,
      "duration_ms": 2.1
    },
    "ErrorAware": {
      "score": 200.0,
      "duration_ms": 5.3
    },
    "Weight": {
      "score": 80.0,
      "duration_ms": 0.1
    }
  },
  "model": "gpt-4"
}
```

**Strategy-Level Logging**:
- **TraceAwareStrategy**: Logs when boosting channels based on trace history
- **ErrorAwareStrategy**: Logs all penalties (consecutive failures, recent failures, low success rate) and boosts (recent success, high success rate)
- **WeightStrategy**: Logs weight calculation with clamping warnings

### Debug Mode

Debug mode provides enhanced observability for troubleshooting:

**Enable via Context**:
```go
opts := &chat.DebugOptions{
    Enabled:               true,
    RecordDecisionDetails: true,
    RecordStrategyDetails: true,
}
ctx = chat.EnableDebugMode(ctx, opts)
```

**DebugInfo Structure**:
```go
type DebugInfo struct {
    RequestID      string
    Timestamp      time.Time
    Model          string
    InputChannels  []ChannelDebugInfo   // Before sorting
    OutputChannels []ChannelDebugInfo   // After sorting
    TotalDuration  time.Duration
}
```

Each `ChannelDebugInfo` includes:
- Channel ID and name
- Total score
- Detailed scores from each strategy
- Strategy execution duration
- Final rank

**Retrieve Debug Info**:
```go
if info := chat.GetDebugInfo(ctx); info != nil {
    // Access detailed decision information
    for _, ch := range info.OutputChannels {
        log.Info(ctx, "Channel ranking",
            log.Int("channel_id", ch.ChannelID),
            log.Int("rank", ch.Rank),
            log.Float64("total_score", ch.TotalScore),
        )
    }
}
```

### Strategy-Specific Logs

**TraceAwareStrategy** logs:
- Debug: When boosting a channel (score: 1000, reason: "last_successful_channel_in_trace")
- Trace: When no trace in context or channel not in trace
- Debug: Errors retrieving trace information

**ErrorAwareStrategy** logs:
- Debug: All penalty calculations with values and reasons
  - Consecutive failures penalty
  - Recent failure penalty (with time-based decay)
  - Low success rate penalty (< 50%)
- Debug: All boost calculations
  - Recent success boost (within 1 minute)
  - High success rate boost (> 90%)
- Warn: When metrics unavailable (uses neutral score)
- Debug: When score clamped to 0

**WeightStrategy** logs:
- Warn: When channel has negative weight (clamped to 0)
- Trace: Weight calculation details

### Viewing Logs

**Enable Debug Logging**:
```bash
export LOG_LEVEL=debug
# or for production
export LOG_LEVEL=info  # will see warnings and errors
```

**Filter Load Balancer Logs**:
```bash
# View all load balancer decisions
tail -f axonhub.log | grep "Load balancing decision"

# View specific channel details
tail -f axonhub.log | grep "Channel load balancing details"

# View TraceAware strategy logs
tail -f axonhub.log | grep "TraceAwareStrategy"

# View ErrorAware strategy logs
tail -f axonhub.log | grep "ErrorAwareStrategy"

# Use jq for structured JSON logs
 tail -f axonhub.log | jq 'select(.msg | contains("Load balancing"))'
 ```

**Production Log Analysis**:
```bash
# Find channels with low scores due to errors
grep "ErrorAwareStrategy.*penalty" axonhub.log | \
  jq '{channel: .channel_name, penalty_reason: .details} | select(.penalty_reason != null)'

# Analyze TraceAware strategy effectiveness
grep "TraceAwareStrategy: boosting" axonhub.log | \
  jq '{channel: .channel_name, trace_id: .trace_id}' | \
  sort | uniq -c | sort -nr
```

### Performance Considerations

1. **Logging Overhead**: Debug-level logs have minimal performance impact when disabled (default log level is typically info or higher)
2. **Structured Logging**: Uses efficient JSON encoding with zap logger
3. **Context-Aware**: Helper functions safely extract request information from context
4. **Opt-in Debug Mode**: Debug mode is disabled by default; explicit opt-in required
5. **Graceful Degradation**: If context information is missing, logs use sensible defaults (e.g., "unknown" for model)

### Debugging Strategy Behavior

**Verify TraceAwareStrategy**:
```bash
# Send request with existing trace_id
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Trace-ID: 12345" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "hello"}]
  }'

# Check logs for trace boosting
tail -f axonhub.log | grep "TraceAwareStrategy: boosting"
```

**Verify ErrorAwareStrategy**:
```bash
# Force channel errors by using invalid API key# Then check logs for penalty application
tail -f axonhub.log | grep "consecutive_failures_penalty"
tail -f axonhub.log | grep "recent_failure_penalty"

# Monitor recovery after fixing errors
tail -f axonhub.log | grep "recent_success_boost"
tail -f axonhub.log | grep "high_success_rate_boost"
```

**Verify WeightStrategy**:
```bash
# Set channel weights in admin UI or database
# Channel A: weight 100
# Channel B: weight 50

# Send multiple requests and check rankings
tail -f axonhub.log | grep "WeightStrategy" | jq '{channel: .channel_name, score: .score}'
# Should see Channel A with double the score of Channel B
```

## Future Enhancements

1. **Connection Tracking**: Implement ConnectionTracker for ConnectionAwareStrategy
2. **Geographic Routing**: Add strategy for geographic proximity
3. **Cost-based Routing**: Add strategy considering channel costs
4. **Dynamic Weights**: Allow runtime weight adjustments based on metrics
5. **A/B Testing**: Support for experimental channel routing
6. **Metrics Integration**: Prometheus metrics for load balancer decisions
7. **Decision Auditing**: Persistent storage of load balancing decisions for analysis

## Related Files

- `/internal/server/chat/load_balancer.go` - Core load balancer with decision logging
- `/internal/server/chat/strategies.go` - Strategy implementations with detailed logging
- `/internal/server/chat/debug.go` - Debug mode implementation and helper functions
- `/internal/server/chat/channels.go` - Channel selector integration
- `/internal/server/biz/channel.go` - Channel service with trace support
- `/internal/server/biz/channel_performance.go` - Performance metrics
