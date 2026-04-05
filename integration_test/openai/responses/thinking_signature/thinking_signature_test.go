package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestReasoningEncryptedContentWithMultiRoundToolsStreamingAndMultiModels(t *testing.T) {
	helper := testutil.NewTestHelper(t, "thinking_signature")
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

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

	tools := []responses.ToolUnionParam{calculatorTool, weatherTool}

	type round struct {
		name          string
		model         string
		userPrompt    string
		needsToolCall bool
		forbidTools   bool
	}

	rounds := []round{
		{
			name:          "round1-deepseek-chat",
			model:         "deepseek-chat",
			userPrompt:    "Use the calculate tool to compute `15 * 23`. Return only the tool call.",
			needsToolCall: true,
		},
		{
			name:          "round2-gemini-3-flash",
			model:         "gemini-3-flash-preview",
			userPrompt:    "Use the get_current_weather tool for `London`. Return only the tool call.",
			needsToolCall: true,
		},
		{
			name:          "round3-claude-sonnet",
			model:         "claude-sonnet-4-5",
			userPrompt:    "Use the calculate tool to compute `50 * 30`. Return only the tool call.",
			needsToolCall: true,
		},
		{
			name:          "round4-gpt-5.2",
			model:         "gpt-5.2",
			userPrompt:    "Do NOT call any tools. Summarize the previous tool results and include: 345, London, and 1500.",
			needsToolCall: false,
			forbidTools:   true,
		},
	}

	streamCalls := 0
	seenAnyEncryptedContent := false
	seenAnyToolCall := false

	for _, r := range rounds {
		t.Logf("=== %s (model=%s) ===", r.name, r.model)

		callTools := tools
		if r.forbidTools {
			callTools = nil
		}

		params := responses.ResponseNewParams{
			Model: shared.ResponsesModel(r.model),
			Input: responses.ResponseNewParamsInputUnion{
				OfString: openai.String(r.userPrompt),
			},
			Tools: callTools,
			Include: []responses.ResponseIncludable{
				responses.ResponseIncludableReasoningEncryptedContent,
			},
			Reasoning: shared.ReasoningParam{
				Effort: shared.ReasoningEffortHigh,
			},
			Store: openai.Bool(false),
		}

		stream := helper.CreateResponseStreamingWithHeaders(ctx, params)
		helper.AssertNoError(t, stream.Err(), "Failed to start Responses streaming")

		var (
			events             int
			outputText         strings.Builder
			functionCallsByID  = map[string]*functionCall{}
			roundHasEncrypted  bool
			roundSawCompletion bool
		)

		for stream.Next() {
			event := stream.Current()
			events++

			if event.Type == "response.output_text.delta" && event.Delta != "" {
				outputText.WriteString(event.Delta)
			}

			if event.Type == "response.function_call_arguments.done" {
				ev := event.AsResponseFunctionCallArgumentsDone()
				call := functionCallsByID[ev.ItemID]
				if call == nil {
					call = &functionCall{ID: ev.ItemID, Type: "function_call"}
					functionCallsByID[ev.ItemID] = call
				}
				if ev.Name != "" {
					call.Name = ev.Name
				}
				call.Arguments = ev.Arguments
			}

			if event.Type == "response.output_item.added" || event.Type == "response.output_item.done" {
				if event.Item.Type == "reasoning" && event.Item.EncryptedContent != "" {
					roundHasEncrypted = true
				}
				if event.Item.Type == "function_call" || event.Item.Type == "custom_tool_call" {
					id := event.Item.ID
					if id != "" {
						call := functionCallsByID[id]
						if call == nil {
							call = &functionCall{ID: id}
							functionCallsByID[id] = call
						}
						call.Type = event.Item.Type
						if event.Item.Name != "" {
							call.Name = event.Item.Name
						}
						if event.Item.Arguments != "" {
							call.Arguments = event.Item.Arguments
						}
						if event.Item.Input != "" {
							call.Input = event.Item.Input
						}
					}
				}
			}

			if event.Type == "response.completed" {
				roundSawCompletion = true
				for _, item := range event.Response.Output {
					if item.Type == "reasoning" && item.EncryptedContent != "" {
						roundHasEncrypted = true
					}
				}
			}
		}

		if err := stream.Err(); err != nil {
			helper.AssertNoError(t, err, "Stream error occurred")
		}

		streamCalls++

		if events == 0 {
			t.Fatalf("%s: Expected at least one streaming event, got 0", r.name)
		}

		functionCalls := flattenCalls(functionCallsByID)

		if r.forbidTools && len(functionCalls) > 0 {
			t.Fatalf("%s: Expected no tool calls, got %d", r.name, len(functionCalls))
		}
		if r.needsToolCall && len(functionCalls) == 0 {
			t.Fatalf("%s: Expected at least one tool call, got none", r.name)
		}

		if r.needsToolCall {
			validateFunctionCallsHaveArgs(t, r.name, functionCalls)
			switch r.name {
			case "round1-deepseek-chat":
				requireFunctionCallArgEquals(t, r.name, functionCalls, "calculate", "expression", "15 * 23")
			case "round2-gemini-3-flash":
				requireFunctionCallArgEquals(t, r.name, functionCalls, "get_current_weather", "location", "London")
			case "round3-claude-sonnet":
				requireFunctionCallArgEquals(t, r.name, functionCalls, "calculate", "expression", "50 * 30")
			}
		}

		if len(functionCalls) > 0 {
			seenAnyToolCall = true
		}
		if roundHasEncrypted {
			seenAnyEncryptedContent = true
		}

		if !roundSawCompletion {
			t.Fatalf("%s: Expected to receive response.completed event", r.name)
		}
		if out := outputText.String(); out == "" && !r.needsToolCall {
			t.Fatalf("%s: Expected non-empty output text for non-tool round", r.name)
		}
	}

	if streamCalls != 4 {
		t.Fatalf("Expected exactly 4 streaming calls, got %d", streamCalls)
	}
	if !seenAnyEncryptedContent {
		t.Fatalf("Expected at least one reasoning item with encrypted_content, got none")
	}
	if !seenAnyToolCall {
		t.Fatalf("Expected at least one tool call across the four rounds, got none")
	}
}

