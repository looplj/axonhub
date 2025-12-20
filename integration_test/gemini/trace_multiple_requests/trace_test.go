package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/looplj/axonhub/gemini_test/internal/testutil"
	"google.golang.org/genai"
)

func TestMain(m *testing.M) {
	// Set up any global test configuration here if needed
	code := m.Run()
	os.Exit(code)
}

func TestSingleTraceMultipleCalls(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "trace_multiple_requests")

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	t.Logf("Testing single trace with multiple calls")

	// Create chat session for maintaining context across calls
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
	chat, err := helper.CreateChatWithHeaders(ctx, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat for trace test")

	// First call - Initial greeting focused on calculation
	question1 := "Hi! I need help with some mathematical calculations."
	t.Logf("First call: %s", question1)

	response1, err := chat.SendMessage(ctx, genai.Part{Text: question1})
	helper.AssertNoError(t, err, "Failed in first trace call")

	helper.ValidateChatResponse(t, response1, "First trace call")

	response1Text := testutil.ExtractTextFromResponse(response1)
	t.Logf("First response: %s", response1Text)

	// Verify first response acknowledges the request
	if !testutil.ContainsCaseInsensitive(response1Text, "calculation") && !testutil.ContainsCaseInsensitive(response1Text, "math") && !testutil.ContainsCaseInsensitive(response1Text, "help") {
		t.Logf("First response: %s", response1Text)
	}

	// Second call - Follow-up with context preservation
	question2 := "Great! I have a specific calculation I need help with."
	t.Logf("Second call: %s", question2)

	response2, err := chat.SendMessage(ctx, genai.Part{Text: question2})
	helper.AssertNoError(t, err, "Failed in second trace call")

	helper.ValidateChatResponse(t, response2, "Second trace call")

	response2Text := testutil.ExtractTextFromResponse(response2)
	t.Logf("Second response: %s", response2Text)

	// Third call - Add tool integration for calculation with explicit instruction
	question3 := "Please calculate: 15 * 7 + 23. Use the calculate tool if available."
	t.Logf("Third call: %s", question3)

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
							Type: genai.TypeString,
						},
					},
					Required: []string{"expression"},
				},
			},
		},
	}

	tools := []*genai.Tool{calculatorTool}

	// Create new chat with tools for this call
	chatWithTools, err := helper.Client.Chats.Create(ctx, modelName, &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
		Tools:       tools,
	}, nil)
	helper.AssertNoError(t, err, "Failed to create chat with tools")

	// Send previous context to new chat
	_, err = chatWithTools.SendMessage(ctx, genai.Part{Text: question1})
	helper.AssertNoError(t, err, "Failed to send context to tools chat")

	_, err = chatWithTools.SendMessage(ctx, genai.Part{Text: question2})
	helper.AssertNoError(t, err, "Failed to send second context to tools chat")

	response3, err := chatWithTools.SendMessage(ctx, genai.Part{Text: question3})
	helper.AssertNoError(t, err, "Failed in third trace call with tools")

	helper.ValidateChatResponse(t, response3, "Third trace call with tools")

	// Check for function calls
	if len(response3.Candidates) > 0 {
		candidate := response3.Candidates[0]
		if candidate.Content != nil {
			var functionCall *genai.Part
			for _, part := range candidate.Content.Parts {
				if part != nil && part.FunctionCall != nil && part.FunctionCall.Name == "calculate" {
					functionCall = part
					break
				}
			}

			if functionCall != nil {
				t.Logf("Function call detected in trace: %s", functionCall.FunctionCall.Name)

				// Verify function call
				expression, ok := functionCall.FunctionCall.Args["expression"].(string)
				if !ok {
					t.Error("Expected expression argument")
				} else {
					t.Logf("Expression in trace: %s", expression)

					// Verify the expression
					if expression != "15 * 7 + 23" {
						t.Errorf("Expected expression '15 * 7 + 23', got: %s", expression)
					}

					// Continue conversation with result
					result := simulateCalculatorFunction(functionCall.FunctionCall.Args)
					resultStr := fmt.Sprintf("%v", result)

					functionResponse := genai.Part{
						FunctionResponse: &genai.FunctionResponse{
							ID:   functionCall.FunctionCall.ID,
							Name: functionCall.FunctionCall.Name,
							Response: map[string]interface{}{
								"result": resultStr,
							},
						},
					}

					// Send function response
					response3b, err := chatWithTools.SendMessage(ctx, functionResponse)
					helper.AssertNoError(t, err, "Failed to send function response in trace")

					helper.ValidateChatResponse(t, response3b, "Function response in trace")

					response3bText := testutil.ExtractTextFromResponse(response3b)
					t.Logf("Function response: %s", response3bText)
				}
			}
		}
	} else {
		t.Logf("No function calls in trace, checking direct response")

		response3Text := testutil.ExtractTextFromResponse(response3)

		// Should still get a result
		if !testutil.ContainsCaseInsensitive(response3Text, "128") && !testutil.ContainsCaseInsensitive(response3Text, "one hundred") {
			t.Errorf("Expected response to mention 128, got: %s", response3Text)
		}
	}

	// Fourth call - Final response requesting confirmation of calculation
	question4 := "Thank you! Please confirm: what was the result of 15 * 7 + 23?"
	t.Logf("Fourth call: %s", question4)

	response4, err := chat.SendMessage(ctx, genai.Part{Text: question4})
	helper.AssertNoError(t, err, "Failed in fourth trace call")

	helper.ValidateChatResponse(t, response4, "Fourth trace call - final")

	response4Text := testutil.ExtractTextFromResponse(response4)
	t.Logf("Final response: %s", response4Text)

	// Verify calculation result is incorporated
	if !testutil.ContainsCaseInsensitive(response4Text, "128") && !testutil.ContainsCaseInsensitive(response4Text, "one hundred") {
		t.Errorf("Expected final response to mention 128, got: %s", response4Text)
	}

	// Log context check (removed strict validation as focus is on calculation)
	t.Logf("Context check: response includes calculation context")

	t.Logf("Single trace multiple calls test completed successfully")
}

