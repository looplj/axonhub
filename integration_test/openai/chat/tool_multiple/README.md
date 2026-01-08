# Multiple Tool Calls Test Case

This test case demonstrates function calling with multiple tools available to the model, including both sequential and parallel tool execution scenarios.

## Tests Included

1. **TestMultipleToolsSequential** - Tests multiple tools with sequential execution
2. **TestMultipleToolsParallel** - Tests parallel tool execution with `parallel_tool_calls`
3. **TestToolChoiceRequired** - Tests forced tool choice with specific function

## Features Tested

- Multiple function tools in a single request
- Tool call extraction and validation from multiple tools
- Sequential tool processing and conversation continuation
- Parallel tool calls with `parallel_tool_calls` parameter
- Tool choice forcing to specific functions
- Complex multi-tool response validation

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

## Running the Tests

```bash
# Run all tests in this directory
go test -v

# Run a specific test
go test -v -run TestMultipleToolsSequential

# Run parallel test
go test -v -run TestMultipleToolsParallel

# Run tool choice test
go test -v -run TestToolChoiceRequired
```

## Expected Behavior

1. **Tool Selection**: The model should intelligently choose which tools to call based on the query
2. **Multiple Calls**: The model should handle queries that require information from multiple sources
3. **Parallel Processing**: When enabled, the model should make parallel tool calls efficiently
4. **Forced Choice**: When tool choice is specified, the model should call the designated function
5. **Response Integration**: Final responses should incorporate results from all relevant tools

## Test Scenarios

- **Sequential**: Weather + calculation in a single complex query
- **Parallel**: Multiple independent pieces of information needed simultaneously
- **Forced Choice**: Mathematical queries that should always use the calculator

## Notes

- These tests require a higher token limit and more complex reasoning from the model
- Parallel tool calls may not be supported by all models
- Tool choice forcing ensures specific functions are called for specialized queries
