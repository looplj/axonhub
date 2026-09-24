package pipeline

import (
	"maps"

	"github.com/looplj/axonhub/llm"
)

// mergeTransformerMetadata fills missing destination keys from the source.
// Values are opaque to pipeline; existing destination values take precedence.
// A new map prevents writes to one attempt or event from changing another.
func mergeTransformerMetadata(source, destination map[string]any) map[string]any {
	if len(source) == 0 {
		return maps.Clone(destination)
	}
	result := maps.Clone(source)
	maps.Copy(result, destination)
	return result
}

// withTransformerMetadata preserves decoder-owned responses and shared sentinels.
func withTransformerMetadata(metadata map[string]any, response *llm.Response) *llm.Response {
	if len(metadata) == 0 || response == nil || response == llm.DoneResponse {
		return response
	}
	result := *response
	result.TransformerMetadata = mergeTransformerMetadata(metadata, response.TransformerMetadata)
	return &result
}
