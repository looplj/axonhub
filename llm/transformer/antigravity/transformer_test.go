package antigravity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/gemini"
)

// mockRoundTripper implements http.RoundTripper.
type mockRoundTripper struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestTransformRequest_Antigravity(t *testing.T) {
	// Use OAuth credentials format: refreshToken|projectID
	config := Config{
		BaseURL: "https://api.antigravity.dev",
		APIKey:  "test-refresh-token|test-project-id",
		Project: "my-project",
	}

	// Mock HTTP Client for OAuth
	mockRT := &mockRoundTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == TokenURL {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(bytes.NewBufferString(`{
						"access_token": "mock-access-token",
						"token_type": "Bearer",
						"expires_in": 3600
					}`)),
					Header: make(http.Header),
				}, nil
			}

			return &http.Response{StatusCode: http.StatusNotFound}, nil
		},
	}
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: mockRT,
	})

	transformer, err := NewTransformer(config, WithHTTPClient(httpClient))
	require.NoError(t, err)

	t.Run("basic request structure", func(t *testing.T) {
		req := &llm.Request{
			Model: "gemini-2.5-flash",
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), req)
		require.NoError(t, err)

		// Check Headers
		assert.Equal(t, "application/json", httpReq.Headers.Get("Content-Type"))
		assert.Equal(t, UserAgent, httpReq.Headers.Get("User-Agent"))
		assert.Equal(t, ApiClient, httpReq.Headers.Get("X-Goog-Api-Client"))
		assert.Equal(t, ClientMetadata, httpReq.Headers.Get("Client-Metadata"))
		// Since we mocked the token response, we expect the mock access token in Auth config
		require.NotNil(t, httpReq.Auth)
		assert.Equal(t, httpclient.AuthTypeBearer, httpReq.Auth.Type)
		assert.Equal(t, "mock-access-token", httpReq.Auth.APIKey)

		// Check URL
		assert.Equal(t, "https://api.antigravity.dev/v1internal:generateContent", httpReq.URL)

		// Check Body (Envelope)
		var envelope AntigravityEnvelope
		err = json.Unmarshal(httpReq.Body, &envelope)
		require.NoError(t, err)

		assert.Equal(t, "my-project", envelope.Project)
		assert.Equal(t, "gemini-2.5-flash", envelope.Model)

		// Check Inner Request
		// Request is interface{}, marshal and unmarshal to specific type
		innerBytes, _ := json.Marshal(envelope.Request)
		var innerReq gemini.GenerateContentRequest
		err = json.Unmarshal(innerBytes, &innerReq)
		require.NoError(t, err)

		assert.Len(t, innerReq.Contents, 1)
		assert.Equal(t, "Hello", innerReq.Contents[0].Parts[0].Text)
	})

	t.Run("schema sanitization", func(t *testing.T) {
		// Create a schema with unsupported features (const)
		schemaJSON := `{"type":"object","properties":{"mode":{"const":"auto"}}}`

		req := &llm.Request{
			Model: "claude-3-5-sonnet",
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Gen JSON")}},
			},
			ResponseFormat: &llm.ResponseFormat{
				Type:       "json_schema",
				JSONSchema: json.RawMessage(schemaJSON),
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), req)
		require.NoError(t, err)

		var envelope AntigravityEnvelope

		err = json.Unmarshal(httpReq.Body, &envelope)
		require.NoError(t, err)

		innerBytes, err := json.Marshal(envelope.Request)
		require.NoError(t, err)
		var innerReq gemini.GenerateContentRequest

		err = json.Unmarshal(innerBytes, &innerReq)
		require.NoError(t, err)

		require.NotNil(t, innerReq.GenerationConfig)
		require.NotEmpty(t, innerReq.GenerationConfig.ResponseSchema)

		var sanitizedSchema map[string]any

		err = json.Unmarshal(innerReq.GenerationConfig.ResponseSchema, &sanitizedSchema)
		require.NoError(t, err)

		// Verify const became enum
		props, _ := sanitizedSchema["properties"].(map[string]any)
		mode, _ := props["mode"].(map[string]any)
		assert.Nil(t, mode["const"])
		assert.Equal(t, []any{"auto"}, mode["enum"])
	})

	t.Run("tool configuration for Claude", func(t *testing.T) {
		req := &llm.Request{
			Model: "claude-3-5-sonnet",
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Help")}},
			},
			Tools: []llm.Tool{
				{
					Type: "function",
					Function: llm.Function{
						Name:       "test_tool",
						Parameters: json.RawMessage(`{"type":"object"}`),
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), req)
		require.NoError(t, err)

		var envelope AntigravityEnvelope

		err = json.Unmarshal(httpReq.Body, &envelope)
		require.NoError(t, err)

		innerBytes, err := json.Marshal(envelope.Request)
		require.NoError(t, err)
		var innerReq gemini.GenerateContentRequest

		err = json.Unmarshal(innerBytes, &innerReq)
		require.NoError(t, err)

		// Check ToolConfig
		require.NotNil(t, innerReq.ToolConfig)
		require.NotNil(t, innerReq.ToolConfig.FunctionCallingConfig)
		assert.Equal(t, "VALIDATED", innerReq.ToolConfig.FunctionCallingConfig.Mode)

		// Check System Instruction Hardening
		require.NotNil(t, innerReq.SystemInstruction)
		require.Greater(t, len(innerReq.SystemInstruction.Parts), 0)
		assert.Contains(t, innerReq.SystemInstruction.Parts[0].Text, "CRITICAL: DO NOT guess tool parameters")
	})

	t.Run("strip thinking for Claude", func(t *testing.T) {
		req := &llm.Request{
			Model: "claude-3-5-sonnet",
			Messages: []llm.Message{
				{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
				},
				{
					Role:             "assistant",
					ReasoningContent: lo.ToPtr("I am thinking..."),
					Content:          llm.MessageContent{Content: lo.ToPtr("Hi")},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), req)
		require.NoError(t, err)

		var envelope AntigravityEnvelope

		err = json.Unmarshal(httpReq.Body, &envelope)
		require.NoError(t, err)

		innerBytes, err := json.Marshal(envelope.Request)
		require.NoError(t, err)
		var innerReq gemini.GenerateContentRequest

		err = json.Unmarshal(innerBytes, &innerReq)
		require.NoError(t, err)

		require.Len(t, innerReq.Contents, 2)
		// Assistant message should only have the text part, reasoning/thought part stripped
		// Note: helper converts ReasoningContent to a Part with Thought=true.
		// Our patch logic strips parts with Thought=true for Claude.

		assistantContent := innerReq.Contents[1]
		assert.Equal(t, "model", assistantContent.Role)
		assert.Len(t, assistantContent.Parts, 1)
		assert.Equal(t, "Hi", assistantContent.Parts[0].Text)
		assert.False(t, assistantContent.Parts[0].Thought)
	})

	t.Run("tool parameters use 'parameters' field not 'parametersJsonSchema'", func(t *testing.T) {
		// CRITICAL: Antigravity API expects "parameters" field, not "parametersJsonSchema"
		// This test verifies the fix for the malformed tool call issue
		req := &llm.Request{
			Model: "gemini-3-pro",
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Help")}},
			},
			Tools: []llm.Tool{
				{
					Type: "function",
					Function: llm.Function{
						Name:        "test_tool",
						Description: "A test tool",
						Parameters:  json.RawMessage(`{"type":"object","properties":{"arg":{"type":"string"}},"required":["arg"]}`),
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), req)
		require.NoError(t, err)

		var envelope AntigravityEnvelope
		err = json.Unmarshal(httpReq.Body, &envelope)
		require.NoError(t, err)

		// Unmarshal the inner request to check tool format
		innerBytes, err := json.Marshal(envelope.Request)
		require.NoError(t, err)

		// Parse as raw JSON to check exact field names
		var rawInner map[string]any
		err = json.Unmarshal(innerBytes, &rawInner)
		require.NoError(t, err)

		// Navigate to tools
		tools, ok := rawInner["tools"].([]any)
		require.True(t, ok, "tools should be an array")
		require.NotEmpty(t, tools, "tools should not be empty")

		firstTool, ok := tools[0].(map[string]any)
		require.True(t, ok, "first tool should be a map")

		functionDeclarations, ok := firstTool["functionDeclarations"].([]any)
		require.True(t, ok, "functionDeclarations should be an array")
		require.NotEmpty(t, functionDeclarations, "functionDeclarations should not be empty")

		firstDecl, ok := functionDeclarations[0].(map[string]any)
		require.True(t, ok, "first declaration should be a map")

		// CRITICAL: Must have "parameters" field, not "parametersJsonSchema"
		_, hasParameters := firstDecl["parameters"]
		_, hasParametersJsonSchema := firstDecl["parametersJsonSchema"]

		assert.True(t, hasParameters, "tool declaration must have 'parameters' field")
		assert.False(t, hasParametersJsonSchema, "tool declaration must NOT have 'parametersJsonSchema' field")

		// Verify the parameters content is correct
		parameters, ok := firstDecl["parameters"].(map[string]any)
		require.True(t, ok, "parameters should be a map")
		assert.Equal(t, "OBJECT", parameters["type"]) // UPPERCASE, as Antigravity API expects
		assert.NotNil(t, parameters["properties"])
	})
}

// TestOAuthFailureNoFallback verifies that when OAuth token retrieval fails, an error is returned (not fallback to API key)
func TestOAuthFailureNoFallback(t *testing.T) {
	config := Config{
		BaseURL: "https://api.antigravity.dev",
		APIKey:  "test-refresh-token|test-project-id",
		Project: "my-project",
	}

	// Mock HTTP Client that fails token retrieval
	mockRT := &mockRoundTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == TokenURL {
				return nil, fmt.Errorf("token retrieval failed")
			}
			return &http.Response{StatusCode: http.StatusNotFound}, nil
		},
	}
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: mockRT,
	})

	transformer, err := NewTransformer(config, WithHTTPClient(httpClient))
	require.NoError(t, err)

	req := &llm.Request{
		Model: "gemini-2.5-flash",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
		},
	}

	_, err = transformer.TransformRequest(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get OAuth token")
}

