package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/gemini"
)

// Token exchange timeout - maximum time to wait for token refresh/retrieval.
const tokenExchangeTimeout = 30 * time.Second

// Option is a functional option for Transformer.
type Option func(*Transformer)

// WithHTTPClient sets the HTTP client.
func WithHTTPClient(client *httpclient.HttpClient) Option {
	return func(t *Transformer) {
		t.httpClient = client
	}
}

// WithOnTokenRefreshed sets the callback for token refresh events.
func WithOnTokenRefreshed(onRefreshed func(ctx context.Context, refreshed *oauth.OAuthCredentials) error) Option {
	return func(t *Transformer) {
		t.onTokenRefreshed = onRefreshed
	}
}

// GetTokenProvider returns the OAuth token provider.
func (t *Transformer) GetTokenProvider() *oauth.TokenProvider {
	return t.tokenProvider
}

// Config holds configuration for Antigravity transformer.
type Config struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Project string `json:"project"` // Google Cloud Project ID
}

// Transformer implements transformer.Outbound for Antigravity protocol.
type Transformer struct {
	config            Config
	geminiTransformer transformer.Outbound
	tokenProvider     *oauth.TokenProvider
	httpClient        *httpclient.HttpClient
	onTokenRefreshed  func(ctx context.Context, refreshed *oauth.OAuthCredentials) error
}

// NewTransformer creates a new Antigravity Transformer.
func NewTransformer(config Config, opts ...Option) (*Transformer, error) {
	// Initialize a Gemini transformer for internal use
	gt, err := gemini.NewOutboundTransformer(config.BaseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create internal gemini transformer: %w", err)
	}

	t := &Transformer{
		config:            config,
		geminiTransformer: gt,
	}

	for _, opt := range opts {
		opt(t)
	}

	if config.APIKey != "" {
		t.initTokenProvider(config.APIKey)
		// If Project is not explicitly set, try to extract it from credentials
		if t.config.Project == "" {
			_, projectID := parseCredentials(config.APIKey)
			if projectID != "" {
				t.config.Project = projectID
			}
		}
	}

	return t, nil
}

func (t *Transformer) initTokenProvider(apiKey string) {
	refreshToken, _ := parseCredentials(apiKey)
	if refreshToken == "" {
		return
	}

	httpClient := t.httpClient
	if httpClient == nil {
		// Create a new client with appropriate timeouts for token exchange
		httpClient = httpclient.NewHttpClientWithClient(&http.Client{
			Timeout: tokenExchangeTimeout,
			Transport: &http.Transport{
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		})
	}

	t.tokenProvider = NewTokenProvider(oauth.TokenProviderParams{
		Credentials: &oauth.OAuthCredentials{
			RefreshToken: refreshToken,
			ClientID:     ClientID,
			Scopes:       Scopes,
		},
		HTTPClient:  httpClient,
		OnRefreshed: t.onTokenRefreshed,
	})
}

func parseCredentials(creds string) (string, string) {
	parts := strings.Split(creds, "|")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}

	return creds, ""
}

// APIFormat returns the API format of the transformer.
func (t *Transformer) APIFormat() llm.APIFormat {
	return llm.APIFormatGeminiContents
}

