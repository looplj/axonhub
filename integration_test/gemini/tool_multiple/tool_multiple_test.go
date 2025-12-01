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

func TestMultipleToolsSequential(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	// Question that explicitly requires multiple tool calls
	question := "Please use the available tools to: 1) Get the current weather in New York, and 2) Calculate 15 * 23. Provide both results."

	t.Logf("Sending question: %s", question)

	// Define multiple tools for Gemini
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

	calculatorTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "calculate",
				Description: "Perform mathematical calculations",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"expression": {
							Type: genai.TypeString,
						},
					},
					Required: []string{"expression"},
				},
			},
		},
	}

	tools := []*genai.Tool{weatherTool, calculatorTool}

	// Prepare the content request with multiple tools
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	// Make the initial API call
	response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, &genai.GenerateContentConfig{
		Tools: tools,
	})
	helper.AssertNoError(t, err, "Failed to get chat completion with multiple tools")

	// Validate the response
	helper.ValidateChatResponse(t, response, "Multiple tools sequential")

	// Check if function calls were made
	if len(response.Candidates) > 0 {
		candidate := response.Candidates[0]
		if candidate.Content != nil {
			var functionCalls []*genai.Part
			for _, part := range candidate.Content.Parts {
				if part != nil && part.FunctionCall != nil {
					functionCalls = append(functionCalls, part)
				}
			}

			if len(functionCalls) > 0 {
				t.Logf("Function calls detected: %d", len(functionCalls))

				// Process each function call
				var toolResults []map[string]interface{}
				for _, functionCall := range functionCalls {
					t.Logf("Processing function call: %s", functionCall.FunctionCall.Name)

					var result interface{}
					switch functionCall.FunctionCall.Name {
					case "get_current_weather":
						result = simulateWeatherFunction(functionCall.FunctionCall.Args)
					case "calculate":
						calcResult := simulateCalculatorFunction(functionCall.FunctionCall.Args)
						result = calcResult
					default:
						result = "Unknown function"
					}

					toolResults = append(toolResults, map[string]interface{}{
						"name":   functionCall.FunctionCall.Name,
						"result": result,
						"args":   functionCall.FunctionCall.Args,
						"call":   functionCall,
					})
					t.Logf("Function %s result: %v", functionCall.FunctionCall.Name, result)
				}

				// Continue the conversation with all tool results
				newContents := []*genai.Content{
					{Parts: []*genai.Part{{Text: question}}},
					candidate.Content,
				}

				// Add function responses
				for _, toolResult := range toolResults {
					functionCall := toolResult["call"].(*genai.Part)
					functionResponse := &genai.Part{
						FunctionResponse: &genai.FunctionResponse{
							ID:   functionCall.FunctionCall.ID,
							Name: functionCall.FunctionCall.Name,
							Response: map[string]interface{}{
								"result": toolResult["result"],
							},
						},
					}
					newContents = append(newContents, &genai.Content{
						Parts: []*genai.Part{functionResponse},
					})
				}

				// Make the follow-up call
				finalResponse, err := helper.Client.Models.GenerateContent(ctx, modelName, newContents, &genai.GenerateContentConfig{
					Tools: tools,
				})
				helper.AssertNoError(t, err, "Failed to get final completion")

				// Validate the final response
				helper.ValidateChatResponse(t, finalResponse, "Multiple tools - final response")

				finalText := testutil.ExtractTextFromResponse(finalResponse)
				t.Logf("Final response: %s", finalText)

				// Verify the final response incorporates information from multiple tools
				if len(finalText) == 0 {
					t.Error("Expected non-empty final response")
				}
			} else {
				t.Logf("No function calls detected, checking direct response")

				// Check direct response
				responseText := testutil.ExtractTextFromResponse(response)

				t.Logf("Direct response: %s", responseText)
				if len(responseText) == 0 {
					t.Error("Expected non-empty response")
				}
			}
		}
	}
}

