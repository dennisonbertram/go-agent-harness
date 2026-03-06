package pricing

type Rates struct {
	InputPer1MTokensUSD      float64 `json:"input_per_1m_tokens_usd"`
	OutputPer1MTokensUSD     float64 `json:"output_per_1m_tokens_usd"`
	CacheReadPer1MTokensUSD  float64 `json:"cache_read_per_1m_tokens_usd,omitempty"`
	CacheWritePer1MTokensUSD float64 `json:"cache_write_per_1m_tokens_usd,omitempty"`
}

type ProviderCatalog struct {
	Aliases map[string]string `json:"aliases,omitempty"`
	Models  map[string]Rates  `json:"models"`
}

type Catalog struct {
	PricingVersion string                     `json:"pricing_version,omitempty"`
	Providers      map[string]ProviderCatalog `json:"providers"`
}

type ResolvedRates struct {
	Provider       string
	Model          string
	PricingVersion string
	Rates          Rates
}

type Resolver interface {
	Resolve(provider, model string) (ResolvedRates, bool)
}
