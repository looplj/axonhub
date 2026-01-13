package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

func TestMain(m *testing.M) {
	// Set up any global test configuration here if needed
	code := m.Run()
	os.Exit(code)
}

func TestMultipleToolsSequential(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "TestMultipleToolsSequential")

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Question that explicitly requires multiple tool calls
	question := "Please use the available tools to: 1) Get the current weather in New York, and 2) Calculate 15 * 23. Provide both results."

	t.Logf("Sending question: %s", question)

	// Define multiple tools
	weatherFunction := shared.FunctionDefinitionParam{
		Name:        "get_current_weather",
		Description: openai.String("Get the current weather for a specified location"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]string{
					"type": "string",
				},
			},
			"required": []string{"location"},
		},
	}

	calculatorFunction := shared.FunctionDefinitionParam{
		Name:        "calculate",
		Description: openai.String("Perform mathematical calculations"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]string{
					"type": "string",
				},
			},
			"required": []string{"expression"},
		},
	}

	weatherTool := openai.ChatCompletionFunctionTool(weatherFunction)
	calculatorTool := openai.ChatCompletionFunctionTool(calculatorFunction)

	// Prepare the chat completion request with multiple tools
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Tools:     []openai.ChatCompletionToolUnionParam{weatherTool, calculatorTool},
		Model:     helper.GetModel(),
		Verbosity: openai.ChatCompletionNewParamsVerbosity("high"),
	}

	// Make the initial API call
	completion, err := helper.CreateChatCompletionWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with multiple tools")

	// Validate the response
	helper.ValidateChatResponse(t, completion, "Multiple tools sequential")

	// Check if tool calls were made
	choices := completion.Choices
	if len(choices) == 0 {
		t.Fatal("No choices in response")
	}

	message := choices[0].Message
	t.Logf("Response message: %+v", message)

	// Check for tool calls
	if len(message.ToolCalls) == 0 {
		t.Fatalf("Expected tool calls, but got none. Response: %s", message.Content)
	}

	t.Logf("Number of tool calls: %d", len(message.ToolCalls))

	// Process each tool call
	var toolResults []string
	for i, toolCall := range message.ToolCalls {
		t.Logf("Processing tool call %d: %s", i+1, toolCall.Function.Name)

		var args map[string]interface{}
		err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		helper.AssertNoError(t, err, "Failed to parse tool arguments")

		// Simulate tool execution based on function name
		var result string
		switch toolCall.Function.Name {
		case "get_current_weather":
			result = simulateWeatherFunction(args)
		case "calculate":
			calcResult := simulateCalculatorFunction(args)
			result = fmt.Sprintf("%v", calcResult)
		default:
			result = "Unknown function"
		}

		toolResults = append(toolResults, result)
		t.Logf("Tool %s result: %s", toolCall.Function.Name, result)
	}

	// Continue the conversation with all tool results
	params.Messages = append(params.Messages, message.ToParam())
	for i, toolCall := range message.ToolCalls {
		params.Messages = append(params.Messages, openai.ToolMessage(toolResults[i], toolCall.ID))
	}

	// Make the follow-up call
	finalCompletion, err := helper.CreateChatCompletionWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed to get final completion")

	// Validate the final response
	helper.ValidateChatResponse(t, finalCompletion, "Multiple tools - final response")

	finalResponse := finalCompletion.Choices[0].Message.Content
	t.Logf("Final response: %s", finalResponse)

	// Verify the final response incorporates information from multiple tools
	if len(finalResponse) == 0 {
		t.Error("Expected non-empty final response")
	}
}

