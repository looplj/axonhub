package main

import (
	"os"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

// TestExtraBodyWithThinkingConfig tests passing extra_body with thinking configuration
func TestExtraBodyWithThinkingConfig(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestExtraBodyWithThinkingConfig")
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()
	question := "What is 25 * 47? Please show your reasoning."

	// Create extra_body with thinking config for Gemini
	extraBody := map[string]interface{}{
		"google": map[string]interface{}{
			"thinking_config": map[string]interface{}{
				"include_thoughts": true,
				"thinking_level":   "high",
			},
		},
	}

	// Create client with extra body support
	client := helper.Client

	// Prepare the chat completion request
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model: helper.GetModel(),
	}
	params.SetExtraFields(map[string]any{"extra_body": extraBody})

	t.Logf("Sending question with extra_body thinking config: %s", question)

	// Make the API call
	completion, err := client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with extra_body")

	// Validate the response
	helper.ValidateChatResponse(t, completion, "Extra body thinking config")

	response := completion.Choices[0].Message.Content
	t.Logf("Response: %s", response)

	// Verify the answer makes sense (should contain "1175" or "1,175")
	if !testutil.ContainsAnyCaseInsensitive(response, "1175", "1,175") {
		t.Errorf("Expected response to contain 1175, got: %s", response)
	}

	// Check if thinking was included in the response (if supported)
	// This would depend on how Gemini returns thinking content
	if completion.Usage.PromptTokens > 0 || completion.Usage.CompletionTokens > 0 {
		t.Logf("Usage - Prompt tokens: %d, Completion tokens: %d, Total tokens: %d",
			completion.Usage.PromptTokens, completion.Usage.CompletionTokens, completion.Usage.TotalTokens)
	}
}

// TestExtraBodyWithThinkingBudget tests passing extra_body with thinking budget
func TestExtraBodyWithThinkingBudget(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestExtraBodyWithThinkingBudget")

	ctx := helper.CreateTestContext()
	question := "Solve 123 + 456. Please show your reasoning."

	// Create extra_body with thinking budget for Gemini 2.5 models
	extraBody := map[string]interface{}{
		"google": map[string]interface{}{
			"thinking_config": map[string]interface{}{
				"include_thoughts": true,
				"thinking_budget":  1024, // Low budget for Gemini 2.5
			},
		},
	}

	client := helper.Client

	// Prepare chat completion request
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model: helper.GetModel(),
	}
	params.SetExtraFields(map[string]any{"extra_body": extraBody})

	t.Logf("Sending question with thinking budget: %s", question)

	// Make API call
	completion, err := client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with thinking budget")

	// Validate the response
	helper.ValidateChatResponse(t, completion, "Extra body thinking budget")

	response := completion.Choices[0].Message.Content
	t.Logf("Response: %s", response)

	// Verify the answer makes sense (should contain "579")
	if !testutil.ContainsAnyCaseInsensitive(response, "579") {
		t.Errorf("Expected response to contain 579, got: %s", response)
	}
}

// TestExtraBodyWithStringThinkingBudget tests passing extra_body with string thinking budget
func TestExtraBodyWithStringThinkingBudget(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestExtraBodyWithStringThinkingBudget")

	ctx := helper.CreateTestContext()
	question := "What is 12 * 12? Please show your reasoning."

	// Create extra_body with string thinking budget for Gemini 3 models
	extraBody := map[string]interface{}{
		"google": map[string]interface{}{
			"thinking_config": map[string]interface{}{
				"include_thoughts": true,
				"thinking_budget":  "high", // String budget for Gemini 3
			},
		},
	}

	client := helper.Client

	// Prepare chat completion request
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model: helper.GetModel(),
	}
	params.SetExtraFields(map[string]any{"extra_body": extraBody})

	t.Logf("Sending question with string thinking budget: %s", question)

	// Make API call
	completion, err := client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with string thinking budget")

	// Validate the response
	helper.ValidateChatResponse(t, completion, "Extra body string thinking budget")

	response := completion.Choices[0].Message.Content
	t.Logf("Response: %s", response)

	// Verify the answer makes sense (should contain "144")
	if !testutil.ContainsAnyCaseInsensitive(response, "144") {
		t.Errorf("Expected response to contain 144, got: %s", response)
	}
}

