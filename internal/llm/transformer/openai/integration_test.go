package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// TestIntegration_OpenAITransformers tests the complete flow of inbound and outbound transformers.
func TestIntegration_OpenAITransformers(t *testing.T) {
	// Create transformers
	inbound := NewInboundTransformer()

	outbound, err := NewOutboundTransformer("https://api.openai.com/v1", "test-api-key")
	if err != nil {
		t.Fatalf("Failed to create outbound transformer: %v", err)
	}

	// Create HTTP client
	httpClient := httpclient.NewHttpClient()

	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))

			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": {"message": "Invalid content type"}}`))

			return
		}

		// Parse request body
		var chatReq llm.Request

		err := json.NewDecoder(r.Body).Decode(&chatReq)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": {"message": "Invalid JSON"}}`))

			return
		}

		// Create mock response
		response := llm.Response{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   chatReq.Model,
			Choices: []llm.Choice{
				{
					Index: 0,
					Message: &llm.Message{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: lo.ToPtr(
								fmt.Sprintf("Echo: %s", *chatReq.Messages[0].Content.Content),
							),
						},
					},
					FinishReason: lo.ToPtr("stop"),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Update outbound transformer to use test server
	outbound.(*OutboundTransformer).SetBaseURL(server.URL)

	// Test data
	originalRequest := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/chat/completions",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: mustMarshal(llm.Request{
			Model: "gpt-4",
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello, world!"),
					},
				},
			},
		}),
	}

	// Step 1: Inbound transformation (HTTP -> ChatCompletionRequest)
	chatReq, err := inbound.TransformRequest(t.Context(), originalRequest)
	if err != nil {
		t.Fatalf("Inbound transformation failed: %v", err)
	}

	if chatReq.Model != "gpt-4" {
		t.Errorf("Expected model gpt-4, got %s", chatReq.Model)
	}

	if len(chatReq.Messages) != 1 || chatReq.Messages[0].Content.Content == nil ||
		*chatReq.Messages[0].Content.Content != "Hello, world!" {
		t.Errorf("Messages not preserved correctly: %+v", chatReq.Messages)
	}

	// Step 2: Outbound transformation (ChatCompletionRequest -> HTTP)
	httpReq, err := outbound.TransformRequest(t.Context(), chatReq)
	if err != nil {
		t.Fatalf("Outbound transformation failed: %v", err)
	}

	if httpReq.Method != http.MethodPost {
		t.Errorf("Expected POST method, got %s", httpReq.Method)
	}

	if !strings.Contains(httpReq.URL, "/chat/completions") {
		t.Errorf("Expected URL to contain /chat/completions, got %s", httpReq.URL)
	}

	// Step 3: Execute HTTP request
	httpResp, err := httpClient.Do(t.Context(), httpReq)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", httpResp.StatusCode)
	}

	// Step 4: Outbound response transformation (HTTP -> ChatCompletionResponse)
	chatResp, err := outbound.TransformResponse(t.Context(), httpResp)
	if err != nil {
		t.Fatalf("Outbound response transformation failed: %v", err)
	}

	if chatResp.Model != "gpt-4" {
		t.Errorf("Expected response model gpt-4, got %s", chatResp.Model)
	}

	if len(chatResp.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(chatResp.Choices))
	}

	expectedContent := "Echo: Hello, world!"
	if chatResp.Choices[0].Message.Content.Content == nil ||
		*chatResp.Choices[0].Message.Content.Content != expectedContent {
		t.Errorf(
			"Expected content %s, got %v",
			expectedContent,
			chatResp.Choices[0].Message.Content,
		)
	}

	// Step 5: Inbound response transformation (ChatCompletionResponse -> HTTP)
	finalResp, err := inbound.TransformResponse(t.Context(), chatResp)
	if err != nil {
		t.Fatalf("Inbound response transformation failed: %v", err)
	}

	if finalResp.StatusCode != http.StatusOK {
		t.Errorf("Expected final response status 200, got %d", finalResp.StatusCode)
	}

	if finalResp.Headers.Get("Content-Type") != "application/json" {
		t.Errorf(
			"Expected Content-Type application/json, got %s",
			finalResp.Headers.Get("Content-Type"),
		)
	}

	// Verify final response body can be unmarshaled back to ChatCompletionResponse
	var finalChatResp llm.Response

	err = json.Unmarshal(finalResp.Body, &finalChatResp)
	if err != nil {
		t.Fatalf("Failed to unmarshal final response: %v", err)
	}

	if finalChatResp.ID != chatResp.ID {
		t.Errorf("Final response ID mismatch: expected %s, got %s", chatResp.ID, finalChatResp.ID)
	}
}

// TestIntegration_StreamingFlow tests the streaming functionality.
func TestIntegration_StreamingFlow(t *testing.T) {
	// Create HTTP client
	httpClient := httpclient.NewHttpClient()

	// Mock streaming server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify streaming headers
		if r.Header.Get("Accept") != "text/event-stream" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "Expected text/event-stream"}`))

			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter does not support flushing")
			return
		}

		// Send streaming events
		events := []string{
			`data: {"id": "chatcmpl-1", "object": "chat.completion.chunk", "choices": [{"delta": {"content": "Hello"}}]}`,
			`data: {"id": "chatcmpl-1", "object": "chat.completion.chunk", "choices": [{"delta": {"content": " world"}}]}`,
			`data: {"id": "chatcmpl-1", "object": "chat.completion.chunk", "choices": [{"delta": {"content": "!"}}]}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			fmt.Fprintf(w, "%s\n\n", event)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	// Create streaming request
	streamReq := &httpclient.Request{
		Method: http.MethodPost,
		URL:    server.URL,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: mustMarshal(llm.Request{
			Model:  "gpt-4",
			Stream: lo.ToPtr(true),
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello, world!"),
					},
				},
			},
		}),
	}

	// Execute streaming request
	stream, err := httpClient.DoStream(t.Context(), streamReq)
	if err != nil {
		t.Fatalf("Streaming request failed: %v", err)
	}
	defer stream.Close()

	// Read events from stream
	eventCount := 0

	for stream.Next() {
		current := stream.Current()
		if current == nil {
			t.Error("Current() returned nil")
			continue
		}

		if len(current.Data) == 0 {
			t.Error("Event data is empty")
		}

		eventCount++

		// Limit to prevent infinite loop in case of issues
		if eventCount > 10 {
			break
		}
	}

	if err := stream.Err(); err != nil {
		t.Errorf("Stream error: %v", err)
	}

	if eventCount == 0 {
		t.Error("No events received from stream")
	}
}
