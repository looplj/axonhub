# Anthropic Go SDK Integration Tests

This directory contains comprehensive integration tests for the Anthropic Go SDK, demonstrating various API usage patterns with proper header handling for AxonHub integration.

## Test Structure

Each test case is organized in its own directory with dedicated tests and documentation:

### Test Cases

1. **[qa_simple](./qa_simple)** - Basic chat completion tests
   - Simple Q&A interactions using configured model
   - Multiple question testing with context preservation
   - Contextual system prompts

2. **[tool_single](./tool_single)** - Single tool calling tests
   - Weather information tool
   - Calculator tool
   - Tool call validation and continuation

3. **[conversation](./conversation)** - Multi-turn conversation tests
   - Context preservation
   - System prompt influence
   - Tool integration in conversations
   - Message history management

## Common Integration Features

### Headers Integration
All tests include proper AxonHub header handling:
- `AH-Trace-Id`: Request tracing identifier
- `AH-Thread-Id`: Conversation thread identifier

### API Patterns Demonstrated
- **Chat Completions**: Basic and advanced chat interactions
- **Function Calling**: Tool definition, execution, and continuation
- **Context Management**: Multi-turn conversation state
- **Error Handling**: Proper error detection and recovery

### Testing Utilities
- **Configuration Management**: Environment-based configuration
- **Client Setup**: Proper Anthropic client initialization
- **Response Validation**: Consistent response verification
- **Tool Simulation**: Mock tool implementations for testing

## Prerequisites

### Environment Setup
1. **Go 1.25+**: Ensure Go is installed and configured
2. **AxonHub API Key**: Set the `TEST_AXONHUB_API_KEY` environment variable (Anthropic API key)
3. **Dependencies**: Run `go mod tidy` to install required packages

### Required Environment Variables
```bash
export TEST_AXONHUB_API_KEY="your-anthropic-api-key"
export TEST_ANTHROPIC_BASE_URL="https://api.anthropic.com"  # Optional, defaults to Anthropic
export TEST_MODEL="claude-3-5-sonnet-20241022"              # Optional, defaults to Claude 3.5 Sonnet
export TEST_TRACE_ID="test-trace-123"                       # Optional, auto-generated
export TEST_THREAD_ID="test-thread-456"                     # Optional, auto-generated
```

## Running Tests

### Run All Tests
```bash
# From the integration test directory
cd /path/to/axonhub/integration_test/anthropic

# Run all tests
go test -v ./...

# Run with coverage
go test -cover ./...
```

### Run Specific Test Cases
```bash
# Simple Q&A tests
go test -v ./qa_simple

# Single tool tests
go test -v ./tool_single

# Conversation tests
go test -v ./conversation
```

### Using Make
```bash
# Run all tests
make test

# Run specific test suites
make test-qa         # Q&A tests
make test-tools      # Tool tests
make test-conversation # Conversation tests

# Quick validation
make quick

# Full validation (lint + tests + coverage)
make validate
```

### Run Individual Tests
```bash
# Run specific test function
go test -v -run TestSimpleQA ./qa_simple

# Run calculator tool tests
go test -v -run TestCalculatorTool ./tool_single

# Run conversation context preservation
go test -v -run TestConversationContextPreservation ./conversation
```

## Test Configuration

### Configuration Structure
Tests use a centralized configuration system defined in `internal/testutil/`:

- **Config**: Environment variable management
- **Client**: Anthropic client setup with authentication
- **Headers**: AxonHub header generation
- **Helper**: Common test utilities and validation

### Custom Configuration
You can customize test behavior with environment variables:

```bash
# Custom API endpoint
export TEST_ANTHROPIC_BASE_URL="https://your-proxy.com"

# Custom model for tests
export TEST_MODEL="claude-3-haiku-20240307"

# Custom trace/thread IDs for testing
export TEST_TRACE_ID="custom-trace-abc"
export TEST_THREAD_ID="custom-thread-xyz"
```

## Model Configuration

All tests now use a configurable model system that allows you to specify which model to use for testing:

### Environment Variable
Set the `TEST_MODEL` environment variable to specify which model to use:

```bash
# Use Claude 3.5 Sonnet (default)
export TEST_MODEL="claude-3-5-sonnet-20241022"

# Use Claude 3 Haiku
export TEST_MODEL="claude-3-haiku-20240307"

# Use Claude 3.7 Sonnet
export TEST_MODEL="claude-3-7-sonnet-20250219"
```

### Code Usage
Tests automatically use the configured model through the `helper.GetModel()` method:

```go
// Get the configured model
model := helper.GetModel()

// Use in API calls
params := anthropic.MessageNewParams{
    Model:     model,
    Messages:  messages,
    MaxTokens: 1024,
}
```

If no `TEST_MODEL` is specified, the system defaults to `claude-3-5-sonnet-20241022`.

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
Tests show how to maintain conversation context and state across multiple API calls, which is essential for AxonHub's conversation threading system.

### Error Handling
Proper error handling and validation patterns that integrate well with AxonHub's error reporting and logging systems.

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
Solution: Check TEST_ANTHROPIC_BASE_URL and network connectivity
```

**Rate Limiting**
```
Solution: Tests include delays and respect rate limits
```

### Debug Mode
Enable verbose logging by setting test flags:

```bash
go test -v -args -test.debug=true
```

## Model Support

These tests support all Anthropic models available through the API:

- **Claude 3.7 Sonnet**: Latest and most capable model
- **Claude 3.5 Sonnet**: Excellent balance of intelligence and speed
- **Claude 3 Haiku**: Fast and cost-effective

The tests automatically adapt to the capabilities of the selected model and handle differences in tool calling support and response formats.
