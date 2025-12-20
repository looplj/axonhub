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

func TestSingleToolCall(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "tool_single")

	ctx := helper.CreateTestContext()

	// Start conversation with tool requirement
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather like in Tokyo?")),
	}

	// Define weather tool
	weatherTool := anthropic.ToolParam{
		Name:        "get_current_weather",
		Description: anthropic.String("Get the current weather for a specified location"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"location": map[string]string{
					"type":        "string",
					"description": "The city and country to get weather for",
				},
			},
			Required: []string{"location"},
		},
	}

	tools := []anthropic.ToolUnionParam{
		{OfTool: &weatherTool},
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 1024,
	}

	response, err := helper.CreateMessageWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed in single tool call")

	helper.ValidateMessageResponse(t, response, "Single tool call test")

	// Check for tool calls
	if response.StopReason == anthropic.StopReasonToolUse {
		t.Logf("Tool calls detected: %d", len(response.Content))

		// Process tool calls
		for _, block := range response.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				t.Logf("Tool call: %s", toolUseBlock.Name)

				var args map[string]interface{}
				err = json.Unmarshal([]byte(toolUseBlock.Input), &args)
				helper.AssertNoError(t, err, "Failed to parse tool arguments")

				// Verify tool call arguments
				if toolUseBlock.Name == "get_current_weather" {
					location, ok := args["location"].(string)
					if !ok {
						t.Error("Expected location argument")
					} else {
						t.Logf("Weather requested for: %s", location)

						// Simulate tool execution
						weatherResult := simulateWeatherFunction(args)

						// Continue conversation with tool result
						// First add the model's response to maintain conversation context
						messages = append(messages, response.ToParam())
						toolResult := anthropic.NewToolResultBlock(toolUseBlock.ID, weatherResult, false)
						messages = append(messages, anthropic.NewUserMessage(toolResult))

						// Get final response
						params.Messages = messages
						response2, err := helper.CreateMessageWithHeaders(ctx, params)
						helper.AssertNoError(t, err, "Failed to get tool result response")

						helper.ValidateMessageResponse(t, response2, "Tool result response")

						// Verify final response incorporates tool result
						finalResponse := ""
						for _, block := range response2.Content {
							if textBlock := block.AsText(); textBlock.Text != "" {
								finalResponse += textBlock.Text
							}
						}

						t.Logf("Final response: %s", finalResponse)

						if len(finalResponse) == 0 {
							t.Error("Expected non-empty final response")
						}

						// Verify weather information is included
						if !strings.Contains(strings.ToLower(finalResponse), "tokyo") &&
							!strings.Contains(strings.ToLower(finalResponse), "weather") {
							t.Errorf("Expected final response to mention weather or location, got: %s", finalResponse)
						}
					}
				}
			}
		}
	} else {
		t.Logf("No tool calls detected, checking direct response")

		// Check if response contains weather information directly
		responseText := ""
		for _, block := range response.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				responseText += textBlock.Text
			}
		}

		if len(responseText) == 0 {
			t.Error("Expected non-empty response")
		}

		t.Logf("Direct response: %s", responseText)
	}
}

func TestCalculatorTool(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "tool_single")

	ctx := helper.CreateTestContext()

	// Test mathematical calculation
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("What is 15 * 7 + 23?")),
	}

	// Define calculator tool
	calculatorTool := anthropic.ToolParam{
		Name:        "calculate",
		Description: anthropic.String("Perform mathematical calculations"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"expression": map[string]string{
					"type":        "string",
					"description": "The mathematical expression to evaluate",
				},
			},
			Required: []string{"expression"},
		},
	}

	tools := []anthropic.ToolUnionParam{
		{OfTool: &calculatorTool},
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 1024,
	}

	response, err := helper.CreateMessageWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed in calculator tool call")

	helper.ValidateMessageResponse(t, response, "Calculator tool test")

	// Check for tool calls
	if response.StopReason == anthropic.StopReasonToolUse {
		t.Logf("Calculator tool call detected")

		// Process tool calls
		for _, block := range response.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				if toolUseBlock.Name == "calculate" {
					var args map[string]interface{}
					err = json.Unmarshal([]byte(toolUseBlock.Input), &args)
					helper.AssertNoError(t, err, "Failed to parse calculator arguments")

					expression, ok := args["expression"].(string)
					if !ok {
						t.Error("Expected expression argument")
					} else {
						t.Logf("Expression to calculate: %s", expression)

						// Verify the expression
						if expression != "15 * 7 + 23" {
							t.Errorf("Expected expression '15 * 7 + 23', got: %s", expression)
						}

						// Simulate calculation
						calcResult := simulateCalculatorFunction(args)

						// Continue conversation with result
						// First add the model's response to maintain conversation context
						messages = append(messages, response.ToParam())
						resultStr := fmt.Sprintf("%v", calcResult)
						toolResult := anthropic.NewToolResultBlock(toolUseBlock.ID, resultStr, false)
						messages = append(messages, anthropic.NewUserMessage(toolResult))

						// Get final response
						params.Messages = messages
						response2, err := helper.CreateMessageWithHeaders(ctx, params)
						helper.AssertNoError(t, err, "Failed to get calculator result response")

						helper.ValidateMessageResponse(t, response2, "Calculator result response")

						// Verify final response
						finalResponse := ""
						for _, block := range response2.Content {
							if textBlock := block.AsText(); textBlock.Text != "" {
								finalResponse += textBlock.Text
							}
						}

						t.Logf("Calculator final response: %s", finalResponse)

						// Verify result is included (should contain 128 or "one hundred twenty-eight")
						if !testutil.ContainsCaseInsensitive(finalResponse, "128") &&
							!testutil.ContainsCaseInsensitive(finalResponse, "one hundred") {
							t.Errorf("Expected final response to mention 128, got: %s", finalResponse)
						}
					}
				}
			}
		}
	} else {
		t.Logf("No tool calls detected for calculator")

		// Check direct response
		responseText := ""
		for _, block := range response.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				responseText += textBlock.Text
			}
		}

		t.Logf("Direct calculation response: %s", responseText)

		// Should still get a result
		if !testutil.ContainsCaseInsensitive(responseText, "128") &&
			!testutil.ContainsCaseInsensitive(responseText, "one hundred") {
			t.Errorf("Expected response to mention 128, got: %s", responseText)
		}
	}
}

// Helper functions

func simulateCalculatorFunction(args map[string]interface{}) float64 {
	expression, _ := args["expression"].(string)

	switch expression {
	case "365 * 24":
		return 8760
	case "100 / 4":
		return 25
	case "50 * 30":
		return 1500
	case "15 * 7 + 23":
		return 128
	default:
		return 42
	}
}

func simulateWeatherFunction(args map[string]interface{}) string {
	location, _ := args["location"].(string)

	// Mock weather data
	weatherData := map[string]map[string]string{
		"tokyo":    {"temp": "25", "condition": "Sunny", "humidity": "60%"},
		"london":   {"temp": "18", "condition": "Rainy", "humidity": "80%"},
		"new york": {"temp": "22", "condition": "Partly cloudy", "humidity": "65%"},
		"paris":    {"temp": "20", "condition": "Cloudy", "humidity": "70%"},
	}

	defaultWeather := map[string]string{"temp": "20", "condition": "Sunny", "humidity": "50%"}

	weather := defaultWeather
	if cityWeather, exists := weatherData[strings.ToLower(location)]; exists {
		weather = cityWeather
	}

	return fmt.Sprintf("Current weather in %s: %s°C, %s, humidity %s",
		location, weather["temp"], weather["condition"], weather["humidity"])
}
