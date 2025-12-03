package main

import (
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

func TestStreamingChatCompletion(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Question for streaming
	question := "Tell me a short story about a robot learning to paint."

	t.Logf("Sending streaming request: %s", question)

	// Prepare streaming request (no Stream field needed)
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model: helper.GetModel(),
	}

	// Make streaming API call
	stream := helper.Client.Chat.Completions.NewStreaming(ctx, params)
	helper.AssertNoError(t, stream.Err(), "Failed to start streaming chat completion")

	// Read and process the stream
	var fullContent strings.Builder
	var chunks []string
	var totalTokens int

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			if content != "" {
				fullContent.WriteString(content)
				chunks = append(chunks, content)
				totalTokens++
			}
		}
	}

	// Check for stream errors
	if err := stream.Err(); err != nil {
		helper.AssertNoError(t, err, "Stream error occurred")
	}

	// Validate streaming response
	finalContent := fullContent.String()
	t.Logf("Received %d chunks with %d total tokens", len(chunks), totalTokens)
	t.Logf("Final content length: %d characters", len(finalContent))

	// Basic validation
	if len(chunks) == 0 {
		t.Error("Expected at least one content chunk from streaming")
	}

	if len(finalContent) == 0 {
		t.Error("Expected non-empty content from streaming response")
	}

	// Verify content makes sense
	if !testutil.ContainsCaseInsensitive(finalContent, "robot") && !testutil.ContainsCaseInsensitive(finalContent, "paint") {
		t.Errorf("Expected content to mention robot or paint, got: %s", finalContent)
	}

	t.Logf("Streamed content preview: %s...", finalContent[:min(200, len(finalContent))])
}

func TestStreamingWithTools(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Question that encourages conversational response before tool usage
	question := `Hello! I'm working on a math and geography project. Could you help me figure out what 25 multiplied by 4 equals? I'm also curious about the current weather conditions in Tokyo for my research. 

Please first introduce yourself briefly and explain how you'll approach helping me with these questions, then use the available tools to get the precise answers I need.`

	t.Logf("Sending streaming request with tools: %s", question)

	// Define tools
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

	calculatorTool := openai.ChatCompletionFunctionTool(calculatorFunction)
	weatherTool := openai.ChatCompletionFunctionTool(weatherFunction)

	// Prepare streaming request with tools
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Tools: []openai.ChatCompletionToolUnionParam{calculatorTool, weatherTool},
		Model: helper.GetModel(),
	}

	// Make streaming API call
	stream := helper.Client.Chat.Completions.NewStreaming(ctx, params)
	helper.AssertNoError(t, stream.Err(), "Failed to start streaming with tools")

	// Process the stream
	var fullContent strings.Builder
	var toolCalls []openai.ChatCompletionChunkChoiceDeltaToolCall
	var chunksReceived int

	for stream.Next() {
		chunk := stream.Current()
		chunksReceived++

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]

			// Collect content
			if choice.Delta.Content != "" {
				fullContent.WriteString(choice.Delta.Content)
			}

			// Collect tool calls
			if len(choice.Delta.ToolCalls) > 0 {
				toolCalls = append(toolCalls, choice.Delta.ToolCalls...)
			}
		}
	}

	// Check for stream errors
	if err := stream.Err(); err != nil {
		helper.AssertNoError(t, err, "Stream error occurred")
	}

	finalContent := fullContent.String()
	t.Logf("Streaming with tools: received %d chunks", chunksReceived)
	t.Logf("Final content: %s", finalContent)

	// Validate that we got some response
	if chunksReceived == 0 {
		t.Error("Expected at least one chunk from streaming with tools")
	}

	// If there were tool calls, they should be collected
	if len(toolCalls) > 0 {
		t.Logf("Collected %d tool calls from streaming", len(toolCalls))

		// Process tool calls
		for i, toolCall := range toolCalls {
			t.Logf("Tool call %d: %s", i+1, toolCall.Function.Name)

			// Simulate tool execution based on function name
			switch toolCall.Function.Name {
			case "calculate":
				result := simulateCalculatorFunctionFromArgs(toolCall.Function.Arguments)
				t.Logf("Calculator result: %v", result)
			case "get_current_weather":
				result := simulateWeatherFunctionFromArgs(toolCall.Function.Arguments)
				t.Logf("Weather result: %s", result)
			}
		}
	}
}