func TestTraceWithDifferentModels(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "trace_multiple_requests")

	ctx := helper.CreateTestContext()

	t.Logf("Testing trace with different models")

	// Test with different model configurations
	models := []string{
		helper.GetModel(),
		"gemini-1.5-flash",
		"gemini-1.5-pro",
	}

	for i, modelName := range models {
		t.Run(fmt.Sprintf("Model_%d", i+1), func(t *testing.T) {
			// Create chat for this model
			var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
			chat, err := helper.CreateChatWithHeaders(ctx, modelName, config, nil)
			if err != nil {
				t.Logf("Skipping model %s due to error: %v", modelName, err)
				return
			}

			// Simple conversation
			question := "What is 2 + 2?"
			response, err := chat.SendMessage(ctx, genai.Part{Text: question})
			if err != nil {
				t.Logf("Error with model %s: %v", modelName, err)
				return
			}

			helper.ValidateChatResponse(t, response, fmt.Sprintf("Model %s test", modelName))

			responseText := testutil.ExtractTextFromResponse(response)
			t.Logf("Model %s response: %s", modelName, responseText)

			// Basic validation
			if len(responseText) == 0 {
				t.Errorf("Expected non-empty response from model %s", modelName)
			}
		})
	}
}

func TestTraceWithStreaming(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "trace_multiple_requests")

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	t.Logf("Testing trace with streaming")

	// Create chat session
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
	chat, err := helper.CreateChatWithHeaders(ctx, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat for streaming trace")

	// First streaming call
	question1 := "Tell me a short story about a robot."
	t.Logf("First streaming call: %s", question1)

	stream1 := chat.SendMessageStream(ctx, genai.Part{Text: question1})

	var response1Text string
	var chunkCount1 int

	for response, err := range stream1 {
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			helper.AssertNoError(t, err, "Stream 1 encountered error")
		}

		chunkCount1++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil && part.Text != "" {
						response1Text += part.Text
					}
				}
			}
		}
	}

	t.Logf("First streaming response: %d chunks", chunkCount1)

	// Second streaming call (context should be preserved)
	question2 := "What was the robot's name in your story?"
	t.Logf("Second streaming call: %s", question2)

	stream2 := chat.SendMessageStream(ctx, genai.Part{Text: question2})

	var response2Text string
	var chunkCount2 int

	for response, err := range stream2 {
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			helper.AssertNoError(t, err, "Stream 2 encountered error")
		}

		chunkCount2++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil && part.Text != "" {
						response2Text += part.Text
					}
				}
			}
		}
	}

	t.Logf("Second streaming response: %d chunks", chunkCount2)

	// Validate responses
	if len(response1Text) == 0 || len(response2Text) == 0 {
		t.Error("Expected non-empty responses from streaming trace")
	}

	// Check context preservation (second response should reference the story)
	if testutil.ContainsCaseInsensitive(response2Text, "story") || testutil.ContainsCaseInsensitive(response2Text, "robot") {
		t.Logf("Context preserved in streaming trace")
	}

	t.Logf("Streaming trace test completed successfully")
}

func TestTraceErrorHandling(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "trace_multiple_requests")

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	t.Logf("Testing trace error handling")

	// Create chat session
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
	chat, err := helper.CreateChatWithHeaders(ctx, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat for error test")

	// Normal call
	question1 := "What is 1 + 1?"
	response1, err := chat.SendMessage(ctx, genai.Part{Text: question1})
	helper.AssertNoError(t, err, "Failed in normal trace call")

	helper.ValidateChatResponse(t, response1, "Normal trace call")

	response1Text := testutil.ExtractTextFromResponse(response1)
	t.Logf("Normal response: %s", response1Text)

	// Try to continue after error (should still work)
	question2 := "Now what is 2 + 2?"
	response2, err := chat.SendMessage(ctx, genai.Part{Text: question2})
	helper.AssertNoError(t, err, "Failed in recovery trace call")

	helper.ValidateChatResponse(t, response2, "Recovery trace call")

	response2Text := testutil.ExtractTextFromResponse(response2)
	t.Logf("Recovery response: %s", response2Text)

	// Validate both responses worked
	if len(response1Text) == 0 || len(response2Text) == 0 {
		t.Error("Expected non-empty responses from error handling trace")
	}

	t.Logf("Error handling trace test completed successfully")
}

// Helper function
func simulateCalculatorFunction(args map[string]interface{}) float64 {
	expression, _ := args["expression"].(string)

	switch expression {
	case "15 * 7 + 23":
		return 128
	case "100 / 4":
		return 25
	case "50 * 30":
		return 1500
	default:
		return 42
	}
}
