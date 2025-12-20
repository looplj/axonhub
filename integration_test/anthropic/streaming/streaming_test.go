package main

import (
	"os"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/looplj/axonhub/anthropic_test/internal/testutil"
)

func TestMain(m *testing.M) {
	// Set up any global test configuration here if needed
	code := m.Run()
	os.Exit(code)
}

func TestBasicStreamingChatCompletion(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "basic_streaming")

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Question for testing
	question := "Tell me a short story about a robot learning to paint."

	t.Logf("Sending request: %s", question)

	// Prepare request
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 1024,
	}

	// Make streaming API call
	stream := helper.CreateMessageStreamWithHeaders(ctx, params)
	defer stream.Close()

	// Accumulate the streaming response
	var message anthropic.Message
	var fullResponse string

	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		helper.AssertNoError(t, err, "Failed to accumulate streaming event")

		// Handle different event types for real-time feedback
		switch event := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			if textDelta, ok := event.Delta.AsAny().(anthropic.TextDelta); ok {
				fullResponse += textDelta.Text
			}
		case anthropic.MessageDeltaEvent:
			if event.Delta.StopReason != "" {
				t.Logf("Stream completed with stop reason: %s", event.Delta.StopReason)
			}
		case anthropic.MessageStartEvent:
			t.Logf("Stream started with message ID: %s", message.ID)
		}
	}

	helper.AssertNoError(t, stream.Err(), "Stream encountered an error")

	// Validate the accumulated response
	helper.ValidateMessageResponse(t, &message, "Basic streaming chat completion")

	// Validate response content
	responseText := ""
	for _, block := range message.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			responseText += textBlock.Text
		}
	}

	t.Logf("Streamed response: %s", responseText)

	// Basic validation
	if len(responseText) == 0 {
		t.Error("Expected non-empty response")
	}

	// Verify content makes sense
	if !testutil.ContainsCaseInsensitive(responseText, "robot") && !testutil.ContainsCaseInsensitive(responseText, "paint") {
		t.Errorf("Expected content to mention robot or paint, got: %s", responseText)
	}

	t.Logf("Content preview: %s...", responseText[:min(200, len(responseText))])
}

func TestLongResponseStreaming(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "basic_streaming")

	ctx := helper.CreateTestContext()

	// Request for a longer response
	question := "Write a detailed explanation of how photosynthesis works, including the light-dependent and light-independent reactions."

	t.Logf("Sending request for long response")

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(question))},
		MaxTokens: 1024,
	}

	// Make streaming API call
	stream := helper.CreateMessageStreamWithHeaders(ctx, params)
	defer stream.Close()

	// Accumulate the streaming response
	var message anthropic.Message
	var fullResponse string

	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		helper.AssertNoError(t, err, "Failed to accumulate streaming event")

		// Handle text deltas for real-time feedback
		if event, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if textDelta, ok := event.Delta.AsAny().(anthropic.TextDelta); ok {
				fullResponse += textDelta.Text
			}
		}
	}

	helper.AssertNoError(t, stream.Err(), "Stream encountered an error")

	helper.ValidateMessageResponse(t, &message, "Long response streaming test")

	responseText := ""
	for _, block := range message.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			responseText += textBlock.Text
		}
	}

	t.Logf("Long streamed response: %d characters", len(responseText))

	// Validate long response
	if len(responseText) < 100 {
		t.Errorf("Expected longer content, got: %d characters", len(responseText))
	}

	// Check for key terms in photosynthesis explanation
	expectedTerms := []string{"photosynthesis", "light", "chlorophyll", "carbon dioxide", "oxygen"}
	foundTerms := 0
	for _, term := range expectedTerms {
		if testutil.ContainsCaseInsensitive(responseText, term) {
			foundTerms++
		}
	}

	if foundTerms < 2 {
		t.Errorf("Expected explanation to contain key terms, found %d/%d", foundTerms, len(expectedTerms))
	}

	t.Logf("Content preview: %s...", responseText[:min(300, len(responseText))])
}

func TestStreamingResponseWithTools(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "basic_streaming")

	ctx := helper.CreateTestContext()

	// Question that might require tools
	question := "What is 25 * 4 and what is the weather in Tokyo?"

	t.Logf("Sending request with tools: %s", question)

	// Define tools
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

	tools := []anthropic.ToolUnionParam{
		{OfTool: &calculatorTool},
		{OfTool: &weatherTool},
	}

	// Prepare request with tools
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 1024,
	}

	// Make streaming API call
	stream := helper.CreateMessageStreamWithHeaders(ctx, params)
	defer stream.Close()

	// Accumulate the streaming response
	var message anthropic.Message

	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		helper.AssertNoError(t, err, "Failed to accumulate streaming event")
	}

	helper.AssertNoError(t, stream.Err(), "Stream encountered an error")

	helper.ValidateMessageResponse(t, &message, "Streaming response with tools")

	// Check if tool calls were made
	if message.StopReason == anthropic.StopReasonToolUse {
		t.Logf("Tool calls detected in streaming: %d", len(message.Content))

		// Process tool calls
		for _, block := range message.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				t.Logf("Tool call: %s", toolUseBlock.Name)

				// Continue conversation with tool result
				var toolResult string
				switch toolUseBlock.Name {
				case "calculate":
					toolResult = "100"
				case "get_current_weather":
					toolResult = "Current weather in Tokyo: 25°C, Sunny, humidity 60%"
				default:
					toolResult = "Unknown function"
				}

				messages = append(messages, message.ToParam())
				resultBlock := anthropic.NewToolResultBlock(toolUseBlock.ID, toolResult, false)
				messages = append(messages, anthropic.NewUserMessage(resultBlock))

				// Get final response with streaming
				finalStream := helper.CreateMessageStreamWithHeaders(ctx, anthropic.MessageNewParams{
					Model:     helper.GetModel(),
					Messages:  messages,
					MaxTokens: 1024,
				})
				defer finalStream.Close()

				var finalMessage anthropic.Message
				for finalStream.Next() {
					event := finalStream.Current()
					err := finalMessage.Accumulate(event)
					helper.AssertNoError(t, err, "Failed to accumulate final streaming event")
				}

				helper.AssertNoError(t, finalStream.Err(), "Final stream encountered an error")

				finalText := ""
				for _, block := range finalMessage.Content {
					if textBlock := block.AsText(); textBlock.Text != "" {
						finalText += textBlock.Text
					}
				}

				t.Logf("Final streamed response with tools: %s", finalText)

				// Verify final response
				if len(finalText) == 0 {
					t.Error("Expected non-empty final response")
				}
			}
		}
	} else {
		t.Logf("No tool calls detected in streaming")

		// Check direct response
		responseText := ""
		for _, block := range message.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				responseText += textBlock.Text
			}
		}

		t.Logf("Direct streamed response: %s", responseText)
		if len(responseText) == 0 {
			t.Error("Expected non-empty response")
		}
	}
}

