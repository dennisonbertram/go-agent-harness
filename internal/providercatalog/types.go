// Package providercatalog is the single source of truth for what the harness
// knows about a model provider.
//
// Before this, the same provider was described in two places that drifted:
// catalog/models.json held endpoints and model metadata, catalog/pricing.json
// held rates, and neither recorded where a price came from or when it was last
// checked. A price with no date is a number nobody can trust — vendors change
// them, and a stale figure is worse than a missing one because it looks
// authoritative.
//
// One file per provider under catalog/providers/ now carries everything about
// that provider: how to reach it, how to authenticate, what it can report about
// itself, and what it charges as of a stated date. The two older catalogs are
// derived from these files by the adaptors in adapt.go, so existing consumers
// keep working unchanged.
package providercatalog

import (
	"strings"
	"time"
)

// Wire protocols a provider can speak.
const (
	ProtocolOpenAICompat = "openai_compat"
	ProtocolAnthropic    = "anthropic"
	ProtocolGemini       = "gemini"
)

// How a provider's credential is supplied.
const (
	AuthAPIKey       = "api_key"
	AuthSubscription = "subscription"
	AuthNone         = "none"
)

// How the credential is presented on the wire.
const (
	SchemeBearer     = "bearer"
	SchemeAPIKey     = "x-api-key"
	SchemeQueryParam = "query-param"
)

// UnitPer1MTokens is the only rate unit the harness understands. Anything
// else would be silently misread by a factor of a thousand or more, so a file
// declaring a different unit is rejected rather than guessed at.
const UnitPer1MTokens = "per_1m_tokens"

// Whether a provider can actually be used today.
//
// Not every interesting provider sells self-serve access. Recording that
// plainly is more useful than omitting the provider, because "we looked and
// there is no public API" is an answer, and without it the same research gets
// repeated.
const (
	AvailabilityGA         = "generally_available"
	AvailabilityWaitlist   = "waitlist"
	AvailabilityEnterprise = "enterprise_only"
	AvailabilityNoAPI      = "no_public_api"
)

// Provider is one provider, as described by one file.
type Provider struct {
	// ID comes from the filename, not the body, so a file cannot disagree
	// with itself about which provider it describes.
	ID           string       `json:"-"`
	DisplayName  string       `json:"display_name"`
	BaseURL      string       `json:"base_url"`
	Protocol     string       `json:"protocol"`
	Auth         Auth         `json:"auth"`
	Capabilities Capabilities `json:"capabilities"`
	Pricing      Pricing      `json:"pricing"`
	// Availability defaults to generally_available when a file omits it,
	// since that is true of every provider anyone bothers to configure.
	Availability string `json:"availability,omitempty"`
	// Notes carry anything the schema cannot express — off-peak discounts,
	// deprecation dates, which regional site a price was read from. Prose is
	// the right shape for facts that exist in one provider and no other.
	Notes []string `json:"notes,omitempty"`
}

