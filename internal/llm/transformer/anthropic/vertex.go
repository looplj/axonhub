package anthropic

import (
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/vertex"
)

// VertexTransformer implements the transformer for Anthropic Claude models on Google Vertex AI.
// It embeds an OutboundTransformer and implements CustomizeExecutor for Vertex AI integration.
type VertexTransformer struct {
	transformer.Outbound

	executor *vertex.Executor
}

func (t *VertexTransformer) CustomizeExecutor(defaultExecutor pipeline.Executor) pipeline.Executor {
	return t.executor
}

// Ensure VertexTransformer implements the ChannelCustomizedExecutor interface.
var _ pipeline.ChannelCustomizedExecutor = (*VertexTransformer)(nil)