// TransformRequest transforms the unified request to Antigravity HTTP request.
func (t *Transformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// Aggressively strip client headers that shouldn't leak or be merged back by the pipeline
	if llmReq.RawRequest != nil && llmReq.RawRequest.Headers != nil {
		headersToStrip := []string{
			"User-Agent",
			"Authorization",
			"Content-Type",
			"Accept",
			"Content-Length",
			"Host",
			"Connection",
			"Pragma",
			"Cache-Control",
			"Client-Metadata",
			"X-Goog-Api-Client",
			"X-Goog-Api-Key",
		}
		for _, h := range headersToStrip {
			llmReq.RawRequest.Headers.Del(h)
		}
	}

	// 1. Use Gemini transformer to get the base request body (serialized)
	// We pass the llmReq to geminiTransformer.TransformRequest.
	// This gives us an httpclient.Request with Body being JSON of GenerateContentRequest.
	geminiHttpReq, err := t.geminiTransformer.TransformRequest(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("gemini conversion failed: %w", err)
	}

	// 2. Unmarshal back to GenerateContentRequest struct to patch it
	var geminiReq gemini.GenerateContentRequest
	if err := json.Unmarshal(geminiHttpReq.Body, &geminiReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gemini request body: %w", err)
	}

	// 3. Patch Gemini request for Antigravity
	if err := t.patchGeminiRequest(ctx, &geminiReq, llmReq); err != nil {
		return nil, err
	}

	// 4. Transform model name for Antigravity API compatibility
	// Antigravity API requires tier suffixes for gemini-3-pro (e.g., gemini-3-pro-low)
	// Store the original model name in metadata for the executor to use for routing
	transformedModel := transformModelForAntigravity(llmReq.Model)

	// 5. Wrap in Antigravity Envelope
	envelope := NewAntigravityEnvelope(t.config.Project, transformedModel, geminiReq)

	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal antigravity envelope: %w", err)
	}

	// 5. Build new Headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("User-Agent", UserAgent)
	headers.Set("X-Goog-Api-Client", ApiClient)
	headers.Set("Client-Metadata", ClientMetadata)
	headers.Set("X-Opencode-Tools-Debug", "1")
	// DO NOT set X-Goog-Api-Key header when using OAuth - it must be absent entirely
	// Setting it to empty string triggers license error #3501

	if llmReq.Stream != nil && *llmReq.Stream {
		headers.Set("Accept", "text/event-stream")
	} else {
		headers.Set("Accept", "application/json")
	}

	// Auth
	var auth *httpclient.AuthConfig

	if t.tokenProvider != nil {
		creds, err := t.tokenProvider.Get(ctx)
		if err == nil {
			headers.Set("Authorization", "Bearer "+creds.AccessToken)
		} else {
			slog.WarnContext(ctx, "failed to get oauth token, attempting fallback to api key", slog.Any("error", err))

			if t.config.APIKey != "" {
				auth = &httpclient.AuthConfig{
					Type:      "api_key",
					APIKey:    t.config.APIKey,
					HeaderKey: "x-goog-api-key",
				}
			}
		}
	} else if t.config.APIKey != "" {
		// Fallback to API Key if no token provider (legacy/testing?)
		auth = &httpclient.AuthConfig{
			Type:      "api_key",
			APIKey:    t.config.APIKey,
			HeaderKey: "x-goog-api-key",
		}
	}

	// URL
	url := t.buildURL(llmReq)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}

	// Store the original model name in metadata for executor routing
	if httpReq.Metadata == nil {
		httpReq.Metadata = make(map[string]string)
	}

	httpReq.Metadata["antigravity_model"] = llmReq.Model

	return httpReq, nil
}

