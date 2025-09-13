package aisdk

import (
	"net/http"

	"github.com/looplj/axonhub/internal/llm/transformer"
)

// TransformerType represents the type of AI SDK transformer to use.
type TransformerType string

const (
	TransformerTypeText       TransformerType = "text"
	TransformerTypeDataStream TransformerType = "datastream"
)

// NewTransformer creates the appropriate AI SDK transformer based on the request headers.
func NewTransformer(headers http.Header) transformer.Inbound {
	// Check for AI SDK data stream header (case-insensitive)
	if headers.Get("X-Vercel-Ai-Ui-Message-Stream") == "v1" {
		return NewDataStreamTransformer()
	}

	// Default to text transformer for backward compatibility
	return NewTextTransformer()
}

func IsDataStream(headers http.Header) bool {
	return headers.Get("X-Vercel-Ai-Ui-Message-Stream") == "v1"
}

// NewTransformerByType creates a transformer by explicit type.
func NewTransformerByType(transformerType TransformerType) transformer.Inbound {
	switch transformerType {
	case TransformerTypeDataStream:
		return NewDataStreamTransformer()
	case TransformerTypeText:
		fallthrough
	default:
		return NewTextTransformer()
	}
}
