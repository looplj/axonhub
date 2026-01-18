package anthropic

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
)

func TestClaudeCodeTransformer_WithSimulator(t *testing.T) {
	ctx := context.Background()

	// 1. Setup Transformers
	inbound := NewInboundTransformer()

	config := &Config{
		Type:    PlatformClaudeCode,
		BaseURL: "https://api.anthropic.com",
		APIKey:  "test-api-key",
	}
	outbound, err := NewClaudeCodeTransformer(config)
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

	// Verify Bearer authentication (Claude Code transformer sets this)
	assert.Equal(t, "Bearer test-api-key", finalReq.Header.Get("Authorization"))

	// Verify Body contains prepended system message
	finalBodyBytes, err := io.ReadAll(finalReq.Body)
	require.NoError(t, err)

	var finalAnthropicReq MessageRequest

	err = json.Unmarshal(finalBodyBytes, &finalAnthropicReq)
	require.NoError(t, err)

	// The outbound transformer moves the system message to the `system` field
	require.NotNil(t, finalAnthropicReq.System)
	require.NotNil(t, finalAnthropicReq.System.Prompt)
	assert.Contains(t, *finalAnthropicReq.System.Prompt, claudeCodeSystemMessage)

	// Verify user message is still there
	assert.Len(t, finalAnthropicReq.Messages, 1)
	assert.Equal(t, "user", finalAnthropicReq.Messages[0].Role)
}

func TestClaudeCodeTransformer_WithSimulator_AlreadyHasBetaQuery(t *testing.T) {
	ctx := context.Background()

	// 1. Setup Transformers
	inbound := NewInboundTransformer()

	config := &Config{
		Type:    PlatformClaudeCode,
		BaseURL: "https://api.anthropic.com/v1",
		APIKey:  "test-api-key",
	}
	outbound, err := NewClaudeCodeTransformer(config)
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

	// Verify Bearer authentication (Claude Code transformer sets this)
	assert.Equal(t, "Bearer test-api-key", finalReq.Header.Get("Authorization"))

	// Verify Body contains prepended system message
	finalBodyBytes, err := io.ReadAll(finalReq.Body)
	require.NoError(t, err)

	var finalAnthropicReq MessageRequest

	err = json.Unmarshal(finalBodyBytes, &finalAnthropicReq)
	require.NoError(t, err)

	// The outbound transformer moves the system message to the `system` field
	require.NotNil(t, finalAnthropicReq.System)
	require.NotNil(t, finalAnthropicReq.System.Prompt)
	assert.Contains(t, *finalAnthropicReq.System.Prompt, claudeCodeSystemMessage)

	// Verify user message is still there
	assert.Len(t, finalAnthropicReq.Messages, 1)
	assert.Equal(t, "user", finalAnthropicReq.Messages[0].Role)
}
