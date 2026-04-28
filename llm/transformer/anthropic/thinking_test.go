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

	result := convertToLlmResponse(anthropicResp, PlatformDirect, shared.TransportScope{})

	require.NotNil(t, result)
	require.Len(t, result.Choices, 1)
	require.NotNil(t, result.Choices[0].Message.ReasoningContent)
	require.Equal(t, thinking, *result.Choices[0].Message.ReasoningContent)
	require.NotNil(t, result.Choices[0].Message.ReasoningSignature)
	require.Equal(t, *shared.EncodeAnthropicSignature(lo.ToPtr(signature), ""), *result.Choices[0].Message.ReasoningSignature)
	require.NotNil(t, result.Choices[0].Message.Content.Content)
	require.Equal(t, answer, *result.Choices[0].Message.Content.Content)
	require.Empty(t, result.Choices[0].Message.Content.MultipleContent)
}

func TestOutboundConvert_GeminiThoughtSignatureBecomesAnthropicRedactedThinking(t *testing.T) {
	geminiSig := shared.EncodeGeminiThoughtSignature(lo.ToPtr("signature_A"), "")
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
				RedactedReasoningContent: geminiSig,
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
	require.Nil(t, assistantMsg.Content.Content)
	require.Len(t, assistantMsg.Content.MultipleContent, 2)
	require.Equal(t, "redacted_thinking", assistantMsg.Content.MultipleContent[0].Type)
	require.Equal(t, *geminiSig, assistantMsg.Content.MultipleContent[0].Data)
	require.Equal(t, "text", assistantMsg.Content.MultipleContent[1].Type)
	require.NotNil(t, assistantMsg.Content.MultipleContent[1].Text)
	require.Equal(t, "Hi", *assistantMsg.Content.MultipleContent[1].Text)
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

	result := convertToLlmResponse(anthropicResp, PlatformDirect, shared.TransportScope{})

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

	result := convertToLlmResponse(anthropicResp, PlatformDirect, shared.TransportScope{})

	require.NotNil(t, result)
	require.Len(t, result.Choices, 1)
	require.NotNil(t, result.Choices[0].Message.ReasoningContent)
	require.Equal(t, thinking, *result.Choices[0].Message.ReasoningContent)
	require.NotNil(t, result.Choices[0].Message.ReasoningSignature)
	require.Equal(t, *shared.EncodeAnthropicSignature(lo.ToPtr(signature), ""), *result.Choices[0].Message.ReasoningSignature)
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

			anthropicReq := convertToAnthropicRequestWithConfig(chatReq, tt.config, shared.TransportScope{})

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

	anthropicReq := convertToAnthropicRequestWithConfig(chatReq, nil, shared.TransportScope{})

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

			anthropicReq := convertToAnthropicRequestWithConfig(chatReq, tt.config, shared.TransportScope{})

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

func TestThinking_AdaptiveOutbound(t *testing.T) {
	tests := []struct {
		name     string
		chatReq  *llm.Request
		validate func(t *testing.T, anthropicReq *MessageRequest)
	}{
		{
			name: "metadata thinking_type=adaptive -> Thinking{Type: adaptive} and omit budget_tokens",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(4096)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				ReasoningEffort: "high",
				ReasoningBudget: lo.ToPtr(int64(30000)),
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyThinkingType: "adaptive",
				},
			},
			validate: func(t *testing.T, anthropicReq *MessageRequest) {
				t.Helper()
				require.NotNil(t, anthropicReq.Thinking)
				require.Equal(t, "adaptive", anthropicReq.Thinking.Type)

				thinkingJSON, err := json.Marshal(anthropicReq.Thinking)
				require.NoError(t, err)
				require.JSONEq(t, `{"type":"adaptive"}`, string(thinkingJSON))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anthropicReq := convertToAnthropicRequest(tt.chatReq)
			tt.validate(t, anthropicReq)
		})
	}
}

