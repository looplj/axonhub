package orchestrator

import (
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// chatCapableAPIFormats lists the API formats that can handle chat requests.
var chatCapableAPIFormats = map[string]struct{}{
	"openai/chat_completions": {},
	"openai/responses":        {},
	"anthropic/messages":      {},
	"gemini/contents":         {},
	"ollama/chat":             {},
}

// embeddingCapableAPIFormats lists API formats for embedding requests.
var embeddingCapableAPIFormats = map[string]struct{}{
	"openai/embeddings": {},
	"jina/embeddings":   {},
	"gemini/embeddings": {},
}

// imageCapableAPIFormats lists API formats for image requests.
var imageCapableAPIFormats = map[string]struct{}{
	"openai/image_generation": {},
	"openai/image_edit":       {},
	"openai/image_variation":  {},
}

// rerankCapableAPIFormats lists API formats for rerank requests.
var rerankCapableAPIFormats = map[string]struct{}{
	"jina/rerank": {},
}

// SelectAPIFormatForRequestType selects the most appropriate APIFormat from a channel's
// resolved endpoints based on the request type. Returns the first matching endpoint's
// APIFormat, or the first endpoint's APIFormat as a fallback.
func SelectAPIFormatForRequestType(endpoints []objects.ChannelEndpoint, requestType llm.RequestType) string {
	if len(endpoints) == 0 {
		return ""
	}

	var allowed map[string]struct{}

	//nolint:exhaustive // checked.
	switch requestType {
	case llm.RequestTypeChat:
		allowed = chatCapableAPIFormats
	case llm.RequestTypeEmbedding:
		allowed = embeddingCapableAPIFormats
	case llm.RequestTypeImage:
		allowed = imageCapableAPIFormats
	case llm.RequestTypeRerank:
		allowed = rerankCapableAPIFormats
	}

	if allowed != nil {
		for _, ep := range endpoints {
			if _, ok := allowed[ep.APIFormat]; ok {
				return ep.APIFormat
			}
		}
	}

	return endpoints[0].APIFormat
}
