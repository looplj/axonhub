package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/looplj/axonhub/anthropic_test/internal/testutil"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestThinkingSignatureWithMultiRoundToolsStreamingAndMultiModels(t *testing.T) {
	helper := testutil.NewTestHelper(t, "thinking_signature")
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

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

	type round struct {
		name          string
		model         anthropic.Model
		userPrompt    string
		needsToolCall bool
		forbidTools   bool
	}

	rounds := []round{
		{
			name:          "round1-deepseek-chat",
			model:         anthropic.Model("deepseek-chat"),
			userPrompt:    "Use the calculate tool to compute `15 * 23`. Return only the tool call.",
			needsToolCall: true,
		},
		{
			name:          "round2-gemini-3-flash",
			model:         anthropic.Model("gemini-3-flash-preview"),
			userPrompt:    "Use the get_current_weather tool for `London`. Return only the tool call.",
			needsToolCall: true,
		},
		{
			name:          "round3-claude-sonnet",
			model:         anthropic.Model("claude-sonnet-4-5"),
			userPrompt:    "Use the calculate tool to compute `50 * 30`. Return only the tool call.",
			needsToolCall: true,
		},
		{
			name:          "round4-gpt-5.2",
			model:         anthropic.Model("gpt-5.2"),
			userPrompt:    "Do NOT call any tools. Summarize the previous tool results and include: 345, London, and 1500.",
			needsToolCall: false,
			forbidTools:   true,
		},
	}

	var messages []anthropic.MessageParam

	streamCalls := 0
	seenAnySignature := false
	seenAnyToolCall := false

	for i, r := range rounds {
		t.Logf("=== %s (model=%s) ===", r.name, r.model)

		callTools := tools
		if r.forbidTools {
			callTools = nil
		}

		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(r.userPrompt)))

		msg, toolUses, hasSignature := streamMessageWithThinking(t, helper, ctx, r.model, messages, callTools)
		streamCalls++

		if r.forbidTools && len(toolUses) > 0 {
			t.Fatalf("Expected no tool calls in %s, got %d", r.name, len(toolUses))
		}
		if r.needsToolCall && len(toolUses) == 0 {
			t.Fatalf("Expected at least one tool call in %s, got none.", r.name)
		}

		if r.needsToolCall {
			validateToolUsesHaveArgs(t, r.name, toolUses)
			switch r.name {
			case "round1-deepseek-chat":
				requireToolUseArgEquals(t, r.name, toolUses, "calculate", "expression", "15 * 23")
			case "round2-gemini-3-flash":
				requireToolUseArgEquals(t, r.name, toolUses, "get_current_weather", "location", "London")
			case "round3-claude-sonnet":
				requireToolUseArgEquals(t, r.name, toolUses, "calculate", "expression", "50 * 30")
			}
		}

		if hasSignature {
			seenAnySignature = true
		}
		if len(toolUses) > 0 {
			seenAnyToolCall = true
		}

		messages = append(messages, msg.ToParam())

		for callIdx, tu := range toolUses {
			callID := tu.ID
			if callID == "" {
				callID = fmt.Sprintf("call_%d_%d", i+1, callIdx+1)
			}

			result := "Unknown function"
			switch tu.Name {
			case "calculate":
				result = fmt.Sprintf("%v", simulateCalculatorFunction(tu.Args))
			case "get_current_weather":
				result = simulateWeatherFunction(tu.Args)
			}

			messages = append(messages, anthropic.NewUserMessage(anthropic.NewToolResultBlock(callID, result, false)))
		}
	}

	if streamCalls != 4 {
		t.Fatalf("Expected exactly 4 streaming calls, got %d", streamCalls)
	}
	if !seenAnySignature {
		t.Fatalf("Expected at least one thinking signature across streamed responses, got none")
	}
	if !seenAnyToolCall {
		t.Fatalf("Expected at least one tool call across the four rounds, got none")
	}
}

type toolUse struct {
	ID   string
	Name string
	Args map[string]any
}

