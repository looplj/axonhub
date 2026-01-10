# Load Balance Testing Guide

Quick reference guide for running and interpreting load balance tests.

## Quick Start

```bash
# Set required environment variable
export TEST_AXONHUB_API_KEY="your-api-key"

# Verify environment
make verify

# Run all tests
make test

# Run specific test categories
make test-trace      # Trace-based load balancing
make test-weight     # Weight-based load balancing
make test-advanced   # Advanced scenarios
```

## Test Categories

### 1. Trace-Based Load Balancing

**Tests**: `TestTrace*`

**What it tests**: Requests in the same trace use the same channel

**Key behavior**:
- TraceAwareStrategy gives 1000 point boost to last successful channel
- Ensures trace consistency across multiple requests
- Different traces can use different channels independently

**Expected results**:
- ✅ All requests in same trace use same channel
- ✅ Different traces can use different channels
- ✅ Trace preference persists across multiple requests

### 2. Weight-Based Load Balancing

**Tests**: `TestWeight*`

**What it tests**: Channels receive requests proportional to their weights

**Key behavior**:
- WeightRoundRobinStrategy normalizes load by weight
- Higher weight channels handle more requests
- Formula: `normalizedCount = effectiveCount / (weight / 100.0)`

**Expected results**:
- ✅ High weight channels (80-100) get ~50% of requests
- ✅ Medium weight channels (40-60) get ~30% of requests
- ✅ Low weight channels (10-30) get ~15% of requests

### 3. Connection-Aware Load Balancing

**Tests**: `TestConnectionAware*`

**What it tests**: Channels with fewer active connections are preferred

**Key behavior**:
- ConnectionAwareStrategy tracks active connections
- Prefers channels with lower connection counts
- Score: `maxScore * (1 - min(connections, cap) / cap)`

**Expected results**:
- ✅ Concurrent requests distributed across channels
- ✅ No single channel handles all concurrent load
- ✅ Channels with 0 connections get priority

### 4. Error-Aware Load Balancing

**Tests**: `TestErrorAware*`

**What it tests**: Channels with failures are penalized

**Key behavior**:
- ErrorAwareStrategy monitors channel health
- Consecutive failures: -50 to -150 points
- Recent errors: -20 points each
- Healthy channels: full 200 points

**Expected results**:
- ✅ Healthy channels are preferred
- ✅ Failing channels are avoided
- ✅ System routes around failures automatically

### 5. Advanced Scenarios

**Tests**: `TestLoadBalancing*`

**What it tests**: Edge cases and optimizations

**Key behaviors**:
- Top-K selection optimization
- Priority group handling
- Inactivity decay
- Scaling factor effects
- Composite strategies

**Expected results**:
- ✅ Efficient channel selection with many channels
- ✅ Priority groups respected
- ✅ Historical load decays over time
- ✅ Custom strategies work correctly

## Test Execution Flow

### Standard Test Flow

```
1. Create TestHelper with unique trace ID
2. Make first request (establishes channel preference)
3. Wait for metrics to be recorded (100ms)
4. Make subsequent requests (should use same channel)
5. Verify responses and consistency
```

### Concurrent Test Flow

```
1. Launch multiple goroutines
2. Each makes independent request
3. Wait for all to complete
4. Verify distribution across channels
5. Check success rate
```

### Failover Test Flow

```
1. Make request that might fail
2. System tries first channel
3. If fails, automatically retries with next channel
4. Continues until success or max retries
5. Verify final success
```

## Interpreting Test Results

### Success Indicators

```
✅ All tests pass
✅ No errors in test output
✅ Response times < 5 seconds
✅ Success rate > 90%
✅ Trace consistency maintained
```

### Warning Signs

```
⚠️ Some tests fail intermittently
⚠️ Response times > 10 seconds
⚠️ Success rate 70-90%
⚠️ Uneven distribution (all requests to one channel)
```

### Critical Issues

```
❌ All tests fail
❌ Server unreachable
❌ No channels available
❌ All channels failing
❌ Trace inconsistency
```

## Common Test Failures

### "Trace inconsistency detected"

**Cause**: Different channels used within same trace

**Fix**:
1. Verify TraceAwareStrategy is enabled
2. Check trace IDs are passed correctly in headers
3. Ensure request service tracks successful channels
4. Review server logs for load balancing decisions

