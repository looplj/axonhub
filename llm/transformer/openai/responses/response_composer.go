package responses

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xmap"
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
)

type responseComposer struct {
	resp *llm.Response
}

func newResponseComposer(resp *llm.Response) *responseComposer {
	return &responseComposer{resp: resp}
}

func (c *responseComposer) Compose() (*Response, []byte, error) {
	if c == nil || c.resp == nil {
		return nil, nil, fmt.Errorf("chat completion response is nil")
	}

	payload := c.structuredResponse()
	payload.Extra = cloneRawMap(c.responseExtensions().TopLevelExtra)
	payload.MetadataRaw = cloneRaw(c.responseExtensions().MetadataRaw)
	payload.Output = c.restoreOutputItems(payload.Output)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	return payload, body, nil
}

func (c *responseComposer) structuredResponse() *Response {
	resp := &Response{
		Object:             "response",
		ID:                 c.resp.ID,
		Model:              c.resp.Model,
		CreatedAt:          c.resp.Created,
		Output:             make([]Item, 0),
		Status:             lo.ToPtr("completed"),
		PreviousResponseID: c.resp.PreviousResponseID,
		Usage:              ConvertLLMUsageToResponsesUsage(c.resp.Usage),
	}

	for _, choice := range c.resp.Choices {
		var message *llm.Message
		if choice.Message != nil {
			message = choice.Message
		} else if choice.Delta != nil {
			message = choice.Delta
		}

		if message == nil {
			continue
		}

		resp.Output = append(resp.Output, c.outputItemsFromMessage(message)...)
		c.applyFinishReason(resp, choice.FinishReason)
	}

	return resp
}

func (c *responseComposer) outputItemsFromMessage(message *llm.Message) []Item {
	if message == nil {
		return nil
	}

	var output []Item
	messageItemID := message.ID
	if messageItemID == "" {
		messageItemID = generateItemID()
	}

	if reasoningItem, ok := buildReasoningItem(*message); ok {
		output = append(output, reasoningItem)
	}

	for _, toolCall := range message.ToolCalls {
		if toolCall.ResponseCustomToolCall != nil {
			output = append(output, Item{
				ID:     toolCall.ID,
				Type:   "custom_tool_call",
				CallID: toolCall.ResponseCustomToolCall.CallID,
				Name:   toolCall.ResponseCustomToolCall.Name,
				Input:  lo.ToPtr(toolCall.ResponseCustomToolCall.Input),
				Status: lo.ToPtr("completed"),
			})
			continue
		}

		output = append(output, Item{
			ID:        toolCall.ID,
			Type:      "function_call",
			CallID:    toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: toolCall.Function.Arguments,
			Status:    lo.ToPtr("completed"),
		})
	}

	if message.Content.Content != nil && *message.Content.Content != "" {
		text := *message.Content.Content
		output = append(output, Item{
			ID:   messageItemID,
			Type: "message",
			Role: "assistant",
			Content: &Input{
				Items: []Item{
					{
						Type:        "output_text",
						Text:        &text,
						Annotations: []Annotation{},
					},
				},
			},
			Status: lo.ToPtr("completed"),
		})

		return output
	}

	if len(message.Content.MultipleContent) == 0 {
		return output
	}

	contentItems := make([]Item, 0)
	for _, part := range message.Content.MultipleContent {
		switch part.Type {
		case "text":
			if part.Text != nil {
				text := *part.Text
				contentItems = append(contentItems, Item{
					Type:        "output_text",
					Text:        &text,
					Annotations: []Annotation{},
				})
			}
		case "image_url":
			if part.ImageURL != nil {
				output = append(output, Item{
					ID:           generateItemID(),
					Type:         "image_generation_call",
					Role:         "assistant",
					Result:       lo.ToPtr(xurl.ExtractBase64FromDataURL(part.ImageURL.URL)),
					Status:       lo.ToPtr("completed"),
					Background:   xmap.GetStringPtr(part.TransformerMetadata, "background"),
					OutputFormat: xmap.GetStringPtr(part.TransformerMetadata, "output_format"),
					Quality:      xmap.GetStringPtr(part.TransformerMetadata, "quality"),
					Size:         xmap.GetStringPtr(part.TransformerMetadata, "size"),
				})
			}
		case "compaction", "compaction_summary":
			if part.Compact != nil {
				output = append(output, compactionItemFromPart(part, part.Type))
			}
		}
	}

	if len(contentItems) > 0 {
		output = append(output, Item{
			ID:      messageItemID,
			Type:    "message",
			Role:    "assistant",
			Content: &Input{Items: contentItems},
			Status:  lo.ToPtr("completed"),
		})
	}

	return output
}

