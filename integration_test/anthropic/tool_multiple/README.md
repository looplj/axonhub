# Multiple Tool Tests

This directory contains tests for multiple tool calling functionality with the Anthropic SDK.

## Test Cases

### TestMultipleToolsSequential
Tests multiple tools with sequential execution:
- Multiple function tools in a single request
- Tool call extraction and validation from multiple tools
- Sequential tool processing and conversation continuation

### TestMultipleToolsParallel
Tests parallel tool execution:
- Multiple independent pieces of information needed simultaneously
- Tool result integration from parallel calls
- Complex multi-tool response validation

### TestToolChoiceRequired
Tests forced tool choice with specific function:
- Tool choice forcing to specific functions
- Mathematical queries that should always use the calculator
- Tool selection validation

## Tools Tested

### Weather Tool
- **Function Name**: `get_current_weather`
- **Parameters**:
  - `location` (string, required): The city name
- **Simulation**: Returns mock weather data based on location

### Calculator Tool
- **Function Name**: `calculate`
- **Parameters**:
  - `expression` (string, required): Mathematical expression to evaluate
- **Simulation**: Evaluates simple mathematical expressions

## Running Tests

```bash
# Run all multiple tool tests
go test -v .

# Run specific test
go test -v -run TestMultipleToolsSequential

# Run parallel test
go test -v -run TestMultipleToolsParallel

# Run tool choice test
go test -v -run TestToolChoiceRequired
```
