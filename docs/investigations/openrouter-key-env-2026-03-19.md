# Investigation: OpenRouter API Key Not Loaded from Environment

**Date**: 2026-03-19
**Status**: Complete

## Issue Summary

The OpenRouter API key is set in `~/.zshrc` as an environment variable, but the TUI shows all OpenRouter models as unavailable even though the key exists. The user must manually paste the key via `/keys` command to make OpenRouter models selectable, despite the env var being present at TUI startup.

## Root Cause Analysis

### 1. How TUI Loads API Keys at Startup (Init Flow)

**Location**: `cmd/harnesscli/tui/model.go` lines 177–203

The TUI loads API keys in two stages:

1. **New()** (line 177–203): Constructor loads from persistent config file
   ```go
   if persistCfg, err := harnessconfig.Load(); err == nil {
       m.modelSwitcher = m.modelSwitcher.WithStarred(persistCfg.StarredModels)
       m.selectedGateway = persistCfg.Gateway
       m.pendingAPIKeys = persistCfg.APIKeys  // Loaded from ~/.harness/config
   }
   ```
   - Only reads from `~/.harness/config` (persistent storage)
   - Does NOT read from shell environment variables (e.g., `~/.zshrc`)

2. **Init()** (line 435–444): Called after New(), replays pendingAPIKeys to server
   ```go
   for provider, apiKey := range m.pendingAPIKeys {
       cmds = append(cmds, setProviderKeyCmd(m.config.BaseURL, provider, apiKey))
   }
   ```
   - Only processes keys that were already loaded from persistent config
   - If a key is only in the environment, it never gets sent to the server

### 2. How providerKeyConfigured() Works

**Location**: `cmd/harnesscli/tui/model.go` lines 355–369

```go
func (m Model) providerKeyConfigured(providerKey string) bool {
    // Check server's provider list (populated by ProvidersLoadedMsg)
    for _, p := range m.apiKeyProviders {
        if p.Name == providerKey && p.Configured {
            return true
        }
    }
    // Fallback: check locally cached keys (set via /keys or loaded from config)
    if key, ok := m.pendingAPIKeys[providerKey]; ok && key != "" {
        return true
    }
    return false
}
```

This checks two sources:
1. **Server-reported state** (`m.apiKeyProviders`): What the harness saw in its own env vars
2. **Local pending keys** (`m.pendingAPIKeys`): Keys from persistent config or manually entered via `/keys`

**The bug**: If a key is only in the user's shell environment (`~/.zshrc`), it won't appear in either source:
- The harness daemon (running as a separate process) saw the key in its own startup env
- But the TUI never reads the user's env vars
- So the TUI doesn't know the key exists

### 3. How the `/keys` Command Works

**Location**: 
- Command handler: `cmd/harnesscli/tui/model.go` line 948–954
- View: `cmd/harnesscli/tui/model.go` lines 1540–1610

When the user opens `/keys`:
1. Calls `fetchProvidersCmd(m.config.BaseURL)` → requests GET /v1/providers from harness
2. Server returns list with `Configured` flags based on its own env vars (line 274 of http.go)
3. User can manually type a key into the overlay
4. `setProviderKeyCmd()` sends PUT /v1/providers/{provider}/key to harness
5. Harness stores it in its `providerRegistry`, making it available to runs
6. TUI saves to persistent config via `harnessconfig.Save()`

**Why this works**: The `/keys` UI explicitly reads what the server knows + lets user enter missing keys.

### 4. Server-Side Provider Detection (HTTP GET /v1/providers)

**Location**: `internal/server/http.go` lines 247–286

```go
func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
    // ...
    for _, name := range providerNames {
        entry := s.catalog.Providers[name]
        configured := false
        if s.providerRegistry != nil {
            configured = s.providerRegistry.IsConfigured(name)
        } else {
            configured = os.Getenv(entry.APIKeyEnv) != ""  // Check server's env
        }
        providers = append(providers, ProviderResponse{
            Name:       name,
            Configured: configured,
            APIKeyEnv:  entry.APIKeyEnv,  // Returns the env var NAME
            BaseURL:    entry.BaseURL,
            ModelCount: len(entry.Models),
        })
    }
}
```

The server checks **its own** environment via `os.Getenv(entry.APIKeyEnv)` or provider registry state. The APIKeyEnv field returned is just the environment variable name (e.g., "OPENROUTER_API_KEY"), not the value.

### 5. Environment Variable Naming

**OpenRouter env var**: Must match the catalog entry for OpenRouter provider.

**Location**: The catalog is loaded from a JSON file referenced in server config. OpenRouter provider entry should specify:
```json
{
  "name": "openrouter",
  "api_key_env": "OPENROUTER_API_KEY",
  // ...
}
```

**Verification needed**: Check if the harness catalog actually defines OpenRouter with the correct env var name.

