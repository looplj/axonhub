package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/looplj/axonhub/gemini_test/internal/testutil"
	"google.golang.org/genai"
)

func TestMain(m *testing.M) {
	// Set up any global test configuration here if needed
	code := m.Run()
	os.Exit(code)
}

func TestSimpleQA(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "qa_simple")
	// Note: genai.Client doesn't have a Close method

	// Print headers for debugging
	helper.PrintHeaders(t)

	// Create test context with headers
	ctx := helper.CreateTestContext()

	// Simple question to test basic chat completion
	question := "What is 2 + 2?"

	t.Logf("Sending question: %s", question)

	// Get the model
	modelName := helper.GetModel()

	// Prepare the generate content request
	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	},
	}

	// Make the API call
	config := helper.MergeHTTPOptions(nil)
	response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, config)
	helper.AssertNoError(t, err, "Failed to generate content")

	// Validate the response
	helper.ValidateChatResponse(t, response, "Simple Q&A")

	// Extract and log the response
	responseText := testutil.ExtractTextFromResponse(response)
	t.Logf("Response: %s", responseText)

	// Verify the answer makes sense (should contain "4" or "four")
	if !containsNumber(responseText) {
		t.Errorf("Expected response to contain a number, got: %s", responseText)
	}
}

func TestSimpleQAWithDifferentQuestion(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "qa_simple")
	// Note: genai.Client doesn't have a Close method

	// Test with a different question using the same configured model
	ctx := helper.CreateTestContext()
	question := "What is the capital of France?"

	t.Logf("Sending question: %s", question)

	// Get the model
	modelName := helper.GetModel()

	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	config := helper.MergeHTTPOptions(nil)
	response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, config)
	helper.AssertNoError(t, err, "Failed to generate content with capital question")

	helper.ValidateChatResponse(t, response, "Simple Q&A with capital question")

	responseText := testutil.ExtractTextFromResponse(response)
	t.Logf("Response: %s", responseText)

	// Verify the answer makes sense (should contain "Paris")
	if !testutil.ContainsCaseInsensitive(responseText, "Paris") {
		t.Errorf("Expected response to contain 'Paris', got: %s", responseText)
	}
}

func TestMultipleQuestions(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "qa_simple")
	// Note: genai.Client doesn't have a Close method

	ctx := helper.CreateTestContext()

	questions := []string{
		"What is the largest planet in our solar system?",
		"Who wrote Romeo and Juliet?",
		"What is the chemical symbol for gold?",
	}

	modelName := helper.GetModel()

	for i, question := range questions {
		t.Logf("Question %d: %s", i+1, question)

		contents := []*genai.Content{{
			Parts: []*genai.Part{{Text: question}},
		},
		}

		config := helper.MergeHTTPOptions(nil)
		response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, config)
		helper.AssertNoError(t, err, fmt.Sprintf("Failed on question %d", i+1))

		helper.ValidateChatResponse(t, response, fmt.Sprintf("Question %d", i+1))

		responseText := testutil.ExtractTextFromResponse(response)
		t.Logf("Answer %d: %s", i+1, responseText)
	}
}

func TestConversationHistory(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "qa_simple")
	// Note: genai.Client doesn't have a Close method

	ctx := helper.CreateTestContext()

	modelName := helper.GetModel()

	// Start a conversation
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
	chat, err := helper.Client.Chats.Create(ctx, modelName, config, nil)
	if err != nil {
		helper.AssertNoError(t, err, "Failed to create chat")
	}

	// First question
	question1 := "My name is Alice. What's your name?"
	t.Logf("Question 1: %s", question1)

	response1, err := chat.SendMessage(ctx, genai.Part{Text: question1})
	helper.AssertNoError(t, err, "Failed to send first message")
	helper.ValidateChatResponse(t, response1, "First message")

	responseText1 := testutil.ExtractTextFromResponse(response1)
	t.Logf("Response 1: %s", responseText1)

	// Follow-up question that references the previous context
	question2 := "What did I just tell you my name is?"
	t.Logf("Question 2: %s", question2)

	response2, err := chat.SendMessage(ctx, genai.Part{Text: question2})
	helper.AssertNoError(t, err, "Failed to send second message")
	helper.ValidateChatResponse(t, response2, "Second message")

	responseText2 := testutil.ExtractTextFromResponse(response2)
	t.Logf("Response 2: %s", responseText2)

	// Verify the model remembers the name
	if !testutil.ContainsAnyCaseInsensitive(responseText2, "Alice", "alice") {
		t.Errorf("Expected response to contain 'Alice', got: %s", responseText2)
	}
}

// Helper functions

func containsNumber(text string) bool {
	numbers := []string{"4", "four", "Four"}
	for _, num := range numbers {
		if testutil.ContainsCaseInsensitive(text, num) {
			return true
		}
	}
	return false
}