func streamMessageWithThinking(
	t *testing.T,
	helper *testutil.TestHelper,
	ctx context.Context,
	model anthropic.Model,
	messages []anthropic.MessageParam,
	tools []anthropic.ToolUnionParam,
) (anthropic.Message, []toolUse, bool) {
	t.Helper()

	params := anthropic.MessageNewParams{
		Model:     model,
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 16000,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 10000,
			},
		},
	}

	stream := helper.CreateMessageStreamWithHeaders(ctx, params)
	defer stream.Close()

	var (
		msg                     anthropic.Message
		toolUses                []toolUse
		sawAnyEvent             bool
		sawAnySignatureDelta    bool
		sawAnyThinkingSignature bool
	)

	for stream.Next() {
		sawAnyEvent = true
		event := stream.Current()

		err := msg.Accumulate(event)
		helper.AssertNoError(t, err, "Failed to accumulate streaming event")

		if event.Type == "content_block_delta" {
			delta := event.AsContentBlockDelta()
			if delta.Delta.Type == "signature_delta" {
				sig := delta.Delta.AsSignatureDelta().Signature
				if sig != "" {
					sawAnySignatureDelta = true
				}
			}
		}
	}

	helper.AssertNoError(t, stream.Err(), "Stream encountered an error")

	if !sawAnyEvent {
		t.Fatalf("No streamed responses received")
	}

	for _, block := range msg.Content {
		switch block.Type {
		case "thinking":
			tb := block.AsThinking()
			if tb.Signature != "" {
				sawAnyThinkingSignature = true
			}
		case "tool_use":
			tb := block.AsToolUse()
			if tb.Name == "" {
				continue
			}

			args, err := parseToolArgs(tb.Input)
			helper.AssertNoError(t, err, "Failed to parse tool_use input JSON")

			toolUses = append(toolUses, toolUse{
				ID:   tb.ID,
				Name: tb.Name,
				Args: args,
			})
		}
	}

	return msg, toolUses, sawAnySignatureDelta || sawAnyThinkingSignature
}

func validateToolUsesHaveArgs(t *testing.T, roundName string, toolUses []toolUse) {
	t.Helper()
	for idx, tu := range toolUses {
		if tu.Name == "" {
			t.Fatalf("%s: toolUses[%d] missing function name", roundName, idx)
		}
		if tu.Args == nil {
			t.Fatalf("%s: toolUses[%d] (%s) missing args map", roundName, idx, tu.Name)
		}
		if len(tu.Args) == 0 {
			t.Fatalf("%s: toolUses[%d] (%s) args map is empty", roundName, idx, tu.Name)
		}
	}
}

func requireToolUseArgEquals(
	t *testing.T,
	roundName string,
	toolUses []toolUse,
	functionName string,
	argKey string,
	want string,
) {
	t.Helper()

	var matched []toolUse
	for _, tu := range toolUses {
		if tu.Name == functionName {
			matched = append(matched, tu)
		}
	}
	if len(matched) == 0 {
		t.Fatalf("%s: expected at least one %q tool call, got none", roundName, functionName)
	}

	gotAny := false
	for _, tu := range matched {
		v, ok := tu.Args[argKey]
		if !ok {
			continue
		}
		got, ok := v.(string)
		if !ok {
			t.Fatalf("%s: %q arg %q is not a string (got %T)", roundName, functionName, argKey, v)
		}
		gotAny = true
		if got != want {
			t.Fatalf("%s: %q arg %q mismatch: want %q got %q", roundName, functionName, argKey, want, got)
		}
	}
	if !gotAny {
		t.Fatalf("%s: expected %q tool call arg %q=%q, but key was missing", roundName, functionName, argKey, want)
	}
}

func parseToolArgs(input json.RawMessage) (map[string]any, error) {
	var args map[string]any
	if len(input) == 0 {
		return nil, fmt.Errorf("empty tool input")
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, err
	}
	return args, nil
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
	case "50 * 30":
		return 1500
	default:
		return 42
	}
}

func normalizeLocation(location string) string {
	return strings.ToLower(location)
}