func TestThinking_AdaptiveInbound(t *testing.T) {
	tests := []struct {
		name         string
		anthropicReq *MessageRequest
		validate     func(t *testing.T, chatReq *llm.Request)
	}{
		{
			name: "Thinking{Type: adaptive} -> TransformerMetadata thinking_type=adaptive",
			anthropicReq: &MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role: "user",
						Content: MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				Thinking: &Thinking{Type: "adaptive"},
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				t.Helper()
				require.NotNil(t, chatReq.TransformerMetadata)
				require.Equal(t, "adaptive", chatReq.TransformerMetadata[TransformerMetadataKeyThinkingType])
				require.Equal(t, "high", chatReq.ReasoningEffort)
				require.Nil(t, chatReq.ReasoningBudget)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatReq, err := convertToLLMRequest(tt.anthropicReq)
			require.NoError(t, err)
			tt.validate(t, chatReq)
		})
	}
}

func TestThinking_AdaptiveJSON(t *testing.T) {
	tests := []struct {
		name     string
		thinking Thinking
		validate func(t *testing.T, data []byte)
	}{
		{
			name: "adaptive marshal omits budget_tokens",
			thinking: Thinking{
				Type: "adaptive",
			},
			validate: func(t *testing.T, data []byte) {
				t.Helper()
				require.JSONEq(t, `{"type":"adaptive"}`, string(data))

				var decoded Thinking

				err := json.Unmarshal(data, &decoded)
				require.NoError(t, err)
				require.Equal(t, "adaptive", decoded.Type)
				require.Equal(t, int64(0), decoded.BudgetTokens)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.thinking)
			require.NoError(t, err)
			tt.validate(t, data)
		})
	}
}

func TestOutputConfig_Outbound(t *testing.T) {
	tests := []struct {
		name     string
		chatReq  *llm.Request
		config   *Config
		validate func(t *testing.T, anthropicReq *MessageRequest)
	}{
		{
			name: "TransformerMetadata output_config_effort=max -> OutputConfig{Effort:max}",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(4096)),
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("hello")},
					},
				},
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyOutputConfigEffort: "max",
				},
			},
			validate: func(t *testing.T, anthropicReq *MessageRequest) {
				t.Helper()
				require.NotNil(t, anthropicReq.OutputConfig)
				require.Equal(t, "max", anthropicReq.OutputConfig.Effort)
			},
		},
		{
			name: "DeepSeek platform output_config_effort=max -> OutputConfig{Effort:max}",
			chatReq: &llm.Request{
				Model:     "deepseek-v4-pro",
				MaxTokens: lo.ToPtr(int64(4096)),
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("hello")},
					},
				},
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyOutputConfigEffort: "max",
				},
			},
			config: &Config{
				Type: PlatformDeepSeek,
			},
			validate: func(t *testing.T, anthropicReq *MessageRequest) {
				t.Helper()
				require.NotNil(t, anthropicReq.OutputConfig)
				require.Equal(t, "max", anthropicReq.OutputConfig.Effort)
				// DeepSeek supports output_config; Thinking is nil when no reasoning effort/budget is set
				require.Nil(t, anthropicReq.Thinking)
			},
		},
		{
			name: "without output_config metadata -> OutputConfig nil",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(4096)),
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("hello")},
					},
				},
			},
			validate: func(t *testing.T, anthropicReq *MessageRequest) {
				t.Helper()
				require.Nil(t, anthropicReq.OutputConfig)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var anthropicReq *MessageRequest
			if tt.config != nil {
				anthropicReq = convertToAnthropicRequestWithConfig(tt.chatReq, tt.config, shared.TransportScope{})
			} else {
				anthropicReq = convertToAnthropicRequest(tt.chatReq)
			}

			tt.validate(t, anthropicReq)
		})
	}
}