func TestStreamingLongResponse(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Request for a longer response
	question := "Write a detailed explanation of how photosynthesis works, including the light-dependent and light-independent reactions."

	t.Logf("Sending request for long streaming response")

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model:               helper.GetModel(),
		MaxCompletionTokens: openai.Int(1000),  // Allow longer response
		Temperature:         openai.Float(0.7), // More creative
	}

	stream := helper.Client.Chat.Completions.NewStreaming(ctx, params)
	helper.AssertNoError(t, stream.Err(), "Failed to start long streaming response")

	// Collect streaming data
	var fullContent strings.Builder
	var chunks []string
	var totalTokens int

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			if content != "" {
				fullContent.WriteString(content)
				chunks = append(chunks, content)
				totalTokens++
			}
		}
	}

	if err := stream.Err(); err != nil {
		helper.AssertNoError(t, err, "Stream error in long response")
	}

	finalContent := fullContent.String()
	t.Logf("Long response: %d chunks, %d tokens, %d characters",
		len(chunks), totalTokens, len(finalContent))

	// Validate long response
	if len(chunks) < 5 {
		t.Errorf("Expected more chunks for long response, got: %d", len(chunks))
	}

	if len(finalContent) < 200 {
		t.Errorf("Expected longer content, got: %d characters", len(finalContent))
	}

	// Check for key terms in photosynthesis explanation
	expectedTerms := []string{"photosynthesis", "light", "chlorophyll", "carbon dioxide", "oxygen"}
	foundTerms := 0
	for _, term := range expectedTerms {
		if testutil.ContainsCaseInsensitive(finalContent, term) {
			foundTerms++
		}
	}

	if foundTerms < 3 {
		t.Errorf("Expected explanation to contain key photosynthesis terms, found %d/%d", foundTerms, len(expectedTerms))
	}

	t.Logf("Content preview: %s...", finalContent[:min(300, len(finalContent))])
}

func TestStreamingErrorHandling(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Test with invalid parameters that might cause streaming issues
	question := "Test question"

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model:               helper.GetModel(),
		MaxCompletionTokens: openai.Int(-1), // Invalid negative value
	}

	// This should fail during request creation or streaming
	stream := helper.Client.Chat.Completions.NewStreaming(ctx, params)
	if err := stream.Err(); err == nil {
		// If no immediate error, try to read from stream
		if stream.Next() {
			// If we get here, the request was accepted despite invalid params
			t.Log("Request accepted despite invalid parameters")
		}
		if err := stream.Err(); err != nil {
			t.Logf("Stream error (expected): %v", err)
		}
	} else {
		t.Logf("Correctly caught error: %v", err)
	}
}

// Helper functions

func simulateCalculatorFunctionFromArgs(argsJSON string) float64 {
	// Simple mock calculation - in real implementation, this would parse JSON properly
	switch argsJSON {
	case `{"expression":"25 * 4"}`:
		return 100
	case `{"expression":"10 + 5"}`:
		return 15
	default:
		return 42
	}
}

func simulateWeatherFunctionFromArgs(argsJSON string) string {
	// Simple mock weather - in real implementation, this would parse JSON properly
	switch argsJSON {
	case `{"location":"Tokyo"}`:
		return "Current weather in Tokyo: 25°C, Sunny, humidity 60%"
	case `{"location":"London"}`:
		return "Current weather in London: 18°C, Rainy, humidity 80%"
	default:
		return "Current weather: 20°C, Sunny, humidity 50%"
	}
}
