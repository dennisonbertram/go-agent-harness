# OpenRouter Dynamic Models Investigation

**Date**: 2026-03-19  
**Status**: Complete  
**Author**: Codebase Exploration

## Summary

The TUI currently uses a hardcoded `DefaultModels` list in the modelswitcher component, supplemented by dynamic model loading from harnessd's `/v1/models` endpoint. To support dynamic OpenRouter model loading, we need to:

1. Update the TUI to optionally fetch models from OpenRouter's API
2. Transform OpenRouter's response format into the internal `ServerModelEntry` structure
3. Maintain backward compatibility with the existing harnessd catalog system

---

## Current Architecture

### 1. ModelEntry Structure (modelswitcher/model.go:9-21)

```go
type ModelEntry struct {
    ID            string // e.g. "gpt-4.1-mini"
    DisplayName   string // e.g. "GPT-4.1 Mini"
    Provider      string // provider key for API (e.g. "openai", "anthropic")
    ProviderLabel string // human-readable provider name for display (e.g. "OpenAI")
    ReasoningMode bool   // true for reasoning models
    IsCurrent     bool
    Available     bool   // whether provider is configured with an API key
}
```

### 2. DefaultModels (modelswitcher/model.go:24-49)

A hardcoded list of ~20 models across 8 providers (OpenAI, Anthropic, Google, DeepSeek, xAI, Groq, Qwen, Kimi):

```go
var DefaultModels = []ModelEntry{
    {ID: "gpt-4.1", DisplayName: "GPT-4.1", Provider: "openai", ProviderLabel: "OpenAI"},
    {ID: "gpt-4.1-mini", DisplayName: "GPT-4.1 Mini", Provider: "openai", ProviderLabel: "OpenAI"},
    // ... 18 more entries
}
```

**Usage**: `New(currentModelID)` copies `DefaultModels` and marks the current model as selected.

### 3. ServerModelEntry Structure (modelswitcher/model.go:93-97)

```go
type ServerModelEntry struct {
    ID       string `json:"id"`
    Provider string `json:"provider"`
}
```

This is the minimal shape returned by `GET /v1/models` from harnessd.

### 4. TUI API Calls (api.go:73-97)

**fetchModelsCmd** makes `GET /v1/models` to harnessd:

```go
func fetchModelsCmd(baseURL string) tea.Cmd {
    return func() tea.Msg {
        url := strings.TrimRight(baseURL, "/") + "/v1/models"
        resp, err := http.Get(url)
        // ... error handling ...
        var mr modelsResponse // {Models: []ServerModelEntry}
        json.NewDecoder(resp.Body).Decode(&mr)
        return ModelsFetchedMsg{Models: mr.Models}
    }
}
```

**modelsResponse** wraps the server's model list:

```go
type modelsResponse struct {
    Models []modelswitcher.ServerModelEntry `json:"models"`
}
```

### 5. Server /v1/models Endpoint (internal/server/http.go:373-450)

**Route**: Registered at line 155: `mux.Handle("/v1/models", auth(read(http.HandlerFunc(s.handleModels))))`

**Response Format**:

```go
type ModelResponse struct {
    ID                string   `json:"id"`
    Provider          string   `json:"provider"`
    Aliases           []string `json:"aliases"`
    InputCostPerMTok  float64  `json:"input_cost_per_mtok"`
    OutputCostPerMTok float64  `json:"output_cost_per_mtok"`
}
```

**Logic**: When `catalog` is non-nil, iterates providers and models in the catalog, building a deterministic list. Returns `{models: [...]}`.

**Key Detail**: The TUI only uses `ID` and `Provider` from the response (ignores aliases and pricing). The display name, provider label, and reasoning mode are enriched client-side using local maps.

### 6. TUI Model Loading Flow

**Trigger**: When user presses "/" to open model switcher (model.go:915-926):

```go
case "model":
    m.modelSwitcher = modelswitcher.New(m.selectedModel).Open()
    // ...
    m.modelSwitcher = m.modelSwitcher.SetLoading(true)
    cmds = append(cmds, fetchModelsCmd(m.config.BaseURL))
    cmds = append(cmds, fetchProvidersCmd(m.config.BaseURL))
```

**Response Handler** (model.go:1150-1154):

```go
case ModelsFetchedMsg:
    currentStarred := m.modelSwitcher.StarredIDs()
    m.modelSwitcher = m.modelSwitcher.WithModels(msg.Models).SetLoading(false)
    m.modelSwitcher = m.modelSwitcher.WithStarred(currentStarred)
```

