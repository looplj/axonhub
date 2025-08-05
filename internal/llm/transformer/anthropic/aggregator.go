package anthropic

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, errors.New("empty stream chunks")
	}

	var (
		messageStart  *StreamEvent
		contentBlocks []ContentBlock
		usage         *Usage
		stopReason    *string
	)

	for _, chunk := range chunks {
		var event StreamEvent
		if err := json.Unmarshal(chunk.Data, &event); err != nil {
			continue // Skip invalid chunks
		}

		log.Debug(ctx, "chat stream event", log.Any("event", event))

		switch event.Type {
		case "message_start":
			messageStart = &event
		case "content_block_start":
			if event.ContentBlock != nil {
				contentBlocks = append(contentBlocks, *event.ContentBlock)
			}
		case "content_block_delta":
			if event.Index != nil {
				index := int(*event.Index)
				// Ensure we have enough content blocks
				for len(contentBlocks) <= index {
					contentBlocks = append(contentBlocks, ContentBlock{Type: "text", Text: ""})
				}

				if event.Delta != nil {
					if event.Delta.Text != nil {
						if contentBlocks[index].Type == "text" {
							contentBlocks[index].Text += *event.Delta.Text
						}
					}

					if event.Delta.Thinking != nil {
						if contentBlocks[index].Type == "thinking" {
							contentBlocks[index].Thinking += *event.Delta.Thinking
						} else {
							// Convert to thinking block if it's not already
							contentBlocks[index].Type = "thinking"
							contentBlocks[index].Thinking = *event.Delta.Thinking
						}
					}

					if event.Delta.Signature != nil {
						// Handle signature delta - append to thinking block signature
						if contentBlocks[index].Type == "thinking" {
							contentBlocks[index].Signature += *event.Delta.Signature
						} else {
							// Convert to thinking block if it's not already
							contentBlocks[index].Type = "thinking"
							contentBlocks[index].Signature = *event.Delta.Signature
						}
					}

					if event.Delta.PartialJSON != nil {
						switch contentBlocks[index].Type {
						case "tool_use":
							if contentBlocks[index].Input == nil {
								contentBlocks[index].Input = []byte(*event.Delta.PartialJSON)
							} else {
								contentBlocks[index].Input = append(contentBlocks[index].Input, []byte(*event.Delta.PartialJSON)...)
							}
						case "text":
							contentBlocks[index].Text += *event.Delta.PartialJSON
						}
					}
				}
			}
		case "message_delta":
			if event.Delta != nil {
				if event.Delta.StopReason != nil {
					stopReason = event.Delta.StopReason
				}
			}

			if event.Usage != nil {
				usage = event.Usage
			}
		case "message_stop":
			// Final event, no additional processing needed
		}
	}

	// If no message_start event, create a default message
	var message *Message

	if messageStart != nil {
		// Ensure we have at least one content block
		if len(contentBlocks) == 0 {
			contentBlocks = []ContentBlock{
				{Type: "text", Text: ""},
			}
		}

		message = &Message{
			ID:         messageStart.Message.ID,
			Type:       messageStart.Message.Type,
			Role:       messageStart.Message.Role,
			Content:    contentBlocks,
			Model:      messageStart.Message.Model,
			StopReason: stopReason,
			Usage:      usage,
		}
	} else {
		// Ensure we have at least one content block
		if len(contentBlocks) == 0 {
			contentBlocks = []ContentBlock{
				{Type: "text", Text: ""},
			}
		}

		// Create a default message when no message_start event is received
		message = &Message{
			ID:         "msg_unknown",
			Type:       "message",
			Role:       "assistant",
			Content:    contentBlocks,
			Model:      "claude-3-sonnet-20240229",
			StopReason: stopReason,
			Usage:      usage,
		}
	}

	// Convert the Anthropic message to ChatCompletion format
	transformer := &OutboundTransformer{}
	chatResp := transformer.convertToChatCompletionResponse(message)

	return json.Marshal(chatResp)
}
