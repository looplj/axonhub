package responses

import (
	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
)

// CompactAPIRequest is the request body for POST /v1/responses/compact.
type CompactAPIRequest struct {
	Model        string `json:"model"`
	Input        Input  `json:"input"`
	Instructions string `json:"instructions,omitempty"`
	// PreviousResponseID string `json:"previous_response_id,omitempty"` // Not supported yet.
	PromptCacheKey string `json:"prompt_cache_key,omitempty"`
}

// CompactAPIResponse is the response body from POST /v1/responses/compact.
type CompactAPIResponse struct {
	ID           string `json:"id"`
	CreatedAt    int64  `json:"created_at"`
	Object       string `json:"object"`
	Model        string `json:"model,omitempty"`
	Instructions string `json:"instructions,omitempty"`
	Output       []Item `json:"output"`
	Usage        *Usage `json:"usage,omitempty"`
}

func compactionContentFromItem(item *Item) *llm.CompactContent {
	return &llm.CompactContent{
		ID:               item.ID,
		EncryptedContent: lo.FromPtr(item.EncryptedContent),
		CreatedBy:        item.CreatedBy,
	}
}

func compactionMessageFromItem(item *Item, contentType string) *llm.Message {
	return &llm.Message{
		ID:   item.ID,
		Role: "assistant",
		Content: llm.MessageContent{
			MultipleContent: []llm.MessageContentPart{
				{
					ID:      item.ID,
					Type:    contentType,
					Compact: compactionContentFromItem(item),
				},
			},
		},
	}
}

func compactionContentPartFromItem(item *Item, contentType string) *llm.MessageContentPart {
	return &llm.MessageContentPart{
		ID:      item.ID,
		Type:    contentType,
		Compact: compactionContentFromItem(item),
	}
}

func compactionItemFromPart(p llm.MessageContentPart, contentType string) Item {
	return Item{
		ID:               p.Compact.ID,
		Type:             contentType,
		EncryptedContent: lo.ToPtr(p.Compact.EncryptedContent),
		CreatedBy:        p.Compact.CreatedBy,
	}
}
