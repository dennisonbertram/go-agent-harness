package modelstore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(filepath.Join(t.TempDir(), "models.json"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

func modelsHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }
}

// Seeding must never clobber a provider the user has already configured —
// their endpoint override has to survive a restart.
func TestSeedDoesNotOverwriteUserConfiguration(t *testing.T) {
	svc := newTestService(t)
	if err := svc.PutProvider(context.Background(), Provider{
		Name: "openai", BaseURL: "https://proxy.internal/v1", KeyRef: EnvRef("K"),
	}, ""); err != nil {
		t.Fatalf("put: %v", err)
	}
	svc.SeedProvider(Provider{Name: "openai", BaseURL: "https://api.openai.com/v1"})

	providers, _ := svc.Snapshot()
	if got := providers["openai"].BaseURL; got != "https://proxy.internal/v1" {
		t.Fatalf("seeding overwrote the user's endpoint: %q", got)
	}
}

// Catalog pricing is how a provider whose API reports no cost still shows one.
func TestSeedModelsSuppliesCatalogPricing(t *testing.T) {
	svc := newTestService(t)
	svc.SeedProvider(Provider{Name: "anthropic", BaseURL: "https://api.anthropic.com/v1"})
	svc.SeedModels("anthropic", []Model{{ID: "claude-opus-5", InputCost: cost(5), OutputCost: cost(25)}})

	_, fetched := svc.Snapshot()
	m := fetched["anthropic"].Models[0]
	if m.CostSource != CostFromCatalog {
		t.Fatalf("cost source = %q, want %q", m.CostSource, CostFromCatalog)
	}
}

// Seeded catalog pricing must survive a live fetch that reports no pricing.
func TestFetchKeepsSeededPricingWhenProviderReportsNone(t *testing.T) {
	server := httptest.NewServer(modelsHandler(`{"data":[{"id":"claude-opus-5"}]}`))
	defer server.Close()

	svc := newTestService(t)
	svc.SeedProvider(Provider{Name: "anthropic", BaseURL: server.URL, Protocol: ProtocolAnthropic, KeyRef: EnvRef("ANTHRO_TEST")})
	svc.SeedModels("anthropic", []Model{{ID: "claude-opus-5", InputCost: cost(5), OutputCost: cost(25)}})
	t.Setenv("ANTHRO_TEST", "sk-x")

	if _, err := svc.FetchProvider(context.Background(), "anthropic"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	_, fetched := svc.Snapshot()
	m := fetched["anthropic"].Models[0]
	if m.InputCost == nil || *m.InputCost != 5 {
		t.Fatalf("seeded price lost after fetch: %v", m.InputCost)
	}
}

func TestFetchPersistsAcrossReload(t *testing.T) {
	server := httptest.NewServer(modelsHandler(`{"data":[{"id":"a"},{"id":"b"}]}`))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "models.json")
	svc, err := NewService(path)
	if err != nil {
		t.Fatal(err)
	}
	svc.SeedProvider(Provider{Name: "p", BaseURL: server.URL, AuthKind: AuthNone})
	n, err := svc.FetchProvider(context.Background(), "p")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if n != 2 {
		t.Fatalf("fetched %d models, want 2", n)
	}
	if err := svc.SetExposed("p", map[string]bool{"b": true}); err != nil {
		t.Fatalf("expose: %v", err)
	}

	reloaded, err := NewService(path)
	if err != nil {
		t.Fatal(err)
	}
	exposed, has := reloaded.ExposedModels()
	if !has {
		t.Fatal("selection did not persist")
	}
	if len(exposed["p"]) != 1 || exposed["p"][0].ID != "b" {
		t.Fatalf("wrong selection persisted: %+v", exposed["p"])
	}
}

func TestSetCostPersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	svc, err := NewService(path)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.SeedProvider(Provider{Name: "p", BaseURL: "https://example.com/v1"})
	svc.SeedModels("p", []Model{{ID: "model-a"}})

	if err := svc.SetCost("p", "model-a", 1.25, 4.5); err != nil {
		t.Fatalf("set cost: %v", err)
	}

	reloaded, err := NewService(path)
	if err != nil {
		t.Fatalf("reload service: %v", err)
	}
	_, fetched := reloaded.Snapshot()
	models := fetched["p"].Models
	if len(models) != 1 {
		t.Fatalf("reloaded models = %+v, want one model", models)
	}
	model := models[0]
	if model.InputCost == nil || *model.InputCost != 1.25 {
		t.Fatalf("input cost = %v, want 1.25", model.InputCost)
	}
	if model.OutputCost == nil || *model.OutputCost != 4.5 {
		t.Fatalf("output cost = %v, want 4.5", model.OutputCost)
	}
	if model.CostSource != CostFromUser {
		t.Fatalf("cost source = %q, want %q", model.CostSource, CostFromUser)
	}
}

