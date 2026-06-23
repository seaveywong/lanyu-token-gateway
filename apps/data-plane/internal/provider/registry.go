// Package provider implements the ProviderAdapter registry and per-provider
// adapters that translate Canonical IR requests into upstream HTTP calls.
package provider

import (
	"fmt"
	"sync"

	"github.com/seaveywong/lanyu-token-gateway/packages/provider_sdk"
)

// Registry holds all registered ProviderAdapter instances, keyed by ProviderID.
// It is safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	adapters map[provider_sdk.ProviderID]provider_sdk.ProviderAdapter
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[provider_sdk.ProviderID]provider_sdk.ProviderAdapter),
	}
}

// Register adds an adapter to the registry. It returns an error if an adapter
// with the same ProviderID already exists.
func (r *Registry) Register(adapter provider_sdk.ProviderAdapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := adapter.Provider()
	if _, exists := r.adapters[id]; exists {
		return fmt.Errorf("provider adapter %q is already registered", id)
	}
	r.adapters[id] = adapter
	return nil
}

// Get returns the adapter for the given provider ID, or an error if not found.
func (r *Registry) Get(id provider_sdk.ProviderID) (provider_sdk.ProviderAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, exists := r.adapters[id]
	if !exists {
		return nil, fmt.Errorf("no adapter registered for provider %q", id)
	}
	return adapter, nil
}

// List returns all registered provider IDs in no guaranteed order.
func (r *Registry) List() []provider_sdk.ProviderID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]provider_sdk.ProviderID, 0, len(r.adapters))
	for id := range r.adapters {
		ids = append(ids, id)
	}
	return ids
}
