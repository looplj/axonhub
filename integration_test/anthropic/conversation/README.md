# Conversation Tests

This directory contains multi-turn conversation tests for the Anthropic SDK, demonstrating context preservation, system prompt influence, and tool integration in conversations.

## Test Cases

### TestMultiTurnConversation
Tests basic multi-turn conversation flow:
- Context preservation across turns
- Follow-up questions maintaining conversation state
- Mathematical calculations within conversations

### TestConversationWithTools
Tests conversation with tool calling:
- Tool definition and execution
- Tool result integration into conversation
- Follow-up questions based on tool results

### TestConversationContextPreservation
Tests context preservation across multiple turns:
- Topic consistency verification
- System prompt influence
- Context scoring across conversation turns

### TestConversationSystemPrompt
Tests system prompt influence:
- Cooking assistant persona
- Recipe and cooking advice
- Topic-specific responses

## Running Tests

```bash
# Run conversation tests
go test -v .

# Run specific test
go test -v -run TestMultiTurnConversation

# Run with verbose logging
go test -v -args -test.v=true
```
