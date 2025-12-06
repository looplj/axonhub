# Thread Multiple Traces - Responses API

This directory contains integration tests for testing multiple traces within a single thread using the Responses API in AxonHub.

## Test Overview

### TestSingleThreadMultipleTraces

Validates multiple independent traces within the same thread context:

- **Multiple traces** in a single thread (Thread ID consistency)
- **Trace ID uniqueness** verification
- **Cross-trace context isolation**
- **Multi-call conversations** within each trace

### TestSingleThreadTraceIsolation

Tests that traces within the same thread maintain separate contexts:

- **Context isolation** between traces
- **No cross-trace information leakage**
- **Independent conversation histories** per trace

## Test Flow

### TestSingleThreadMultipleTraces

#### Trace 1: Project Planning
1. **Call 1** - Ask about project phases
2. **Call 2** - Ask about tools and technologies (uses previous_response_id)

#### Trace 2: Team Management (New Trace, Same Thread)
1. **Call 1** - Ask about team structure
2. **Call 2** - Ask about timeline and milestones (uses previous_response_id)

#### Trace 3: Resource Planning (New Trace, Same Thread)
1. **Call 1** - Ask about cost estimation
2. **Call 2** - Ask about scope adjustment (uses previous_response_id)

### TestSingleThreadTraceIsolation

#### Trace 1: Alice
- Introduce "Alice loves programming"

#### Trace 2: Bob (New Trace, Same Thread)
- Introduce "Bob loves cooking"
- Ask "What's my name?" → Should recall "Bob", NOT "Alice"

## Key Features Tested

- ✅ Multiple traces in single thread
- ✅ Thread ID consistency across traces
- ✅ Trace ID uniqueness for each trace
- ✅ Context isolation between traces
- ✅ `previous_response_id` chaining within each trace
- ✅ Independent conversation histories

## Key Differences from Chat Completions

| Feature | Chat Completions | Responses API |
|---------|-----------------|---------------|
| Context management | Message arrays | `previous_response_id` |
| Trace isolation | Manual (separate message arrays) | Automatic (via trace headers) |
| Tool support | Yes | No |

## Running the Tests

```bash
# Run all tests in this directory
go test -v

# Run multiple traces test
go test -v -run TestSingleThreadMultipleTraces

# Run trace isolation test
go test -v -run TestSingleThreadTraceIsolation
```

## Important Notes

- All traces share the same **Thread ID** (from test configuration)
- Each trace has a unique **Trace ID** (created by `CreateTestHelperWithNewTrace`)
- Traces within the same thread are **logically isolated** - they don't share context
- Each trace maintains its own conversation history via `previous_response_id` chains
- The Responses API automatically handles context isolation based on trace headers
