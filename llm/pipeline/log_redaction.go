package pipeline

import (
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

type responseLogSummary struct {
	ID                     string `json:"id"`
	Object                 string `json:"object"`
	Model                  string `json:"model"`
	ChoiceCount            int    `json:"choice_count"`
	HasUsage               bool   `json:"has_usage"`
	TransformerMetadataKey int    `json:"transformer_metadata_key_count"`
}

func redactedResponseLogSummary(response *llm.Response) responseLogSummary {
	if response == nil {
		return responseLogSummary{}
	}

	return responseLogSummary{
		ID:                     response.ID,
		Object:                 response.Object,
		Model:                  response.Model,
		ChoiceCount:            len(response.Choices),
		HasUsage:               response.Usage != nil,
		TransformerMetadataKey: len(response.TransformerMetadata),
	}
}

type streamEventLogSummary struct {
	Type      string `json:"type"`
	DataBytes int    `json:"data_bytes"`
	LastEvent bool   `json:"has_last_event_id"`
}

func redactedStreamEventLogSummary(event *httpclient.StreamEvent) streamEventLogSummary {
	if event == nil {
		return streamEventLogSummary{}
	}

	return streamEventLogSummary{
		Type:      event.Type,
		DataBytes: len(event.Data),
		LastEvent: event.LastEventID != "",
	}
}
