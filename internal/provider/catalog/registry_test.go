package catalog

import (
	"sync"
	"testing"
)

func registryTestCatalog() *Catalog {
	return &Catalog{
		CatalogVersion: "v1-test",
		Providers: map[string]ProviderEntry{
			"openai": {
				DisplayName: "OpenAI",
				BaseURL:     "https://api.openai.com",
				APIKeyEnv:   "OPENAI_API_KEY",
				Protocol:    "openai",
				Models: map[string]Model{
					"gpt-4.1-mini": {
						DisplayName:   "GPT-4.1 Mini",
						ContextWindow: 128000,
						ToolCalling:   true,
						Streaming:     true,
					},
				},
			},
			"deepseek": {
				DisplayName: "DeepSeek",
				BaseURL:     "https://api.deepseek.com",
				APIKeyEnv:   "DEEPSEEK_API_KEY",
				Protocol:    "openai",
				Models: map[string]Model{
					"deepseek-chat": {
						DisplayName:   "DeepSeek Chat",
						ContextWindow: 64000,
						ToolCalling:   true,
						Streaming:     true,
					},
					"deepseek-reasoner": {
						DisplayName:   "DeepSeek Reasoner",
						ContextWindow: 64000,
						ToolCalling:   true,
						Streaming:     true,
					},
				},
				Aliases: map[string]string{
					"deepseek": "deepseek-chat",
				},
			},
		},
	}
}

func fakeGetenv(vals map[string]string) func(string) string {
	return func(key string) string {
		return vals[key]
	}
}

// stubClient is a test double that implements ProviderClient.
type stubClient struct {
	providerName string
}

func stubFactory(apiKey, baseURL, providerName string) (ProviderClient, error) {
	return &stubClient{providerName: providerName}, nil
}

func TestGetClient_KnownProvider(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistryWithEnv(cat, fakeGetenv(map[string]string{
		"OPENAI_API_KEY": "sk-test-key",
	}))
	reg.SetClientFactory(stubFactory)

	client, err := reg.GetClient("openai")
	if err != nil {
		t.Fatalf("GetClient(openai) error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestGetClient_UnknownProvider(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistryWithEnv(cat, fakeGetenv(map[string]string{}))
	reg.SetClientFactory(stubFactory)

	_, err := reg.GetClient("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestGetClient_MissingAPIKey(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistryWithEnv(cat, fakeGetenv(map[string]string{}))
	reg.SetClientFactory(stubFactory)

	_, err := reg.GetClient("openai")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestGetClient_NoFactory(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistryWithEnv(cat, fakeGetenv(map[string]string{
		"OPENAI_API_KEY": "sk-test-key",
	}))

	_, err := reg.GetClient("openai")
	if err == nil {
		t.Fatal("expected error when no factory is configured")
	}
}

func TestGetClientForModel_FindsCorrectProvider(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistryWithEnv(cat, fakeGetenv(map[string]string{
		"DEEPSEEK_API_KEY": "ds-test-key",
	}))
	reg.SetClientFactory(stubFactory)

	client, providerName, err := reg.GetClientForModel("deepseek-chat")
	if err != nil {
		t.Fatalf("GetClientForModel error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if providerName != "deepseek" {
		t.Fatalf("expected provider deepseek, got %q", providerName)
	}
}

func TestGetClientForModel_UnknownModel(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistryWithEnv(cat, fakeGetenv(map[string]string{}))
	reg.SetClientFactory(stubFactory)

	_, _, err := reg.GetClientForModel("unknown-model")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestResolveProvider_FindsCorrectProvider(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistry(cat)

	name, found := reg.ResolveProvider("gpt-4.1-mini")
	if !found {
		t.Fatal("expected to find provider for gpt-4.1-mini")
	}
	if name != "openai" {
		t.Fatalf("expected openai, got %q", name)
	}
}

func TestResolveProvider_FindsViaAlias(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistry(cat)

	name, found := reg.ResolveProvider("deepseek")
	if !found {
		t.Fatal("expected to find provider for deepseek alias")
	}
	if name != "deepseek" {
		t.Fatalf("expected deepseek, got %q", name)
	}
}

func TestResolveProvider_UnknownModel(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistry(cat)

	_, found := reg.ResolveProvider("nonexistent-model")
	if found {
		t.Fatal("expected not found for nonexistent model")
	}
}

func TestGetClient_ThreadSafety(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistryWithEnv(cat, fakeGetenv(map[string]string{
		"OPENAI_API_KEY":   "sk-test-key",
		"DEEPSEEK_API_KEY": "ds-test-key",
	}))
	reg.SetClientFactory(stubFactory)

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := reg.GetClient("openai")
			if err != nil {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			_, err := reg.GetClient("deepseek")
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent GetClient error: %v", err)
	}
}

func TestGetClient_LazyInitialization(t *testing.T) {
	t.Parallel()
	cat := registryTestCatalog()
	reg := NewProviderRegistryWithEnv(cat, fakeGetenv(map[string]string{
		"OPENAI_API_KEY": "sk-test-key",
	}))
	reg.SetClientFactory(stubFactory)

	client1, err := reg.GetClient("openai")
	if err != nil {
		t.Fatalf("first GetClient error: %v", err)
	}
	client2, err := reg.GetClient("openai")
	if err != nil {
		t.Fatalf("second GetClient error: %v", err)
	}
	if client1 != client2 {
		t.Fatal("expected same client instance on second call (lazy init)")
	}
}

func TestCatalogAccessor(t *testing.T) {
	t.Parallel()

	cat := registryTestCatalog()
	reg := NewProviderRegistry(cat)

	got := reg.Catalog()
	if got != cat {
		t.Fatal("expected Catalog() to return same pointer")
	}
	if got.CatalogVersion != "v1-test" {
		t.Fatalf("expected v1-test, got %q", got.CatalogVersion)
	}
}

func TestCatalogAccessorNil(t *testing.T) {
	t.Parallel()

	reg := NewProviderRegistry(nil)
	if reg.Catalog() != nil {
		t.Fatal("expected nil catalog")
	}
}