**Model Enrichment** (modelswitcher/model.go:372-409):

`WithModels` method transforms `ServerModelEntry` (just ID + provider) into full `ModelEntry` by:
- Looking up `DisplayName` in `modelDisplayNames` map (line 53-70)
- Looking up `ProviderLabel` in `providerLabels` map (line 79-91)
- Checking `reasoningModelIDs` map to set `ReasoningMode` flag (line 73-77)
- Re-applying provider availability if previously set via `WithAvailability`

---

## OpenRouter API Reference

**Endpoint**: `GET https://openrouter.ai/api/v1/models`

**Response Format**:

```json
{
  "data": [
    {
      "id": "openai/gpt-4-turbo",
      "name": "OpenAI GPT-4 Turbo",
      "description": "...",
      "pricing": {
        "prompt": "0.01",
        "completion": "0.03"
      },
      "context_length": 128000,
      "architecture": {...},
      "top_provider": {...}
    },
    // ... more models ...
  ]
}
```

**Key Differences**:
- Models have a `/` separator: `"openai/gpt-4-turbo"`
- Response is wrapped in `{data: [...]}` not `{models: [...]}`
- Model IDs include provider prefix (unlike harnessd where ID is just the model name)
- Includes detailed metadata (name, description, pricing, context_length, etc.)

---

## Required Changes for OpenRouter Support

### 1. New modelswitcher Mapping: OpenRoute

Add in `modelswitcher/model.go`:

```go
// openRouterProvider maps OpenRouter model IDs to native provider keys.
// Maps full IDs like "openai/gpt-4-turbo" to native provider keys.
var openRouterProviderMap = map[string]string{
    "openai/gpt-4-turbo": "openai",
    "openai/gpt-4.1": "openai",
    // ... etc ...
}
```

Or extract provider from the `provider/` prefix dynamically.

### 2. New API Function in tui/api.go

```go
// OpenRouterModelEntry matches the JSON structure from openrouter.ai API.
type OpenRouterModelEntry struct {
    ID              string `json:"id"`
    Name            string `json:"name"`
    Description     string `json:"description"`
    Pricing         map[string]string `json:"pricing,omitempty"`
    ContextLength   int `json:"context_length,omitempty"`
}

// fetchOpenRouterModelsCmd fetches models from the public OpenRouter API.
// Transforms the OpenRouter response into ServerModelEntry format.
func fetchOpenRouterModelsCmd() tea.Cmd {
    return func() tea.Msg {
        url := "https://openrouter.ai/api/v1/models"
        resp, err := http.Get(url)
        if err != nil {
            return ModelsFetchErrorMsg{Err: "OpenRouter API: " + err.Error()}
        }
        defer resp.Body.Close()
        
        var orResp struct {
            Data []OpenRouterModelEntry `json:"data"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
            return ModelsFetchErrorMsg{Err: "OpenRouter decode: " + err.Error()}
        }
        
        // Transform: "openai/gpt-4" -> provider="openai", id="gpt-4"
        // OR keep full ID and map separately
        models := make([]modelswitcher.ServerModelEntry, 0, len(orResp.Data))
        for _, m := range orResp.Data {
            // Extract provider from ID (split on "/")
            parts := strings.Split(m.ID, "/")
            provider := parts[0] // "openai" from "openai/gpt-4"
            modelID := m.ID      // keep full ID or use parts[1]
            
            models = append(models, modelswitcher.ServerModelEntry{
                ID:       modelID,
                Provider: provider,
            })
        }
        
        return ModelsFetchedMsg{Models: models}
    }
}
```

### 3. Update Display Name Mapping

The existing `modelDisplayNames` map won't cover all OpenRouter models. Need to:

**Option A**: Fall back to the OpenRouter `name` field when ID not in map (already happens in `WithModels` at line 378-380)

**Option B**: Add OpenRouter name to the enrichment:

```go
type OpenRouterModelEntry struct {
    ID   string
    Name string
    // ...
}

