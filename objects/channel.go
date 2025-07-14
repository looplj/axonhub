package objects

import (
	"github.com/looplj/axonhub/llm/provider"
)

type ChannelSettings struct {
	ModelMappings []provider.ModelMapping
}
