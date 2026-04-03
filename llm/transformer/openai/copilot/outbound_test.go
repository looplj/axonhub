package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// mockTokenProvider is a mock implementation of TokenProvider for testing.
type mockTokenProvider struct {
	token string
	err   error
}

func (m *mockTokenProvider) GetToken(ctx context.Context) (string, error) {
	return m.token, m.err
}

func TestNewOutboundTransformer(t *testing.T) {
	tests := []struct {
		name        string
		params      OutboundTransformerParams
		wantErr     bool
		errContains string
	}{
		{
			name: "successful creation with defaults",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: "test-token"},
			},
			wantErr: false,
		},
		{
			name: "successful creation with custom base URL",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: "test-token"},
				BaseURL:       "https://custom.copilot.api",
			},
			wantErr: false,
		},
		{
			name: "successful creation with trailing slash in base URL",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: "test-token"},
				BaseURL:       "https://custom.copilot.api/",
			},
			wantErr: false,
		},
		{
			name:        "error when token provider is nil",
			params:      OutboundTransformerParams{},
			wantErr:     true,
			errContains: "token provider is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformer(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, transformer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, transformer)
				assert.NotNil(t, transformer.tokenProvider)
				assert.False(t, strings.HasSuffix(transformer.baseURL, "/"), "base URL should not have trailing slash")
			}
		})
	}
}

func TestOutboundTransformer_APIFormat(t *testing.T) {
	transformer := &OutboundTransformer{}
	assert.Equal(t, llm.APIFormatOpenAIChatCompletion, transformer.APIFormat())
}

