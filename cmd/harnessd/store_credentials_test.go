package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/modelstore"
	"go-agent-harness/internal/provider/catalog"
)

// A key saved through the settings UI lands in the Keychain or a file and is
// recorded in the model store. The provider registry, which builds the client a
// run actually uses, reads only environment variables and runtime overrides —
// so without the handoff the settings page reports a working credential while
// every run fails with "API key env ... is not set". The credential is present
// and unusable at the same time, which is the worst of both.
func TestStoreCredentialReachesTheProviderRegistry(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key")
	if err := os.WriteFile(keyFile, []byte("sk-from-the-store"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc, err := modelstore.NewService(filepath.Join(dir, "models.json"))
	if err != nil {
		t.Fatalf("new model store: %v", err)
	}
	if err := svc.PutProvider(context.Background(), modelstore.Provider{
		Name:     "acme",
		BaseURL:  "https://acme.example/v1",
		Protocol: modelstore.ProtocolOpenAICompat,
		AuthKind: modelstore.AuthAPIKey,
		KeyRef:   "file:" + keyFile,
	}, ""); err != nil {
		t.Fatalf("put provider: %v", err)
	}

	cat := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{
		"acme": {BaseURL: "https://acme.example/v1", APIKeyEnv: "ACME_API_KEY",
			Models: map[string]catalog.Model{"acme-1": {}}},
	}}
	// An empty environment: the key exists only where the store points.
	registry := catalog.NewProviderRegistryWithEnv(cat, func(string) string { return "" })

	if registry.IsConfigured("acme") {
		t.Fatal("precondition: the registry must not already consider acme configured")
	}

	applied := applyStoreCredentials(context.Background(), svc, registry)
	if applied != 1 {
		t.Fatalf("handed %d credentials to the registry, want 1", applied)
	}
	if !registry.IsConfigured("acme") {
		t.Fatal("the registry still cannot see the stored credential, so a run would fail")
	}
}

// An env: reference needs no handoff — the registry already reads the
// environment itself, and copying it in would only duplicate what it can see.
func TestEnvReferencesAreNotCopiedIntoTheRegistry(t *testing.T) {
	dir := t.TempDir()
	svc, err := modelstore.NewService(filepath.Join(dir, "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.PutProvider(context.Background(), modelstore.Provider{
		Name:     "envy",
		BaseURL:  "https://envy.example/v1",
		Protocol: modelstore.ProtocolOpenAICompat,
		AuthKind: modelstore.AuthAPIKey,
		KeyRef:   modelstore.EnvRef("ENVY_API_KEY"),
	}, ""); err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{
		"envy": {BaseURL: "https://envy.example/v1", APIKeyEnv: "ENVY_API_KEY",
			Models: map[string]catalog.Model{"envy-1": {}}},
	}}
	registry := catalog.NewProviderRegistryWithEnv(cat, func(name string) string {
		if name == "ENVY_API_KEY" {
			return "sk-from-the-environment"
		}
		return ""
	})

	if applied := applyStoreCredentials(context.Background(), svc, registry); applied != 0 {
		t.Fatalf("copied %d env references into the registry, want 0", applied)
	}
	if !registry.IsConfigured("envy") {
		t.Fatal("the registry should already see the environment variable on its own")
	}
}

// A subscription provider has no static key to hand over; its credential comes
// from a token source that is wired separately.
func TestSubscriptionProvidersAreSkipped(t *testing.T) {
	dir := t.TempDir()
	svc, err := modelstore.NewService(filepath.Join(dir, "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.PutProvider(context.Background(), modelstore.Provider{
		Name:     "subby",
		BaseURL:  "https://subby.example/v1",
		Protocol: modelstore.ProtocolOpenAICompat,
		AuthKind: modelstore.AuthSubscription,
	}, ""); err != nil {
		t.Fatal(err)
	}
	registry := catalog.NewProviderRegistryWithEnv(
		&catalog.Catalog{Providers: map[string]catalog.ProviderEntry{}},
		func(string) string { return "" })
	if applied := applyStoreCredentials(context.Background(), svc, registry); applied != 0 {
		t.Fatalf("handed %d credentials for a subscription provider, want 0", applied)
	}
}
