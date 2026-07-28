package providercatalog

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// absoluteHTTPURL requires an absolute http(s) URL and returns it trimmed. A
// provider file that loads with a malformed URL fails silently much later —
// at request construction, or when a human clicks a source link that doesn't
// resolve — instead of at the one place a typo is cheap to catch.
func absoluteHTTPURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	u, err := url.Parse(trimmed)
	if err != nil || !u.IsAbs() || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("must be an absolute http(s) URL")
	}
	return trimmed, nil
}

// Load reads every *.json file in dir as one provider.
//
// A malformed file fails the whole load rather than being skipped. A provider
// that silently vanishes because of a typo is far worse than a startup error:
// the picker just stops offering its models and nothing says why.
func Load(dir string) (*Catalog, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read provider catalog directory %s: %w", dir, err)
	}

	cat := &Catalog{Providers: map[string]Provider{}}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		p, err := LoadFile(path)
		if err != nil {
			return nil, err
		}
		if _, clash := cat.Providers[p.ID]; clash {
			return nil, fmt.Errorf("two provider files describe %q", p.ID)
		}
		cat.Providers[p.ID] = p
	}
	if len(cat.Providers) == 0 {
		return nil, fmt.Errorf("no provider files found in %s", dir)
	}
	return cat, nil
}

// LoadFS is Load against an fs.FS, so the catalog can be embedded in the
// binary as well as read from disk.
func LoadFS(fsys fs.FS, dir string) (*Catalog, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read provider catalog directory %s: %w", dir, err)
	}
	cat := &Catalog{Providers: map[string]Provider{}}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		// fs.FS paths are always slash-separated, whatever the host OS.
		data, err := fs.ReadFile(fsys, path.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		p, err := parseProvider(idFromFilename(entry.Name()), data)
		if err != nil {
			return nil, err
		}
		if _, clash := cat.Providers[p.ID]; clash {
			return nil, fmt.Errorf("two provider files describe %q", p.ID)
		}
		cat.Providers[p.ID] = p
	}
	if len(cat.Providers) == 0 {
		return nil, fmt.Errorf("no provider files found in %s", dir)
	}
	return cat, nil
}

// LoadFile reads one provider file. The id comes from the filename.
func LoadFile(path string) (Provider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Provider{}, fmt.Errorf("read %s: %w", path, err)
	}
	return parseProvider(idFromFilename(filepath.Base(path)), data)
}

func idFromFilename(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func parseProvider(id string, data []byte) (Provider, error) {
	var p Provider
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	// An unknown field is nearly always a typo or a schema drift, and silently
	// dropping it loses the very information the file exists to carry.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return Provider{}, fmt.Errorf("parse provider %q: %w", id, err)
	}
	// Trailing content means the file is not what it appears to be — a second
	// object, or a truncated edit — and silently ignoring it would serve half
	// a file as though it were whole.
	if decoder.More() {
		return Provider{}, fmt.Errorf("parse provider %q: unexpected trailing content", id)
	}
	p.ID = id
	if err := Validate(&p); err != nil {
		return Provider{}, err
	}
	return p, nil
}

// Validate checks and normalises a provider, reporting the first problem.
//
// This runs at load, so a bad file stops the daemon at startup with a specific
// message rather than producing a provider that fails mysteriously later.
func Validate(p *Provider) error {
	p.ID = strings.TrimSpace(p.ID)
	if p.ID == "" {
		return fmt.Errorf("provider id is empty (it comes from the filename)")
	}
	if p.DisplayName == "" {
		p.DisplayName = p.ID
	}
	if p.Availability == "" {
		p.Availability = AvailabilityGA
	}
	switch p.Availability {
	case AvailabilityGA, AvailabilityWaitlist, AvailabilityEnterprise, AvailabilityNoAPI:
	default:
		return fmt.Errorf("provider %q: unknown availability %q", p.ID, p.Availability)
	}

	// A provider with no public API legitimately has no endpoint, protocol or
	// auth. Requiring them would force placeholder values that read as real.
	if p.Availability != AvailabilityNoAPI {
		if strings.TrimSpace(p.BaseURL) == "" {
			return fmt.Errorf("provider %q: base_url is required unless availability is %q",
				p.ID, AvailabilityNoAPI)
		}
		// A malformed base_url loads fine and only fails once something tries
		// to build a request against it — parse it now so the error names the
		// provider and the bad value instead of surfacing as a mystery dial failure.
		base, err := absoluteHTTPURL(p.BaseURL)
		if err != nil {
			return fmt.Errorf("provider %q: base_url %q %w", p.ID, p.BaseURL, err)
		}
		p.BaseURL = base
		switch p.Protocol {
		case ProtocolOpenAICompat, ProtocolAnthropic, ProtocolGemini:
		default:
			return fmt.Errorf("provider %q: unknown protocol %q (want %q, %q or %q)",
				p.ID, p.Protocol, ProtocolOpenAICompat, ProtocolAnthropic, ProtocolGemini)
		}
		switch p.Auth.Kind {
		case AuthAPIKey, AuthSubscription, AuthNone:
		case "":
			return fmt.Errorf("provider %q: auth.kind is required", p.ID)
		default:
			return fmt.Errorf("provider %q: unknown auth kind %q", p.ID, p.Auth.Kind)
		}
		switch p.Auth.Scheme {
		case SchemeBearer, SchemeAPIKey, SchemeQueryParam, "":
		default:
			return fmt.Errorf("provider %q: unknown auth scheme %q", p.ID, p.Auth.Scheme)
		}
		if p.Auth.Env != "" {
			env := strings.TrimSpace(p.Auth.Env)
			// Whitespace surviving the trim can only be inside the name, and an
			// env var name with a space in it can never be what's actually set
			// in the shell — the registry would report "unconfigured" forever.
			if strings.ContainsFunc(env, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }) {
				return fmt.Errorf("provider %q: auth.env %q contains whitespace", p.ID, p.Auth.Env)
			}
			p.Auth.Env = env
		}
	}

	// Endpoint paths are joined onto base_url wherever they're used, so a
	// value missing its leading slash silently concatenates into the wrong
	// URL rather than failing loudly.
	p.Capabilities.ModelsEndpoint = strings.TrimSpace(p.Capabilities.ModelsEndpoint)
	if p.Capabilities.ModelsEndpoint != "" && !strings.HasPrefix(p.Capabilities.ModelsEndpoint, "/") {
		return fmt.Errorf("provider %q: capabilities.models_endpoint %q must start with \"/\"",
			p.ID, p.Capabilities.ModelsEndpoint)
	}
	p.Capabilities.AccountUsageEndpoint = strings.TrimSpace(p.Capabilities.AccountUsageEndpoint)
	if p.Capabilities.AccountUsageEndpoint != "" && !strings.HasPrefix(p.Capabilities.AccountUsageEndpoint, "/") {
		return fmt.Errorf("provider %q: capabilities.account_usage_endpoint %q must start with \"/\"",
			p.ID, p.Capabilities.AccountUsageEndpoint)
	}

	return validatePricing(p)
}

