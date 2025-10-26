# Thread Multiple Traces Integration Test

This directory contains integration tests for testing multiple traces within a single thread in AxonHub using the Anthropic SDK.

## Test Overview

**Test Function:** `TestSingleThreadMultipleTraces`

This test validates the ability to perform multiple traces within the same thread context, including:

- **Multiple traces** in a single thread
- **Thread ID consistency** across all traces
- **Trace ID uniqueness** for each trace
- **Function calling** with cost calculation tools
- **Cross-trace context isolation**

## Test Flow

### Trace 1: Project Planning
- Initial project planning discussion
- Technology and tools recommendation

### Trace 2: Team Management
- Team structure and roles
- Project timeline and milestones

### Trace 3: Resource Planning
- Project cost estimation
- Budget breakdown using calculator function
- Scope adjustment recommendations

## Key Features Tested

- ✅ Multiple traces in single thread
- ✅ Thread ID consistency verification
- ✅ Trace ID uniqueness verification
- ✅ Function calling with cost calculation
- ✅ Context isolation between traces
- ✅ Tool execution and result processing

## Test Tools

- **Cost Calculator Function** - Estimates project costs based on team size, duration, and hourly rates
- **Anthropic Messages API** - For AI interactions
- **AxonHub Test Utilities** - For trace and thread management

## Running the Test

```bash
go test -v -run TestSingleThreadMultipleTraces
```

## Important Notes

- All traces share the same **Thread ID** but have unique **Trace IDs**
- Each trace maintains independent context and conversation history
- Function calls are processed within their respective trace contexts
