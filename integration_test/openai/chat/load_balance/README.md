# Load Balance Integration Tests

This directory contains comprehensive integration tests for AxonHub's load balancing system. These tests verify that the load balancing strategies work correctly in real-world scenarios.

## Overview

AxonHub uses a sophisticated multi-strategy load balancing system to distribute requests across multiple channels. The load balancer combines several strategies to make optimal routing decisions:

### Load Balancing Strategies

1. **TraceAwareStrategy** (Priority 1)
   - Score: 0 or 1000 points
   - Gives massive boost (1000 points) to the last successful channel in a trace
   - Ensures trace consistency - all requests in the same trace use the same channel
   - Overrides all other strategies when active

2. **ErrorAwareStrategy** (Priority 2)
   - Score: 0-200 points
   - Monitors channel health and error rates
   - Penalties:
     - 1 consecutive failure: -50 points
     - 2 consecutive failures: -100 points
     - 3+ consecutive failures: -150 points
     - Recent errors (last 5 min): -20 points each
   - Healthy channels get full 200 points

3. **WeightRoundRobinStrategy** (Priority 3)
   - Score: 10-150 points
   - Distributes load proportionally based on channel weight
   - Formula: `normalizedCount = effectiveCount / (weight / 100.0)`
   - Higher weight channels can handle more requests before score drops
   - Includes inactivity decay to prevent permanent penalization

4. **ConnectionAwareStrategy** (Priority 4)
   - Score: 0-50 points
   - Prefers channels with fewer active connections
   - Formula: `score = maxScore * (1 - min(connections, cap) / cap)`
   - Helps distribute concurrent load evenly

### Total Score Calculation

The final channel score is the sum of all strategy scores:

```
Total Score = TraceAware + ErrorAware + WeightRoundRobin + ConnectionAware
            = (0-1000) + (0-200) + (10-150) + (0-50)
            = 10-1400 points
```

When TraceAwareStrategy is active (1000 points), it dominates all other strategies, ensuring trace consistency.

## Test Files

### `load_balance_test.go`

Core load balancing tests covering the main strategies:

- **TestTraceBasedLoadBalancing**: Verifies that requests in the same trace use the same channel
- **TestTraceBasedLoadBalancing_MultipleTraces**: Tests that different traces can use different channels independently
- **TestWeightBasedLoadBalancing**: Verifies weight-based distribution across channels
- **TestWeightRoundRobinLoadBalancing**: Tests weighted round-robin with concurrent requests
- **TestLoadBalancingWithRetry**: Verifies retry mechanism with load balancing
- **TestTraceAwareStrategyPriority**: Confirms TraceAwareStrategy dominates other strategies
- **TestLoadBalancingDebugMode**: Tests debug logging for load balancing decisions
- **TestLoadBalancingStrategyComposition**: Verifies all strategies work together correctly

### `advanced_test.go`

Advanced load balancing tests for edge cases and optimizations:

- **TestConnectionAwareLoadBalancing**: Tests connection-aware distribution
- **TestErrorAwareLoadBalancing**: Verifies error-based channel penalization
- **TestLoadBalancingWithChannelFailover**: Tests automatic failover to alternative channels
- **TestLoadBalancingTopKSelection**: Verifies top-K optimization for efficiency
- **TestLoadBalancingPriorityGroups**: Tests priority-based channel grouping
- **TestLoadBalancingInactivityDecay**: Verifies historical load decay for inactive channels
- **TestLoadBalancingScalingFactor**: Tests exponential score decay with load
- **TestLoadBalancingWeightNormalization**: Verifies weight-based request normalization
- **TestLoadBalancingCompositeStrategy**: Tests custom strategy composition

## Running the Tests

### Prerequisites

1. **AxonHub server must be running** on `http://localhost:8090` (or set `TEST_OPENAI_BASE_URL`)
2. **Valid API key** must be set in `TEST_AXONHUB_API_KEY` environment variable
3. **Multiple channels configured** with different weights for proper load distribution
4. **Model available** (default: `deepseek-chat`, or set `TEST_MODEL`)

### Environment Variables

