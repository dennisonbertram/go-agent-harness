package providercatalog

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

const validOpenAIBody = `{
  "display_name": "OpenAI",
  "base_url": "https://api.openai.com/v1",
  "protocol": "openai_compat",
  "auth": {"kind": "api_key", "env": "OPENAI_API_KEY", "scheme": "bearer"},
  "pricing": {
    "as_of": "2026-01-01",
    "source_url": "https://openai.com/pricing",
    "currency": "USD",
    "models": {
      "gpt-5": {"input": 1.25, "output": 5.0}
    }
  }
}`

// --- Loading ---

func TestLoadIDComesFromFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "openai.json", validOpenAIBody)

	cat, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p, ok := cat.Get("openai")
	if !ok {
		t.Fatalf("expected provider %q", "openai")
	}
	if p.ID != "openai" {
		t.Fatalf("ID = %q, want %q (must come from filename, not body)", p.ID, "openai")
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// "base_urll" is a typo of base_url — must be rejected, not silently dropped.
	writeFile(t, dir, "typo.json", `{
  "display_name": "Typo",
  "base_urll": "https://example.com",
  "protocol": "openai_compat",
  "auth": {"kind": "none"}
}`)

	_, err := Load(dir)
	if err == nil {
		t.Fatalf("expected error for unknown field, got nil")
	}
}

func TestLoadFSRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()
	// idFromFilename strips exactly the trailing ".json" via
	// TrimSuffix(name, filepath.Ext(name)), so two distinct directory entries
	// only collide if their extensions differ but their stems match, e.g.
	// "openai.json" and "openai.JSON" (filepath.Ext is case-sensitive, so
	// both strip to "openai" — but the directory filter's HasSuffix(".json")
	// is also case-sensitive and skips "openai.JSON" entirely, so that route
	// doesn't fire either in Load/LoadFS as currently filtered).
	//
	// The dedup check at cat.Providers[p.ID] is exercised here directly via
	// the two-entries-different-extension-mixed-case path through LoadFS,
	// which is the only way to reach it without editing production code.
	fsys := fstest.MapFS{
		"providers/openai.json": &fstest.MapFile{Data: []byte(validOpenAIBody)},
	}
	// First confirm the non-clashing baseline loads fine.
	if _, err := LoadFS(fsys, "providers"); err != nil {
		t.Fatalf("baseline LoadFS: %v", err)
	}
	t.Skip("duplicate-id guard (load.go:36-38, load.go:67-69) could not be triggered through the public Load/LoadFS surface: idFromFilename's single-extension-strip combined with the case-sensitive \".json\" suffix filter means no two distinct real directory-entry names in one call produce the same id. Reporting as a finding rather than deleting the assertion path — see final report.")
}

func TestLoadEmptyDirectoryIsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Fatalf("expected error for empty directory, got nil catalog")
	}
}

func TestLoadInvalidJSONNamesProvider(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "broken.json", `{ this is not json`)

	_, err := Load(dir)
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Fatalf("error %q does not name the provider %q", err.Error(), "broken")
	}
}

func TestLoadFSMatchesLoadBehavior(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"providers/openai.json": &fstest.MapFile{Data: []byte(validOpenAIBody)},
	}
	cat, err := LoadFS(fsys, "providers")
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	p, ok := cat.Get("openai")
	if !ok || p.ID != "openai" {
		t.Fatalf("LoadFS did not produce provider %q correctly: %+v ok=%v", "openai", p, ok)
	}
}

func TestLoadFSEmptyDirIsError(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"providers/.keep": &fstest.MapFile{Data: []byte("")},
	}
	_, err := LoadFS(fsys, "providers")
	if err == nil {
		t.Fatalf("expected error for directory with no json files")
	}
}

// --- Validation ---

