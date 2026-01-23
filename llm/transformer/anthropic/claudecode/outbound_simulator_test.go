package claudecode

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/simulator"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestClaudeCodeTransformer_WithSimulator(t *testing.T) {
	ctx := context.Background()

	// 1. Setup Transformers
	inbound := anthropic.NewInboundTransformer()

	outbound, err := NewOutboundTransformer(Params{
		TokenProvider: newMockTokenProvider("test-api-key"),
	})
	require.NoError(t, err)

	// 2. Create Simulator
	sim := simulator.NewSimulator(inbound, outbound)

	// 3. Create a raw Anthropic request (what the Claude Code CLI would send)
	anthropicReqBody := map[string]any{
		"model": "claude-3-5-sonnet-20241022",
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "Hello",
			},
		},
		"max_tokens": 1024,
	}
	bodyBytes, err := json.Marshal(anthropicReqBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1/messages", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", "client-api-key")

	// 4. Run Simulation
	finalReq, err := sim.Simulate(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, finalReq)

	// 5. Verify Results

	// Verify URL and Query
	assert.Equal(t, "https://api.anthropic.com/v1/messages?beta=true", finalReq.URL.String())

	// Verify Claude Code specific headers
	assert.Equal(t, "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14", finalReq.Header.Get("Anthropic-Beta"))
	assert.Equal(t, "2023-06-01", finalReq.Header.Get("Anthropic-Version"))
	assert.Equal(t, "true", finalReq.Header.Get("Anthropic-Dangerous-Direct-Browser-Access"))
	assert.Equal(t, "claude-cli/1.0.83 (external, cli)", finalReq.Header.Get("User-Agent"))
	assert.Equal(t, "cli", finalReq.Header.Get("X-App"))

	// Verify Bearer authentication (Claude Code OAuth always uses Bearer)
	assert.Equal(t, "Bearer test-api-key", finalReq.Header.Get("Authorization"))
	assert.Empty(t, finalReq.Header.Get("X-Api-Key"))

	// Verify Body contains prepended system message
	finalBodyBytes, err := io.ReadAll(finalReq.Body)
	require.NoError(t, err)

	var finalAnthropicReq anthropic.MessageRequest

	err = json.Unmarshal(finalBodyBytes, &finalAnthropicReq)
	require.NoError(t, err)

	// The outbound transformer moves the system message to the `system` field
	require.NotNil(t, finalAnthropicReq.System)
	require.NotEmpty(t, finalAnthropicReq.System.MultiplePrompts)
	// Check that the first system prompt contains the Claude Code message
	assert.Equal(t, "text", finalAnthropicReq.System.MultiplePrompts[0].Type)
	assert.Contains(t, finalAnthropicReq.System.MultiplePrompts[0].Text, claudeCodeSystemMessage)

	// Verify user message is still there
	assert.Len(t, finalAnthropicReq.Messages, 1)
	assert.Equal(t, "user", finalAnthropicReq.Messages[0].Role)
}

func TestClaudeCodeTransformer_WithSimulator_AlreadyHasBetaQuery(t *testing.T) {
	ctx := context.Background()

	// 1. Setup Transformers
	inbound := anthropic.NewInboundTransformer()

	outbound, err := NewOutboundTransformer(Params{
		TokenProvider: newMockTokenProvider("test-api-key"),
	})
	require.NoError(t, err)

	// 2. Create Simulator
	sim := simulator.NewSimulator(inbound, outbound)

	// 3. Create a raw Anthropic request (what the Claude Code CLI would send)
	anthropicReqBody := map[string]any{
		"model": "claude-3-5-sonnet-20241022",
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "Hello",
			},
		},
		"max_tokens": 1024,
	}
	bodyBytes, err := json.Marshal(anthropicReqBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1/messages?beta=true", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", "client-api-key")

	// 4. Run Simulation
	finalReq, err := sim.Simulate(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, finalReq)

	// 5. Verify Results

	// Verify URL and Query - beta=true should already be in the URL from BaseURL
	// When RawURL is true, it appends /messages to the BaseURL
	// Since BaseURL already has beta=true, the transformer should not add it again to Query
	assert.Equal(t, "https://api.anthropic.com/v1/messages?beta=true", finalReq.URL.String())

	// Verify Claude Code specific headers
	assert.Equal(t, "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14", finalReq.Header.Get("Anthropic-Beta"))
	assert.Equal(t, "2023-06-01", finalReq.Header.Get("Anthropic-Version"))
	assert.Equal(t, "true", finalReq.Header.Get("Anthropic-Dangerous-Direct-Browser-Access"))
	assert.Equal(t, "claude-cli/1.0.83 (external, cli)", finalReq.Header.Get("User-Agent"))
	assert.Equal(t, "cli", finalReq.Header.Get("X-App"))

	// Verify Bearer authentication (Claude Code OAuth always uses Bearer)
	assert.Equal(t, "Bearer test-api-key", finalReq.Header.Get("Authorization"))
	assert.Empty(t, finalReq.Header.Get("X-Api-Key"))

	// Verify Body contains prepended system message
	finalBodyBytes, err := io.ReadAll(finalReq.Body)
	require.NoError(t, err)

	var finalAnthropicReq anthropic.MessageRequest

	err = json.Unmarshal(finalBodyBytes, &finalAnthropicReq)
	require.NoError(t, err)

	// The outbound transformer moves the system message to the `system` field
	require.NotNil(t, finalAnthropicReq.System)
	require.NotEmpty(t, finalAnthropicReq.System.MultiplePrompts)
	// Check that the first system prompt contains the Claude Code message
	assert.Equal(t, "text", finalAnthropicReq.System.MultiplePrompts[0].Type)
	assert.Contains(t, finalAnthropicReq.System.MultiplePrompts[0].Text, claudeCodeSystemMessage)

	// Verify user message is still there
	assert.Len(t, finalAnthropicReq.Messages, 1)
	assert.Equal(t, "user", finalAnthropicReq.Messages[0].Role)
}

