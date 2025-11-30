# Gemini Go SDK Integration Tests

This directory contains comprehensive integration tests for the Gemini Go SDK using the `github.com/googleapis/go-genai` library, demonstrating various API usage patterns with proper header handling for AxonHub integration.

## Test Structure

Each test case is organized in its own directory with dedicated tests and documentation:

### Test Cases

1. **[qa_simple](./qa_simple)** - Basic chat completion tests
   - Simple Q&A interactions using configured model
   - Multiple question testing with context preservation
   - Conversation history testing
   - Configurable model support

## Common Integration Features

### Headers Integration
All tests include proper AxonHub header handling:
- `AH-Trace-Id`: Request tracing identifier
- `AH-Thread-Id`: Conversation thread identifier

### API Patterns Demonstrated
- **Generate Content**: Basic and advanced chat interactions
- **Chat Sessions**: Multi-turn conversation with context preservation
- **Content Generation**: Text generation with various prompts
- **Error Handling**: Proper error detection and recovery

### Testing Utilities
- **Configuration Management**: Environment-based configuration
- **Client Setup**: Proper Gemini client initialization with go-genai
- **Response Validation**: Consistent response verification
- **Text Extraction**: Helper functions for extracting text from responses

## Prerequisites

### Environment Setup
1. **Go 1.25+**: Ensure Go is installed and configured
2. **AxonHub API Key**: Set the `TEST_AXONHUB_API_KEY` environment variable
3. **Dependencies**: Run `go mod tidy` to install required packages

### Required Environment Variables
```bash
export TEST_AXONHUB_API_KEY="your-api-key-here"
export TEST_GEMINI_BASE_URL="http://localhost:8090/gemini"  # Optional, defaults to AxonHub
export TEST_TRACE_ID="test-trace-123"              # Optional, defaults provided
export TEST_THREAD_ID="test-thread-456"            # Optional, defaults provided
export TEST_PROJECT_ID="test-project"              # Optional, defaults provided
export TEST_MODEL="gemini-1.5-flash"               # Optional, defaults to gemini-1.5-flash
```

## Running Tests

### Run All Tests
```bash
# From the integration test directory
cd /path/to/axonhub/integration_test/gemini

# Run all tests
go test -v ./...

# Run with coverage
go test -cover ./...
```

### Run Specific Test Cases
```bash
# Simple Q&A tests
go test -v ./qa_simple

# Run specific test function
go test -v -run TestSimpleQA ./qa_simple

# Run conversation history test
go test -v -run TestConversationHistory ./qa_simple
```

## Test Configuration

### Configuration Structure
Tests use a centralized configuration system defined in `internal/testutil/`:

- **Config**: Environment variable management
- **Client**: Gemini client setup with authentication using go-genai
- **Headers**: AxonHub header generation
- **Helper**: Common test utilities and validation

### Custom Configuration
You can customize test behavior with environment variables:

```bash
# Custom API endpoint
export TEST_GEMINI_BASE_URL="https://your-proxy.com/gemini"

# Custom trace/thread IDs for testing
export TEST_TRACE_ID="custom-trace-abc"
export TEST_THREAD_ID="custom-thread-xyz"

# Custom model for tests
export TEST_MODEL="gemini-1.5-pro"
```

## Model Configuration

All tests use a configurable model system that allows you to specify which model to use for testing:

### Environment Variable
Set the `TEST_MODEL` environment variable to specify which model to use:

```bash
# Use Gemini 1.5 Flash (default)
export TEST_MODEL="gemini-1.5-flash"

# Use Gemini 1.5 Pro
export TEST_MODEL="gemini-1.5-pro"

# Use Gemini Pro
export TEST_MODEL="gemini-pro"
```

### Code Usage
Tests automatically use the configured model through the `helper.GetModel()` method:

```go
// Get the configured model
modelName := helper.GetModel()

// Use in API calls
model := helper.Client.GenerativeModel(modelName)
response, err := model.GenerateContent(ctx, parts...)
```

If no `TEST_MODEL` is specified, the system defaults to `gemini-1.5-flash`.

## Integration with AxonHub

These tests demonstrate proper integration patterns for AxonHub:

### Header Propagation
All requests include standard AxonHub headers for tracing and threading:

```go
headers := map[string]string{
    "AH-Trace-Id":  traceID,
    "AH-Thread-Id": threadID,
}
```

### Context Management
Tests show how to maintain conversation context and state across multiple API calls using Gemini chat sessions, which is essential for AxonHub's conversation threading system.

### Error Handling
Proper error handling and validation patterns that integrate well with AxonHub's error reporting and logging systems.

## Gemini SDK Specific Features

### go-genai Library Usage
These tests use the official `github.com/googleapis/go-genai` library:

```go
import "github.com/googleapis/go-genai"

// Create client
client, err := genai.NewClient(ctx, opts...)

// Create model
model := client.GenerativeModel("gemini-1.5-flash")

// Generate content
response, err := model.GenerateContent(ctx, genai.Text("Hello"))

// Chat session
session := model.StartChat()
response, err := session.SendMessage(ctx, genai.Text("Follow-up question"))
```

### Content Parts
Gemini uses a flexible content parts system:

```go
parts := []genai.Part{
    genai.Text("Hello"),
    genai.Image("path/to/image.jpg"), // For multimodal
}
```

### Response Structure
Gemini responses have a different structure than OpenAI:

```go
type GenerateContentResponse struct {
    Candidates []*Candidate
    // ... other fields
}

type Candidate struct {
    Content *Content
    // ... other fields
}

type Content struct {
    Parts []Part
    Role  string
}
```

## Development

### Adding New Tests
1. Create a new directory under the test root
2. Add your test files following the existing patterns
3. Include a README.md with documentation
4. Update this main README if needed

### Test Patterns
Follow these patterns for consistency:

- Use `TestMain` for setup/teardown
- Leverage `internal/testutil` for common functionality
- Include comprehensive error checking
- Document test expectations and scenarios
- Use descriptive test names and logging
- Remember to close the client with `defer helper.Client.Close()`

### Code Quality
- Follow Go testing best practices
- Include proper error handling
- Use descriptive variable names
- Add comments for complex logic
- Maintain consistent formatting

## Troubleshooting

### Common Issues

**Missing API Key**
```
Error: TEST_AXONHUB_API_KEY environment variable is required
Solution: Set TEST_AXONHUB_API_KEY environment variable
```

**Import Errors**
```
Solution: Run `go mod tidy` to resolve dependencies
```

**Network Issues**
```
Solution: Check TEST_GEMINI_BASE_URL and network connectivity
```

**Rate Limiting**
```
Solution: Tests include delays and respect rate limits
```

**Client Not Closed**
```
Solution: Always use `defer helper.Client.Close()` to avoid resource leaks
```

### Debug Mode
Enable verbose logging by setting test flags:

```bash
go test -v -args -test.debug=true
```

### Model-Specific Issues
Different Gemini models have different capabilities:

- **gemini-1.5-flash**: Fast, lightweight responses
- **gemini-1.5-pro**: Higher quality, slower responses
- **gemini-pro**: Legacy model, consider upgrading

Make sure your test expectations match the model's capabilities.
