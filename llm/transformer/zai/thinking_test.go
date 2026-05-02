package zai

import (
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openai"
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
			name:            "none reasoning effort (disabled)",
			reasoningEffort: "none",
			expectedType:    "disabled",
		},
		{
			name:            "unknown reasoning effort",
			reasoningEffort: "unknown",
			expectedType:    "enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatReq := &llm.Request{
				ReasoningEffort: tt.reasoningEffort,
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: &[]string{"Hello, world!"}[0],
						},
					},
				},
			}

			zaiReq := Request{}

			if chatReq.ReasoningEffort != "" {
				var thinkingType string
				switch chatReq.ReasoningEffort {
				case "none":
					thinkingType = "disabled"
				default:
					thinkingType = "enabled"
				}
				zaiReq.Thinking = &Thinking{
					Type: thinkingType,
				}
			}

			if zaiReq.Thinking == nil {
				t.Error("Expected Thinking to be non-nil when ReasoningEffort is set")
			}

			if zaiReq.Thinking.Type != tt.expectedType {
				t.Errorf("Expected Thinking.Type to be '%s', got %s", tt.expectedType, zaiReq.Thinking.Type)
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

	zaiReq := Request{}

	if chatReq.ReasoningEffort != "" {
		var thinkingType string
		switch chatReq.ReasoningEffort {
		case "none":
			thinkingType = "disabled"
		default:
			thinkingType = "enabled"
		}
		zaiReq.Thinking = &Thinking{
			Type: thinkingType,
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
		Request: *openai.RequestFromLLM(chatReq, openai.ReasoningFieldContent),
		UserID:  "test-user",
	}

	if chatReq.ReasoningEffort != "" {
		var thinkingType string
		switch chatReq.ReasoningEffort {
		case "none":
			thinkingType = "disabled"
		default:
			thinkingType = "enabled"
		}
		zaiReq.Thinking = &Thinking{
			Type: thinkingType,
		}
	}

	if zaiReq.Thinking != nil {
		t.Error("Expected Thinking to be nil when ReasoningEffort is not set")
	}
}
