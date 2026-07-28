package main

import (
	"context"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/modelstore"
	"go-agent-harness/internal/provider/catalog"
)

func storeWith(t *testing.T, p modelstore.Provider, models []modelstore.Model) *modelstore.Service {
	t.Helper()
	svc, err := modelstore.NewService(filepath.Join(t.TempDir(), "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc.SeedProvider(p)
	svc.SeedModels(p.Name, models)
	return svc
}

// The regression: the picker offered a model that lived only in the store, and
// starting a run failed with "not found in any provider" because resolution
// only consulted the bundled catalog.
func TestStoreModelsAreResolvableByName(t *testing.T) {
	cat := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{
		"kimi-subscription": {
			DisplayName: "Kimi", BaseURL: "https://api.kimi.com/coding/v1",
			Protocol: "openai_compat", TokenSourceRequired: true,
			Models: map[string]catalog.Model{},
		},
	}}
	svc := storeWith(t,
		modelstore.Provider{
			Name: "kimi-subscription", BaseURL: "https://api.kimi.com/coding/v1",
			AuthKind: modelstore.AuthSubscription,
		},
		[]modelstore.Model{{ID: "k3"}, {ID: "k3-256k"}})

	registry := catalog.NewProviderRegistry(cat)
	registerStoreModels(svc, registry, cat)

	provider, found := registry.ResolveProviderContext(context.Background(), "k3")
	if !found {
		t.Fatal(`"k3" is offered by the picker but does not resolve to any provider`)
	}
	if provider != "kimi-subscription" {
		t.Fatalf("resolved to %q, want kimi-subscription", provider)
	}
}

// A provider the user added by hand is absent from the catalog, and the
// registry skips discoverers for unknown providers — so its models would be
// unrunnable without also registering a catalog entry.
func TestCustomProviderModelsAreResolvable(t *testing.T) {
	cat := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{}}
	svc := storeWith(t,
		modelstore.Provider{
			Name: "my-proxy", BaseURL: "https://gw.example/v1",
			Protocol: "openai_compat", AuthKind: modelstore.AuthAPIKey,
		},
		[]modelstore.Model{{ID: "house-model"}})

	registry := catalog.NewProviderRegistry(cat)
	registerStoreModels(svc, registry, cat)

	provider, found := registry.ResolveProviderContext(context.Background(), "house-model")
	if !found {
		t.Fatal("a custom provider's model does not resolve")
	}
	if provider != "my-proxy" {
		t.Fatalf("resolved to %q, want my-proxy", provider)
	}
	// The catalog entry must carry enough to build a client, not just a name.
	if got := cat.Providers["my-proxy"].BaseURL; got != "https://gw.example/v1" {
		t.Fatalf("catalog entry lost the endpoint: %q", got)
	}
}

// Registering must not overwrite a provider the catalog already describes.
func TestRegisteringDoesNotClobberCatalogEntries(t *testing.T) {
	cat := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{
		"openai": {DisplayName: "OpenAI", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY"},
	}}
	svc := storeWith(t,
		modelstore.Provider{Name: "openai", BaseURL: "https://proxy.local/v1"},
		[]modelstore.Model{{ID: "gpt-x"}})

	registerStoreModels(svc, catalog.NewProviderRegistry(cat), cat)

	if got := cat.Providers["openai"].APIKeyEnv; got != "OPENAI_API_KEY" {
		t.Fatalf("catalog entry was overwritten: APIKeyEnv = %q", got)
	}
}
