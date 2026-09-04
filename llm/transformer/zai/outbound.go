package zai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// Config holds all configuration for the Zai outbound transformer.
type Config struct {
	// API configuration
	BaseURL        string              `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKeyProvider auth.APIKeyProvider `json:"-"`                  // API key provider
	Version        string              `json:"version,omitempty"`  // API version (default: "v4")

	// EndpointPath replaces the default "/chat/completions" path. When set, the
	// base URL is kept as-is without version normalization (same convention as
	// the OpenAI transformer).
	EndpointPath string `json:"endpoint_path,omitempty"`
}

// OutboundTransformer implements transformer.Outbound for Zai format.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL        string
	APIKeyProvider auth.APIKeyProvider
	endpointPath   string
	rawURL         bool
}

// GLM/z.ai accept user_id values of 6-128 characters (error 1214); the field is
// omitted when the client-supplied value is shorter.
const (
	minUserIDLength = 6
	maxUserIDLength = 128
)

// NewOutboundTransformer creates a new Zai OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Zai OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
		ReasoningField: openai.ReasoningFieldContent,
	}

	t, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid Zai transformer configuration: %w", err)
	}

	version := config.Version
	if version == "" {
		version = "v4"
	}

	// "##" suffix marks a raw full request URL (same convention as the OpenAI
	// transformer); strip it here so it never leaks into the request URL.
	rawURL := strings.HasSuffix(config.BaseURL, "##")
	if rawURL {
		config.BaseURL = strings.TrimSuffix(config.BaseURL, "##")
	}

	var baseURL string
	switch {
	case rawURL:
		baseURL = strings.TrimRight(config.BaseURL, "/")
	case config.EndpointPath != "":
		baseURL = transformer.NormalizeBaseURL(config.BaseURL, "")
	default:
		baseURL = transformer.NormalizeBaseURL(config.BaseURL, version)
	}

	return &OutboundTransformer{
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
		endpointPath:   config.EndpointPath,
		rawURL:         rawURL,
		Outbound:       t,
	}, nil
}

type Request struct {
	openai.Request

	UserID    string    `json:"user_id,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Thinking  *Thinking `json:"thinking,omitempty"`
}

type Thinking struct {
	// Enable or disable thinking.
	// enabled | disabled.
	Type string `json:"type"`
}

// glmVersionPattern matches GLM model names such as glm-5.3, glm-5.3-flash or
// zai-org/glm-5.3:thinking and captures the major/minor version.
var glmVersionPattern = regexp.MustCompile(`(?i)(?:^|[/:])glm-([0-9]+)\.([0-9]+)`)

