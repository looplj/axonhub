package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/looplj/axonhub/anthropic_test/internal/testutil"
)

func TestSingleThreadMultipleTraces(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	// Get current thread ID for this test
	currentThreadID := helper.Config.ThreadID
	t.Logf("Using thread ID: %s", currentThreadID)

	t.Logf("Starting single thread with multiple traces...")

	// Trace 1: Project planning
	t.Logf("=== Starting Trace 1: Project Planning ===")
	ctx1 := helper.CreateTestContext()

	var messages1 []anthropic.MessageParam
	messages1 = append(messages1, anthropic.NewUserMessage(anthropic.NewTextBlock("I need to plan a software development project. What are the key phases?")))

	params1 := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages1,
		MaxTokens: 1024,
	}

	completion1, err := helper.Client.Messages.New(ctx1, params1)
	helper.AssertNoError(t, err, "Failed in trace 1, call 1")

	helper.ValidateMessageResponse(t, completion1, "Trace 1, call 1")

	response1Text := ""
	for _, block := range completion1.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			response1Text += textBlock.Text
		}
	}
	t.Logf("Trace 1 - Assistant: %s", response1Text)

	// Continue trace 1 with more calls
	messages1 = append(messages1, completion1.ToParam())
	messages1 = append(messages1, anthropic.NewUserMessage(anthropic.NewTextBlock("What tools and technologies should I consider for each phase?")))

	params1.Messages = messages1
	completion2, err := helper.Client.Messages.New(ctx1, params1)
	helper.AssertNoError(t, err, "Failed in trace 1, call 2")

	helper.ValidateMessageResponse(t, completion2, "Trace 1, call 2")

	response2Text := ""
	for _, block := range completion2.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			response2Text += textBlock.Text
		}
	}
	t.Logf("Trace 1 - Assistant: %s", response2Text)

	// Trace 2: Different topic but same thread
	t.Logf("=== Starting Trace 2: Team Management ===")

	// Create new trace but same thread using helper function
	helper2 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)

	ctx2 := helper2.CreateTestContext()

	var messages2 []anthropic.MessageParam
	messages2 = append(messages2, anthropic.NewUserMessage(anthropic.NewTextBlock("For the development team, how should I structure the team roles?")))

	params2 := anthropic.MessageNewParams{
		Model:     helper2.GetModel(),
		Messages:  messages2,
		MaxTokens: 1024,
	}

	completion3, err := helper2.Client.Messages.New(ctx2, params2)
	helper.AssertNoError(t, err, "Failed in trace 2, call 1")

	helper.ValidateMessageResponse(t, completion3, "Trace 2, call 1")

	response3Text := ""
	for _, block := range completion3.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			response3Text += textBlock.Text
		}
	}
	t.Logf("Trace 2 - Assistant: %s", response3Text)

	// Continue trace 2
	messages2 = append(messages2, completion3.ToParam())
	messages2 = append(messages2, anthropic.NewUserMessage(anthropic.NewTextBlock("What about the project timeline and milestones?")))

	params2.Messages = messages2
	completion4, err := helper2.Client.Messages.New(ctx2, params2)
	helper.AssertNoError(t, err, "Failed in trace 2, call 2")

	helper.ValidateMessageResponse(t, completion4, "Trace 2, call 2")

	response4Text := ""
	for _, block := range completion4.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			response4Text += textBlock.Text
		}
	}
	t.Logf("Trace 2 - Assistant: %s", response4Text)

	// Trace 3: Tool usage in same thread
	t.Logf("=== Starting Trace 3: Resource Planning ===")

	helper3 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)

	ctx3 := helper3.CreateTestContext()

	var messages3 []anthropic.MessageParam
	messages3 = append(messages3, anthropic.NewUserMessage(anthropic.NewTextBlock("I need to estimate the project costs. What's a typical budget breakdown?")))

	// Define calculator tool for cost estimation
	calculatorTool := anthropic.ToolParam{
		Name:        "calculate_cost",
		Description: anthropic.String("Calculate project costs based on team size and duration"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"team_size": map[string]any{
					"type": "number",
				},
				"duration_months": map[string]any{
					"type": "number",
				},
				"hourly_rate": map[string]any{
					"type": "number",
				},
			},
			Required: []string{"team_size", "duration_months", "hourly_rate"},
		},
	}

	tools := []anthropic.ToolUnionParam{
		{OfTool: &calculatorTool},
	}

	params3 := anthropic.MessageNewParams{
		Model:     helper3.GetModel(),
		Messages:  messages3,
		Tools:     tools,
		MaxTokens: 1024,
	}

	completion5, err := helper3.Client.Messages.New(ctx3, params3)
	helper.AssertNoError(t, err, "Failed in trace 3, call 1 with tools")

	helper.ValidateMessageResponse(t, completion5, "Trace 3, call 1 with tools")

	// Check for tool calls
	if completion5.StopReason == anthropic.StopReasonToolUse {
		t.Logf("Tool calls detected in trace 3: %d", len(completion5.Content))

		// Process tool calls
		var toolResults []string
		for _, block := range completion5.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				var args map[string]interface{}
				err = json.Unmarshal([]byte(toolUseBlock.Input), &args)
				helper.AssertNoError(t, err, "Failed to parse tool arguments")

				var result string
				switch toolUseBlock.Name {
				case "calculate_cost":
					// Simulate cost calculation: team_size * duration_months * hourly_rate * 160 (hours per month)
					teamSize := args["team_size"].(float64)
					duration := args["duration_months"].(float64)
					hourlyRate := args["hourly_rate"].(float64)
					totalCost := teamSize * duration * hourlyRate * 160
					result = fmt.Sprintf("Total estimated cost: $%.2f", totalCost)
				default:
					result = "Unknown function"
				}

				toolResults = append(toolResults, result)
				t.Logf("Tool %s result: %s", toolUseBlock.Name, result)
			}
		}

		// Continue conversation with tool results
		messages3 = append(messages3, completion5.ToParam())
		for _, block := range completion5.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				// Find the corresponding result
				for i, result := range toolResults {
					if i < len(completion5.Content) {
						toolResult := anthropic.NewToolResultBlock(toolUseBlock.ID, result, false)
						messages3 = append(messages3, anthropic.NewUserMessage(toolResult))
						break
					}
				}
			}
		}

		// Add follow-up question
		messages3 = append(messages3, anthropic.NewUserMessage(anthropic.NewTextBlock("Based on this estimate, how should I adjust the project scope?")))

		// Final call with tool results
		params3.Messages = messages3
		params3.Tools = []anthropic.ToolUnionParam{} // Clear tools
		completion6, err := helper3.Client.Messages.New(ctx3, params3)
		helper.AssertNoError(t, err, "Failed in trace 3, call 2")

		helper.ValidateMessageResponse(t, completion6, "Trace 3, call 2")

		response6Text := ""
		for _, block := range completion6.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				response6Text += textBlock.Text
			}
		}
		t.Logf("Trace 3 - Assistant: %s", response6Text)

		t.Logf("Single thread test completed successfully with 3 traces, 6 total AI calls, and tool usage")
	} else {
		// If no tool calls, continue with text conversation
		response5Text := ""
		for _, block := range completion5.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				response5Text += textBlock.Text
			}
		}
		t.Logf("Trace 3 - Assistant: %s", response5Text)

		t.Logf("Single thread test completed successfully with 3 traces and 5 total AI calls (text only)")
	}

	// Verify all traces used the same thread ID
	if helper.Config.ThreadID != helper2.Config.ThreadID || helper2.Config.ThreadID != helper3.Config.ThreadID {
		t.Errorf("Expected all traces to use the same thread ID %s, but got: %s, %s, %s",
			currentThreadID, helper.Config.ThreadID, helper2.Config.ThreadID, helper3.Config.ThreadID)
	}

	// Verify all traces used different trace IDs
	if helper.Config.TraceID == helper2.Config.TraceID || helper2.Config.TraceID == helper3.Config.TraceID || helper.Config.TraceID == helper3.Config.TraceID {
		t.Errorf("Expected all traces to use different trace IDs, but got duplicates: %s, %s, %s",
			helper.Config.TraceID, helper2.Config.TraceID, helper3.Config.TraceID)
	}

	t.Logf("Thread ID consistency verified: %s", currentThreadID)
	t.Logf("Trace ID uniqueness verified: %s, %s, %s",
		helper.Config.TraceID, helper2.Config.TraceID, helper3.Config.TraceID)
}