func TestUsesResponsesAPI(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{
			name:     "gpt-5 uses responses API",
			model:    "gpt-5",
			expected: true,
		},
		{
			name:     "gpt-5-mini does not use responses API",
			model:    "gpt-5-mini",
			expected: false,
		},
		{
			name:     "gpt-5.3 uses responses API",
			model:    "gpt-5.3",
			expected: true,
		},
		{
			name:     "gpt-5.4 uses responses API",
			model:    "gpt-5.4",
			expected: true,
		},
		{
			name:     "gpt-5.4 preview uses responses API",
			model:    "gpt-5.4-preview",
			expected: true,
		},
		{
			name:     "gpt-5.5 uses responses API",
			model:    "gpt-5.5",
			expected: true,
		},
		{
			name:     "gpt-5.10 uses responses API",
			model:    "gpt-5.10",
			expected: true,
		},
		{
			name:     "gpt-6 uses responses API",
			model:    "gpt-6",
			expected: true,
		},
		{
			name:     "gpt-6.1 uses responses API",
			model:    "gpt-6.1",
			expected: true,
		},
		{
			name:     "gpt-6-preview uses responses API",
			model:    "gpt-6-preview",
			expected: true,
		},
		{
			name:     "regular chat model does not use responses API",
			model:    "gpt-4o",
			expected: false,
		},
		{
			name:     "claude model does not use responses API",
			model:    "claude-sonnet-4.6",
			expected: false,
		},
		{
			name:     "claude-3-5-sonnet does not use responses API",
			model:    "claude-3-5-sonnet",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, usesResponsesAPI(tt.model))
		})
	}
}

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"

	// Create a valid LLM request
	createValidRequest := func() *llm.Request {
		return &llm.Request{
			Model: "gpt-4o",
			Messages: []llm.Message{
				{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr("Hello, Copilot!")},
				},
			},
		}
	}

	tests := []struct {
		name        string
		params      OutboundTransformerParams
		request     *llm.Request
		wantErr     bool
		errContains string
		validate    func(t *testing.T, req *httpclient.Request)
	}{
		{
			name: "successful transformation with default headers",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: mockToken},
			},
			request: createValidRequest(),
			wantErr: false,
			validate: func(t *testing.T, req *httpclient.Request) {
				// Validate method and URL
				assert.Equal(t, "POST", req.Method)
				assert.Equal(t, DefaultCopilotBaseURL+CopilotChatCompletionsEndpoint, req.URL)

				// Validate headers
				assert.Equal(t, "application/json", req.Headers.Get("Content-Type"))
				assert.Equal(t, "application/json", req.Headers.Get("Accept"))

				// Validate LiteLLM-style editor headers
				assert.Equal(t, DefaultEditorVersion, req.Headers.Get(EditorVersionHeader))
				assert.Equal(t, DefaultEditorPluginVersion, req.Headers.Get(EditorPluginVersionHeader))
				assert.Equal(t, DefaultUserAgent, req.Headers.Get(UserAgentHeader))
				assert.Equal(t, DefaultCopilotIntegrationID, req.Headers.Get(CopilotIntegrationIDHeader))
				assert.Equal(t, DefaultOpenAIIntent, req.Headers.Get(OpenAIIntentHeader))

				// Vision header should NOT be present for text-only request
				assert.Empty(t, req.Headers.Get(CopilotVisionRequestHeader))

				// Validate auth config
				assert.NotNil(t, req.Auth)
				assert.Equal(t, httpclient.AuthTypeBearer, req.Auth.Type)
				assert.Equal(t, mockToken, req.Auth.APIKey)

				// Validate API format
				assert.Equal(t, string(llm.APIFormatOpenAIChatCompletion), req.APIFormat)

				// Validate body is valid JSON
				var body map[string]any
				err := json.Unmarshal(req.Body, &body)
				assert.NoError(t, err)
				assert.Equal(t, "gpt-4o", body["model"])
			},
		},
		{
			name:        "error when request is nil",
			params:      OutboundTransformerParams{TokenProvider: &mockTokenProvider{token: mockToken}},
			request:     nil,
			wantErr:     true,
			errContains: "request is nil",
		},
		{
			name:   "error when model is empty",
			params: OutboundTransformerParams{TokenProvider: &mockTokenProvider{token: mockToken}},
			request: &llm.Request{
				Model: "",
				Messages: []llm.Message{
					{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
				},
			},
			wantErr:     true,
			errContains: "model is required",
		},
		{
			name:   "error when messages are empty",
			params: OutboundTransformerParams{TokenProvider: &mockTokenProvider{token: mockToken}},
			request: &llm.Request{
				Model:    "gpt-4o",
				Messages: []llm.Message{},
			},
			wantErr:     true,
			errContains: "messages are required",
		},
		{
			name: "error when token provider fails",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{err: errors.New("token fetch failed")},
			},
			request:     createValidRequest(),
			wantErr:     true,
			errContains: "failed to get copilot token",
		},
		{
			name: "successful transformation with custom base URL",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: mockToken},
				BaseURL:       "https://custom.copilot.github.com",
			},
			request: createValidRequest(),
			wantErr: false,
			validate: func(t *testing.T, req *httpclient.Request) {
				assert.Equal(t, "https://custom.copilot.github.com"+CopilotChatCompletionsEndpoint, req.URL)
			},
		},
		{
			name: "gpt-5.4 uses responses API endpoint",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: mockToken},
			},
			request: &llm.Request{
				Model: "gpt-5.4",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello, Copilot!")},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, req *httpclient.Request) {
				assert.Equal(t, DefaultCopilotBaseURL+"/v1/responses", req.URL)
			},
		},
		{
			name: "codex model uses responses API endpoint",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: mockToken},
			},
			request: &llm.Request{
				Model: "gpt-5.2-codex",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello, Copilot!")},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, req *httpclient.Request) {
				assert.Equal(t, DefaultCopilotBaseURL+"/v1/responses", req.URL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformer(tt.params)
			require.NoError(t, err)

			httpReq, err := transformer.TransformRequest(ctx, tt.request)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, httpReq)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, httpReq)
				if tt.validate != nil {
					tt.validate(t, httpReq)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformRequest_VisionHeaders(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"
	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
	})
	require.NoError(t, err)

	tests := []struct {
		name         string
		request      *llm.Request
		expectVision bool
		visionValue  string
	}{
		{
			name: "text only - no vision header",
			request: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Just text")},
					},
				},
			},
			expectVision: false,
		},
		{
			name: "image_url type - vision header present",
			request: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type:     "image_url",
									ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"},
								},
								{
									Type: "text",
									Text: lo.ToPtr("What's in this image?"),
								},
							},
						},
					},
				},
			},
			expectVision: true,
			visionValue:  "true",
		},
		{
			name: "data:image URL in text - vision header present",
			request: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/png;base64,iVBORw0KGgo...")},
					},
				},
			},
			expectVision: true,
			visionValue:  "true",
		},
		{
			name: "data:image URL in multiple content - vision header present",
			request: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("data:image/jpeg;base64,/9j/4AAQ..."),
								},
							},
						},
					},
				},
			},
			expectVision: true,
			visionValue:  "true",
		},
		{
			name: "mixed text and image - vision header present",
			request: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role:    "system",
						Content: llm.MessageContent{Content: lo.ToPtr("You are a helpful assistant")},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("Describe this:"),
								},
								{
									Type:     "image_url",
									ImageURL: &llm.ImageURL{URL: "https://example.com/photo.jpg", Detail: lo.ToPtr("high")},
								},
							},
						},
					},
				},
			},
			expectVision: true,
			visionValue:  "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq, err := transformer.TransformRequest(ctx, tt.request)
			require.NoError(t, err)
			require.NotNil(t, httpReq)

			visionHeader := httpReq.Headers.Get(CopilotVisionRequestHeader)
			if tt.expectVision {
				assert.Equal(t, tt.visionValue, visionHeader)
			} else {
				assert.Empty(t, visionHeader)
			}
		})
	}
}

