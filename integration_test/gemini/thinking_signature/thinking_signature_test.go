package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/looplj/axonhub/gemini_test/internal/testutil"
	"google.golang.org/genai"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestThinkingSignatureWithMultiRoundToolsStreamingAndMultiModels(t *testing.T) {
	helper := testutil.NewTestHelper(t, "thinking_signature")
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	tools := []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "calculate",
					Description: "Perform mathematical calculations",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"expression": {Type: genai.TypeString},
						},
						Required: []string{"expression"},
					},
				},
			},
		},
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "get_current_weather",
					Description: "Get the current weather for a specified location",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"location": {Type: genai.TypeString},
						},
						Required: []string{"location"},
					},
				},
			},
		},
	}

	baseConfig := &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.1),
		Tools:       tools,
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingLevel:   genai.ThinkingLevelHigh,
		},
	}

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

	var conversation []*genai.Content
	streamCalls := 0
	seenAnyThoughtSignature := false
	seenAnyToolCall := false

	for i, r := range rounds {
		t.Logf("=== %s (model=%s) ===", r.name, r.model)

		roundConfig := &genai.GenerateContentConfig{}
		*roundConfig = *baseConfig
		if r.forbidTools {
			roundConfig.Tools = nil
		}

		userContent := &genai.Content{
			Parts: []*genai.Part{{Text: r.userPrompt}},
		}

		contents := append(append([]*genai.Content{}, conversation...), userContent)

		modelContent, functionCalls, hadThoughtSignature := streamGenerateContent(t, helper, ctx, r.model, contents, roundConfig, fmt.Sprintf("r%d", i+1))
		streamCalls++

		if r.forbidTools && len(functionCalls) > 0 {
			t.Fatalf("Expected no tool calls in %s, got %d", r.name, len(functionCalls))
		}
		if r.needsToolCall && len(functionCalls) == 0 {
			t.Fatalf("Expected at least one tool call in %s, got none.", r.name)
		}

		if r.needsToolCall {
			validateToolCallsHaveArgs(t, r.name, functionCalls)
			switch r.name {
			case "round1-deepseek-chat":
				requireToolCallArgEquals(t, r.name, functionCalls, "calculate", "expression", "15 * 23")
			case "round2-gemini-3-flash":
				requireToolCallArgEquals(t, r.name, functionCalls, "get_current_weather", "location", "London")
			case "round3-claude-sonnet":
				requireToolCallArgEquals(t, r.name, functionCalls, "calculate", "expression", "50 * 30")
			}
		}

		if hadThoughtSignature {
			seenAnyThoughtSignature = true
		}
		if len(functionCalls) > 0 {
			seenAnyToolCall = true
		}

		// Append this round to the conversation.
		conversation = append(conversation, userContent)
		if modelContent != nil {
			conversation = append(conversation, modelContent)
		}

		// Append tool results for the next round.
		for callIdx, callPart := range functionCalls {
			if callPart == nil || callPart.FunctionCall == nil {
				continue
			}

			callID := callPart.FunctionCall.ID
			if callID == "" {
				callID = fmt.Sprintf("call_%d_%d", i+1, callIdx+1)
				callPart.FunctionCall.ID = callID
			}

			var result any
			switch callPart.FunctionCall.Name {
			case "calculate":
				result = simulateCalculatorFunction(callPart.FunctionCall.Args)
			case "get_current_weather":
				result = simulateWeatherFunction(callPart.FunctionCall.Args)
			default:
				result = "Unknown function"
			}

			functionResponse := &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					ID:   callID,
					Name: callPart.FunctionCall.Name,
					Response: map[string]any{
						"result": result,
					},
				},
			}

			conversation = append(conversation, &genai.Content{
				Parts: []*genai.Part{functionResponse},
			})
		}
	}

	if streamCalls != 4 {
		t.Fatalf("Expected exactly 4 streaming calls, got %d", streamCalls)
	}
	if !seenAnyThoughtSignature {
		t.Fatalf("Expected at least one thought signature in streamed responses when IncludeThoughts=true, got none")
	}
	if !seenAnyToolCall {
		t.Fatalf("Expected at least one tool call across the four rounds, got none")
	}
}

func validateToolCallsHaveArgs(t *testing.T, roundName string, functionCalls []*genai.Part) {
	t.Helper()
	for idx, p := range functionCalls {
		if p == nil || p.FunctionCall == nil {
			continue
		}
		if p.FunctionCall.Name == "" {
			t.Fatalf("%s: functionCalls[%d] missing function name", roundName, idx)
		}
		if p.FunctionCall.Args == nil {
			t.Fatalf("%s: functionCalls[%d] (%s) missing args map", roundName, idx, p.FunctionCall.Name)
		}
		if len(p.FunctionCall.Args) == 0 {
			t.Fatalf("%s: functionCalls[%d] (%s) args map is empty", roundName, idx, p.FunctionCall.Name)
		}
	}
}