func (c *responseComposer) applyFinishReason(resp *Response, finishReason *string) {
	if resp == nil || finishReason == nil {
		return
	}

	switch *finishReason {
	case "stop", "tool_calls":
		resp.Status = lo.ToPtr("completed")
	case "length":
		resp.Status = lo.ToPtr("incomplete")
	case "error":
		resp.Status = lo.ToPtr("failed")
	}
}

func (c *responseComposer) restoreOutputItems(structured []Item) []Item {
	responseExt := c.responseExtensions()
	if len(responseExt.OutputItems) == 0 {
		return c.ensureNonEmptyOutput(structured, false)
	}

	knownByKey, contentExtraByKey, rawTopLevel := splitResponseRawOutputItems(responseExt.OutputItems)
	structured = applyKnownOutputExtras(structured, knownByKey, contentExtraByKey)
	restored := interleaveRawOutputItems(structured, rawTopLevel)

	return c.ensureNonEmptyOutput(restored, len(rawTopLevel) > 0)
}

func (c *responseComposer) ensureNonEmptyOutput(output []Item, hasRawOutput bool) []Item {
	if len(output) > 0 || hasRawOutput {
		return output
	}

	emptyText := ""

	return []Item{
		{
			ID:   generateItemID(),
			Type: "message",
			Role: "assistant",
			Content: &Input{
				Items: []Item{
					{
						Type:        "output_text",
						Text:        &emptyText,
						Annotations: []Annotation{},
					},
				},
			},
			Status: lo.ToPtr("completed"),
		},
	}
}

func (c *responseComposer) responseExtensions() *llm.OpenAIResponsesResponseExtensions {
	if c == nil || c.resp == nil || c.resp.ProviderExtensions == nil ||
		c.resp.ProviderExtensions.OpenAIResponses == nil ||
		c.resp.ProviderExtensions.OpenAIResponses.Response == nil {
		return &llm.OpenAIResponsesResponseExtensions{}
	}

	return c.resp.ProviderExtensions.OpenAIResponses.Response
}

func splitResponseRawOutputItems(
	items []llm.OpenAIResponsesRawItem,
) (map[string]llm.OpenAIResponsesRawItem, map[string]llm.OpenAIResponsesRawItem, []llm.OpenAIResponsesRawItem) {
	knownByKey := map[string]llm.OpenAIResponsesRawItem{}
	contentExtraByKey := map[string]llm.OpenAIResponsesRawItem{}
	var rawTopLevel []llm.OpenAIResponsesRawItem

	for _, item := range items {
		if item.ContentIndex != nil {
			if item.SemanticKey != "" {
				contentExtraByKey[item.SemanticKey] = item
			}
			continue
		}

		rawTopLevel = append(rawTopLevel, item)
		if item.SemanticKey != "" {
			knownByKey[item.SemanticKey] = item
		}
	}

	sort.SliceStable(rawTopLevel, func(i, j int) bool {
		return rawItemOriginalIndex(rawTopLevel[i]) < rawItemOriginalIndex(rawTopLevel[j])
	})

	return knownByKey, contentExtraByKey, rawTopLevel
}

