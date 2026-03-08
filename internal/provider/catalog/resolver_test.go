package catalog

import (
	"go-agent-harness/internal/provider/pricing"
	"testing"
)

func resolverTestCatalog() *Catalog {
	return &Catalog{
		CatalogVersion: "1.0.0",
		Providers: map[string]ProviderEntry{
			"openai": {
				DisplayName: "OpenAI",
				BaseURL:     "https://api.openai.com/v1",
				APIKeyEnv:   "OPENAI_API_KEY",
				Protocol:    "openai_compat",
				Models: map[string]Model{
					"gpt-4.1-mini": {
						DisplayName:   "GPT-4.1 Mini",
						Description:   "Fast and affordable",
						ContextWindow: 1000000,
						ToolCalling:   true,
						Streaming:     true,
						Pricing: &ModelPricing{
							InputPer1MTokensUSD:  0.40,
							OutputPer1MTokensUSD: 1.60,
						},
					},
					"gpt-4.1": {
						DisplayName:   "GPT-4.1",
						Description:   "Flagship model",
						ContextWindow: 1000000,
						ToolCalling:   true,
						Streaming:     true,
						Pricing: &ModelPricing{
							InputPer1MTokensUSD:  2.00,
							OutputPer1MTokensUSD: 8.00,
						},
					},
				},
				Aliases: map[string]string{
					"gpt4-mini": "gpt-4.1-mini",
				},
			},
		},
	}
}

func TestCatalogPricingResolver_KnownModel(t *testing.T) {
	r := NewCatalogPricingResolver(resolverTestCatalog())

	rates, ok := r.Resolve("openai", "gpt-4.1-mini")
	if !ok {
		t.Fatal("expected ok=true for known model")
	}
	if rates.Provider != "openai" {
		t.Errorf("provider = %q, want %q", rates.Provider, "openai")
	}
	if rates.Model != "gpt-4.1-mini" {
		t.Errorf("model = %q, want %q", rates.Model, "gpt-4.1-mini")
	}
	if rates.PricingVersion != "1.0.0" {
		t.Errorf("pricing_version = %q, want %q", rates.PricingVersion, "1.0.0")
	}
	if rates.Rates.InputPer1MTokensUSD != 0.40 {
		t.Errorf("input = %f, want 0.40", rates.Rates.InputPer1MTokensUSD)
	}
	if rates.Rates.OutputPer1MTokensUSD != 1.60 {
		t.Errorf("output = %f, want 1.60", rates.Rates.OutputPer1MTokensUSD)
	}
}

func TestCatalogPricingResolver_AliasedModel(t *testing.T) {
	r := NewCatalogPricingResolver(resolverTestCatalog())

	rates, ok := r.Resolve("openai", "gpt4-mini")
	if !ok {
		t.Fatal("expected ok=true for aliased model")
	}
	if rates.Model != "gpt-4.1-mini" {
		t.Errorf("model = %q, want %q (resolved from alias)", rates.Model, "gpt-4.1-mini")
	}
	if rates.Rates.InputPer1MTokensUSD != 0.40 {
		t.Errorf("input = %f, want 0.40", rates.Rates.InputPer1MTokensUSD)
	}
}

func TestCatalogPricingResolver_UnknownProvider(t *testing.T) {
	r := NewCatalogPricingResolver(resolverTestCatalog())

	_, ok := r.Resolve("unknown", "gpt-4.1-mini")
	if ok {
		t.Fatal("expected ok=false for unknown provider")
	}
}

func TestCatalogPricingResolver_UnknownModel(t *testing.T) {
	r := NewCatalogPricingResolver(resolverTestCatalog())

	_, ok := r.Resolve("openai", "unknown-model")
	if ok {
		t.Fatal("expected ok=false for unknown model")
	}
}

func TestCatalogPricingResolver_NilCatalog(t *testing.T) {
	r := NewCatalogPricingResolver(nil)

	_, ok := r.Resolve("openai", "gpt-4.1-mini")
	if ok {
		t.Fatal("expected ok=false for nil catalog")
	}
}

func TestCatalogPricingResolver_NilResolver(t *testing.T) {
	var r *CatalogPricingResolver

	_, ok := r.Resolve("openai", "gpt-4.1-mini")
	if ok {
		t.Fatal("expected ok=false for nil resolver")
	}
}

func TestCatalogPricingResolver_EmptyProvider(t *testing.T) {
	r := NewCatalogPricingResolver(resolverTestCatalog())

	_, ok := r.Resolve("", "gpt-4.1-mini")
	if ok {
		t.Fatal("expected ok=false for empty provider")
	}
}

func TestCatalogPricingResolver_EmptyModel(t *testing.T) {
	r := NewCatalogPricingResolver(resolverTestCatalog())

	_, ok := r.Resolve("openai", "")
	if ok {
		t.Fatal("expected ok=false for empty model")
	}
}

func TestCatalogPricingResolver_CaseInsensitiveProvider(t *testing.T) {
	r := NewCatalogPricingResolver(resolverTestCatalog())

	rates, ok := r.Resolve("OpenAI", "gpt-4.1-mini")
	if !ok {
		t.Fatal("expected ok=true for case-insensitive provider lookup")
	}
	if rates.Rates.InputPer1MTokensUSD != 0.40 {
		t.Errorf("input = %f, want 0.40", rates.Rates.InputPer1MTokensUSD)
	}
}

func TestCatalogPricingResolver_NilPricing(t *testing.T) {
	cat := &Catalog{
		CatalogVersion: "1.0.0",
		Providers: map[string]ProviderEntry{
			"test": {
				BaseURL:   "https://test.api/v1",
				APIKeyEnv: "TEST_KEY",
				Models: map[string]Model{
					"free-model": {
						ContextWindow: 8192,
					},
				},
			},
		},
	}
	r := NewCatalogPricingResolver(cat)

	rates, ok := r.Resolve("test", "free-model")
	if !ok {
		t.Fatal("expected ok=true for model with nil pricing")
	}
	if rates.Rates.InputPer1MTokensUSD != 0 {
		t.Errorf("input = %f, want 0", rates.Rates.InputPer1MTokensUSD)
	}
}

// Verify CatalogPricingResolver satisfies the pricing.Resolver interface.
var _ pricing.Resolver = (*CatalogPricingResolver)(nil)
