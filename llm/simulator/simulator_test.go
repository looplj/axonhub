package simulator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestSimulator_OpenAIToAnthropic(t *testing.T) {
	// 1. Setup Transformers
	inbound := openai.NewInboundTransformer()
	outbound, err := anthropic.NewOutboundTransformer("https://api.anthropic.com/v1", "sk-ant-test")
	require.NoError(t, err)

	// 2. Create Simulator
	sim := NewSimulator(inbound, outbound)

	// 3. Create a raw OpenAI request (what the client sends)
	openAIReqBody := map[string]any{
		"model": "gpt-4",
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "Hello, how are you?",
			},
		},
		"temperature": 0.7,
	}
	bodyBytes, _ := json.Marshal(openAIReqBody)
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1/chat/completions", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-openai-test")

	// 4. Run Simulation
	ctx := context.Background()
	finalReq, err := sim.Simulate(ctx, req)

	// 5. Verify Results
	require.NoError(t, err)
	assert.NotNil(t, finalReq)

	// Check final request properties (Anthropic format)
	assert.Equal(t, http.MethodPost, finalReq.Method)
	// Anthropic outbound appends /messages to baseURL
	assert.Equal(t, "https://api.anthropic.com/v1/messages", finalReq.URL.String())
	assert.Equal(t, "application/json", finalReq.Header.Get("Content-Type"))
	assert.Equal(t, "sk-ant-test", finalReq.Header.Get("X-Api-Key"))
	assert.Equal(t, "2023-06-01", finalReq.Header.Get("Anthropic-Version"))

	// Check body
	finalBodyBytes, err := io.ReadAll(finalReq.Body)
	require.NoError(t, err)

	var anthropicReqBody map[string]any

	err = json.Unmarshal(finalBodyBytes, &anthropicReqBody)
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", anthropicReqBody["model"])
	messages := anthropicReqBody["messages"].([]any)
	assert.Len(t, messages, 1)
	msg := messages[0].(map[string]any)
	assert.Equal(t, "user", msg["role"])
	assert.Equal(t, "Hello, how are you?", msg["content"])
}

func TestSimulator_GeminiToOpenAI(t *testing.T) {
	// 1. Setup Transformers
	// Gemini uses content-based format for inbound
	inbound := gemini.NewInboundTransformer()

	outbound, err := openai.NewOutboundTransformer("https://api.openai.com/v1", "sk-openai-test")
	require.NoError(t, err)

	// 2. Create Simulator
	sim := NewSimulator(inbound, outbound)

	// 3. Create a raw Gemini request (what the client sends)
	geminiReqBody := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{
						"text": "Explain quantum physics.",
					},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature": 0.5,
		},
	}
	bodyBytes, _ := json.Marshal(geminiReqBody)
	// Note: Gemini inbound expects a specific path usually, but the transformer might be flexible
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// 4. Run Simulation
	ctx := context.Background()
	finalReq, err := sim.Simulate(ctx, req)

	// 5. Verify Results
	require.NoError(t, err)
	assert.NotNil(t, finalReq)

	// Check final request properties (OpenAI format)
	assert.Equal(t, http.MethodPost, finalReq.Method)
	assert.Equal(t, "https://api.openai.com/v1/chat/completions", finalReq.URL.String())
	assert.Equal(t, "application/json", finalReq.Header.Get("Content-Type"))
	assert.Equal(t, "Bearer sk-openai-test", finalReq.Header.Get("Authorization"))

	// Check body
	finalBodyBytes, err := io.ReadAll(finalReq.Body)
	require.NoError(t, err)

	var openAIReqBody map[string]any

	err = json.Unmarshal(finalBodyBytes, &openAIReqBody)
	require.NoError(t, err)

	// Gemini model might be extracted from the URL or body by the transformer
	// In this case, we didn't specify a model in the body, let's see what the transformer does
	messages := openAIReqBody["messages"].([]any)
	assert.Len(t, messages, 1)
	msg := messages[0].(map[string]any)
	assert.Equal(t, "user", msg["role"])
	assert.Equal(t, "Explain quantum physics.", msg["content"])
}
