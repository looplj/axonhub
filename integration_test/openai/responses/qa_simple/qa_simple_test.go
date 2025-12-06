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

func TestSimpleQA(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	// Create test context with headers
	ctx := helper.CreateTestContext()

	// Simple question to test basic responses
	question := "What is 2 + 2?"

	t.Logf("Sending question: %s", question)

	// Prepare the responses request
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
	}

	// Make the API call
	resp, err := helper.Client.Responses.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get response")

	// Validate the response
	if resp == nil {
		t.Fatal("Response is nil")
	}

	output := resp.OutputText()
	t.Logf("Response: %s", output)

	if output == "" {
		t.Fatal("Expected non-empty output")
	}

	// Verify the answer makes sense (should contain "4" or "four")
	if !testutil.ContainsAnyCaseInsensitive(output, "4", "four") {
		t.Errorf("Expected response to contain '4' or 'four', got: %s", output)
	}
}

func TestSimpleQAWithDifferentQuestion(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Test with a different question using the same configured model
	ctx := helper.CreateTestContext()
	question := "What is the capital of France?"

	t.Logf("Sending question: %s", question)

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
	}

	resp, err := helper.Client.Responses.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get response with capital question")

	if resp == nil {
		t.Fatal("Response is nil")
	}

	output := resp.OutputText()
	t.Logf("Response: %s", output)

	if output == "" {
		t.Fatal("Expected non-empty output")
	}

	// Verify the answer makes sense (should contain "Paris")
	if !testutil.ContainsCaseInsensitive(output, "paris") {
		t.Errorf("Expected response to contain 'Paris', got: %s", output)
	}
}

func TestMultipleQuestions(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	questions := []string{
		"What is the largest planet in our solar system?",
		"Who wrote Romeo and Juliet?",
		"What is the chemical symbol for gold?",
	}

	for i, question := range questions {
		t.Logf("Question %d: %s", i+1, question)

		params := responses.ResponseNewParams{
			Model: shared.ResponsesModel(helper.GetModel()),
			Input: responses.ResponseNewParamsInputUnion{
				OfString: openai.String(question),
			},
		}

		resp, err := helper.Client.Responses.New(ctx, params)
		helper.AssertNoError(t, err, "Failed on question", i+1)

		if resp == nil {
			t.Fatalf("Response is nil for question %d", i+1)
		}

		output := resp.OutputText()
		t.Logf("Answer %d: %s", i+1, output)

		if output == "" {
			t.Errorf("Expected non-empty output for question %d", i+1)
		}
	}
}
