package catalog

import (
	"go-agent-harness/internal/provider/pricing"
	"strings"
)

// CatalogPricingResolver implements pricing.Resolver using the model catalog.
type CatalogPricingResolver struct {
	catalog *Catalog
}

// NewCatalogPricingResolver creates a resolver backed by the given catalog.
func NewCatalogPricingResolver(cat *Catalog) *CatalogPricingResolver {
	return &CatalogPricingResolver{catalog: cat}
}

// Resolve looks up pricing for a provider/model pair from the catalog.
// It supports alias resolution and returns zero costs if not found.
func (r *CatalogPricingResolver) Resolve(provider, model string) (pricing.ResolvedRates, bool) {
	if r == nil || r.catalog == nil {
		return pricing.ResolvedRates{}, false
	}

	providerKey := strings.ToLower(strings.TrimSpace(provider))
	if providerKey == "" {
		return pricing.ResolvedRates{}, false
	}

	modelKey := strings.TrimSpace(model)
	if modelKey == "" {
		return pricing.ResolvedRates{}, false
	}

	p, ok := findProvider(r.catalog.Providers, providerKey)
	if !ok {
		return pricing.ResolvedRates{}, false
	}

	canonicalModel := resolveAlias(p.Aliases, modelKey)
	m, ok := p.Models[canonicalModel]
	if !ok {
		return pricing.ResolvedRates{}, false
	}

	if m.Pricing == nil {
		return pricing.ResolvedRates{
			Provider:       providerKey,
			Model:          canonicalModel,
			PricingVersion: r.catalog.CatalogVersion,
			Rates:          pricing.Rates{},
		}, true
	}

	return pricing.ResolvedRates{
		Provider:       providerKey,
		Model:          canonicalModel,
		PricingVersion: r.catalog.CatalogVersion,
		Rates: pricing.Rates{
			InputPer1MTokensUSD:      m.Pricing.InputPer1MTokensUSD,
			OutputPer1MTokensUSD:     m.Pricing.OutputPer1MTokensUSD,
			CacheReadPer1MTokensUSD:  m.Pricing.CacheReadPer1MTokensUSD,
			CacheWritePer1MTokensUSD: m.Pricing.CacheWritePer1MTokensUSD,
		},
	}, true
}

func findProvider(providers map[string]ProviderEntry, provider string) (ProviderEntry, bool) {
	if p, ok := providers[provider]; ok {
		return p, true
	}
	for key, p := range providers {
		if strings.EqualFold(strings.TrimSpace(key), provider) {
			return p, true
		}
	}
	return ProviderEntry{}, false
}

func resolveAlias(aliases map[string]string, model string) string {
	if len(aliases) == 0 {
		return model
	}
	current := model
	seen := map[string]struct{}{}
	for i := 0; i < 8; i++ {
		key := strings.TrimSpace(current)
		if key == "" {
			break
		}
		if _, exists := seen[key]; exists {
			break
		}
		seen[key] = struct{}{}
		next, ok := aliases[key]
		if !ok || strings.TrimSpace(next) == "" {
			break
		}
		current = strings.TrimSpace(next)
	}
	return current
}