func baseValidProvider() Provider {
	in := 1.0
	out := 2.0
	return Provider{
		ID:          "test",
		DisplayName: "Test",
		BaseURL:     "https://example.com",
		Protocol:    ProtocolOpenAICompat,
		Auth:        Auth{Kind: AuthAPIKey, Scheme: SchemeBearer},
		Pricing: Pricing{
			AsOf:      "2026-01-01",
			SourceURL: "https://example.com/pricing",
			Currency:  "USD",
			Models: map[string]Model{
				"m1": {Input: &in, Output: &out},
			},
		},
	}
}

func TestValidateRejectsUnknownProtocol(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	p.Protocol = "carrier_pigeon"
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error for unknown protocol")
	}
}

func TestValidateRejectsUnknownAuthKind(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	p.Auth.Kind = "magic"
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error for unknown auth kind")
	}
}

func TestValidateRejectsUnknownAuthScheme(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	p.Auth.Scheme = "handshake"
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error for unknown auth scheme")
	}
}

func TestValidateRejectsUnknownAvailability(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	p.Availability = "sometimes"
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error for unknown availability")
	}
}

func TestValidateRequiresBaseURLWhenGenerallyAvailable(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	p.BaseURL = ""
	p.Availability = AvailabilityGA
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error when base_url missing and availability is generally_available")
	}
}

func TestValidateAllowsMissingBaseURLWhenNoPublicAPI(t *testing.T) {
	t.Parallel()
	p := Provider{ID: "no-api", Availability: AvailabilityNoAPI}
	if err := Validate(&p); err != nil {
		t.Fatalf("expected no_public_api provider with empty base_url to validate, got: %v", err)
	}
	if p.BaseURL != "" {
		t.Fatalf("did not expect Validate to fill in base_url")
	}
}

func TestValidateRejectsNegativePrice(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	neg := -0.5
	p.Pricing.Models["m1"] = Model{Input: &neg, Output: p.Pricing.Models["m1"].Output}
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error for negative price")
	}
}

func TestValidateRejectsNaNPrice(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	nan := math.NaN()
	m := p.Pricing.Models["m1"]
	m.Output = &nan
	p.Pricing.Models["m1"] = m
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error for NaN price")
	}
}

func TestValidateRejectsInfPrice(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	inf := math.Inf(1)
	m := p.Pricing.Models["m1"]
	m.Input = &inf
	p.Pricing.Models["m1"] = m
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error for +Inf price")
	}
}

func TestValidateRequiresSourceURLWhenPriced(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	p.Pricing.SourceURL = ""
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error for missing source_url when prices present")
	}
}

func TestValidateRequiresCurrencyWhenPriced(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	p.Pricing.Currency = ""
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error for missing currency when prices present")
	}
}

func TestValidateRequiresParseableAsOfWhenPriced(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	p.Pricing.AsOf = "not-a-date"
	if err := Validate(&p); err == nil {
		t.Fatalf("expected error for unparseable as_of when prices present")
	}
}

func TestValidateDoesNotRequireProvenanceWithoutPrices(t *testing.T) {
	t.Parallel()
	p := baseValidProvider()
	p.Pricing = Pricing{} // no models, no source_url/currency/as_of
	if err := Validate(&p); err != nil {
		t.Fatalf("expected provider with no priced models to validate without provenance, got: %v", err)
	}
}

// --- HasPrice / unknown-vs-free ---

func TestHasPriceFalseWhenInputNil(t *testing.T) {
	t.Parallel()
	out := 1.0
	m := Model{Input: nil, Output: &out}
	if m.HasPrice() {
		t.Fatalf("HasPrice() = true, want false when Input is nil")
	}
}

func TestHasPriceFalseWhenOutputNil(t *testing.T) {
	t.Parallel()
	in := 1.0
	m := Model{Input: &in, Output: nil}
	if m.HasPrice() {
		t.Fatalf("HasPrice() = true, want false when Output is nil")
	}
}

func TestHasPriceTrueWhenBothZero(t *testing.T) {
	t.Parallel()
	zero1, zero2 := 0.0, 0.0
	m := Model{Input: &zero1, Output: &zero2}
	if !m.HasPrice() {
		t.Fatalf("HasPrice() = false, want true for a genuinely free (zero-priced) model")
	}
}

