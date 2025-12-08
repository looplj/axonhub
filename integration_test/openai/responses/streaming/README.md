# Streaming Response Test Case - Responses API

This test case demonstrates streaming response generation using the OpenAI Responses API.

## Tests Included

1. **TestResponsesStreaming** - Basic streaming response generation
2. **TestResponsesStreamingWithTools** - Streaming with tool/function calling support
3. **TestResponsesStreamingLongResponse** - Testing longer response generation with streaming
4. **TestResponsesStreamingErrorHandling** - Error handling and validation scenarios

## Features Tested

- Real-time streaming response processing via Responses API
- Event-based streaming (response.output_text.delta events)
- Content aggregation from stream events
- Response validation
- Tool/function calling with streaming (response.tool_call.delta, response.function_call.delta events)
- Long response handling with token limits and temperature control
- Error handling for invalid parameters and streaming failures
- Event type differentiation and processing

## Running the Tests

```bash
# Run all tests in this directory
go test -v

# Run specific tests
go test -v -run TestResponsesStreaming
go test -v -run TestResponsesStreamingWithTools
go test -v -run TestResponsesStreamingLongResponse
go test -v -run TestResponsesStreamingErrorHandling
```

## Expected Behavior

1. **Real-time Streaming**: Responses should be received as events during generation
2. **Content Accumulation**: Delta events should be properly aggregated into final response
3. **Event Types**: Should receive response.output_text.delta and response.completed events
4. **Performance**: Streaming should provide faster perceived response times
5. **Tool Integration**: Tool calls should be properly handled during streaming with appropriate event types
6. **Long Responses**: Extended content should be streamed efficiently with proper token management
7. **Error Handling**: Invalid parameters should be caught and reported appropriately

## Notes

- Responses API streaming uses Server-Sent Events (SSE)
- Events have a `type` field to distinguish event types
- Delta events contain incremental text in the `delta` field
- Streaming is useful for long-form content generation
- Tool calls during streaming produce different event types (response.tool_call.delta, response.function_call.delta)
- The Responses API has different parameter naming compared to Chat Completions API (MaxOutputTokens vs MaxCompletionTokens)
