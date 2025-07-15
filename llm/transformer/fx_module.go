package transformer

import (
	"go.uber.org/fx"
)

// NewTransformerRegistry creates a new transformer registry
func NewTransformerRegistry() TransformerRegistry {
	return NewRegistry()
}

var Module = fx.Module("transformer",
	fx.Provide(NewTransformerRegistry),
)