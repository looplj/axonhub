package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
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
			{Type: "thinking", Thinking: lo.ToPtr(thinking), Signature: lo.ToPtr(signature)},
			{Type: "text", Text: lo.ToPtr(answer)},
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

func TestOutboundConvert_GeminiThoughtSignatureNotAnthropicRedactedThinking(t *testing.T) {
	chatReq := &llm.Request{
		Model:     "claude-sonnet-4-5-20250929",
		MaxTokens: lo.ToPtr(int64(16000)),
		Messages: []llm.Message{
			{
				Role: "assistant",
				Content: llm.MessageContent{
					Content: lo.ToPtr("hello"),
				},
			},
			{
				Role:                     "assistant",
				RedactedReasoningContent: shared.EncodeGeminiThoughtSignature(lo.ToPtr("signature_A")),
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hi"),
				},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Continue"),
				},
			},
		},
	}

	anthropicReq := convertToAnthropicRequest(chatReq)

	require.NotNil(t, anthropicReq)
	require.Len(t, anthropicReq.Messages, 3)

	assistantMsg := anthropicReq.Messages[1]
	require.Equal(t, "assistant", assistantMsg.Role)
	require.NotNil(t, assistantMsg.Content.Content)
	require.Equal(t, "Hi", *assistantMsg.Content.Content)
	require.Empty(t, assistantMsg.Content.MultipleContent)
}

func TestConvertToChatCompletionResponse_WithRedactedThinking(t *testing.T) {
	const (
		redactedData = "EmwKAhgBEgy3va3pzix/LafPsn4aDFIT2Xlxh0L5L8rLVyIwxtE3rAFBa8cr3qpPkNRj2YfWXGmKDxH4mPnZ5sQ7vB9URj2pLmN3kF8/dW5hR7xJ0aP1oLs9yTcMnKVf2wRpEGjH9XZaBt4UvDcPrQ..."
		answer       = "Based on my analysis..."
	)

	anthropicResp := &Message{
		ID:   "msg_redacted",
		Type: "message",
		Role: "assistant",
		Content: []MessageContentBlock{
			{Type: "redacted_thinking", Data: redactedData},
			{Type: "text", Text: lo.ToPtr(answer)},
		},
		Model: "claude-sonnet-4-5-20250929",
	}

	result := convertToLlmResponse(anthropicResp, PlatformDirect)

	require.NotNil(t, result)
	require.Len(t, result.Choices, 1)
	require.NotNil(t, result.Choices[0].Message.RedactedReasoningContent)
	require.Equal(t, redactedData, *result.Choices[0].Message.RedactedReasoningContent)
	require.Nil(t, result.Choices[0].Message.ReasoningContent)
	require.NotNil(t, result.Choices[0].Message.Content.Content)
	require.Equal(t, answer, *result.Choices[0].Message.Content.Content)
}

