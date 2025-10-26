package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
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

	messages1 := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("I need to plan a software development project. What are the key phases?"),
	}

	params1 := openai.ChatCompletionNewParams{
		Messages: messages1,
		Model:    helper.GetModel(),
	}

	completion1, err := helper.Client.Chat.Completions.New(ctx1, params1)
	helper.AssertNoError(t, err, "Failed in trace 1, call 1")

	helper.ValidateChatResponse(t, completion1, "Trace 1, call 1")

	response1 := completion1.Choices[0].Message.Content
	t.Logf("Trace 1 - Assistant: %s", response1)

	// Continue trace 1 with more calls
	messages1 = append(messages1, completion1.Choices[0].Message.ToParam())
	messages1 = append(messages1, openai.UserMessage("What tools and technologies should I consider for each phase?"))

	params1.Messages = messages1
	completion2, err := helper.Client.Chat.Completions.New(ctx1, params1)
	helper.AssertNoError(t, err, "Failed in trace 1, call 2")

	helper.ValidateChatResponse(t, completion2, "Trace 1, call 2")

	response2 := completion2.Choices[0].Message.Content
	t.Logf("Trace 1 - Assistant: %s", response2)

	// Trace 2: Different topic but same thread
	t.Logf("=== Starting Trace 2: Team Management ===")

	// Create new trace but same thread using helper function
	helper2 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)

	ctx2 := helper2.CreateTestContext()

	messages2 := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("For the development team, how should I structure the team roles?"),
	}

	params2 := openai.ChatCompletionNewParams{
		Messages: messages2,
		Model:    helper2.GetModel(),
	}

	completion3, err := helper2.Client.Chat.Completions.New(ctx2, params2)
	helper.AssertNoError(t, err, "Failed in trace 2, call 1")

	helper.ValidateChatResponse(t, completion3, "Trace 2, call 1")

	response3 := completion3.Choices[0].Message.Content
	t.Logf("Trace 2 - Assistant: %s", response3)

	// Continue trace 2
	messages2 = append(messages2, completion3.Choices[0].Message.ToParam())
	messages2 = append(messages2, openai.UserMessage("What about the project timeline and milestones?"))

	params2.Messages = messages2
	completion4, err := helper2.Client.Chat.Completions.New(ctx2, params2)
	helper.AssertNoError(t, err, "Failed in trace 2, call 2")

	helper.ValidateChatResponse(t, completion4, "Trace 2, call 2")

	response4 := completion4.Choices[0].Message.Content
	t.Logf("Trace 2 - Assistant: %s", response4)

	// Trace 3: Tool usage in same thread
	t.Logf("=== Starting Trace 3: Resource Planning ===")

	helper3 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)

	ctx3 := helper3.CreateTestContext()

	messages3 := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("I need to estimate the project costs. What's a typical budget breakdown?"),
	}

	// Define calculator tool for cost estimation
	calculatorFunction := shared.FunctionDefinitionParam{
		Name:        "calculate_cost",
		Description: openai.String("Calculate project costs based on team size and duration"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
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
			"required": []string{"team_size", "duration_months", "hourly_rate"},
		},
	}

	calculatorTool := openai.ChatCompletionFunctionTool(calculatorFunction)

	params3 := openai.ChatCompletionNewParams{
		Messages: messages3,
		Tools:    []openai.ChatCompletionToolUnionParam{calculatorTool},
		Model:    helper3.GetModel(),
	}

	completion5, err := helper3.Client.Chat.Completions.New(ctx3, params3)
	helper.AssertNoError(t, err, "Failed in trace 3, call 1 with tools")

	helper.ValidateChatResponse(t, completion5, "Trace 3, call 1 with tools")

	// Check for tool calls
	if len(completion5.Choices) > 0 && completion5.Choices[0].Message.ToolCalls != nil {
		t.Logf("Tool calls detected in trace 3: %d", len(completion5.Choices[0].Message.ToolCalls))

		// Process tool calls
		var toolResults []string
		for _, toolCall := range completion5.Choices[0].Message.ToolCalls {
			var args map[string]interface{}
			err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			helper.AssertNoError(t, err, "Failed to parse tool arguments")

			var result string
			switch toolCall.Function.Name {
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
			t.Logf("Tool %s result: %s", toolCall.Function.Name, result)
		}

		// Continue conversation with tool results
		messages3 = append(messages3, completion5.Choices[0].Message.ToParam())
		for i, toolCall := range completion5.Choices[0].Message.ToolCalls {
			messages3 = append(messages3, openai.ToolMessage(toolResults[i], toolCall.ID))
		}

		// Add follow-up question
		messages3 = append(messages3, openai.UserMessage("Based on this estimate, how should I adjust the project scope?"))

		// Final call with tool results
		params3.Messages = messages3
		params3.Tools = nil
		completion6, err := helper3.Client.Chat.Completions.New(ctx3, params3)
		helper.AssertNoError(t, err, "Failed in trace 3, call 2")

		helper.ValidateChatResponse(t, completion6, "Trace 3, call 2")

		response6 := completion6.Choices[0].Message.Content
		t.Logf("Trace 3 - Assistant: %s", response6)

		t.Logf("Single thread test completed successfully with 3 traces, %d total AI calls, and tool usage", 6)
	} else {
		// If no tool calls, continue with text conversation
		response5 := completion5.Choices[0].Message.Content
		t.Logf("Trace 3 - Assistant: %s", response5)

		t.Logf("Single thread test completed successfully with 3 traces and %d total AI calls (text only)", 5)
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
