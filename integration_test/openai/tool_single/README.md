# Single Tool Call Test Case

This test case demonstrates function calling with a single tool, including both the initial tool call and the follow-up conversation with the tool result.

## Tests Included

1. **TestSingleToolCall** - Tests weather tool calling with conversation continuation
2. **TestSingleToolCallMath** - Tests calculator tool calling with math operations

## Features Tested

- Single function tool definition
- Tool call extraction and validation
- Function argument parsing
- Tool result simulation
- Conversation continuation after tool execution
- Response validation

## Tools Tested

### Weather Tool
- **Function Name**: `get_current_weather`
- **Parameters**:
  - `location` (string, required): The city name
  - `unit` (string, optional): Temperature unit (celsius/fahrenheit)
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
go test -v -run TestSingleToolCall

# Run the math test
go test -v -run TestSingleToolCallMath
```

## Expected Behavior

1. **Tool Call Detection**: The model should recognize when a tool is needed
2. **Function Arguments**: The model should provide correct function arguments
3. **Tool Execution**: Simulated tools return appropriate results
4. **Conversation Flow**: The model should incorporate tool results into the final response
5. **Response Quality**: Final responses should be coherent and informative
