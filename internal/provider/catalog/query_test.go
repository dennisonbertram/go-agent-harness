package catalog

import (
	"sort"
	"testing"
)

// queryCatalog builds a small catalog with 2 providers and 4 models for query testing.
func queryCatalog() *Catalog {
	return &Catalog{
		CatalogVersion: "1.0",
		Providers: map[string]ProviderEntry{
			"openai": {
				DisplayName: "OpenAI",
				BaseURL:     "https://api.openai.com/v1",
				APIKeyEnv:   "OPENAI_API_KEY",
				Protocol:    "openai",
				Models: map[string]Model{
					"gpt-4o": {
						DisplayName:   "GPT-4o",
						Description:   "Flagship multimodal model",
						ContextWindow: 128000,
						Modalities:    []string{"text", "vision"},
						ToolCalling:   true,
						Streaming:     true,
						ReasoningMode: false,
						Strengths:     []string{"general", "code"},
						BestFor:       []string{"general-assistant", "code-generation"},
						SpeedTier:     "fast",
						CostTier:      "standard",
					},
					"o1": {
						DisplayName:   "o1",
						Description:   "Reasoning model",
						ContextWindow: 200000,
						Modalities:    []string{"text"},
						ToolCalling:   true,
						Streaming:     false,
						ReasoningMode: true,
						Strengths:     []string{"reasoning", "math"},
						BestFor:       []string{"complex-reasoning"},
						SpeedTier:     "slow",
						CostTier:      "premium",
					},
				},
			},
			"deepseek": {
				DisplayName: "DeepSeek",
				BaseURL:     "https://api.deepseek.com/v1",
				APIKeyEnv:   "DEEPSEEK_API_KEY",
				Protocol:    "openai-compatible",
				Models: map[string]Model{
					"deepseek-chat": {
						DisplayName:   "DeepSeek Chat",
						Description:   "Fast budget chat model",
						ContextWindow: 64000,
						Modalities:    []string{"text"},
						ToolCalling:   true,
						Streaming:     true,
						ReasoningMode: false,
						Strengths:     []string{"code", "chat"},
						BestFor:       []string{"code-generation", "chat"},
						SpeedTier:     "ultra-fast",
						CostTier:      "budget",
					},
					"deepseek-reasoner": {
						DisplayName:   "DeepSeek Reasoner",
						Description:   "Reasoning model",
						ContextWindow: 64000,
						Modalities:    []string{"text"},
						ToolCalling:   false,
						Streaming:     true,
						ReasoningMode: true,
						Strengths:     []string{"reasoning", "math"},
						BestFor:       []string{"complex-reasoning"},
						SpeedTier:     "slow",
						CostTier:      "budget",
					},
				},
			},
		},
	}
}

func TestListModelsReturnsAll(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	results := cat.ListModels()
	if len(results) != 4 {
		t.Fatalf("expected 4 models, got %d", len(results))
	}
	for _, r := range results {
		if r.Provider == "" {
			t.Fatalf("expected non-empty provider key")
		}
		if r.ProviderName == "" {
			t.Fatalf("expected non-empty provider display name")
		}
		if r.ModelID == "" {
			t.Fatalf("expected non-empty model ID")
		}
	}
}

func TestFilterModelsByProvider(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	results := cat.FilterModels(FilterOptions{Provider: "deepseek"})
	if len(results) != 2 {
		t.Fatalf("expected 2 deepseek models, got %d", len(results))
	}
	for _, r := range results {
		if r.Provider != "deepseek" {
			t.Fatalf("expected provider deepseek, got %q", r.Provider)
		}
	}
}

func TestFilterModelsByToolCalling(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	tc := true
	results := cat.FilterModels(FilterOptions{ToolCalling: &tc})
	if len(results) != 3 {
		t.Fatalf("expected 3 models with tool_calling, got %d", len(results))
	}
	for _, r := range results {
		if !r.Model.ToolCalling {
			t.Fatalf("expected tool_calling=true for %s/%s", r.Provider, r.ModelID)
		}
	}
}

func TestFilterModelsByCostTier(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	results := cat.FilterModels(FilterOptions{CostTier: "budget"})
	if len(results) != 2 {
		t.Fatalf("expected 2 budget models, got %d", len(results))
	}
	for _, r := range results {
		if r.Model.CostTier != "budget" {
			t.Fatalf("expected cost_tier=budget for %s/%s", r.Provider, r.ModelID)
		}
	}
}

func TestFilterModelsBySpeedTier(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	results := cat.FilterModels(FilterOptions{SpeedTier: "ultra-fast"})
	if len(results) != 1 {
		t.Fatalf("expected 1 ultra-fast model, got %d", len(results))
	}
	if results[0].ModelID != "deepseek-chat" {
		t.Fatalf("expected deepseek-chat, got %s", results[0].ModelID)
	}
}

