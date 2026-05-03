package responses

import "slices"

type streamEventLogSummary struct {
	Type         StreamEventType `json:"type"`
	Sequence     int             `json:"sequence_number"`
	HasResponse  bool            `json:"has_response"`
	HasItem      bool            `json:"has_item"`
	ItemType     string          `json:"item_type,omitempty"`
	OutputIndex  int             `json:"output_index,omitempty"`
	ContentIndex *int            `json:"content_index,omitempty"`
	DeltaBytes   int             `json:"delta_bytes,omitempty"`
	HasUsage     bool            `json:"has_usage"`
	HasError     bool            `json:"has_error"`
	ExtraKeys    []string        `json:"extra_keys,omitempty"`
}

func redactedStreamEventLogSummary(event StreamEvent) streamEventLogSummary {
	summary := streamEventLogSummary{
		Type:         event.Type,
		Sequence:     event.SequenceNumber,
		HasResponse:  event.Response != nil,
		HasItem:      event.Item != nil,
		OutputIndex:  event.OutputIndex,
		ContentIndex: event.ContentIndex,
		HasError:     event.Type == StreamEventTypeError || event.Code != "" || event.Message != "",
	}

	if event.Item != nil {
		summary.ItemType = event.Item.Type
	}
	summary.DeltaBytes = len(event.Delta)
	if event.Response != nil {
		summary.HasUsage = event.Response.Usage != nil
	}
	if len(event.Extra) > 0 {
		summary.ExtraKeys = make([]string, 0, len(event.Extra))
		for key := range event.Extra {
			summary.ExtraKeys = append(summary.ExtraKeys, key)
		}
		slices.Sort(summary.ExtraKeys)
	}

	return summary
}