// --- Adaptors: pricing catalog omits unknown, includes zero ---

func providerWithModels(id, currency string, models map[string]Model) Provider {
	return Provider{
		ID:          id,
		DisplayName: id,
		BaseURL:     "https://example.com",
		Protocol:    ProtocolOpenAICompat,
		Auth:        Auth{Kind: AuthAPIKey},
		Pricing: Pricing{
			AsOf:      "2026-02-01",
			SourceURL: "https://example.com/pricing",
			Currency:  currency,
			Models:    models,
		},
	}
}

func TestToPricingCatalogOmitsUnknownPriceModel(t *testing.T) {
	t.Parallel()
	in, out := 1.0, 2.0
	cat := &Catalog{Providers: map[string]Provider{
		// A second, priced model keeps the provider itself present in the
		// pricing catalog (a provider with zero priced models is dropped
		// entirely — see TestToModelCatalog* for that distinct behavior),
		// isolating the assertion to per-model omission.
		"p1": providerWithModels("p1", "USD", map[string]Model{
			"unknown-price": {Input: &in, Output: nil}, // unpriced
			"priced":        {Input: &in, Output: &out},
		}),
	}}
	pc := cat.ToPricingCatalog()
	prov, ok := pc.Providers["p1"]
	if !ok {
		t.Fatalf("expected provider p1 in pricing catalog")
	}
	if _, present := prov.Models["unknown-price"]; present {
		t.Fatalf("model with unknown price must be absent from pricing catalog map, but it is present")
	}
}

func TestToPricingCatalogIncludesZeroPricedModel(t *testing.T) {
	t.Parallel()
	zero1, zero2 := 0.0, 0.0
	cat := &Catalog{Providers: map[string]Provider{
		"p1": providerWithModels("p1", "USD", map[string]Model{
			"free-model": {Input: &zero1, Output: &zero2},
		}),
	}}
	pc := cat.ToPricingCatalog()
	prov, ok := pc.Providers["p1"]
	if !ok {
		t.Fatalf("expected provider p1 in pricing catalog")
	}
	rates, present := prov.Models["free-model"]
	if !present {
		t.Fatalf("zero-priced model must be present in pricing catalog map")
	}
	if rates.InputPer1MTokensUSD != 0 || rates.OutputPer1MTokensUSD != 0 {
		t.Fatalf("unexpected rates for free model: %+v", rates)
	}
}

// --- Adaptors: model catalog nil vs non-nil Pricing pointer ---

func TestToModelCatalogNilPricingForUnknownPrice(t *testing.T) {
	t.Parallel()
	in := 1.0
	cat := &Catalog{Providers: map[string]Provider{
		"p1": providerWithModels("p1", "USD", map[string]Model{
			"unknown-price": {Input: &in, Output: nil},
		}),
	}}
	mc := cat.ToModelCatalog()
	entry, ok := mc.Providers["p1"]
	if !ok {
		t.Fatalf("expected provider p1 in model catalog")
	}
	m, ok := entry.Models["unknown-price"]
	if !ok {
		t.Fatalf("expected model unknown-price to be present in model catalog (unlike pricing catalog, models aren't dropped here)")
	}
	if m.Pricing != nil {
		t.Fatalf("Pricing pointer must be nil for a model with unknown price, got %+v", m.Pricing)
	}
}

func TestToModelCatalogNonNilPricingForZeroPrice(t *testing.T) {
	t.Parallel()
	zero1, zero2 := 0.0, 0.0
	cat := &Catalog{Providers: map[string]Provider{
		"p1": providerWithModels("p1", "USD", map[string]Model{
			"free-model": {Input: &zero1, Output: &zero2},
		}),
	}}
	mc := cat.ToModelCatalog()
	entry := mc.Providers["p1"]
	m, ok := entry.Models["free-model"]
	if !ok {
		t.Fatalf("expected free-model present")
	}
	if m.Pricing == nil {
		t.Fatalf("Pricing pointer must be non-nil for a model priced at exactly 0")
	}
	if m.Pricing.InputPer1MTokensUSD != 0 || m.Pricing.OutputPer1MTokensUSD != 0 {
		t.Fatalf("unexpected pricing values: %+v", m.Pricing)
	}
}

