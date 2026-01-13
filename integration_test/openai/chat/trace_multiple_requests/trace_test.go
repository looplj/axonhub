package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

func TestSingleTraceMultipleCalls(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "TestSingleTraceMultipleCalls")

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Start a trace with multiple AI calls
	t.Logf("Starting single trace with multiple AI calls...")

	// First call: Simple greeting
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello! I need help with a calculation task."),
	}

	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    helper.GetModel(),
	}

	completion, err := helper.CreateChatCompletionWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed in first trace call")

	helper.ValidateChatResponse(t, completion, "First trace call")

	firstResponse := completion.Choices[0].Message.Content
	t.Logf("Assistant (first in trace): %s", firstResponse)

	// Second call: Follow-up question in same trace
	messages = append(messages, completion.Choices[0].Message.ToParam())
	messages = append(messages, openai.UserMessage("I need to perform some calculations. Can you help?"))

	params.Messages = messages
	completion2, err := helper.CreateChatCompletionWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed in second trace call")

	helper.ValidateChatResponse(t, completion2, "Second trace call")

	secondResponse := completion2.Choices[0].Message.Content
	t.Logf("Assistant (second in trace): %s", secondResponse)

	// Third call: Tool usage in same trace
	messages = append(messages, completion2.Choices[0].Message.ToParam())
	messages = append(messages, openai.UserMessage("What's 15 * 7 + 23? I need this calculation for one of the tasks."))

	// Define calculator tool
	calculatorFunction := shared.FunctionDefinitionParam{
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
	}

	calculatorTool := openai.ChatCompletionFunctionTool(calculatorFunction)

	params = openai.ChatCompletionNewParams{
		Messages: messages,
		Tools:    []openai.ChatCompletionToolUnionParam{calculatorTool},
		Model:    helper.GetModel(),
	}

	completion3, err := helper.CreateChatCompletionWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed in third trace call with tools")

	helper.ValidateChatResponse(t, completion3, "Third trace call with tools")

	// Check for tool calls
	if len(completion3.Choices) > 0 && completion3.Choices[0].Message.ToolCalls != nil {
		t.Logf("Tool calls detected: %d", len(completion3.Choices[0].Message.ToolCalls))

		// Process tool calls
		var toolResults []string
		for _, toolCall := range completion3.Choices[0].Message.ToolCalls {
			var args map[string]interface{}
			err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			helper.AssertNoError(t, err, "Failed to parse tool arguments")

			var result string
			switch toolCall.Function.Name {
			case "calculate":
				calcResult := simulateCalculatorFunction(args)
				result = fmt.Sprintf("%v", calcResult)
			default:
				result = "Unknown function"
			}

			toolResults = append(toolResults, result)
			t.Logf("Tool %s result: %s", toolCall.Function.Name, result)
		}

		// Continue conversation with tool results
		messages = append(messages, completion3.Choices[0].Message.ToParam())
		for i, toolCall := range completion3.Choices[0].Message.ToolCalls {
			messages = append(messages, openai.ToolMessage(toolResults[i], toolCall.ID))
		}

		// Add final question that explicitly references the calculation
		messages = append(messages, openai.UserMessage("Thank you! Please confirm: what was the result of the calculation 15 * 7 + 23?"))

		// Fourth call with tool results
		params.Messages = messages
		params.Tools = nil // No more tools needed
		completion4, err := helper.CreateChatCompletionWithHeaders(ctx, params)
		helper.AssertNoError(t, err, "Failed in fourth trace call")

		helper.ValidateChatResponse(t, completion4, "Fourth trace call")

		fourthResponse := completion4.Choices[0].Message.Content
		t.Logf("Assistant (fourth in trace): %s", fourthResponse)

		// Verify calculation was correct (15 * 7 + 23 = 105 + 23 = 128)
		if !testutil.ContainsCaseInsensitive(fourthResponse, "128") && !testutil.ContainsCaseInsensitive(fourthResponse, "one hundred") {
			t.Errorf("Expected calculation result 128 in final response, got: %s", fourthResponse)
		}

		t.Logf("Single trace test completed successfully with %d AI calls and tool usage", len(messages)/2+1)
	} else {
		// If no tool calls, continue with text conversation
		thirdResponse := completion3.Choices[0].Message.Content
		t.Logf("Assistant (third in trace): %s", thirdResponse)

		// Verify calculation in text response
		if !testutil.ContainsCaseInsensitive(thirdResponse, "128") && !testutil.ContainsCaseInsensitive(thirdResponse, "one hundred") {
			t.Errorf("Expected calculation result 128, got: %s", thirdResponse)
		}

		t.Logf("Single trace test completed successfully with %d AI calls (text only)", len(messages)/2+1)
	}
}

func simulateCalculatorFunction(args map[string]interface{}) float64 {
	expression, _ := args["expression"].(string)

	switch expression {
	case "365 * 24":
		return 8760
	case "100 / 4":
		return 25
	case "50 * 30":
		return 1500
	case "15 * 7 + 23":
		return 128
	default:
		return 42
	}
}
