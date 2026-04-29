# OpenRouter + DeepSeek V4 in go-agent-harness

**Date:** 2026-04-28
**Scope:** Survey current provider plumbing, determine OpenRouter compatibility path, identify the right DeepSeek V4 slug, document concrete wiring, and list patches needed.

## TL;DR

OpenRouter is **already a first-class provider** in the harness — it has a catalog entry (`internal/provider/openai/client.go` is reused via the `openai_compat` protocol), a runtime model-discovery client (`internal/provider/catalog/discovery.go`), and routing fallback for any `vendor/model` slug (`registry.go:178`). To use **DeepSeek V4 Pro** today you only need:

1. `export OPENROUTER_API_KEY=...`
2. `export HARNESS_MODEL=deepseek/deepseek-v4-pro` (or pass `model` in the run request)

…and it will route. **However**, three concrete gaps will bite you for serious agent loops: missing `HTTP-Referer`/`X-Title` headers, no `reasoning_content` passback in `mapMessages` (which V4-Pro requires for multi-turn tool use per OpenRouter docs), and no static catalog/pricing entry for V4-Pro/V4-Flash so cost tracking falls back to "unpriced model" until live discovery merges in.

---

## 1. Survey of current provider support

### Provider directory layout

```
internal/provider/
├── anthropic/    — native Anthropic Messages API client (x-api-key, anthropic-version)
├── openai/       — OpenAI Chat Completions + Responses API client; reused for ALL openai_compat providers
├── catalog/      — registry, loader, OpenRouter live discovery, alias resolution
└── pricing/      — file-based price catalog + resolver
```

### Provider client interface

Both clients implement the `harness.Provider` interface (`internal/harness/runner.go:1497-1507`). The OpenAI client (`internal/provider/openai/client.go`) is generic — its `Config` accepts `BaseURL`, `ProviderName`, `ModelIDPrefix`, `NoParallelTools`, and `ForceNonStreaming`, which is how Gemini, Groq, DeepSeek, and OpenRouter all reuse it.

### Catalog & registry

- `catalog/models.json` (top-level) — static catalog declaring providers, base URLs, env-var name, models, aliases, pricing.
- `internal/provider/catalog/registry.go` — the `ProviderRegistry`:
  - `ResolveProvider(modelID)` — maps a model ID to a provider; checks (a) static catalog by exact ID, (b) static aliases, (c) live OpenRouter discovery, (d) **falls back to `openrouter` for ANY slug containing a `/`** (line 178-183).
  - `GetClient(providerName)` — lazily constructs a client via the factory set in `cmd/harnessd/bootstrap_helpers.go:114-138`.
  - `MaxContextTokens(modelID)` — used by runner for context-window guard (`runner.go:2440`).
- `internal/provider/catalog/discovery.go` — `OpenRouterDiscovery`. Hits `GET https://openrouter.ai/api/v1/models`, caches 5 min, returns `{ID, Name, ContextWindow}`. Wired in `bootstrap_helpers.go:108-113` only if the catalog declares an `openrouter` provider.

### Currently registered providers (from `catalog/models.json`)

| Provider     | base_url                                                  | api_key_env         | Protocol      | Notes                                                     |
| ------------ | --------------------------------------------------------- | ------------------- | ------------- | --------------------------------------------------------- |
| `openai`     | `https://api.openai.com/v1`                               | `OPENAI_API_KEY`    | `openai`      | Some models routed to `/v1/responses` via `api: responses` field. |
| `anthropic`  | `https://api.anthropic.com`                               | `ANTHROPIC_API_KEY` | native        | Native client, separate code path.                        |
| `deepseek`   | `https://api.deepseek.com/v1`                             | `DEEPSEEK_API_KEY`  | `openai_compat` | Quirk: `reasoning_content_passback`. Models: `deepseek-chat`, `deepseek-reasoner`. |
| `groq`       | `https://api.groq.com/openai/v1`                          | `GROQ_API_KEY`      | `openai_compat` |                                                           |
| `openrouter` | `https://openrouter.ai/api/v1`                            | `OPENROUTER_API_KEY`| `openai_compat` | Live model discovery. Static catalog has only `openai/gpt-4.1-mini`. |
| `gemini`     | `https://generativelanguage.googleapis.com/v1beta/openai` | `GOOGLE_API_KEY`    | OpenAI-compat | Quirks: `no_parallel_tool_calls`, `models/` prefix.       |

### Auth / headers / streaming summary (OpenAI-compat path)