// Auth describes how to authenticate, never the credential itself. Secrets
// live in the environment, the keychain, or a file; this only names which.
type Auth struct {
	Kind         string            `json:"kind"`
	Env          string            `json:"env,omitempty"`
	Scheme       string            `json:"scheme,omitempty"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
}

// Capabilities records what a provider's API can tell us about itself.
//
// These are observed facts, not aspirations, and they decide real behaviour:
// whether a fetch can discover models, whether cost can be computed from a
// response, and whether quota can be read without waiting for a 429.
type Capabilities struct {
	// ModelsEndpoint is the path that lists models, relative to BaseURL.
	// Empty means the provider publishes no list and the curated models below
	// are all we will ever have.
	ModelsEndpoint string `json:"models_endpoint,omitempty"`
	// PricingInModelsEndpoint is true for the rare provider that returns
	// rates with its model list — OpenRouter does. For those, curated prices
	// are a fallback and the live values win.
	PricingInModelsEndpoint bool `json:"pricing_in_models_endpoint"`
	// UsageInCompletion is true when a completion response carries token
	// counts. Without it, per-call cost cannot be computed at all.
	UsageInCompletion bool     `json:"usage_in_completion"`
	UsageFields       []string `json:"usage_fields,omitempty"`
	// HasCachedInputPricing marks providers that bill cached input at a
	// different rate. Ignoring it overstates cost badly — an order of
	// magnitude on some models — so it must be recorded per provider.
	HasCachedInputPricing bool `json:"has_cached_input_pricing"`
	// AccountUsageEndpoint is a path returning balance or cumulative spend.
	// Most providers have none and expose this only in a web console.
	AccountUsageEndpoint string `json:"account_usage_endpoint,omitempty"`
	// RateLimitHeaders are the response headers carrying remaining quota.
	// Some providers send these without documenting them; recording the
	// observed names is what makes them usable.
	RateLimitHeaders []string `json:"rate_limit_headers,omitempty"`
}

// Pricing is a provider's rate card together with its provenance.
//
// AsOf and SourceURL are required whenever any price is present. A rate with
// no date and no source cannot be checked by the next person to read it, and
// silently goes stale.
type Pricing struct {
	AsOf      string `json:"as_of"`
	SourceURL string `json:"source_url"`
	// Currency is whatever the cited page quoted. Vendors publish their own
	// CNY and USD schedules that are not conversions of each other, so a
	// converted figure would be a number no vendor ever charged.
	Currency string `json:"currency"`
	Unit     string `json:"unit"`
	// PerRequest marks a provider that bills by request rather than by token,
	// which no token rate can represent.
	PerRequest bool             `json:"per_request,omitempty"`
	Models     map[string]Model `json:"models,omitempty"`
}

// Model is one model's rates and limits.
//
// Every price is a pointer because "unknown" and "free" are different facts.
// A plain float64 renders an unknown price as $0, which reads as free and is
// exactly the mistake this type exists to prevent.
type Model struct {
	Input       *float64 `json:"input"`
	Output      *float64 `json:"output"`
	CachedInput *float64 `json:"cached_input,omitempty"`
	// CacheWrite is charged by providers that bill for populating a cache as
	// well as reading from it.
	CacheWrite    *float64 `json:"cache_write,omitempty"`
	ContextWindow int      `json:"context_window,omitempty"`
	MaxOutput     int      `json:"max_output,omitempty"`
	Modalities    []string `json:"modalities,omitempty"`
	DisplayName   string   `json:"display_name,omitempty"`
}

// HasPrice reports whether both sides of the rate are known. A model priced on
// one side only cannot produce a total, so it counts as unpriced.
func (m Model) HasPrice() bool { return m.Input != nil && m.Output != nil }

// UsableRates reports whether this provider's rates can be converted into the
// harness's USD-per-million-tokens totals at all.
//
// A non-USD rate must never be added to a USD total — vendors publish separate
// CNY and USD schedules that are not conversions of each other, so the sum
// would be money in no currency. A per-request price cannot be expressed as a
// token rate either.
func (p Provider) UsableRates() bool {
	return strings.EqualFold(p.Pricing.Currency, "USD") && !p.Pricing.PerRequest
}

// Catalog is every provider file, loaded.
type Catalog struct {
	Providers map[string]Provider
}

// PricedAt returns when a provider's rates were last checked, and whether the
// file said at all.
func (p Provider) PricedAt() (time.Time, bool) {
	if p.Pricing.AsOf == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", p.Pricing.AsOf)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// PriceAge reports how long ago the rates were checked. Callers use it to warn
// before anyone bases a decision on a figure nobody has looked at in months.
func (p Provider) PriceAge(now time.Time) (time.Duration, bool) {
	at, ok := p.PricedAt()
	if !ok {
		return 0, false
	}
	return now.Sub(at), true
}

// Usable reports whether a provider can actually be configured and called.
func (p Provider) Usable() bool {
	switch p.Availability {
	case "", AvailabilityGA:
		return p.BaseURL != ""
	default:
		return false
	}
}
