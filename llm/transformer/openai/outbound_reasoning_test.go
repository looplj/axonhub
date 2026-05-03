package openai

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/llm"
)

func TestMessageFromLLMWithConfig_ReasoningFieldContent(t *testing.T) {
	reasoningText := "This is my reasoning"
	llmMsg := llm.Message{
		Role:             "assistant",
		ReasoningContent: &reasoningText,
		Reasoning:        lo.ToPtr("This should be ignored"),
	}

	msg := MessageFromLLMWithConfig(llmMsg, ReasoningFieldContent)

	assert.Equal(t, "assistant", msg.Role)
	assert.NotNil(t, msg.ReasoningContent)
	assert.Equal(t, reasoningText, *msg.ReasoningContent)
	assert.Nil(t, msg.Reasoning, "Reasoning field should be nil when using ReasoningFieldContent")
}

func TestMessageFromLLMWithConfig_ReasoningFieldReasoning(t *testing.T) {
	reasoningText := "This is my reasoning"
	llmMsg := llm.Message{
		Role:             "assistant",
		ReasoningContent: lo.ToPtr("This should be ignored"),
		Reasoning:        &reasoningText,
	}

	msg := MessageFromLLMWithConfig(llmMsg, ReasoningFieldReasoning)

	assert.Equal(t, "assistant", msg.Role)
	assert.NotNil(t, msg.Reasoning)
	assert.Equal(t, reasoningText, *msg.Reasoning)
	assert.Nil(t, msg.ReasoningContent, "ReasoningContent field should be nil when using ReasoningFieldReasoning")
}

func TestMessageFromLLMWithConfig_ReasoningFieldNone(t *testing.T) {
	llmMsg := llm.Message{
		Role:             "assistant",
		ReasoningContent: lo.ToPtr("This should be stripped"),
		Reasoning:        lo.ToPtr("This should also be stripped"),
	}

	msg := MessageFromLLMWithConfig(llmMsg, ReasoningFieldNone)

	assert.Equal(t, "assistant", msg.Role)
	assert.Nil(t, msg.ReasoningContent, "ReasoningContent field should be nil when using ReasoningFieldNone")
	assert.Nil(t, msg.Reasoning, "Reasoning field should be nil when using ReasoningFieldNone")
}

func TestMessageFromLLMWithConfig_DefaultAll(t *testing.T) {
	reasoningContentText := "reasoning_content value"
	reasoningText := "reasoning value"
	llmMsg := llm.Message{
		Role:             "assistant",
		ReasoningContent: &reasoningContentText,
		Reasoning:        &reasoningText,
	}

	msg := MessageFromLLM(llmMsg)

	assert.Equal(t, "assistant", msg.Role)
	assert.NotNil(t, msg.ReasoningContent, "ReasoningContent should be preserved by default (ReasoningFieldAll)")
	assert.Equal(t, reasoningContentText, *msg.ReasoningContent)
	assert.NotNil(t, msg.Reasoning, "Reasoning should be preserved by default (ReasoningFieldAll)")
	assert.Equal(t, reasoningText, *msg.Reasoning)
}

func TestMessageFromLLMWithConfig_ReasoningFieldAll_SyncFromReasoning(t *testing.T) {
	reasoningText := "This is my reasoning"
	llmMsg := llm.Message{
		Role:      "assistant",
		Reasoning: &reasoningText,
	}

	msg := MessageFromLLMWithConfig(llmMsg, ReasoningFieldAll)

	assert.Equal(t, "assistant", msg.Role)
	assert.NotNil(t, msg.ReasoningContent, "ReasoningContent should be synced from Reasoning")
	assert.Equal(t, reasoningText, *msg.ReasoningContent)
	assert.NotNil(t, msg.Reasoning)
	assert.Equal(t, reasoningText, *msg.Reasoning)
}

func TestMessageFromLLMWithConfig_ReasoningFieldAll_SyncFromReasoningContent(t *testing.T) {
	reasoningContentText := "This is my reasoning content"
	llmMsg := llm.Message{
		Role:             "assistant",
		ReasoningContent: &reasoningContentText,
	}

	msg := MessageFromLLMWithConfig(llmMsg, ReasoningFieldAll)

	assert.Equal(t, "assistant", msg.Role)
	assert.NotNil(t, msg.ReasoningContent)
	assert.Equal(t, reasoningContentText, *msg.ReasoningContent)
	assert.NotNil(t, msg.Reasoning, "Reasoning should be synced from ReasoningContent")
	assert.Equal(t, reasoningContentText, *msg.Reasoning)
}

func TestMessageFromLLMWithConfig_ReasoningFieldAll_BothPresent(t *testing.T) {
	reasoningContentText := "reasoning_content value"
	reasoningText := "reasoning value"
	llmMsg := llm.Message{
		Role:             "assistant",
		ReasoningContent: &reasoningContentText,
		Reasoning:        &reasoningText,
	}

	msg := MessageFromLLMWithConfig(llmMsg, ReasoningFieldAll)

	assert.Equal(t, "assistant", msg.Role)
	assert.NotNil(t, msg.ReasoningContent)
	assert.Equal(t, reasoningContentText, *msg.ReasoningContent)
	assert.NotNil(t, msg.Reasoning)
	assert.Equal(t, reasoningText, *msg.Reasoning)
}

