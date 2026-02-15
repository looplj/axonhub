package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xmap"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

var _ transformer.Outbound = (*OutboundTransformer)(nil)

// Config holds all configuration for the OpenAI Responses outbound transformer.
type Config struct {
	// BaseURL is the base URL for the OpenAI API, required.
	BaseURL string `json:"base_url,omitempty"`

	// RawURL is whether to use raw URL for requests, default is false.
	// If true, the request URL will be used as is, without appending the response endpoint.
	RawURL bool `json:"raw_url,omitempty"`

	// APIKeyProvider provides API keys for authentication, required.
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

func NewOutboundTransformer(baseURL, apiKey string) (*OutboundTransformer, error) {
	if apiKey == "" || baseURL == "" {
		return nil, fmt.Errorf("apiKey or baseURL is empty")
	}

	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

func NewOutboundTransformerWithConfig(config *Config) (*OutboundTransformer, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("API key provider is required")
	}

	if strings.HasSuffix(config.BaseURL, "##") {
		config.RawURL = true
		config.BaseURL = strings.TrimSuffix(config.BaseURL, "##")
	} else {
		config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "v1")
	}

	return &OutboundTransformer{
		config: config,
	}, nil
}

type OutboundTransformer struct {
	config *Config
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIResponse
}

// TransformError transforms HTTP error response to unified error response.
func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	if rawErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: http.StatusText(http.StatusInternalServerError),
				Type:    "api_error",
			},
		}
	}

	// Try to parse as OpenAI error format first
	var openaiError struct {
		Error llm.ErrorDetail `json:"error"`
	}

	err := json.Unmarshal(rawErr.Body, &openaiError)
	if err == nil && openaiError.Error.Message != "" {
		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail:     openaiError.Error,
		}
	}

	// If JSON parsing fails, use the upstream status text
	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: strings.TrimSpace(string(rawErr.Body)),
			Type:    "api_error",
		},
	}
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("chat request is nil")
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeChat, "":
		// continue
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	// Initialize TransformerMetadata if nil
	if llmReq.TransformerMetadata == nil {
		llmReq.TransformerMetadata = map[string]any{}
	}

	var tools []Tool
	// Convert tools to Responses API format
	for _, item := range llmReq.Tools {
		switch item.Type {
		case llm.ToolTypeImageGeneration:
			tool := convertImageGenerationToTool(item)
			tools = append(tools, tool)
			// Store image output format in TransformerMetadata
			llmReq.TransformerMetadata["image_output_format"] = tool.OutputFormat
		case "function":
			tool := convertFunctionToTool(item)
			tools = append(tools, tool)
		default:
			// Skip unsupported tool types
			continue
		}
	}

	payload := Request{
		Model:                llmReq.Model,
		Input:                convertInputFromMessages(llmReq.Messages, llmReq.TransformOptions),
		Instructions:         convertInstructionsFromMessages(llmReq.Messages),
		Tools:                tools,
		ParallelToolCalls:    llmReq.ParallelToolCalls,
		Stream:               llmReq.Stream,
		Text:                 convertToTextOptions(llmReq),
		Store:                llmReq.Store,
		ServiceTier:          llmReq.ServiceTier,
		SafetyIdentifier:     llmReq.SafetyIdentifier,
		User:                 llmReq.User,
		Metadata:             llmReq.Metadata,
		MaxOutputTokens:      llmReq.MaxCompletionTokens,
		TopLogprobs:          llmReq.TopLogprobs,
		TopP:                 llmReq.TopP,
		ToolChoice:           convertToolChoice(llmReq.ToolChoice),
		StreamOptions:        convertStreamOptions(llmReq.StreamOptions, llmReq.TransformerMetadata),
		Reasoning:            convertReasoning(llmReq),
		PromptCacheKey:       llmReq.PromptCacheKey,
		Include:              xmap.GetStringSlice(llmReq.TransformerMetadata, "include"),
		MaxToolCalls:         xmap.GetInt64Ptr(llmReq.TransformerMetadata, "max_tool_calls"),
		PromptCacheRetention: xmap.GetStringPtr(llmReq.TransformerMetadata, "prompt_cache_retention"),
		Truncation:           xmap.GetStringPtr(llmReq.TransformerMetadata, "truncation"),
	}

	// Clear `parallel_tool_calls` when no tools are sent (Responses API compatibility).
	if len(payload.Tools) == 0 {
		payload.ParallelToolCalls = nil
	}

	// Set MaxOutputTokens to MaxTokens if not set
	if payload.MaxOutputTokens == nil {
		payload.MaxOutputTokens = llmReq.MaxTokens
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal responses api request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	fullURL, err := t.buildFullRequestURL(llmReq)
	if err != nil {
		return nil, err
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     fullURL,
		Headers: headers,
		Body:    body,
		Auth: &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: t.config.APIKeyProvider.Get(ctx),
		},
		TransformerMetadata:   llmReq.TransformerMetadata,
		SkipInboundQueryMerge: true,
	}, nil
}

// buildFullRequestURL constructs the appropriate URL based on the platform.
func (t *OutboundTransformer) buildFullRequestURL(_ *llm.Request) (string, error) {
	if t.config.RawURL {
		return t.config.BaseURL, nil
	}
	return t.config.BaseURL + "/responses", nil
}

