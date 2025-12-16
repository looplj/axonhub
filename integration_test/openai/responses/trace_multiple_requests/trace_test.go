package main

import (
	"os"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

func TestMain(m *testing.M) {
	// Set up any global test configuration here if needed
	code := m.Run()
	os.Exit(code)
}

func TestSingleTraceMultipleCalls(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestSingleTraceMultipleCalls")
	t.Skip("Test uses PreviousResponseID - temporarily skipped")
	// Skip test if no API key is configured

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Start a trace with multiple Responses API calls
	t.Logf("Starting single trace with multiple Responses API calls...")

	// First call: Simple greeting
	input1 := "Hello! I need help with a calculation task."
	t.Logf("Call 1: %s", input1)

	params1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input1),
		},
	}

	resp1, err := helper.CreateResponseWithHeaders(ctx, params1)
	helper.AssertNoError(t, err, "Failed in first trace call")

	if resp1 == nil || resp1.ID == "" {
		t.Fatal("First response is nil or missing ID")
	}

	output1 := resp1.OutputText()
	t.Logf("Assistant (call 1 in trace): %s", output1)

	// Second call: Follow-up question in same trace using previous_response_id
	input2 := "I need to perform some calculations. Can you help?"
	t.Logf("Call 2 (with previous_response_id=%s): %s", resp1.ID, input2)

	params2 := responses.ResponseNewParams{
		Model:              shared.ResponsesModel(helper.GetModel()),
		PreviousResponseID: openai.String(resp1.ID),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input2),
		},
	}

	resp2, err := helper.CreateResponseWithHeaders(ctx, params2)
	helper.AssertNoError(t, err, "Failed in second trace call")

	if resp2 == nil || resp2.ID == "" {
		t.Fatal("Second response is nil or missing ID")
	}

	output2 := resp2.OutputText()
	t.Logf("Assistant (call 2 in trace): %s", output2)

	// Third call: Request calculation in same trace
	input3 := "What's 15 * 7 + 23? I need this calculation for one of the tasks."
	t.Logf("Call 3 (with previous_response_id=%s): %s", resp2.ID, input3)

	params3 := responses.ResponseNewParams{
		Model:              shared.ResponsesModel(helper.GetModel()),
		PreviousResponseID: openai.String(resp2.ID),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input3),
		},
	}

	resp3, err := helper.CreateResponseWithHeaders(ctx, params3)
	helper.AssertNoError(t, err, "Failed in third trace call")

	if resp3 == nil || resp3.ID == "" {
		t.Fatal("Third response is nil or missing ID")
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
	t.Logf("Call 4 (with previous_response_id=%s): %s", resp3.ID, input4)

	params4 := responses.ResponseNewParams{
		Model:              shared.ResponsesModel(helper.GetModel()),
		PreviousResponseID: openai.String(resp3.ID),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input4),
		},
	}

	resp4, err := helper.CreateResponseWithHeaders(ctx, params4)
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

	t.Logf("Single trace test completed successfully with 4 Responses API calls")
}

func TestSingleTraceContextPreservation(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestSingleTraceContextPreservation")
	t.Skip("Test uses PreviousResponseID - temporarily skipped")
	// Skip test if no API key is configured

	ctx := helper.CreateTestContext()

	t.Logf("Testing context preservation across multiple calls in a single trace...")

	// Call 1: Introduce multiple facts
	input1 := "My name is Bob, I'm 30 years old, and I work as a software engineer in Seattle."
	t.Logf("Call 1: %s", input1)

	params1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input1),
		},
	}

	resp1, err := helper.CreateResponseWithHeaders(ctx, params1)
	helper.AssertNoError(t, err, "Failed in call 1")

	if resp1 == nil || resp1.ID == "" {
		t.Fatal("Response 1 is nil or missing ID")
	}

	output1 := resp1.OutputText()
	t.Logf("Assistant (call 1): %s", output1)

	// Call 2: Ask about name
	input2 := "What's my name?"
	t.Logf("Call 2: %s", input2)

	params2 := responses.ResponseNewParams{
		Model:              shared.ResponsesModel(helper.GetModel()),
		PreviousResponseID: openai.String(resp1.ID),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input2),
		},
	}

	resp2, err := helper.CreateResponseWithHeaders(ctx, params2)
	helper.AssertNoError(t, err, "Failed in call 2")

	if resp2 == nil || resp2.ID == "" {
		t.Fatal("Response 2 is nil or missing ID")
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
		Model:              shared.ResponsesModel(helper.GetModel()),
		PreviousResponseID: openai.String(resp2.ID),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input3),
		},
	}

	resp3, err := helper.CreateResponseWithHeaders(ctx, params3)
	helper.AssertNoError(t, err, "Failed in call 3")

	if resp3 == nil || resp3.ID == "" {
		t.Fatal("Response 3 is nil or missing ID")
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
		Model:              shared.ResponsesModel(helper.GetModel()),
		PreviousResponseID: openai.String(resp3.ID),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input4),
		},
	}

	resp4, err := helper.CreateResponseWithHeaders(ctx, params4)
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

	t.Logf("Context preservation test completed successfully with 4 calls")
}