func TestFilterModelsByBestFor(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	results := cat.FilterModels(FilterOptions{BestFor: "code-generation"})
	if len(results) != 2 {
		t.Fatalf("expected 2 models best for code-generation, got %d", len(results))
	}
	ids := []string{results[0].ModelID, results[1].ModelID}
	sort.Strings(ids)
	if ids[0] != "deepseek-chat" || ids[1] != "gpt-4o" {
		t.Fatalf("unexpected models: %v", ids)
	}
}

func TestFilterModelsByStrength(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	results := cat.FilterModels(FilterOptions{Strength: "reasoning"})
	if len(results) != 2 {
		t.Fatalf("expected 2 models with reasoning strength, got %d", len(results))
	}
}

func TestFilterModelsByMinContext(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	results := cat.FilterModels(FilterOptions{MinContext: 100000})
	if len(results) != 2 {
		t.Fatalf("expected 2 models with context >= 100000, got %d", len(results))
	}
	for _, r := range results {
		if r.Model.ContextWindow < 100000 {
			t.Fatalf("expected context >= 100000 for %s/%s, got %d", r.Provider, r.ModelID, r.Model.ContextWindow)
		}
	}
}

func TestFilterModelsByReasoning(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	reasoning := true
	results := cat.FilterModels(FilterOptions{Reasoning: &reasoning})
	if len(results) != 2 {
		t.Fatalf("expected 2 reasoning models, got %d", len(results))
	}
	for _, r := range results {
		if !r.Model.ReasoningMode {
			t.Fatalf("expected reasoning_mode=true for %s/%s", r.Provider, r.ModelID)
		}
	}
}

func TestFilterModelsMultipleFilters(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	reasoning := true
	results := cat.FilterModels(FilterOptions{
		CostTier:  "budget",
		Reasoning: &reasoning,
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 budget reasoning model, got %d", len(results))
	}
	if results[0].ModelID != "deepseek-reasoner" {
		t.Fatalf("expected deepseek-reasoner, got %s", results[0].ModelID)
	}
}

func TestFilterModelsByStreaming(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	streaming := false
	results := cat.FilterModels(FilterOptions{Streaming: &streaming})
	if len(results) != 1 {
		t.Fatalf("expected 1 non-streaming model, got %d", len(results))
	}
	if results[0].ModelID != "o1" {
		t.Fatalf("expected o1, got %s", results[0].ModelID)
	}
}

func TestFilterModelsByModality(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	results := cat.FilterModels(FilterOptions{Modality: "vision"})
	if len(results) != 1 {
		t.Fatalf("expected 1 vision model, got %d", len(results))
	}
	if results[0].ModelID != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %s", results[0].ModelID)
	}
}

func TestModelInfoKnown(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	result, ok := cat.ModelInfo("openai", "gpt-4o")
	if !ok {
		t.Fatalf("expected to find openai/gpt-4o")
	}
	if result.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", result.Provider)
	}
	if result.ModelID != "gpt-4o" {
		t.Fatalf("expected model_id gpt-4o, got %q", result.ModelID)
	}
	if result.Model.ContextWindow != 128000 {
		t.Fatalf("expected context_window 128000, got %d", result.Model.ContextWindow)
	}
}

func TestModelInfoUnknown(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	_, ok := cat.ModelInfo("openai", "nonexistent")
	if ok {
		t.Fatalf("expected not to find openai/nonexistent")
	}
	_, ok = cat.ModelInfo("unknown-provider", "gpt-4o")
	if ok {
		t.Fatalf("expected not to find unknown-provider/gpt-4o")
	}
}

func TestListProviders(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	providers := cat.ListProviders()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Key < providers[j].Key
	})
	if providers[0].Key != "deepseek" || providers[0].ModelCount != 2 {
		t.Fatalf("unexpected deepseek provider: %+v", providers[0])
	}
	if providers[1].Key != "openai" || providers[1].ModelCount != 2 {
		t.Fatalf("unexpected openai provider: %+v", providers[1])
	}
	if providers[0].DisplayName != "DeepSeek" {
		t.Fatalf("expected display name DeepSeek, got %q", providers[0].DisplayName)
	}
	if providers[1].BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("unexpected base_url: %q", providers[1].BaseURL)
	}
}

func TestFilterModelsNoMatch(t *testing.T) {
	t.Parallel()
	cat := queryCatalog()
	results := cat.FilterModels(FilterOptions{Provider: "nonexistent"})
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestListModelsEmpty(t *testing.T) {
	t.Parallel()
	cat := &Catalog{
		CatalogVersion: "1.0",
		Providers:      map[string]ProviderEntry{},
	}
	results := cat.ListModels()
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}
