package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/looplj/axonhub/gemini_test/internal/testutil"
	"google.golang.org/genai"
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
	modelName := helper.GetModel()

	// Start conversation with tool requirement
	question := "What's the weather like in Tokyo?"

	t.Logf("Testing single tool call: %s", question)

	// Define weather tool for Gemini
	weatherTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "get_current_weather",
				Description: "Get the current weather for a specified location",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"location": {
							Type:        genai.TypeString,
							Description: "The city and country to get weather for",
						},
					},
					Required: []string{"location"},
				},
			},
		},
	}

	tools := []*genai.Tool{weatherTool}

	// Prepare content with tools
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	// Make API call with tools
	response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, &genai.GenerateContentConfig{
		Tools: tools,
	})
	helper.AssertNoError(t, err, "Failed in single tool call")

	helper.ValidateChatResponse(t, response, "Single tool call test")

	// Check for function calls
	if len(response.Candidates) > 0 {
		candidate := response.Candidates[0]
		if candidate.Content != nil {
			var hasFunctionCall bool
			for _, part := range candidate.Content.Parts {
				if part != nil && part.FunctionCall != nil {
					hasFunctionCall = true
					t.Logf("Function call detected: %s", part.FunctionCall.Name)

					if part.FunctionCall.Name == "get_current_weather" {
						// Verify tool call arguments
						location, ok := part.FunctionCall.Args["location"].(string)
						if !ok {
							t.Error("Expected location argument")
						} else {
							t.Logf("Weather requested for: %s", location)

							// Simulate tool execution
							args := map[string]interface{}{"location": location}
							weatherResult := simulateWeatherFunction(args)

							// Continue conversation with tool result
							functionResponse := genai.Part{
								FunctionResponse: &genai.FunctionResponse{
									ID:   part.FunctionCall.ID,
									Name: part.FunctionCall.Name,
									Response: map[string]interface{}{
										"result": weatherResult,
									},
								},
							}

							// Create new content with function response
							newContents := []*genai.Content{
								{
									Parts: []*genai.Part{{Text: question}},
								},
								{
									Parts: []*genai.Part{part, &functionResponse},
								},
							}

							// Get final response
							response2, err := helper.Client.Models.GenerateContent(ctx, modelName, newContents, &genai.GenerateContentConfig{
								Tools: tools,
							})
							helper.AssertNoError(t, err, "Failed to get tool result response")

							helper.ValidateChatResponse(t, response2, "Tool result response")

							// Verify final response incorporates tool result
							finalResponse := testutil.ExtractTextFromResponse(response2)

							t.Logf("Final response: %s", finalResponse)

							if len(finalResponse) == 0 {
								t.Error("Expected non-empty final response")
							}

							// Verify weather information is included
							if !testutil.ContainsCaseInsensitive(finalResponse, "tokyo") &&
								!testutil.ContainsCaseInsensitive(finalResponse, "weather") {
								t.Errorf("Expected final response to mention weather or location, got: %s", finalResponse)
							}
						}
					}
				}
			}

			if !hasFunctionCall {
				t.Logf("No function calls detected, checking direct response")

				// Check if response contains weather information directly
				responseText := testutil.ExtractTextFromResponse(response)

				if len(responseText) == 0 {
					t.Error("Expected non-empty response")
				}

				t.Logf("Direct response: %s", responseText)
			}
		}
	}
}

