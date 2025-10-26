# Trace Multiple Requests Integration Test

This directory contains integration tests for testing multiple AI API calls within a single trace in AxonHub using the Anthropic SDK.

## Test Overview

**Test Function:** `TestSingleTraceMultipleCalls`

This test validates the ability to perform multiple sequential AI calls within the same trace context, including:

- **Multi-turn conversation** within a single trace
- **Function calling** with tool integration
- **Response validation** and calculation verification
- **Context preservation** across multiple API calls

## Test Flow

1. **Initial Call** - Simple greeting and project discussion
2. **Follow-up Call** - Task breakdown request
3. **Tool Integration** - Mathematical calculation using a calculator function
4. **Final Call** - Timeline organization based on previous context

## Key Features Tested

- ✅ Multiple API calls in single trace
- ✅ Function calling and tool execution
- ✅ Context preservation between calls
- ✅ Response validation
- ✅ Mathematical calculation accuracy (15 × 7 + 23 = 128)

## Test Tools

- **Calculator Function** - Simulates mathematical calculations
- **Anthropic Messages API** - For AI interactions
- **AxonHub Test Utilities** - For test setup and validation

## Running the Test

```bash
go test -v -run TestSingleTraceMultipleCalls
```
