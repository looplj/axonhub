# Trace Multiple Requests - Responses API

This directory contains integration tests for testing multiple Responses API calls within a single trace in AxonHub.

## Test Overview

### TestSingleTraceMultipleCalls

Validates multiple sequential Responses API calls within the same trace context, including:

- **Multi-turn conversation** using `previous_response_id`
- **Context preservation** across multiple API calls  
- **Calculation requests** (15 × 7 + 23 = 128)
- **Response validation** and verification

### TestSingleTraceContextPreservation

Tests information recall across multiple calls:

- **Fact introduction** in first call
- **Selective recall** of specific facts (name, occupation, location)
- **Context chain** validation through response IDs
- **Long-term context** preservation

## Test Flow

### TestSingleTraceMultipleCalls
1. **Call 1** - Initial greeting about calculation task
2. **Call 2** - Follow-up offering to help (uses previous_response_id)
3. **Call 3** - Calculation request (15 × 7 + 23)
4. **Call 4** - Confirmation of calculation result

### TestSingleTraceContextPreservation
1. **Call 1** - Introduce name, age, occupation, and location
2. **Call 2** - Ask about name → should recall "Bob"
3. **Call 3** - Ask about work → should recall "software engineer"
4. **Call 4** - Ask about location → should recall "Seattle"

## Key Features Tested

- ✅ Multiple API calls in single trace
- ✅ Context preservation via `previous_response_id`
- ✅ Response chaining and context accumulation
- ✅ Information recall from earlier calls
- ✅ Trace ID consistency verification

## Key Differences from Chat Completions

The Responses API version differs from the Chat Completions version:

| Feature | Chat Completions | Responses API |
|---------|-----------------|---------------|
| Context | Message array | `previous_response_id` |
| Tool calls | Supported | Not supported |
| State | Stateless (client manages) | Stateful (server manages via IDs) |

## Running the Tests

```bash
# Run all tests in this directory
go test -v

# Run single trace test
go test -v -run TestSingleTraceMultipleCalls

# Run context preservation test
go test -v -run TestSingleTraceContextPreservation
```

## Important Notes

- All calls share the same **Trace ID** (from test context headers)
- Each response has a unique **Response ID** used for chaining
- Context is maintained automatically through response ID chain
- Responses API doesn't support tool calling (calculations done by model directly)
