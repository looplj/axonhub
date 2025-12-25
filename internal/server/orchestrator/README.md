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

## File Structure

### Core Components

- **`orchestrator.go`** - Main orchestrator implementation that coordinates the entire request pipeline
- **`inbound.go`** - Handles inbound request processing and transformation
- **`outbound.go`** - Manages outbound request processing and provider communication
- **`transformer.go`** - Request/response transformation utilities

### Load Balancing

- **`load_balancer.go`** - Core load balancing logic and `LoadBalancer` struct
- **`load_balancer_debug.go`** - Debug utilities for load balancing decisions
- **`lb_strategy_*.go`** - Load balancing strategy implementations:
  - `lb_strategy_rr.go` - Round-robin and weighted round-robin strategies
  - `lb_strategy_bp.go` - Error-aware/best practices strategy
  - `lb_strategy_composite.go` - Composite strategy combining multiple approaches
  - `lb_strategy_trace.go` - Tracing strategy for debugging
  - `lb_strategy_weight.go` - Weight-based strategy

### Candidate Selection

- **`candidates.go`** - Main candidate selection logic for channels/models
- **`candidates_anthropic.go`** - Anthropic-specific candidate logic
- **`candidates_google.go`** - Google/Gemini-specific candidate logic
- **`select_candidates.go`** - Candidate selection algorithms

### State Management

- **`state.go`** - Orchestrator state management
- **`connection_tracker.go`** - Connection tracking utilities
- **`connection_tracking.go`** - Connection state management

### Request Processing

- **`request.go`** - Request handling and validation
- **`request_execution.go`** - Request execution coordination
- **`retry.go`** - Retry logic and policies

### Utilities

- **`model_mapper.go`** - Model mapping and compatibility
- **`performance.go`** - Performance monitoring and metrics
- **`tester.go`** - Testing utilities

### Documentation

- **`load-balancing.md`** - Detailed load balancing documentation
- **`README.md`** - This file

## Load Balancing Strategies

The orchestrator supports multiple load balancing strategies that can be combined:

1. **Round Robin** - Distributes requests evenly across channels
2. **Weighted Round Robin** - Proportional distribution based on channel weights
3. **Error Aware** - Penalizes channels with recent failures
4. **Composite** - Combines multiple strategies with configurable weights
5. **Trace** - Debug strategy for logging detailed decisions

## Key Interfaces

- **`LoadBalanceStrategy`** - Interface for load balancing strategies
- **`ChannelMetricsProvider`** - Provides channel performance metrics
- **`RetryPolicyProvider`** - Supplies retry policy configuration

## Testing

Comprehensive test coverage includes:
- Unit tests for individual components
- Integration tests for end-to-end flows
- Load balancing strategy tests
- Candidate selection tests
- Performance and stress tests

Run tests with: `go test ./internal/server/orchestrator/...`
