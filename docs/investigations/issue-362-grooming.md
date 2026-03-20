# Issue #362 — Harness: Remove the OpenAI-Only Bootstrap Requirement from Server Startup

**Date:** 2026-03-19  
**Status:** Ready for Implementation  
**Severity:** Medium (Enabler, not blocker — workarounds exist)

---

## Executive Summary

Issue #362 is **currently not addressed**. The server requires `OPENAI_API_KEY` at startup (lines 163–166 in `cmd/harnessd/main.go`), even though:

1. The Runner has provider-registry-based resolution (via `providerRegistry` field, wired at construction time)
2. The provider catalog system supports multi-provider routing per-run
3. Per-run `provider_name` routing is implemented in the runner's `StartRun` handler

**The fix is atomic**: Move the OPENAI_API_KEY check from startup to first-run time, conditioned on whether a provider catalog is loaded. If a catalog is available, defer the check until a model is requested; if no catalog, require OpenAI as the fallback for backward compatibility.

---

## 1. Current Evidence of the Problem

### 1.1 Hard OPENAI_API_KEY Check at Startup

**File:** `cmd/harnessd/main.go`, lines 163–166

```go
apiKey := getenv("OPENAI_API_KEY")
if apiKey == "" {
    return fmt.Errorf("OPENAI_API_KEY is required")
}
```

This check happens **before** the config cascade (layers 1–5) is resolved and **before** the provider catalog is loaded. It blocks startup entirely if the env var is absent.

### 1.2 Default OpenAI Provider is Always Constructed

**File:** `cmd/harnessd/main.go`, lines 323–332

```go
provider, err := newProvider(openai.Config{
    APIKey:          apiKey,
    BaseURL:         getenv("OPENAI_BASE_URL"),
    Model:           model,
    PricingResolver: pricingResolver,
    ModelAPILookup:  lookupModelAPI,
})
if err != nil {
    return fmt.Errorf("create openai provider: %w", err)
}
```

This always creates an OpenAI provider as the "default provider" for the runner, **even if the provider catalog is loaded and contains multiple providers** (e.g., Anthropic, Gemini, etc.).

### 1.3 Observational Memory Also Requires OPENAI_API_KEY

**File:** `cmd/harnessd/main.go`, line 214–217

```go
memoryLLMMode := strings.TrimSpace(strings.ToLower(envOrDefault("HARNESS_MEMORY_LLM_MODE", "openai")))
memoryLLMModel := strings.TrimSpace(envOrDefault("HARNESS_MEMORY_LLM_MODEL", "gpt-5-nano"))
memoryLLMBaseURL := strings.TrimSpace(envOrDefault("HARNESS_MEMORY_LLM_BASE_URL", getenv("OPENAI_BASE_URL")))
memoryLLMAPIKey := strings.TrimSpace(envOrDefault("HARNESS_MEMORY_LLM_API_KEY", apiKey))  // <-- fallback to apiKey
```

The observational memory defaults to OpenAI mode and reuses `apiKey` as a fallback. This creates a secondary hard dependency.

---

## 2. Provider Registry & Multi-Provider Support Already Exists

The infrastructure to support optional providers is **already wired**:

### 2.1 Provider Catalog & Registry

**File:** `internal/provider/catalog/registry.go` — `ProviderRegistry`

- `IsConfigured(providerName string) bool` — checks runtime override or env var key
- `GetClient(providerName string)` — lazy-creates clients with factory pattern
- `GetClientForModel(modelID string)` — searches all providers to find which has the model
- `ResolveProvider(modelID string) (string, bool)` — returns provider name for a model ID

### 2.2 Runner Has Provider Registry Field

**File:** `internal/harness/runner.go`, line 226

```go
providerRegistry *catalog.ProviderRegistry
```

The runner is **already configured** with the provider registry (line 312 in main.go, and passed via `RunnerConfig.ProviderRegistry`).

### 2.3 Per-Run Provider Routing Works

In the runner's `StartRun` handler, models are routed to the correct provider via the registry:

```go
// Lines 1107–1129 in runner.go
if r.providerRegistry == nil {
    // No registry: use default provider
    client = r.provider
} else {
    // Registry exists: try to find client for the model
    client, err := r.providerRegistry.GetClient(preferredProvider)
    // OR
    client, providerName, err := r.providerRegistry.GetClientForModel(model)
}
```

This means **the runner can function without a default OpenAI provider, as long as the catalog routes the requested model to an available provider**.

---

## 3. Acceptance Criteria (Clarity Assessment)

### 3.1 Requirement A: Optional OPENAI_API_KEY at Startup