func TestOutputConfig_Inbound(t *testing.T) {
	tests := []struct {
		name         string
		anthropicReq *MessageRequest
		validate     func(t *testing.T, chatReq *llm.Request)
	}{
		{
			name: "OutputConfig effort=high -> TransformerMetadata output_config_effort=high and ReasoningEffort=high",
			anthropicReq: &MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role:    "user",
						Content: MessageContent{Content: lo.ToPtr("hello")},
					},
				},
				OutputConfig: &OutputConfig{Effort: "high"},
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				t.Helper()
				require.NotNil(t, chatReq.TransformerMetadata)
				require.Equal(t, "high", chatReq.TransformerMetadata[TransformerMetadataKeyOutputConfigEffort])
				require.Equal(t, "high", chatReq.ReasoningEffort)
			},
		},
		{
			name: "OutputConfig effort=max -> TransformerMetadata output_config_effort=max and ReasoningEffort=xhigh",
			anthropicReq: &MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role:    "user",
						Content: MessageContent{Content: lo.ToPtr("hello")},
					},
				},
				OutputConfig: &OutputConfig{Effort: "max"},
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				t.Helper()
				require.NotNil(t, chatReq.TransformerMetadata)
				require.Equal(t, "max", chatReq.TransformerMetadata[TransformerMetadataKeyOutputConfigEffort])
				require.Equal(t, "xhigh", chatReq.ReasoningEffort)
			},
		},
		{
			name: "without output_config -> TransformerMetadata has no output_config_effort",
			anthropicReq: &MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 4096,
				Messages: []MessageParam{
					{
						Role:    "user",
						Content: MessageContent{Content: lo.ToPtr("hello")},
					},
				},
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				t.Helper()
				require.NotNil(t, chatReq.TransformerMetadata)
				_, ok := chatReq.TransformerMetadata[TransformerMetadataKeyOutputConfigEffort]
				require.False(t, ok)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatReq, err := convertToLLMRequest(tt.anthropicReq)
			require.NoError(t, err)
			tt.validate(t, chatReq)
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

	// Signatures in unified format are encoded with the Anthropic prefix + channel footprint
	encodedSignature := *shared.EncodeAnthropicSignature(lo.ToPtr(signature), "")

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
				ReasoningSignature:       lo.ToPtr(encodedSignature),
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
	// Official Anthropic platforms decode the internal marker before sending upstream.
	require.Equal(t, shared.EnsureBase64Encoding(signature), *assistantMsg.Content.MultipleContent[0].Signature)

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

func TestEnsureAssistantThinkingBlocks_SimpleContent(t *testing.T) {
	msgs := []MessageParam{
		{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("Hello")}},
	}
	ensureAssistantThinkingBlocks(msgs)
	require.Len(t, msgs, 1)
	require.Nil(t, msgs[0].Content.Content)
	require.Len(t, msgs[0].Content.MultipleContent, 2)
	require.Equal(t, "thinking", msgs[0].Content.MultipleContent[0].Type)
	require.Equal(t, "", *msgs[0].Content.MultipleContent[0].Thinking)
	require.Equal(t, "text", msgs[0].Content.MultipleContent[1].Type)
	require.Equal(t, "Hello", *msgs[0].Content.MultipleContent[1].Text)
}

func TestEnsureAssistantThinkingBlocks_AlreadyHasThinking(t *testing.T) {
	msgs := []MessageParam{
		{
			Role: "assistant",
			Content: MessageContent{
				MultipleContent: []MessageContentBlock{
					{Type: "thinking", Thinking: lo.ToPtr("I think...")},
					{Type: "text", Text: lo.ToPtr("Answer")},
				},
			},
		},
	}
	ensureAssistantThinkingBlocks(msgs)
	require.Len(t, msgs[0].Content.MultipleContent, 2)
	require.Equal(t, "I think...", *msgs[0].Content.MultipleContent[0].Thinking)
}

func TestEnsureAssistantThinkingBlocks_MultipleContentWithoutThinking(t *testing.T) {
	msgs := []MessageParam{
		{
			Role: "assistant",
			Content: MessageContent{
				MultipleContent: []MessageContentBlock{
					{Type: "text", Text: lo.ToPtr("Hello")},
					{Type: "tool_use", ID: "123", Name: lo.ToPtr("calc"), Input: json.RawMessage(`{}`)},
				},
			},
		},
	}
	ensureAssistantThinkingBlocks(msgs)
	require.Len(t, msgs[0].Content.MultipleContent, 3)
	require.Equal(t, "thinking", msgs[0].Content.MultipleContent[0].Type)
	require.Equal(t, "", *msgs[0].Content.MultipleContent[0].Thinking)
	require.Equal(t, "text", msgs[0].Content.MultipleContent[1].Type)
	require.Equal(t, "tool_use", msgs[0].Content.MultipleContent[2].Type)
}