// In fetchOpenRouterModelsCmd response handler:
// Attach name metadata so WithModels can use it
```

But `ServerModelEntry` only has `ID` and `Provider`. Need to extend it:

```go
type ServerModelEntry struct {
    ID       string `json:"id"`
    Provider string `json:"provider"`
    // New optional fields:
    DisplayName string `json:"display_name,omitempty"`
    Aliases     []string `json:"aliases,omitempty"`
}
```

Then `WithModels` checks: if `DisplayName` is provided, use it; else look up in map.

### 4. Conditional Fetch in TUI

When model switcher opens:

```go
if m.config.UseOpenRouter {
    cmds = append(cmds, fetchOpenRouterModelsCmd())
} else {
    cmds = append(cmds, fetchModelsCmd(m.config.BaseURL))
}
```

Where `m.config.UseOpenRouter` is a boolean flag (already exists: `gatewayOptions` at model.go:38-41 with "openrouter" option).

### 5. Update Provider Availability Tracking

OpenRouter is a single gateway—doesn't need per-provider API keys. The availability check should:

```go
// For native providers: check if OPENAI_API_KEY, ANTHROPIC_API_KEY, etc. are set
// For OpenRouter models: check if OPENROUTER_API_KEY is set

func modelAvailable(provider string, gateway string) bool {
    if gateway == "openrouter" {
        return hasEnvKey("OPENROUTER_API_KEY")
    }
    // Native provider logic
    return checkProviderKey(provider)
}
```

---

## Implementation Checklist

### Phase 1: Fetch OpenRouter Models

- [ ] Add `fetchOpenRouterModelsCmd()` to `tui/api.go`
- [ ] Transform OpenRouter response to `ServerModelEntry` format
- [ ] Handle API errors (rate limits, timeouts, no internet)
- [ ] Cache response to avoid repeated API calls

### Phase 2: Extend ServerModelEntry

- [ ] Add optional `DisplayName` field to `ServerModelEntry`
- [ ] Update `WithModels` to use provided `DisplayName` if available
- [ ] Fallback to `modelDisplayNames` map for unknown models
- [ ] Update `internal/server/http.go` ModelResponse to include display hints

### Phase 3: Gateway Selection

- [ ] Use existing `gatewaySelected` state to determine fetch source
- [ ] When gateway = "openrouter", call `fetchOpenRouterModelsCmd()`
- [ ] When gateway = "", call `fetchModelsCmd(baseURL)` (native providers)

### Phase 4: Availability Tracking

- [ ] Update provider availability function to handle "openrouter" as special case
- [ ] Check `OPENROUTER_API_KEY` env var for OpenRouter models
- [ ] Maintain per-provider checks for native gateway

### Phase 5: Testing & Refinement

- [ ] Test with public OpenRouter API
- [ ] Verify model selection works with OpenRouter IDs
- [ ] Test fallback to `DefaultModels` if API fails
- [ ] Verify backward compatibility with harnessd catalog

---

## Risk Analysis

### Backward Compatibility
- **Low Risk**: Changes are additive; `DefaultModels` remains as fallback
- **Mitigation**: Only fetch OpenRouter API when explicitly selected via gateway toggle

### API Reliability
- **Medium Risk**: OpenRouter API could be slow, rate-limited, or down
- **Mitigation**: 
  - Set HTTP timeout (5-10s)
  - Fallback to `DefaultModels` on fetch error
  - Cache response for session duration

### Model ID Collisions
- **Low Risk**: OpenRouter model IDs include provider prefix (e.g. "openai/gpt-4"); unlikely to collide with native IDs
- **Mitigation**: Document naming convention; test with both native and OpenRouter models selected

### Provider Key Management
- **Medium Risk**: OpenRouter requires a single API key; native providers each need separate keys
- **Mitigation**: UI clearly indicates which auth is required based on gateway selection

---

## Existing References in Codebase

### OpenRouter Slug Mapping (modelswitcher/model.go:486-513)

Already exists! Maps native model IDs to OpenRouter equivalents:

```go
var openRouterSlugs = map[string]string{
    "gpt-4.1":                   "openai/gpt-4.1",
    "gpt-4.1-mini":              "openai/gpt-4.1-mini",
    "claude-opus-4-6":           "anthropic/claude-opus-4-6",
    // ... 12 more ...
}

