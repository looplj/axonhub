package anthropic

import (
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/bedrock"
)

type BedrockTransformer struct {
	transformer.Outbound

	bedrock *bedrock.Executor
}

func (b *BedrockTransformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	return b.bedrock
}
