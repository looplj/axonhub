package anthropic

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/kaptinlin/jsonrepair"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

//nolint:maintidx // TODO: fix this.
func AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent, platformType PlatformType) ([]byte, llm.ResponseMeta, error) {
	if len(chunks) == 0 {
		return nil, llm.ResponseMeta{}, errors.New("empty stream chunks")
	}

	var (
		messageStart  *StreamEvent
		contentBlocks []MessageContentBlock
		usage         *Usage
		stopReason    *string
	)

	for _, chunk := range chunks {
		var event StreamEvent

		err := json.Unmarshal(chunk.Data, &event)
		if err != nil {
			continue // Skip invalid chunks
		}

		// log.Debug(ctx, "chat stream event", log.Any("event", event))

		switch event.Type {
		case "message_start":
			messageStart = &event
			if event.Message != nil && event.Message.Usage != nil {
				usage = event.Message.Usage
			}
		case "content_block_start":
			if event.ContentBlock != nil {
				block := *event.ContentBlock
				// For tool_use blocks, initialize Input as nil to be built from deltas
				if block.Type == "tool_use" {
					block.Input = nil
				}
				// redacted_thinking blocks come complete in content_block_start
				// with their Data field already populated

				contentBlocks = append(contentBlocks, block)
			}
		case "content_block_delta":
			if event.Index != nil {
				index := int(*event.Index)
				// Ensure we have enough content blocks
				for len(contentBlocks) <= index {
					contentBlocks = append(contentBlocks, MessageContentBlock{Type: "text", Text: lo.ToPtr("")})
				}

				if event.Delta != nil {
					if event.Delta.Text != nil {
						if contentBlocks[index].Type == "text" {
							if contentBlocks[index].Text == nil {
								contentBlocks[index].Text = lo.ToPtr("")
							}

							*contentBlocks[index].Text += *event.Delta.Text
						}
					}

					if event.Delta.Thinking != nil {
						if contentBlocks[index].Type == "thinking" {
							if contentBlocks[index].Thinking == nil {
								contentBlocks[index].Thinking = lo.ToPtr("")
							}

							*contentBlocks[index].Thinking += *event.Delta.Thinking
						} else {
							// Convert to thinking block if it's not already
							contentBlocks[index].Type = "thinking"
							contentBlocks[index].Thinking = event.Delta.Thinking
						}
					}

					if event.Delta.Signature != nil {
						// Handle signature delta - append to thinking block signature
						if contentBlocks[index].Type == "thinking" {
							if event.Delta.Signature != nil {
								if contentBlocks[index].Signature == nil {
									contentBlocks[index].Signature = event.Delta.Signature
								} else {
									contentBlocks[index].Signature = lo.ToPtr(*contentBlocks[index].Signature + *event.Delta.Signature)
								}
							}
						} else {
							// Convert to thinking block if it's not already
							contentBlocks[index].Type = "thinking"
							contentBlocks[index].Signature = event.Delta.Signature
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
							if contentBlocks[index].Text == nil {
								contentBlocks[index].Text = lo.ToPtr("")
							}

							*contentBlocks[index].Text += *event.Delta.PartialJSON
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
				if usage == nil {
					usage = event.Usage
				} else {
					// Merge usage information from message_delta with message_start
					// Keep input tokens from message_start, update output tokens from message_delta
					usage.OutputTokens = event.Usage.OutputTokens
					if event.Usage.InputTokens > 0 {
						usage.InputTokens = event.Usage.InputTokens
					}

					if event.Usage.CachedTokens > 0 {
						usage.CachedTokens = event.Usage.CachedTokens
						usage.InputTokens -= event.Usage.CacheReadInputTokens
					}

					if event.Usage.CacheCreationInputTokens > 0 {
						usage.CacheCreationInputTokens = event.Usage.CacheCreationInputTokens
					}

					if event.Usage.CacheReadInputTokens > 0 {
						usage.CacheReadInputTokens = event.Usage.CacheReadInputTokens
					}
				}
			}
		case "content_block_stop":
			if event.Index != nil && int(*event.Index) < len(contentBlocks) {
				index := int(*event.Index)

				block := contentBlocks[index]
				if block.Type == "tool_use" {
					if !json.Valid(block.Input) {
						log.Warn(ctx, "invalid tool use input", log.String("input", string(block.Input)))

						repaired, err := jsonrepair.JSONRepair(string(block.Input))
						if err == nil {
							block.Input = []byte(repaired)
						}
					}
				}
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
			contentBlocks = []MessageContentBlock{
				{Type: "text", Text: lo.ToPtr("")},
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
			contentBlocks = []MessageContentBlock{
				{Type: "text", Text: lo.ToPtr("")},
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

	data, err := json.Marshal(message)
	if err != nil {
		return nil, llm.ResponseMeta{}, err
	}

	// Convert and return usage if available
	if usage != nil {
		return data, llm.ResponseMeta{
			ID:    message.ID,
			Usage: convertToLlmUsage(usage, platformType),
		}, nil
	}

	return data, llm.ResponseMeta{
		ID: message.ID,
	}, nil
}
