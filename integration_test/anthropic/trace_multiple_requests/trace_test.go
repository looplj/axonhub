package main

import (
	"encoding/json"
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

func TestSingleTraceMultipleCalls(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Test multiple calls within the same trace
	var messages []anthropic.MessageParam

	// First call - Initial greeting and project discussion
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Hi! I'm working on a software project and need some help with planning.")))

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 1024,
	}

	response1, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in first trace call")

	helper.ValidateMessageResponse(t, response1, "First trace call")

	response1Text := ""
	for _, block := range response1.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			response1Text += textBlock.Text
		}
	}
	t.Logf("First response: %s", response1Text)

	// Verify first response acknowledges the project
	if !testutil.ContainsCaseInsensitive(response1Text, "project") && !testutil.ContainsCaseInsensitive(response1Text, "planning") {
		t.Errorf("Expected first response to acknowledge project, got: %s", response1Text)
	}

	// Second call - Follow-up with context preservation
	messages = append(messages, response1.ToParam())
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Can you help me break down the project into smaller tasks?")))

	params.Messages = messages
	response2, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in second trace call")

	helper.ValidateMessageResponse(t, response2, "Second trace call")

	response2Text := ""
	for _, block := range response2.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			response2Text += textBlock.Text
		}
	}
	t.Logf("Second response: %s", response2Text)

	// Third call - Add tool integration for calculation
	messages = append(messages, response2.ToParam())
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Actually, let me ask: what is 15 * 7 + 23?")))

	// Add calculator tool
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
		{OfTool: &calculatorTool},
	}

	params.Messages = messages
	params.Tools = tools
	response3, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in third trace call with tools")

	helper.ValidateMessageResponse(t, response3, "Third trace call with tools")

	// Check for tool calls
	if response3.StopReason == anthropic.StopReasonToolUse {
		t.Logf("Tool calls detected in trace: %d", len(response3.Content))

		// Process tool calls
		for _, block := range response3.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				t.Logf("Tool call in trace: %s", toolUseBlock.Name)

				// Verify tool call
				if toolUseBlock.Name == "calculate" {
					var args map[string]interface{}
					err = json.Unmarshal([]byte(toolUseBlock.Input), &args)
					helper.AssertNoError(t, err, "Failed to parse tool arguments")

					expression, ok := args["expression"].(string)
					if !ok {
						t.Error("Expected expression argument")
					} else {
						t.Logf("Expression in trace: %s", expression)

						// Verify the expression
						if expression != "15 * 7 + 23" {
							t.Errorf("Expected expression '15 * 7 + 23', got: %s", expression)
						}

						// Continue conversation with result
						messages = append(messages, response3.ToParam())
						resultBlock := anthropic.NewToolResultBlock(toolUseBlock.ID, "128", false)
						messages = append(messages, anthropic.NewUserMessage(resultBlock))

						// Fourth call - Final response with calculation
						params.Messages = messages
						params.Tools = []anthropic.ToolUnionParam{} // Clear tools
						response4, err := helper.Client.Messages.New(ctx, params)
						helper.AssertNoError(t, err, "Failed in fourth trace call")

						helper.ValidateMessageResponse(t, response4, "Fourth trace call - final")

						response4Text := ""
						for _, block := range response4.Content {
							if textBlock := block.AsText(); textBlock.Text != "" {
								response4Text += textBlock.Text
							}
						}
						t.Logf("Final response: %s", response4Text)

						// Verify calculation result is incorporated
						if !testutil.ContainsCaseInsensitive(response4Text, "128") && !testutil.ContainsCaseInsensitive(response4Text, "one hundred") {
							t.Errorf("Expected final response to mention 128, got: %s", response4Text)
						}

						// Verify context is maintained across all calls
						if !testutil.ContainsCaseInsensitive(response4Text, "project") && !testutil.ContainsCaseInsensitive(response4Text, "planning") {
							t.Errorf("Expected context preservation across trace calls, got: %s", response4Text)
						}
					}
				}
			}
		}
	} else {
		t.Logf("No tool calls in trace, checking direct response")

		response3Text := ""
		for _, block := range response3.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				response3Text += textBlock.Text
			}
		}

		// Should still get a result
		if !testutil.ContainsCaseInsensitive(response3Text, "128") && !testutil.ContainsCaseInsensitive(response3Text, "one hundred") {
			t.Errorf("Expected response to mention 128, got: %s", response3Text)
		}
	}

	t.Logf("Single trace multiple calls test completed with %d total messages", len(messages))
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