type functionCall struct {
	ID        string
	Type      string
	Name      string
	Arguments string
	Input     string
}

func flattenCalls(m map[string]*functionCall) []functionCall {
	out := make([]functionCall, 0, len(m))
	for _, v := range m {
		if v == nil {
			continue
		}
		out = append(out, *v)
	}
	return out
}

func validateFunctionCallsHaveArgs(t *testing.T, roundName string, calls []functionCall) {
	t.Helper()
	for idx, c := range calls {
		if c.Name == "" {
			t.Fatalf("%s: functionCalls[%d] missing function name", roundName, idx)
		}
		if c.Type == "function_call" && c.Arguments == "" {
			t.Fatalf("%s: functionCalls[%d] (%s) missing arguments", roundName, idx, c.Name)
		}
		if c.Type == "custom_tool_call" && c.Input == "" {
			t.Fatalf("%s: functionCalls[%d] (%s) missing input", roundName, idx, c.Name)
		}
	}
}

func requireFunctionCallArgEquals(
	t *testing.T,
	roundName string,
	calls []functionCall,
	functionName string,
	argKey string,
	want string,
) {
	t.Helper()

	var matched []functionCall
	for _, c := range calls {
		if c.Name == functionName {
			matched = append(matched, c)
		}
	}
	if len(matched) == 0 {
		t.Fatalf("%s: expected at least one %q tool call, got none", roundName, functionName)
	}

	gotAny := false
	for _, c := range matched {
		var args map[string]any
		raw := c.Arguments
		if c.Type == "custom_tool_call" {
			raw = c.Input
		}
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			t.Fatalf("%s: %q arguments not valid JSON: %v", roundName, functionName, err)
		}
		v, ok := args[argKey]
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