// glmVersion extracts the GLM major/minor version from a model name.
func glmVersion(model string) (major, minor int, ok bool) {
	m := glmVersionPattern.FindStringSubmatch(model)
	if len(m) != 3 {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(m[1])
	minor, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// supportsNativeReasoningEffort reports whether the model supports Zhipu's native
// reasoning_effort parameter (GLM-5.2 and later).
func supportsNativeReasoningEffort(model string) bool {
	major, minor, ok := glmVersion(model)
	return ok && (major > 5 || (major == 5 && minor >= 2))
}

// isAlwaysThinkingModel reports whether the model always enables thinking and rejects
// thinking.type=disabled (GLM-5.3 and GLM-5.3-FLASH).
func isAlwaysThinkingModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.HasPrefix(lower, "glm-5.3") || strings.Contains(lower, "/glm-5.3")
}

// normalizeGLM53Effort maps any effort value into GLM-5.3's valid set (low/high/max).
// GLM-5.3 always thinks and rejects every other value, so none/minimal and unknown
// values fall back to the lightest valid effort.
func normalizeGLM53Effort(effort string) string {
	switch strings.ToLower(effort) {
	case "low":
		return "low"
	case "medium", "high":
		return "high"
	case "xhigh", "max":
		return "max"
	default: // none, minimal, unknown
		return "low"
	}
}

// resolveZaiThinking converts the unified reasoning effort into Zhipu's thinking and
// native reasoning_effort fields.
//
//   - GLM-5.3 / GLM-5.3-FLASH always think: thinking.type=disabled is rejected and only
//     reasoning_effort=low|high|max are accepted (none/minimal are mapped to low).
//   - GLM-5.2 supports both thinking on/off and the full reasoning_effort ladder.
//   - Older GLM models only understand thinking.type enabled/disabled.
func resolveZaiThinking(model, reasoningEffort string) (*Thinking, *string) {
	if reasoningEffort == "" {
		return nil, nil
	}

	effort := reasoningEffort

	if !supportsNativeReasoningEffort(model) {
		// GLM models before 5.2 (4.x, 5.0/5.1, and unknown/non-GLM fallback): thinking on/off only.
		if effort == "none" {
			return &Thinking{Type: "disabled"}, nil
		}
		return &Thinking{Type: "enabled"}, nil
	}

	if isAlwaysThinkingModel(model) {
		// GLM-5.3: always think, only low/high/max accepted.
		return &Thinking{Type: "enabled"}, lo.ToPtr(normalizeGLM53Effort(effort))
	}

	// GLM-5.2: thinking can still be disabled; the native reasoning_effort
	// ladder is exposed for supported values. "minimal" still requests reasoning,
	// so keep thinking enabled and map it to the lightest native effort.
	if effort == "none" {
		return &Thinking{Type: "disabled"}, nil
	}
	if effort == "minimal" {
		return &Thinking{Type: "enabled"}, lo.ToPtr(llm.ReasoningEffortLow)
	}
	return &Thinking{Type: "enabled"}, lo.ToPtr(effort)
}

// TransformRequest transforms ChatCompletionRequest to Request.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	// Validate required fields
	if llmReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeChat, "":
		// continue
	case llm.RequestTypeImage:
		return t.buildImageGenerationAPIRequest(ctx, llmReq)
	case llm.RequestTypeCompact:
		return nil, fmt.Errorf("%w: compact is only supported by OpenAI Responses API", transformer.ErrInvalidRequest)
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Convert llm.Request to openai.Request first
	oaiReq := openai.RequestFromLLM(ctx, llmReq, openai.ReasoningFieldContent)

	// Zai doesn't support json_schema, convert to json_object
	if oaiReq.ResponseFormat != nil && oaiReq.ResponseFormat.Type == "json_schema" {
		oaiReq.ResponseFormat.Type = "json_object"
		oaiReq.ResponseFormat.JSONSchema = nil
	}

	// Create Zai-specific request by adding request_id/user_id
	zaiReq := Request{
		Request:   *oaiReq,
		UserID:    "",
		RequestID: "",
	}

	if llmReq.Metadata != nil {
		zaiReq.UserID = llmReq.Metadata["user_id"]
		zaiReq.RequestID = llmReq.Metadata["request_id"]
	}

	// GLM/z.ai validate user_id as 6-128 characters (error 1214). Clients like
	// Claude Code send a long JSON blob (device_id/session_id) here; send only
	// values the upstream accepts 鈥?truncate long ones, drop short ones. An
	// empty value omits the field entirely (json omitempty).
	if runes := []rune(zaiReq.UserID); len(runes) > maxUserIDLength {
		zaiReq.UserID = string(runes[:maxUserIDLength])
	} else if len(runes) < minUserIDLength {
		zaiReq.UserID = ""
	}

	if zaiReq.RequestID == "" {
		sessionID, _ := shared.GetSessionID(ctx)
		zaiReq.RequestID = sessionID
	}

	// zai only support auto tool choice.
	if zaiReq.ToolChoice != nil {
		zaiReq.ToolChoice = &openai.ToolChoice{
			ToolChoice: lo.ToPtr("auto"),
		}
	}

	// zai request does not support metadata (extracted to user_id/request_id)
	zaiReq.Metadata = nil

	// Convert ReasoningEffort to Zhipu thinking / reasoning_effort fields.
	// GLM-5.3 always thinks (none is mapped to low); GLM-5.2+ expose the native
	// reasoning_effort parameter; older GLM models only understand thinking on/off.
	if llmReq.ReasoningEffort != "" {
		thinking, nativeEffort := resolveZaiThinking(llmReq.Model, llmReq.ReasoningEffort)
		zaiReq.Thinking = thinking
		if nativeEffort != nil {
			zaiReq.ReasoningEffort = *nativeEffort
		} else {
			// Avoid leaking an unsupported reasoning_effort to the upstream.
			zaiReq.ReasoningEffort = ""
		}
	}

	body, err := json.Marshal(zaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Get API key from provider
	apiKey := t.APIKeyProvider.Get(ctx)

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: apiKey,
	}

	url := t.BaseURL
	switch {
	case t.rawURL:
		// Base URL is already the full request URL.
	case t.endpointPath != "":
		url += t.endpointPath
	default:
		url += "/chat/completions"
	}

	return &httpclient.Request{
		Method:    http.MethodPost,
		URL:       url,
		Headers:   headers,
		Body:      body,
		Auth:      auth,
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
	}, nil
}

// TransformResponse transforms the HTTP response to llm.Response.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	// Check for HTTP error status codes
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	// If this looks like Image Generation API, use image generation response transformer
	if httpResp.Request != nil && httpResp.Request.APIFormat == string(llm.APIFormatOpenAIImageGeneration) {
		return transformImageGenerationResponse(ctx, httpResp)
	}

	// For regular chat completions, delegate to the wrapped OpenAI transformer
	return t.Outbound.TransformResponse(ctx, httpResp)
}
