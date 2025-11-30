package main

import (
	"io"
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

func TestBasicStreamingChatCompletion(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	// Question for testing
	question := "Tell me a short story about a robot learning to paint."

	t.Logf("Sending streaming request: %s", question)

	// Prepare content for streaming
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	// Make streaming API call
	stream := helper.Client.Models.GenerateContentStream(ctx, modelName, contents, nil)

	// Accumulate the streaming response
	var fullResponse string
	var responseCount int

	for response, err := range stream {
		if err != nil {
			if err == io.EOF {
				break // Normal end of stream
			}
			helper.AssertNoError(t, err, "Stream encountered an error")
		}

		responseCount++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil && part.Text != "" {
						fullResponse += part.Text
						t.Logf("Stream chunk %d: %s", responseCount, part.Text)
					}
				}
			}
		}
	}

	t.Logf("Total streaming responses: %d", responseCount)

	// Validate the accumulated response
	if len(fullResponse) == 0 {
		t.Error("Expected non-empty streaming response")
	}

	// Verify content makes sense
	if !testutil.ContainsCaseInsensitive(fullResponse, "robot") && !testutil.ContainsCaseInsensitive(fullResponse, "paint") {
		t.Errorf("Expected content to mention robot or paint, got: %s", fullResponse)
	}

	t.Logf("Complete streamed response: %s", fullResponse)
	t.Logf("Content preview: %s...", fullResponse[:min(200, len(fullResponse))])
}

func TestLongResponseStreaming(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	// Request for a longer response
	question := "Write a detailed explanation of how photosynthesis works, including the light-dependent and light-independent reactions."

	t.Logf("Sending streaming request for long response")

	// Prepare content for streaming
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	// Make streaming API call
	stream := helper.Client.Models.GenerateContentStream(ctx, modelName, contents, nil)

	// Accumulate the streaming response
	var fullResponse string
	var chunkCount int

	for response, err := range stream {
		if err != nil {
			if err == io.EOF {
				break // Normal end of stream
			}
			helper.AssertNoError(t, err, "Long stream encountered an error")
		}

		chunkCount++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil && part.Text != "" {
						fullResponse += part.Text
					}
				}
			}
		}
	}

	t.Logf("Long streamed response: %d characters in %d chunks", len(fullResponse), chunkCount)

	// Validate long response
	if len(fullResponse) < 100 {
		t.Errorf("Expected longer content, got: %d characters", len(fullResponse))
	}

	// Check for key terms in photosynthesis explanation
	expectedTerms := []string{"photosynthesis", "light", "chlorophyll", "carbon dioxide", "oxygen"}
	foundTerms := 0
	for _, term := range expectedTerms {
		if testutil.ContainsCaseInsensitive(fullResponse, term) {
			foundTerms++
		}
	}

	if foundTerms < 2 {
		t.Errorf("Expected explanation to contain key terms, found %d/%d", foundTerms, len(expectedTerms))
	}

	t.Logf("Content preview: %s...", fullResponse[:min(300, len(fullResponse))])
}

func TestStreamingResponseWithTools(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	// Question that might require tools
	question := "What is 25 * 4 and what is the weather in Tokyo?"

	t.Logf("Sending streaming request with tools: %s", question)

	// Define tools for Gemini
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

	tools := []*genai.Tool{calculatorTool, weatherTool}

	// Prepare content for streaming with tools
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	// Make streaming API call with tools
	stream := helper.Client.Models.GenerateContentStream(ctx, modelName, contents, &genai.GenerateContentConfig{
		Tools: tools,
	})

	// Accumulate the streaming response
	var fullResponse string
	var hasFunctionCalls bool
	var chunkCount int

	for response, err := range stream {
		if err != nil {
			if err == io.EOF {
				break // Normal end of stream
			}
			helper.AssertNoError(t, err, "Tool stream encountered an error")
		}

		chunkCount++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil {
						if part.Text != "" {
							fullResponse += part.Text
						} else if part.FunctionCall != nil {
							hasFunctionCalls = true
							t.Logf("Function call detected in stream: %s", part.FunctionCall.Name)
						}
					}
				}
			}
		}
	}

	t.Logf("Tool streaming completed: %d chunks, has function calls: %v", chunkCount, hasFunctionCalls)

	if hasFunctionCalls {
		t.Logf("Function calls were detected in streaming response")
		// Note: In a real implementation, you would handle the function calls here
		// and potentially continue the conversation with function responses
	} else {
		t.Logf("No function calls detected, direct response received")
	}

	// Validate response
	if len(fullResponse) == 0 {
		t.Error("Expected non-empty response from streaming with tools")
	}

	t.Logf("Tool streaming response: %s", fullResponse)
}

