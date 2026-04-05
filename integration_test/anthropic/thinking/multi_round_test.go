package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/looplj/axonhub/anthropic_test/internal/testutil"
)

func TestExtendedThinkingMultipleToolsMultiRound(t *testing.T) {
	helper := testutil.NewTestHelper(t, "extended_thinking_multi_round")
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

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

	question := "Use the available tools to: 1) Get the current weather in London, and 2) Calculate 15 * 23. Return only tool calls."

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 2048,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: "medium",
		},
	}

	response, err := helper.CreateMessageWithHeaders(ctx, params)
	if err != nil {
		if isThinkingNotSupportedErr(err) {
			t.Skipf("Skipping because model does not support thinking: %v", err)
		}
		helper.AssertNoError(t, err, "Failed to get extended thinking response with multiple tools")
	}

	helper.ValidateMessageResponse(t, response, "Extended thinking multiple tools - round1")

	thinkingSignature := ""
	toolUseBlocks := make([]anthropic.ToolUseBlock, 0)
	for _, block := range response.Content {
		switch block.Type {
		case "thinking":
			tb := block.AsThinking()
			if tb.Signature != "" && thinkingSignature == "" {
				thinkingSignature = tb.Signature
			}
		case "tool_use":
			tb := block.AsToolUse()
			if tb.Name != "" {
				toolUseBlocks = append(toolUseBlocks, tb)
			}
		}
	}

	if thinkingSignature == "" {
		t.Fatalf("Expected thinking signature in response content blocks")
	}
	if response.StopReason != anthropic.StopReasonToolUse {
		t.Fatalf("Expected stop_reason=tool_use, got %s", response.StopReason)
	}
	if len(toolUseBlocks) < 2 {
		t.Fatalf("Expected at least 2 tool_use blocks, got %d", len(toolUseBlocks))
	}

	assistantParam := response.ToParam()
	assistantParamJSON, err := json.Marshal(assistantParam)
	helper.AssertNoError(t, err, "Failed to marshal assistant param")
	if !bytes.Contains(assistantParamJSON, []byte(`"signature"`)) {
		t.Fatalf("Expected assistant message param to include thinking signature for multi-round continuation")
	}

	messages = append(messages, assistantParam)

	for _, tu := range toolUseBlocks {
		var args map[string]any
		err := json.Unmarshal([]byte(tu.Input), &args)
		helper.AssertNoError(t, err, "Failed to parse tool_use input JSON")

		result := "Unknown function"
		switch tu.Name {
		case "get_current_weather":
			result = simulateWeatherFunction(args)
		case "calculate":
			result = fmt.Sprintf("%v", simulateCalculatorFunction(args))
		}

		callID := tu.ID
		if callID == "" {
			callID = "call_fallback"
		}
		messages = append(messages, anthropic.NewUserMessage(anthropic.NewToolResultBlock(callID, result, false)))
	}

	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Now respond with a final answer that includes both the weather result and the calculation.")))

	params.Messages = messages
	params.Tools = nil

	finalResponse, err := helper.CreateMessageWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed to get final completion for multi-round thinking")

	helper.ValidateMessageResponse(t, finalResponse, "Extended thinking multiple tools - round2")

	finalText := ""
	for _, block := range finalResponse.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			finalText += textBlock.Text
		}
	}

	if finalText == "" {
		t.Fatalf("Expected non-empty final response text")
	}

	if !testutil.ContainsAnyCaseInsensitive(finalText, "london") {
		t.Fatalf("Expected final answer to mention London, got: %s", finalText)
	}
	if !testutil.ContainsAnyCaseInsensitive(finalText, "345") {
		t.Fatalf("Expected final answer to mention 345, got: %s", finalText)
	}
}

func simulateWeatherFunction(args map[string]any) string {
	location, _ := args["location"].(string)

	weatherData := map[string]map[string]string{
		"new york": {"temp": "22", "condition": "Partly cloudy", "humidity": "65%"},
		"london":   {"temp": "18", "condition": "Rainy", "humidity": "80%"},
		"tokyo":    {"temp": "25", "condition": "Sunny", "humidity": "60%"},
		"paris":    {"temp": "20", "condition": "Clear", "humidity": "55%"},
	}

	defaultWeather := map[string]string{"temp": "20", "condition": "Sunny", "humidity": "50%"}

	weather := defaultWeather
	if cityWeather, exists := weatherData[normalizeLocation(location)]; exists {
		weather = cityWeather
	}

	return fmt.Sprintf("Current weather in %s: %s°C, %s, humidity %s",
		location, weather["temp"], weather["condition"], weather["humidity"])
}

func simulateCalculatorFunction(args map[string]any) float64 {
	expression, _ := args["expression"].(string)

	switch expression {
	case "15 * 23":
		return 345
	default:
		return 42
	}
}

func normalizeLocation(location string) string {
	return strings.ToLower(location)
}

func isThinkingNotSupportedErr(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "thinking") && !strings.Contains(msg, "thought") {
		return false
	}

	return strings.Contains(msg, "not support") ||
		strings.Contains(msg, "doesn't support") ||
		strings.Contains(msg, "does not support") ||
		strings.Contains(msg, "unsupported")
}