func TestCalculatorTool(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "tool_single")

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	// Test mathematical calculation
	question := "What is 15 * 7 + 23?"

	t.Logf("Testing calculator tool: %s", question)

	// Define calculator tool for Gemini
	calculatorTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "calculate",
				Description: "Perform mathematical calculations",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"expression": {
							Type:        genai.TypeString,
							Description: "The mathematical expression to evaluate",
						},
					},
					Required: []string{"expression"},
				},
			},
		},
	}

	tools := []*genai.Tool{calculatorTool}

	// Prepare content with tools
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	// Make API call with tools
	response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, &genai.GenerateContentConfig{
		Tools: tools,
	})
	helper.AssertNoError(t, err, "Failed in calculator tool call")

	helper.ValidateChatResponse(t, response, "Calculator tool test")

	// Check for function calls
	if len(response.Candidates) > 0 {
		candidate := response.Candidates[0]
		if candidate.Content != nil {
			var hasFunctionCall bool
			for _, part := range candidate.Content.Parts {
				if part != nil && part.FunctionCall != nil {
					hasFunctionCall = true
					t.Logf("Calculator function call detected")

					if part.FunctionCall.Name == "calculate" {
						expression, ok := part.FunctionCall.Args["expression"].(string)
						if !ok {
							t.Error("Expected expression argument")
						} else {
							t.Logf("Expression to calculate: %s", expression)

							// Verify the expression
							if expression != "15 * 7 + 23" {
								t.Errorf("Expected expression '15 * 7 + 23', got: %s", expression)
							}

							// Simulate calculation
							args := map[string]interface{}{"expression": expression}
							calcResult := simulateCalculatorFunction(args)

							// Continue conversation with result
							resultStr := fmt.Sprintf("%v", calcResult)
							functionResponse := genai.Part{
								FunctionResponse: &genai.FunctionResponse{
									ID:   part.FunctionCall.ID,
									Name: part.FunctionCall.Name,
									Response: map[string]interface{}{
										"result": resultStr,
									},
								},
							}

							// Create new content with function response
							newContents := []*genai.Content{
								{
									Parts: []*genai.Part{{Text: question}},
								},
								{
									Parts: []*genai.Part{part, &functionResponse},
								},
							}

							// Get final response
							response2, err := helper.Client.Models.GenerateContent(ctx, modelName, newContents, &genai.GenerateContentConfig{
								Tools: tools,
							})
							helper.AssertNoError(t, err, "Failed to get calculator result response")

							helper.ValidateChatResponse(t, response2, "Calculator result response")

							// Verify final response
							finalResponse := testutil.ExtractTextFromResponse(response2)

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

			if !hasFunctionCall {
				t.Logf("No function calls detected for calculator")

				// Check direct response
				responseText := testutil.ExtractTextFromResponse(response)

				t.Logf("Direct calculation response: %s", responseText)

				// Should still get a result
				if !testutil.ContainsCaseInsensitive(responseText, "128") &&
					!testutil.ContainsCaseInsensitive(responseText, "one hundred") {
					t.Errorf("Expected response to mention 128, got: %s", responseText)
				}
			}
		}
	}
}

func TestToolCallWithChat(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "tool_single")

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	t.Logf("Testing tool call with chat session")

	// Define weather tool
	weatherTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "get_current_weather",
				Description: "Get the current weather for a specified location",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"location": {
							Type: genai.TypeString,
						},
					},
					Required: []string{"location"},
				},
			},
		},
	}

	tools := []*genai.Tool{weatherTool}

	// Create chat with tools
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
		Tools:       tools,
	}
	chat, err := helper.CreateChatWithHeaders(ctx, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat with tools")

	// Send message that should trigger tool call
	question := "What's the weather like in London?"
	t.Logf("Question: %s", question)

	response, err := chat.SendMessage(ctx, genai.Part{Text: question})
	helper.AssertNoError(t, err, "Failed to send message with tools")

	helper.ValidateChatResponse(t, response, "Chat with tools test")

	// Check for function calls in chat response
	if len(response.Candidates) > 0 {
		candidate := response.Candidates[0]
		if candidate.Content != nil {
			var hasFunctionCall bool
			for _, part := range candidate.Content.Parts {
				if part != nil && part.FunctionCall != nil {
					hasFunctionCall = true
					t.Logf("Function call in chat: %s", part.FunctionCall.Name)

					if part.FunctionCall.Name == "get_current_weather" {
						location, ok := part.FunctionCall.Args["location"].(string)
						if !ok {
							t.Error("Expected location argument")
						} else {
							t.Logf("Weather requested for: %s", location)

							// Simulate tool execution
							args := map[string]interface{}{"location": location}
							weatherResult := simulateWeatherFunction(args)

							// Send function response back to chat
							functionResponse := genai.Part{
								FunctionResponse: &genai.FunctionResponse{
									ID:   part.FunctionCall.ID,
									Name: part.FunctionCall.Name,
									Response: map[string]interface{}{
										"result": weatherResult,
									},
								},
							}

							// Get final response from chat
							response2, err := chat.SendMessage(ctx, functionResponse)
							helper.AssertNoError(t, err, "Failed to get tool result response in chat")

							helper.ValidateChatResponse(t, response2, "Chat tool result response")

							finalResponse := testutil.ExtractTextFromResponse(response2)
							t.Logf("Chat final response: %s", finalResponse)

							if len(finalResponse) == 0 {
								t.Error("Expected non-empty final response in chat")
							}

							// Verify weather information is included
							if !testutil.ContainsCaseInsensitive(finalResponse, "london") &&
								!testutil.ContainsCaseInsensitive(finalResponse, "weather") {
								t.Errorf("Expected chat response to mention weather or location, got: %s", finalResponse)
							}
						}
					}
				}
			}

			if !hasFunctionCall {
				t.Logf("No function calls in chat, checking direct response")
				responseText := testutil.ExtractTextFromResponse(response)
				t.Logf("Direct chat response: %s", responseText)
			}
		}
	}
}

func TestInvalidToolCall(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "tool_single")

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	t.Logf("Testing invalid tool call handling")

	// Define a tool with missing required parameter
	invalidTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "get_weather",
				Description: "Get weather (intentionally missing location parameter)",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"temperature": {
							Type: genai.TypeNumber,
						},
					},
					Required: []string{"location"}, // Required but not in properties
				},
			},
		},
	}

	tools := []*genai.Tool{invalidTool}

	// Prepare content
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: "What's the weather like?"}},
	}}

	// This should either work with direct response or handle the tool definition gracefully
	response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, &genai.GenerateContentConfig{
		Tools: tools,
	})
	if err != nil {
		t.Logf("Correctly caught error for invalid tool: %v", err)
		return
	}

	helper.ValidateChatResponse(t, response, "Invalid tool test")

	// Should get a direct response since the tool is invalid
	responseText := testutil.ExtractTextFromResponse(response)
	t.Logf("Response with invalid tool: %s", responseText)

	if len(responseText) == 0 {
		t.Error("Expected non-empty response even with invalid tool")
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
