package catalog

import (
	"context"
	"testing"
)

// Two providers serve "kimi-k2.5": metered "kimi" (needs MOONSHOT_API_KEY) and
// "kimi-subscription" (mirrors kimi's models via models_from). Map iteration
// order is random, so before this the same model routed to a different
// provider on each process start; landing on the unconfigured one silently
// fell back to the default provider and produced an unrelated error.
func TestResolvePrefersConfiguredProviderForSharedModel(t *testing.T) {
	catalog, err := LoadCatalog("../../../catalog/models.json")
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	const model = "kimi-k2.5"
	serving := 0
	for _, entry := range catalog.Providers {
		if _, ok := entry.Models[model]; ok {
			serving++
		}
	}
	if serving < 2 {
		t.Skipf("model %q is served by %d provider(s); this test needs an overlap", model, serving)
	}

	reg := NewProviderRegistry(catalog)
	// Only the subscription twin has credentials.
	reg.SetAPIKey("kimi-subscription", "test-token")

	// Resolution must be stable, not a coin flip across repeated calls.
	for i := 0; i < 50; i++ {
		name, found := reg.ResolveProvider(model)
		if !found {
			t.Fatalf("iteration %d: model %q did not resolve at all", i, model)
		}
		if name != "kimi-subscription" {
			t.Fatalf("iteration %d: resolved to %q, want the configured provider %q",
				i, name, "kimi-subscription")
		}
	}
}

// With no credentials anywhere the answer still must not vary run to run.
func TestResolveIsDeterministicWithNoConfiguredProvider(t *testing.T) {
	catalog, err := LoadCatalog("../../../catalog/models.json")
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	reg := NewProviderRegistry(catalog)

	first, found := reg.ResolveProvider("kimi-k2.5")
	if !found {
		t.Skip("model not present in this catalog")
	}
	for i := 0; i < 50; i++ {
		again, _ := reg.ResolveProvider("kimi-k2.5")
		if again != first {
			t.Fatalf("resolution is unstable: got %q then %q", first, again)
		}
	}
}

// stubDiscovery serves a fixed model list for a provider.
type stubDiscovery struct{ ids []string }

func (s stubDiscovery) Models(ctx context.Context) ([]DiscoveredModel, error) {
	out := make([]DiscoveredModel, 0, len(s.ids))
	for _, id := range s.ids {
		out = append(out, DiscoveredModel{ID: id})
	}
	return out, nil
}

// The same ambiguity exists for models that only ever came from a live fetch and
// are absent from the bundled catalog. That path iterated a map, so it picked a
// different provider from one call to the next and could land on an
// unconfigured one.
func TestDiscoveredModelResolvesToConfiguredProviderDeterministically(t *testing.T) {
	cat := &Catalog{Providers: map[string]ProviderEntry{
		"aaa-unconfigured": {BaseURL: "https://a.example/v1", Models: map[string]Model{}},
		"zzz-configured":   {BaseURL: "https://z.example/v1", Models: map[string]Model{}},
	}}
	reg := NewProviderRegistry(cat)
	// Both discover the same id; only the alphabetically later one has a key.
	reg.SetDiscovery("aaa-unconfigured", stubDiscovery{ids: []string{"shared-model"}})
	reg.SetDiscovery("zzz-configured", stubDiscovery{ids: []string{"shared-model"}})
	reg.SetAPIKey("zzz-configured", "test-token")

	for i := 0; i < 50; i++ {
		name, found := reg.ResolveProvider("shared-model")
		if !found {
			t.Fatalf("iteration %d: discovered model did not resolve", i)
		}
		if name != "zzz-configured" {
			t.Fatalf("iteration %d: resolved to %q, want the configured provider %q",
				i, name, "zzz-configured")
		}
	}
}

// A bundled provider with no credentials must not shadow a configured provider
// that also serves the model, or the run fails against a provider the user
// never set up.
func TestUnconfiguredCatalogMatchDoesNotShadowConfiguredDiscovery(t *testing.T) {
	cat := &Catalog{Providers: map[string]ProviderEntry{
		"bundled": {
			BaseURL: "https://bundled.example/v1",
			Models:  map[string]Model{"shared-model": {DisplayName: "Shared"}},
		},
		"gateway": {BaseURL: "https://gw.example/v1", Models: map[string]Model{}},
	}}
	reg := NewProviderRegistry(cat)
	reg.SetDiscovery("gateway", stubDiscovery{ids: []string{"shared-model"}})
	reg.SetAPIKey("gateway", "test-token") // only the gateway is usable

	name, found := reg.ResolveProvider("shared-model")
	if !found {
		t.Fatal("model did not resolve")
	}
	if name != "gateway" {
		t.Fatalf("resolved to %q, want the configured provider %q", name, "gateway")
	}
}
