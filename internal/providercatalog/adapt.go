package providercatalog

import (
	"strings"

	"go-agent-harness/internal/provider/catalog"
	"go-agent-harness/internal/provider/pricing"
)

// The harness already consumes two shapes: catalog.Catalog for endpoints and
// model metadata, and pricing.Catalog for rates. Rather than rewrite every
// consumer of those, the per-provider files become the source of truth and
// these adaptors derive both views from them. That keeps the blast radius to
// this file and makes the drift between the two catalogs impossible, because
// they now come from the same input.

// ToModelCatalog renders the provider files as the model catalog the provider
// registry reads.
func (c *Catalog) ToModelCatalog() *catalog.Catalog {
	out := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{}}
	if c == nil {
		return out
	}
	for _, id := range c.IDs() {
		p := c.Providers[id]
		// A provider nobody can call would appear in the picker and fail on
		// use, so it is described in the files but not offered here.
		if !p.Usable() {
			continue
		}
		entry := catalog.ProviderEntry{
			DisplayName:         p.DisplayName,
			BaseURL:             p.BaseURL,
			Protocol:            p.Protocol,
			APIKeyEnv:           p.Auth.Env,
			APIKeyOptional:      p.Auth.Kind == AuthNone,
			TokenSourceRequired: p.Auth.Kind == AuthSubscription,
			Models:              map[string]catalog.Model{},
		}
		usable := p.UsableRates()
		for modelID, m := range p.Pricing.Models {
			model := catalog.Model{
				DisplayName:     firstNonEmpty(m.DisplayName, modelID),
				ContextWindow:   m.ContextWindow,
				MaxOutputTokens: m.MaxOutput,
				Modalities:      m.Modalities,
			}
			// The catalog's pricing fields are named USD and its consumers add
			// them up, so a rate in another currency — or one billed per
			// request — is carried as no rate rather than a wrong one.
			if usable {
				model.Pricing = modelPricing(m)
			}
			entry.Models[modelID] = model
		}
		out.Providers[id] = entry
	}
	return out
}

// modelPricing converts to the catalog's pricing shape, which uses plain
// numbers. A model whose price is unknown gets no pricing block at all rather
// than a zero, because a zero renders as free.
func modelPricing(m Model) *catalog.ModelPricing {
	if !m.HasPrice() {
		return nil
	}
	p := &catalog.ModelPricing{
		InputPer1MTokensUSD:  *m.Input,
		OutputPer1MTokensUSD: *m.Output,
	}
	if m.CachedInput != nil {
		p.CacheReadPer1MTokensUSD = *m.CachedInput
	}
	if m.CacheWrite != nil {
		p.CacheWritePer1MTokensUSD = *m.CacheWrite
	}
	return p
}

// ToPricingCatalog renders the provider files as the rate card the cost
// resolver reads.
//
// Models with an unknown price are omitted entirely. The resolver reports "no
// rate found" for them, which is the truth; including them at zero would have
// it confidently report a free call.
func (c *Catalog) ToPricingCatalog() *pricing.Catalog {
	out := &pricing.Catalog{Providers: map[string]pricing.ProviderCatalog{}}
	if c == nil {
		return out
	}
	var versions []string
	for _, id := range c.IDs() {
		p := c.Providers[id]
		if !p.UsableRates() {
			continue
		}
		models := map[string]pricing.Rates{}
		for modelID, m := range p.Pricing.Models {
			if !m.HasPrice() {
				continue
			}
			rates := pricing.Rates{
				InputPer1MTokensUSD:  *m.Input,
				OutputPer1MTokensUSD: *m.Output,
			}
			if m.CachedInput != nil {
				rates.CacheReadPer1MTokensUSD = *m.CachedInput
			}
			if m.CacheWrite != nil {
				rates.CacheWritePer1MTokensUSD = *m.CacheWrite
			}
			models[modelID] = rates
		}
		if len(models) == 0 {
			continue
		}
		out.Providers[id] = pricing.ProviderCatalog{Models: models}
		if p.Pricing.AsOf != "" {
			versions = append(versions, id+"@"+p.Pricing.AsOf)
		}
	}
	// The version string records when each provider's rates were read, so a
	// cost report can be traced back to the rate card that produced it.
	out.PricingVersion = strings.Join(versions, " ")
	return out
}

