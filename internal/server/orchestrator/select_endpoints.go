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

// compactCapableAPIFormats lists API formats for compact requests.
var compactCapableAPIFormats = map[string]struct{}{
	"openai/responses_compact": {},
}

// completionCapableAPIFormats lists API formats for completion requests.
var completionCapableAPIFormats = map[string]struct{}{
	"openai/completions": {},
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

// videoCapableAPIFormats lists API formats for video requests.
var videoCapableAPIFormats = map[string]struct{}{
	"openai/video":   {},
	"seedance/video": {},
}

// SelectAPIFormat selects the most appropriate APIFormat from a channel's resolved endpoints
// based on the request type and inbound API format. Prefers an endpoint whose API format
// matches the inbound request format so that pass-through can be enabled when identical
// formats are used. Falls back to the first capable endpoint, then the first endpoint.
func SelectAPIFormat(endpoints []objects.ChannelEndpoint, req *llm.Request) string {
	if len(endpoints) == 0 {
		return ""
	}

	requestType := req.RequestType
	preferredFormat := string(req.APIFormat)

	var allowed map[string]struct{}

	//nolint:exhaustive // checked.
	switch requestType {
	case llm.RequestTypeChat:
		allowed = chatCapableAPIFormats
	case llm.RequestTypeCompact:
		allowed = compactCapableAPIFormats
	case llm.RequestTypeCompletion:
		allowed = completionCapableAPIFormats
	case llm.RequestTypeEmbedding:
		allowed = embeddingCapableAPIFormats
	case llm.RequestTypeImage:
		allowed = imageCapableAPIFormats
	case llm.RequestTypeRerank:
		allowed = rerankCapableAPIFormats
	case llm.RequestTypeVideo:
		allowed = videoCapableAPIFormats
	}

	if allowed != nil {
		if preferredFormat != "" {
			for _, ep := range endpoints {
				if _, ok := allowed[ep.APIFormat]; ok && ep.APIFormat == preferredFormat {
					return ep.APIFormat
				}
			}
		}

		for _, ep := range endpoints {
			if _, ok := allowed[ep.APIFormat]; ok {
				return ep.APIFormat
			}
		}
	}

	return endpoints[0].APIFormat
}
