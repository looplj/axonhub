# Conversation Test Cases - Responses API

This directory contains conversation test cases using the OpenAI Responses API, including both original Responses API tests and migrated tests from the standard chat completions API.

## Tests Included

### conversation_test.go
1. **TestResponsesConversation** - Basic multi-turn conversation with context preservation
2. **TestResponsesConversationWithInstructions** - Conversation with system instructions
3. **TestResponsesConversationContextChain** - Long conversation chain testing context preservation
4. **TestMultiTurnConversation** - Migrated basic multi-turn conversation (from original conversation tests)
5. **TestConversationWithTools** - Conversation with tool integration (adapted for Responses API)
6. **TestConversationContextPreservation** - Context preservation across multiple turns with system prompt
7. **TestConversationSystemPrompt** - System prompt influence on conversation flow

### conversation_stateless_test.go
1. **TestResponsesConversationStateless** - Stateless conversation using input history arrays
2. **TestResponsesConversationStatelessWithInstructions** - Stateless conversation with instructions
3. **TestResponsesConversationStatelessContextChain** - Longer stateless conversation chain

## Features Tested

- Multi-turn conversations using `previous_response_id`
- Context preservation across multiple turns
- Instructions parameter for consistent assistant behavior
- Context chaining through response IDs
- Information recall from earlier turns
- Tool integration in conversational flow (adapted for Responses API)
- System prompt influence and persistence
- Stateless conversation management using input arrays
- Message history maintenance

## API Formats

### Stateful Conversations (using PreviousResponseID)
- Uses `PreviousResponseID` to chain conversation turns
- Automatic context management by the API
- Simplified conversation flow

### Stateless Conversations (using input arrays)
- Manages conversation history manually
- Uses `ResponseInputParam` with message arrays
- More control over conversation context

## Running the Tests

```bash
# Run all tests in this directory
go test -v

# Run specific test files
go test -v conversation_test.go
go test -v conversation_stateless_test.go

# Run specific tests
go test -v -run TestResponsesConversation
go test -v -run TestResponsesConversationWithInstructions
go test -v -run TestMultiTurnConversation
go test -v -run TestConversationWithTools
go test -v -run TestConversationContextPreservation
go test -v -run TestConversationSystemPrompt

# Run stateless tests
go test -v -run TestResponsesConversationStateless
```

## Migration Notes

The original conversation tests from `/integration_test/openai/conversation/conversation_test.go` have been migrated to use the Responses API with the following changes:

1. **API Calls**: Changed from `Chat.Completions.New` to `Responses.New`
2. **Parameters**: Updated to use `responses.ResponseNewParams`
3. **Context Management**: Uses `PreviousResponseID` for stateful conversations
4. **Input Format**: Supports both string input and structured input arrays
5. **Tool Support**: Adapted for Responses API (tools may have different support levels)

## Expected Behavior

1. **Context Preservation**: Information from earlier turns should be accessible in later turns
2. **Response Chaining**: Using previous_response_id should maintain conversation context
3. **Instructions Persistence**: Instructions should guide behavior throughout the conversation
4. **Information Recall**: Assistant should remember facts stated in earlier turns
5. **Tool Integration**: Tools should be available and usable throughout conversations
6. **System Prompt Influence**: System instructions should consistently guide assistant behavior

## Notes

- The Responses API uses `previous_response_id` instead of message arrays for context
- Each response has a unique ID that can be referenced in subsequent requests
- Instructions parameter can guide assistant behavior throughout the conversation
- Context is maintained automatically through the response ID chain
- Some tests may be temporarily skipped if they use features not fully supported in the current Responses API implementation
- Tool support in Responses API may differ from standard chat completions API
