package main

import (
	"os"
	"strings"
	"testing"

	"github.com/looplj/axonhub/gemini_test/internal/testutil"
	"google.golang.org/genai"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestGenerateContentWithThinkingConfig(t *testing.T) {
	helper := testutil.NewTestHelper(t)

	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	question := "What is 25 * 47? Please show your reasoning."

	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	config := &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingLevel:   genai.ThinkingLevelHigh,
		},
	}

	response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, config)
	if err != nil {
		if isThinkingNotSupportedErr(err) {
			t.Skipf("Skipping because model does not support thinking: %v", err)
		}
		helper.AssertNoError(t, err, "Failed to generate content with thinking config")
	}

	helper.ValidateChatResponse(t, response, "Thinking config")

	nonThoughtText := extractNonThoughtText(response)
	if nonThoughtText == "" {
		t.Fatalf("Expected non-thought text output")
	}

	if !testutil.ContainsAnyCaseInsensitive(nonThoughtText, "1175", "1,175") {
		t.Fatalf("Expected answer 1175 in non-thought text, got: %s", nonThoughtText)
	}

	thoughtParts := collectThoughtParts(response)
	thoughtTokens := int64(0)
	if response.UsageMetadata != nil {
		thoughtTokens = int64(response.UsageMetadata.ThoughtsTokenCount)
	}

	if len(thoughtParts) == 0 && thoughtTokens == 0 {
		t.Fatalf("Expected thoughts in response when IncludeThoughts=true; got thoughtsParts=%d thoughtsTokenCount=%d", len(thoughtParts), thoughtTokens)
	}
}

func TestGenerateContentWithThinkingBudget(t *testing.T) {
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	question := "Solve 123 + 456. Please show your reasoning."

	contents := []*genai.Content{{
		Parts: []*genai.Part{{Text: question}},
	}}

	config := &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  genai.Ptr[int32](1024),
		},
	}

	response, err := helper.Client.Models.GenerateContent(ctx, modelName, contents, config)
	if err != nil {
		if isThinkingNotSupportedErr(err) {
			t.Skipf("Skipping because model does not support thinking: %v", err)
		}
		helper.AssertNoError(t, err, "Failed to generate content with thinking budget")
	}

	helper.ValidateChatResponse(t, response, "Thinking budget")

	nonThoughtText := extractNonThoughtText(response)
	if nonThoughtText == "" {
		t.Fatalf("Expected non-thought text output")
	}

	if !testutil.ContainsAnyCaseInsensitive(nonThoughtText, "579") {
		t.Fatalf("Expected answer 579 in non-thought text, got: %s", nonThoughtText)
	}

	thoughtParts := collectThoughtParts(response)
	thoughtTokens := int64(0)
	if response.UsageMetadata != nil {
		thoughtTokens = int64(response.UsageMetadata.ThoughtsTokenCount)
	}

	if len(thoughtParts) == 0 && thoughtTokens == 0 {
		t.Fatalf("Expected thoughts in response when IncludeThoughts=true; got thoughtsParts=%d thoughtsTokenCount=%d", len(thoughtParts), thoughtTokens)
	}
}

func TestChatWithThinkingConfig(t *testing.T) {
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()
	modelName := helper.GetModel()

	config := &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingLevel:   genai.ThinkingLevelHigh,
		},
	}

	chat, err := helper.Client.Chats.Create(ctx, modelName, config, nil)
	if err != nil {
		if isThinkingNotSupportedErr(err) {
			t.Skipf("Skipping because model does not support thinking: %v", err)
		}
		helper.AssertNoError(t, err, "Failed to create chat with thinking")
	}

	question := "What is 12 * 12? Please show your reasoning."
	response, err := chat.SendMessage(ctx, genai.Part{Text: question})
	if err != nil {
		if isThinkingNotSupportedErr(err) {
			t.Skipf("Skipping because model does not support thinking: %v", err)
		}
		helper.AssertNoError(t, err, "Failed to send chat message with thinking")
	}

	helper.ValidateChatResponse(t, response, "Chat thinking config")

	nonThoughtText := extractNonThoughtText(response)
	if nonThoughtText == "" {
		t.Fatalf("Expected non-thought text output")
	}

	if !testutil.ContainsAnyCaseInsensitive(nonThoughtText, "144") {
		t.Fatalf("Expected answer 144 in non-thought text, got: %s", nonThoughtText)
	}

	thoughtParts := collectThoughtParts(response)
	thoughtTokens := int64(0)
	if response.UsageMetadata != nil {
		thoughtTokens = int64(response.UsageMetadata.ThoughtsTokenCount)
	}

	if len(thoughtParts) == 0 && thoughtTokens == 0 {
		t.Fatalf("Expected thoughts in response when IncludeThoughts=true; got thoughtsParts=%d thoughtsTokenCount=%d", len(thoughtParts), thoughtTokens)
	}
}

func extractNonThoughtText(response *genai.GenerateContentResponse) string {
	if response == nil || len(response.Candidates) == 0 {
		return ""
	}

	candidate := response.Candidates[0]
	if candidate.Content == nil {
		return ""
	}

	var b strings.Builder
	for _, part := range candidate.Content.Parts {
		if part == nil {
			continue
		}
		if part.Thought {
			continue
		}
		if part.Text == "" {
			continue
		}
		b.WriteString(part.Text)
	}
	return b.String()
}

func collectThoughtParts(response *genai.GenerateContentResponse) []*genai.Part {
	if response == nil || len(response.Candidates) == 0 {
		return nil
	}

	candidate := response.Candidates[0]
	if candidate.Content == nil {
		return nil
	}

	parts := make([]*genai.Part, 0)
	for _, part := range candidate.Content.Parts {
		if part == nil {
			continue
		}
		if part.Thought {
			parts = append(parts, part)
		}
	}
	return parts
}

func isThinkingNotSupportedErr(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "thinking") && !strings.Contains(msg, "thought") {
		return false
	}

	return strings.Contains(msg, "not support") ||
		strings.Contains(msg, "doesn't support") ||
		strings.Contains(msg, "does not support") ||
		strings.Contains(msg, "unsupported")
}