// A fetch against a provider with no credential must say so plainly instead of
// sending an unauthenticated request and reporting the provider's 401.
func TestFetchWithoutCredentialExplainsItself(t *testing.T) {
	svc := newTestService(t)
	svc.SeedProvider(Provider{Name: "p", BaseURL: "https://example.invalid/v1", AuthKind: AuthAPIKey})
	_, err := svc.FetchProvider(context.Background(), "p")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("error should name the cause: %v", err)
	}
}

// A failed fetch is recorded against the provider so the UI can explain a
// stale list rather than silently showing old data.
func TestFailedFetchIsRecordedAndListSurvives(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failing.Close()

	svc := newTestService(t)
	svc.SeedProvider(Provider{Name: "p", BaseURL: failing.URL, AuthKind: AuthNone})
	svc.SeedModels("p", []Model{{ID: "kept", Exposed: true}})

	if _, err := svc.FetchProvider(context.Background(), "p"); err == nil {
		t.Fatal("expected the fetch to fail")
	}
	_, fetched := svc.Snapshot()
	if len(fetched["p"].Models) != 1 {
		t.Fatal("the previous list was discarded on failure")
	}
	if fetched["p"].Error == "" {
		t.Fatal("the failure was not recorded")
	}
}

// A provider needing no credential (Ollama, LM Studio) fetches fine.
func TestCredentiallessProviderFetches(t *testing.T) {
	server := httptest.NewServer(modelsHandler(`{"data":[{"id":"llama3"}]}`))
	defer server.Close()

	svc := newTestService(t)
	svc.SeedProvider(Provider{Name: "ollama", BaseURL: server.URL, AuthKind: AuthNone})
	if _, err := svc.FetchProvider(context.Background(), "ollama"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
}

func TestDeleteRemovesProviderAndModels(t *testing.T) {
	svc := newTestService(t)
	if err := svc.PutProvider(context.Background(), Provider{
		Name: "custom", BaseURL: "https://x/v1", KeyRef: EnvRef("K"),
	}, ""); err != nil {
		t.Fatal(err)
	}
	svc.SeedModels("custom", []Model{{ID: "m"}})
	if err := svc.DeleteProvider(context.Background(), "custom"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	providers, fetched := svc.Snapshot()
	if _, ok := providers["custom"]; ok {
		t.Fatal("provider survived deletion")
	}
	if _, ok := fetched["custom"]; ok {
		t.Fatal("models survived deletion")
	}
}

// Snapshot must hand out copies; a caller mutating the result must not corrupt
// the store.
func TestSnapshotIsACopy(t *testing.T) {
	svc := newTestService(t)
	svc.SeedProvider(Provider{Name: "p", BaseURL: "https://x/v1"})
	svc.SeedModels("p", []Model{{ID: "m"}})

	_, fetched := svc.Snapshot()
	fetched["p"].Models[0].ID = "mutated"

	_, again := svc.Snapshot()
	if again["p"].Models[0].ID != "m" {
		t.Fatal("the store was mutated through a snapshot")
	}
}

// A subscription provider's token comes from the vendor's refresh flow, not
// from a file. Without a resolver the fetch would go out unauthenticated and
// the provider would answer 401 — which reads as "your login is broken" when
// the login is fine.
func TestSubscriptionProviderUsesItsTokenResolver(t *testing.T) {
	var sawAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"id":"k3"}]}`))
	}))
	defer server.Close()

	svc := newTestService(t)
	svc.SeedProvider(Provider{
		Name: "kimi-subscription", BaseURL: server.URL, AuthKind: AuthSubscription,
	})
	svc.SetTokenResolver("kimi-subscription", func(context.Context) (string, error) {
		return "refreshed-token", nil
	})

	if _, err := svc.FetchProvider(context.Background(), "kimi-subscription"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if sawAuth != "Bearer refreshed-token" {
		t.Fatalf("Authorization = %q — the resolver's token was not used", sawAuth)
	}
}

// Without a resolver the error must point at the fix, not report a bare 401.
func TestSubscriptionWithoutResolverExplainsItself(t *testing.T) {
	svc := newTestService(t)
	svc.SeedProvider(Provider{
		Name: "kimi-subscription", BaseURL: "https://example.invalid", AuthKind: AuthSubscription,
	})
	_, err := svc.FetchProvider(context.Background(), "kimi-subscription")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "no token source") {
		t.Fatalf("error should name the cause: %v", err)
	}
}

// A provider that publishes no list is not a failure to shout about.
func TestNonListableProviderSaysSoPlainly(t *testing.T) {
	svc := newTestService(t)
	svc.SeedProvider(Provider{
		Name: "codex-subscription", BaseURL: "https://chatgpt.com/backend-api/codex",
		AuthKind: AuthSubscription, NoListing: true,
	})
	svc.SeedModels("codex-subscription", []Model{{ID: "gpt-5.6-sol"}})

	_, err := svc.FetchProvider(context.Background(), "codex-subscription")
	if err == nil {
		t.Fatal("expected a explanation")
	}
	if !strings.Contains(err.Error(), "does not publish a model list") {
		t.Fatalf("message should explain rather than alarm: %v", err)
	}
	// And the catalog-seeded model must remain available.
	_, fetched := svc.Snapshot()
	if len(fetched["codex-subscription"].Models) != 1 {
		t.Fatal("the seeded model was lost")
	}
}

// The settings page must not claim a credential exists just because a
// reference was configured. A seeded "env:CEREBRAS_API_KEY" is a reference
// whether or not the variable is set, and reporting that as configured is what
// made the page disagree with the fetch it had just refused.
func TestCredentialStatusResolvesRatherThanAssumes(t *testing.T) {
	svc := newTestService(t)
	svc.SeedProvider(Provider{
		Name: "cerebras", BaseURL: "https://api.cerebras.ai/v1",
		AuthKind: AuthAPIKey, KeyRef: EnvRef("MODELSTORE_UNSET_KEY"),
	})
	if svc.CredentialStatus(context.Background(), "cerebras") {
		t.Fatal("an unset environment variable must not count as a credential")
	}

	t.Setenv("MODELSTORE_UNSET_KEY", "sk-now-set")
	if !svc.CredentialStatus(context.Background(), "cerebras") {
		t.Fatal("a set environment variable should count")
	}
}

// A local server needs nothing, and must not be flagged as unconfigured.
func TestCredentiallessProviderCountsAsConfigured(t *testing.T) {
	svc := newTestService(t)
	svc.SeedProvider(Provider{Name: "ollama", BaseURL: "http://localhost:11434/v1", AuthKind: AuthNone})
	if !svc.CredentialStatus(context.Background(), "ollama") {
		t.Fatal("a provider needing no credential should count as configured")
	}
}

// Saving a key for an existing provider must give it a real credential —
// the path that was missing for the built-in providers entirely.
func TestSavingAKeyForAnExistingProviderMakesItUsable(t *testing.T) {
	if !KeychainAvailable() {
		t.Skip("keychain not available")
	}
	svc := newTestService(t)
	svc.SeedProvider(Provider{
		Name: "modelstore-selftest-provider", BaseURL: "https://x/v1",
		AuthKind: AuthAPIKey, KeyRef: EnvRef("MODELSTORE_STILL_UNSET"),
	})
	ctx := context.Background()
	if svc.CredentialStatus(ctx, "modelstore-selftest-provider") {
		t.Fatal("precondition: should start with no credential")
	}

	providers, _ := svc.Snapshot()
	p := providers["modelstore-selftest-provider"]
	p.KeyRef = "" // the UI sends no reference; the service picks the keychain
	t.Cleanup(func() {
		_ = DeleteCredential(ctx, KeychainRef("modelstore-selftest-provider"))
	})
	if err := svc.PutProvider(ctx, p, "sk-typed-in"); err != nil {
		t.Fatalf("save key: %v", err)
	}
	if !svc.CredentialStatus(ctx, "modelstore-selftest-provider") {
		t.Fatal("the provider still reports no credential after a key was saved")
	}
}
