# Issue #315 — Provider Auth TUI: Implementation Summary

**Date:** 2026-03-18
**Issue:** [#315 TUI: add provider authentication management](https://github.com/dennisonbertram/go-agent-harness/issues/315)
**Commit:** `feat(#315): provider auth TUI — unavailable model → /keys flow, empty-state prompt, codex instructions`

---

## What Was Built

Three targeted UX gaps were identified from the research report (`docs/investigations/issue-315-provider-auth-research.md`) and implemented using TDD.

### Gap 1: Greyed-out model → open /keys overlay pre-positioned on that provider

**Location:** `cmd/harnesscli/tui/model.go` (Level-0 model overlay Enter handler)

When the user presses Enter on a model that is marked `Available=false` (provider unconfigured, from #313), instead of entering the config panel, the TUI now:
1. Closes the model overlay
2. Opens the `/keys` overlay (`activeOverlay = "apikeys"`)
3. Pre-positions the cursor on the provider for that model (e.g. selecting `gpt-4.1` when OpenAI is unconfigured positions the cursor on `openai`)

**Key guard:** Only redirects when `m.modelSwitcher.AvailabilityKnown()` is true (i.e. `ProvidersLoadedMsg` has been received). Before provider info loads, `Available` defaults to `false` for all models — the guard ensures we don't redirect during the loading window.

**New method added to modelswitcher:** `AvailabilityKnown() bool` — exposes the `availabilitySet` field so the parent model can distinguish "not yet loaded" from "confirmed unconfigured".

**Helper added to model.go:**
- `providerIndexInAPIKeyList(providerName string) int` — finds the cursor index for a named provider in `apiKeyProviders`
- `APIKeyCursor() int` — testing accessor for the cursor position
- `ModelSwitcher() modelswitcher.Model` — testing accessor for the model switcher state

### Gap 2: Empty-state status bar prompt

**Location:** `cmd/harnesscli/tui/model.go` (`ProvidersLoadedMsg` handler)

When `ProvidersLoadedMsg` arrives with all providers having `Configured=false`, a hint is shown in the status bar:

```
No providers configured — press / then keys to add API keys
```

The hint is only shown when:
- The providers list is non-empty (if the server returns nothing, we don't know if that's "really unconfigured" or "no providers loaded yet")
- Every provider in the list has `Configured=false`

The hint auto-dismisses after `statusMsgDuration` (3 seconds). Once a key is saved and `ProvidersLoadedMsg` is re-received with at least one configured provider, no hint is shown.

### Gap 3: Codex auth instruction path

**Location:** `cmd/harnesscli/tui/model.go` (`ModelSelectedMsg` handler)

When the user selects any model whose ID contains "codex" while the OpenAI provider is not configured, the status bar shows:

```
Codex uses your OpenAI API key. Set OPENAI_API_KEY or enter it via /keys.
```

This is shown instead of the normal "Model: ..." status message. When OpenAI is configured, or when a non-codex model is selected, the normal status message is shown.

**Helper added:** `isCodexModel(modelID string) bool` — returns true when `strings.Contains(strings.ToLower(modelID), "codex")`.

---

## Test Coverage

**Test file:** `cmd/harnesscli/tui/model_315_test.go`

9 new tests, all passing:

| Test | Gap | Purpose |
|---|---|---|
| `TestTUI315_UnavailableModelEnterOpensKeysOverlay` | 1 | Enter on unavailable model opens /keys |
| `TestTUI315_UnavailableModelKeysOverlayPrePositioned` | 1 | Cursor is pre-positioned on the provider |
| `TestTUI315_AvailableModelEnterOpensConfigPanel` | 1 | Available model still opens config panel |
| `TestTUI315_EmptyStateHintWhenNoProvidersConfigured` | 2 | All-unconfigured → hint in status bar |
| `TestTUI315_NoEmptyStateHintWhenProviderConfigured` | 2 | At least one configured → no hint |
| `TestTUI315_NoEmptyStateHintWhenProvidersListEmpty` | 2 | Empty providers list → no hint |
| `TestTUI315_CodexModelUnconfiguredShowsInstruction` | 3 | Codex + unconfigured → special message |
| `TestTUI315_CodexModelConfiguredDoesNotShowInstruction` | 3 | Codex + configured → normal message |
| `TestTUI315_NonCodexUnconfiguredModelDoesNotShowCodexInstruction` | 3 | Non-codex → no codex message |

---

## Files Changed

| File | Change |
|---|---|
| `cmd/harnesscli/tui/model.go` | Gap 1, 2, 3 implementation + accessor methods + helper functions |
| `cmd/harnesscli/tui/components/modelswitcher/model.go` | Added `AvailabilityKnown() bool` accessor |
| `cmd/harnesscli/tui/model_315_test.go` | New — 9 tests for all three gaps |

---

## What Was Not Implemented

Per the research report's recommendations, the following remain deferred:

- **Codex OAuth login flow** (device code or browser redirect) — requires schema change to `ProviderEntry` to add `AuthMode` field. Tracked separately.
- **OS keychain integration** — keys are still stored plaintext at `~/.config/harnesscli/config.json` (0600). Security constraint documentation omitted per scope.
- **Transport encryption** (HTTPS for harnessd) — infrastructure concern, separate issue.

---

*Generated: 2026-03-18*