func requireToolCallArgEquals(
	t *testing.T,
	roundName string,
	functionCalls []*genai.Part,
	functionName string,
	argKey string,
	want string,
) {
	t.Helper()

	var matched []*genai.Part
	for _, p := range functionCalls {
		if p == nil || p.FunctionCall == nil {
			continue
		}
		if p.FunctionCall.Name == functionName {
			matched = append(matched, p)
		}
	}
	if len(matched) == 0 {
		t.Fatalf("%s: expected at least one %q tool call, got none", roundName, functionName)
	}

	gotAny := false
	for _, p := range matched {
		v, ok := p.FunctionCall.Args[argKey]
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

func streamGenerateContent(
	t *testing.T,
	helper *testutil.TestHelper,
	ctx context.Context,
	model string,
	contents []*genai.Content,
	config *genai.GenerateContentConfig,
	idPrefix string,
) (*genai.Content, []*genai.Part, bool) {
	t.Helper()

	// Copy config per-call to avoid accidental mutation.
	callConfig := &genai.GenerateContentConfig{}
	*callConfig = *config

	// Ensure headers are attached per call.
	callConfig = helper.MergeHTTPOptions(callConfig)

	chunks := helper.Client.Models.GenerateContentStream(ctx, model, contents, callConfig)

	var (
		modelParts      []*genai.Part
		functionCalls   []*genai.Part
		textBuilder     strings.Builder
		thinkingBuilder strings.Builder
		receivedAny     bool
		hadSignature    bool
	)

	for chunk, err := range chunks {
		if err != nil {
			if err == io.EOF {
				break
			}
			helper.AssertNoError(t, err, "Stream encountered an error")
		}

		if chunk == nil || len(chunk.Candidates) == 0 || chunk.Candidates[0] == nil || chunk.Candidates[0].Content == nil {
			continue
		}

		receivedAny = true

		for _, part := range chunk.Candidates[0].Content.Parts {
			if part == nil {
				continue
			}

			cp := clonePart(part)
			if len(cp.ThoughtSignature) > 0 {
				hadSignature = true
			}
			if cp.FunctionCall != nil && cp.FunctionCall.ID == "" {
				cp.FunctionCall.ID = fmt.Sprintf("%s_fc_%d", idPrefix, len(functionCalls)+1)
			}

			if cp.Text != "" {
				if cp.Thought {
					thinkingBuilder.WriteString(cp.Text)
				} else {
					textBuilder.WriteString(cp.Text)
				}
			}
			if cp.FunctionCall != nil {
				functionCalls = append(functionCalls, cp)
			}
		}
	}

	if !receivedAny {
		t.Fatalf("No streamed responses received")
	}

	var modelContent *genai.Content

	if thought := thinkingBuilder.String(); thought != "" {
		modelParts = append(modelParts, &genai.Part{
			Text:    thought,
			Thought: true,
		})
	}

	if text := textBuilder.String(); text != "" {
		modelParts = append(modelParts, &genai.Part{
			Text: text,
		})
	}

	if len(functionCalls) > 0 {
		modelParts = append(modelParts, functionCalls...)
	}

	if len(modelParts) > 0 {
		modelContent = &genai.Content{
			Role:  "model",
			Parts: modelParts,
		}
	}

	return modelContent, functionCalls, hadSignature
}

func clonePart(p *genai.Part) *genai.Part {
	if p == nil {
		return nil
	}
	cp := *p
	if p.FunctionCall != nil {
		fc := *p.FunctionCall
		if p.FunctionCall.Args != nil {
			argsCopy := make(map[string]any, len(p.FunctionCall.Args))
			for k, v := range p.FunctionCall.Args {
				argsCopy[k] = v
			}
			fc.Args = argsCopy
		}
		cp.FunctionCall = &fc
	}
	if p.FunctionResponse != nil {
		fr := *p.FunctionResponse
		if p.FunctionResponse.Response != nil {
			respCopy := make(map[string]any, len(p.FunctionResponse.Response))
			for k, v := range p.FunctionResponse.Response {
				respCopy[k] = v
			}
			fr.Response = respCopy
		}
		cp.FunctionResponse = &fr
	}
	if p.ThoughtSignature != nil {
		sigCopy := make([]byte, len(p.ThoughtSignature))
		copy(sigCopy, p.ThoughtSignature)
		cp.ThoughtSignature = sigCopy
	}
	return &cp
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
