# Issue #315 — Provider Auth Management: Research Report

**Date:** 2026-03-18
**Issue:** [#315 TUI: add provider authentication management for Codex login and API keys](https://github.com/dennisonbertram/go-agent-harness/issues/315)
**Researcher:** subagent

---

## Executive Summary

Issue #315 is **substantially pre-built**. The TUI already has a `/keys` overlay, a provider list endpoint, a runtime key injection endpoint, persistent key storage, and availability rendering in the model picker. The remaining gap is specifically the **Codex OAuth login flow**, which requires either a browser redirect or instructing the user to run an external CLI — neither of which maps to the current API-key-only auth model. Issue #313 (model availability) has already been implemented and closes the visibility gap described in #315 for API-key providers.

---

## 1. What Auth/API-Key Handling Already Exists

### 1.1 TUI — `/keys` Overlay (FULLY IMPLEMENTED)

The TUI has a complete API key management overlay accessible via `/keys`:

**File:** `cmd/harnesscli/tui/model.go`

- `/keys` slash command opens `activeOverlay = "apikeys"` (line 885)
- Two-level navigation: list mode (cursor navigation) + input mode (typing a key)
- `viewAPIKeysOverlay()` (line 1441) renders a rounded-border overlay with:
  - Provider name + env var name + configured/unconfigured status indicator
  - Arrow key navigation, Enter to enter input mode, Escape to go back
  - Input mode shows provider name, env var hint, text input with block cursor
  - Footer: `enter confirm  ctrl+u clear  esc back`
- Ctrl+U clears input, backspace removes last char, any rune accumulates
- On Enter with non-empty input: fires `setProviderKeyCmd(baseURL, provider, apiKey)` which calls `PUT /v1/providers/{provider}/key`
- On `APIKeySetMsg`: updates persistent config file and shows status bar message "Key saved for {provider}"

**Internal state in `Model`:**
```go
apiKeyProviders  []apiKeyProvider  // from ProvidersLoadedMsg
apiKeyCursor     int
apiKeyInput      string
apiKeyInputMode  bool
pendingAPIKeys   map[string]string // loaded from config, replayed on Init()
```

**Test coverage:** 14 tests in `model_apikeys_test.go` covering every interaction path.

### 1.2 TUI — Model Config Panel (ALSO IMPLEMENTED)

A second, parallel key-entry path exists in the Level-1 model config panel (`viewModelConfigPanel()`). Fields:
- `modelConfigKeyInputMode bool`
- `modelConfigKeyInput string`
- `modelConfigSection int` — tabbed navigation across gateway / API key / reasoning sections

This allows entering an API key directly while configuring a model, without opening the `/keys` overlay separately.

### 1.3 API — Fetch Providers (`GET /v1/providers`)

**File:** `cmd/harnesscli/tui/api.go` — `fetchProvidersCmd(baseURL)`

Called on TUI startup via `Init()`. Fires `ProvidersLoadedMsg` with a list of `ProviderInfo`:
```go
type ProviderInfo struct {
    Name       string
    Configured bool
    APIKeyEnv  string
}
```

**File:** `internal/server/http.go` — `handleProviders()`

Returns JSON array of `ProviderResponse{Name, Configured, APIKeyEnv, BaseURL, ModelCount}`. `Configured` is driven by `ProviderRegistry.IsConfigured(name)`, which checks either a runtime override key or `os.Getenv(entry.APIKeyEnv)`.

### 1.4 API — Set Provider Key (`PUT /v1/providers/{name}/key`)

**File:** `cmd/harnesscli/tui/api.go` — `setProviderKeyCmd(baseURL, provider, apiKey)`
**File:** `internal/server/http.go` — `handleProviderByName()`

`PUT` body: `{"key": "<value>"}`. On 204: emits `APIKeySetMsg{Provider, Key}`.

Backend handler calls `s.providerRegistry.SetAPIKey(name, body.Key)`, which calls:
```go
// internal/provider/catalog/registry.go
func (r *ProviderRegistry) SetAPIKey(provider, key string) {
    r.overrideKeys[provider] = key
    delete(r.clients, provider) // evict cached client
}
```

This is a **runtime-only in-memory override** — it is not persisted to disk on the server. The persistence happens only on the TUI client side (via `harnessconfig.Save()`).

### 1.5 Persistent CLI Config

**File:** `cmd/harnesscli/config/config.go`

```go
type Config struct {
    StarredModels []string          `json:"starred_models,omitempty"`
    Gateway       string            `json:"gateway,omitempty"`
    APIKeys       map[string]string `json:"api_keys,omitempty"`
}
```

Stored at `~/.config/harnesscli/config.json` (0600 permissions). On TUI startup, `pendingAPIKeys` is loaded from this file and replayed via `setProviderKeyCmd` on `Init()`, injecting the keys into the live server.

### 1.6 Model Availability Rendering (PART OF #313, NOW DONE)

**File:** `cmd/harnesscli/tui/components/modelswitcher/model.go`

`ModelEntry.Available bool` field was added. `WithAvailability(fn func(string) bool)` marks each model based on its provider's configured state. `WithKeyStatus(fn)` cross-references key status during rendering. Tested in `availability_test.go` with 10+ tests tagged `TestTUI313_*`.

The `ProvidersLoadedMsg` handler in `model.go` (line 1132) now calls `m.modelSwitcher.WithAvailability(...)` using the loaded provider list, greying out models whose provider is unconfigured.

---

## 2. How Provider Credentials Are Stored/Loaded Today

### Server side (`internal/config/`)

`internal/config/config.go` handles harnessd's 6-layer configuration. It does **not** contain any provider credentials — credentials are always read from environment variables. The catalog's `ProviderEntry.APIKeyEnv` field names the env var (e.g. `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`).

Runtime overrides applied via `PUT /v1/providers/{name}/key` live only in `ProviderRegistry.overrideKeys` (in-memory), surviving only as long as the harnessd process is alive. They are not written to any config file on the server side.

### Client side (`cmd/harnesscli/config/`)

`~/.config/harnesscli/config.json` stores the `api_keys` map (provider name -> key value). This is a **plaintext JSON file with mode 0600**. On TUI startup, keys from this file are re-injected into the live server via `PUT /v1/providers/{name}/key`. This creates a client-side-owned persistence model: the TUI is the durable key store; harnessd is the runtime consumer.

### Security posture (current)

- Keys stored plaintext at `~/.config/harnesscli/config.json` (0600)
- Keys transmitted over HTTP to harnessd (no transport encryption enforced)
- No OS keychain integration
- No encrypted config file
- No key rotation or expiry mechanism
- `PUT /v1/providers/{name}/key` requires `admin` scope when the auth layer is active

---

## 3. Provider Auth Requirements Per Provider

### API-key providers (all current catalog entries)

Every provider in the current catalog uses `APIKeyEnv` as its only auth mechanism:

| Provider | Env Var | Notes |
|----------|---------|-------|
| openai | OPENAI_API_KEY | Standard Bearer token |
| anthropic | ANTHROPIC_API_KEY | Standard Bearer token |
| groq | GROQ_API_KEY | Standard Bearer token |
| deepseek | DEEPSEEK_API_KEY | Standard Bearer token |
| xai | XAI_API_KEY | Standard Bearer token |
| gemini | GEMINI_API_KEY | Standard Bearer token |
| qwen | QWEN_API_KEY | Standard Bearer token |
| kimi | KIMI_API_KEY | Standard Bearer token |

**Catalog schema constraint:** `validateCatalog()` in `internal/provider/catalog/loader.go` (line 50) requires `api_key_env` to be non-empty for every provider. There is no `auth_mode` or `auth_type` field in `ProviderEntry`. OAuth-based providers cannot be represented in the current schema without a schema change.

### Codex provider

The "codex" provider as implemented in this codebase refers to **OpenAI's Codex model family** (`gpt-5.1-codex-mini`, `gpt-5.3-codex`) accessed via OpenAI's Responses API (`POST /v1/responses`). These models use the **same `OPENAI_API_KEY` Bearer token** as all other OpenAI models. There is no separate OAuth login required for these models — they are just another OpenAI model behind the same API key.

The issue's reference to "Codex login" appears to refer to the **OpenAI Codex CLI product** (`codex auth` / `codex login`) — a separate user-facing product distinct from the `gpt-5.x-codex` model family. The Codex CLI uses OAuth with a browser redirect flow; it stores tokens in a platform-specific credential store (not a simple env var).

### What "Codex login" would actually involve

If the intent is to support the OpenAI Codex CLI's auth flow:

1. **Token location:** `codex auth` stores an OAuth access token in `~/.config/codex/` (or equivalent platform config dir). The token is a short-lived OAuth bearer token, not an API key.
2. **Auth flow:** OAuth 2.0 device code flow or browser redirect. Cannot be reproduced inside a TUI without either (a) launching a browser, (b) printing a device code URL for the user to visit, or (c) telling the user to run `codex auth` externally.
3. **Token refresh:** OAuth tokens expire and require refresh. The current API-key model has no expiry.
4. **Backend changes required:** `ProviderEntry` needs an `AuthMode` field (`api_key_env` vs `oauth_codex`). `ProviderRegistry.IsConfigured()` needs to check the OAuth token store, not just env vars. `GetClient()` needs to read from the OAuth token file rather than an env var.

None of these changes exist in the codebase today.

---

## 4. Relationship to Issue #313

Issue #313 asked for model availability rendering in the TUI model picker. **It has been fully implemented:**

- `ModelEntry.Available` field added to modelswitcher
- `WithAvailability(fn)` method marks models based on provider configured state
- Model picker renders available models emphasized and unavailable ones muted
- `ProvidersLoadedMsg` handler calls `WithAvailability` using the loaded provider list
- 10+ tagged `TestTUI313_*` tests cover all branches
- The `ProviderAvailabilityPanel` component tracks provider availability for the config panel

**Does #313 partially satisfy #315?** Yes, for the visibility aspect. Users can now see which providers are configured and which are not. The gap #315 adds is the **action path**: currently users must copy-paste an API key into the `/keys` overlay (which works), but:

1. There is no first-run wizard prompting for keys when none are configured
2. There is no Codex OAuth login initiation path (not representable in current schema)
3. There is no link between the availability indicator (greyed-out model) and the `/keys` overlay (i.e. pressing Enter on a greyed-out model could open the `/keys` overlay for that provider — this is not implemented)

---

## 5. Concrete UX Proposal for In-TUI Auth Management

### State A: What already works (no new code needed)

- `/keys` opens the API keys overlay
- Provider list shows configured/unconfigured per provider
- Entering a key and pressing Enter injects it into the live server and persists it
- Model picker greys out models whose provider is unconfigured
- Keys are replayed at startup from `~/.config/harnesscli/config.json`

### State B: High-value additions (medium complexity)

**B1: Link unavailable model selection to `/keys` overlay**

When a user selects a greyed-out model in the model picker and presses Enter, show a prompt: "Provider `{name}` is not configured. Open API Keys panel? [y/n]". If yes, open the `/keys` overlay with the cursor pre-positioned on that provider.

Implementation: in the `ModelSelectedMsg` handler, check `Available` on the entry; if false, emit `OverlayOpenMsg{Kind: "apikeys"}` instead of accepting the selection, and set `apiKeyCursor` to the index of the relevant provider.

**B2: First-run empty-state in the model picker**

When `ProvidersLoadedMsg` arrives and all providers are unconfigured, render an inline prompt at the top of the model picker: "No providers configured. Type /keys to add API keys."

**B3: Status bar auth notification on startup**

After `Init()` fires `fetchProvidersCmd`, if the result shows zero configured providers, emit a `StatusMsg` like "No providers configured — use /keys to add an API key".

### State C: Codex OAuth login (high complexity, schema change required)

**C1: Schema change in `ProviderEntry`**

Add `AuthMode string` field (`"api_key_env"` or `"oauth_codex_cli"`). Update `validateCatalog()` to allow empty `APIKeyEnv` when `AuthMode` is `"oauth_codex_cli"`. Update `IsConfigured()` to check the OAuth token file for OAuth providers.

**C2: Backend token reader for Codex OAuth**

`ProviderRegistry.GetClient()` needs a path that reads from `~/.config/codex/auth.json` (or similar) rather than an env var. The token needs to be read fresh on each client creation (OAuth tokens expire).

**C3: TUI login flow for Codex**

Three options (in ascending complexity):

1. **Instruction-only (simplest):** When a Codex provider is unconfigured, render in the `/keys` overlay: "Codex uses OAuth. Run `codex auth` in a terminal, then return here to refresh." A "Refresh" action re-fetches provider state without a full restart.

2. **Device code flow (medium):** The server initiates a `POST /v1/providers/codex/login` which returns a `{verification_uri, user_code, expires_in}` response. The TUI renders the device code URL and code, polls `GET /v1/providers/codex/login/status`, and shows "Authenticated" when done.

3. **Browser redirect (complex):** Server handles the full OAuth redirect callback on a local loopback listener, stores the token, and notifies the TUI via SSE. Requires a temporary HTTP listener on the server side, cross-platform browser launching, and PKCE implementation.

**Recommendation:** Option 1 (instruction-only) is the right starting point for Codex since it requires zero schema changes to the auth flow. Options 2 and 3 are material features requiring dedicated issues.

---

## 6. Complexity Estimate and Recommended Approach

### Gap analysis

| Capability | Status | Work Needed |
|---|---|---|
| `/keys` overlay UI | DONE | None |
| Provider list with configured status | DONE | None |
| Runtime key injection (`PUT /v1/providers/{name}/key`) | DONE | None |
| Persistent key storage (client-side JSON) | DONE | None |
| Model availability rendering (#313) | DONE | None |
| Link unavailable-model → `/keys` overlay | MISSING | Small (1–2 days) |
| First-run / empty-state UX | MISSING | Small (1 day) |
| Codex OAuth login (schema change) | MISSING | Large (5–8 days) |
| OS keychain integration | MISSING | Medium (2–3 days) |
| Transport encryption (HTTPS harnessd) | MISSING | Infrastructure (separate issue) |
| Security constraint documentation | MISSING | 0.5 day |

### Recommended phased approach

**Phase 1 — Close the current UX gap (small, ~2–3 days)**

The issue is largely satisfied already for API-key providers. The concrete remaining work is:

1. When a user navigates to a greyed-out model and presses Enter, guide them to `/keys` with the cursor pre-positioned (B1 above).
2. Add a status bar notification on startup when no providers are configured (B3).
3. Write the security constraints doc (plaintext 0600 JSON is the current model; document what is not done: no keychain, no encryption, keys visible in process memory).

This closes the acceptance criteria for API-key providers without any schema changes.

**Phase 2 — Codex OAuth / instruction path (small, ~1 day)**

Add `AuthMode` field to `ProviderEntry` (optional, backward-compatible). When the `/keys` overlay encounters a provider with `auth_mode: "oauth_codex_cli"`, render an instruction screen instead of a key entry form. Check for the existence of the Codex auth token file to determine status.

This satisfies "Codex login status/action path" at a UX level without implementing the OAuth flow itself.

**Phase 3 — Device code OAuth flow (deferred, ~5–8 days)**

Implement the server-side device code flow with polling. This is a separate issue and should not block Phase 1 or 2.

---

## 7. Files of Interest

| File | Purpose |
|---|---|
| `cmd/harnesscli/tui/model.go` | Main TUI model; `/keys` overlay state + view (lines 142–151, 499–577, 712–734, 885–888, 1132–1155, 1441–1511) |
| `cmd/harnesscli/tui/api.go` | `fetchProvidersCmd`, `setProviderKeyCmd` (lines 129–178) |
| `cmd/harnesscli/tui/messages.go` | `ProviderInfo`, `ProvidersLoadedMsg`, `APIKeySetMsg` (lines 162–179) |
| `cmd/harnesscli/config/config.go` | Persistent CLI config at `~/.config/harnesscli/config.json` |
| `cmd/harnesscli/tui/components/modelswitcher/model.go` | `ModelEntry.Available`, `WithAvailability`, `WithKeyStatus` |
| `internal/server/http.go` | `handleProviders()` (line 247), `handleProviderByName()` (line 288) |
| `internal/provider/catalog/registry.go` | `SetAPIKey()`, `IsConfigured()`, `GetClient()` |
| `internal/provider/catalog/types.go` | `ProviderEntry` struct — missing `AuthMode` field |
| `internal/provider/catalog/loader.go` | `validateCatalog()` — enforces `api_key_env` non-empty |

---

## 8. Answers to the Original Research Questions

**Q1: What auth handling already exists in the TUI?**
A complete `/keys` overlay with list navigation, key input, server injection, and persistent storage. Also a second key-entry path inside the model config panel. Model availability rendering in the model picker (greyed-out providers). All tested with 14+ tests.

**Q2: How are provider credentials stored/loaded today?**
Server side: env vars only; runtime overrides in `ProviderRegistry.overrideKeys` (in-memory, not persisted). Client side: `~/.config/harnesscli/config.json` (plaintext 0600 JSON), replayed to server on startup.

**Q3: What does each provider need for auth?**
All current catalog providers use a simple API key env var. The catalog schema currently requires `api_key_env` to be non-empty for every provider.

**Q4: What is Codex auth?**
"Codex login" refers to the OpenAI Codex CLI product (not the `gpt-5.x-codex` model family). It uses OAuth 2.0 (device code or browser redirect flow), stores tokens in a platform config dir, and tokens expire. The harness's "codex" provider in the catalog is actually just `gpt-5.1-codex-mini` accessed via the OpenAI Responses API with the standard `OPENAI_API_KEY` — no separate OAuth is required for model access.

**Q5: What would in-TUI auth management look like?**
See Section 5 above. For API-key providers: link unavailable model selection to the `/keys` overlay, add empty-state prompts. For Codex OAuth: start with instruction-only UX (show token file path and `codex auth` instructions), defer device code flow to a separate issue.

**Q6: Is this still needed, or partially satisfied by #313?**
Partially satisfied. #313 closed the visibility gap (users can see which providers are configured). #315's remaining open work is: (a) the action path linking unavailable providers to the key entry flow, (b) the Codex OAuth flow, and (c) security constraint documentation.

---

*Generated: 2026-03-18*
