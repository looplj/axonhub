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

func TestSingleThreadMultipleTracesStateless(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "TestSingleThreadMultipleTracesStateless")

	// Print headers for debugging
	helper.PrintHeaders(t)

	// Get current thread ID for this test
	currentThreadID := helper.Config.ThreadID
	t.Logf("Using thread ID: %s", currentThreadID)

	t.Logf("Starting single thread with multiple traces using stateless Responses API...")

	// Trace 1: Project planning
	t.Logf("=== Starting Trace 1: Project Planning ===")
	ctx1 := helper.CreateTestContext()

	input1_1 := "I need to plan a software development project. What are the key phases?"
	t.Logf("Trace 1, Call 1: %s", input1_1)

	params1_1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1_1),
			},
		},
	}

	resp1_1, err := helper.CreateResponseWithHeaders(ctx1, params1_1)
	helper.AssertNoError(t, err, "Failed in trace 1, call 1")

	if resp1_1 == nil {
		t.Fatal("Trace 1, Call 1 response is nil")
	}

	output1_1 := resp1_1.OutputText()
	t.Logf("Trace 1, Call 1 - Assistant: %s", output1_1)

	// Continue trace 1 with follow-up using stateless history
	input1_2 := "What tools and technologies should I consider for each phase?"
	t.Logf("Trace 1, Call 2 (stateless): %s", input1_2)

	params1_2 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1_1),
				createAssistantMessage(output1_1),
				createUserMessage(input1_2),
			},
		},
	}

	resp1_2, err := helper.CreateResponseWithHeaders(ctx1, params1_2)
	helper.AssertNoError(t, err, "Failed in trace 1, call 2")

	if resp1_2 == nil {
		t.Fatal("Trace 1, Call 2 response is nil")
	}

	output1_2 := resp1_2.OutputText()
	t.Logf("Trace 1, Call 2 - Assistant: %s", output1_2)

	// Trace 2: Different topic but same thread
	t.Logf("=== Starting Trace 2: Team Management ===")

	// Create new trace but same thread using helper function
	helper2 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)
	ctx2 := helper2.CreateTestContext()

	input2_1 := "For the development team, how should I structure the team roles?"
	t.Logf("Trace 2, Call 1: %s", input2_1)

	params2_1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper2.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input2_1),
			},
		},
	}

	resp2_1, err := helper2.CreateResponseWithHeaders(ctx2, params2_1)
	helper.AssertNoError(t, err, "Failed in trace 2, call 1")

	if resp2_1 == nil {
		t.Fatal("Trace 2, Call 1 response is nil")
	}

	output2_1 := resp2_1.OutputText()
	t.Logf("Trace 2, Call 1 - Assistant: %s", output2_1)

	// Continue trace 2 using stateless history
	input2_2 := "What about the project timeline and milestones?"
	t.Logf("Trace 2, Call 2 (stateless): %s", input2_2)

	params2_2 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper2.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input2_1),
				createAssistantMessage(output2_1),
				createUserMessage(input2_2),
			},
		},
	}

	resp2_2, err := helper2.CreateResponseWithHeaders(ctx2, params2_2)
	helper.AssertNoError(t, err, "Failed in trace 2, call 2")

	if resp2_2 == nil {
		t.Fatal("Trace 2, Call 2 response is nil")
	}

	output2_2 := resp2_2.OutputText()
	t.Logf("Trace 2, Call 2 - Assistant: %s", output2_2)

	// Trace 3: Resource planning in same thread
	t.Logf("=== Starting Trace 3: Resource Planning ===")

	helper3 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)
	ctx3 := helper3.CreateTestContext()

	input3_1 := "I need to estimate the project costs. If I have a team of 5 engineers working for 6 months at $100/hour, what's the estimated cost?"
	t.Logf("Trace 3, Call 1: %s", input3_1)

	params3_1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper3.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input3_1),
			},
		},
	}

	resp3_1, err := helper3.CreateResponseWithHeaders(ctx3, params3_1)
	helper.AssertNoError(t, err, "Failed in trace 3, call 1")

	if resp3_1 == nil {
		t.Fatal("Trace 3, Call 1 response is nil")
	}

	output3_1 := resp3_1.OutputText()
	t.Logf("Trace 3, Call 1 - Assistant: %s", output3_1)

	// Follow up on cost estimate using stateless history
	input3_2 := "Based on this estimate, how should I adjust the project scope if the budget is limited?"
	t.Logf("Trace 3, Call 2 (stateless): %s", input3_2)

	params3_2 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper3.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input3_1),
				createAssistantMessage(output3_1),
				createUserMessage(input3_2),
			},
		},
	}

	resp3_2, err := helper3.CreateResponseWithHeaders(ctx3, params3_2)
	helper.AssertNoError(t, err, "Failed in trace 3, call 2")

	if resp3_2 == nil {
		t.Fatal("Trace 3, Call 2 response is nil")
	}

	output3_2 := resp3_2.OutputText()
	t.Logf("Trace 3, Call 2 - Assistant: %s", output3_2)

	t.Logf("Single thread stateless test completed successfully with 3 traces and 6 total Responses API calls")

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

