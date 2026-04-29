package biz

import (
	"fmt"
	"strings"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// SupportedAPIFormats lists the API formats that are recognized as valid endpoint api_format values.
var SupportedAPIFormats = map[string]struct{}{
	llm.APIFormatOpenAIChatCompletion.String():  {},
	llm.APIFormatOpenAIResponse.String():        {},
	llm.APIFormatOpenAIResponseCompact.String(): {},
	llm.APIFormatOpenAIEmbedding.String():       {},
	llm.APIFormatOpenAIImageGeneration.String(): {},
	llm.APIFormatOpenAIImageEdit.String():       {},
	llm.APIFormatOpenAIImageVariation.String():  {},
	llm.APIFormatOpenAIVideo.String():           {},
	llm.APIFormatAnthropicMessage.String():      {},
	llm.APIFormatGeminiContents.String():        {},
	llm.APIFormatGeminiEmbedding.String():       {},
	llm.APIFormatJinaRerank.String():            {},
	llm.APIFormatJinaEmbedding.String():         {},
}

// ValidateEndpoints validates channel endpoint configurations.
// Ensures api_format is non-empty, supported, and unique within the channel.
// Ensures path is empty, starts with "/", and is not a full URL.
func ValidateEndpoints(endpoints []objects.ChannelEndpoint) error {
	seen := make(map[string]bool, len(endpoints))
	for i, ep := range endpoints {
		if ep.APIFormat == "" {
			return fmt.Errorf("endpoint[%d]: api_format is required", i)
		}

		if _, ok := SupportedAPIFormats[ep.APIFormat]; !ok {
			return fmt.Errorf("endpoint[%d]: unsupported api_format %q", i, ep.APIFormat)
		}

		if seen[ep.APIFormat] {
			return fmt.Errorf("endpoint[%d]: duplicate api_format %q", i, ep.APIFormat)
		}

		seen[ep.APIFormat] = true

		if ep.Path != "" {
			if strings.HasPrefix(ep.Path, "http://") || strings.HasPrefix(ep.Path, "https://") {
				return fmt.Errorf("endpoint[%d]: path must not be a full URL, got %q", i, ep.Path)
			}

			if !strings.HasPrefix(ep.Path, "/") {
				return fmt.Errorf("endpoint[%d]: path must start with '/', got %q", i, ep.Path)
			}
		}
	}

	return nil
}
