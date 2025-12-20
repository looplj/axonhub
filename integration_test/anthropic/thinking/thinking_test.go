package main

import (
	"os"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/looplj/axonhub/anthropic_test/internal/testutil"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestExtendedThinking(t *testing.T) {
	helper := testutil.NewTestHelper(t, "extended_thinking")

	ctx := helper.CreateTestContext()

	question := "What is 27 * 453? Please show your reasoning step by step."

	t.Logf("Sending request with extended thinking: %s", question)

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 16000,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 10000,
			},
		},
	}

	response, err := helper.CreateMessageWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed to get extended thinking response")

	helper.ValidateMessageResponse(t, response, "Extended thinking test")

	var thinkingContent string
	var textContent string
	hasThinkingBlock := false

	for _, block := range response.Content {
		switch block.Type {
		case "thinking":
			hasThinkingBlock = true
			thinkingBlock := block.AsThinking()
			thinkingContent = thinkingBlock.Thinking
			t.Logf("Thinking block signature: %s...", thinkingBlock.Signature[:min(50, len(thinkingBlock.Signature))])
		case "text":
			textBlock := block.AsText()
			textContent += textBlock.Text
		}
	}

	if !hasThinkingBlock {
		t.Error("Expected response to contain a thinking block")
	}

	if len(thinkingContent) == 0 {
		t.Error("Expected non-empty thinking content")
	}

	if len(textContent) == 0 {
		t.Error("Expected non-empty text content")
	}

	t.Logf("Thinking content preview: %s...", thinkingContent[:min(200, len(thinkingContent))])
	t.Logf("Text response: %s", textContent)

	if !testutil.ContainsAnyCaseInsensitive(textContent, "12231", "12,231") {
		t.Logf("Note: Expected answer 12231 not found in response, but model may have different calculation")
	}
}

func TestExtendedThinkingStreaming(t *testing.T) {
	helper := testutil.NewTestHelper(t, "extended_thinking")

	ctx := helper.CreateTestContext()

	question := "Are there an infinite number of prime numbers such that n mod 4 == 3?"

	t.Logf("Sending streaming request with extended thinking: %s", question)

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 16000,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 10000,
			},
		},
	}

	stream := helper.CreateMessageStreamWithHeaders(ctx, params)
	defer stream.Close()

	var message anthropic.Message
	var thinkingDeltaCount int
	var textDeltaCount int

	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		helper.AssertNoError(t, err, "Failed to accumulate streaming event")

		if event.Type != "content_block_delta" {
			continue
		}

		delta := event.AsContentBlockDelta()

		switch delta.Delta.Type {
		case "thinking_delta":
			thinkingDeltaCount++
			t.Logf("Received thinking delta %s", delta.Delta.AsThinkingDelta().Thinking)
		case "text_delta":
			textDeltaCount++
			t.Logf("Received text delta %s", delta.Delta.AsTextDelta().Text)
		case "signature_delta":
			t.Logf("Received signature delta %s", delta.Delta.AsSignatureDelta().Signature)
		}
	}

	helper.AssertNoError(t, stream.Err(), "Stream encountered an error")

	t.Logf("Thinking deltas: %d, Text deltas: %d", thinkingDeltaCount, textDeltaCount)

	hasThinkingBlock := false
	var thinkingContent string
	var textContent string

	for _, block := range message.Content {
		switch block.Type {
		case "thinking":
			hasThinkingBlock = true
			thinkingContent = block.AsThinking().Thinking
		case "text":
			textContent += block.AsText().Text
		}
	}

	if !hasThinkingBlock {
		t.Error("Expected response to contain a thinking block")
	}

	if thinkingDeltaCount == 0 {
		t.Error("Expected to receive thinking delta events")
	}

	if len(thinkingContent) == 0 {
		t.Error("Expected non-empty thinking content")
	}

	if len(textContent) == 0 {
		t.Error("Expected non-empty text content")
	}

	t.Logf("Thinking content preview: %s...", thinkingContent[:min(200, len(thinkingContent))])
	t.Logf("Text response preview: %s...", textContent[:min(300, len(textContent))])
}

