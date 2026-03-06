package pricing

import "strings"

type FileResolver struct {
	catalog *Catalog
}

func NewFileResolver(path string) (*FileResolver, error) {
	catalog, err := LoadCatalog(path)
	if err != nil {
		return nil, err
	}
	return &FileResolver{catalog: catalog}, nil
}

func NewResolverFromCatalog(catalog *Catalog) *FileResolver {
	return &FileResolver{catalog: catalog}
}

func (r *FileResolver) Resolve(provider, model string) (ResolvedRates, bool) {
	if r == nil || r.catalog == nil {
		return ResolvedRates{}, false
	}
	providerKey := strings.ToLower(strings.TrimSpace(provider))
	if providerKey == "" {
		return ResolvedRates{}, false
	}
	modelKey := strings.TrimSpace(model)
	if modelKey == "" {
		return ResolvedRates{}, false
	}

	p, ok := findProvider(r.catalog.Providers, providerKey)
	if !ok {
		return ResolvedRates{}, false
	}
	canonicalModel := resolveAlias(p.Aliases, modelKey)
	rates, ok := p.Models[canonicalModel]
	if !ok {
		return ResolvedRates{}, false
	}
	return ResolvedRates{
		Provider:       providerKey,
		Model:          canonicalModel,
		PricingVersion: strings.TrimSpace(r.catalog.PricingVersion),
		Rates:          rates,
	}, true
}

func findProvider(providers map[string]ProviderCatalog, provider string) (ProviderCatalog, bool) {
	if p, ok := providers[provider]; ok {
		return p, true
	}
	for key, p := range providers {
		if strings.EqualFold(strings.TrimSpace(key), provider) {
			return p, true
		}
	}
	return ProviderCatalog{}, false
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
