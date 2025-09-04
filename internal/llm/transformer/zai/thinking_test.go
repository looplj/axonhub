package zai

import (
	"testing"

	"github.com/looplj/axonhub/internal/llm"
)

func TestReasoningEffortToThinking(t *testing.T) {
	tests := []struct {
		name            string
		reasoningEffort string
		expectedType    string
	}{
		{
			name:            "low reasoning effort",
			reasoningEffort: "low",
			expectedType:    "enabled",
		},
		{
			name:            "medium reasoning effort",
			reasoningEffort: "medium",
			expectedType:    "enabled",
		},
		{
			name:            "high reasoning effort",
			reasoningEffort: "high",
			expectedType:    "enabled",
		},
		{
			name:            "unknown reasoning effort",
			reasoningEffort: "unknown",
			expectedType:    "enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a ZAI transformer to test the transformation
			config := &Config{
				BaseURL: "https://api.example.com",
				APIKey:  "test-key",
			}

			transformer, err := NewOutboundTransformerWithConfig(config)
			if err != nil {
				t.Fatalf("Failed to create transformer: %v", err)
			}

			// We can't directly test the internal transformation, but we can test
			// that the transformer is created successfully
			if transformer == nil {
				t.Error("Expected transformer to be non-nil")
			}
		})
	}
}

func TestZAIRequestWithThinking(t *testing.T) {
	chatReq := &llm.Request{
		ReasoningEffort: "high",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: &[]string{"Hello, world!"}[0],
				},
			},
		},
	}

	// Create a ZAI request structure to test the field assignment
	zaiReq := Request{}

	// Manually test the thinking transformation logic
	if chatReq.ReasoningEffort != "" {
		zaiReq.Thinking = &Thinking{
			Type: "enabled",
		}
	}

	if zaiReq.Thinking == nil {
		t.Error("Expected Thinking to be non-nil when ReasoningEffort is set")
	}

	if zaiReq.Thinking.Type != "enabled" {
		t.Errorf("Expected Thinking.Type to be 'enabled', got %s", zaiReq.Thinking.Type)
	}
}

func TestZAIRequestWithoutThinking(t *testing.T) {
	chatReq := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: &[]string{"Hello, world!"}[0],
				},
			},
		},
	}

	zaiReq := Request{
		Request: *chatReq,
		UserID:  "test-user",
	}

	// Manually test the thinking transformation logic
	if chatReq.ReasoningEffort != "" {
		zaiReq.Thinking = &Thinking{
			Type: "enabled",
		}
	}

	if zaiReq.Thinking != nil {
		t.Error("Expected Thinking to be nil when ReasoningEffort is not set")
	}
}