func validatePricing(p *Provider) error {
	priced := 0
	for id, m := range p.Pricing.Models {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			return fmt.Errorf("provider %q: a model has an empty id", p.ID)
		}
		// A padded id can never match what the provider actually returns, and
		// silently rewriting the map key would hide a file that needs fixing
		// at the source rather than papering over it at load time.
		if trimmed != id {
			return fmt.Errorf("provider %q: model id %q has leading/trailing whitespace", p.ID, id)
		}
		for label, v := range map[string]*float64{
			"input": m.Input, "output": m.Output,
			"cached_input": m.CachedInput, "cache_write": m.CacheWrite,
		} {
			if v == nil {
				continue
			}
			// NaN and ±Inf survive JSON round-tripping through Go floats in
			// some encoders and would poison every cost calculation downstream.
			if math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 {
				return fmt.Errorf("provider %q model %q: %s is not a usable price", p.ID, id, label)
			}
		}
		if m.HasPrice() {
			priced++
		}
	}

	// Provenance is only required once there is something to attribute. A
	// provider with no prices has nothing to go stale.
	if priced > 0 {
		// An omitted unit means the only one supported. A DIFFERENT one is
		// rejected: it would be read off by a factor of a thousand or more
		// with no visible symptom.
		if p.Pricing.Unit == "" {
			p.Pricing.Unit = UnitPer1MTokens
		}
		if p.Pricing.Unit != UnitPer1MTokens {
			return fmt.Errorf("provider %q: pricing.unit must be %q (got %q)",
				p.ID, UnitPer1MTokens, p.Pricing.Unit)
		}
		if strings.TrimSpace(p.Pricing.SourceURL) == "" {
			return fmt.Errorf("provider %q: pricing.source_url is required when prices are given", p.ID)
		}
		// The source URL is the provenance for every rate in the file; one
		// that can't even be parsed makes the rates unverifiable by anyone
		// who goes looking.
		source, err := absoluteHTTPURL(p.Pricing.SourceURL)
		if err != nil {
			return fmt.Errorf("provider %q: pricing.source_url %q %w", p.ID, p.Pricing.SourceURL, err)
		}
		p.Pricing.SourceURL = source
		if strings.TrimSpace(p.Pricing.Currency) == "" {
			return fmt.Errorf("provider %q: pricing.currency is required when prices are given", p.ID)
		}
		at, ok := p.PricedAt()
		if !ok {
			return fmt.Errorf(
				"provider %q: pricing.as_of must be a YYYY-MM-DD date when prices are given (got %q)",
				p.ID, p.Pricing.AsOf)
		}
		// A mistyped year in the future would mark the rates permanently fresh
		// and suppress every staleness warning for as long as the typo stands.
		if at.After(time.Now().AddDate(0, 0, 1)) {
			return fmt.Errorf("provider %q: pricing.as_of %q is in the future", p.ID, p.Pricing.AsOf)
		}
	}
	return nil
}

// IDs returns every provider id, sorted, so output and iteration are stable.
func (c *Catalog) IDs() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.Providers))
	for id := range c.Providers {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Get returns one provider.
func (c *Catalog) Get(id string) (Provider, bool) {
	if c == nil {
		return Provider{}, false
	}
	p, ok := c.Providers[id]
	return p, ok
}

// StaleProviders returns the providers whose prices have not been checked
// within maxAge, oldest first, so a caller can warn rather than quietly serve
// figures nobody has verified in months.
func (c *Catalog) StaleProviders(now time.Time, maxAge time.Duration) []string {
	if c == nil {
		return nil
	}
	type aged struct {
		id  string
		age time.Duration
	}
	var stale []aged
	for _, id := range c.IDs() {
		p := c.Providers[id]
		// A provider with nothing priced has no rate to go stale; warning
		// about it is noise that trains people to ignore the warning.
		if !hasAnyPrice(p) {
			continue
		}
		age, ok := p.PriceAge(now)
		if !ok || age <= maxAge {
			continue
		}
		stale = append(stale, aged{id: id, age: age})
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i].age > stale[j].age })
	out := make([]string, 0, len(stale))
	for _, s := range stale {
		out = append(out, s.id)
	}
	return out
}

func hasAnyPrice(p Provider) bool {
	for _, m := range p.Pricing.Models {
		if m.HasPrice() {
			return true
		}
	}
	return false
}