### 6. Model Switcher Availability Rendering

**Location**: `cmd/harnesscli/tui/components/modelswitcher/model.go` lines 355–371 + view.go lines 188–204

When `WithAvailability(fn func(string) bool)` is called:
```go
func (m Model) WithAvailability(fn func(string) bool) Model {
    // ...
    for i := range newModels {
        if fn != nil {
            newModels[i].Available = fn(newModels[i].Provider)  // ← Check provider key
        }
    }
    return result
}
```

The function `fn` receives the provider key (e.g., "openrouter") and returns whether that provider is configured.

In the view (view.go line 140):
```go
isUnavailable := m.availabilitySet && !entry.Available
```

If a model's provider is not available, it's rendered with "(unavailable)" suffix in dimmed style (view.go lines 188–204).

## The Full Flow: Why OpenRouter Shows Unavailable

1. **User starts TUI**: Env vars from shell are NOT read
2. **TUI sends Init cmds**: Only `pendingAPIKeys` from persistent config are replayed (usually empty)
3. **ProvidersLoadedMsg arrives**: Server reports its own env vars and registry state
4. **TUI calls `WithAvailability(providerKeyConfigured)`**: Maps each provider → whether it's configured
5. **For OpenRouter models**: Since the TUI never loaded the env var, `providerKeyConfigured("openrouter")` returns false
6. **View renders**: All OpenRouter models show "(unavailable)" in dimmed style

## Gap: Environment Variable Not Bootstrapped

The issue is a **bootstrap gap between shell environment and TUI persistence**:
- The harness daemon sees the env var because it was started with the user's shell env
- The TUI never reads the user's shell env vars (by design—it's a separate process)
- The TUI only learns about keys via:
  1. Persistent config file (`~/.harness/config`)
  2. Server's report of its own env vars (via `/v1/providers`)
  3. Manual entry via `/keys` command

## Solution Approaches

### Option 1: TUI Reads Environment at Startup (Simplest)
Add logic to `New()` to scan environment for known provider keys:
```go
for _, provider := range []string{"openrouter", "openai", "anthropic", ...} {
    if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
        m.pendingAPIKeys["openrouter"] = key
    }
}
```
- Pro: Simple, works immediately
- Con: TUI would read all env vars, which may not be desired in all deployments

### Option 2: Server Includes Provider Keys in Response (Better)
Modify `/v1/providers` response to include an `available_via_env` flag:
```json
{
  "name": "openrouter",
  "configured": true,
  "api_key_env": "OPENROUTER_API_KEY",
  "available_via_env": true  // ← Computed from os.Getenv()
}
```
- Pro: Server authoritative, works across all UI clients
- Con: Requires API change and documentation update

### Option 3: TUI Queries Server at Startup (Current + Enhancement)
The current flow already does this via `fetchProvidersCmd()`, but it's only called on:
- `/model` command (open model switcher)
- `/keys` command (open API keys overlay)
- Not on TUI startup

**Enhancement**: Call `fetchProvidersCmd()` in `Init()` or on first `WindowSizeMsg`:
```go
// In Update(), case tea.WindowSizeMsg:
if !m.providersLoaded {
    cmds = append(cmds, fetchProvidersCmd(m.config.BaseURL))
    m.providersLoaded = true
}
```
- Pro: Keys are detected as soon as /v1/providers is queried
- Con: Relies on harness daemon being running with correct env; adds startup network call

## Current Behavior Summary

| Source | TUI Loads? | Example |
|--------|-----------|---------|
| `~/.harness/config` (persistent) | ✓ Yes | Keys saved via `/keys` command |
| `~/.zshrc` or shell env | ✗ No | `OPENROUTER_API_KEY` env var |
| Server's own env (via `/v1/providers`) | ✓ Partial | Only on `/model` or `/keys` open |
| Manual `/keys` entry | ✓ Yes | User types key in overlay |

## Recommendation

**For immediate fix**: Implement Option 1 (TUI reads environment) + document that OpenRouter keys can be pre-loaded via `~/.harness/config`.

**For long-term**: Implement Option 3 (call `fetchProvidersCmd()` at startup) to keep TUI and server state in sync without env var reading.

## Files Affected

- `cmd/harnesscli/tui/model.go` (New, Init, providerKeyConfigured)
- `cmd/harnesscli/tui/api.go` (fetchProvidersCmd, setProviderKeyCmd)
- `cmd/harnesscli/tui/messages.go` (ProvidersLoadedMsg, ProviderInfo)
- `cmd/harnesscli/tui/components/modelswitcher/model.go` (WithAvailability)
- `cmd/harnesscli/tui/components/modelswitcher/view.go` (view rendering)
- `internal/server/http.go` (handleProviders, ProviderResponse)
- `internal/provider/catalog/` (provider definitions, APIKeyEnv)
