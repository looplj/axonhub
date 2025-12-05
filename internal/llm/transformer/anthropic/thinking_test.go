package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestConvertToChatCompletionResponse_WithThinking(t *testing.T) {
	const (
		thinking  = "Chain-of-thought reasoning"
		signature = "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds"
		answer    = "Here is the final answer."
	)

	anthropicResp := &Message{
		ID:   "msg_thinking",
		Type: "message",
		Role: "assistant",
		Content: []MessageContentBlock{
			{Type: "thinking", Thinking: thinking, Signature: signature},
			{Type: "text", Text: answer},
		},
		Model: "claude-3-sonnet-20240229",
	}

	result := convertToLlmResponse(anthropicResp, PlatformDirect)

	require.NotNil(t, result)
	require.Len(t, result.Choices, 1)
	require.NotNil(t, result.Choices[0].Message.ReasoningContent)
	require.Equal(t, thinking, *result.Choices[0].Message.ReasoningContent)
	require.NotNil(t, result.Choices[0].Message.ReasoningSignature)
	require.Equal(t, signature, *result.Choices[0].Message.ReasoningSignature)
	require.NotNil(t, result.Choices[0].Message.Content.Content)
	require.Equal(t, answer, *result.Choices[0].Message.Content.Content)
	require.Empty(t, result.Choices[0].Message.Content.MultipleContent)
}

func TestReasoningEffortToThinking(t *testing.T) {
	tests := []struct {
		name            string
		reasoningEffort string
		expectedType    string
		expectedBudget  int64
		config          *Config
	}{
		{
			name:            "low reasoning effort",
			reasoningEffort: "low",
			expectedType:    "enabled",
			expectedBudget:  5000,
			config:          nil,
		},
		{
			name:            "medium reasoning effort",
			reasoningEffort: "medium",
			expectedType:    "enabled",
			expectedBudget:  15000,
			config:          nil,
		},
		{
			name:            "high reasoning effort",
			reasoningEffort: "high",
			expectedType:    "enabled",
			expectedBudget:  30000,
			config:          nil,
		},
		{
			name:            "custom mapping",
			reasoningEffort: "high",
			expectedType:    "enabled",
			expectedBudget:  50000,
			config: &Config{
				ReasoningEffortToBudget: map[string]int64{
					"low":    3000,
					"medium": 10000,
					"high":   50000,
				},
			},
		},
		{
			name:            "unknown reasoning effort",
			reasoningEffort: "unknown",
			expectedType:    "enabled",
			expectedBudget:  15000,
			config:          nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatReq := &llm.Request{
				Model:           "claude-3-sonnet-20240229",
				ReasoningEffort: tt.reasoningEffort,
			}

			anthropicReq := convertToAnthropicRequestWithConfig(chatReq, tt.config)

			if anthropicReq.Thinking == nil {
				t.Errorf("Expected Thinking to be non-nil")
				return
			}

			if anthropicReq.Thinking.Type != tt.expectedType {
				t.Errorf("Expected Thinking.Type to be %s, got %s", tt.expectedType, anthropicReq.Thinking.Type)
			}

			if anthropicReq.Thinking.BudgetTokens != tt.expectedBudget {
				t.Errorf("Expected Thinking.BudgetTokens to be %d, got %d", tt.expectedBudget, anthropicReq.Thinking.BudgetTokens)
			}
		})
	}
}

func TestNoReasoningEffort(t *testing.T) {
	chatReq := &llm.Request{
		Model: "claude-3-sonnet-20240229",
	}

	anthropicReq := convertToAnthropicRequestWithConfig(chatReq, nil)

	if anthropicReq.Thinking != nil {
		t.Errorf("Expected Thinking to be nil when ReasoningEffort is not set")
	}
}

