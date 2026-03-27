package fireworks

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestIntegration_TransformRequest(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got: %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header with bearer token, got: %s", r.Header.Get("Authorization"))
		}

		response := map[string]any{
			"id":      "chatcmpl-test-123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "fireworks-ai/llama-3.1-70b-instruct",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello! This is a test response.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 7,
				"total_tokens":      17,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	transformerInterface, err := NewOutboundTransformer(server.URL, "test-api-key")
	require.NoError(t, err, "Failed to create Fireworks transformer")

	transformer := transformerInterface.(*OutboundTransformer)

	llmReq := &llm.Request{
		Model: "fireworks-ai/llama-3.1-70b-instruct",
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: lo.ToPtr("You are a helpful assistant."),
				},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("What is 2+2?"),
				},
				Reasoning: lo.ToPtr("The user is asking a simple math question."),
			},
			{
				Role: "assistant",
				Content: llm.MessageContent{
					Content: lo.ToPtr("The answer is 4."),
				},
				Reasoning:        lo.ToPtr("I'm calculating 2+2=4."),
				ReasoningContent: lo.ToPtr("Step 1: Add 2 and 2. Step 2: The result is 4."),
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Can you explain why?"),
				},
				Reasoning: lo.ToPtr("User wants an explanation."),
			},
		},
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err, "TransformRequest should not fail")
	require.NotNil(t, httpReq, "HTTP request should not be nil")

	assert.Equal(t, http.MethodPost, httpReq.Method, "Expected POST method")
	assert.Equal(t, server.URL+"/chat/completions", httpReq.URL, "URL should include chat/completions endpoint")

	rawHTTPReq, err := httpclient.BuildHttpRequest(t.Context(), httpReq)
	require.NoError(t, err, "BuildHttpRequest should not fail")

	client := &http.Client{Timeout: 5 * time.Second}
	rawHTTPResp, err := client.Do(rawHTTPReq)
	require.NoError(t, err, "HTTP request should not fail")
	defer rawHTTPResp.Body.Close()

	assert.Equal(t, http.StatusOK, rawHTTPResp.StatusCode, "Expected 200 OK from test server")

	require.NotEmpty(t, capturedBody, "Captured request body should not be empty")

	var capturedRequest map[string]any
	err = json.Unmarshal(capturedBody, &capturedRequest)
	require.NoError(t, err, "Captured body should be valid JSON")

	assert.Equal(t, "fireworks-ai/llama-3.1-70b-instruct", capturedRequest["model"], "Model should match")

	messages, ok := capturedRequest["messages"].([]any)
	require.True(t, ok, "messages should be an array")
	require.Len(t, messages, 4, "Should have 4 messages")

	for i, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		require.True(t, ok, "Message %d should be an object", i)

		assert.NotNil(t, msgMap["role"], "Message %d should have role", i)
		assert.NotNil(t, msgMap["content"], "Message %d should have content", i)

		_, hasReasoning := msgMap["reasoning"]
		assert.False(t, hasReasoning, "Message %d should NOT have 'reasoning' field in HTTP request", i)

		_, hasReasoningContent := msgMap["reasoning_content"]
		assert.False(t, hasReasoningContent, "Message %d should NOT have 'reasoning_content' field in HTTP request", i)
	}

	userMsg := messages[1].(map[string]any)
	assert.Equal(t, "user", userMsg["role"], "Second message should be from user")
	assert.Equal(t, "What is 2+2?", userMsg["content"], "User message content should be preserved")

	assistantMsg := messages[2].(map[string]any)
	assert.Equal(t, "assistant", assistantMsg["role"], "Third message should be from assistant")
	assert.Equal(t, "The answer is 4.", assistantMsg["content"], "Assistant message content should be preserved")
}

func TestIntegration_TransformRequest_NoReasoningFields(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		r.Body.Close()

		response := map[string]any{
			"id":      "chatcmpl-test-456",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "fireworks-ai/llama-3.1-8b-instruct",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Test response",
					},
					"finish_reason": "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	transformerInterface, err := NewOutboundTransformer(server.URL, "test-api-key")
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	llmReq := &llm.Request{
		Model: "fireworks-ai/llama-3.1-8b-instruct",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello!"),
				},
			},
		},
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	rawHTTPReq, err := httpclient.BuildHttpRequest(t.Context(), httpReq)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	rawHTTPResp, err := client.Do(rawHTTPReq)
	require.NoError(t, err)
	defer rawHTTPResp.Body.Close()

	assert.Equal(t, http.StatusOK, rawHTTPResp.StatusCode)

	var capturedRequest map[string]any
	err = json.Unmarshal(capturedBody, &capturedRequest)
	require.NoError(t, err)

	messages, ok := capturedRequest["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)

	msg := messages[0].(map[string]any)
	assert.Equal(t, "user", msg["role"])
	assert.Equal(t, "Hello!", msg["content"])

	_, hasReasoning := msg["reasoning"]
	assert.False(t, hasReasoning, "Should not have reasoning field")
	_, hasReasoningContent := msg["reasoning_content"]
	assert.False(t, hasReasoningContent, "Should not have reasoning_content field")
}