func TestMultipleToolsParallel(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "TestMultipleToolsParallel")

	ctx := helper.CreateTestContext()

	// Question that explicitly requires parallel tool calls
	question := "Please use the available tools to: 1) Get weather for New York, 2) Get weather for London, and 3) Calculate 100 / 4. I need all three results."

	t.Logf("Sending question: %s", question)

	// Define multiple tools
	weatherFunction := shared.FunctionDefinitionParam{
		Name:        "get_current_weather",
		Description: openai.String("Get the current weather for a specified location"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]string{
					"type": "string",
				},
			},
			"required": []string{"location"},
		},
	}

	calculatorFunction := shared.FunctionDefinitionParam{
		Name:        "calculate",
		Description: openai.String("Perform mathematical calculations"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]string{
					"type": "string",
				},
			},
			"required": []string{"expression"},
		},
	}

	weatherTool := openai.ChatCompletionFunctionTool(weatherFunction)
	calculatorTool := openai.ChatCompletionFunctionTool(calculatorFunction)

	// Enable parallel tool calls
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Tools:             []openai.ChatCompletionToolUnionParam{weatherTool, calculatorTool},
		ParallelToolCalls: openai.Bool(true),
		Model:             helper.GetModel(),
	}

	completion, err := helper.CreateChatCompletionWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with parallel tools")

	helper.ValidateChatResponse(t, completion, "Parallel tools")

	if len(completion.Choices) == 0 {
		t.Fatal("No choices returned for parallel execution")
	}

	currentCompletion := completion
	const maxParallelIterations = 5
	var finalCompletion *openai.ChatCompletion

	for iteration := 0; iteration < maxParallelIterations; iteration++ {
		message := currentCompletion.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			finalCompletion = currentCompletion
			break
		}

		if iteration == 0 {
			t.Logf("Number of parallel tool calls: %d", len(message.ToolCalls))
		} else {
			t.Logf("Additional parallel tool call round %d: %d", iteration+1, len(message.ToolCalls))
		}

		params.Messages = append(params.Messages, message.ToParam())
		for i, toolCall := range message.ToolCalls {
			result := executeSimulatedToolCall(t, helper, toolCall.Function.Name, toolCall.Function.Arguments)
			params.Messages = append(params.Messages, openai.ToolMessage(result, toolCall.ID))
			t.Logf("Parallel tool %d (%s) result: %s", i+1, toolCall.Function.Name, result)
		}

		currentCompletion, err = helper.CreateChatCompletionWithHeaders(ctx, params)
		helper.AssertNoError(t, err, "Failed to continue parallel completion")
		helper.ValidateChatResponse(t, currentCompletion, fmt.Sprintf("Parallel tools iteration %d", iteration+2))
	}

	if finalCompletion == nil {
		t.Fatalf("Parallel tool execution did not finish within %d iterations", maxParallelIterations)
	}

	finalResponse := finalCompletion.Choices[0].Message.Content
	t.Logf("Final parallel response: %s", finalResponse)

	// Verify response contains information from multiple sources
	if !testutil.ContainsCaseInsensitive(finalResponse, "weather") && !testutil.ContainsCaseInsensitive(finalResponse, "25") {
		t.Errorf("Expected response to contain weather or calculation info, got: %s", finalResponse)
	}
}