// TestNoAPIKeyAuthConfig verifies that AuthConfig is never set to api_key type
func TestNoAPIKeyAuthConfig(t *testing.T) {
	config := Config{
		BaseURL: "https://api.antigravity.dev",
		APIKey:  "test-refresh-token|test-project-id",
		Project: "my-project",
	}

	// Mock successful OAuth token retrieval
	mockRT := &mockRoundTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == TokenURL {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(bytes.NewBufferString(`{
						"access_token": "mock-access-token",
						"token_type": "Bearer",
						"expires_in": 3600
					}`)),
					Header: make(http.Header),
				}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound}, nil
		},
	}
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: mockRT,
	})

	transformer, err := NewTransformer(config, WithHTTPClient(httpClient))
	require.NoError(t, err)

	req := &llm.Request{
		Model: "gemini-2.5-flash",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
		},
	}

	httpReq, err := transformer.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	// Verify Auth config is set with Bearer token (OAuth)
	require.NotNil(t, httpReq.Auth)
	assert.Equal(t, httpclient.AuthTypeBearer, httpReq.Auth.Type)
	assert.Equal(t, "mock-access-token", httpReq.Auth.APIKey)

	// Verify no X-Goog-Api-Key header is set
	assert.Empty(t, httpReq.Headers.Get("X-Goog-Api-Key"))
}