func TestEnsureAssistantThinkingBlocks_SkipsNonAssistant(t *testing.T) {
	msgs := []MessageParam{
		{Role: "user", Content: MessageContent{Content: lo.ToPtr("hi")}},
		{Role: "system", Content: MessageContent{Content: lo.ToPtr("be helpful")}},
	}
	ensureAssistantThinkingBlocks(msgs)

	for _, msg := range msgs {
		require.NotNil(t, msg.Content.Content) // still simple string, not converted
	}
}

func TestEnsureAssistantThinkingBlocks_EmptyContent(t *testing.T) {
	msgs := []MessageParam{
		{Role: "assistant", Content: MessageContent{MultipleContent: []MessageContentBlock{}}},
	}
	ensureAssistantThinkingBlocks(msgs)
	require.Len(t, msgs[0].Content.MultipleContent, 1)
	require.Equal(t, "thinking", msgs[0].Content.MultipleContent[0].Type)
	require.Equal(t, "", *msgs[0].Content.MultipleContent[0].Thinking)
}

func TestIsThinkingEnabled(t *testing.T) {
	tests := []struct {
		name     string
		req      *MessageRequest
		expected bool
	}{
		{name: "nil thinking -> enabled", req: &MessageRequest{}, expected: true},
		{name: "thinking enabled -> enabled", req: &MessageRequest{Thinking: &Thinking{Type: "enabled"}}, expected: true},
		{name: "thinking adaptive -> enabled", req: &MessageRequest{Thinking: &Thinking{Type: "adaptive"}}, expected: true},
		{name: "thinking disabled -> disabled", req: &MessageRequest{Thinking: &Thinking{Type: "disabled"}}, expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, isThinkingEnabled(tt.req))
		})
	}
}

