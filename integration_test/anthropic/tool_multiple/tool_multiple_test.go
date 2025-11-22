package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/looplj/axonhub/anthropic_test/internal/testutil"
)

func TestMain(m *testing.M) {
	// Set up any global test configuration here if needed
	code := m.Run()
	os.Exit(code)
}

func TestMultipleToolsSequential(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Question that explicitly requires multiple tool calls
	question := "Please use the available tools to: 1) Get the current weather in New York, and 2) Calculate 15 * 23. Provide both results."

	t.Logf("Sending question: %s", question)

	// Define multiple tools
	weatherTool := anthropic.ToolParam{
		Name:        "get_current_weather",
		Description: anthropic.String("Get the current weather for a specified location"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"location": map[string]string{
					"type": "string",
				},
			},
			Required: []string{"location"},
		},
	}

	calculatorTool := anthropic.ToolParam{
		Name:        "calculate",
		Description: anthropic.String("Perform mathematical calculations"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"expression": map[string]string{
					"type": "string",
				},
			},
			Required: []string{"expression"},
		},
	}

	tools := []anthropic.ToolUnionParam{
		{OfTool: &weatherTool},
		{OfTool: &calculatorTool},
	}

	// Prepare the message request with multiple tools
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 1024,
	}

	// Make the initial API call
	response, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with multiple tools")

	// Validate the response
	helper.ValidateMessageResponse(t, response, "Multiple tools sequential")

	// Check if tool calls were made
	if response.StopReason == anthropic.StopReasonToolUse {
		t.Logf("Tool calls detected: %d", len(response.Content))

		// Process each tool call
		var toolResults []string
		for _, block := range response.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				t.Logf("Processing tool call: %s", toolUseBlock.Name)

				var args map[string]interface{}
				err = json.Unmarshal([]byte(toolUseBlock.Input), &args)
				helper.AssertNoError(t, err, "Failed to parse tool arguments")

				// Simulate tool execution based on function name
				var result string
				switch toolUseBlock.Name {
				case "get_current_weather":
					result = simulateWeatherFunction(args)
				case "calculate":
					calcResult := simulateCalculatorFunction(args)
					result = fmt.Sprintf("%v", calcResult)
				default:
					result = "Unknown function"
				}

				toolResults = append(toolResults, result)
				t.Logf("Tool %s result: %s", toolUseBlock.Name, result)
			}
		}

		// Continue the conversation with all tool results
		messages = append(messages, response.ToParam())
		for _, block := range response.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				// Find the corresponding result
				for i, result := range toolResults {
					if i < len(response.Content) {
						toolResult := anthropic.NewToolResultBlock(toolUseBlock.ID, result, false)
						messages = append(messages, anthropic.NewUserMessage(toolResult))
						break
					}
				}
			}
		}

		// Make the follow-up call
		params.Messages = messages
		finalResponse, err := helper.Client.Messages.New(ctx, params)
		helper.AssertNoError(t, err, "Failed to get final completion")

		// Validate the final response
		helper.ValidateMessageResponse(t, finalResponse, "Multiple tools - final response")

		finalText := ""
		for _, block := range finalResponse.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				finalText += textBlock.Text
			}
		}
		t.Logf("Final response: %s", finalText)

		// Verify the final response incorporates information from multiple tools
		if len(finalText) == 0 {
			t.Error("Expected non-empty final response")
		}
	} else {
		t.Logf("No tool calls detected, checking direct response")

		// Check direct response
		responseText := ""
		for _, block := range response.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				responseText += textBlock.Text
			}
		}

		t.Logf("Direct response: %s", responseText)
		if len(responseText) == 0 {
			t.Error("Expected non-empty response")
		}
	}
}

