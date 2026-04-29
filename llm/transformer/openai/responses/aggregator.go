package responses

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// streamAggregator holds the state for aggregating stream chunks.
type streamAggregator struct {
	// Response metadata
	responseID         string
	model              string
	createdAt          int64
	status             string
	previousResponseID *string

	// Output items - keyed by output_index.
	// Some streams may (unexpectedly) reuse output_index for multiple items, so we store a slice.
	outputItems map[int][]*aggregatedItem
	// Fast lookup by item.id (and a fallback for cases where an upstream sends call_id as item_id).
	outputItemsByID map[string]*aggregatedItem

	// Usage
	usage *Usage
}

// aggregatedItem holds the accumulated state for an output item.
type aggregatedItem struct {
	ID               string
	Type             string
	Status           string
	Role             string
	CallID           string
	Name             string
	Arguments        *strings.Builder
	EncryptedContent *string

	// For custom_tool_call type
	Input *string

	// For message type
	Content []*aggregatedContentPart

	// For reasoning type
	SummaryParts map[int]*aggregatedSummaryPart
}

type aggregatedSummaryPart struct {
	Type  string
	Text  *strings.Builder
	Final bool
}

// aggregatedContentPart holds the accumulated state for a content part.
type aggregatedContentPart struct {
	Type string
	Text *strings.Builder
}

func newAggregatedItem() *aggregatedItem {
	return &aggregatedItem{
		Arguments:    &strings.Builder{},
		SummaryParts: make(map[int]*aggregatedSummaryPart),
	}
}

func newAggregatedContentPart() *aggregatedContentPart {
	return &aggregatedContentPart{
		Text: &strings.Builder{},
	}
}

func ensureSummaryPart(item *aggregatedItem, summaryIndex int) *aggregatedSummaryPart {
	if item == nil {
		return nil
	}

	if item.SummaryParts == nil {
		item.SummaryParts = make(map[int]*aggregatedSummaryPart)
	}

	if part, ok := item.SummaryParts[summaryIndex]; ok && part != nil {
		if part.Text == nil {
			part.Text = &strings.Builder{}
		}

		if part.Type == "" {
			part.Type = "summary_text"
		}

		return part
	}

	part := &aggregatedSummaryPart{
		Type: "summary_text",
		Text: &strings.Builder{},
	}
	item.SummaryParts[summaryIndex] = part

	return part
}

func newStreamAggregator() *streamAggregator {
	return &streamAggregator{
		outputItems:     make(map[int][]*aggregatedItem),
		outputItemsByID: make(map[string]*aggregatedItem),
		status:          "in_progress",
	}
}

func (a *streamAggregator) lastItemByOutputIndex(outputIndex int) *aggregatedItem {
	items := a.outputItems[outputIndex]
	if len(items) == 0 {
		return nil
	}

	return items[len(items)-1]
}

func (a *streamAggregator) getItemForEvent(outputIndex int, itemID *string) *aggregatedItem {
	if itemID != nil && *itemID != "" {
		if item, ok := a.outputItemsByID[*itemID]; ok {
			return item
		}

		// Some upstream implementations might use call_id as item_id in delta events.
		for _, items := range a.outputItems {
			for _, it := range items {
				if it.CallID == *itemID {
					return it
				}
			}
		}
	}

	return a.lastItemByOutputIndex(outputIndex)
}