```bash
# Required
export TEST_AXONHUB_API_KEY="your-api-key"

# Optional
export TEST_OPENAI_BASE_URL="http://localhost:8090/v1"  # Default
export TEST_MODEL="deepseek-chat"                        # Default
export TEST_DISABLE_TRACE="false"                        # Default
export TEST_DISABLE_THREAD="false"                       # Default
```

### Run All Tests

```bash
# Run all load balance tests
make test

# Or use go test directly
go test -v ./...

# Run with timeout
go test -v -timeout 5m ./...
```

### Run Specific Tests

```bash
# Run only trace-based tests
go test -v -run TestTrace

# Run only weight-based tests
go test -v -run TestWeight

# Run only advanced tests
go test -v -run TestConnection

# Run a specific test
go test -v -run TestTraceBasedLoadBalancing
```

### Debug Mode

Enable debug mode to see detailed load balancing decisions:

```bash
# Set environment variable on server
export AXONHUB_DEBUG_LOAD_BALANCER_ENABLED=true

# Or send AH-Debug header in requests
# (Tests can be modified to include this header)
```

Debug mode logs include:
- Individual strategy scores for each channel
- Total scores and final ranking
- Strategy execution times
- Detailed decision breakdown

## Test Scenarios

### Scenario 1: Trace Consistency

**Goal**: Verify that all requests in the same trace use the same channel.

**Test**: `TestTraceBasedLoadBalancing`

**Expected Behavior**:
1. First request establishes channel preference
2. Subsequent requests in same trace use the same channel
3. TraceAwareStrategy gives 1000 point boost to the last successful channel

**Verification**:
- Check server logs for consistent channel IDs within a trace
- All requests should succeed
- Response times should be consistent

### Scenario 2: Weight-Based Distribution

**Goal**: Verify that channels receive requests proportional to their weights.

**Test**: `TestWeightBasedLoadBalancing`

**Expected Behavior**:
- High weight channels (80-100) get ~50-60% of requests
- Medium weight channels (40-60) get ~25-35% of requests
- Low weight channels (10-30) get ~10-15% of requests

**Verification**:
- Send 20+ requests across different traces
- Check channel metrics for request distribution
- Distribution should match weight ratios

### Scenario 3: Connection-Aware Distribution

**Goal**: Verify that channels with fewer active connections are preferred.

**Test**: `TestConnectionAwareLoadBalancing`

**Expected Behavior**:
- Concurrent requests spread across available channels
- Channels with 0 connections get full 50 points
- Channels with more connections get proportionally lower scores

**Verification**:
- Send 10+ concurrent requests
- Check that requests are distributed across channels
- No single channel should handle all concurrent requests

### Scenario 4: Error-Aware Penalization

**Goal**: Verify that channels with failures are penalized appropriately.

**Test**: `TestErrorAwareLoadBalancing`

**Expected Behavior**:
- Healthy channels maintain full 200 point score
- Channels with consecutive failures lose 50-150 points
- System automatically routes around failing channels

**Verification**:
- All requests should succeed (healthy channels selected)
- If a channel fails, subsequent requests avoid it
- Failed channel recovers after successful requests

### Scenario 5: Automatic Failover

**Goal**: Verify that the system automatically retries with alternative channels.

**Test**: `TestLoadBalancingWithChannelFailover`

**Expected Behavior**:
- If first channel fails, system tries next best channel
- Retry count determined by `MaxChannelRetries` setting
- Load balancing scores determine retry order

**Verification**:
- Requests eventually succeed even if some channels fail
- Check server logs for retry attempts
- Alternative channels should be tried in score order

## Channel Configuration Recommendations

For optimal test results, configure channels with varied weights:

```yaml
channels:
  - name: "High Priority Channel"
    weight: 100
    status: enabled
    
  - name: "Medium Priority Channel"
    weight: 50
    status: enabled
    
  - name: "Low Priority Channel"
    weight: 20
    status: enabled
    
  - name: "Backup Channel"
    weight: 10
    status: enabled
```

## Interpreting Results

### Success Criteria

✅ **All tests pass**: Load balancing is working correctly
✅ **Trace consistency**: Same channel used within a trace
✅ **Weight distribution**: Requests distributed according to weights
✅ **Failover works**: System recovers from channel failures
✅ **No errors**: All requests complete successfully

### Common Issues