func TestConvertToChatCompletionResponse_WithThinkingAndRedactedThinking(t *testing.T) {
	const (
		thinking     = "Let me analyze this step by step..."
		signature    = "WaUjzkypQ2mUEVM36O2TxuC06KN8xyfbJwyem2dw3URve/op91XWHOEBLLqIOMfFG/UvLEczmEsUjavL...."
		redactedData = "EmwKAhgBEgy3va3pzix/LafPsn4aDFIT2Xlxh0L5L8rLVyIwxtE3rAFBa8cr3qpPkNRj2YfWXGmKDxH4mPnZ5sQ7vB9URj2pLmN3kF8/dW5hR7xJ0aP1oLs9yTcMnKVf2wRpEGjH9XZaBt4UvDcPrQ..."
		answer       = "Based on my analysis..."
	)

	anthropicResp := &Message{
		ID:   "msg_mixed",
		Type: "message",
		Role: "assistant",
		Content: []MessageContentBlock{
			{Type: "thinking", Thinking: lo.ToPtr(thinking), Signature: lo.ToPtr(signature)},
			{Type: "redacted_thinking", Data: redactedData},
			{Type: "text", Text: lo.ToPtr(answer)},
		},
		Model: "claude-sonnet-4-5-20250929",
	}

	result := convertToLlmResponse(anthropicResp, PlatformDirect)

	require.NotNil(t, result)
	require.Len(t, result.Choices, 1)
	require.NotNil(t, result.Choices[0].Message.ReasoningContent)
	require.Equal(t, thinking, *result.Choices[0].Message.ReasoningContent)
	require.NotNil(t, result.Choices[0].Message.ReasoningSignature)
	require.Equal(t, signature, *result.Choices[0].Message.ReasoningSignature)
	require.NotNil(t, result.Choices[0].Message.RedactedReasoningContent)
	require.Equal(t, redactedData, *result.Choices[0].Message.RedactedReasoningContent)
	require.NotNil(t, result.Choices[0].Message.Content.Content)
	require.Equal(t, answer, *result.Choices[0].Message.Content.Content)
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

func TestInboundTransformer_RedactedThinkingInRequest(t *testing.T) {
	const (
		thinking     = "Let me analyze this step by step..."
		signature    = "WaUjzkypQ2mUEVM36O2TxuC06KN8xyfbJwyem2dw3URve/op91XWHOEBLLqIOMfFG/UvLEczmEsUjavL...."
		redactedData = "EmwKAhgBEgy3va3pzix/LafPsn4aDFIT2Xlxh0L5L8rLVyIwxtE3rAFBa8cr3qpPkNRj2YfWXGmKDxH4mPnZ5sQ7vB9URj2pLmN3kF8/dW5hR7xJ0aP1oLs9yTcMnKVf2wRpEGjH9XZaBt4UvDcPrQ..."
		textContent  = "Based on my analysis..."
	)

	// Test case: assistant message with thinking, redacted_thinking, and text blocks
	anthropicReq := MessageRequest{
		Model:     "claude-sonnet-4-5-20250929",
		MaxTokens: 16000,
		Messages: []MessageParam{
			{
				Role: "user",
				Content: MessageContent{
					Content: lo.ToPtr("Hello"),
				},
			},
			{
				Role: "assistant",
				Content: MessageContent{
					MultipleContent: []MessageContentBlock{
						{Type: "thinking", Thinking: lo.ToPtr(thinking), Signature: lo.ToPtr(signature)},
						{Type: "redacted_thinking", Data: redactedData},
						{Type: "text", Text: lo.ToPtr(textContent)},
					},
				},
			},
			{
				Role: "user",
				Content: MessageContent{
					Content: lo.ToPtr("Continue"),
				},
			},
		},
		Thinking: &Thinking{
			Type:         "enabled",
			BudgetTokens: 10000,
		},
	}

	body, err := json.Marshal(anthropicReq)
	require.NoError(t, err)

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
	require.NoError(t, err)

	// Find the assistant message
	var assistantMsg *llm.Message

	for i := range chatReq.Messages {
		if chatReq.Messages[i].Role == "assistant" {
			assistantMsg = &chatReq.Messages[i]
			break
		}
	}

	require.NotNil(t, assistantMsg)
	require.NotNil(t, assistantMsg.ReasoningContent)
	require.Equal(t, thinking, *assistantMsg.ReasoningContent)
	require.NotNil(t, assistantMsg.ReasoningSignature)
	require.Equal(t, signature, *assistantMsg.ReasoningSignature)
	require.NotNil(t, assistantMsg.RedactedReasoningContent)
	require.Equal(t, redactedData, *assistantMsg.RedactedReasoningContent)
}

func TestInboundTransformer_RedactedThinkingOnlyInRequest(t *testing.T) {
	const (
		redactedData = "EmwKAhgBEgy3va3pzix/LafPsn4aDFIT2Xlxh0L5L8rLVyIwxtE3rAFBa8cr3qpPkNRj2YfWXGmKDxH4mPnZ5sQ7vB9URj2pLmN3kF8/dW5hR7xJ0aP1oLs9yTcMnKVf2wRpEGjH9XZaBt4UvDcPrQ..."
		textContent  = "Based on my analysis..."
	)

	// Test case: assistant message with only redacted_thinking and text blocks
	anthropicReq := MessageRequest{
		Model:     "claude-sonnet-4-5-20250929",
		MaxTokens: 16000,
		Messages: []MessageParam{
			{
				Role: "user",
				Content: MessageContent{
					Content: lo.ToPtr("Hello"),
				},
			},
			{
				Role: "assistant",
				Content: MessageContent{
					MultipleContent: []MessageContentBlock{
						{Type: "redacted_thinking", Data: redactedData},
						{Type: "text", Text: lo.ToPtr(textContent)},
					},
				},
			},
			{
				Role: "user",
				Content: MessageContent{
					Content: lo.ToPtr("Continue"),
				},
			},
		},
		Thinking: &Thinking{
			Type:         "enabled",
			BudgetTokens: 10000,
		},
	}

	body, err := json.Marshal(anthropicReq)
	require.NoError(t, err)

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
	require.NoError(t, err)

	// Find the assistant message
	var assistantMsg *llm.Message

	for i := range chatReq.Messages {
		if chatReq.Messages[i].Role == "assistant" {
			assistantMsg = &chatReq.Messages[i]
			break
		}
	}

	require.NotNil(t, assistantMsg)
	require.Nil(t, assistantMsg.ReasoningContent)
	require.NotNil(t, assistantMsg.RedactedReasoningContent)
	require.Equal(t, redactedData, *assistantMsg.RedactedReasoningContent)
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

func TestOutboundConvert_RedactedThinkingToAnthropic(t *testing.T) {
	const (
		thinking     = "Let me analyze this step by step..."
		signature    = "WaUjzkypQ2mUEVM36O2TxuC06KN8xyfbJwyem2dw3URve/op91XWHOEBLLqIOMfFG/UvLEczmEsUjavL...."
		redactedData = "EmwKAhgBEgy3va3pzix/LafPsn4aDFIT2Xlxh0L5L8rLVyIwxtE3rAFBa8cr3qpPkNRj2YfWXGmKDxH4mPnZ5sQ7vB9URj2pLmN3kF8/dW5hR7xJ0aP1oLs9yTcMnKVf2wRpEGjH9XZaBt4UvDcPrQ..."
		textContent  = "Based on my analysis..."
	)

	chatReq := &llm.Request{
		Model:           "claude-sonnet-4-5-20250929",
		MaxTokens:       lo.ToPtr(int64(16000)),
		ReasoningEffort: "medium",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello"),
				},
			},
			{
				Role:                     "assistant",
				ReasoningContent:         lo.ToPtr(thinking),
				ReasoningSignature:       lo.ToPtr(signature),
				RedactedReasoningContent: lo.ToPtr(redactedData),
				Content: llm.MessageContent{
					Content: lo.ToPtr(textContent),
				},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Continue"),
				},
			},
		},
	}

	anthropicReq := convertToAnthropicRequest(chatReq)

	require.NotNil(t, anthropicReq)
	require.Len(t, anthropicReq.Messages, 3)

	// Check the assistant message has thinking, redacted_thinking, and text blocks
	assistantMsg := anthropicReq.Messages[1]
	require.Equal(t, "assistant", assistantMsg.Role)
	require.NotNil(t, assistantMsg.Content.MultipleContent)
	require.Len(t, assistantMsg.Content.MultipleContent, 3)

	// First block should be thinking
	require.Equal(t, "thinking", assistantMsg.Content.MultipleContent[0].Type)
	require.NotNil(t, assistantMsg.Content.MultipleContent[0].Thinking)
	require.Equal(t, thinking, *assistantMsg.Content.MultipleContent[0].Thinking)
	require.NotNil(t, assistantMsg.Content.MultipleContent[0].Signature)
	require.Equal(t, signature, *assistantMsg.Content.MultipleContent[0].Signature)

	// Second block should be redacted_thinking
	require.Equal(t, "redacted_thinking", assistantMsg.Content.MultipleContent[1].Type)
	require.Equal(t, redactedData, assistantMsg.Content.MultipleContent[1].Data)

	// Third block should be text
	require.Equal(t, "text", assistantMsg.Content.MultipleContent[2].Type)
	require.NotNil(t, assistantMsg.Content.MultipleContent[2].Text)
	require.Equal(t, textContent, *assistantMsg.Content.MultipleContent[2].Text)
}