func OpenRouterSlug(modelID string) string {
    if slug, ok := openRouterSlugs[modelID]; ok {
        return slug
    }
    return modelID
}
```

**Usage**: When sending requests via OpenRouter gateway, the native model ID is mapped to OpenRouter's slug format.

### Gateway Selection UI (model.go:38-41)

```go
var gatewayOptions = []gatewayOption{
    {ID: "", Label: "Direct", Desc: "Use each model's native provider"},
    {ID: "openrouter", Label: "OpenRouter", Desc: "Route all models via openrouter.ai"},
}
```

Gateway selection is already implemented in the TUI. The field `m.selectedGateway` tracks which gateway is active.

---

## Summary of Answers

### What does the ModelEntry struct look like in modelswitcher?

```go
type ModelEntry struct {
    ID            string // model identifier
    DisplayName   string // human-readable name for UI
    Provider      string // provider key (e.g., "openai", "anthropic", "openrouter")
    ProviderLabel string // display label (e.g., "OpenAI", "OpenRouter")
    ReasoningMode bool   // true if model supports reasoning effort selection
    IsCurrent     bool   // true if this is the currently selected model
    Available     bool   // true if provider is configured with an API key
}
```

### How are DefaultModels currently used?

`DefaultModels` is a hardcoded slice of ~20 models. When the model switcher opens:
1. `modelswitcher.New(currentModelID)` copies `DefaultModels` into the Model state
2. The model with matching ID is marked `IsCurrent`
3. These models are shown in the dropdown until replaced by server-fetched models
4. Server-fetched models (from harnessd's `/v1/models`) replace `DefaultModels` via `WithModels()`
5. If server fetch fails, `DefaultModels` serves as fallback

### Does harnessd expose a /v1/models endpoint? If so, what does it return?

**Yes**. Route: `GET /v1/models` (registered in http.go:155, handled at http.go:373-450)

**Response**:

```json
{
  "models": [
    {
      "id": "gpt-4.1",
      "provider": "openai",
      "aliases": ["gpt-4-turbo"],
      "input_cost_per_mtok": 0.01,
      "output_cost_per_mtok": 0.03
    },
    // ... more models from configured catalog ...
  ]
}
```

**Notes**:
- Only returns `ID` and `Provider` fields (TUI ignores aliases and pricing)
- Requires authentication (bearer token)
- Reads from the configured provider catalog (set up at server startup)
- Returns empty list `{models: []}` if catalog is nil

### Where is FetchModels called from in the TUI?

In `cmd/harnesscli/tui/model.go`, the `Update()` method handles the "model" slash command (line 915-926):

```go
case "model":
    m.modelSwitcher = modelswitcher.New(m.selectedModel).Open()
    m.modelSwitcher = m.modelSwitcher.SetLoading(true)
    cmds = append(cmds, fetchModelsCmd(m.config.BaseURL))  // <-- HERE
    cmds = append(cmds, fetchProvidersCmd(m.config.BaseURL))
```

The response is handled in the message switch (line 1150-1154):

```go
case ModelsFetchedMsg:
    currentStarred := m.modelSwitcher.StarredIDs()
    m.modelSwitcher = m.modelSwitcher.WithModels(msg.Models).SetLoading(false)
    m.modelSwitcher = m.modelSwitcher.WithStarred(currentStarred)
```

### What would need to change to support dynamically loading models from OpenRouter API?

**Minimal changes**:

1. **Add API call in `tui/api.go`**:
   - New function `fetchOpenRouterModelsCmd()` that calls OpenRouter's public API
   - Transform response from `{data: []}` format to `{models: [ServerModelEntry]}` format

2. **Extend `ServerModelEntry` in `modelswitcher/model.go`**:
   - Add optional `DisplayName` field so OpenRouter can provide the name
   - Update `WithModels` to use provided `DisplayName` if available

3. **Conditional fetch in TUI `model.go`**:
   - Check `m.selectedGateway` when opening model switcher
   - If "openrouter", call `fetchOpenRouterModelsCmd()`
   - If "" (default), call `fetchModelsCmd(baseURL)`

4. **Update availability tracking**:
   - Extend provider availability function to recognize "openrouter" as special case
   - Check `OPENROUTER_API_KEY` env var instead of per-provider keys

5. **No changes needed to**:
   - `DefaultModels` (remains as fallback)
   - Model selection/acceptance logic (works with any ID format)
   - `openRouterSlugs` mapping (already exists for ID transformation when actually making requests)

---

## Conclusion

The codebase already has most infrastructure in place:
- Gateway selection UI exists (`gatewayOptions`)
- OpenRouter slug mapping exists (`openRouterSlugs`)
- Server-side model fetching is abstracted (`fetchModelsCmd`, `ModelsFetchedMsg`)
- Client-side enrichment is flexible (`WithModels`, `modelDisplayNames` maps)

Adding OpenRouter API support is primarily a matter of:
1. Adding a parallel fetch function for OpenRouter's public API
2. Transforming OpenRouter's response format to match internal `ServerModelEntry`
3. Conditionally choosing which fetch function to call based on gateway selection
4. Handling availability checks for the OpenRouter auth case

Risk is low because changes are additive and defaults remain intact.