func TestRedactedThinking(t *testing.T) {
	helper := testutil.NewTestHelper(t, "extended_thinking")

	ctx := helper.CreateTestContext()

	magicString := "ANTHROPIC_MAGIC_STRING_TRIGGER_REDACTED_THINKING_46C9A13E193C177646C7398A98432ECCCE4C1253D5E2D82641AC0E52CC2876CB"

	t.Logf("Sending request to trigger redacted thinking")

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(magicString)),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 16000,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 10000,
			},
		},
	}

	response, err := helper.CreateMessageWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed to get redacted thinking response")

	helper.ValidateMessageResponse(t, response, "Redacted thinking test")

	hasRedactedThinking := false
	var redactedData string

	for _, block := range response.Content {
		switch block.Type {
		case "redacted_thinking":
			hasRedactedThinking = true
			redactedBlock := block.AsRedactedThinking()
			redactedData = redactedBlock.Data
			t.Logf("Found redacted thinking block with data length: %d", len(redactedData))
		case "thinking":
			thinkingBlock := block.AsThinking()
			t.Logf("Found thinking block with content length: %d", len(thinkingBlock.Thinking))
		case "text":
			textBlock := block.AsText()
			t.Logf("Text response: %s", textBlock.Text)
		}
	}

	if !hasRedactedThinking {
		t.Log("Note: No redacted_thinking block found. This may be expected if the model/provider doesn't support this feature.")
	} else {
		if len(redactedData) == 0 {
			t.Error("Expected non-empty redacted thinking data")
		}
		t.Logf("Redacted thinking data preview: %s...", redactedData[:min(100, len(redactedData))])
	}
}

func TestRedactedThinkingStreaming(t *testing.T) {
	helper := testutil.NewTestHelper(t, "extended_thinking")

	ctx := helper.CreateTestContext()

	magicString := "ANTHROPIC_MAGIC_STRING_TRIGGER_REDACTED_THINKING_46C9A13E193C177646C7398A98432ECCCE4C1253D5E2D82641AC0E52CC2876CB"

	t.Logf("Sending streaming request to trigger redacted thinking")

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(magicString)),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 16000,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 10000,
			},
		},
	}

	stream := helper.CreateMessageStreamWithHeaders(ctx, params)
	defer stream.Close()

	var message anthropic.Message

	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		helper.AssertNoError(t, err, "Failed to accumulate streaming event")

		switch e := event.AsAny().(type) {
		case anthropic.ContentBlockStartEvent:
			t.Logf("Content block started: type=%s", e.ContentBlock.Type)
		}
	}

	helper.AssertNoError(t, stream.Err(), "Stream encountered an error")

	hasRedactedThinking := false

	for _, block := range message.Content {
		switch block.Type {
		case "redacted_thinking":
			hasRedactedThinking = true
			redactedBlock := block.AsRedactedThinking()
			t.Logf("Found redacted thinking block with data length: %d", len(redactedBlock.Data))
		case "thinking":
			thinkingBlock := block.AsThinking()
			t.Logf("Found thinking block with content length: %d", len(thinkingBlock.Thinking))
		case "text":
			textBlock := block.AsText()
			t.Logf("Text response: %s", textBlock.Text)
		}
	}

	if !hasRedactedThinking {
		t.Log("Note: No redacted_thinking block found in streaming. This may be expected if the model/provider doesn't support this feature.")
	}
}

func TestExtendedThinkingWithToolUse(t *testing.T) {
	helper := testutil.NewTestHelper(t, "extended_thinking")

	ctx := helper.CreateTestContext()

	question := "What's the weather in Paris? Please think through your approach."

	t.Logf("Sending request with extended thinking and tools: %s", question)

	weatherTool := anthropic.ToolParam{
		Name:        "get_weather",
		Description: anthropic.String("Get the current weather for a specified location"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"location": map[string]string{
					"type":        "string",
					"description": "The city and country, e.g. Paris, France",
				},
			},
			Required: []string{"location"},
		},
	}

	tools := []anthropic.ToolUnionParam{
		{OfTool: &weatherTool},
	}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 16000,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 10000,
			},
		},
	}

	response, err := helper.CreateMessageWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed to get extended thinking with tools response")

	helper.ValidateMessageResponse(t, response, "Extended thinking with tools test")

	var hasThinkingBlock bool
	var hasToolUse bool

	for _, block := range response.Content {
		switch block.Type {
		case "thinking":
			hasThinkingBlock = true
			thinkingBlock := block.AsThinking()
			t.Logf("Thinking content preview: %s...", thinkingBlock.Thinking[:min(200, len(thinkingBlock.Thinking))])
		case "tool_use":
			hasToolUse = true
			toolUseBlock := block.AsToolUse()
			t.Logf("Tool use: %s with ID: %s", toolUseBlock.Name, toolUseBlock.ID)
		case "text":
			textBlock := block.AsText()
			t.Logf("Text response: %s", textBlock.Text)
		}
	}

	if !hasThinkingBlock {
		t.Error("Expected response to contain a thinking block")
	}

	if response.StopReason == anthropic.StopReasonToolUse {
		if !hasToolUse {
			t.Error("Stop reason is tool_use but no tool_use block found")
		}
		t.Log("Model requested tool use as expected")
	} else {
		t.Logf("Model did not request tool use, stop reason: %s", response.StopReason)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