### "Uneven distribution"

**Cause**: All requests going to one channel

**Fix**:
1. Check multiple channels support the model
2. Verify channel weights are different
3. Ensure channels are enabled
4. Check if trace context is interfering

### "High failure rate"

**Cause**: Many requests failing

**Fix**:
1. Check channel health and availability
2. Verify API keys are valid
3. Ensure model is available on channels
4. Review error logs for specific failures

### "Connection timeout"

**Cause**: Requests timing out

**Fix**:
1. Increase TEST_TIMEOUT value
2. Check server is running and responsive
3. Verify network connectivity
4. Review server resource usage

## Debug Techniques

### Enable Debug Logging

```bash
# On server
export AXONHUB_LOAD_BALANCER_DEBUG=true

# Restart server
# Run tests again
```

### Check Server Logs

Look for these log entries:

```
Load balancing decision completed
  total_channels=4
  selected_channels=3
  top_channel_id=1
  top_channel_score=1250.5
  
Channel load balancing details
  channel_id=1
  channel_name="High Priority"
  total_score=1250.5
  final_rank=1
  strategy_breakdown={...}
```

### Verify Channel Configuration

```bash
# Check channels via API
curl -H "Authorization: Bearer $API_KEY" \
     http://localhost:8090/graphql \
     -d '{"query": "{ channels { id name orderingWeight status } }"}'
```

### Monitor Metrics

```bash
# Check channel metrics
curl -H "Authorization: Bearer $API_KEY" \
     http://localhost:8090/graphql \
     -d '{"query": "{ channels { id metrics { requestCount successRate } } }"}'
```

## Performance Benchmarks

### Expected Performance

- **Single request**: < 2 seconds
- **10 concurrent requests**: < 5 seconds
- **20 sequential requests**: < 30 seconds
- **Test suite completion**: < 5 minutes

### Performance Tuning

If tests are slow:

1. **Reduce request count** in weight distribution tests
2. **Decrease sleep delays** between requests
3. **Use faster model** (set TEST_MODEL)
4. **Run tests in parallel**: `go test -parallel 4`

## Test Maintenance

### Adding New Tests

1. Follow naming convention: `TestLoadBalancing*`
2. Use TestHelper for consistency
3. Add comprehensive logging
4. Document expected behavior
5. Update README with new test

### Updating Existing Tests

1. Maintain backward compatibility
2. Update documentation
3. Verify all related tests still pass
4. Check for timing dependencies

### Test Data Cleanup

Tests use unique trace IDs, so no cleanup needed. However:

- Server metrics accumulate over time
- Consider periodic server restart for clean state
- Use separate test project if needed

## CI/CD Integration

### GitHub Actions Example

```yaml
- name: Run Load Balance Tests
  env:
    TEST_AXONHUB_API_KEY: ${{ secrets.TEST_API_KEY }}
    TEST_OPENAI_BASE_URL: http://localhost:8090/v1
    TEST_MODEL: deepseek-chat
  run: |
    cd integration_test/openai/chat/load_balance
    make ci
```

### Pre-commit Hook

```bash
#!/bin/bash
cd integration_test/openai/chat/load_balance
make test-quick || exit 1
```

## Troubleshooting Checklist

Before reporting issues, verify:

- [ ] Server is running on correct port
- [ ] TEST_AXONHUB_API_KEY is set and valid
- [ ] Multiple channels are configured and enabled
- [ ] Channels support the test model
- [ ] Network connectivity is working
- [ ] Server logs show no errors
- [ ] Tests compile without errors
- [ ] Environment variables are correct

## Getting Help

If tests continue to fail:

1. Check server logs for errors
2. Review channel configuration
3. Verify load balancing is enabled
4. Test with single channel first
5. Enable debug mode for detailed logs
6. Check GitHub issues for similar problems
7. Contact maintainers with logs and configuration

## Additional Resources

- [Load Balancer Implementation](../../../../internal/server/orchestrator/load_balancer.go)
- [Strategy Documentation](../../../../internal/server/orchestrator/lb_strategy_*.go)
- [Main README](./README.md)
- [AxonHub Documentation](../../../../docs/)
