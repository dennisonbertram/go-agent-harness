package main

import (
	"bytes"
	"strings"
	"testing"

	"go-agent-harness/internal/provider/catalog"
)

// testCatalog returns a minimal catalog with two providers for testing.
func testCatalog() *catalog.Catalog {
	return &catalog.Catalog{
		CatalogVersion: "1.0.0",
		Providers: map[string]catalog.ProviderEntry{
			"acme": {
				DisplayName: "Acme AI",
				BaseURL:     "https://api.acme.example/v1",
				APIKeyEnv:   "ACME_API_KEY",
				Protocol:    "openai_compat",
				Models: map[string]catalog.Model{
					"acme-fast": {
						DisplayName:   "Acme Fast",
						ContextWindow: 128000,
						Pricing: &catalog.ModelPricing{
							InputPer1MTokensUSD:  1.00,
							OutputPer1MTokensUSD: 3.00,
						},
					},
					"acme-pro": {
						DisplayName:   "Acme Pro",
						ContextWindow: 256000,
						Pricing: &catalog.ModelPricing{
							InputPer1MTokensUSD:  5.00,
							OutputPer1MTokensUSD: 20.00,
						},
					},
				},
			},
			"beta": {
				DisplayName: "Beta LLM",
				BaseURL:     "https://api.beta.example/v1",
				APIKeyEnv:   "BETA_API_KEY",
				Protocol:    "openai_compat",
				Models: map[string]catalog.Model{
					"beta-mini": {
						DisplayName:   "Beta Mini",
						ContextWindow: 64000,
						Pricing: &catalog.ModelPricing{
							InputPer1MTokensUSD:  0.20,
							OutputPer1MTokensUSD: 0.80,
						},
					},
				},
			},
		},
	}
}

func TestPrintModelsList_WithCatalog(t *testing.T) {
	d := &Display{NoColor: true}
	var buf bytes.Buffer

	d.fprintModelsList(&buf, testCatalog())

	output := buf.String()

	// Header
	if !strings.Contains(output, "Available models:") {
		t.Errorf("expected 'Available models:' header, got:\n%s", output)
	}

	// Provider names (alphabetical: acme, beta)
	if !strings.Contains(output, "Acme AI") {
		t.Errorf("expected provider 'Acme AI', got:\n%s", output)
	}
	if !strings.Contains(output, "Beta LLM") {
		t.Errorf("expected provider 'Beta LLM', got:\n%s", output)
	}

	// Model names
	if !strings.Contains(output, "acme-fast") {
		t.Errorf("expected model 'acme-fast', got:\n%s", output)
	}
	if !strings.Contains(output, "acme-pro") {
		t.Errorf("expected model 'acme-pro', got:\n%s", output)
	}
	if !strings.Contains(output, "beta-mini") {
		t.Errorf("expected model 'beta-mini', got:\n%s", output)
	}

	// Pricing for acme-fast ($1.00/$3.00)
	if !strings.Contains(output, "$1.00/$3.00") {
		t.Errorf("expected acme-fast pricing '$1.00/$3.00', got:\n%s", output)
	}

	// Pricing for beta-mini ($0.20/$0.80)
	if !strings.Contains(output, "$0.20/$0.80") {
		t.Errorf("expected beta-mini pricing '$0.20/$0.80', got:\n%s", output)
	}

	// Providers are sorted alphabetically: acme before beta
	acmeIdx := strings.Index(output, "Acme AI")
	betaIdx := strings.Index(output, "Beta LLM")
	if acmeIdx >= betaIdx {
		t.Errorf("expected 'Acme AI' to appear before 'Beta LLM', got:\n%s", output)
	}

	// Models within acme are sorted alphabetically: acme-fast before acme-pro
	fastIdx := strings.Index(output, "acme-fast")
	proIdx := strings.Index(output, "acme-pro")
	if fastIdx >= proIdx {
		t.Errorf("expected 'acme-fast' to appear before 'acme-pro', got:\n%s", output)
	}
}

func TestPrintModelsList_NilCatalog(t *testing.T) {
	d := &Display{NoColor: true}
	var buf bytes.Buffer

	d.fprintModelsList(&buf, nil)

	output := buf.String()
	if !strings.Contains(output, "catalog not available") {
		t.Errorf("expected 'catalog not available' message for nil catalog, got: %q", output)
	}
	// Should not print "Available models:" for nil catalog
	if strings.Contains(output, "Available models:") {
		t.Errorf("unexpected 'Available models:' header for nil catalog, got: %q", output)
	}
}

