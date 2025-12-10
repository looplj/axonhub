package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

var _ transformer.Outbound = (*OutboundTransformer)(nil)

func NewOutboundTransformer(baseURL, apiKey string) (*OutboundTransformer, error) {
	if apiKey == "" || baseURL == "" {
		return nil, fmt.Errorf("apiKey or baseURL is empty")
	}

	baseURL = strings.TrimSuffix(baseURL, "/")

	return &OutboundTransformer{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}, nil
}

type OutboundTransformer struct {
	APIKey  string
	BaseURL string
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIResponse
}

func (t *OutboundTransformer) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return nil
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, chatReq *llm.Request) (*httpclient.Request, error) {
	if chatReq == nil {
		return nil, fmt.Errorf("chat request is nil")
	}

	var tools []Tool

	metadata := map[string]string{}

	// Convert tools to Responses API format
	for _, item := range chatReq.Tools {
		switch item.Type {
		case llm.ToolTypeImageGeneration:
			tool := convertImageGenerationToTool(item)
			tools = append(tools, tool)
			metadata["image_output_format"] = tool.OutputFormat
		case "function":
			tool := convertFunctionToTool(item)
			tools = append(tools, tool)
		default:
			// Skip unsupported tool types
			continue
		}
	}

	payload := Request{
		Model:             chatReq.Model,
		Input:             convertInputFromMessages(chatReq.Messages),
		Instructions:      convertInstructionsFromMessages(chatReq.Messages),
		Tools:             tools,
		ParallelToolCalls: chatReq.ParallelToolCalls,
		Stream:            chatReq.Stream,
		Text:              convertToTextOptions(chatReq),
		Store:             chatReq.Store,
		ServiceTier:       chatReq.ServiceTier,
		SafetyIdentifier:  chatReq.SafetyIdentifier,
		User:              chatReq.User,
		Metadata:          chatReq.Metadata,
		MaxOutputTokens:   chatReq.MaxCompletionTokens,
		TopLogprobs:       chatReq.TopLogprobs,
		TopP:              chatReq.TopP,
		ToolChoice:        convertToolChoice(chatReq.ToolChoice),
		StreamOptions:     convertStreamOptions(chatReq.StreamOptions),
		Reasoning:         convertReasoning(chatReq),
		Include:           chatReq.Include,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal responses api request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     t.BaseURL + "/responses",
		Headers: headers,
		Body:    body,
		Auth: &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: t.APIKey,
		},
		Metadata: metadata,
	}, nil
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
		contentParts     []llm.MessageContentPart
		textContent      strings.Builder
		reasoningContent strings.Builder
		toolCalls        []llm.ToolCall
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
		case "image_generation_call":
			imageOutputFormat := "png"
			if httpResp.Request != nil && httpResp.Request.Metadata != nil && httpResp.Request.Metadata["image_output_format"] != "" {
				imageOutputFormat = httpResp.Request.Metadata["image_output_format"]
			}
			// Image generation result
			if outputItem.Result != nil && *outputItem.Result != "" {
				contentParts = append(contentParts, llm.MessageContentPart{
					Type: "image_url",
					ImageURL: &llm.ImageURL{
						URL: `data:image/` + imageOutputFormat + `;base64,` + *outputItem.Result,
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

// TransformStream and AggregateStreamChunks are implemented in outbound_stream.go

// TransformStreamChunk maps a Responses API streaming event to a partial llm.Response.
// We support emitting a full response structure whenever we can extract an image URL.
func (t *OutboundTransformer) TransformStreamChunk(
	ctx context.Context,
	event *httpclient.StreamEvent,
) (*llm.Response, error) {
	if event == nil || len(event.Data) == 0 {
		return nil, fmt.Errorf("empty stream event")
	}

	// Some streams carry discrete event types; try to extract image urls
	eType := gjson.GetBytes(event.Data, "type").String()
	switch eType {
	case "response.image_generation_call.partial_image",
		"response.image_generation_call.generating",
		"response.image_generation_call.completed":
		// Try to find a data URL under common fields
		url := gjson.GetBytes(event.Data, "image_url.url").String()
		if url == "" {
			url = gjson.GetBytes(event.Data, "result").String()
		}

		if url != "" {
			msg := &llm.Message{Role: "assistant"}
			msg.Content = llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: url}},
			}}
			// Build minimal response
			return &llm.Response{Object: "chat.completion", Choices: []llm.Choice{{Index: 0, Delta: msg}}}, nil
		}
	}

	// If not an image-related event, attempt to interpret as a full Responses payload
	// to enable non-streaming path reuse.
	var dummy httpclient.Response

	dummy.Body = event.Data

	return t.TransformResponse(ctx, &dummy)
}