// NonUSDProviders lists providers whose rates are quoted in another currency
// and are therefore absent from the USD pricing catalog. Callers surface this
// so the omission is visible rather than looking like missing data.
func (c *Catalog) NonUSDProviders() []string {
	if c == nil {
		return nil
	}
	var out []string
	for _, id := range c.IDs() {
		p := c.Providers[id]
		if len(p.Pricing.Models) == 0 {
			continue
		}
		if !p.UsableRates() {
			out = append(out, id)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// MergeInto folds the provider files into an existing model catalog.
//
// Deliberately additive. The bundled catalog carries hand-curated detail the
// provider files do not model — aliases, quirks, models_from mirrors — and a
// wholesale replacement would silently drop it. So a provider present in both
// keeps everything it already had; the files only add providers that are
// missing, add models that are missing, and fill in fields the catalog left
// blank. Curation wins every conflict.
//
// Returns the ids of providers added and models added, for logging: a merge
// that quietly does nothing looks identical to one that worked.
func (c *Catalog) MergeInto(cat *catalog.Catalog) (addedProviders, addedModels []string) {
	if c == nil || cat == nil {
		return nil, nil
	}
	if cat.Providers == nil {
		cat.Providers = map[string]catalog.ProviderEntry{}
	}
	for _, id := range c.IDs() {
		p := c.Providers[id]
		if !p.Usable() {
			continue
		}
		entry, exists := cat.Providers[id]
		if !exists {
			entry = catalog.ProviderEntry{Models: map[string]catalog.Model{}}
			addedProviders = append(addedProviders, id)
		}
		if entry.Models == nil {
			entry.Models = map[string]catalog.Model{}
		}
		// Only fill blanks — never overwrite a configured value.
		entry.DisplayName = firstNonEmpty(entry.DisplayName, p.DisplayName)
		entry.BaseURL = firstNonEmpty(entry.BaseURL, p.BaseURL)
		entry.Protocol = firstNonEmpty(entry.Protocol, p.Protocol)
		entry.APIKeyEnv = firstNonEmpty(entry.APIKeyEnv, p.Auth.Env)
		// Auth flags are set only for a provider the files introduced. On one
		// the catalog already curated, flipping these would change how the
		// harness authenticates an existing provider — the opposite of
		// additive, and a curated false is indistinguishable from an unset one.
		if !exists {
			entry.APIKeyOptional = p.Auth.Kind == AuthNone
			entry.TokenSourceRequired = p.Auth.Kind == AuthSubscription
		}

		for modelID, m := range p.Pricing.Models {
			existing, had := entry.Models[modelID]
			if !had {
				added := catalog.Model{
					DisplayName:     firstNonEmpty(m.DisplayName, modelID),
					ContextWindow:   m.ContextWindow,
					MaxOutputTokens: m.MaxOutput,
					Modalities:      m.Modalities,
				}
				if p.UsableRates() {
					added.Pricing = modelPricing(m)
				}
				entry.Models[modelID] = added
				addedModels = append(addedModels, id+"/"+modelID)
				continue
			}
			// A model the catalog already knows keeps its metadata; the file
			// supplies only what was missing. Pricing in particular: a curated
			// rate was chosen deliberately and outranks a scraped one.
			if existing.Pricing == nil && p.UsableRates() {
				existing.Pricing = modelPricing(m)
			}
			if existing.ContextWindow == 0 {
				existing.ContextWindow = m.ContextWindow
			}
			if existing.MaxOutputTokens == 0 {
				existing.MaxOutputTokens = m.MaxOutput
			}
			if len(existing.Modalities) == 0 {
				existing.Modalities = m.Modalities
			}
			entry.Models[modelID] = existing
		}
		cat.Providers[id] = entry
	}
	return addedProviders, addedModels
}