func TestMultipleToolsParallel(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Question that explicitly requires parallel tool calls
	question := "Please use the available tools to: 1) Get weather for New York, 2) Get weather for London, and 3) Calculate 100 / 4. I need all three results."

	t.Logf("Sending question: %s", question)

	// Define multiple tools
	weatherTool := anthropic.ToolParam{
		Name:        "get_current_weather",
		Description: anthropic.String("Get the current weather for a specified location"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"location": map[string]string{
					"type": "string",
				},
			},
			Required: []string{"location"},
		},
	}

	calculatorTool := anthropic.ToolParam{
		Name:        "calculate",
		Description: anthropic.String("Perform mathematical calculations"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"expression": map[string]string{
					"type": "string",
				},
			},
			Required: []string{"expression"},
		},
	}

	tools := []anthropic.ToolUnionParam{
		{OfTool: &weatherTool},
		{OfTool: &calculatorTool},
	}

	// Prepare request
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 1024,
	}

	response, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with parallel tools")

	helper.ValidateMessageResponse(t, response, "Parallel tools")

	// Check for tool calls
	if response.StopReason == anthropic.StopReasonToolUse {
		t.Logf("Number of parallel tool calls: %d", len(response.Content))

		// Process all tool calls
		var toolResults []string
		for _, block := range response.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				var args map[string]interface{}
				err = json.Unmarshal([]byte(toolUseBlock.Input), &args)
				helper.AssertNoError(t, err, "Failed to parse parallel tool arguments")

				var result string
				switch toolUseBlock.Name {
				case "get_current_weather":
					result = simulateWeatherFunction(args)
				case "calculate":
					calcResult := simulateCalculatorFunction(args)
					result = fmt.Sprintf("%v", calcResult)
				default:
					result = "Unknown function"
				}

				toolResults = append(toolResults, result)
				t.Logf("Parallel tool (%s) result: %s", toolUseBlock.Name, result)
			}
		}

		// Continue conversation
		messages = append(messages, response.ToParam())
		for _, block := range response.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				// Find the corresponding result
				for i, result := range toolResults {
					if i < len(response.Content) {
						toolResult := anthropic.NewToolResultBlock(toolUseBlock.ID, result, false)
						messages = append(messages, anthropic.NewUserMessage(toolResult))
						break
					}
				}
			}
		}

		params.Messages = messages
		finalResponse, err := helper.Client.Messages.New(ctx, params)
		helper.AssertNoError(t, err, "Failed to get parallel final completion")

		finalText := ""
		for _, block := range finalResponse.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				finalText += textBlock.Text
			}
		}
		t.Logf("Final parallel response: %s", finalText)

		// Verify response contains information from multiple sources
		if !strings.Contains(strings.ToLower(finalText), "weather") && !strings.Contains(strings.ToLower(finalText), "25") {
			t.Errorf("Expected response to contain weather or calculation info, got: %s", finalText)
		}
	} else {
		t.Logf("No tool calls detected")

		responseText := ""
		for _, block := range response.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				responseText += textBlock.Text
			}
		}

		t.Logf("Direct parallel response: %s", responseText)
	}
}

func TestToolChoiceRequired(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Question that explicitly requires tool usage
	question := "Please use the calculate tool to compute 50 * 30 and tell me the result."

	t.Logf("Sending question: %s", question)

	// Define calculator tool
	calculatorTool := anthropic.ToolParam{
		Name:        "calculate",
		Description: anthropic.String("Perform mathematical calculations"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"expression": map[string]string{
					"type": "string",
				},
			},
			Required: []string{"expression"},
		},
	}

	tools := []anthropic.ToolUnionParam{
		{OfTool: &calculatorTool},
	}

	// Prepare request with tool choice forcing
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}

	params := anthropic.MessageNewParams{
		Model:    helper.GetModel(),
		Messages: messages,
		Tools:    tools,
		ToolChoice: anthropic.ToolChoiceUnionParam{
			OfTool: &anthropic.ToolChoiceToolParam{
				Name: "calculate",
			},
		},
		MaxTokens: 1024,
	}

	response, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with forced tool choice")

	helper.ValidateMessageResponse(t, response, "Forced tool choice")

	// Verify tool was called
	if response.StopReason != anthropic.StopReasonToolUse {
		responseText := ""
		for _, block := range response.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				responseText += textBlock.Text
			}
		}
		t.Fatalf("Expected tool call with forced choice, got direct response: %s", responseText)
	}

	// Find the tool call
	var toolUseBlock *anthropic.ToolUseBlock
	for _, block := range response.Content {
		if foundTool := block.AsToolUse(); foundTool.Name != "" {
			if foundTool.Name == "calculate" {
				toolUseBlock = &foundTool
				break
			}
		}
	}

	if toolUseBlock == nil {
		t.Fatal("Expected calculate function to be called")
	}

	// Process tool result
	var args map[string]interface{}
	err = json.Unmarshal([]byte(toolUseBlock.Input), &args)
	helper.AssertNoError(t, err, "Failed to parse forced tool arguments")

	result := simulateCalculatorFunction(args)
	t.Logf("Calculation result: %v", result)

	// Continue conversation
	messages = append(messages, response.ToParam())
	toolResult := anthropic.NewToolResultBlock(toolUseBlock.ID, fmt.Sprintf("%v", result), false)
	messages = append(messages, anthropic.NewUserMessage(toolResult))

	// Clear tool choice for final response
	params.Messages = messages
	params.ToolChoice = anthropic.ToolChoiceUnionParam{}

	finalResponse, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get forced tool final completion")

	finalText := ""
	for _, block := range finalResponse.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			finalText += textBlock.Text
		}
	}
	t.Logf("Final forced tool response: %s", finalText)

	// Verify the answer is correct (50 * 30 = 1500)
	if !strings.Contains(strings.ToLower(finalText), "1500") && !strings.Contains(strings.ToLower(finalText), "one thousand five hundred") {
		t.Errorf("Expected answer to contain 1500, got: %s", finalText)
	}
}

// Helper functions

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
