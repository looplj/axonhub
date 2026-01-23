# Orchestrator Package

The orchestrator package is the core component of AxonHub's bidirectional data transformation proxy. It implements the request pipeline that routes client requests through inbound transformers, unified request routing, outbound transformers, and provider communication.

## Architecture Overview

The request pipeline follows this flow:
```
Client → Inbound Transformer → Unified Request Router → Outbound Transformer → Provider
```

This architecture provides:
- Zero learning curve for OpenAI SDK users
- Auto failover and load balancing across channels
- Real-time tracing and per-project usage logs
- Support for multiple API formats (OpenAI, Anthropic, Gemini, and custom variants)
- Model-aware circuit breaking and auto-failover
- Dynamic request body and header overrides with template support
- Quota enforcement and prompt injection
- Model access control via API key profiles
- Channel selection with tag-based filtering
- Native tool support for Anthropic and Google APIs

## File Structure

### Core Components

- **`orchestrator.go`** - Main orchestrator implementation that coordinates the entire request pipeline
- **`inbound.go`** - Handles inbound request processing and persistent stream wrapping
- **`outbound.go`** - Manages outbound request processing and persistent stream wrapping
- **`transformer.go`** - Persistent transformer factory with state management
- **`request.go`** - Request persistence middleware
- **`request_execution.go`** - Request execution coordination
- **`retry.go`** - Retry logic and error handling utilities
- **`state.go`** - Orchestrator state management (PersistenceState)
- **`performance.go`** - Performance monitoring and metrics
- **`prompt.go`** - Prompt injection logic for projects and models
- **`quota.go`** - API key quota enforcement middleware
- **`override.go`** - Request body and header override middleware with template support
- **`model_circuit_breaker.go`** - Circuit breaker tracker for specific models on channels
- **`tester.go`** - Testing utilities

### Load Balancing

- **`load_balancer.go`** - Core load balancing logic and `LoadBalancer` struct with partial sorting
- **`load_balancer_debug.go`** - Debug utilities for load balancing decisions
- **`lb_strategy_rr.go`** - Round-robin strategy with inactivity decay and request count capping
- **`lb_strategy_bp.go`** - Error-aware strategy that penalizes channels with recent failures
- **`lb_strategy_composite.go`** - Composite strategy combining multiple approaches with weights
- **`lb_strategy_trace.go`** - Trace-aware strategy for prioritizing last successful channel
- **`lb_strategy_weight.go`** - Weight-based strategy using channel ordering weight
- **`lb_strategy_model_aware_circuit_breaker.go`** - Strategy that considers model health on specific channels
- **`lb_strategy_random.go`** - Simple random strategy for tie-breaking

### Candidate Selection

- **`candidates.go`** - Main candidate selection logic for channels/models with association cache
- **`candidates_anthropic.go`** - Anthropic-specific candidate logic (native tools support)
- **`candidates_google.go`** - Google/Gemini-specific candidate logic (native tools support)
- **`candidates_stream_policy.go`** - Filters candidates based on request stream requirement and channel policy
- **`select_candidates.go`** - Candidate selection middleware with API key profile filtering

### Connection Tracking

- **`connection_tracker.go`** - Connection tracking utilities (DefaultConnectionTracker)
- **`connection_tracking.go`** - Connection state management

### Model Management

- **`model_mapper.go`** - Model mapping and compatibility
- **`model_access.go`** - Model access control middleware for API key profiles
- **`transform_options.go`** - Transform options application (ForceArrayInstructions, ForceArrayInputs)

### Documentation

- **`load-balancing.md`** - Detailed load balancing documentation
- **`README.md`** - This file

## Load Balancing Strategies

The orchestrator supports multiple load balancing strategies that can be combined:

1. **Trace Aware** - Prioritizes the last successful channel from trace context (highest priority)
2. **Error Aware** - Penalizes channels with recent failures, consecutive errors, and low success rates
3. **Round Robin** - Distributes requests evenly across channels using historical request count with inactivity decay
4. **Connection Aware** - Considers active connection count per channel
5. **Weight** - Uses channel ordering weight for prioritization
6. **Model Aware Circuit Breaker** - Dynamically penalizes channels where the requested model is currently failing
7. **Random** - Adds a small random factor to break ties between channels with identical scores

The load balancer uses partial sorting for efficient top-k candidate selection based on retry policy configuration.

## Key Interfaces

- **`LoadBalanceStrategy`** - Interface for load balancing strategies with `Score()` and `ScoreWithDebug()` methods
- **`ChannelMetricsProvider`** - Provides channel performance metrics (AggregatedMetrics)
- **`RetryPolicyProvider`** - Supplies retry policy configuration
- **`ChannelTraceProvider`** - Provides trace-related channel information
- **`CandidateSelector`** - Interface for selecting channel model candidates
- **`ConnectionTracker`** - Interface for tracking active connections per channel
- **`PromptProvider`** - Supplies enabled prompts for injection
- **`ModelCircuitBreakerProvider`** - Provides model-level circuit breaker statistics and weights

## Pipeline Architecture

The orchestrator uses a pipeline-based architecture with middleware support:

1. **Inbound Pipeline**:
   - API key authentication
   - Quota enforcement
   - Model access control
   - Model mapping
   - Candidate selection (with API key profile and stream policy filtering)
   - Prompt injection
   - Request persistence

2. **Outbound Pipeline**:
   - Channel selection
   - Request body and header overrides
   - Transform options application
   - Performance tracking
   - Request execution persistence
   - Connection tracking
   - Model circuit breaking (optional)
   - Provider communication
   - Response transformation
   - Usage logging

## State Management

The `PersistenceState` struct maintains shared state across the request pipeline:
- API key and user information
- Request and execution tracking
- Channel model candidates
- Performance metrics
- Load balancer and retry policy references

## Testing

Comprehensive test coverage includes:
- Unit tests for individual components
- Integration tests for end-to-end flows
- Load balancing strategy tests
- Candidate selection tests (including cache, tags, decorator tests)
- Performance and stress tests
- Connection tracking tests

Run tests with: `go test ./internal/server/orchestrator/...`