func TestReasoningBudgetPriority(t *testing.T) {
	tests := []struct {
		name            string
		reasoningEffort string
		reasoningBudget *int64
		config          *Config
		expectedBudget  int64
	}{
		{
			name:            "reasoning budget takes priority over config mapping",
			reasoningEffort: "medium",
			reasoningBudget: lo.ToPtr(int64(25000)),
			config: &Config{
				ReasoningEffortToBudget: map[string]int64{
					"medium": 15000,
				},
			},
			expectedBudget: 25000,
		},
		{
			name:            "reasoning budget takes priority over default mapping",
			reasoningEffort: "high",
			reasoningBudget: lo.ToPtr(int64(35000)),
			config:          nil,
			expectedBudget:  35000,
		},
		{
			name:            "fallback to config mapping when reasoning budget is nil",
			reasoningEffort: "low",
			reasoningBudget: nil,
			config: &Config{
				ReasoningEffortToBudget: map[string]int64{
					"low": 3000,
				},
			},
			expectedBudget: 3000,
		},
		{
			name:            "fallback to default mapping when reasoning budget is nil and no config",
			reasoningEffort: "medium",
			reasoningBudget: nil,
			config:          nil,
			expectedBudget:  15000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatReq := &llm.Request{
				Model:           "claude-3-sonnet-20240229",
				ReasoningEffort: tt.reasoningEffort,
				ReasoningBudget: tt.reasoningBudget,
			}

			anthropicReq := convertToAnthropicRequestWithConfig(chatReq, tt.config)

			if anthropicReq.Thinking == nil {
				t.Errorf("Expected Thinking to be non-nil")
				return
			}

			if anthropicReq.Thinking.Type != "enabled" {
				t.Errorf("Expected Thinking.Type to be enabled, got %s", anthropicReq.Thinking.Type)
			}

			if anthropicReq.Thinking.BudgetTokens != tt.expectedBudget {
				t.Errorf("Expected Thinking.BudgetTokens to be %d, got %d", tt.expectedBudget, anthropicReq.Thinking.BudgetTokens)
			}
		})
	}
}

func TestInboundTransformer_ThinkingTransform(t *testing.T) {
	tests := []struct {
		name           string
		anthropicReq   MessageRequest
		expectedEffort string
	}{
		{
			name: "thinking enabled with low budget",
			anthropicReq: MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role: "user",
						Content: MessageContent{
							Content: &[]string{"Hello"}[0],
						},
					},
				},
				Thinking: &Thinking{
					Type:         "enabled",
					BudgetTokens: 5000,
				},
			},
			expectedEffort: "low",
		},
		{
			name: "thinking enabled with medium budget",
			anthropicReq: MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role: "user",
						Content: MessageContent{
							Content: &[]string{"Hello"}[0],
						},
					},
				},
				Thinking: &Thinking{
					Type:         "enabled",
					BudgetTokens: 15000,
				},
			},
			expectedEffort: "medium",
		},
		{
			name: "thinking enabled with high budget",
			anthropicReq: MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role: "user",
						Content: MessageContent{
							Content: &[]string{"Hello"}[0],
						},
					},
				},
				Thinking: &Thinking{
					Type:         "enabled",
					BudgetTokens: 30000,
				},
			},
			expectedEffort: "high",
		},
		{
			name: "thinking disabled",
			anthropicReq: MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role: "user",
						Content: MessageContent{
							Content: &[]string{"Hello"}[0],
						},
					},
				},
				Thinking: &Thinking{
					Type: "disabled",
				},
			},
			expectedEffort: "",
		},
		{
			name: "no thinking configuration",
			anthropicReq: MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role: "user",
						Content: MessageContent{
							Content: &[]string{"Hello"}[0],
						},
					},
				},
			},
			expectedEffort: "",
		},
		{
			name: "thinking enabled with custom budget (low range)",
			anthropicReq: MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role: "user",
						Content: MessageContent{
							Content: &[]string{"Hello"}[0],
						},
					},
				},
				Thinking: &Thinking{
					Type:         "enabled",
					BudgetTokens: 3000,
				},
			},
			expectedEffort: "low",
		},
		{
			name: "thinking enabled with custom budget (high range)",
			anthropicReq: MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role: "user",
						Content: MessageContent{
							Content: &[]string{"Hello"}[0],
						},
					},
				},
				Thinking: &Thinking{
					Type:         "enabled",
					BudgetTokens: 20000,
				},
			},
			expectedEffort: "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create HTTP request
			body, err := json.Marshal(tt.anthropicReq)
			if err != nil {
				t.Fatalf("Failed to marshal anthropic request: %v", err)
			}

			httpReq := &httpclient.Request{
				Method: http.MethodPost,
				URL:    "https://api.anthropic.com/v1/messages",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: body,
			}

			// Transform request
			transformer := NewInboundTransformer()

			chatReq, err := transformer.TransformRequest(context.Background(), httpReq)
			if err != nil {
				t.Fatalf("Failed to transform request: %v", err)
			}

			// Check reasoning effort
			if chatReq.ReasoningEffort != tt.expectedEffort {
				t.Errorf("Expected ReasoningEffort to be %s, got %s", tt.expectedEffort, chatReq.ReasoningEffort)
			}

			// Check ReasoningBudget preservation for enabled thinking
			if tt.anthropicReq.Thinking != nil && tt.anthropicReq.Thinking.Type == "enabled" {
				if chatReq.ReasoningBudget == nil {
					t.Errorf("Expected ReasoningBudget to be non-nil when thinking is enabled")
				} else if *chatReq.ReasoningBudget != tt.anthropicReq.Thinking.BudgetTokens {
					t.Errorf("Expected ReasoningBudget to be %d, got %d", tt.anthropicReq.Thinking.BudgetTokens, *chatReq.ReasoningBudget)
				}
			} else {
				if chatReq.ReasoningBudget != nil {
					t.Errorf("Expected ReasoningBudget to be nil when thinking is disabled or not present")
				}
			}

			// Verify other fields are preserved
			if chatReq.Model != tt.anthropicReq.Model {
				t.Errorf("Expected Model to be %s, got %s", tt.anthropicReq.Model, chatReq.Model)
			}

			if *chatReq.MaxTokens != tt.anthropicReq.MaxTokens {
				t.Errorf("Expected MaxTokens to be %d, got %d", tt.anthropicReq.MaxTokens, *chatReq.MaxTokens)
			}
		})
	}
}

