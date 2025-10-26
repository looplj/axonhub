# Streaming Tests

This directory contains tests for streaming functionality with the Anthropic SDK.

## Test Cases

### TestBasicStreamingChatCompletion
Tests basic streaming chat completion:
- Real-time streaming response processing
- Chunk-by-chunk content aggregation
- Content validation and verification

### TestLongResponseStreaming
Tests long-form content streaming:
- Detailed explanations that generate many chunks
- Token counting and response validation
- Performance characteristics of streaming

### TestStreamingResponseWithTools
Tests streaming with tool calls:
- Tool call handling in streaming mode
- Tool result integration during streaming
- Multi-turn streaming conversations

### TestStreamingErrorHandling
Tests error handling for streaming requests:
- Invalid parameters and network issues
- Error detection during streaming
- Graceful error recovery

### TestStreamingEventHandling
Tests different streaming event types:
- Event counting and validation
- MessageStart, MessageStop, ContentBlockDelta events
- Real-time event processing

### TestStreamingWithSystemPrompt
Tests streaming with system prompts:
- System prompt integration in streaming
- Context preservation during streaming
- Technical content validation
## Running Tests

```bash
# Run all streaming tests
go test -v .

# Run specific test
go test -v -run TestBasicStreamingChatCompletion

# Run streaming with tools
go test -v -run TestStreamingResponseWithTools

# Run long response test
go test -v -run TestLongResponseStreaming

# Run event handling test
go test -v -run TestStreamingEventHandling

# Run system prompt test
go test -v -run TestStreamingWithSystemPrompt
```

## Notes

- Streaming functionality in Anthropic SDK may differ from OpenAI
- These tests demonstrate streaming patterns that work with Anthropic API
- Tool calling in streaming mode allows for interactive conversations