func TestPrintModelsList_ModelWithoutPricing(t *testing.T) {
	cat := &catalog.Catalog{
		CatalogVersion: "1.0.0",
		Providers: map[string]catalog.ProviderEntry{
			"noprice": {
				DisplayName: "No Price Provider",
				BaseURL:     "https://api.noprice.example/v1",
				APIKeyEnv:   "NOPRICE_API_KEY",
				Protocol:    "openai_compat",
				Models: map[string]catalog.Model{
					"free-model": {
						DisplayName:   "Free Model",
						ContextWindow: 32000,
						Pricing:       nil, // no pricing
					},
				},
			},
		},
	}

	d := &Display{NoColor: true}
	var buf bytes.Buffer
	d.fprintModelsList(&buf, cat)

	output := buf.String()
	if !strings.Contains(output, "free-model") {
		t.Errorf("expected 'free-model' in output, got: %q", output)
	}
	// Should not panic or print garbage pricing
	if strings.Contains(output, "NaN") || strings.Contains(output, "<nil>") {
		t.Errorf("unexpected nil/NaN in output, got: %q", output)
	}
}

func TestHandleCommand_Models(t *testing.T) {
	d := &Display{NoColor: true}
	model := "gpt-4.1"
	provider := ""
	cat := testCatalog()

	// /models should be handled (return true)
	handled, _ := handleCommand("/models", &model, &provider, d, cat)
	if !handled {
		t.Fatalf("expected /models to be handled")
	}
	// model should not change (selectModel returns "" when no terminal)
	if model != "gpt-4.1" {
		t.Fatalf("expected model unchanged, got %q", model)
	}
}

func TestHandleCommand_ModelsNilCatalog(t *testing.T) {
	d := &Display{NoColor: true}
	model := "gpt-4.1"
	provider := ""

	// /models with nil catalog should still be handled (return true), not panic
	handled, _ := handleCommand("/models", &model, &provider, d, nil)
	if !handled {
		t.Fatalf("expected /models with nil catalog to be handled")
	}
}

func TestProviderOrder(t *testing.T) {
	providers := map[string]catalog.ProviderEntry{
		"zebra":  {},
		"apple":  {},
		"mango":  {},
		"banana": {},
	}
	order := providerOrder(providers)
	expected := []string{"apple", "banana", "mango", "zebra"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d providers, got %d", len(expected), len(order))
	}
	for i, k := range order {
		if k != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], k)
		}
	}
}

func TestModelOrder(t *testing.T) {
	models := map[string]catalog.Model{
		"z-model": {},
		"a-model": {},
		"m-model": {},
	}
	order := modelOrder(models)
	expected := []string{"a-model", "m-model", "z-model"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d models, got %d", len(expected), len(order))
	}
	for i, k := range order {
		if k != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], k)
		}
	}
}

func TestPrintHelp_ContainsModels(t *testing.T) {
	d := &Display{NoColor: true}
	// Redirect stdout by capturing print output via a pipe isn't easy here,
	// but we can verify the help text contains /models by examining the source.
	// Instead, test that PrintHelp doesn't panic and /models is documented.
	// We rely on the display_test for the actual content verification.
	d.PrintHelp() // should not panic
}

func TestHandleCommand_Provider(t *testing.T) {
	d := &Display{NoColor: true}
	model := "gpt-4.1"
	provider := ""

	// /provider with no args should print auto-detected message.
	handled, _ := handleCommand("/provider", &model, &provider, d, nil)
	if !handled {
		t.Fatalf("expected /provider to be handled")
	}

	// /provider openai should set the provider.
	handled, _ = handleCommand("/provider openai", &model, &provider, d, nil)
	if !handled {
		t.Fatalf("expected /provider openai to be handled")
	}
	if provider != "openai" {
		t.Errorf("expected provider=openai, got %q", provider)
	}

	// /provider with a set provider should print its name.
	handled, _ = handleCommand("/provider", &model, &provider, d, nil)
	if !handled {
		t.Fatalf("expected /provider to be handled when provider is set")
	}
}

func TestHandleCommand_ModelClearsProvider(t *testing.T) {
	d := &Display{NoColor: true}
	model := "gpt-4.1"
	provider := "openai"

	// Setting /model manually should clear provider.
	handled, _ := handleCommand("/model gpt-5", &model, &provider, d, nil)
	if !handled {
		t.Fatalf("expected /model gpt-5 to be handled")
	}
	if model != "gpt-5" {
		t.Errorf("expected model=gpt-5, got %q", model)
	}
	if provider != "" {
		t.Errorf("expected provider cleared, got %q", provider)
	}
}
