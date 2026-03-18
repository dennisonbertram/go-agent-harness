package catalog

import (
	"fmt"
	"os"
	"sync"
)

// ProviderClient is the interface that provider clients must implement.
// This avoids an import cycle with the harness package.
type ProviderClient interface{}

// ClientFactory creates a provider client given API key, base URL, and provider name.
type ClientFactory func(apiKey, baseURL, providerName string) (ProviderClient, error)

// ProviderRegistry holds a Catalog and lazily creates provider client instances per provider.
type ProviderRegistry struct {
	catalog       *Catalog
	mu            sync.RWMutex
	clients       map[string]ProviderClient
	overrideKeys  map[string]string
	getenv        func(string) string
	clientFactory ClientFactory
}

// NewProviderRegistry creates a registry that uses os.Getenv for API key lookup.
func NewProviderRegistry(catalog *Catalog) *ProviderRegistry {
	return &ProviderRegistry{
		catalog: catalog,
		clients: make(map[string]ProviderClient),
		getenv:  os.Getenv,
	}
}

// NewProviderRegistryWithEnv creates a registry with a custom getenv function (for testing).
func NewProviderRegistryWithEnv(catalog *Catalog, getenv func(string) string) *ProviderRegistry {
	if getenv == nil {
		getenv = os.Getenv
	}
	return &ProviderRegistry{
		catalog: catalog,
		clients: make(map[string]ProviderClient),
		getenv:  getenv,
	}
}

// SetClientFactory sets the factory function used to create provider clients.
// Must be called before GetClient/GetClientForModel if client creation is needed.
func (r *ProviderRegistry) SetClientFactory(factory ClientFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clientFactory = factory
}

// SetAPIKey stores a runtime API key override for the named provider.
// When set, GetClient uses this key instead of the environment variable.
func (r *ProviderRegistry) SetAPIKey(provider, key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.overrideKeys == nil {
		r.overrideKeys = make(map[string]string)
	}
	r.overrideKeys[provider] = key
	// Evict any cached client so the next GetClient uses the new key.
	delete(r.clients, provider)
}

// IsConfigured returns true if the named provider has an API key available,
// either via a runtime override or the environment variable.
func (r *ProviderRegistry) IsConfigured(providerName string) bool {
	r.mu.RLock()
	if k := r.overrideKeys[providerName]; k != "" {
		r.mu.RUnlock()
		return true
	}
	r.mu.RUnlock()
	entry, ok := r.catalog.Providers[providerName]
	if !ok {
		return false
	}
	return r.getenv(entry.APIKeyEnv) != ""
}

// GetClient returns (or lazily creates) a provider client for the named provider.
func (r *ProviderRegistry) GetClient(providerName string) (ProviderClient, error) {
	// Fast path: check if already created.
	r.mu.RLock()
	if client, ok := r.clients[providerName]; ok {
		r.mu.RUnlock()
		return client, nil
	}
	r.mu.RUnlock()

	// Slow path: create client under write lock.
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock.
	if client, ok := r.clients[providerName]; ok {
		return client, nil
	}

	entry, ok := r.catalog.Providers[providerName]
	if !ok {
		return nil, fmt.Errorf("provider %q not found in catalog", providerName)
	}

	// Check runtime override before falling back to environment variable.
	apiKey := r.overrideKeys[providerName]
	if apiKey == "" {
		apiKey = r.getenv(entry.APIKeyEnv)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("provider %q: API key env %q is not set", providerName, entry.APIKeyEnv)
	}

	if r.clientFactory == nil {
		return nil, fmt.Errorf("provider %q: no client factory configured", providerName)
	}

	client, err := r.clientFactory(apiKey, entry.BaseURL, providerName)
	if err != nil {
		return nil, fmt.Errorf("create client for provider %q: %w", providerName, err)
	}

	r.clients[providerName] = client
	return client, nil
}

// GetClientForModel searches all providers to find which one has the given model,
// returns the client and provider name.
func (r *ProviderRegistry) GetClientForModel(modelID string) (ProviderClient, string, error) {
	providerName, found := r.ResolveProvider(modelID)
	if !found {
		return nil, "", fmt.Errorf("model %q not found in any provider", modelID)
	}

	client, err := r.GetClient(providerName)
	if err != nil {
		return nil, "", err
	}
	return client, providerName, nil
}

// ResolveProvider searches all providers to find which one has the given model (including aliases).
func (r *ProviderRegistry) ResolveProvider(modelID string) (string, bool) {
	if r.catalog == nil {
		return "", false
	}
	for name, entry := range r.catalog.Providers {
		// Check direct model match.
		if _, ok := entry.Models[modelID]; ok {
			return name, true
		}
		// Check alias match.
		if target, ok := entry.Aliases[modelID]; ok {
			if _, modelOK := entry.Models[target]; modelOK {
				return name, true
			}
		}
	}
	return "", false
}

// Catalog returns the underlying catalog (read-only access).
func (r *ProviderRegistry) Catalog() *Catalog {
	return r.catalog
}

// MaxContextTokens returns the context window size for the given model from the
// catalog. Returns 0 and false when the model is not found or the registry has
// no catalog. The value comes from Model.ContextWindow which is validated > 0
// at load time, so a non-zero return is always a valid token count.
func (r *ProviderRegistry) MaxContextTokens(modelID string) (int, bool) {
	if r == nil || r.catalog == nil {
		return 0, false
	}
	providerName, found := r.ResolveProvider(modelID)
	if !found {
		return 0, false
	}
	entry, ok := r.catalog.Providers[providerName]
	if !ok {
		return 0, false
	}
	// Resolve alias if needed.
	resolved := modelID
	if target, ok := entry.Aliases[modelID]; ok {
		if _, modelOK := entry.Models[target]; modelOK {
			resolved = target
		}
	}
	m, ok := entry.Models[resolved]
	if !ok {
		return 0, false
	}
	return m.ContextWindow, true
}
