# Streaming Chat Completion Test Case

This test case demonstrates streaming chat completions and real-time response handling using the OpenAI API.

## Tests Included

1. **TestStreamingChatCompletion** - Basic streaming chat completion
2. **TestStreamingWithTools** - Streaming with tool calls and responses
3. **TestStreamingLongResponse** - Long-form content streaming
4. **TestStreamingErrorHandling** - Error handling for streaming requests

## Features Tested

- Real-time streaming response processing
- Chunk-by-chunk content aggregation
- Tool call handling in streaming mode
- Long response streaming with token limits
- Error handling and recovery
- Content validation and verification

## Running the Tests

```bash
# Run all tests in this directory
go test -v

# Run a specific test
go test -v -run TestStreamingChatCompletion

# Run streaming with tools
go test -v -run TestStreamingWithTools

# Run long response test
go test -v -run TestStreamingLongResponse
```

## Expected Behavior

1. **Real-time Streaming**: Responses should be received in chunks as they are generated
2. **Content Accumulation**: All chunks should be properly aggregated into final response
3. **Tool Integration**: Tool calls should be detected and processed during streaming
4. **Error Handling**: Invalid requests should be handled gracefully
5. **Performance**: Streaming should provide faster perceived response times

## Test Scenarios

- **Basic Streaming**: Simple question with immediate response
- **Tool Streaming**: Complex queries requiring tool calls in streaming mode
- **Long Content**: Detailed explanations that generate many chunks
- **Error Cases**: Invalid parameters and network issues

## Notes

- Streaming provides better user experience for long responses
- Tool calls in streaming mode allow for interactive conversations
- Error handling is crucial for production streaming applications
- Token counting helps monitor API usage and costs
