# OpenAI Responses API Test Cases

This directory contains comprehensive integration tests for the OpenAI **Responses API** using the `github.com/openai/openai-go/v3/responses` package against the AxonHub OpenAI-compatible `/v1/responses` endpoint.

## Test Organization

Tests are organized into subdirectories by functionality:

### qa_simple/
Basic question-answer tests demonstrating simple Responses API usage:
- **TestSimpleQA** - Basic arithmetic question ("What is 2 + 2?")
- **TestSimpleQAWithDifferentQuestion** - Different question types
- **TestMultipleQuestions** - Sequential independent questions

### streaming/
Streaming response tests:
- **TestResponsesStreaming** - Basic streaming with event handling

### conversation/
Multi-turn conversation tests using `previous_response_id`:
- **TestResponsesConversation** - Basic context preservation across turns
- **TestResponsesConversationWithInstructions** - Conversation with system instructions
- **TestResponsesConversationContextChain** - Long conversation chains

### trace_multiple_requests/
Multiple Responses API calls within a single trace:
- **TestSingleTraceMultipleCalls** - Sequential calls with context chaining
- **TestSingleTraceContextPreservation** - Long-term information recall

### thread_multiple_traces/
Multiple independent traces within a single thread:
- **TestSingleThreadMultipleTraces** - Multiple traces, same thread ID
- **TestSingleThreadTraceIsolation** - Cross-trace context isolation verification

### Root Directory
Comprehensive parameter and feature tests:
- **TestResponsesSimpleQA** - Basic QA
- **TestResponsesMultipleQuestions** - Multiple questions
- **TestResponsesWithInstructions** - Instructions parameter
- **TestResponsesWithTemperature** - Temperature settings (0.0, 0.5, 1.0)
- **TestResponsesWithMaxOutputTokens** - Output token limits
- **TestResponsesStreaming** - Streaming responses
- **TestResponsesWithMetadata** - Custom metadata
- **TestResponsesWithPreviousResponseID** - Context preservation
- **TestResponsesGet** - Response retrieval by ID

## Features Tested

- **Basic API Operations**
  - Creating responses with simple text input
  - Retrieving responses by ID
  - Multiple sequential API calls

- **Parameters**
  - `model` - Model selection
  - `input` - String input
  - `instructions` - System/developer instructions
  - `temperature` - Sampling temperature
  - `top_p` - Nucleus sampling
  - `max_output_tokens` - Output token limit
  - `metadata` - Custom metadata
  - `previous_response_id` - Conversation continuation

- **Streaming**
  - Streaming responses via SSE
  - Event handling for different event types
  - Accumulating streamed content

- **Response Handling**
  - `OutputText()` helper method
  - Response structure validation
  - Token usage information
  - Metadata preservation

## Running the Tests

```bash
# From the repository root, run all tests in this directory
cd integration_test/openai/responses
go test -v

# Or from the repository root without changing directories
go test -v ./integration_test/openai/responses

# Run a specific test
go test -v ./integration_test/openai/responses -run TestResponsesSimpleQA

# Run all streaming tests
go test -v ./integration_test/openai/responses -run Streaming

# Run all parameter tests
go test -v ./integration_test/openai/responses -run "With(Temperature|TopP|MaxOutputTokens)"
```

## Configuration

These tests rely on the same environment configuration as other OpenAI integration tests:

- `TEST_AXONHUB_API_KEY` – AxonHub API key used by the OpenAI-compatible endpoints (required)
- `TEST_OPENAI_BASE_URL` – Base URL for the OpenAI-compatible API (defaults to `http://localhost:8090/v1`)
- `TEST_MODEL` – Default model ID used for testing (defaults to `deepseek-chat`)
- `TEST_TRACE_ID` – Optional trace ID for request tracing
- `TEST_THREAD_ID` – Optional thread ID for request grouping

If required configuration is missing, the tests will be skipped rather than fail.

## Test Structure

All tests follow the same structure:

1. Create a test helper with `testutil.NewTestHelper(t)`
2. Create a context with headers via `helper.CreateTestContext()`
3. Make API calls using `helper.Client.Responses.*`
4. Validate responses using helper assertions
5. Log important information for debugging

Example:

```go
func TestResponsesExample(t *testing.T) {
    helper := testutil.NewTestHelper(t)
    ctx := helper.CreateTestContext()
    
    params := responses.ResponseNewParams{
        Model: shared.ResponsesModel(helper.GetModel()),
        Input: responses.ResponseNewParamsInputUnion{
            OfString: openai.String("Hello!"),
        },
    }
    
    resp, err := helper.Client.Responses.New(ctx, params)
    helper.AssertNoError(t, err, "Failed to get response")
    
    output := resp.OutputText()
    t.Logf("Response: %s", output)
}
```
