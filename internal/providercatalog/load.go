package providercatalog

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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
		data, err := fs.ReadFile(fsys, filepath.Join(dir, entry.Name()))
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
	}

	return validatePricing(p)
}

func validatePricing(p *Provider) error {
	priced := 0
	for id, m := range p.Pricing.Models {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("provider %q: a model has an empty id", p.ID)
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
		if strings.TrimSpace(p.Pricing.SourceURL) == "" {
			return fmt.Errorf("provider %q: pricing.source_url is required when prices are given", p.ID)
		}
		if strings.TrimSpace(p.Pricing.Currency) == "" {
			return fmt.Errorf("provider %q: pricing.currency is required when prices are given", p.ID)
		}
		if _, ok := p.PricedAt(); !ok {
			return fmt.Errorf(
				"provider %q: pricing.as_of must be a YYYY-MM-DD date when prices are given (got %q)",
				p.ID, p.Pricing.AsOf)
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