// TransformResponse converts an OpenAI Responses API HTTP response to unified llm.Response.
// It focuses on image generation results (image_generation_call) and maps them to
// assistant message content with image_url parts.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var resp Response
	if err := json.Unmarshal(httpResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal responses api response: %w", err)
	}

	// Convert to unified llm.Response format
	llmResp := &llm.Response{
		Object:  "chat.completion",
		ID:      resp.ID,
		Model:   resp.Model,
		Created: resp.CreatedAt,
		Choices: make([]llm.Choice, 0),
	}

	// Convert usage if present
	if resp.Usage != nil {
		llmResp.Usage = resp.Usage.ToUsage()
	}

	// Process output items - aggregate all into a single choice (Chat Completions format)
	var (
		contentParts       []llm.MessageContentPart
		textContent        strings.Builder
		reasoningContent   strings.Builder
		reasoningSignature *string
		toolCalls          []llm.ToolCall
	)

	for _, outputItem := range resp.Output {
		switch outputItem.Type {
		case "message":
			// Extract text content from message content array
			for _, contentItem := range outputItem.GetContentItems() {
				if contentItem.Type == "output_text" {
					textContent.WriteString(contentItem.Text)
				}
			}
		case "output_text":
			// Direct text output
			if outputItem.Text != nil {
				textContent.WriteString(*outputItem.Text)
			}
		case "function_call":
			// Function call output - aggregate all tool calls
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:   outputItem.CallID,
				Type: "function",
				Function: llm.FunctionCall{
					Name:      outputItem.Name,
					Arguments: outputItem.Arguments,
				},
			})
		case "reasoning":
			// Handle reasoning output - convert to ReasoningContent
			for _, summary := range outputItem.Summary {
				reasoningContent.WriteString(summary.Text)
			}
			// Preserve encrypted reasoning content in ReasoningSignature.
			if outputItem.EncryptedContent != nil && *outputItem.EncryptedContent != "" {
				reasoningSignature = shared.EncodeOpenAIEncryptedContent(outputItem.EncryptedContent)
			}
		case "image_generation_call":
			imageOutputFormat := "png"

			if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
				if fmt, ok := httpResp.Request.TransformerMetadata["image_output_format"].(string); ok && fmt != "" {
					imageOutputFormat = fmt
				}
			}
			// Image generation result
			if outputItem.Result != nil && *outputItem.Result != "" {
				contentParts = append(contentParts, llm.MessageContentPart{
					Type: "image_url",
					ImageURL: &llm.ImageURL{
						URL: `data:image/` + imageOutputFormat + `;base64,` + *outputItem.Result,
					},
					TransformerMetadata: map[string]any{
						"background":    outputItem.Background,
						"output_format": outputItem.OutputFormat,
						"quality":       outputItem.Quality,
						"size":          outputItem.Size,
					},
				})
			}
		case "input_image":
			// Input image (for reference)
			if outputItem.ImageURL != nil && *outputItem.ImageURL != "" {
				contentParts = append(contentParts, llm.MessageContentPart{
					Type: "image_url",
					ImageURL: &llm.ImageURL{
						URL: *outputItem.ImageURL,
					},
				})
			}
		}
	}

	// Build the single choice
	choice := llm.Choice{
		Index: 0,
		Message: &llm.Message{
			Role:      "assistant",
			ToolCalls: toolCalls,
		},
	}

	// Set reasoning content if present
	if reasoningContent.Len() > 0 {
		choice.Message.ReasoningContent = lo.ToPtr(reasoningContent.String())
	}

	if reasoningSignature != nil {
		choice.Message.ReasoningSignature = reasoningSignature
	}

	// Set message content
	if textContent.Len() > 0 {
		if len(contentParts) > 0 {
			// Mixed content: text + images
			textPart := llm.MessageContentPart{
				Type: "text",
				Text: lo.ToPtr(textContent.String()),
			}
			contentParts = append([]llm.MessageContentPart{textPart}, contentParts...)
			choice.Message.Content = llm.MessageContent{
				MultipleContent: contentParts,
			}
		} else {
			// Text only
			choice.Message.Content = llm.MessageContent{
				Content: lo.ToPtr(textContent.String()),
			}
		}
	} else if len(contentParts) > 0 {
		// Images only
		choice.Message.Content = llm.MessageContent{
			MultipleContent: contentParts,
		}
	}

	// Set finish reason based on status and content
	if len(toolCalls) > 0 {
		choice.FinishReason = lo.ToPtr("tool_calls")
	} else if resp.Status != nil {
		switch *resp.Status {
		case "completed":
			choice.FinishReason = lo.ToPtr("stop")
		case "failed":
			choice.FinishReason = lo.ToPtr("error")
		case "incomplete":
			choice.FinishReason = lo.ToPtr("length")
		}
	}

	llmResp.Choices = append(llmResp.Choices, choice)

	// If no choices were created, create a default empty choice
	if len(llmResp.Choices) == 0 {
		llmResp.Choices = []llm.Choice{
			{
				Index:        0,
				FinishReason: lo.ToPtr("stop"),
				Message: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr(""),
					},
				},
			},
		}
	}

	return llmResp, nil
}