func TestClaudeCodeTransformer_WithSimulator_InboundHeadersCannotOverride(t *testing.T) {
	ctx := context.Background()

	inbound := anthropic.NewInboundTransformer()

	outbound, err := NewOutboundTransformer(Params{
		TokenProvider: newMockTokenProvider("test-api-key"),
	})
	require.NoError(t, err)

	sim := simulator.NewSimulator(inbound, outbound)

	anthropicReqBody := map[string]any{
		"model": "claude-3-5-sonnet-20241022",
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "Hello",
			},
		},
		"max_tokens": 1024,
	}
	bodyBytes, err := json.Marshal(anthropicReqBody)
	require.NoError(t, err)

	tests := []struct {
		name            string
		inboundUA       string
		wantFinalUA     string
		wantFinalBeta   string
		wantFinalXApp   string
		wantFinalVer    string
		wantFinalDanger string
	}{
		{
			name:            "non-claude UA is ignored",
			inboundUA:       "axonhub-test/0.0.1",
			wantFinalUA:     UserAgent,
			wantFinalBeta:   "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14",
			wantFinalXApp:   "cli",
			wantFinalVer:    "2023-06-01",
			wantFinalDanger: "true",
		},
		{
			name:            "claude-cli UA is preserved",
			inboundUA:       "claude-cli/1.0.99 (external, cli)",
			wantFinalUA:     "claude-cli/1.0.99 (external, cli)",
			wantFinalBeta:   "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14",
			wantFinalXApp:   "cli",
			wantFinalVer:    "2023-06-01",
			wantFinalDanger: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1/messages", bytes.NewReader(bodyBytes))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Api-Key", "client-api-key")
			req.Header.Set("User-Agent", tt.inboundUA)
			req.Header.Set("Anthropic-Beta", "injected")
			req.Header.Set("Anthropic-Version", "1999-01-01")
			req.Header.Set("Anthropic-Dangerous-Direct-Browser-Access", "false")
			req.Header.Set("X-App", "web")
			req.Header.Set("X-Stainless-Package-Version", "999.0.0")

			finalReq, err := sim.Simulate(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, finalReq)

			assert.Equal(t, tt.wantFinalBeta, finalReq.Header.Get("Anthropic-Beta"))
			assert.Equal(t, tt.wantFinalVer, finalReq.Header.Get("Anthropic-Version"))
			assert.Equal(t, tt.wantFinalDanger, finalReq.Header.Get("Anthropic-Dangerous-Direct-Browser-Access"))
			assert.Equal(t, tt.wantFinalUA, finalReq.Header.Get("User-Agent"))
			assert.Equal(t, tt.wantFinalXApp, finalReq.Header.Get("X-App"))
			// Claude Code OAuth always uses Bearer authentication
			assert.Equal(t, "Bearer test-api-key", finalReq.Header.Get("Authorization"))
			assert.Empty(t, finalReq.Header.Get("X-Api-Key"))
			assert.Equal(t, "0.55.1", finalReq.Header.Get("X-Stainless-Package-Version"))
		})
	}
}
