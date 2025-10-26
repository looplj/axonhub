# Conversation Test Case

This test case demonstrates multi-turn conversations with context preservation, system prompts, and tool integration across conversation turns.

## Tests Included

1. **TestMultiTurnConversation** - Basic multi-turn conversation flow
2. **TestConversationWithTools** - Conversation with tool calls and results
3. **TestConversationContextPreservation** - Context maintenance across turns
4. **TestConversationSystemPrompt** - System prompt influence on conversation

## Features Tested

- Multi-turn conversation management
- Context preservation across conversation turns
- System prompt influence and persistence
- Tool integration in conversational flow
- Message history maintenance
- Conversation state management

## Running the Tests

```bash
# Run all tests in this directory
go test -v

# Run a specific test
go test -v -run TestMultiTurnConversation

# Run context preservation test
go test -v -run TestConversationContextPreservation

# Run system prompt test
go test -v -run TestConversationSystemPrompt
```

## Expected Behavior

1. **Context Preservation**: Each turn should maintain context from previous turns
2. **Tool Integration**: Tools should be available and usable throughout the conversation
3. **System Prompt**: System instructions should influence the entire conversation
4. **Message History**: All messages should be properly maintained in order
5. **Natural Flow**: Conversation should feel natural and coherent

## Test Scenarios

- **Basic Conversation**: Simple back-and-forth discussion
- **Tool-Enhanced**: Conversation that requires tool calls and processes results
- **Context Heavy**: Complex topics that require maintaining detailed context
- **System-Guided**: Conversations guided by specific system instructions

## Message Types Tested

- **System Messages**: Instructions that guide the assistant's behavior
- **User Messages**: User inputs and questions
- **Assistant Messages**: AI responses and tool calls
- **Tool Messages**: Results from tool executions

## Notes

- Context length limits may affect very long conversations
- System prompts should be clear and specific for best results
- Tool availability should be consistent throughout the conversation
- Message ordering must be maintained for proper context flow