func TestIntegration_TransformResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()

		response := map[string]any{
			"id":      "chatcmpl-integration-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "fireworks-ai/llama-3.1-70b-instruct",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "This is a test response from Fireworks.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     15,
				"completion_tokens": 10,
				"total_tokens":      25,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	transformerInterface, err := NewOutboundTransformer(server.URL, "test-api-key")
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	llmReq := &llm.Request{
		Model: "fireworks-ai/llama-3.1-70b-instruct",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Test message"),
				},
			},
		},
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	rawHTTPReq, err := httpclient.BuildHttpRequest(t.Context(), httpReq)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	rawHTTPResp, err := client.Do(rawHTTPReq)
	require.NoError(t, err)
	defer rawHTTPResp.Body.Close()

	body, err := io.ReadAll(rawHTTPResp.Body)
	require.NoError(t, err)

	httpResp := &httpclient.Response{
		StatusCode: rawHTTPResp.StatusCode,
		Headers:    rawHTTPResp.Header,
		Body:       body,
	}

	llmResp, err := transformer.TransformResponse(t.Context(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)

	assert.Equal(t, "chatcmpl-integration-test", llmResp.ID)
	assert.Equal(t, "fireworks-ai/llama-3.1-70b-instruct", llmResp.Model)
	assert.Len(t, llmResp.Choices, 1)
	assert.Equal(t, "assistant", llmResp.Choices[0].Message.Role)
	assert.Equal(t, "stop", *llmResp.Choices[0].FinishReason)
	require.NotNil(t, llmResp.Choices[0].Message.Content.Content)
	assert.Equal(t, "This is a test response from Fireworks.", *llmResp.Choices[0].Message.Content.Content)
}

func TestIntegration_EndToEnd_ReasoningStripping(t *testing.T) {
	var capturedBody []byte
	bodyReceived := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
		}
		r.Body.Close()
		bodyReceived <- true

		response := map[string]any{
			"id":      "chatcmpl-e2e-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "accounts/fireworks/models/llama-v3p1-70b-instruct",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "I understand your question.",
					},
					"finish_reason": "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	transformerInterface, err := NewOutboundTransformer(server.URL, "test-api-key")
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	llmReq := &llm.Request{
		Model: "accounts/fireworks/models/llama-v3p1-70b-instruct",
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: lo.ToPtr("You are a helpful AI assistant."),
				},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello, can you help me?"),
				},
				Reasoning: lo.ToPtr("User is greeting and asking for help"),
			},
			{
				Role: "assistant",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Of course! I'd be happy to help."),
				},
				Reasoning:        lo.ToPtr("Responding affirmatively to user's request"),
				ReasoningContent: lo.ToPtr("I should be polite and helpful"),
			},
		},
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	rawHTTPReq, err := httpclient.BuildHttpRequest(t.Context(), httpReq)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	rawHTTPResp, err := client.Do(rawHTTPReq)
	require.NoError(t, err)
	defer rawHTTPResp.Body.Close()

	select {
	case <-bodyReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for server to receive request")
	}

	require.NotEmpty(t, capturedBody, "Request body should have been captured")

	var requestBody map[string]any
	err = json.Unmarshal(capturedBody, &requestBody)
	require.NoError(t, err)

	assert.Equal(t, "accounts/fireworks/models/llama-v3p1-70b-instruct", requestBody["model"])

	messages, ok := requestBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 3)

	for i, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		require.True(t, ok)

		_, hasReasoning := msgMap["reasoning"]
		_, hasReasoningContent := msgMap["reasoning_content"]

		assert.False(t, hasReasoning, "Message %d should not contain 'reasoning' field", i)
		assert.False(t, hasReasoningContent, "Message %d should not contain 'reasoning_content' field", i)

		assert.NotNil(t, msgMap["role"])
		assert.NotNil(t, msgMap["content"])
	}

	systemMsg := messages[0].(map[string]any)
	assert.Equal(t, "system", systemMsg["role"])

	userMsg := messages[1].(map[string]any)
	assert.Equal(t, "user", userMsg["role"])
	assert.Equal(t, "Hello, can you help me?", userMsg["content"])

	assistantMsg := messages[2].(map[string]any)
	assert.Equal(t, "assistant", assistantMsg["role"])
	assert.Equal(t, "Of course! I'd be happy to help.", assistantMsg["content"])

	body, err := io.ReadAll(rawHTTPResp.Body)
	require.NoError(t, err)

	httpResp := &httpclient.Response{
		StatusCode: rawHTTPResp.StatusCode,
		Headers:    rawHTTPResp.Header,
		Body:       body,
	}

	llmResp, err := transformer.TransformResponse(t.Context(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)

	assert.Equal(t, http.StatusOK, rawHTTPResp.StatusCode)
	assert.Equal(t, "chatcmpl-e2e-test", llmResp.ID)
	assert.Len(t, llmResp.Choices, 1)
}

