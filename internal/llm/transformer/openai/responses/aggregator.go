package responses

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// streamAggregator holds the state for aggregating stream chunks.
type streamAggregator struct {
	// Response metadata
	responseID string
	model      string
	createdAt  int64
	status     string

	// Output items - keyed by output_index
	outputItems map[int]*aggregatedItem

	// Usage
	usage *Usage
}

// aggregatedItem holds the accumulated state for an output item.
type aggregatedItem struct {
	ID        string
	Type      string
	Status    string
	Role      string
	CallID    string
	Name      string
	Arguments *strings.Builder

	// For message type
	Content []*aggregatedContentPart

	// For reasoning type
	Summary *strings.Builder
}

// aggregatedContentPart holds the accumulated state for a content part.
type aggregatedContentPart struct {
	Type string
	Text *strings.Builder
}

func newAggregatedItem() *aggregatedItem {
	return &aggregatedItem{
		Arguments: &strings.Builder{},
		Summary:   &strings.Builder{},
	}
}

func newAggregatedContentPart() *aggregatedContentPart {
	return &aggregatedContentPart{
		Text: &strings.Builder{},
	}
}

func newStreamAggregator() *streamAggregator {
	return &streamAggregator{
		outputItems: make(map[int]*aggregatedItem),
		status:      "in_progress",
	}
}

// AggregateStreamChunks aggregates OpenAI Responses API streaming chunks into a complete Response.
// This is a shared implementation used by both InboundTransformer and OutboundTransformer.
//
//nolint:maintidx,gocognit // Aggregation logic is inherently complex.
func AggregateStreamChunks(_ context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	if len(chunks) == 0 {
		return nil, llm.ResponseMeta{}, errors.New("empty stream chunks")
	}

	agg := newStreamAggregator()

	for _, chunk := range chunks {
		if chunk == nil || len(chunk.Data) == 0 {
			continue
		}

		// Skip [DONE] marker
		if string(chunk.Data) == "[DONE]" {
			continue
		}

		var ev StreamEvent
		if err := json.Unmarshal(chunk.Data, &ev); err != nil {
			continue
		}

		agg.processEvent(&ev)
	}

	resp := agg.buildResponse()

	body, err := json.Marshal(resp)
	if err != nil {
		return nil, llm.ResponseMeta{}, err
	}

	meta := llm.ResponseMeta{
		ID: agg.responseID,
	}

	if agg.usage != nil {
		meta.Usage = agg.usage.ToUsage()
	}

	return body, meta, nil
}