func TestXInitiatorDefault(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"
	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
	})
	require.NoError(t, err)

	request := &llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
			},
		},
	}

	httpReq, err := transformer.TransformRequest(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, httpReq)
	assert.Equal(t, "agent", httpReq.Headers.Get(InitiatorHeader))
}

func TestXInitiatorForwarding(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"
	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
	})
	require.NoError(t, err)

	tests := []struct {
		name           string
		initiatorValue string
		expected       string
	}{
		{
			name:           "forwards custom initiator value",
			initiatorValue: "editor",
			expected:       "editor",
		},
		{
			name:           "forwards agent initiator value",
			initiatorValue: "agent",
			expected:       "agent",
		},
		{
			name:           "forwards empty string as empty",
			initiatorValue: "",
			expected:       "agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
				RawRequest: &httpclient.Request{
					Headers: make(http.Header),
				},
			}
			if tt.initiatorValue != "" {
				request.RawRequest.Headers.Set(InitiatorHeader, tt.initiatorValue)
			}

			httpReq, err := transformer.TransformRequest(ctx, request)
			require.NoError(t, err)
			require.NotNil(t, httpReq)
			assert.Equal(t, tt.expected, httpReq.Headers.Get(InitiatorHeader))
		})
	}
}

func TestHasVisionContent(t *testing.T) {
	tests := []struct {
		name     string
		request  *llm.Request
		expected bool
	}{
		{
			name: "text only - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Just text")},
					},
				},
			},
			expected: false,
		},
		{
			name: "image_url type - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type:     "image_url",
									ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"},
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "image_url with nil ImageURL - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type:     "image_url",
									ImageURL: nil,
								},
							},
						},
					},
				},
			},
			expected: true, // Type is image_url, so it should detect vision
		},
		{
			name: "data:image in single content - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/png;base64,abc123")},
					},
				},
			},
			expected: true,
		},
		{
			name: "data:image/jpeg in single content - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/jpeg;base64,xyz789")},
					},
				},
			},
			expected: true,
		},
		{
			name: "data:image/webp in single content - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/webp;base64,webp123")},
					},
				},
			},
			expected: true,
		},
		{
			name: "data:image in multiple content text field - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("data:image/gif;base64,gif456"),
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "text with data: prefix but not image - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:text/plain;base64,hello")},
					},
				},
			},
			expected: false,
		},
		{
			name: "regular URL with image in path - no vision (not data URL)",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("https://example.com/images/photo.png")},
					},
				},
			},
			expected: false,
		},
		{
			name: "empty content - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("")},
					},
				},
			},
			expected: false,
		},
		{
			name: "nil content - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{},
					},
				},
			},
			expected: false,
		},
		{
			name: "text type in multiple content - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("Just text content"),
								},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "mixed content with image - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "system",
						Content: llm.MessageContent{Content: lo.ToPtr("You are helpful")},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("Look at this:"),
								},
								{
									Type:     "image_url",
									ImageURL: &llm.ImageURL{URL: "https://example.com/img.png"},
								},
							},
						},
					},
					{
						Role:    "assistant",
						Content: llm.MessageContent{Content: lo.ToPtr("I see the image")},
					},
				},
			},
			expected: true,
		},
		{
			name: "multiple messages with data URL - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("First message")},
					},
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/png;base64,second")},
					},
				},
			},
			expected: true,
		},
		{
			name: "assistant message with vision - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "assistant",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/png;base64,assistant")},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasVisionContent(tt.request)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsImageDataURL(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "PNG data URL",
			content:  "data:image/png;base64,iVBORw0KGgo=",
			expected: true,
		},
		{
			name:     "JPEG data URL",
			content:  "data:image/jpeg;base64,/9j/4AAQ=",
			expected: true,
		},
		{
			name:     "WEBP data URL",
			content:  "data:image/webp;base64,UklGR=",
			expected: true,
		},
		{
			name:     "GIF data URL",
			content:  "data:image/gif;base64,R0lGOD=",
			expected: true,
		},
		{
			name:     "SVG data URL",
			content:  "data:image/svg+xml;base64,PHN2Zw=",
			expected: true,
		},
		{
			name:     "Plain text",
			content:  "Hello, world!",
			expected: false,
		},
		{
			name:     "Regular HTTP URL with image",
			content:  "https://example.com/image.png",
			expected: false,
		},
		{
			name:     "data:text URL",
			content:  "data:text/plain;base64,SGVsbG8=",
			expected: false,
		},
		{
			name:     "data:application URL",
			content:  "data:application/pdf;base64,JVBERi0=",
			expected: false,
		},
		{
			name:     "Empty string",
			content:  "",
			expected: false,
		},
		{
			name:     "Partial data prefix",
			content:  "data:ima",
			expected: false,
		},
		{
			name:     "Case sensitive - uppercase",
			content:  "DATA:IMAGE/PNG;base64,ABC=",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isImageDataURL(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	ctx := context.Background()
	transformer := &OutboundTransformer{}

	tests := []struct {
		name        string
		httpResp    *httpclient.Response
		wantErr     bool
		errContains string
		validate    func(t *testing.T, resp *llm.Response)
	}{
		{
			name:        "error when http response is nil",
			httpResp:    nil,
			wantErr:     true,
			errContains: "http response is nil",
		},
		{
			name: "error on HTTP 400 status",
			httpResp: &httpclient.Response{
				StatusCode: 400,
				Body:       []byte(`{"error": "bad request"}`),
			},
			wantErr:     true,
			errContains: "HTTP error 400",
		},
		{
			name: "error on HTTP 500 status",
			httpResp: &httpclient.Response{
				StatusCode: 500,
				Body:       []byte(`{"error": "internal error"}`),
			},
			wantErr:     true,
			errContains: "HTTP error 500",
		},
		{
			name: "error when body is empty",
			httpResp: &httpclient.Response{
				StatusCode: 200,
				Body:       []byte{},
			},
			wantErr:     true,
			errContains: "response body is empty",
		},
		{
			name: "error when body is invalid JSON",
			httpResp: &httpclient.Response{
				StatusCode: 200,
				Body:       []byte(`not valid json`),
			},
			wantErr:     true,
			errContains: "failed to unmarshal response",
		},
		{
			name: "successful transformation",
			httpResp: &httpclient.Response{
				StatusCode: 200,
				Body: []byte(`{
					"id": "chatcmpl-123",
					"object": "chat.completion",
					"created": 1700000000,
					"model": "gpt-4o",
					"choices": [
						{
							"index": 0,
							"message": {
								"role": "assistant",
								"content": "Hello! How can I help you today?"
							},
							"finish_reason": "stop"
						}
					]
				}`),
			},
			wantErr: false,
			validate: func(t *testing.T, resp *llm.Response) {
				assert.Equal(t, "chatcmpl-123", resp.ID)
				assert.Equal(t, "chat.completion", resp.Object)
				assert.Equal(t, "gpt-4o", resp.Model)
				assert.Len(t, resp.Choices, 1)
				assert.Equal(t, "assistant", resp.Choices[0].Message.Role)
				if resp.Choices[0].Message.Content.Content != nil {
					assert.Equal(t, "Hello! How can I help you today?", *resp.Choices[0].Message.Content.Content)
				}
				if resp.Choices[0].FinishReason != nil {
					assert.Equal(t, "stop", *resp.Choices[0].FinishReason)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := transformer.TransformResponse(ctx, tt.httpResp)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.validate != nil {
					tt.validate(t, resp)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformError(t *testing.T) {
	ctx := context.Background()
	transformer := &OutboundTransformer{}

	tests := []struct {
		name     string
		rawErr   *httpclient.Error
		validate func(t *testing.T, respErr *llm.ResponseError)
	}{
		{
			name:   "nil error - returns generic error",
			rawErr: nil,
			validate: func(t *testing.T, respErr *llm.ResponseError) {
				assert.Equal(t, 500, respErr.StatusCode)
				assert.Equal(t, "Internal Server Error", respErr.Detail.Message)
				assert.Equal(t, "api_error", respErr.Detail.Type)
			},
		},
		{
			name: "error with OpenAI format - error field",
			rawErr: &httpclient.Error{
				StatusCode: 401,
				Body:       []byte(`{"error": {"message": "Invalid API key", "type": "authentication_error"}}`),
			},
			validate: func(t *testing.T, respErr *llm.ResponseError) {
				assert.Equal(t, 401, respErr.StatusCode)
				assert.Equal(t, "Invalid API key", respErr.Detail.Message)
				assert.Equal(t, "authentication_error", respErr.Detail.Type)
			},
		},
		{
			name: "error with OpenAI format - errors field",
			rawErr: &httpclient.Error{
				StatusCode: 429,
				Body:       []byte(`{"errors": {"message": "Rate limit exceeded", "type": "rate_limit_error"}}`),
			},
			validate: func(t *testing.T, respErr *llm.ResponseError) {
				assert.Equal(t, 429, respErr.StatusCode)
				assert.Equal(t, "Rate limit exceeded", respErr.Detail.Message)
				assert.Equal(t, "rate_limit_error", respErr.Detail.Type)
			},
		},
		{
			name: "error with non-JSON body - uses status text",
			rawErr: &httpclient.Error{
				StatusCode: 503,
				Body:       []byte(`service unavailable`),
			},
			validate: func(t *testing.T, respErr *llm.ResponseError) {
				assert.Equal(t, 503, respErr.StatusCode)
				assert.Equal(t, "Service Unavailable", respErr.Detail.Message)
				assert.Equal(t, "api_error", respErr.Detail.Type)
			},
		},
		{
			name: "error with empty body - uses status text",
			rawErr: &httpclient.Error{
				StatusCode: 502,
				Body:       []byte{},
			},
			validate: func(t *testing.T, respErr *llm.ResponseError) {
				assert.Equal(t, 502, respErr.StatusCode)
				assert.Equal(t, "Bad Gateway", respErr.Detail.Message)
				assert.Equal(t, "api_error", respErr.Detail.Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respErr := transformer.TransformError(ctx, tt.rawErr)
			assert.NotNil(t, respErr)
			tt.validate(t, respErr)
		})
	}
}