func TestIntegration_RequestBodyJSONIntegrity(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		r.Body.Close()

		response := map[string]any{
			"id":      "chatcmpl-json-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "fireworks-ai/model",
			"choices": []map[string]any{},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	transformerInterface, err := NewOutboundTransformer(server.URL, "test-key")
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	llmReq := &llm.Request{
		Model: "fireworks-ai/model",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Test"),
				},
				Reasoning: lo.ToPtr("reasoning to be stripped"),
			},
		},
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	rawHTTPReq, err := httpclient.BuildHttpRequest(t.Context(), httpReq)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(rawHTTPReq)
	require.NoError(t, err)
	resp.Body.Close()

	require.NotEmpty(t, capturedBody)

	var bodyMap map[string]any
	err = json.Unmarshal(capturedBody, &bodyMap)
	require.NoError(t, err)

	reEncoded, err := json.Marshal(bodyMap)
	require.NoError(t, err)

	var reDecoded map[string]any
	err = json.Unmarshal(reEncoded, &reDecoded)
	require.NoError(t, err)

	assert.Equal(t, bodyMap, reDecoded)

	_, hasReasoning := bodyMap["messages"].([]any)[0].(map[string]any)["reasoning"]
	assert.False(t, hasReasoning)

	content := bodyMap["messages"].([]any)[0].(map[string]any)["content"]
	assert.Equal(t, "Test", content)
}

func TestIntegration_EmptyReasoningFields(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		r.Body.Close()

		response := map[string]any{
			"id":      "chatcmpl-empty-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "fireworks-ai/model",
			"choices": []map[string]any{},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	transformerInterface, err := NewOutboundTransformer(server.URL, "test-key")
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	llmReq := &llm.Request{
		Model: "fireworks-ai/model",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Test message"),
				},
				Reasoning:        lo.ToPtr(""),
				ReasoningContent: lo.ToPtr(""),
			},
		},
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	rawHTTPReq, err := httpclient.BuildHttpRequest(t.Context(), httpReq)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(rawHTTPReq)
	require.NoError(t, err)
	resp.Body.Close()

	require.NotEmpty(t, capturedBody)

	var bodyMap map[string]any
	err = json.Unmarshal(capturedBody, &bodyMap)
	require.NoError(t, err)

	messages := bodyMap["messages"].([]any)
	require.Len(t, messages, 1)

	msg := messages[0].(map[string]any)
	assert.Equal(t, "user", msg["role"])
	assert.Equal(t, "Test message", msg["content"])

	_, hasReasoning := msg["reasoning"]
	_, hasReasoningContent := msg["reasoning_content"]

	assert.False(t, hasReasoning, "Empty reasoning should still be stripped")
	assert.False(t, hasReasoningContent, "Empty reasoning_content should still be stripped")
}

func TestIntegration_LargeRequestWithReasoning(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		r.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-large-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "fireworks-ai/model",
			"choices": []map[string]any{},
		})
	}))
	defer server.Close()

	transformerInterface, err := NewOutboundTransformer(server.URL, "test-key")
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	messages := make([]llm.Message, 0, 10)
	for i := 0; i < 10; i++ {
		messages = append(messages, llm.Message{
			Role: "user",
			Content: llm.MessageContent{
				Content: lo.ToPtr("Message " + string(rune('0'+i))),
			},
			Reasoning: lo.ToPtr("Reasoning for message " + string(rune('0'+i))),
		})
		messages = append(messages, llm.Message{
			Role: "assistant",
			Content: llm.MessageContent{
				Content: lo.ToPtr("Response " + string(rune('0'+i))),
			},
			Reasoning:        lo.ToPtr("Assistant reasoning " + string(rune('0'+i))),
			ReasoningContent: lo.ToPtr("Assistant reasoning content " + string(rune('0'+i))),
		})
	}

	llmReq := &llm.Request{
		Model:    "fireworks-ai/model",
		Messages: messages,
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	rawHTTPReq, err := httpclient.BuildHttpRequest(t.Context(), httpReq)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(rawHTTPReq)
	require.NoError(t, err)
	resp.Body.Close()

	require.NotEmpty(t, capturedBody)

	var bodyMap map[string]any
	err = json.Unmarshal(capturedBody, &bodyMap)
	require.NoError(t, err)

	receivedMessages := bodyMap["messages"].([]any)
	require.Len(t, receivedMessages, 20)

	for i, msg := range receivedMessages {
		msgMap := msg.(map[string]any)
		_, hasReasoning := msgMap["reasoning"]
		_, hasReasoningContent := msgMap["reasoning_content"]

		assert.False(t, hasReasoning, "Message %d should not have reasoning", i)
		assert.False(t, hasReasoningContent, "Message %d should not have reasoning_content", i)
	}
}

