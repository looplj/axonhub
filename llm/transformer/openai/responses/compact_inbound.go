package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/samber/lo"
)

var _ transformer.Inbound = (*CompactInboundTransformer)(nil)

// CompactInboundTransformer implements transformer.Inbound for the OpenAI Responses Compact API.
type CompactInboundTransformer struct{}

// NewCompactInboundTransformer creates a new CompactInboundTransformer.
func NewCompactInboundTransformer() *CompactInboundTransformer {
	return &CompactInboundTransformer{}
}

func (t *CompactInboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIResponseCompact
}

// TransformRequest transforms HTTP compact request to llm.Request.
func (t *CompactInboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	contentType := httpReq.Headers.Get("Content-Type")
	if contentType != "" && !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("%w: unsupported content type: %s", transformer.ErrInvalidRequest, contentType)
	}

	var req CompactAPIRequest
	if err := json.Unmarshal(httpReq.Body, &req); err != nil {
		return nil, fmt.Errorf("%w: failed to decode compact request: %w", transformer.ErrInvalidRequest, err)
	}

	if req.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	// Reuse convertInputToMessages to convert Input to []llm.Message
	inputMessages, err := convertInputToMessages(&req.Input)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to convert input: %w", transformer.ErrInvalidRequest, err)
	}

	llmReq := &llm.Request{
		Model:       req.Model,
		Messages:    []llm.Message{},
		RequestType: llm.RequestTypeCompact,
		APIFormat:   llm.APIFormatOpenAIResponseCompact,
		Stream:      lo.ToPtr(false),
		Compact: &llm.CompactRequest{
			Input:          inputMessages,
			Instructions:   req.Instructions,
			PromptCacheKey: req.PromptCacheKey,
		},
	}

	return llmReq, nil
}

// TransformResponse transforms llm.Response to HTTP compact response.
func (t *CompactInboundTransformer) TransformResponse(ctx context.Context, llmResp *llm.Response) (*httpclient.Response, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("compact response is nil")
	}

	if llmResp.Compact == nil {
		return nil, fmt.Errorf("compact response missing compact data")
	}

	outputItems := convertCompactMessagesToItems(llmResp.Compact.Output)

	var usage *Usage
	if llmResp.Usage != nil {
		usage = ConvertLLMUsageToResponsesUsage(llmResp.Usage)
	}

	body, err := json.Marshal(CompactAPIResponse{
		ID:           llmResp.Compact.ID,
		CreatedAt:    llmResp.Compact.CreatedAt,
		Object:       llmResp.Compact.Object,
		Model:        llmResp.Model,
		Instructions: llmResp.Compact.Instructions,
		Output:       outputItems,
		Usage:        usage,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compact response: %w", err)
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

func convertCompactMessagesToItems(msgs []llm.Message) []Item {
	items := make([]Item, 0, len(msgs))

	for _, msg := range msgs {
		if reasoningItem, ok := buildReasoningItem(msg); ok {
			items = append(items, reasoningItem)
		}

		items = append(items, convertCompactMessageToItems(msg)...)
	}

	return items
}

func convertCompactMessageToItems(msg llm.Message) []Item {
	role := msg.Role
	if role == "" {
		role = "assistant"
	}

	var items []Item
	var contentItems []Item

	textItemType := "input_text"
	if role == "assistant" {
		textItemType = "output_text"
	}

	flushMessage := func() {
		if len(contentItems) == 0 {
			return
		}
		items = append(items, Item{
			ID:      msg.ID,
			Type:    "message",
			Role:    role,
			Content: &Input{Items: contentItems},
			Status:  lo.ToPtr("completed"),
		})
		contentItems = nil
	}

	if msg.Content.Content != nil {
		contentItems = append(contentItems, Item{
			Type:        textItemType,
			Text:        msg.Content.Content,
			Annotations: []Annotation{},
		})
	}

	for _, part := range msg.Content.MultipleContent {
		switch part.Type {
		case "text":
			if part.Text != nil {
				contentItems = append(contentItems, Item{
					Type:        textItemType,
					Text:        part.Text,
					Annotations: []Annotation{},
				})
			}
		case "image_url":
			if part.ImageURL != nil {
				contentItems = append(contentItems, Item{
					Type:     "input_image",
					ImageURL: lo.ToPtr(part.ImageURL.URL),
					Detail:   part.ImageURL.Detail,
				})
			}
		case "compaction", "compaction_summary":
			if part.Compact != nil {
				flushMessage()
				items = append(items, compactionItemFromPart(part, part.Type))
			}
		}
	}

	flushMessage()

	return items
}

// TransformStream is not supported for compact requests.
func (t *CompactInboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: compact does not support streaming", transformer.ErrInvalidRequest)
}

// AggregateStreamChunks is not supported for compact requests.
func (t *CompactInboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("compact does not support streaming")
}

// TransformError transforms errors to HTTP error responses.
func (t *CompactInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	inbound := NewInboundTransformer()
	return inbound.TransformError(ctx, rawErr)
}