func TestMultipleToolsParallel(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	// Question that explicitly requires parallel tool calls
	question := "Please use the available tools to: 1) Get weather for New York, 2) Get weather for London, and 3) Calculate 100 / 4. I need all three results."

	t.Logf("Sending question: %s", question)

	// Define multiple tools
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

	calculatorTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "calculate",
				Description: "Perform mathematical calculations",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"expression": {
							Type: genai.TypeString,
						},
					},
					Required: []string{"expression"},
				},
			},
		},
	}

	tools := []*genai.Tool{weatherTool, calculatorTool}

	// Prepare request
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, &genai.GenerateContentConfig{
		Tools: tools,
	})
	helper.AssertNoError(t, err, "Failed to get chat completion with parallel tools")

	helper.ValidateChatResponse(t, response, "Parallel tools")

	// Check for function calls
	if len(response.Candidates) > 0 {
		candidate := response.Candidates[0]
		if candidate.Content != nil {
			var functionCalls []*genai.Part
			for _, part := range candidate.Content.Parts {
				if part != nil && part.FunctionCall != nil {
					functionCalls = append(functionCalls, part)
				}
			}

			if len(functionCalls) > 0 {
				t.Logf("Number of parallel function calls: %d", len(functionCalls))

				// Process all function calls
				var toolResults []map[string]interface{}
				for _, functionCall := range functionCalls {
					var result interface{}
					switch functionCall.FunctionCall.Name {
					case "get_current_weather":
						result = simulateWeatherFunction(functionCall.FunctionCall.Args)
					case "calculate":
						calcResult := simulateCalculatorFunction(functionCall.FunctionCall.Args)
						result = calcResult
					default:
						result = "Unknown function"
					}

					toolResults = append(toolResults, map[string]interface{}{
						"name":   functionCall.FunctionCall.Name,
						"result": result,
						"args":   functionCall.FunctionCall.Args,
						"call":   functionCall,
					})
					t.Logf("Parallel function (%s) result: %v", functionCall.FunctionCall.Name, result)
				}

				// Continue conversation
				newContents := []*genai.Content{
					{Parts: []*genai.Part{{Text: question}}},
					candidate.Content,
				}

				// Add function responses
				for _, toolResult := range toolResults {
					functionCall := toolResult["call"].(*genai.Part)
					functionResponse := genai.Part{
						FunctionResponse: &genai.FunctionResponse{
							ID:   functionCall.FunctionCall.ID,
							Name: functionCall.FunctionCall.Name,
							Response: map[string]interface{}{
								"result": toolResult["result"],
							},
						},
					}
					newContents = append(newContents, &genai.Content{
						Parts: []*genai.Part{&functionResponse},
					})
				}

				finalResponse, err := helper.Client.Models.GenerateContent(ctx, modelName, newContents, &genai.GenerateContentConfig{
					Tools: tools,
				})
				helper.AssertNoError(t, err, "Failed to get parallel final completion")

				finalText := testutil.ExtractTextFromResponse(finalResponse)
				t.Logf("Final parallel response: %s", finalText)

				// Verify response contains information from multiple sources
				if !strings.Contains(strings.ToLower(finalText), "weather") && !strings.Contains(strings.ToLower(finalText), "25") {
					t.Errorf("Expected response to contain weather or calculation info, got: %s", finalText)
				}
			} else {
				t.Logf("No function calls detected")

				responseText := testutil.ExtractTextFromResponse(response)
				t.Logf("Direct parallel response: %s", responseText)
			}
		}
	}
}

func TestMultipleToolsWithChat(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	t.Logf("Testing multiple tools with chat session")

	// Define multiple tools
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

	calculatorTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "calculate",
				Description: "Perform mathematical calculations",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"expression": {
							Type: genai.TypeString,
						},
					},
					Required: []string{"expression"},
				},
			},
		},
	}

	tools := []*genai.Tool{weatherTool, calculatorTool}

	// Create chat with multiple tools
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
		Tools:       tools,
	}
	chat, err := helper.Client.Chats.Create(ctx, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat with multiple tools")

	// Send message that should trigger multiple tool calls
	question := "I need to know the weather in Tokyo and also calculate 365 * 24. Can you help with both?"
	t.Logf("Question: %s", question)

	response, err := chat.SendMessage(ctx, genai.Part{Text: question})
	helper.AssertNoError(t, err, "Failed to send message with multiple tools")

	helper.ValidateChatResponse(t, response, "Chat with multiple tools test")

	// Check for function calls in chat response
	if len(response.Candidates) > 0 {
		candidate := response.Candidates[0]
		if candidate.Content != nil {
			var functionCalls []*genai.Part
			for _, part := range candidate.Content.Parts {
				if part != nil && part.FunctionCall != nil {
					functionCalls = append(functionCalls, part)
				}
			}

			if len(functionCalls) > 0 {
				t.Logf("Multiple function calls in chat: %d", len(functionCalls))

				// Process each function call
				for _, functionCall := range functionCalls {
					t.Logf("Processing chat function call: %s", functionCall.FunctionCall.Name)

					var functionResponse genai.Part
					switch functionCall.FunctionCall.Name {
					case "get_current_weather":
						result := simulateWeatherFunction(functionCall.FunctionCall.Args)
						functionResponse = genai.Part{
							FunctionResponse: &genai.FunctionResponse{
								ID:   functionCall.FunctionCall.ID,
								Name: functionCall.FunctionCall.Name,
								Response: map[string]interface{}{
									"result": result,
								},
							},
						}
					case "calculate":
						calcResult := simulateCalculatorFunction(functionCall.FunctionCall.Args)
						functionResponse = genai.Part{
							FunctionResponse: &genai.FunctionResponse{
								ID:   functionCall.FunctionCall.ID,
								Name: functionCall.FunctionCall.Name,
								Response: map[string]interface{}{
									"result": calcResult,
								},
							},
						}
					default:
						functionResponse = genai.Part{
							FunctionResponse: &genai.FunctionResponse{
								ID:   functionCall.FunctionCall.ID,
								Name: functionCall.FunctionCall.Name,
								Response: map[string]interface{}{
									"result": "Unknown function",
								},
							},
						}
					}

					// Send function response back to chat
					response2, err := chat.SendMessage(ctx, functionResponse)
					helper.AssertNoError(t, err, "Failed to send function response in chat")

					helper.ValidateChatResponse(t, response2, "Chat function result response")

					responseText := testutil.ExtractTextFromResponse(response2)
					t.Logf("Chat function response for %s: %s", functionCall.FunctionCall.Name, responseText)
				}

				// Send a final follow-up message
				finalQuestion := "Based on the results you just got, can you summarize what you found?"
				finalResponse, err := chat.SendMessage(ctx, genai.Part{Text: finalQuestion})
				helper.AssertNoError(t, err, "Failed to get final chat response")

				helper.ValidateChatResponse(t, finalResponse, "Chat final summary")

				finalText := testutil.ExtractTextFromResponse(finalResponse)
				t.Logf("Chat final summary: %s", finalText)

				if len(finalText) == 0 {
					t.Error("Expected non-empty final summary in chat")
				}
			} else {
				t.Logf("No function calls in chat, checking direct response")
				responseText := testutil.ExtractTextFromResponse(response)
				t.Logf("Direct chat response: %s", responseText)
			}
		}
	}
}