func TestStreamingErrorHandling(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Test with invalid model name
	invalidModel := "invalid-model-name"

	// Prepare content for streaming
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: "Test question"}},
	}}

	// This should fail with streaming
	stream := helper.Client.Models.GenerateContentStream(ctx, invalidModel, contents, nil)

	chunkCount := 0
	for _, err := range stream {
		if err != nil {
			if err == io.EOF {
				t.Error("Expected error but stream completed normally")
				break
			}
			t.Logf("Correctly caught streaming error: %v", err)
			return
		}
		chunkCount++
		if chunkCount > 10 {
			// Safety limit to prevent infinite loop
			t.Error("Stream continued too long without error")
			break
		}
	}

	t.Logf("Streaming request completed despite invalid model")
}

func TestStreamingEventHandling(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	// Question for testing
	question := "Write a step-by-step guide to making coffee."

	t.Logf("Testing streaming event handling: %s", question)

	// Prepare content for streaming
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	// Make streaming API call
	stream := helper.Client.Models.GenerateContentStream(ctx, modelName, contents, nil)

	// Track streaming events
	var fullResponse string
	var chunkCount int
	var totalChars int

	for response, err := range stream {
		if err != nil {
			if err == io.EOF {
				break // Normal end of stream
			}
			helper.AssertNoError(t, err, "Event stream encountered an error")
		}

		chunkCount++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil && part.Text != "" {
						chunkText := part.Text
						fullResponse += chunkText
						totalChars += len(chunkText)
						t.Logf("Stream chunk %d: %d chars - %s", chunkCount, len(chunkText), chunkText)
					}
				}
			}
		}
	}

	// Validate we received streaming data
	t.Logf("Total chunks: %d, Total characters: %d", chunkCount, totalChars)

	if chunkCount == 0 {
		t.Error("Expected to receive streaming chunks")
	}

	if totalChars == 0 {
		t.Error("Expected to receive text content")
	}

	if len(fullResponse) == 0 {
		t.Error("Expected non-empty final response")
	}

	t.Logf("Final streaming response: %s", fullResponse)
}

func TestStreamingWithSystemPrompt(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	// Question with system prompt
	question := "Explain quantum computing."
	systemPrompt := "You are a quantum physics professor. Explain concepts clearly but use technical terms appropriately."

	t.Logf("Testing streaming with system prompt")

	// Prepare content for streaming with system prompt
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	// Make streaming API call with system instruction
	stream := helper.Client.Models.GenerateContentStream(ctx, modelName, contents, &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
	})

	// Accumulate the streaming response
	var fullResponse string
	var chunkCount int

	for response, err := range stream {
		if err != nil {
			if err == io.EOF {
				break // Normal end of stream
			}
			helper.AssertNoError(t, err, "System prompt stream encountered an error")
		}

		chunkCount++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil && part.Text != "" {
						fullResponse += part.Text
					}
				}
			}
		}
	}

	t.Logf("System prompt streaming completed: %d chunks", chunkCount)

	// Check for quantum computing terms
	quantumTerms := []string{"quantum", "superposition", "entanglement", "qubit", "algorithm"}
	foundTerms := 0
	for _, term := range quantumTerms {
		if testutil.ContainsCaseInsensitive(fullResponse, term) {
			foundTerms++
		}
	}

	if foundTerms < 2 {
		t.Errorf("Expected quantum computing explanation to contain technical terms, found %d/%d", foundTerms, len(quantumTerms))
	}

	t.Logf("System prompt streaming response: %s", fullResponse)
}

func TestStreamingChatSession(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	t.Logf("Testing streaming with chat session")

	// Create chat session
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
	chat, err := helper.Client.Chats.Create(ctx, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat for streaming")

	// First message with streaming
	question1 := "Tell me about artificial intelligence."
	t.Logf("Question 1: %s", question1)

	stream1 := chat.SendMessageStream(ctx, genai.Part{Text: question1})

	var response1 string
	var chunkCount1 int

	for response, err := range stream1 {
		if err != nil {
			if err == io.EOF {
				break
			}
			helper.AssertNoError(t, err, "Chat stream 1 encountered an error")
		}

		chunkCount1++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil && part.Text != "" {
						response1 += part.Text
					}
				}
			}
		}
	}

	t.Logf("Chat streaming response 1: %d chunks", chunkCount1)

	// Second message with streaming (context should be preserved)
	question2 := "How does machine learning relate to AI?"
	t.Logf("Question 2: %s", question2)

	stream2 := chat.SendMessageStream(ctx, genai.Part{Text: question2})

	var response2 string
	var chunkCount2 int

	for response, err := range stream2 {
		if err != nil {
			if err == io.EOF {
				break
			}
			helper.AssertNoError(t, err, "Chat stream 2 encountered an error")
		}

		chunkCount2++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil && part.Text != "" {
						response2 += part.Text
					}
				}
			}
		}
	}

	t.Logf("Chat streaming response 2: %d chunks", chunkCount2)

	// Validate responses
	if len(response1) == 0 || len(response2) == 0 {
		t.Error("Expected non-empty responses from streaming chat")
	}

	// Check context preservation
	if testutil.ContainsCaseInsensitive(response2, "previous") || testutil.ContainsCaseInsensitive(response2, "earlier") {
		t.Logf("Context preserved in second response")
	}

	t.Logf("Chat streaming completed successfully")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