func (t *Transformer) patchGeminiRequest(ctx context.Context, req *gemini.GenerateContentRequest, llmReq *llm.Request) error {
	// A. Schema Sanitization
	if req.GenerationConfig != nil && len(req.GenerationConfig.ResponseSchema) > 0 {
		var schema map[string]any
		if err := json.Unmarshal(req.GenerationConfig.ResponseSchema, &schema); err == nil {
			sanitized := SanitizeJSONSchema(schema)
			// CRITICAL: Uppercase all type values for Antigravity API
			sanitized = UppercaseSchemaTypes(sanitized)
			req.GenerationConfig.ResponseSchema = xjson.MustMarshal(sanitized)
		} else {
			slog.DebugContext(ctx, "failed to unmarshal response schema", slog.Any("error", err))
		}
	}

	// Sanitize tool schemas and convert to Antigravity format
	hasTools := false
	for _, tool := range req.Tools {
		for _, fd := range tool.FunctionDeclarations {
			hasTools = true

			// CRITICAL: Antigravity API expects "parameters" field, not "parametersJsonSchema"
			// The Gemini transformer sets ParametersJsonSchema, but Antigravity uses strict
			// protobuf validation and only accepts "parameters" (lowercase, no camelCase)

			// Priority: ParametersJsonSchema first (set by Gemini transformer), then Parameters
			var schemaData json.RawMessage
			if len(fd.ParametersJsonSchema) > 0 {
				schemaData = fd.ParametersJsonSchema
			} else if len(fd.Parameters) > 0 {
				schemaData = fd.Parameters
			}

			if len(schemaData) > 0 {
				var schema map[string]any
				if err := json.Unmarshal(schemaData, &schema); err == nil {
					// Apply sanitization transformations
					sanitized := SanitizeJSONSchema(schema)
					// CRITICAL: Uppercase all type values (object -> OBJECT, string -> STRING)
					// Antigravity API expects uppercase types per protobuf spec
					// Reference: opencode-antigravity-auth/src/plugin/transform/gemini.ts toGeminiSchema()
					sanitized = UppercaseSchemaTypes(sanitized)
					// Set to Parameters field (what Antigravity expects)
					fd.Parameters = xjson.MustMarshal(sanitized)
					// Clear ParametersJsonSchema to avoid sending both
					fd.ParametersJsonSchema = nil
				} else {
					slog.DebugContext(ctx, "failed to unmarshal tool parameters", slog.String("tool", fd.Name), slog.Any("error", err))
				}
			}
		}
	}

	// B. Tool Config (VALIDATED mode for Claude/Antigravity)
	if hasTools {
		// Enforce VALIDATED mode
		if req.ToolConfig == nil {
			req.ToolConfig = &gemini.ToolConfig{}
		}
		if req.ToolConfig.FunctionCallingConfig == nil {
			req.ToolConfig.FunctionCallingConfig = &gemini.FunctionCallingConfig{}
		}
		req.ToolConfig.FunctionCallingConfig.Mode = "VALIDATED"

		// C. Tool Hardening Instruction
		hardeningMsg := "CRITICAL: DO NOT guess tool parameters. ONLY use the exact parameter structure defined in the tool schema. Parameter names are EXACT."

		if req.SystemInstruction == nil {
			req.SystemInstruction = &gemini.Content{
				Parts: []*gemini.Part{{Text: hardeningMsg}},
			}
		} else {
			// Append to existing system instruction
			req.SystemInstruction.Parts = append(req.SystemInstruction.Parts, &gemini.Part{Text: "\n\n" + hardeningMsg})
		}
	}

	// Inject Antigravity System Instruction (required for CLIProxy compatibility)
	// Sets role to "user" and prepends the instruction.
	if req.SystemInstruction == nil {
		req.SystemInstruction = &gemini.Content{
			Parts: []*gemini.Part{{Text: ANTIGRAVITY_SYSTEM_INSTRUCTION}},
		}
	} else if len(req.SystemInstruction.Parts) > 0 {
		firstPart := req.SystemInstruction.Parts[0]
		firstPart.Text = ANTIGRAVITY_SYSTEM_INSTRUCTION + "\n\n" + firstPart.Text
	} else {
		req.SystemInstruction.Parts = append([]*gemini.Part{{Text: ANTIGRAVITY_SYSTEM_INSTRUCTION}}, req.SystemInstruction.Parts...)
	}
	// Crucial: Set role to "user" for system instruction
	req.SystemInstruction.Role = "user"

	// D. Thinking / Cross-Model Sanitization
	isClaude := strings.Contains(strings.ToLower(llmReq.Model), "claude")
	isGemini3 := strings.Contains(strings.ToLower(llmReq.Model), "gemini-3")

	// Fix Thinking Config for Gemini 3 vs 2.5
	if isGemini3 {
		if req.GenerationConfig == nil {
			req.GenerationConfig = &gemini.GenerationConfig{}
		}

		if req.GenerationConfig.ThinkingConfig == nil {
			// Default to 'low' thinking level for Gemini 3 as per reference
			// Gemini 3 Flash/Pro require thinkingLevel (string) not budget
			req.GenerationConfig.ThinkingConfig = &gemini.ThinkingConfig{
				IncludeThoughts: true,
				ThinkingLevel:   "low",
			}
		}
	}

	if req.GenerationConfig != nil && req.GenerationConfig.ThinkingConfig != nil {
		if isGemini3 {
			// Gemini 3 uses ThinkingLevel (string)
			// Ensure Budget is nil if we are enforcing strictness
			req.GenerationConfig.ThinkingConfig.ThinkingBudget = nil
		}
	}

	if isClaude {
		for _, content := range req.Contents {
			// Filter parts
			var newParts []*gemini.Part
			for _, part := range content.Parts {
				// Strip thinking parts
				if part.Thought {
					continue
				}
				newParts = append(newParts, part)
			}
			content.Parts = newParts
		}
	}

	return nil
}