func TestToolChoiceRequired(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	// Question that explicitly requires tool usage
	question := "Please use the calculate tool to compute 50 * 30 and tell me the result."

	t.Logf("Sending question: %s", question)

	// Define calculator tool
	calculatorTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "calculate",
				Description: "Perform mathematical calculations",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"expression": {
							Type: genai.TypeString,
						},
					},
					Required: []string{"expression"},
				},
			},
		},
	}

	tools := []*genai.Tool{calculatorTool}

	// Prepare request with tool choice forcing
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	// Note: Gemini doesn't have explicit tool_choice like Anthropic, but we can encourage tool usage
	response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, &genai.GenerateContentConfig{
		Tools:       tools,
		Temperature: genai.Ptr[float32](0.1), // Lower temperature for more deterministic behavior
	})
	helper.AssertNoError(t, err, "Failed to get chat completion with forced tool choice")

	helper.ValidateChatResponse(t, response, "Forced tool choice")

	// Check for function call
	if len(response.Candidates) > 0 {
		candidate := response.Candidates[0]
		if candidate.Content != nil {
			var functionCall *genai.Part
			for _, part := range candidate.Content.Parts {
				if part != nil && part.FunctionCall != nil && part.FunctionCall.Name == "calculate" {
					functionCall = part
					break
				}
			}

			if functionCall != nil {
				t.Logf("Calculator function call detected as expected")

				// Process tool result
				result := simulateCalculatorFunction(functionCall.FunctionCall.Args)
				t.Logf("Calculation result: %v", result)

				// Continue conversation
				functionResponse := &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						ID:   functionCall.FunctionCall.ID,
						Name: functionCall.FunctionCall.Name,
						Response: map[string]interface{}{
							"result": result,
						},
					},
				}

				newContents := []*genai.Content{
					{Parts: []*genai.Part{{Text: question}}},
					candidate.Content,
					{Parts: []*genai.Part{functionResponse}},
				}

				finalResponse, err := helper.Client.Models.GenerateContent(ctx, modelName, newContents, &genai.GenerateContentConfig{
					Tools: tools,
				})
				helper.AssertNoError(t, err, "Failed to get forced tool final completion")

				finalText := testutil.ExtractTextFromResponse(finalResponse)
				t.Logf("Final forced tool response: %s", finalText)

				// Verify the answer is correct (50 * 30 = 1500)
				if !testutil.ContainsAnyCaseInsensitive(finalText, "1500", "1,500", "one thousand five hundred") {
					t.Errorf("Expected answer to contain 1500, got: %s", finalText)
				}
			} else {
				// Check if we got a direct response instead
				responseText := testutil.ExtractTextFromResponse(response)
				t.Logf("Direct response instead of tool call: %s", responseText)

				// Should still get the correct answer
				if !testutil.ContainsAnyCaseInsensitive(responseText, "1500", "1,500", "one thousand five hundred") {
					t.Errorf("Expected answer to contain 1500, got: %s", responseText)
				}
			}
		}
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
	case "365 * 24":
		return 8760
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