func applyDoneText(dst *strings.Builder, doneText string) {
	if doneText == "" {
		return
	}

	existing := dst.String()
	if existing == "" || len(doneText) >= len(existing) {
		dst.Reset()
		dst.WriteString(doneText)
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

//nolint:gocognit,maintidx // Event processing is inherently complex.
func (a *streamAggregator) processEvent(ev *StreamEvent) {
	//nolint:exhaustive //Only process events we care about.
	switch ev.Type {
	case StreamEventTypeResponseCreated, StreamEventTypeResponseInProgress:
		if ev.Response != nil {
			a.responseID = ev.Response.ID
			a.model = ev.Response.Model
			a.createdAt = ev.Response.CreatedAt
			a.previousResponseID = ev.Response.PreviousResponseID

			if ev.Response.Usage != nil {
				a.usage = ev.Response.Usage
			}
		}

	case StreamEventTypeOutputItemAdded:
		// Initialize a new output item, or merge into an existing placeholder if present.
		var item *aggregatedItem
		if ev.Item != nil && ev.Item.ID != "" {
			item = a.outputItemsByID[ev.Item.ID]
		}

		if item == nil {
			item = newAggregatedItem()
			item.Status = "in_progress"
			a.outputItems[ev.OutputIndex] = append(a.outputItems[ev.OutputIndex], item)
		}

		if ev.Item != nil {
			item.ID = ev.Item.ID
			item.Type = ev.Item.Type
			item.Role = ev.Item.Role
			item.CallID = ev.Item.CallID
			item.Name = ev.Item.Name
			item.Arguments.WriteString(ev.Item.Arguments)
			item.EncryptedContent = ev.Item.EncryptedContent
			item.Input = ev.Item.Input

			if len(ev.Item.Summary) > 0 {
				for idx, s := range ev.Item.Summary {
					part := ensureSummaryPart(item, idx)
					part.Type = s.Type
					applyDoneText(part.Text, s.Text)
					part.Final = true
				}
			}
		}

		if item.ID != "" {
			a.outputItemsByID[item.ID] = item
		}

	case StreamEventTypeContentPartAdded:
		item := a.getItemForEvent(ev.OutputIndex, ev.ItemID)
		if item != nil {
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
		item := a.getItemForEvent(ev.OutputIndex, ev.ItemID)
		if item != nil {
			if ev.ContentIndex != nil && *ev.ContentIndex < len(item.Content) {
				item.Content[*ev.ContentIndex].Text.WriteString(ev.Delta)
			}
		}

	case StreamEventTypeOutputTextDone:
		item := a.getItemForEvent(ev.OutputIndex, ev.ItemID)
		if item != nil {
			if ev.ContentIndex != nil && *ev.ContentIndex < len(item.Content) && ev.Text != "" {
				applyDoneText(item.Content[*ev.ContentIndex].Text, ev.Text)
			}
		}

	case StreamEventTypeFunctionCallArgumentsDelta:
		// Find item by item_id
		if ev.ItemID != nil {
			if item := a.getItemForEvent(ev.OutputIndex, ev.ItemID); item != nil {
				item.Arguments.WriteString(ev.Delta)
			}
		}

	case StreamEventTypeFunctionCallArgumentsDone:
		// Find item and finalize arguments
		if ev.ItemID != nil {
			if item := a.getItemForEvent(ev.OutputIndex, ev.ItemID); item != nil {
				if ev.Name != "" {
					item.Name = ev.Name
				}

				if ev.Arguments != "" {
					// Replace accumulated arguments with final version
					item.Arguments.Reset()
					item.Arguments.WriteString(ev.Arguments)
				}
			}
		}

	case StreamEventTypeCustomToolCallInputDelta:
		// Accumulate custom tool call input delta
		if ev.ItemID != nil {
			if item := a.getItemForEvent(ev.OutputIndex, ev.ItemID); item != nil {
				current := lo.FromPtr(item.Input)
				item.Input = lo.ToPtr(current + ev.Delta)
			}
		}

	case StreamEventTypeCustomToolCallInputDone:
		// Finalize custom tool call input
		if ev.ItemID != nil {
			if item := a.getItemForEvent(ev.OutputIndex, ev.ItemID); item != nil {
				if ev.Input != "" {
					item.Input = lo.ToPtr(ev.Input)
				}
			}
		}

	case StreamEventTypeReasoningSummaryPartAdded:
		item := a.getItemForEvent(ev.OutputIndex, ev.ItemID)
		if item == nil {
			item = newAggregatedItem()
			item.Type = "reasoning"

			item.Status = "in_progress"
			if ev.ItemID != nil && *ev.ItemID != "" {
				item.ID = *ev.ItemID
				a.outputItemsByID[item.ID] = item
			}

			a.outputItems[ev.OutputIndex] = append(a.outputItems[ev.OutputIndex], item)
		}

		summaryIndex := lo.FromPtr(ev.SummaryIndex)
		part := ensureSummaryPart(item, summaryIndex)

		if ev.Part != nil {
			if ev.Part.Type != "" {
				part.Type = ev.Part.Type
			}

			if ev.Part.Text != nil {
				part.Text.WriteString(*ev.Part.Text)
			}
		}

	case StreamEventTypeReasoningSummaryPartDone:
		item := a.getItemForEvent(ev.OutputIndex, ev.ItemID)
		if item == nil {
			item = newAggregatedItem()
			item.Type = "reasoning"

			item.Status = "in_progress"
			if ev.ItemID != nil && *ev.ItemID != "" {
				item.ID = *ev.ItemID
				a.outputItemsByID[item.ID] = item
			}

			a.outputItems[ev.OutputIndex] = append(a.outputItems[ev.OutputIndex], item)
		}

		summaryIndex := lo.FromPtr(ev.SummaryIndex)
		part := ensureSummaryPart(item, summaryIndex)

		if ev.Part != nil {
			if ev.Part.Type != "" {
				part.Type = ev.Part.Type
			}

			if ev.Part.Text != nil {
				applyDoneText(part.Text, *ev.Part.Text)
			}
		}

		part.Final = true

	case StreamEventTypeReasoningSummaryTextDelta:
		item := a.getItemForEvent(ev.OutputIndex, ev.ItemID)
		if item == nil {
			item = newAggregatedItem()
			item.Type = "reasoning"

			item.Status = "in_progress"
			if ev.ItemID != nil && *ev.ItemID != "" {
				item.ID = *ev.ItemID
				a.outputItemsByID[item.ID] = item
			}

			a.outputItems[ev.OutputIndex] = append(a.outputItems[ev.OutputIndex], item)
		}

		summaryIndex := lo.FromPtr(ev.SummaryIndex)
		part := ensureSummaryPart(item, summaryIndex)
		part.Text.WriteString(ev.Delta)

	case StreamEventTypeReasoningSummaryTextDone:
		item := a.getItemForEvent(ev.OutputIndex, ev.ItemID)
		if item == nil {
			item = newAggregatedItem()
			item.Type = "reasoning"

			item.Status = "in_progress"
			if ev.ItemID != nil && *ev.ItemID != "" {
				item.ID = *ev.ItemID
				a.outputItemsByID[item.ID] = item
			}

			a.outputItems[ev.OutputIndex] = append(a.outputItems[ev.OutputIndex], item)
		}

		summaryIndex := lo.FromPtr(ev.SummaryIndex)
		part := ensureSummaryPart(item, summaryIndex)
		applyDoneText(part.Text, ev.Text)
		part.Final = true

	case StreamEventTypeOutputItemDone:
		// Mark item as completed and update with final data
		if ev.Item != nil {
			item := a.outputItemsByID[ev.Item.ID]
			if item == nil {
				item = a.lastItemByOutputIndex(ev.OutputIndex)
			}

			if item != nil {
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

				if len(ev.Item.Summary) > 0 {
					for idx, s := range ev.Item.Summary {
						part := ensureSummaryPart(item, idx)
						part.Type = s.Type
						applyDoneText(part.Text, s.Text)
						part.Final = true
					}
				}

				if ev.Item.EncryptedContent != nil {
					item.EncryptedContent = ev.Item.EncryptedContent
				}
			}
		}

	case StreamEventTypeResponseCompleted:
		a.status = "completed"
		if ev.Response != nil {
			a.previousResponseID = ev.Response.PreviousResponseID
			if ev.Response.Usage != nil {
				a.usage = ev.Response.Usage
			}
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
		items, ok := a.outputItems[i]
		if !ok || len(items) == 0 {
			continue
		}

		for _, item := range items {
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

			case "custom_tool_call":
				output = append(output, Item{
					ID:     item.ID,
					Type:   item.Type,
					Status: lo.ToPtr(item.Status),
					CallID: item.CallID,
					Name:   item.Name,
					Input:  item.Input,
				})

			case "reasoning":
				var summary []ReasoningSummary

				if len(item.SummaryParts) > 0 {
					maxSummaryIndex := -1
					for idx := range item.SummaryParts {
						if idx > maxSummaryIndex {
							maxSummaryIndex = idx
						}
					}

					summary = make([]ReasoningSummary, 0, maxSummaryIndex+1)
					for idx := 0; idx <= maxSummaryIndex; idx++ {
						sp, ok := item.SummaryParts[idx]
						if !ok || sp == nil {
							summary = append(summary, ReasoningSummary{Type: "summary_text", Text: ""})
							continue
						}

						summaryType := sp.Type
						if summaryType == "" {
							summaryType = "summary_text"
						}

						var text string
						if sp.Text != nil {
							text = sp.Text.String()
						}

						summary = append(summary, ReasoningSummary{Type: summaryType, Text: text})
					}
				}

				output = append(output, Item{
					ID:               item.ID,
					Type:             item.Type,
					Status:           lo.ToPtr(item.Status),
					Summary:          summary,
					EncryptedContent: item.EncryptedContent,
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
		Object:             "response",
		ID:                 a.responseID,
		Model:              a.model,
		CreatedAt:          a.createdAt,
		Status:             lo.ToPtr(a.status),
		Output:             output,
		Usage:              a.usage,
		PreviousResponseID: a.previousResponseID,
	}
}