func TestDeepSeek_EnsureThinkingBlocksInAssistantMessages(t *testing.T) {
	tests := []struct {
		name     string
		chatReq  *llm.Request
		config   *Config
		validate func(t *testing.T, anthropicReq *MessageRequest)
	}{
		{
			name: "DeepSeek with simple assistant content -> thinking block added",
			chatReq: &llm.Request{
				Model:     "deepseek-v4-pro",
				MaxTokens: lo.ToPtr(int64(4096)),
				Messages: []llm.Message{
					{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
					{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hello!")}},
				},
			},
			config: &Config{Type: PlatformDeepSeek},
			validate: func(t *testing.T, req *MessageRequest) {
				t.Helper()
				require.Len(t, req.Messages, 2)
				// assistant message should have thinking + text blocks
				assistantMsg := req.Messages[1]
				require.Nil(t, assistantMsg.Content.Content)
				require.Len(t, assistantMsg.Content.MultipleContent, 2)
				require.Equal(t, "thinking", assistantMsg.Content.MultipleContent[0].Type)
				require.Equal(t, "", *assistantMsg.Content.MultipleContent[0].Thinking)
				require.Equal(t, "text", assistantMsg.Content.MultipleContent[1].Type)
				require.Equal(t, "Hello!", *assistantMsg.Content.MultipleContent[1].Text)
			},
		},
		{
			name: "Non-DeepSeek platform -> no thinking blocks added",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(4096)),
				Messages: []llm.Message{
					{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
					{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hello!")}},
				},
			},
			config: &Config{Type: PlatformDirect},
			validate: func(t *testing.T, req *MessageRequest) {
				t.Helper()
				require.Len(t, req.Messages, 2)
				assistantMsg := req.Messages[1]
				require.NotNil(t, assistantMsg.Content.Content)
				require.Equal(t, "Hello!", *assistantMsg.Content.Content)
			},
		},
		{
			name: "DeepSeek with existing thinking block -> no duplicate",
			chatReq: &llm.Request{
				Model:     "deepseek-v4-pro",
				MaxTokens: lo.ToPtr(int64(4096)),
				Messages: []llm.Message{
					{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
					{
						Role:             "assistant",
						ReasoningContent: lo.ToPtr("Let me think..."),
						Content:          llm.MessageContent{Content: lo.ToPtr("Answer")},
					},
				},
			},
			config: &Config{Type: PlatformDeepSeek},
			validate: func(t *testing.T, req *MessageRequest) {
				t.Helper()
				require.Len(t, req.Messages, 2)
				assistantMsg := req.Messages[1]
				require.Len(t, assistantMsg.Content.MultipleContent, 2)
				// thinking block preserved as-is, not duplicated
				require.Equal(t, "thinking", assistantMsg.Content.MultipleContent[0].Type)
				require.Equal(t, "Let me think...", *assistantMsg.Content.MultipleContent[0].Thinking)
				require.Equal(t, "text", assistantMsg.Content.MultipleContent[1].Type)
				require.Equal(t, "Answer", *assistantMsg.Content.MultipleContent[1].Text)
			},
		},
		{
			name: "DeepSeek with output_config -> thinking blocks added",
			chatReq: &llm.Request{
				Model:     "deepseek-v4-pro",
				MaxTokens: lo.ToPtr(int64(4096)),
				Messages: []llm.Message{
					{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
					{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hello!")}},
				},
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyOutputConfigEffort: "high",
				},
			},
			config: &Config{Type: PlatformDeepSeek},
			validate: func(t *testing.T, req *MessageRequest) {
				t.Helper()
				require.NotNil(t, req.OutputConfig)
				require.Equal(t, "high", req.OutputConfig.Effort)
				assistantMsg := req.Messages[1]
				require.Len(t, assistantMsg.Content.MultipleContent, 2)
				require.Equal(t, "thinking", assistantMsg.Content.MultipleContent[0].Type)
				require.Equal(t, "", *assistantMsg.Content.MultipleContent[0].Thinking)
				require.Equal(t, "text", assistantMsg.Content.MultipleContent[1].Type)
				require.Equal(t, "Hello!", *assistantMsg.Content.MultipleContent[1].Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anthropicReq := convertToAnthropicRequestWithConfig(tt.chatReq, tt.config, shared.TransportScope{})
			require.NotNil(t, anthropicReq)
			tt.validate(t, anthropicReq)
		})
	}
}

func TestOutboundConvert_RedactedThinkingToAnthropicCompatiblePlatformKeepsEncodedSignature(t *testing.T) {
	const (
		thinking    = "Let me analyze this step by step..."
		signature   = "WaUjzkypQ2mUEVM36O2TxuC06KN8xyfbJwyem2dw3URve/op91XWHOEBLLqIOMfFG/UvLEczmEsUjavL...."
		textContent = "Based on my analysis..."
	)

	encodedSignature := *shared.EncodeAnthropicSignature(lo.ToPtr(signature), "")

	chatReq := &llm.Request{
		Model:           "deepseek-chat",
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
				Role:               "assistant",
				ReasoningContent:   lo.ToPtr(thinking),
				ReasoningSignature: lo.ToPtr(encodedSignature),
				Content: llm.MessageContent{
					Content: lo.ToPtr(textContent),
				},
			},
		},
	}

	anthropicReq := convertToAnthropicRequestWithConfig(chatReq, &Config{Type: PlatformDeepSeek}, shared.TransportScope{})

	require.NotNil(t, anthropicReq)
	require.Len(t, anthropicReq.Messages, 2)
	require.Len(t, anthropicReq.Messages[1].Content.MultipleContent, 2)
	require.Equal(t, "thinking", anthropicReq.Messages[1].Content.MultipleContent[0].Type)
	require.NotNil(t, anthropicReq.Messages[1].Content.MultipleContent[0].Signature)
	require.Equal(t, encodedSignature, *anthropicReq.Messages[1].Content.MultipleContent[0].Signature)
}