❌ **Trace inconsistency**: Different channels used in same trace
- Check TraceAwareStrategy is enabled
- Verify trace IDs are being passed correctly
- Ensure request service is tracking successful channels

❌ **Uneven distribution**: All requests go to one channel
- Check channel weights are configured differently
- Verify WeightRoundRobinStrategy is enabled
- Ensure multiple channels support the requested model

❌ **High failure rate**: Many requests fail
- Check channel health and availability
- Verify ErrorAwareStrategy is working
- Ensure retry policy is enabled

❌ **Slow performance**: Requests take too long
- Check if channels are overloaded
- Verify ConnectionAwareStrategy is distributing load
- Consider increasing channel weights or adding more channels

## Load Balancing Metrics

Monitor these metrics to verify load balancing effectiveness:

1. **Request Distribution**
   - Requests per channel
   - Should match weight ratios (without trace context)

2. **Channel Health**
   - Success rate per channel
   - Consecutive failure counts
   - Recent error counts

3. **Connection Counts**
   - Active connections per channel
   - Should be evenly distributed under load

4. **Trace Consistency**
   - Requests per trace
   - Channel switches within trace (should be 0)

5. **Failover Rate**
   - Retry attempts per request
   - Alternative channels used
   - Final success rate

## Advanced Configuration

### Custom Load Balancer Strategy

The system supports custom load balancing strategies:

```go
// Weighted strategy only
weightedLoadBalancer := NewLoadBalancer(systemService, NewWeightStrategy())

// Adaptive strategy (default)
adaptiveLoadBalancer := NewLoadBalancer(systemService,
    NewTraceAwareStrategy(requestService),
    NewErrorAwareStrategy(channelService),
    NewWeightRoundRobinStrategy(channelService),
    NewConnectionAwareStrategy(channelService, connectionTracker),
)

// Custom composite strategy
customStrategy := NewCompositeStrategy(
    NewWeightStrategy(),
    NewErrorAwareStrategy(channelService),
).WithWeights(2.0, 1.0)
```

### Retry Policy Configuration

Configure retry behavior in system settings:

```yaml
retry_policy:
  enabled: true
  max_channel_retries: 2        # Try up to 3 channels total (1 + 2 retries)
  max_single_channel_retries: 1 # Retry same channel once before switching
  retry_delay_ms: 100           # Wait 100ms between retries
  load_balancer_strategy: "adaptive"  # or "weighted"
```

### Top-K Optimization

The load balancer uses partial sorting to only sort the top K channels needed:

```
K = 1 + MaxChannelRetries
```

This optimization reduces overhead when many channels are available but only a few are needed.

## Troubleshooting

### Enable Debug Logging

```bash
# On server
export AXONHUB_DEBUG_LOAD_BALANCER_ENABLED=true

# Restart server to apply
```

Debug logs show:
- Strategy scores for each channel
- Total scores and rankings
- Execution times
- Decision rationale

### Check Channel Metrics

Query channel metrics to see load distribution:

```graphql
query {
  channels {
    id
    name
    orderingWeight
    status
    metrics {
      requestCount
      successRate
      activeConnections
    }
  }
}
```

### Verify Trace Context

Ensure trace IDs are being passed correctly:

```bash
# Check request headers
curl -H "AH-Trace-Id: test-trace-123" \
     -H "Authorization: Bearer $API_KEY" \
     http://localhost:8090/v1/chat/completions
```

### Review Server Logs

Look for load balancing decision logs:

```
INFO Load balancing decision completed
  total_channels=4
  selected_channels=3
  retry_enabled=true
  max_channel_retries=2
  top_channel_id=1
  top_channel_name="High Priority Channel"
  top_channel_score=1250.5
```

## Contributing

When adding new load balancing tests:

1. Follow the existing test structure
2. Use descriptive test names
3. Add comprehensive logging
4. Document expected behavior
5. Include verification steps
6. Update this README

## References

- [Load Balancer Implementation](../../../../internal/server/orchestrator/load_balancer.go)
- [Strategy Implementations](../../../../internal/server/orchestrator/lb_strategy_*.go)
- [Orchestrator](../../../../internal/server/orchestrator/orchestrator.go)
- [Unit Tests](../../../../internal/server/orchestrator/candidates_loadbalance_test.go)
