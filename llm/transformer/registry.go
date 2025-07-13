package transformer

import (
	"fmt"
	"sync"
)

// Registry implements TransformerRegistry interface
type Registry struct {
	mu                  sync.RWMutex
	inboundTransformers map[string]Transformer
}

// NewRegistry creates a new transformer registry
func NewRegistry() TransformerRegistry {
	return &Registry{
		inboundTransformers: make(map[string]Transformer),
	}
}

// RegisterInboundTransformer registers an inbound transformer
func (r *Registry) RegisterInboundTransformer(name string, transformer Transformer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inboundTransformers[name] = transformer
}

// GetInboundTransformer retrieves an inbound transformer by name
func (r *Registry) GetInboundTransformer(name string) (Transformer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	transformer, exists := r.inboundTransformers[name]
	if !exists {
		return nil, fmt.Errorf("inbound transformer %s not found", name)
	}
	return transformer, nil
}

// ListInboundTransformers returns all registered inbound transformer names
func (r *Registry) ListInboundTransformers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.inboundTransformers))
	for name := range r.inboundTransformers {
		names = append(names, name)
	}
	return names
}

// UnregisterInboundTransformer removes an inbound transformer
func (r *Registry) UnregisterInboundTransformer(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.inboundTransformers, name)
}

// GetSupportedFormats returns all supported formats
func (r *Registry) GetSupportedFormats() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	formats := make([]string, 0)
	formats = append(formats, "application/json") // Default supported format
	return formats
}
