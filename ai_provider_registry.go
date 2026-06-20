package main

// ai_provider_registry.go — Provider factory registry

import (
	"fmt"
	"sync"
)

// ProviderFactory creates a provider from config
type ProviderFactory func(config map[string]any, apiKey string) (AgentProvider, error)

// ProviderRegistry holds registered provider factories
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]ProviderFactory
}

// NewProviderRegistry creates a registry and registers all built-in providers
func NewProviderRegistry() *ProviderRegistry {
	r := &ProviderRegistry{
		providers: make(map[string]ProviderFactory),
	}
	r.Register("openrouter", newOpenRouterProvider)
	r.Register("local", newLocalProvider)
	r.Register("mimo", newMiMoProvider)
	r.Register("webhook", newWebhookProvider)
	r.Register("websocket", newWebSocketProvider)
	r.Register("subprocess", newSubprocessProvider)
	r.Register("mcp", newMCPProvider)
	r.Register("reve", newReveProvider)
	return r
}

// Register adds a provider factory
func (r *ProviderRegistry) Register(name string, factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = factory
}

// Create creates a provider by type
func (r *ProviderRegistry) Create(providerType string, config map[string]any, apiKey string) (AgentProvider, error) {
	r.mu.RLock()
	factory, ok := r.providers[providerType]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
	return factory(config, apiKey)
}

// SupportedTypes returns all registered provider types
func (r *ProviderRegistry) SupportedTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]string, 0, len(r.providers))
	for k := range r.providers {
		types = append(types, k)
	}
	return types
}