func TestSingleThreadTraceIsolationStateless(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "TestSingleThreadTraceIsolationStateless")

	currentThreadID := helper.Config.ThreadID
	t.Logf("Testing trace isolation within thread %s (stateless)", currentThreadID)

	// Trace 1: Introduce fact about Alice
	t.Logf("=== Trace 1: Alice ===")
	ctx1 := helper.CreateTestContext()

	input1 := "My name is Alice and I love programming."
	t.Logf("Trace 1: %s", input1)

	params1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
			},
		},
	}

	resp1, err := helper.CreateResponseWithHeaders(ctx1, params1)
	helper.AssertNoError(t, err, "Failed in trace 1")

	if resp1 == nil {
		t.Fatal("Trace 1 response is nil")
	}

	output1 := resp1.OutputText()
	t.Logf("Trace 1 - Assistant: %s", output1)

	// Trace 2: Different context (Bob), should NOT know about Alice
	t.Logf("=== Trace 2: Bob (New Trace) ===")

	helper2 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)
	ctx2 := helper2.CreateTestContext()

	input2 := "My name is Bob and I love cooking."
	t.Logf("Trace 2: %s", input2)

	params2 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper2.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input2),
			},
		},
	}

	resp2, err := helper2.CreateResponseWithHeaders(ctx2, params2)
	helper.AssertNoError(t, err, "Failed in trace 2")

	if resp2 == nil {
		t.Fatal("Trace 2 response is nil")
	}

	output2 := resp2.OutputText()
	t.Logf("Trace 2 - Assistant: %s", output2)

	// Verify Trace 2 context isolation - ask about name using stateless history
	input2_2 := "What's my name?"
	t.Logf("Trace 2, Follow-up (stateless): %s", input2_2)

	params2_2 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper2.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input2),
				createAssistantMessage(output2),
				createUserMessage(input2_2),
			},
		},
	}

	resp2_2, err := helper2.CreateResponseWithHeaders(ctx2, params2_2)
	helper.AssertNoError(t, err, "Failed in trace 2 follow-up")

	if resp2_2 == nil {
		t.Fatal("Trace 2 follow-up response is nil")
	}

	output2_2 := resp2_2.OutputText()
	t.Logf("Trace 2 Follow-up - Assistant: %s", output2_2)

	// Should recall "Bob", not "Alice"
	if testutil.ContainsCaseInsensitive(output2_2, "alice") {
		t.Errorf("Trace isolation violated: Trace 2 should not know about Alice, but got: %s", output2_2)
	}

	if !testutil.ContainsCaseInsensitive(output2_2, "bob") {
		t.Errorf("Expected Trace 2 to recall 'Bob', got: %s", output2_2)
	}

	t.Logf("Trace isolation verified: traces maintain separate contexts (stateless)")
}