func (t *Transformer) buildURL(llmReq *llm.Request) string {
	action := "generateContent"
	if llmReq.Stream != nil && *llmReq.Stream {
		action = "streamGenerateContent?alt=sse"
	}
	return fmt.Sprintf("%s/v1internal:%s", strings.TrimSuffix(t.config.BaseURL, "/"), action)
}

// TransformResponse transforms Antigravity response to unified response.
func (t *Transformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, string(httpResp.Body))
	}

	// Antigravity returns { "response": { "candidates": [...] } }
	// We need to unwrap it before passing to Gemini transformer
	var envelope struct {
		Response json.RawMessage `json:"response"`
	}

	if err := json.Unmarshal(httpResp.Body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal antigravity response envelope: %w", err)
	}

	if len(envelope.Response) == 0 {
		return nil, fmt.Errorf("empty response field in antigravity response")
	}

	// Create a fake HTTP response with the unwrapped body
	fakeResp := &httpclient.Response{
		StatusCode: httpResp.StatusCode,
		Headers:    httpResp.Headers,
		Body:       envelope.Response,
	}

	// Use Gemini transformer to convert response
	return t.geminiTransformer.TransformResponse(ctx, fakeResp)
}

// TransformError transforms HTTP error.
func (t *Transformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	// Delegate to Gemini transformer
	return t.geminiTransformer.TransformError(ctx, rawErr)
}

// SetAPIKey updates the API key.
func (t *Transformer) SetAPIKey(apiKey string) {
	t.config.APIKey = apiKey
	t.initTokenProvider(apiKey)

	// Also update internal gemini transformer?
	// geminiTransformer interface doesn't expose SetAPIKey directly unless we assert it.
	if gt, ok := t.geminiTransformer.(*gemini.OutboundTransformer); ok {
		gt.SetAPIKey(apiKey)
	}
}

// SetBaseURL updates the base URL.
func (t *Transformer) SetBaseURL(baseURL string) {
	t.config.BaseURL = baseURL
	if gt, ok := t.geminiTransformer.(*gemini.OutboundTransformer); ok {
		gt.SetBaseURL(baseURL)
	}
}

// AggregateStreamChunks implements transformer.Outbound.
func (t *Transformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	// We need to unwrap the chunks before delegating to Gemini transformer.
	// Since we shouldn't modify the input chunks in place (they might be used elsewhere),
	// we create a new slice of chunks with modified data.
	unwrappedChunks := make([]*httpclient.StreamEvent, len(chunks))
	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}

		// Copy the chunk
		newChunk := *chunk
		unwrappedChunks[i] = &newChunk

		if len(chunk.Data) == 0 {
			continue
		}

		// Check for DONE marker
		if string(chunk.Data) == "[DONE]" {
			continue
		}

		// Unwrap logic
		var wrapper struct {
			Response json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal(chunk.Data, &wrapper); err == nil && len(wrapper.Response) > 0 {
			unwrappedChunks[i].Data = wrapper.Response
		}
	}

	return t.geminiTransformer.AggregateStreamChunks(ctx, unwrappedChunks)
}

// TransformStream implements transformer.Outbound.
func (t *Transformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	// We need to intercept the stream events to unwrap the "response" envelope
	// before Gemini transformer processes them.

	// Create a new stream that maps events
	unwrappedStream := streams.Map(stream, func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
		if event == nil || len(event.Data) == 0 {
			return event
		}

		// Check if it's the DONE event
		if string(event.Data) == "[DONE]" {
			return event
		}

		// Unwrap logic
		var wrapper struct {
			Response json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal(event.Data, &wrapper); err == nil && len(wrapper.Response) > 0 {
			// Create a copy of the event to avoid mutating the original stream (though safe here)
			newEvent := *event
			newEvent.Data = wrapper.Response

			return &newEvent
		}

		return event
	})

	return t.geminiTransformer.TransformStream(ctx, unwrappedStream)
}

// CustomizeExecutor implements pipeline.ChannelCustomizedExecutor.
// It wraps the standard executor with Antigravity-specific endpoint fallback logic.
func (t *Transformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	return NewExecutor(executor)
}
