package main

import (
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

// Helper function
func createUserMessage(text string) responses.ResponseInputItemUnionParam {
	return responses.ResponseInputItemParamOfInputMessage(
		responses.ResponseInputMessageContentListParam{
			responses.ResponseInputContentParamOfInputText(text),
		},
		"user",
	)
}

// Helper function to create an assistant message input item from response output
func createAssistantMessage(text string) responses.ResponseInputItemUnionParam {
	return responses.ResponseInputItemParamOfOutputMessage(
		[]responses.ResponseOutputMessageContentUnionParam{
			{
				OfOutputText: &responses.ResponseOutputTextParam{
					Text:        text,
					Annotations: []responses.ResponseOutputTextAnnotationUnionParam{},
				},
			},
		},
		"", // id can be empty for stateless
		responses.ResponseOutputMessageStatusCompleted,
	)
}

func TestSingleTraceMultipleCallsStateless(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Start a trace with multiple Responses API calls using stateless approach
	t.Logf("Starting single trace with multiple Responses API calls (stateless)...")

	// First call: Simple greeting
	input1 := "Hello! I need help with a calculation task."
	t.Logf("Call 1: %s", input1)

	params1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
			},
		},
	}

	resp1, err := helper.Client.Responses.New(ctx, params1)
	helper.AssertNoError(t, err, "Failed in first trace call")

	if resp1 == nil {
		t.Fatal("First response is nil")
	}

	output1 := resp1.OutputText()
	t.Logf("Assistant (call 1 in trace): %s", output1)

	// Second call: Follow-up question in same trace using stateless history
	input2 := "I need to perform some calculations. Can you help?"
	t.Logf("Call 2 (stateless with history): %s", input2)

	params2 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
				createAssistantMessage(output1),
				createUserMessage(input2),
			},
		},
	}

	resp2, err := helper.Client.Responses.New(ctx, params2)
	helper.AssertNoError(t, err, "Failed in second trace call")

	if resp2 == nil {
		t.Fatal("Second response is nil")
	}

	output2 := resp2.OutputText()
	t.Logf("Assistant (call 2 in trace): %s", output2)

	// Third call: Request calculation in same trace
	input3 := "What's 15 * 7 + 23? I need this calculation for one of the tasks."
	t.Logf("Call 3 (stateless with history): %s", input3)

	params3 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
				createAssistantMessage(output1),
				createUserMessage(input2),
				createAssistantMessage(output2),
				createUserMessage(input3),
			},
		},
	}

	resp3, err := helper.Client.Responses.New(ctx, params3)
	helper.AssertNoError(t, err, "Failed in third trace call")

	if resp3 == nil {
		t.Fatal("Third response is nil")
	}

	output3 := resp3.OutputText()
	t.Logf("Assistant (call 3 in trace): %s", output3)

	// Verify calculation result (15 * 7 + 23 = 128)
	if !testutil.ContainsAnyCaseInsensitive(output3, "128", "one hundred twenty") {
		t.Logf("Warning: Expected calculation result 128 in response, got: %s", output3)
		// Note: Not failing the test as the model might explain it differently
	}

	// Fourth call: Follow-up confirmation in same trace
	input4 := "Thank you! Please confirm: what was the result of the calculation 15 * 7 + 23?"
	t.Logf("Call 4 (stateless with history): %s", input4)

	params4 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
				createAssistantMessage(output1),
				createUserMessage(input2),
				createAssistantMessage(output2),
				createUserMessage(input3),
				createAssistantMessage(output3),
				createUserMessage(input4),
			},
		},
	}

	resp4, err := helper.Client.Responses.New(ctx, params4)
	helper.AssertNoError(t, err, "Failed in fourth trace call")

	if resp4 == nil {
		t.Fatal("Fourth response is nil")
	}

	output4 := resp4.OutputText()
	t.Logf("Assistant (call 4 in trace): %s", output4)

	// Verify calculation is confirmed
	if !testutil.ContainsAnyCaseInsensitive(output4, "128", "one hundred twenty") {
		t.Logf("Warning: Expected calculation result 128 in confirmation, got: %s", output4)
	}

	t.Logf("Single trace stateless test completed successfully with 4 Responses API calls")
}

func TestSingleTraceContextPreservationStateless(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	t.Logf("Testing context preservation across multiple stateless calls in a single trace...")

	// Call 1: Introduce multiple facts
	input1 := "My name is Bob, I'm 30 years old, and I work as a software engineer in Seattle."
	t.Logf("Call 1: %s", input1)

	params1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
			},
		},
	}

	resp1, err := helper.Client.Responses.New(ctx, params1)
	helper.AssertNoError(t, err, "Failed in call 1")

	if resp1 == nil {
		t.Fatal("Response 1 is nil")
	}

	output1 := resp1.OutputText()
	t.Logf("Assistant (call 1): %s", output1)

	// Call 2: Ask about name
	input2 := "What's my name?"
	t.Logf("Call 2: %s", input2)

	params2 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
				createAssistantMessage(output1),
				createUserMessage(input2),
			},
		},
	}

	resp2, err := helper.Client.Responses.New(ctx, params2)
	helper.AssertNoError(t, err, "Failed in call 2")

	if resp2 == nil {
		t.Fatal("Response 2 is nil")
	}

	output2 := resp2.OutputText()
	t.Logf("Assistant (call 2): %s", output2)

	// Verify name is recalled
	if !testutil.ContainsCaseInsensitive(output2, "bob") {
		t.Errorf("Expected name 'Bob' to be recalled, got: %s", output2)
	}

	// Call 3: Ask about occupation
	input3 := "What do I do for work?"
	t.Logf("Call 3: %s", input3)

	params3 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
				createAssistantMessage(output1),
				createUserMessage(input2),
				createAssistantMessage(output2),
				createUserMessage(input3),
			},
		},
	}

	resp3, err := helper.Client.Responses.New(ctx, params3)
	helper.AssertNoError(t, err, "Failed in call 3")

	if resp3 == nil {
		t.Fatal("Response 3 is nil")
	}

	output3 := resp3.OutputText()
	t.Logf("Assistant (call 3): %s", output3)

	// Verify occupation is recalled
	if !testutil.ContainsCaseInsensitive(output3, "software engineer") {
		t.Errorf("Expected occupation 'software engineer' to be recalled, got: %s", output3)
	}

	// Call 4: Ask about location
	input4 := "Where do I live?"
	t.Logf("Call 4: %s", input4)

	params4 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
				createAssistantMessage(output1),
				createUserMessage(input2),
				createAssistantMessage(output2),
				createUserMessage(input3),
				createAssistantMessage(output3),
				createUserMessage(input4),
			},
		},
	}

	resp4, err := helper.Client.Responses.New(ctx, params4)
	helper.AssertNoError(t, err, "Failed in call 4")

	if resp4 == nil {
		t.Fatal("Response 4 is nil")
	}

	output4 := resp4.OutputText()
	t.Logf("Assistant (call 4): %s", output4)

	// Verify location is recalled
	if !testutil.ContainsCaseInsensitive(output4, "seattle") {
		t.Errorf("Expected location 'Seattle' to be recalled, got: %s", output4)
	}

	t.Logf("Stateless context preservation test completed successfully with 4 calls")
}
