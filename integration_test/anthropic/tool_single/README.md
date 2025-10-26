# Single Tool Tests

This directory contains tests for single tool calling functionality with the Anthropic SDK.

## Test Cases

### TestSingleToolCall
Tests basic single tool calling:
- Tool definition and registration
- Tool call detection and execution
- Tool result integration into conversation

### TestWeatherTool
Tests weather information tool:
- Location-based weather queries
- Tool input validation
- Response formatting

### TestCalculatorTool
Tests mathematical calculation tool:
- Expression evaluation
- Mathematical operations
- Error handling for invalid expressions

## Running Tests

```bash
# Run single tool tests
go test -v .

# Run specific test
go test -v -run TestSingleToolCall
```