- **Auth:** `Authorization: Bearer <apiKey>` (`client.go:146`, `:981`).
- **Other headers:** only `Content-Type: application/json`. **No `HTTP-Referer`, no `X-Title`.**
- **Endpoints:** `<baseURL>/v1/chat/completions` (Chat) and `<baseURL>/v1/responses` (Responses, opt-in via catalog `api: responses`). Note `BaseURL` is normalized by stripping a trailing `/v1` (`client.go:75-76`), so a catalog value of `https://openrouter.ai/api/v1` works correctly.
- **Tool-call format:** standard OpenAI Chat Completions — `tools: [{type: "function", function: {name, description, parameters}}]`, `tool_choice: "auto"`. Responses are decoded as `choices[0].message.tool_calls[]`.
- **Streaming:** SSE; processes `data:` lines, accumulates content, `reasoning_content`, and `tool_calls` deltas (`client.go:407-481`). Handles `[DONE]`. Includes `stream_options.include_usage: true` so usage tokens land on the final chunk.
- **Cost tracking:** `pricing.Resolver.Resolve(providerName, model)` — keyed by `(provider, model)`. If the resolver returns no entry, status becomes `CostStatusUnpricedModel` and `CostUSD` is 0. OpenRouter responses can also carry an explicit `cost_usd` field which `explicitCostUSD` (line 600-608) honors if present.

---

## 2. OpenRouter compatibility path

**Verdict: reuse the existing OpenAI client. No new provider package needed.** The catalog already declares `openrouter` with `protocol: openai_compat` (line 691 of `catalog/models.json`). Configuration in `bootstrap_helpers.go:114-138` will instantiate the OpenAI client for any provider whose name is not `anthropic`.

### How a model ID gets routed to OpenRouter

1. User passes `model: "deepseek/deepseek-v4-pro"` in run request (or sets `HARNESS_MODEL`).
2. `runner.go:1484` calls `providerRegistry.GetClientForModel(model)`.
3. `ResolveProviderContext` (`registry.go:164`):
   - Not in any provider's static `Models` map → no static hit.
   - Calls `hasOpenRouterDiscoveredModel` → fetches live `/api/v1/models`. If `deepseek/deepseek-v4-pro` is in the live list → returns `"openrouter"`.
   - Else, the `strings.Contains(modelID, "/")` fallback (line 178) **still routes to `openrouter`** as long as the provider entry exists.
4. Registry constructs the OpenAI client with `BaseURL=https://openrouter.ai/api/v1`, `ProviderName="openrouter"`.
5. Request goes to `https://openrouter.ai/api/v1/chat/completions` with `Authorization: Bearer $OPENROUTER_API_KEY`.

### Gotchas

| Concern                              | Status                                                                                              |
| ------------------------------------ | --------------------------------------------------------------------------------------------------- |
| Auth header format                   | OK — `Authorization: Bearer <key>`. Same as OpenAI.                                                 |
| Base URL                             | OK — catalog has `/v1`; client's normalization in `client.go:75-76` is idempotent.                  |
| Model id namespacing                 | OK — slug like `deepseek/deepseek-v4-pro` is sent verbatim.                                         |
| **`HTTP-Referer` header**            | **NOT SET.** Optional but recommended (analytics/leaderboards); not strictly required.              |
| **`X-Title` / `X-OpenRouter-Title`** | **NOT SET.** Optional.                                                                              |
| Tool-call format                     | OK — standard OpenAI shape; OpenRouter normalizes across providers.                                 |
| Streaming format                     | OK — SSE, OpenAI-style chunks.                                                                      |
| Reasoning passback for multi-turn    | **BROKEN for V4-Pro** — see Gaps section.                                                           |
| Cost / usage                         | Partial — OpenRouter sometimes includes `usage.cost_usd` which the client honors.                   |

