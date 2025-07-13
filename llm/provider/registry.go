package provider

import (
	"fmt"
	"sync"
)

// Registry implements ProviderRegistry interface
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	modelMap  map[string]string // model -> provider name mapping
}

// NewRegistry creates a new provider registry
func NewRegistry() ProviderRegistry {
	return &Registry{
		providers: make(map[string]Provider),
		modelMap:  make(map[string]string),
	}
}

// RegisterProvider registers a provider
func (r *Registry) RegisterProvider(name string, provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

// GetProvider retrieves a provider by name
func (r *Registry) GetProvider(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return provider, nil
}

// ListProviders returns all registered provider names
func (r *Registry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// UnregisterProvider removes a provider
func (r *Registry) UnregisterProvider(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
	
	// Remove model mappings for this provider
	for model, providerName := range r.modelMap {
		if providerName == name {
			delete(r.modelMap, model)
		}
	}
}

// GetProviderForModel returns the appropriate provider for a model
func (r *Registry) GetProviderForModel(model string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// First check if we have a specific mapping
	if providerName, exists := r.modelMap[model]; exists {
		if provider, exists := r.providers[providerName]; exists {
			return provider, nil
		}
	}
	
	// Otherwise, check all providers to see which one supports the model
	for _, provider := range r.providers {
		if provider.SupportsModel(model) {
			return provider, nil
		}
	}
	
	return nil, fmt.Errorf("no provider found for model %s", model)
}

// RegisterModelMapping registers a model to provider mapping
func (r *Registry) RegisterModelMapping(model, providerName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modelMap[model] = providerName
}