**Current state:** Hard-required  
**Desired state:** Required only if:
- No provider catalog is configured, OR
- The catalog is loaded but contains no providers, OR
- The user's first request uses a model that requires OpenAI and no other provider is configured

**Acceptance:** Server starts successfully with no OPENAI_API_KEY set, as long as a provider catalog with at least one configured provider is loaded.

**Evidence of success:**
```bash
# Without OPENAI_API_KEY, without catalog → startup fails (backward compat)
$ unset OPENAI_API_KEY
$ harnessd
# Error: OPENAI_API_KEY is required (or similar)

# Without OPENAI_API_KEY, with catalog → startup succeeds
$ unset OPENAI_API_KEY
$ HARNESS_MODEL_CATALOG_PATH=catalog/models.json harnessd
# Server starts listening on...

# First run with Anthropic model → succeeds (if ANTHROPIC_API_KEY is set)
$ curl -X POST http://localhost:8080/v1/runs \
  -d '{"prompt":"hello","model":"claude-3-opus"}'
# {"run_id":"...", ...}
```

### 3.2 Requirement B: Observational Memory Should Not Require OPENAI_API_KEY

**Current state:** Defaults to OpenAI mode, requires OPENAI_API_KEY for memory reflection  
**Desired state:**
- If `HARNESS_MEMORY_LLM_MODE=inherit`, use the default provider's model (no separate key needed)
- If `HARNESS_MEMORY_LLM_MODE=openai`, require OPENAI_API_KEY (or allow fallback to provider catalog's OpenAI entry)
- If memory is disabled, no API key needed for memory (only for the runner's provider)

**Acceptance:** Memory can be configured without OPENAI_API_KEY if `HARNESS_MEMORY_LLM_MODE=inherit` or memory is disabled.

---

## 4. Code Changes Required (Scope)

### 4.1 Main Change: Defer OPENAI_API_KEY Check

**File:** `cmd/harnessd/main.go`

**Lines 163–166:** Remove or convert the hard check.

```go
// OLD (lines 163–166):
apiKey := getenv("OPENAI_API_KEY")
if apiKey == "" {
    return fmt.Errorf("OPENAI_API_KEY is required")
}

// NEW:
// Defer the check; allow empty for now if provider catalog is configured.
apiKey := getenv("OPENAI_API_KEY")
// (no immediate error)
```

### 4.2 Conditional Default Provider Creation

**File:** `cmd/harnessd/main.go`

**Lines 323–332:** Create default OpenAI provider only if it will be used.

```go
// OLD: always create
provider, err := newProvider(openai.Config{ ... })

// NEW: create only if:
// - No provider catalog is loaded, OR
// - OPENAI_API_KEY is set (backward compat)
var provider harness.Provider
if modelCatalog == nil && apiKey == "" {
    return fmt.Errorf("OPENAI_API_KEY is required (or load a provider catalog)")
}
if modelCatalog == nil || apiKey != "" {
    // Fallback to OpenAI for backward compat
    provider, err := newProvider(openai.Config{ ... })
    if err != nil {
        return fmt.Errorf("create openai provider: %w", err)
    }
} else {
    // Create a nil provider; per-run routing via providerRegistry will handle it
    provider = nil
}
```

### 4.3 Update Memory LLM Defaults

**File:** `cmd/harnessd/main.go`

**Lines 214–217:** Make memory LLM mode default to "inherit" if no OPENAI_API_KEY.

```go
// OLD:
memoryLLMMode := strings.TrimSpace(strings.ToLower(envOrDefault("HARNESS_MEMORY_LLM_MODE", "openai")))

// NEW:
defaultMemoryMode := "inherit" // or "openai" if apiKey != ""
if apiKey != "" {
    defaultMemoryMode = "openai"
}
memoryLLMMode := strings.TrimSpace(strings.ToLower(envOrDefault("HARNESS_MEMORY_LLM_MODE", defaultMemoryMode)))
```

### 4.4 Handle Nil Default Provider in Runner

**File:** `internal/harness/runner.go` (if necessary)

If the default provider is nil, ensure the runner can still function via the provider registry for per-run routing. A code review is needed to confirm the runner's `Complete()` method doesn't assume a non-nil default provider.

---

## 5. Testing & Validation

### 5.1 New Test Cases

**File:** `cmd/harnessd/main_test.go`

Add tests:

```go
// Test 1: Server starts with no OPENAI_API_KEY if provider catalog is loaded
TestRunWithoutOpenAIKeyButCatalogLoaded()

// Test 2: Server fails without OPENAI_API_KEY if no catalog is loaded (backward compat)
TestRunFailsWithoutOpenAIKeyAndNoCatalog()

// Test 3: Memory defaults to "inherit" if OPENAI_API_KEY is absent
TestMemoryDefaultsToInheritWithoutOpenAIKey()

// Test 4: Per-run model routing works without default OpenAI provider
TestPerRunProviderRoutingWithoutDefaultProvider()
```

### 5.2 Manual Testing

```bash
# Test 1: No key, no catalog → fail at startup
$ unset OPENAI_API_KEY HARNESS_MODEL_CATALOG_PATH
$ harnessd 2>&1 | grep -i "OPENAI_API_KEY is required"

# Test 2: No key, with catalog → start successfully
$ unset OPENAI_API_KEY
$ HARNESS_MODEL_CATALOG_PATH=catalog/models.json harnessd
# → "harness server listening on ..."

# Test 3: First run with valid model → succeeds (if provider is configured)
$ ANTHROPIC_API_KEY=sk-ant-... curl -X POST http://localhost:8080/v1/runs \
  -d '{"prompt":"hello","model":"claude-3-opus"}'
# → {"run_id":"...", ...}
```

---

## 6. Blockers & Dependencies

### 6.1 No Hard Blockers

This change has **no blocking dependencies**:
- Provider registry is already wired ✓
- Multi-provider routing works ✓
- Model catalog loading is optional ✓

### 6.2 Soft Dependency: Runner Nil-Provider Handling

**Risk:** The runner's default provider field might be used in places that assume non-nil.

**Mitigation:** Code review `internal/harness/runner.go` to confirm:
1. The default provider is only used as a fallback when no provider registry or model is specified
2. Per-run requests that specify a model or provider use the registry

**Effort:** Low (likely no changes needed; the registry check is already in place).

---

## 7. Effort & Complexity Assessment

| Aspect | Estimate | Notes |
|--------|----------|-------|
| Code changes in main.go | 1–2 hours | ~20 lines; conditional logic is straightforward |
| Runner validation | 30 min | Code review; likely no changes needed |
| Unit tests | 1–2 hours | 4–5 test cases covering startup paths |
| Integration tests | 1 hour | Manual verification with catalog + providers |
| Documentation updates | 30 min | Update runbooks and CLAUDE.md provider notes |
| **Total** | **4–5 hours** | Atomic, low-risk change |

### Recommended Labels

- **Priority:** `medium` (Enabler for multi-provider deployment; not a blocker)
- **Size:** `small` (Atomic change confined to startup logic)
- **Status:** `well-specified` (All requirements and code paths are clear)
- **Type:** `enhancement` (Removes unnecessary coupling to OpenAI)

---

## 8. Risk Assessment

### 8.1 Backward Compatibility

**Risk:** Low  
**Mitigation:**
- If no catalog is loaded AND no OPENAI_API_KEY → fail at startup (same as today)
- If OPENAI_API_KEY is set → default provider is created (same as today)
- Existing deployments are unaffected

### 8.2 Observational Memory Breakage

**Risk:** Low  
**Mitigation:**
- Default to "inherit" mode when OPENAI_API_KEY is absent
- "Inherit" mode uses the runner's primary provider, so no extra API key is needed
- Existing configs with explicit `HARNESS_MEMORY_LLM_MODE=openai` continue to work

### 8.3 Per-Run Provider Routing

**Risk:** Very Low  
**Evidence:** Provider registry routing is already tested in `internal/provider/catalog/registry_test.go` and used in production for the TUI (`issue-315-provider-auth.md`).

---

## 9. Recommended Next Steps

1. **Code review** of `internal/harness/runner.go` to confirm nil-provider handling (15 min)
2. **Implement changes** in `cmd/harnessd/main.go` (1–2 hours)
3. **Add unit tests** in `cmd/harnessd/main_test.go` (1–2 hours)
4. **Manual testing** with a model catalog and multiple providers (30 min)
5. **Documentation update** (30 min)
6. **Create PR** and request review

---

## 10. Summary Table

| Criterion | Status | Notes |
|-----------|--------|-------|
| **Already addressed?** | No | Hard requirement still in place at lines 163–166 |
| **Clear?** | Yes | Root cause identified; fix path is straightforward |
| **Acceptance criteria complete?** | Yes | Two requirements: defer key check + memory LLM mode defaults |
| **Atomic?** | Yes | Single responsibility: remove OpenAI hard coupling at startup |
| **Blockers?** | None | Provider registry already wired; no dependencies |
| **Effort** | 4–5 hours | Small change; well-understood scope |
| **Recommended label** | small + well-specified | Ready for implementation |

