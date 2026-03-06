package pricing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCatalogAndResolve(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "pricing.json")
	if err := os.WriteFile(path, []byte(`{
  "pricing_version": "2026-03-05",
  "providers": {
    "openai": {
      "aliases": {"gpt-5": "gpt-5-nano"},
      "models": {
        "gpt-5-nano": {
          "input_per_1m_tokens_usd": 1.25,
          "output_per_1m_tokens_usd": 3.75,
          "cache_read_per_1m_tokens_usd": 0.25
        }
      }
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write catalog: %v", err)
	}

	resolver, err := NewFileResolver(path)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	out, ok := resolver.Resolve("openai", "gpt-5")
	if !ok {
		t.Fatalf("expected model resolution")
	}
	if out.Model != "gpt-5-nano" {
		t.Fatalf("expected aliased model, got %q", out.Model)
	}
	if out.PricingVersion != "2026-03-05" {
		t.Fatalf("unexpected pricing version: %q", out.PricingVersion)
	}
	if out.Rates.InputPer1MTokensUSD != 1.25 || out.Rates.OutputPer1MTokensUSD != 3.75 {
		t.Fatalf("unexpected rates: %+v", out.Rates)
	}
}

func TestResolveUnknownProviderOrModel(t *testing.T) {
	t.Parallel()

	resolver := NewResolverFromCatalog(&Catalog{
		PricingVersion: "v1",
		Providers: map[string]ProviderCatalog{
			"openai": {
				Models: map[string]Rates{
					"gpt-4.1-mini": {
						InputPer1MTokensUSD:  0.15,
						OutputPer1MTokensUSD: 0.60,
					},
				},
			},
		},
	})

	if _, ok := resolver.Resolve("missing", "gpt-4.1-mini"); ok {
		t.Fatalf("expected unknown provider to fail")
	}
	if _, ok := resolver.Resolve("openai", "missing-model"); ok {
		t.Fatalf("expected unknown model to fail")
	}
}

func TestLoadCatalogValidation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "bad.json")
	if err := os.WriteFile(path, []byte(`{"pricing_version":"x","providers":{}}`), 0o644); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
	if _, err := NewFileResolver(path); err == nil {
		t.Fatalf("expected validation error")
	}
	if _, err := NewFileResolver(filepath.Join(root, "missing.json")); err == nil {
		t.Fatalf("expected missing file error")
	}
}