//nolint:gocognit // Event processing is inherently complex.
func (a *streamAggregator) processEvent(ev *StreamEvent) {
	//nolint:exhaustive //Only process events we care about.
	switch ev.Type {
	case StreamEventTypeResponseCreated, StreamEventTypeResponseInProgress:
		if ev.Response != nil {
			a.responseID = ev.Response.ID
			a.model = ev.Response.Model
			a.createdAt = ev.Response.CreatedAt

			if ev.Response.Usage != nil {
				a.usage = ev.Response.Usage
			}
		}

	case StreamEventTypeOutputItemAdded:
		// Initialize a new output item
		item := newAggregatedItem()
		item.Status = "in_progress"

		if ev.Item != nil {
			item.ID = ev.Item.ID
			item.Type = ev.Item.Type
			item.Role = ev.Item.Role
			item.CallID = ev.Item.CallID
			item.Name = ev.Item.Name
			item.Arguments.WriteString(ev.Item.Arguments)
		}

		a.outputItems[ev.OutputIndex] = item

	case StreamEventTypeContentPartAdded:
		if item, ok := a.outputItems[ev.OutputIndex]; ok {
			contentPart := newAggregatedContentPart()

			if ev.Part != nil {
				contentPart.Type = ev.Part.Type
				if ev.Part.Text != nil {
					contentPart.Text.WriteString(*ev.Part.Text)
				}
			}

			item.Content = append(item.Content, contentPart)
		}

	case StreamEventTypeOutputTextDelta:
		if item, ok := a.outputItems[ev.OutputIndex]; ok {
			if ev.ContentIndex != nil && *ev.ContentIndex < len(item.Content) {
				item.Content[*ev.ContentIndex].Text.WriteString(ev.Delta)
			}
		}

	case StreamEventTypeFunctionCallArgumentsDelta:
		// Find item by item_id
		if ev.ItemID != nil {
			for _, item := range a.outputItems {
				if item.ID == *ev.ItemID || item.CallID == *ev.ItemID {
					item.Arguments.WriteString(ev.Delta)
					break
				}
			}
		}

	case StreamEventTypeFunctionCallArgumentsDone:
		// Find item and finalize arguments
		if ev.ItemID != nil {
			for _, item := range a.outputItems {
				if item.ID == *ev.ItemID || item.CallID == *ev.ItemID {
					if ev.Name != "" {
						item.Name = ev.Name
					}

					if ev.Arguments != "" {
						// Replace accumulated arguments with final version
						item.Arguments.Reset()
						item.Arguments.WriteString(ev.Arguments)
					}

					break
				}
			}
		}

	case StreamEventTypeReasoningSummaryTextDelta:
		if item, ok := a.outputItems[ev.OutputIndex]; ok {
			item.Summary.WriteString(ev.Delta)
		}

	case StreamEventTypeReasoningSummaryTextDone:
		if item, ok := a.outputItems[ev.OutputIndex]; ok {
			if ev.Text != "" {
				item.Summary.Reset()
				item.Summary.WriteString(ev.Text)
			}
		}

	case StreamEventTypeOutputTextDone:
		if item, ok := a.outputItems[ev.OutputIndex]; ok {
			if ev.ContentIndex != nil && *ev.ContentIndex < len(item.Content) && ev.Text != "" {
				item.Content[*ev.ContentIndex].Text.Reset()
				item.Content[*ev.ContentIndex].Text.WriteString(ev.Text)
			}
		}

	case StreamEventTypeOutputItemDone:
		// Mark item as completed and update with final data
		if ev.Item != nil {
			if item, ok := a.outputItems[ev.OutputIndex]; ok {
				if ev.Item.Status != nil {
					item.Status = *ev.Item.Status
				}

				if item.Status == "" {
					item.Status = "completed"
				}

				// Update with final data if provided
				if ev.Item.Arguments != "" {
					item.Arguments.Reset()
					item.Arguments.WriteString(ev.Item.Arguments)
				}
			}
		}

	case StreamEventTypeResponseCompleted:
		a.status = "completed"
		if ev.Response != nil && ev.Response.Usage != nil {
			a.usage = ev.Response.Usage
		}

	case StreamEventTypeResponseFailed:
		a.status = "failed"

	case StreamEventTypeResponseIncomplete:
		a.status = "incomplete"
	}
}

// buildResponse builds the final Response object from aggregated state.
// This is used by responsesInboundStream to build the response.completed event.
func (a *streamAggregator) buildResponse() *Response {
	// Build output items
	output := make([]Item, 0, len(a.outputItems))

	// Sort by output index
	maxIndex := 0
	for idx := range a.outputItems {
		if idx > maxIndex {
			maxIndex = idx
		}
	}

	for i := 0; i <= maxIndex; i++ {
		if item, ok := a.outputItems[i]; ok {
			switch item.Type {
			case "message":
				// Convert aggregated content parts to []Item for Content.Items
				contentItems := make([]Item, 0, len(item.Content))
				for _, cp := range item.Content {
					text := cp.Text.String()
					contentItems = append(contentItems, Item{
						Type: cp.Type,
						Text: &text,
					})
				}

				output = append(output, Item{
					ID:     item.ID,
					Type:   item.Type,
					Role:   item.Role,
					Status: lo.ToPtr(item.Status),
					Content: &Input{
						Items: contentItems,
					},
				})

			case "function_call":
				output = append(output, Item{
					ID:        item.ID,
					Type:      item.Type,
					Status:    lo.ToPtr(item.Status),
					CallID:    item.CallID,
					Name:      item.Name,
					Arguments: item.Arguments.String(),
				})

			case "reasoning":
				var summary []ReasoningSummary
				if item.Summary.Len() > 0 {
					summary = []ReasoningSummary{{
						Type: "summary_text",
						Text: item.Summary.String(),
					}}
				}

				output = append(output, Item{
					ID:      item.ID,
					Type:    item.Type,
					Status:  lo.ToPtr(item.Status),
					Summary: summary,
				})

			default:
				// Generic item
				output = append(output, Item{
					ID:     item.ID,
					Type:   item.Type,
					Status: lo.ToPtr(item.Status),
					Role:   item.Role,
				})
			}
		}
	}

	return &Response{
		Object:    "response",
		ID:        a.responseID,
		Model:     a.model,
		CreatedAt: a.createdAt,
		Status:    lo.ToPtr(a.status),
		Output:    output,
		Usage:     a.usage,
	}
}