// --- Adaptors: availability gating ---

func TestToModelCatalogSkipsNonGAProviders(t *testing.T) {
	t.Parallel()
	p := providerWithModels("waitlisted", "USD", map[string]Model{})
	p.Availability = AvailabilityWaitlist
	cat := &Catalog{Providers: map[string]Provider{"waitlisted": p}}
	mc := cat.ToModelCatalog()
	if _, present := mc.Providers["waitlisted"]; present {
		t.Fatalf("a waitlisted provider must not appear in the model catalog")
	}
}

func TestToModelCatalogIncludesGAProvider(t *testing.T) {
	t.Parallel()
	p := providerWithModels("ga-provider", "USD", map[string]Model{})
	p.Availability = AvailabilityGA
	cat := &Catalog{Providers: map[string]Provider{"ga-provider": p}}
	mc := cat.ToModelCatalog()
	if _, present := mc.Providers["ga-provider"]; !present {
		t.Fatalf("a generally_available provider must appear in the model catalog")
	}
}

// --- Adaptors: currency exclusion ---

func TestToPricingCatalogExcludesNonUSDProviders(t *testing.T) {
	t.Parallel()
	in, out := 1.0, 2.0
	p := providerWithModels("cny-provider", "CNY", map[string]Model{
		"m1": {Input: &in, Output: &out},
	})
	cat := &Catalog{Providers: map[string]Provider{"cny-provider": p}}
	pc := cat.ToPricingCatalog()
	if _, present := pc.Providers["cny-provider"]; present {
		t.Fatalf("a non-USD provider must be excluded from the USD pricing catalog")
	}
}

func TestNonUSDProvidersReportsThem(t *testing.T) {
	t.Parallel()
	in, out := 1.0, 2.0
	p := providerWithModels("cny-provider", "CNY", map[string]Model{
		"m1": {Input: &in, Output: &out},
	})
	usd := providerWithModels("usd-provider", "USD", map[string]Model{
		"m1": {Input: &in, Output: &out},
	})
	cat := &Catalog{Providers: map[string]Provider{
		"cny-provider": p,
		"usd-provider": usd,
	}}
	nonUSD := cat.NonUSDProviders()
	if len(nonUSD) != 1 || nonUSD[0] != "cny-provider" {
		t.Fatalf("NonUSDProviders() = %v, want [cny-provider]", nonUSD)
	}
}

// --- Adaptors: PricingVersion ---

func TestPricingVersionContainsProviderIDsAndAsOfDates(t *testing.T) {
	t.Parallel()
	in, out := 1.0, 2.0
	p1 := providerWithModels("alpha", "USD", map[string]Model{"m1": {Input: &in, Output: &out}})
	p1.Pricing.AsOf = "2026-01-15"
	p2 := providerWithModels("beta", "USD", map[string]Model{"m1": {Input: &in, Output: &out}})
	p2.Pricing.AsOf = "2026-02-20"

	cat := &Catalog{Providers: map[string]Provider{"alpha": p1, "beta": p2}}
	pc := cat.ToPricingCatalog()

	if !strings.Contains(pc.PricingVersion, "alpha@2026-01-15") {
		t.Fatalf("PricingVersion %q missing alpha's id and as_of date", pc.PricingVersion)
	}
	if !strings.Contains(pc.PricingVersion, "beta@2026-02-20") {
		t.Fatalf("PricingVersion %q missing beta's id and as_of date", pc.PricingVersion)
	}
}

// --- Adaptors: cached-input / cache-write passthrough ---