func TestOutboundConvert_RedactedThinkingOnlyToAnthropic(t *testing.T) {
	const (
		redactedData = "EmwKAhgBEgy3va3pzix/LafPsn4aDFIT2Xlxh0L5L8rLVyIwxtE3rAFBa8cr3qpPkNRj2YfWXGmKDxH4mPnZ5sQ7vB9URj2pLmN3kF8/dW5hR7xJ0aP1oLs9yTcMnKVf2wRpEGjH9XZaBt4UvDcPrQ..."
		textContent  = "Based on my analysis..."
	)

	chatReq := &llm.Request{
		Model:           "claude-sonnet-4-5-20250929",
		MaxTokens:       lo.ToPtr(int64(16000)),
		ReasoningEffort: "medium",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello"),
				},
			},
			{
				Role:                     "assistant",
				RedactedReasoningContent: lo.ToPtr(redactedData),
				Content: llm.MessageContent{
					Content: lo.ToPtr(textContent),
				},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Continue"),
				},
			},
		},
	}

	anthropicReq := convertToAnthropicRequest(chatReq)

	require.NotNil(t, anthropicReq)
	require.Len(t, anthropicReq.Messages, 3)

	// Check the assistant message has redacted_thinking and text blocks
	assistantMsg := anthropicReq.Messages[1]
	require.Equal(t, "assistant", assistantMsg.Role)
	require.NotNil(t, assistantMsg.Content.MultipleContent)
	require.Len(t, assistantMsg.Content.MultipleContent, 2)

	// First block should be redacted_thinking
	require.Equal(t, "redacted_thinking", assistantMsg.Content.MultipleContent[0].Type)
	require.Equal(t, redactedData, assistantMsg.Content.MultipleContent[0].Data)

	// Second block should be text
	require.Equal(t, "text", assistantMsg.Content.MultipleContent[1].Type)
	require.NotNil(t, assistantMsg.Content.MultipleContent[1].Text)
	require.Equal(t, textContent, *assistantMsg.Content.MultipleContent[1].Text)
}
