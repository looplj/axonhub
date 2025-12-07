package main

import (
	"os"
	"strings"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

func TestMain(m *testing.M) {
	// Set up any global test configuration here if needed
	code := m.Run()
	os.Exit(code)
}

func TestResponsesStreaming(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Question for streaming
	question := "Tell me a short story about a robot learning to paint."

	t.Logf("Sending streaming request: %s", question)

	// Prepare streaming request using Responses API
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
	}

	// Make streaming API call
	stream := helper.Client.Responses.NewStreaming(ctx, params)
	helper.AssertNoError(t, stream.Err(), "Failed to start Responses streaming")

	// Read and process the stream
	var fullContent strings.Builder
	var chunks int

	for stream.Next() {
		event := stream.Current()
		chunks++

		// Handle text delta events
		if event.Type == "response.output_text.delta" && event.Delta != "" {
			fullContent.WriteString(event.Delta)
		}
	}

	// Check for stream errors
	if err := stream.Err(); err != nil {
		helper.AssertNoError(t, err, "Stream error occurred")
	}

	// Validate streaming response
	finalContent := fullContent.String()
	t.Logf("Received %d events", chunks)
	t.Logf("Final content length: %d characters", len(finalContent))

	// Basic validation
	if chunks == 0 {
		t.Error("Expected at least one event from Responses streaming")
	}

	if len(finalContent) == 0 {
		t.Error("Expected non-empty content from Responses streaming response")
	}

	// Verify content makes sense
	if !testutil.ContainsCaseInsensitive(finalContent, "robot") && !testutil.ContainsCaseInsensitive(finalContent, "paint") {
		t.Errorf("Expected content to mention robot or paint, got: %s", finalContent)
	}

	t.Logf("Streamed content preview: %s...", finalContent[:min(200, len(finalContent))])
}

func TestResponsesStreamingWithTools(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Question that encourages conversational response before tool usage
	question := `Hello! I'm working on a math and geography project. Could you help me figure out what 25 multiplied by 4 equals? I'm also curious about the current weather conditions in Tokyo for my research. 

Please first introduce yourself briefly and explain how you'll approach helping me with these questions, then use the available tools to get the precise answers I need.`

	t.Logf("Sending Responses streaming request with tools: %s", question)

	// Define tools for Responses API
	calculatorTool := responses.ToolUnionParam{
		OfFunction: &responses.FunctionToolParam{
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
		},
	}

	weatherTool := responses.ToolUnionParam{
		OfFunction: &responses.FunctionToolParam{
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
		},
	}

	// Prepare streaming request with tools using Responses API
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
		Tools: []responses.ToolUnionParam{calculatorTool, weatherTool},
	}

	// Make streaming API call
	stream := helper.Client.Responses.NewStreaming(ctx, params)
	helper.AssertNoError(t, stream.Err(), "Failed to start Responses streaming with tools")

	// Process the stream
	var fullContent strings.Builder
	var chunksReceived int
	var toolCallEvents []responses.ResponseFunctionCallArgumentsDoneEvent

	for stream.Next() {
		event := stream.Current()
		chunksReceived++

		// Handle text delta events
		if event.Type == "response.output_text.done" {
			fullContent.WriteString(event.AsResponseOutputTextDone().Text)
		}

		// Handle function call events
		if event.Type == "response.function_call_arguments.done" {
			toolCallEvents = append(toolCallEvents, event.AsResponseFunctionCallArgumentsDone())
		}

	}

	// Check for stream errors
	if err := stream.Err(); err != nil {
		helper.AssertNoError(t, err, "Stream error occurred")
	}

	finalContent := fullContent.String()
	t.Logf("Responses streaming with tools: received %d chunks", chunksReceived)
	t.Logf("Final content: %s", finalContent)
	t.Logf("Tool call events: %d", len(toolCallEvents))

	// Validate that we got some response
	if chunksReceived == 0 {
		t.Error("Expected at least one chunk from Responses streaming with tools")
	}

	// If there were tool call events, they should be collected
	if len(toolCallEvents) > 0 {
		t.Logf("Collected %d tool call events from streaming", len(toolCallEvents))

		// Process tool call events (simplified - in real implementation would parse JSON)
		for i, toolEvent := range toolCallEvents {
			t.Logf("Tool call event %d: %s", i+1, toolEvent.Arguments)

			// Simulate tool execution based on content
			if strings.Contains(toolEvent.Arguments, "calculate") {
				result := simulateCalculatorFunctionFromArgs(toolEvent.Arguments)
				t.Logf("Calculator result: %v", result)
			}
			if strings.Contains(toolEvent.Arguments, "get_current_weather") {
				result := simulateWeatherFunctionFromArgs(toolEvent.Arguments)
				t.Logf("Weather result: %s", result)
			}
		}
	}
}

func TestResponsesStreamingLongResponse(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Request for a longer response
	question := "Write a detailed explanation of how photosynthesis works, including the light-dependent and light-independent reactions."

	t.Logf("Sending Responses request for long streaming response")

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
		MaxOutputTokens: openai.Int(1000),  // Allow longer response
		Temperature:     openai.Float(0.7), // More creative
	}

	stream := helper.Client.Responses.NewStreaming(ctx, params)
	helper.AssertNoError(t, stream.Err(), "Failed to start long Responses streaming response")

	// Collect streaming data
	var fullContent strings.Builder
	var chunks []string
	var totalTokens int

	for stream.Next() {
		event := stream.Current()

		// Handle text delta events
		if event.Type == "response.output_text.delta" && event.Delta != "" {
			fullContent.WriteString(event.Delta)
			chunks = append(chunks, event.Delta)
			totalTokens++
		}
	}

	if err := stream.Err(); err != nil {
		helper.AssertNoError(t, err, "Stream error in long Responses response")
	}

	finalContent := fullContent.String()
	t.Logf("Long Responses response: %d chunks, %d tokens, %d characters",
		len(chunks), totalTokens, len(finalContent))

	// Validate long response
	if len(chunks) < 5 {
		t.Errorf("Expected more chunks for long Responses response, got: %d", len(chunks))
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

func TestResponsesStreamingErrorHandling(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Test with invalid parameters that might cause streaming issues
	question := "Test question"

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
		MaxOutputTokens: openai.Int(-1), // Invalid negative value
	}

	// This should fail during request creation or streaming
	stream := helper.Client.Responses.NewStreaming(ctx, params)
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

// Helper functions for Responses API tool simulation

func simulateCalculatorFunctionFromArgs(argsJSON string) float64 {
	// Simple mock calculation for Responses API - in real implementation, this would parse JSON properly
	if strings.Contains(argsJSON, "25") && strings.Contains(argsJSON, "4") {
		return 100
	}
	if strings.Contains(argsJSON, "10") && strings.Contains(argsJSON, "5") {
		return 15
	}
	return 42
}

func simulateWeatherFunctionFromArgs(argsJSON string) string {
	// Simple mock weather for Responses API - in real implementation, this would parse JSON properly
	if strings.Contains(argsJSON, "Tokyo") {
		return "Current weather in Tokyo: 25°C, Sunny, humidity 60%"
	}
	if strings.Contains(argsJSON, "London") {
		return "Current weather in London: 18°C, Rainy, humidity 80%"
	}
	return "Current weather: 20°C, Sunny, humidity 50%"
}