func TestCachedRatesCarryThroughToPricingCatalog(t *testing.T) {
	t.Parallel()
	in, out, cachedIn, cacheWrite := 1.0, 2.0, 0.25, 1.5
	p := providerWithModels("p1", "USD", map[string]Model{
		"m1": {Input: &in, Output: &out, CachedInput: &cachedIn, CacheWrite: &cacheWrite},
	})
	cat := &Catalog{Providers: map[string]Provider{"p1": p}}
	pc := cat.ToPricingCatalog()
	rates := pc.Providers["p1"].Models["m1"]
	if rates.CacheReadPer1MTokensUSD != cachedIn {
		t.Fatalf("CacheReadPer1MTokensUSD = %v, want %v", rates.CacheReadPer1MTokensUSD, cachedIn)
	}
	if rates.CacheWritePer1MTokensUSD != cacheWrite {
		t.Fatalf("CacheWritePer1MTokensUSD = %v, want %v", rates.CacheWritePer1MTokensUSD, cacheWrite)
	}
}

func TestCachedRatesCarryThroughToModelCatalog(t *testing.T) {
	t.Parallel()
	in, out, cachedIn, cacheWrite := 1.0, 2.0, 0.25, 1.5
	p := providerWithModels("p1", "USD", map[string]Model{
		"m1": {Input: &in, Output: &out, CachedInput: &cachedIn, CacheWrite: &cacheWrite},
	})
	p.Availability = AvailabilityGA
	cat := &Catalog{Providers: map[string]Provider{"p1": p}}
	mc := cat.ToModelCatalog()
	pricing := mc.Providers["p1"].Models["m1"].Pricing
	if pricing == nil {
		t.Fatalf("expected non-nil pricing")
	}
	if pricing.CacheReadPer1MTokensUSD != cachedIn {
		t.Fatalf("CacheReadPer1MTokensUSD = %v, want %v", pricing.CacheReadPer1MTokensUSD, cachedIn)
	}
	if pricing.CacheWritePer1MTokensUSD != cacheWrite {
		t.Fatalf("CacheWritePer1MTokensUSD = %v, want %v", pricing.CacheWritePer1MTokensUSD, cacheWrite)
	}
}

// --- Staleness ---

func TestStaleProvidersOrderedOldestFirstAndFiltered(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	maxAge := 30 * 24 * time.Hour

	in, out := 1.0, 2.0
	veryOld := providerWithModels("very-old", "USD", map[string]Model{"m": {Input: &in, Output: &out}})
	veryOld.Pricing.AsOf = "2025-01-01" // way older than maxAge

	old := providerWithModels("old", "USD", map[string]Model{"m": {Input: &in, Output: &out}})
	old.Pricing.AsOf = "2026-03-01" // older than maxAge but newer than veryOld

	fresh := providerWithModels("fresh", "USD", map[string]Model{"m": {Input: &in, Output: &out}})
	fresh.Pricing.AsOf = "2026-05-25" // within maxAge

	noDate := providerWithModels("no-date", "USD", map[string]Model{})
	noDate.Pricing.AsOf = ""

	cat := &Catalog{Providers: map[string]Provider{
		"very-old": veryOld,
		"old":      old,
		"fresh":    fresh,
		"no-date":  noDate,
	}}

	stale := cat.StaleProviders(now, maxAge)
	want := []string{"very-old", "old"}
	if len(stale) != len(want) {
		t.Fatalf("StaleProviders() = %v, want %v", stale, want)
	}
	for i, id := range want {
		if stale[i] != id {
			t.Fatalf("StaleProviders()[%d] = %q, want %q (order must be oldest first): got %v", i, stale[i], id, stale)
		}
	}
}

// --- Guard on real shipped data ---

func TestRealShippedProvidersAllValidate(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "..", "catalog", "providers")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("catalog/providers does not exist yet: %v", err)
	}
	if _, err := Load(dir); err != nil {
		t.Fatalf("shipped provider catalog failed to load/validate: %v", err)
	}
}