func TestIntegration_RequestHeadersAndAuth(t *testing.T) {
	var receivedHeaders http.Header
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header

		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			receivedAuth = authHeader[7:]
		}

		io.Copy(io.Discard, r.Body)
		r.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-auth-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "fireworks-ai/model",
			"choices": []map[string]any{},
		})
	}))
	defer server.Close()

	testAPIKey := "sk-fireworks-test-key-12345"
	transformerInterface, err := NewOutboundTransformer(server.URL, testAPIKey)
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	llmReq := &llm.Request{
		Model: "fireworks-ai/model",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello"),
				},
				Reasoning: lo.ToPtr("test reasoning"),
			},
		},
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	rawHTTPReq, err := httpclient.BuildHttpRequest(t.Context(), httpReq)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(rawHTTPReq)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, testAPIKey, receivedAuth)
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
	assert.Equal(t, "application/json", receivedHeaders.Get("Accept"))
}

func TestIntegration_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json {"))
	}))
	defer server.Close()

	transformerInterface, err := NewOutboundTransformer(server.URL, "test-key")
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	llmReq := &llm.Request{
		Model: "fireworks-ai/model",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Test"),
				},
			},
		},
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	rawHTTPReq, err := httpclient.BuildHttpRequest(t.Context(), httpReq)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	rawHTTPResp, err := client.Do(rawHTTPReq)
	require.NoError(t, err)

	body, err := io.ReadAll(rawHTTPResp.Body)
	require.NoError(t, err)
	rawHTTPResp.Body.Close()

	httpResp := &httpclient.Response{
		StatusCode: rawHTTPResp.StatusCode,
		Headers:    rawHTTPResp.Header,
		Body:       body,
	}

	_, err = transformer.TransformResponse(t.Context(), httpResp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestIntegration_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "Internal server error"}}`))

		var capturedBody map[string]any
		json.Unmarshal(body, &capturedBody)
		messages := capturedBody["messages"].([]any)
		for _, msg := range messages {
			msgMap := msg.(map[string]any)
			if _, ok := msgMap["reasoning"]; ok {
				t.Errorf("Request body should not contain reasoning field")
			}
			if _, ok := msgMap["reasoning_content"]; ok {
				t.Errorf("Request body should not contain reasoning_content field")
			}
		}
	}))
	defer server.Close()

	transformerInterface, err := NewOutboundTransformer(server.URL, "test-key")
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	llmReq := &llm.Request{
		Model: "fireworks-ai/model",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Test"),
				},
				Reasoning: lo.ToPtr("test reasoning"),
			},
		},
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	httpClientReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     server.URL + "/chat/completions",
		Headers: httpReq.Headers,
		Body:    httpReq.Body,
		Auth:    httpReq.Auth,
	}

	httpClient := httpclient.NewHttpClient()
	_, err = httpClient.Do(t.Context(), httpClientReq)

	assert.Error(t, err)
	httpErr, ok := err.(*httpclient.Error)
	require.True(t, ok, "Error should be httpclient.Error")
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
}

func TestIntegration_RequestBodySize(t *testing.T) {
	var bodySize int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodySize = len(body)
		r.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-size-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "fireworks-ai/model",
			"choices": []map[string]any{},
		})
	}))
	defer server.Close()

	transformerInterface, err := NewOutboundTransformer(server.URL, "test-key")
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	largeContent := ""
	for i := 0; i < 1000; i++ {
		largeContent += "This is a test message. "
	}

	llmReq := &llm.Request{
		Model: "fireworks-ai/model",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: &largeContent,
				},
				Reasoning:        lo.ToPtr("some reasoning that should be stripped"),
				ReasoningContent: lo.ToPtr("some reasoning content that should be stripped"),
			},
		},
	}

	httpReq, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	rawHTTPReq, err := httpclient.BuildHttpRequest(t.Context(), httpReq)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(rawHTTPReq)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Greater(t, bodySize, 0)

	var capturedBody map[string]any
	bodyReader := bytes.NewReader(httpReq.Body)
	err = json.NewDecoder(bodyReader).Decode(&capturedBody)
	require.NoError(t, err)

	_, hasReasoning := capturedBody["messages"].([]any)[0].(map[string]any)["reasoning"]
	_, hasReasoningContent := capturedBody["messages"].([]any)[0].(map[string]any)["reasoning_content"]

	assert.False(t, hasReasoning)
	assert.False(t, hasReasoningContent)
}
