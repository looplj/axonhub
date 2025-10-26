package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
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

	// Simple question to test basic chat completion
	question := "What is 2 + 2?"

	t.Logf("Sending question: %s", question)

	// Prepare the chat completion request
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model: helper.GetModel(),
	}

	// Make the API call
	completion, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion")

	// Validate the response
	helper.ValidateChatResponse(t, completion, "Simple Q&A")

	// Log the response
	t.Logf("Response: %s", completion.Choices[0].Message.Content)

	// Verify the answer makes sense (should contain "4" or "four")
	response := completion.Choices[0].Message.Content
	if !containsNumber(response) {
		t.Errorf("Expected response to contain a number, got: %s", response)
	}
}

func TestSimpleQAWithDifferentQuestion(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Test with a different question using the same configured model
	ctx := helper.CreateTestContext()
	question := "What is the capital of France?"

	t.Logf("Sending question: %s", question)

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model: helper.GetModel(),
	}

	completion, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with capital question")

	helper.ValidateChatResponse(t, completion, "Simple Q&A with capital question")

	response := completion.Choices[0].Message.Content
	t.Logf("Response: %s", response)

	// Verify the answer makes sense (should contain "Paris")
	if !contains(response, "Paris") {
		t.Errorf("Expected response to contain 'Paris', got: %s", response)
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

		params := openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(question),
			},
			Model: helper.GetModel(),
		}

		completion, err := helper.Client.Chat.Completions.New(ctx, params)
		helper.AssertNoError(t, err, fmt.Sprintf("Failed on question %d", i+1))

		helper.ValidateChatResponse(t, completion, fmt.Sprintf("Question %d", i+1))

		response := completion.Choices[0].Message.Content
		t.Logf("Answer %d: %s", i+1, response)
	}
}

// Helper functions

func containsNumber(text string) bool {
	numbers := []string{"4", "four", "Four"}
	for _, num := range numbers {
		if contains(text, num) {
			return true
		}
	}
	return false
}

func contains(text, substring string) bool {
	return len(text) >= len(substring) &&
		   (text == substring ||
		    len(text) > len(substring) &&
		    (text[:len(substring)] == substring ||
		     text[len(text)-len(substring):] == substring ||
		     containsMiddle(text, substring)))
}

func containsMiddle(text, substring string) bool {
	for i := 0; i <= len(text)-len(substring); i++ {
		if text[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}