// TestExtraBodyWithMinimalThinkingConfig tests passing extra_body with minimal thinking configuration
func TestExtraBodyWithMinimalThinkingConfig(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestExtraBodyWithMinimalThinkingConfig")

	ctx := helper.CreateTestContext()
	question := "What is 7 * 8? Please show your reasoning."

	// Create extra_body with minimal thinking config
	extraBody := map[string]interface{}{
		"google": map[string]interface{}{
			"thinking_config": map[string]interface{}{
				"include_thoughts": true,
				"thinking_level":   "minimal",
			},
		},
	}

	client := helper.Client

	// Prepare the chat completion request
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model: helper.GetModel(),
	}
	params.SetExtraFields(map[string]any{"extra_body": extraBody})

	t.Logf("Sending question with minimal thinking config: %s", question)

	// Make the API call
	completion, err := client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion with minimal thinking")

	// Validate the response
	helper.ValidateChatResponse(t, completion, "Extra body minimal thinking")

	response := completion.Choices[0].Message.Content
	t.Logf("Response: %s", response)

	// Verify the answer makes sense (should contain "56")
	if !testutil.ContainsAnyCaseInsensitive(response, "56") {
		t.Errorf("Expected response to contain 56, got: %s", response)
	}
}

// TestExtraBodyWithoutThinkingConfig tests passing extra_body without thinking configuration
func TestExtraBodyWithoutThinkingConfig(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestExtraBodyWithoutThinkingConfig")

	ctx := helper.CreateTestContext()
	question := "What is the capital of France?"

	// Create extra_body without thinking config
	extraBody := map[string]interface{}{
		"google": map[string]interface{}{
			// Empty google config
		},
	}

	client := helper.Client

	// Prepare the chat completion request
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model: helper.GetModel(),
	}
	params.SetExtraFields(map[string]any{"extra_body": extraBody})

	t.Logf("Sending question without thinking config: %s", question)

	// Make the API call
	completion, err := client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to get chat completion without thinking")

	// Validate the response
	helper.ValidateChatResponse(t, completion, "Extra body without thinking")

	response := completion.Choices[0].Message.Content
	t.Logf("Response: %s", response)

	// Verify the answer makes sense (should contain "Paris")
	if !testutil.ContainsAnyCaseInsensitive(response, "Paris") {
		t.Errorf("Expected response to contain 'Paris', got: %s", response)
	}
}

// TestExtraBodyWithInvalidThinkingConfig tests error handling with invalid thinking configuration
func TestExtraBodyWithInvalidThinkingConfig(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestExtraBodyWithInvalidThinkingConfig")

	ctx := helper.CreateTestContext()
	question := "What is 2 + 2?"

	// Create extra_body with invalid thinking config
	extraBody := map[string]interface{}{
		"google": map[string]interface{}{
			"thinking_config": map[string]interface{}{
				"include_thoughts": "invalid", // Should be boolean
				"thinking_budget":  "invalid", // Should be int or "low"/"high"
			},
		},
	}

	client := helper.Client

	// Prepare the chat completion request with invalid extra body
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Model: helper.GetModel(),
	}
	params.SetExtraFields(map[string]any{"extra_body": extraBody})

	t.Logf("Sending question with invalid thinking config: %s", question)

	// Make the API call - should handle invalid config gracefully
	completion, err := client.Chat.Completions.New(ctx, params)

	// Either the request succeeds (server handles invalid config) or fails gracefully
	if err != nil {
		t.Logf("Expected error with invalid config: %v", err)
		// This is acceptable - server should reject invalid config
	} else {
		// If it succeeds, validate the response
		helper.ValidateChatResponse(t, completion, "Invalid thinking config handled gracefully")
		response := completion.Choices[0].Message.Content
		t.Logf("Response despite invalid config: %s", response)
	}
}
