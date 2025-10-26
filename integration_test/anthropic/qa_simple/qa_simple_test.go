package main

import (
	"fmt"
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

func TestSimpleQA(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Simple Q&A test
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hello! How are you today?")),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 1024,
	}

	response, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get simple Q&A response")

	helper.ValidateMessageResponse(t, response, "Simple Q&A test")

	// Validate response content
	responseText := ""
	for _, block := range response.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			responseText += textBlock.Text
		}
	}

	t.Logf("Response: %s", responseText)

	// Verify response is not empty and contains some greeting-like content
	if len(responseText) == 0 {
		t.Error("Expected non-empty response")
	}

	// Check for basic conversational response
	greetingTerms := []string{"hello", "hi", "greetings", "good", "well", "fine"}
	foundGreeting := false
	for _, term := range greetingTerms {
		if testutil.ContainsCaseInsensitive(responseText, term) {
			foundGreeting = true
			break
		}
	}

	if !foundGreeting {
		t.Errorf("Expected response to contain greeting terms, got: %s", responseText)
	}
}

func TestMultipleQuestions(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	questions := []string{
		"What is the capital of France?",
		"What is 2 + 2?",
		"Tell me a joke",
	}

	for i, question := range questions {
		t.Run(fmt.Sprintf("Question_%d", i+1), func(t *testing.T) {
			messages := []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
			}

			params := anthropic.MessageNewParams{
				Model:     helper.GetModel(),
				Messages:  messages,
				MaxTokens: 1024,
			}

			response, err := helper.Client.Messages.New(ctx, params)
			helper.AssertNoError(t, err, fmt.Sprintf("Failed on question %d", i+1))

			helper.ValidateMessageResponse(t, response, fmt.Sprintf("Question %d", i+1))

			// Validate response content
			responseText := ""
			for _, block := range response.Content {
				if textBlock := block.AsText(); textBlock.Text != "" {
					responseText += textBlock.Text
				}
			}

			t.Logf("Q%d: %s", i+1, question)
			t.Logf("A%d: %s", i+1, responseText)

			// Verify response is not empty
			if len(responseText) == 0 {
				t.Errorf("Expected non-empty response for question %d", i+1)
			}

			// Basic validation based on question type
			switch i {
			case 0: // Capital of France
				if !testutil.ContainsCaseInsensitive(responseText, "paris") {
					t.Errorf("Expected response to mention Paris, got: %s", responseText)
				}
			case 1: // Math question
				if !testutil.ContainsCaseInsensitive(responseText, "4") && !testutil.ContainsCaseInsensitive(responseText, "four") {
					t.Errorf("Expected response to mention 4, got: %s", responseText)
				}
			case 2: // Joke
				// Just check that it's not empty, jokes are subjective
				if len(responseText) < 10 {
					t.Errorf("Expected longer joke response, got: %s", responseText)
				}
			}
		})
	}
}

func TestQuestionWithContext(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Test with system context
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("I need help with my homework about animals.")),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 1024,
		System:    []anthropic.TextBlockParam{{Type: "text", Text: "You are a helpful tutor specializing in biology and animal science."}},
	}

	response, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get contextual response")

	helper.ValidateMessageResponse(t, response, "Contextual Q&A test")

	// Validate response content
	responseText := ""
	for _, block := range response.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			responseText += textBlock.Text
		}
	}

	t.Logf("Contextual response: %s", responseText)

	// Verify response acknowledges the context
	contextTerms := []string{"homework", "animals", "biology", "help", "tutor"}
	foundContext := false
	for _, term := range contextTerms {
		if testutil.ContainsCaseInsensitive(responseText, term) {
			foundContext = true
			break
		}
	}

	if !foundContext {
		t.Errorf("Expected response to acknowledge context, got: %s", responseText)
	}
}
