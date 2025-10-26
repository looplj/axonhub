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

func TestSingleToolCall(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Question that requires a tool call
	question := "What is the current weather in New York?"

	t.Logf("Sending question: %s", question)

	// Define a weather tool using the correct API
	weatherFunction := shared.FunctionDefinitionParam{
		Name:        "get_current_weather",
		Description: openai.String("Get the current weather for a specified location"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]string{
					"type": "string",
				},
				"unit": map[string]any{
					"type": "string",
					"enum": []string{"celsius", "fahrenheit"},
				},
			},
			"required": []string{"location"},
		},
	}

	weatherTool := openai.ChatCompletionFunctionTool(weatherFunction)

	// Prepare the chat completion request with tools
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Tools: []openai.ChatCompletionToolUnionParam{weatherTool},
		Model: helper.GetModel(),
	}

	// Make the initial API call
	completion, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with tools")

	// Validate the response
	helper.ValidateChatResponse(t, completion, "Single tool call")

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

	toolCall := message.ToolCalls[0]
	if toolCall.Function.Name != "get_current_weather" {
		t.Errorf("Expected function name 'get_current_weather', got '%s'", toolCall.Function.Name)
	}

	// Parse the function arguments
	var args map[string]interface{}
	err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
	helper.AssertNoError(t, err, "Failed to parse function arguments")

	// Verify the arguments contain location
	location, ok := args["location"]
	if !ok {
		t.Error("Expected 'location' in function arguments")
	} else {
		t.Logf("Function called with location: %v", location)
	}

	// Simulate tool execution and continue conversation
	weatherResult := simulateWeatherFunction(args)
	t.Logf("Simulated weather result: %s", weatherResult)

	// Continue the conversation with the tool result
	params.Messages = append(params.Messages, message.ToParam())
	params.Messages = append(params.Messages, openai.ToolMessage(weatherResult, toolCall.ID))

	// Make the follow-up call
	finalCompletion, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get final completion")

	// Validate the final response
	helper.ValidateChatResponse(t, finalCompletion, "Single tool call - final response")

	finalResponse := finalCompletion.Choices[0].Message.Content
	t.Logf("Final response: %s", finalResponse)

	// Verify the final response mentions the weather
	if len(finalResponse) == 0 {
		t.Error("Expected non-empty final response")
	}
}

func TestSingleToolCallMath(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Math question that requires calculation
	question := "What is 15 * 23 + 7?"

	t.Logf("Sending math question: %s", question)

	// Define a calculator tool using the correct API
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

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Tools: []openai.ChatCompletionToolUnionParam{calculatorTool},
		Model: helper.GetModel(),
	}

	completion, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with calculator tool")

	helper.ValidateChatResponse(t, completion, "Calculator tool call")

	// Check for tool calls
	if len(completion.Choices) == 0 || completion.Choices[0].Message.ToolCalls == nil {
		t.Fatalf("Expected tool calls for calculator, got: %s", completion.Choices[0].Message.Content)
	}

	toolCall := completion.Choices[0].Message.ToolCalls[0]
	if toolCall.Function.Name != "calculate" {
		t.Errorf("Expected function name 'calculate', got '%s'", toolCall.Function.Name)
	}

	// Parse and verify arguments
	var args map[string]interface{}
	err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
	helper.AssertNoError(t, err, "Failed to parse calculator arguments")

	expression, ok := args["expression"]
	if !ok {
		t.Error("Expected 'expression' in calculator arguments")
	} else {
		t.Logf("Calculator expression: %v", expression)
	}

	// Simulate calculation
	result := simulateCalculatorFunction(args)
	t.Logf("Calculator result: %v", result)

	// Continue conversation
	params.Messages = append(params.Messages, completion.Choices[0].Message.ToParam())
	params.Messages = append(params.Messages, openai.ToolMessage(fmt.Sprintf("%v", result), toolCall.ID))

	finalCompletion, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get final calculation completion")

	finalResponse := finalCompletion.Choices[0].Message.Content
	t.Logf("Final calculation response: %s", finalResponse)

	// Verify the answer is correct (15 * 23 + 7 = 352)
	if !testutil.ContainsCaseInsensitive(finalResponse, "352") && !testutil.ContainsCaseInsensitive(finalResponse, "three hundred fifty-two") {
		t.Errorf("Expected answer to contain 352, got: %s", finalResponse)
	}
}

// Simulation functions

func simulateWeatherFunction(args map[string]interface{}) string {
	location, _ := args["location"].(string)
	unit, _ := args["unit"].(string)

	if unit == "" {
		unit = "celsius"
	}

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

	if unit == "fahrenheit" {
		// Convert to Fahrenheit (rough conversion)
		// Note: This is a simplified conversion for demo purposes
		temp := weather["temp"]
		weather["temp"] = fmt.Sprintf("%.0f", float64(temp[0]-'0')*9/5+32)
	}

	return fmt.Sprintf("Current weather in %s: %s°C, %s, humidity %s",
		location, weather["temp"], weather["condition"], weather["humidity"])
}

func simulateCalculatorFunction(args map[string]interface{}) float64 {
	expression, _ := args["expression"].(string)

	// Simple mock calculation - in real implementation, this would use a proper math library
	switch expression {
	case "15 * 23 + 7":
		return 352
	case "10 + 5":
		return 15
	case "100 / 4":
		return 25
	default:
		return 42 // Default answer
	}
}

func normalizeLocation(location string) string {
	// Simple normalization - convert to lowercase
	return strings.ToLower(location)
}