func applyKnownOutputExtras(
	structured []Item,
	knownByKey map[string]llm.OpenAIResponsesRawItem,
	contentExtraByKey map[string]llm.OpenAIResponsesRawItem,
) []Item {
	ordinalByType := map[string]int{}
	for i := range structured {
		key := outputItemSemanticKey(structured[i], ordinalByType)
		if rawItem, ok := knownByKey[key]; ok {
			structured[i].Extra = mergeRawMaps(rawItem.Extra, structured[i].Extra)
		}

		if structured[i].Content == nil || len(structured[i].Content.Items) == 0 {
			continue
		}

		contentOrdinalByType := map[string]int{}
		for j := range structured[i].Content.Items {
			contentKey := outputContentItemSemanticKey(key, structured[i].Content.Items[j], contentOrdinalByType)
			if rawItem, ok := contentExtraByKey[contentKey]; ok {
				structured[i].Content.Items[j].Extra = mergeRawMaps(rawItem.Extra, structured[i].Content.Items[j].Extra)
			}
		}
	}

	return structured
}

func interleaveRawOutputItems(structured []Item, rawTopLevel []llm.OpenAIResponsesRawItem) []Item {
	if len(rawTopLevel) == 0 {
		return structured
	}

	structuredByKey := map[string][]int{}
	ordinalByType := map[string]int{}
	for i := range structured {
		key := outputItemSemanticKey(structured[i], ordinalByType)
		if key == "" {
			continue
		}
		structuredByKey[key] = append(structuredByKey[key], i)
	}

	used := make([]bool, len(structured))
	restored := make([]Item, 0, len(structured)+len(rawTopLevel))

	for _, rawItem := range rawTopLevel {
		if rawItem.SemanticKey == "" {
			if item, ok := itemFromRawOutput(rawItem); ok {
				restored = append(restored, item)
			}
			continue
		}

		indexes := structuredByKey[rawItem.SemanticKey]
		if len(indexes) == 0 {
			if item, ok := itemFromRawOutput(rawItem); ok {
				restored = append(restored, item)
			}
			continue
		}

		idx := indexes[0]
		structuredByKey[rawItem.SemanticKey] = indexes[1:]
		used[idx] = true
		restored = append(restored, structured[idx])
	}

	for i, item := range structured {
		if !used[i] {
			restored = append(restored, item)
		}
	}

	return restored
}

func appendRawOnlyOutputItems(structured []Item, rawTopLevel []llm.OpenAIResponsesRawItem) []Item {
	if len(rawTopLevel) == 0 {
		return structured
	}

	restored := append([]Item(nil), structured...)
	for _, rawItem := range rawTopLevel {
		if rawItem.SemanticKey != "" {
			continue
		}
		if item, ok := itemFromRawOutput(rawItem); ok {
			restored = append(restored, item)
		}
	}

	return restored
}

func itemFromRawOutput(rawItem llm.OpenAIResponsesRawItem) (Item, bool) {
	if len(rawItem.Raw) == 0 {
		return Item{}, false
	}

	var item Item
	if err := json.Unmarshal(rawItem.Raw, &item); err != nil {
		return Item{}, false
	}
	for _, key := range []string{"content", "input", "output"} {
		value := rawField(rawItem.Raw, key)
		if len(value) == 0 {
			continue
		}
		if item.Extra == nil {
			item.Extra = map[string]json.RawMessage{}
		}
		item.Extra[key] = value
	}

	return item, true
}

func rawItemOriginalIndex(item llm.OpenAIResponsesRawItem) int {
	if item.OriginalIndex == nil {
		return int(^uint(0) >> 1)
	}

	return *item.OriginalIndex
}

func mergeRawMaps(
	raw map[string]json.RawMessage,
	structured map[string]json.RawMessage,
) map[string]json.RawMessage {
	if len(raw) == 0 && len(structured) == 0 {
		return nil
	}

	merged := cloneRawMap(raw)
	if merged == nil {
		merged = map[string]json.RawMessage{}
	}
	for key, value := range structured {
		merged[key] = cloneRaw(value)
	}

	return merged
}