func TestMessageFromLLMWithConfig_ReasoningFieldAll_EmptyStringNotSynced(t *testing.T) {
	reasoningText := "This is my reasoning"
	llmMsg := llm.Message{
		Role:             "assistant",
		ReasoningContent: lo.ToPtr(""),
		Reasoning:        &reasoningText,
	}

	msg := MessageFromLLMWithConfig(llmMsg, ReasoningFieldAll)

	assert.Equal(t, "assistant", msg.Role)
	assert.NotNil(t, msg.ReasoningContent)
	assert.Equal(t, "", *msg.ReasoningContent, "Empty ReasoningContent should not be overwritten")
	assert.NotNil(t, msg.Reasoning)
	assert.Equal(t, reasoningText, *msg.Reasoning)
}

func TestMessageFromLLMWithConfig_FallbackToReasoning(t *testing.T) {
	reasoningText := "This is my reasoning"
	llmMsg := llm.Message{
		Role:      "assistant",
		Reasoning: &reasoningText,
	}

	msg := MessageFromLLMWithConfig(llmMsg, ReasoningFieldContent)

	assert.Equal(t, "assistant", msg.Role)
	assert.NotNil(t, msg.ReasoningContent)
	assert.Equal(t, reasoningText, *msg.ReasoningContent, "Should fallback to Reasoning field when ReasoningContent is nil")
	assert.Nil(t, msg.Reasoning, "Reasoning field should be nil when using ReasoningFieldContent")
}

func TestRequestFromLLMWithConfig_ReasoningFieldContent(t *testing.T) {
	reasoningText := "This is my reasoning"
	llmReq := &llm.Request{
		Model: "test-model",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
			},
			{
				Role:             "assistant",
				ReasoningContent: &reasoningText,
				Reasoning:        lo.ToPtr("This should be ignored"),
			},
		},
	}

	req := RequestFromLLM(llmReq, ReasoningFieldContent)

	assert.Equal(t, "test-model", req.Model)
	assert.Len(t, req.Messages, 2)

	userMsg := req.Messages[0]
	assert.Equal(t, "user", userMsg.Role)
	assert.Nil(t, userMsg.ReasoningContent)
	assert.Nil(t, userMsg.Reasoning)

	assistantMsg := req.Messages[1]
	assert.Equal(t, "assistant", assistantMsg.Role)
	assert.NotNil(t, assistantMsg.ReasoningContent)
	assert.Equal(t, reasoningText, *assistantMsg.ReasoningContent)
	assert.Nil(t, assistantMsg.Reasoning, "Reasoning field should be nil when using ReasoningFieldContent")
}

func TestRequestFromLLMWithConfig_ReasoningFieldReasoning(t *testing.T) {
	reasoningText := "This is my reasoning"
	llmReq := &llm.Request{
		Model: "test-model",
		Messages: []llm.Message{
			{
				Role:             "assistant",
				ReasoningContent: lo.ToPtr("This should be ignored"),
				Reasoning:        &reasoningText,
			},
		},
	}

	req := RequestFromLLM(llmReq, ReasoningFieldReasoning)

	assert.Len(t, req.Messages, 1)

	assistantMsg := req.Messages[0]
	assert.Equal(t, "assistant", assistantMsg.Role)
	assert.NotNil(t, assistantMsg.Reasoning)
	assert.Equal(t, reasoningText, *assistantMsg.Reasoning)
	assert.Nil(t, assistantMsg.ReasoningContent, "ReasoningContent field should be nil when using ReasoningFieldReasoning")
}

func TestRequestFromLLMWithConfig_ReasoningFieldNone(t *testing.T) {
	llmReq := &llm.Request{
		Model: "test-model",
		Messages: []llm.Message{
			{
				Role:             "assistant",
				ReasoningContent: lo.ToPtr("This should be stripped"),
				Reasoning:        lo.ToPtr("This should also be stripped"),
			},
		},
	}

	req := RequestFromLLM(llmReq, ReasoningFieldNone)

	assert.Len(t, req.Messages, 1)

	assistantMsg := req.Messages[0]
	assert.Equal(t, "assistant", assistantMsg.Role)
	assert.Nil(t, assistantMsg.ReasoningContent, "ReasoningContent field should be nil when using ReasoningFieldNone")
	assert.Nil(t, assistantMsg.Reasoning, "Reasoning field should be nil when using ReasoningFieldNone")
}

func TestRequestFromLLM_DefaultNone(t *testing.T) {
	reasoningText := "This is my reasoning"
	llmReq := &llm.Request{
		Model: "test-model",
		Messages: []llm.Message{
			{
				Role:             "assistant",
				ReasoningContent: &reasoningText,
				Reasoning:        lo.ToPtr("Another reasoning"),
			},
		},
	}

	req := RequestFromLLM(llmReq, ReasoningFieldNone)

	assert.Len(t, req.Messages, 1)

	assistantMsg := req.Messages[0]
	assert.Equal(t, "assistant", assistantMsg.Role)
	assert.Nil(t, assistantMsg.ReasoningContent, "ReasoningContent should be nil by default (ReasoningFieldNone)")
	assert.Nil(t, assistantMsg.Reasoning, "Reasoning should be nil by default (ReasoningFieldNone)")
}