func TestStreamingErrorHandling(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "basic_streaming")

	ctx := helper.CreateTestContext()

	// Test with invalid parameters
	question := "Test question"

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(question))},
		MaxTokens: -1, // Invalid negative value
	}

	// This should fail with streaming
	stream := helper.CreateMessageStreamWithHeaders(ctx, params)
	defer stream.Close()

	// Try to consume the stream
	for stream.Next() {
		// Just consume events until error
	}

	// Check if there's an error
	err := stream.Err()
	if err == nil {
		t.Log("Streaming request accepted despite invalid parameters")
	} else {
		t.Logf("Correctly caught streaming error: %v", err)
	}
}

func TestStreamingEventHandling(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "basic_streaming")

	ctx := helper.CreateTestContext()

	// Question for testing
	question := "Write a step-by-step guide to making coffee."

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(question))},
		MaxTokens: 1024,
	}

	t.Logf("Testing streaming event handling: %s", question)

	// Make streaming API call
	stream := helper.CreateMessageStreamWithHeaders(ctx, params)
	defer stream.Close()

	// Track different event types
	var message anthropic.Message
	var eventCount int
	var textDeltaCount int
	var messageStartCount int
	var messageStopCount int

	for stream.Next() {
		eventCount++
		event := stream.Current()
		err := message.Accumulate(event)
		helper.AssertNoError(t, err, "Failed to accumulate streaming event")

		// Count different event types
		switch event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			textDeltaCount++
		case anthropic.MessageStartEvent:
			messageStartCount++
		case anthropic.MessageStopEvent:
			messageStopCount++
		}
	}

	helper.AssertNoError(t, stream.Err(), "Stream encountered an error")

	// Validate we received various event types
	t.Logf("Total events: %d, Text deltas: %d, Message starts: %d, Message stops: %d",
		eventCount, textDeltaCount, messageStartCount, messageStopCount)

	if eventCount == 0 {
		t.Error("Expected to receive streaming events")
	}

	if textDeltaCount == 0 {
		t.Error("Expected to receive text delta events")
	}

	if messageStartCount == 0 {
		t.Error("Expected to receive message start event")
	}

	if messageStopCount == 0 {
		t.Error("Expected to receive message stop event")
	}

	// Validate final message
	helper.ValidateMessageResponse(t, &message, "Streaming event handling test")

	finalText := ""
	for _, block := range message.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			finalText += textBlock.Text
		}
	}

	if len(finalText) == 0 {
		t.Error("Expected non-empty final response")
	}

	t.Logf("Final streaming response: %s", finalText)
}

func TestStreamingWithSystemPrompt(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "basic_streaming")

	ctx := helper.CreateTestContext()

	// Question with system prompt
	question := "Explain quantum computing."
	systemPrompt := "You are a quantum physics professor. Explain concepts clearly but use technical terms appropriately."

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(question))},
		MaxTokens: 1024,
		System:    []anthropic.TextBlockParam{{Type: "text", Text: systemPrompt}},
	}

	t.Logf("Testing streaming with system prompt")

	// Make streaming API call
	stream := helper.CreateMessageStreamWithHeaders(ctx, params)
	defer stream.Close()

	// Accumulate the streaming response
	var message anthropic.Message
	var fullResponse string

	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		helper.AssertNoError(t, err, "Failed to accumulate streaming event")

		// Handle text deltas for real-time feedback
		if event, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if textDelta, ok := event.Delta.AsAny().(anthropic.TextDelta); ok {
				fullResponse += textDelta.Text
			}
		}
	}

	helper.AssertNoError(t, stream.Err(), "Stream encountered an error")

	helper.ValidateMessageResponse(t, &message, "Streaming with system prompt test")

	// Validate response content
	responseText := ""
	for _, block := range message.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			responseText += textBlock.Text
		}
	}

	// Check for quantum computing terms
	quantumTerms := []string{"quantum", "superposition", "entanglement", "qubit", "algorithm"}
	foundTerms := 0
	for _, term := range quantumTerms {
		if testutil.ContainsCaseInsensitive(responseText, term) {
			foundTerms++
		}
	}

	if foundTerms < 2 {
		t.Errorf("Expected quantum computing explanation to contain technical terms, found %d/%d", foundTerms, len(quantumTerms))
	}

	t.Logf("System prompt streaming response: %s", responseText)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