Sources:
- [OpenRouter API quickstart](https://openrouter.ai/docs/quickstart) — confirms `https://openrouter.ai/api/v1/chat/completions`, `Bearer` auth, optional `HTTP-Referer`/`X-OpenRouter-Title` headers.
- [OpenRouter docs overview](https://openrouter.ai/docs) — confirms OpenAI-compatible endpoints, `:thinking`/`:nitro`/`:free` model variants.

---

## 3. DeepSeek V4 specifics

OpenRouter publishes two V4 SKUs in the DeepSeek lineup as of April 2026:

### `deepseek/deepseek-v4-pro`
- **Slug:** `deepseek/deepseek-v4-pro`
- **Context window:** 1,048,576 tokens (1 M).
- **Pricing:** $0.435 / 1 M input · $0.87 / 1 M output.
- **Reasoning:** Supports `reasoning.effort: "high" | "xhigh"`. `reasoning_details` array is returned. **For multi-turn tool use, `reasoning_details` from prior assistant turns must be passed back** (per OpenRouter's V4-Pro page: *"when continuing a conversation, preserve the complete `reasoning_details`"*).
- **Tool calling:** Yes (per [DeepSeek API and Models on OpenRouter](https://openrouter.ai/deepseek)).
- **Quirk:** A bug filed against an unrelated client confirms: *"The reasoning_content in the thinking mode must be passed back to the API"* on follow-up tool-call turns ([openclaw#71160](https://github.com/openclaw/openclaw/issues/71160)).

### `deepseek/deepseek-v4-flash`
- **Slug:** `deepseek/deepseek-v4-flash`
- **Context window:** 1,048,576 tokens (1 M).
- **Pricing:** $0.14 / 1 M input · $0.28 / 1 M output.
- **Reasoning:** Same `high`/`xhigh` effort knobs.
- **Tool calling:** Per the DeepSeek family card on OpenRouter, V4 SKUs support tool use; V4-Flash is positioned for cheap, fast agent loops.

Sources:
- [DeepSeek V4 Pro · OpenRouter](https://openrouter.ai/deepseek/deepseek-v4-pro)
- [DeepSeek V4 Flash · OpenRouter](https://openrouter.ai/deepseek/deepseek-v4-flash)
- [DeepSeek API and Models · OpenRouter](https://openrouter.ai/deepseek)
- [Simon Willison: DeepSeek V4 review](https://simonwillison.net/2026/apr/24/deepseek-v4/)

---

## 4. Concrete wiring

### Where to set the model

Order of precedence (later overrides earlier — `internal/config/config.go:393-545`):

1. **Defaults:** `cfg.Model = "gpt-4.1-mini"` (`config.go:197`).
2. **Project TOML** (`harness.toml`): top-level `model = "..."` (`config.go:158`).
3. **Profile TOML** (`prompts/<profile>.toml`): `model = "..."` (`config.go:253`, applied by `applyProfileLayer`).
4. **Env var:** `HARNESS_MODEL=...` (`config.go:552-554`).
5. **Per-request override:** `RunRequest.Model` in the `/v1/runs` JSON body — passed straight through to `harness.CompletionRequest.Model`.

### Where the API key is read

- Catalog declares `api_key_env: "OPENROUTER_API_KEY"` (`catalog/models.json:690`).
- Registry reads it via the env hook in `registry.go:122-127`.
- A runtime override is also supported via `ProviderRegistry.SetAPIKey("openrouter", "...")` (`registry.go:69-78`) — the CLI uses this for its login flow.

### Where the client is built

`cmd/harnessd/bootstrap_helpers.go:114-138` — the `ClientFactory`. Anthropic gets its native client; everything else (including `openrouter`) gets `openai.NewClient(openai.Config{...})` with the provider's `BaseURL` from the catalog.

### Where context-window enforcement runs

`internal/harness/runner.go:2424-2440` — calls `providerRegistry.MaxContextTokens(model)`. Live OpenRouter discovery feeds this via `effectiveCatalog` (`registry.go:283-302`), so V4-Pro's 1 M context is correctly recognized once the live `/models` fetch returns.

### How to use it today

```bash
# 1. Auth.
export OPENROUTER_API_KEY=sk-or-v1-...

# 2. Pick model. Pro for hard work, Flash for cheap agent loops.
export HARNESS_MODEL=deepseek/deepseek-v4-pro
# or: export HARNESS_MODEL=deepseek/deepseek-v4-flash

# 3. Start harnessd as usual.
./bin/harnessd

# 4. Smoke test via CLI.
./bin/harnesscli run --message "Write a Go function to merge sorted slices."
```

Per-request override (no env change needed):

```bash
curl -N http://localhost:8080/v1/runs \
  -H 'Content-Type: application/json' \
  -d '{
    "messages": [{"role":"user","content":"hello"}],
    "model": "deepseek/deepseek-v4-flash"
  }'
```

Profile snippet (`prompts/profiles/cheap-agent.toml`):

```toml
model = "deepseek/deepseek-v4-flash"
max_steps = 40
```

---

## 5. What needs patching

### Punch list

1. **Add static catalog entries for `deepseek/deepseek-v4-pro` and `deepseek/deepseek-v4-flash`.**
   - File: `catalog/models.json`, under the `openrouter` provider's `models` map.
   - Why: Today, DeepSeek V4 routes via the `strings.Contains(modelID, "/")` fallback (`registry.go:178`). That works for routing but yields no static metadata. With static entries you get reliable `MaxContextTokens` (1 M), accurate `display_name`, and pricing without depending on a live discovery hit.
   - Suggested entries (1 M context, prices above, `tool_calling: true`, `streaming: true`, `reasoning_mode: true`).

2. **Wire pricing.**
   - File: pricing catalog (path set by `HARNESS_PRICING_CATALOG_PATH`); see `pricing/catalog.go` and `pricing/resolver.go`.
   - Without an entry, `computeCost` returns `CostStatusUnpricedModel` (`client.go:580-582`). OpenRouter does sometimes echo `usage.cost_usd` and `explicitCostUSD` (line 600) honors it, but treat that as a fallback, not the source of truth.

3. **Set OpenRouter recommended headers.** (Not required, but expected.)
   - File: `internal/provider/openai/client.go`, both at line ~146 (chat-completions) and ~981 (responses).
   - Add: `HTTP-Referer` and `X-Title` (or `X-OpenRouter-Title`) — only when `c.providerName == "openrouter"`. Pull values from new `Config` fields wired through `bootstrap_helpers.go` (e.g. read `OPENROUTER_HTTP_REFERER` and `OPENROUTER_APP_TITLE` from env).

4. **Pass `reasoning_content` / `reasoning_details` back to the provider on follow-up turns.** (Critical for V4-Pro multi-turn tool use.)
   - File: `internal/provider/openai/client.go`, function `mapMessages` (line 624-651).
   - Today: only `Role`, `Content`, `ToolCalls`, `ToolCallID`, `Name` are forwarded. The `harness.Message.Reasoning` field (`internal/harness/types.go:72`) is **dropped**.
   - Required: when `providerName == "openrouter"` (and possibly `deepseek` too — note the existing `reasoning_content_passback` quirk in the deepseek catalog entry but the absence of any code that honors it), include the prior assistant turn's reasoning back to the API. OpenRouter expects either `reasoning_content` (legacy DeepSeek-style) or a structured `reasoning_details` array (V4-Pro). Failing to do so produces *"The reasoning_content in the thinking mode must be passed back to the API"* on the second tool-call turn.
   - Verify: this is already a known DeepSeek quirk in the catalog (`quirks: ["reasoning_content_passback"]`, line 219) but no code branches on it. The right fix is to consume `cfg.Quirks` in the OpenAI client and pass `Reasoning` back when the quirk is present.

5. **Expose `reasoning.effort` per request.**
   - The Chat Completions client already serializes `ReasoningEffort` (`client.go:312`), and `harness.CompletionRequest.ReasoningEffort` already plumbs through. **However**, OpenRouter expects `reasoning: { effort: "high" }` as a nested object on its enhanced endpoints, not the OpenAI-style flat `reasoning_effort` string. Audit whether OpenRouter accepts the OpenAI flat form for DeepSeek V4 — if not, add a per-provider serialization branch.

6. **Pricing resolver double-keying.**
   - Pricing is resolved by `(providerName, model)` (`client.go:579`). For OpenRouter, the model id is `deepseek/deepseek-v4-pro` — make sure pricing entries are keyed under provider `openrouter`, not `deepseek`. The existing `deepseek` static entries (lines 248-253) are for the **direct DeepSeek API** and won't apply to OpenRouter routing.

7. **No hardcoded provider checks block this.** I grep'd: aside from the special-case branch for `anthropic` in the client factory and the `gemini`-specific `NoParallelTools`/`ModelIDPrefix` flags, nothing assumes a closed set of providers. The `openrouter` fallback in `registry.go:178` is intentional and correct.

8. **TUI alias map.**
   - `cmd/harnesscli/tui/components/modelswitcher/model.go:727-728` aliases `deepseek-chat` → `deepseek/deepseek-chat`. Add aliases `deepseek-v4` → `deepseek/deepseek-v4-pro` and `deepseek-v4-flash` → `deepseek/deepseek-v4-flash` if you want shortcut model picks in the TUI.

### Risk-ordered shortlist for someone landing this in a single PR

1. **(blocker for tool loops)** Patch `mapMessages` to forward reasoning when the provider quirk is set.
2. **(correctness)** Add static catalog + pricing entries for V4-Pro and V4-Flash under `openrouter`.
3. **(polish)** Send `HTTP-Referer` / `X-Title` headers when provider is `openrouter`.
4. **(verification)** Add an integration test that round-trips a tool call through OpenRouter against `deepseek/deepseek-v4-flash` (cheapest SKU).

---

## Sources

- [OpenRouter docs overview](https://openrouter.ai/docs)
- [OpenRouter quickstart (curl)](https://openrouter.ai/docs/quickstart)
- [DeepSeek V4 Pro · OpenRouter](https://openrouter.ai/deepseek/deepseek-v4-pro)
- [DeepSeek V4 Flash · OpenRouter](https://openrouter.ai/deepseek/deepseek-v4-flash)
- [DeepSeek API and Models · OpenRouter](https://openrouter.ai/deepseek)
- [DeepSeek API Docs](https://api-docs.deepseek.com/)
- [Simon Willison — DeepSeek V4 review](https://simonwillison.net/2026/apr/24/deepseek-v4/)
- [openclaw#71160 — DeepSeek V4 Pro reasoning_content passback bug](https://github.com/openclaw/openclaw/issues/71160)