func TestThinkingBudgetToReasoningEffort(t *testing.T) {
	tests := []struct {
		name           string
		budgetTokens   int64
		expectedEffort string
	}{
		{"zero budget", 0, "low"},
		{"low budget", 5000, "low"},
		{"low budget boundary", 5001, "medium"},
		{"medium budget", 15000, "medium"},
		{"medium budget boundary", 15001, "high"},
		{"high budget", 30000, "high"},
		{"very high budget", 100000, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := thinkingBudgetToReasoningEffort(tt.budgetTokens)
			if result != tt.expectedEffort {
				t.Errorf("Expected %s, got %s for budget %d", tt.expectedEffort, result, tt.budgetTokens)
			}
		})
	}
}

func TestInboundTransformer_ThinkingWithOtherFields(t *testing.T) {
	anthropicReq := MessageRequest{
		Model:       "claude-3-sonnet-20240229",
		MaxTokens:   4096,
		Temperature: &[]float64{0.7}[0],
		TopP:        &[]float64{0.9}[0],
		Messages: []MessageParam{
			{
				Role: "user",
				Content: MessageContent{
					Content: &[]string{"Hello"}[0],
				},
			},
		},
		Thinking: &Thinking{
			Type:         "enabled",
			BudgetTokens: 10000,
		},
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		t.Fatalf("Failed to marshal anthropic request: %v", err)
	}

	httpReq := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "https://api.anthropic.com/v1/messages",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: body,
	}

	transformer := NewInboundTransformer()

	chatReq, err := transformer.TransformRequest(context.Background(), httpReq)
	if err != nil {
		t.Fatalf("Failed to transform request: %v", err)
	}

	// Check all fields are preserved correctly
	if chatReq.Model != anthropicReq.Model {
		t.Errorf("Model mismatch: expected %s, got %s", anthropicReq.Model, chatReq.Model)
	}

	if *chatReq.MaxTokens != anthropicReq.MaxTokens {
		t.Errorf("MaxTokens mismatch: expected %d, got %d", anthropicReq.MaxTokens, *chatReq.MaxTokens)
	}

	if *chatReq.Temperature != *anthropicReq.Temperature {
		t.Errorf("Temperature mismatch: expected %f, got %f", *anthropicReq.Temperature, *chatReq.Temperature)
	}

	if *chatReq.TopP != *anthropicReq.TopP {
		t.Errorf("TopP mismatch: expected %f, got %f", *anthropicReq.TopP, *chatReq.TopP)
	}

	if chatReq.ReasoningEffort != "medium" {
		t.Errorf("ReasoningEffort mismatch: expected medium, got %s", chatReq.ReasoningEffort)
	}

	if len(chatReq.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(chatReq.Messages))
	}

	if chatReq.Messages[0].Role != "user" {
		t.Errorf("Expected user role, got %s", chatReq.Messages[0].Role)
	}
}
