package main

import (
	"testing"

	"go-agent-harness/internal/provider/catalog"
	"go-agent-harness/internal/provider/pricing"
	"go-agent-harness/internal/providercatalog"
)

// Discovery used to be a hardcoded list of four provider names, so a configured
// provider's newly released models stayed invisible until someone edited that
// list. A provider file that declares a models endpoint should get a discoverer
// on its own.
func TestDeclaredModelsEndpointGetsADiscoverer(t *testing.T) {
	cat := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{
		"acme": {BaseURL: "https://acme.example/v1", APIKeyEnv: "ACME_API_KEY",
			Protocol: providercatalog.ProtocolOpenAICompat,
			Models:   map[string]catalog.Model{"acme-1": {}}},
	}}
	registry := catalog.NewProviderRegistryWithEnv(cat, func(name string) string {
		if name == "ACME_API_KEY" {
			return "sk-test"
		}
		return ""
	})
	files := &providercatalog.Catalog{Providers: map[string]providercatalog.Provider{
		"acme": {
			ID: "acme", BaseURL: "https://acme.example/v1",
			Protocol:     providercatalog.ProtocolOpenAICompat,
			Auth:         providercatalog.Auth{Kind: providercatalog.AuthAPIKey, Env: "ACME_API_KEY"},
			Capabilities: providercatalog.Capabilities{ModelsEndpoint: "/models"},
		},
	}}

	registerModelDiscoverers(registry, files)

	if !registry.HasDiscovery("acme") {
		t.Fatal("a provider declaring a models endpoint got no discoverer, so its new models stay invisible")
	}
}

// A provider that publishes no listing must not be asked for one: a failed
// discovery costs a request every time it is retried.
func TestProviderWithoutAModelsEndpointGetsNoDiscoverer(t *testing.T) {
	cat := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{
		"quiet": {BaseURL: "https://quiet.example/v1", APIKeyEnv: "QUIET_API_KEY",
			Models: map[string]catalog.Model{"quiet-1": {}}},
	}}
	registry := catalog.NewProviderRegistryWithEnv(cat, func(name string) string {
		if name == "QUIET_API_KEY" {
			return "sk-test"
		}
		return ""
	})
	files := &providercatalog.Catalog{Providers: map[string]providercatalog.Provider{
		"quiet": {
			ID: "quiet", BaseURL: "https://quiet.example/v1",
			Protocol: providercatalog.ProtocolOpenAICompat,
			Auth:     providercatalog.Auth{Kind: providercatalog.AuthAPIKey, Env: "QUIET_API_KEY"},
			// No ModelsEndpoint.
		},
	}}

	registerModelDiscoverers(registry, files)

	if registry.HasDiscovery("quiet") {
		t.Fatal("a provider that publishes no listing was given a discoverer anyway")
	}
}

// An unconfigured provider has no credential to discover with, so asking would
// only produce an authentication failure.
func TestUnconfiguredProviderGetsNoDiscoverer(t *testing.T) {
	cat := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{
		"broke": {BaseURL: "https://broke.example/v1", APIKeyEnv: "BROKE_API_KEY",
			Models: map[string]catalog.Model{"broke-1": {}}},
	}}
	registry := catalog.NewProviderRegistryWithEnv(cat, func(string) string { return "" })
	files := &providercatalog.Catalog{Providers: map[string]providercatalog.Provider{
		"broke": {
			ID: "broke", BaseURL: "https://broke.example/v1",
			Protocol:     providercatalog.ProtocolOpenAICompat,
			Auth:         providercatalog.Auth{Kind: providercatalog.AuthAPIKey, Env: "BROKE_API_KEY"},
			Capabilities: providercatalog.Capabilities{ModelsEndpoint: "/models"},
		},
	}}

	registerModelDiscoverers(registry, files)

	if registry.HasDiscovery("broke") {
		t.Fatal("an unconfigured provider was given a discoverer, which can only fail to authenticate")
	}
}

// An explicit pricing file used to replace the provider files outright, so a
// provider it did not mention resolved to no rate at all — while the picker
// happily displayed one.
func TestExplicitPricingFileDoesNotHideProviderFileRates(t *testing.T) {
	explicit := pricing.NewResolverFromCatalog(&pricing.Catalog{
		Providers: map[string]pricing.ProviderCatalog{
			"known": {Models: map[string]pricing.Rates{
				"known-1": {InputPer1MTokensUSD: 1, OutputPer1MTokensUSD: 2},
			}},
		},
	})
	fromFiles := pricing.NewResolverFromCatalog(&pricing.Catalog{
		Providers: map[string]pricing.ProviderCatalog{
			"known": {Models: map[string]pricing.Rates{
				"known-1": {InputPer1MTokensUSD: 99, OutputPer1MTokensUSD: 99},
			}},
			"extra": {Models: map[string]pricing.Rates{
				"extra-1": {InputPer1MTokensUSD: 3, OutputPer1MTokensUSD: 4},
			}},
		},
	})
	resolver := pricing.NewFallbackResolver(explicit, fromFiles)

	// The explicit file wins where it speaks.
	got, ok := resolver.Resolve("known", "known-1")
	if !ok {
		t.Fatal("the explicitly configured rate did not resolve")
	}
	if got.Rates.InputPer1MTokensUSD != 1 {
		t.Fatalf("provider files overrode the explicit rate: got %v, want 1",
			got.Rates.InputPer1MTokensUSD)
	}

	// And no longer hides what it does not mention.
	got, ok = resolver.Resolve("extra", "extra-1")
	if !ok {
		t.Fatal("a provider absent from the explicit file resolved to no rate at all")
	}
	if got.Rates.InputPer1MTokensUSD != 3 {
		t.Fatalf("wrong fallback rate: got %v, want 3", got.Rates.InputPer1MTokensUSD)
	}
}