// TestOAuthOnlyHeaders verifies that X-Goog-Api-Key header is never sent
func TestOAuthOnlyHeaders(t *testing.T) {
	config := Config{
		BaseURL: "https://api.antigravity.dev",
		APIKey:  "test-refresh-token|test-project-id",
		Project: "my-project",
	}

	// Mock successful OAuth token retrieval
	mockRT := &mockRoundTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == TokenURL {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(bytes.NewBufferString(`{
						"access_token": "oauth-access-token",
						"token_type": "Bearer",
						"expires_in": 3600
					}`)),
					Header: make(http.Header),
				}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound}, nil
		},
	}
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: mockRT,
	})

	transformer, err := NewTransformer(config, WithHTTPClient(httpClient))
	require.NoError(t, err)

	req := &llm.Request{
		Model: "claude-3-5-sonnet",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Test")}},
		},
	}

	httpReq, err := transformer.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	// Verify Auth config is set correctly with Bearer token
	require.NotNil(t, httpReq.Auth)
	assert.Equal(t, httpclient.AuthTypeBearer, httpReq.Auth.Type)
	assert.Equal(t, "oauth-access-token", httpReq.Auth.APIKey)

	// Verify X-Goog-Api-Key header is NOT present
	assert.Empty(t, httpReq.Headers.Get("X-Goog-Api-Key"), "X-Goog-Api-Key header should not be set when using OAuth")

	// Verify other required headers are present
	assert.Equal(t, "application/json", httpReq.Headers.Get("Content-Type"))
	assert.Equal(t, UserAgent, httpReq.Headers.Get("User-Agent"))
	assert.Equal(t, ApiClient, httpReq.Headers.Get("X-Goog-Api-Client"))
	assert.Equal(t, ClientMetadata, httpReq.Headers.Get("Client-Metadata"))
}