func TestToolChoiceRequired(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "TestToolChoiceRequired")

	ctx := helper.CreateTestContext()

	// Question that explicitly requires tool usage
	question := "Please use the calculate tool to compute 50 * 30 and tell me the result."

	t.Logf("Sending question: %s", question)

	// Define calculator tool
	calculatorFunction := shared.FunctionDefinitionParam{
		Name:        "calculate",
		Description: openai.String("Perform mathematical calculations"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]string{
					"type": "string",
				},
			},
			"required": []string{"expression"},
		},
	}

	calculatorTool := openai.ChatCompletionFunctionTool(calculatorFunction)

	// Force tool usage with tool choice - using string instead of constant
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Tools: []openai.ChatCompletionToolUnionParam{calculatorTool},
		ToolChoice: openai.ChatCompletionToolChoiceOptionUnionParam{
			OfFunctionToolChoice: &openai.ChatCompletionNamedToolChoiceParam{
				Function: openai.ChatCompletionNamedToolChoiceFunctionParam{Name: "calculate"},
				// Type field will be automatically set to "function" by the API
			},
		},
		Model: helper.GetModel(),
	}

	completion, err := helper.CreateChatCompletionWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with forced tool choice")

	helper.ValidateChatResponse(t, completion, "Forced tool choice")

	// Verify tool was called
	if len(completion.Choices) == 0 || completion.Choices[0].Message.ToolCalls == nil {
		t.Fatalf("Expected tool call with forced choice, got: %s", completion.Choices[0].Message.Content)
	}

	toolCall := completion.Choices[0].Message.ToolCalls[0]
	if toolCall.Function.Name != "calculate" {
		t.Errorf("Expected calculate function, got: %s", toolCall.Function.Name)
	}

	// Process tool result
	var args map[string]interface{}
	err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
	helper.AssertNoError(t, err, "Failed to parse forced tool arguments")

	result := simulateCalculatorFunction(args)
	t.Logf("Calculation result: %v", result)

	// Continue conversation
	params.Messages = append(params.Messages, completion.Choices[0].Message.ToParam())
	params.Messages = append(params.Messages, openai.ToolMessage(fmt.Sprintf("%v", result), toolCall.ID))

	// Remove tool choice for final response
	params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{}

	finalCompletion, err := helper.CreateChatCompletionWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed to get forced tool final completion")

	finalResponse := finalCompletion.Choices[0].Message.Content
	t.Logf("Final forced tool response: %s", finalResponse)

	// Verify the answer is correct (50 * 30 = 1500)
	if !testutil.ContainsAnyCaseInsensitive(finalResponse, "1500", "1,500", "one thousand five hundred") {
		t.Errorf("Expected answer to contain 1500, got: %s", finalResponse)
	}
}

// Simulation functions (same as in tool_single)

func simulateWeatherFunction(args map[string]interface{}) string {
	location, _ := args["location"].(string)

	// Mock weather data based on location
	weatherData := map[string]map[string]string{
		"new york": {"temp": "22", "condition": "Partly cloudy", "humidity": "65%"},
		"london":   {"temp": "18", "condition": "Rainy", "humidity": "80%"},
		"tokyo":    {"temp": "25", "condition": "Sunny", "humidity": "60%"},
		"paris":    {"temp": "20", "condition": "Clear", "humidity": "55%"},
	}

	// Default weather data
	defaultWeather := map[string]string{"temp": "20", "condition": "Sunny", "humidity": "50%"}

	weather := defaultWeather
	if cityWeather, exists := weatherData[normalizeLocation(location)]; exists {
		weather = cityWeather
	}

	return fmt.Sprintf("Current weather in %s: %s°C, %s, humidity %s",
		location, weather["temp"], weather["condition"], weather["humidity"])
}

func simulateCalculatorFunction(args map[string]interface{}) float64 {
	expression, _ := args["expression"].(string)

	// Simple mock calculation - in real implementation, this would use a proper math library
	switch expression {
	case "15 * 23":
		return 345
	case "100 / 4":
		return 25
	case "50 * 30":
		return 1500
	case "10 + 5":
		return 15
	default:
		return 42 // Default answer
	}
}

func normalizeLocation(location string) string {
	// Simple normalization - convert to lowercase
	return strings.ToLower(location)
}

func executeSimulatedToolCall(t *testing.T, helper *testutil.TestHelper, functionName string, rawArgs string) string {
	t.Helper()

	var args map[string]interface{}
	err := json.Unmarshal([]byte(rawArgs), &args)
	helper.AssertNoError(t, err, "Failed to parse tool arguments")

	switch functionName {
	case "get_current_weather":
		return simulateWeatherFunction(args)
	case "calculate":
		calcResult := simulateCalculatorFunction(args)
		return fmt.Sprintf("%v", calcResult)
	default:
		return "Unknown function"
	}
}
