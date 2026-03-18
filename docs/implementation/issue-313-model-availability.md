# Issue #313: TUI Model Availability from Provider Configuration

## Summary

The TUI model switcher now visually distinguishes available vs unavailable models by
reading provider configuration state from the backend's `GET /v1/providers` endpoint.

## Problem

The model switcher rendered all models identically regardless of whether their provider
was configured. A user selecting an unconfigured model (e.g. DeepSeek when no
`DEEPSEEK_API_KEY` is set) would only discover the issue after submitting a prompt and
receiving a backend error.

## Solution

### New Field: `ModelEntry.Available bool`

Added to `cmd/harnesscli/tui/components/modelswitcher/model.go`:

```go
type ModelEntry struct {
    // ... existing fields ...
    // Available indicates whether this model's provider is currently configured.
    // Zero value (false) means "unknown" — no availability info loaded yet.
    Available bool
}
```

Zero value semantics: `Available=false` with `availabilitySet=false` means "unknown"
and no indicator is shown. This preserves backwards compatibility when the provider
fetch fails (the `DefaultModels` fallback shows no unavailability indicators).

### New Method: `Model.WithAvailability(fn func(string) bool) Model`

```go
func (m Model) WithAvailability(fn func(string) bool) Model
```

- Accepts a provider-check function (same signature as `WithKeyStatus`)
- Marks each `ModelEntry.Available` based on `fn(entry.Provider)`
- Stores the function so a subsequent `WithModels` call (server fetch) re-applies
  availability to the new model list
- Passing `nil` leaves all models with `Available=false`
- Value semantics: does not mutate the receiver

### View Rendering

In `viewModelList`:
- Available models render with the existing bright/normal style (no change)
- Unavailable models (when `availabilitySet=true && entry.Available==false`) render:
  - The display name in a muted/greyed style (`unavailableStyle`)
  - A `(unavailable)` suffix in a dimmed style (`unavailableSuffixStyle`)
- Selected (cursor-on) unavailable rows still get the reverse-video highlight, with
  `(unavailable)` included in the highlighted text — so the user always sees the cursor

Unavailable models remain selectable. If selected, the run fails with a clear backend
error rather than being silently hidden.

### TUI Integration

In `cmd/harnesscli/tui/model.go`, the `ProvidersLoadedMsg` handler now also calls
`WithAvailability`:

```go
case ProvidersLoadedMsg:
    // ... populate apiKeyProviders ...
    m.modelSwitcher = m.modelSwitcher.WithKeyStatus(m.providerKeyConfigured)
    // New: mark models as available/unavailable for visual distinction
    m.modelSwitcher = m.modelSwitcher.WithAvailability(m.providerKeyConfigured)
```

Both `WithKeyStatus` (dot indicators `● ○`) and `WithAvailability` (dim/greyed rows)
use the same `providerKeyConfigured` function, so the indicators are consistent.

### Fallback Behavior

If `GET /v1/providers` fails, `fetchProvidersCmd` emits `ProvidersLoadedMsg{}` with
an empty provider list. The `ProvidersLoadedMsg` handler is never reached in the error
path — `fetchProvidersCmd` itself returns the empty message. The `DefaultModels` remain
loaded and `availabilitySet` stays false, so no `(unavailable)` indicators appear.

## Files Changed

- `cmd/harnesscli/tui/components/modelswitcher/model.go` — `Available` field,
  `availabilityFn`/`availabilitySet` fields on `Model`, `WithAvailability()` method,
  updated `WithModels()` to re-apply availability
- `cmd/harnesscli/tui/components/modelswitcher/view.go` — `unavailableStyle`,
  `unavailableSuffixStyle`, `isUnavailable` logic in `viewModelList`
- `cmd/harnesscli/tui/model.go` — `WithAvailability` call in `ProvidersLoadedMsg` handler

## Tests Added

`cmd/harnesscli/tui/components/modelswitcher/availability_test.go` (12 tests):

- `TestTUI313_ModelEntryAvailableField` — field exists and is settable
- `TestTUI313_DefaultModelsAvailableUnset` — DefaultModels have zero-value `Available`
- `TestTUI313_WithAvailabilityMarksModels` — fn applied per provider
- `TestTUI313_WithAvailabilityNilFnLeavesAllUnavailable` — nil fn safe
- `TestTUI313_WithAvailabilityDoesNotMutateOriginal` — value semantics
- `TestTUI313_WithAvailabilityAllConfigured` — all available when fn always true
- `TestTUI313_WithAvailabilityNoneConfigured` — all unavailable when fn always false
- `TestTUI313_WithModelsPreservesAvailability` — WithModels re-applies fn
- `TestTUI313_ViewAvailableModelNotDimmed` — no indicator when all available
- `TestTUI313_ViewUnavailableModelShowsIndicator` — `(unavailable)` shown
- `TestTUI313_ViewUnavailableModelStillPresent` — unavailable models not hidden
- `TestTUI313_ViewNoAvailabilitySetShowsNoDimming` — backwards compat (no info = no dim)
- `TestTUI313_ViewSelectedUnavailableModelHighlightedCorrectly` — cursor still shown
- `TestTUI313_WithAvailabilityPreservesExistingFields` — no field clobbering

All 14 tests (including 2 bonus tests) pass. All `./cmd/harnesscli/...` tests pass.

## Acceptance Criteria Status

- [x] TUI reads configured/available state from backend provider endpoint
- [x] Available models render emphasized (normal, bright style)
- [x] Unavailable models render muted with `(unavailable)` suffix
- [x] Any hardcoded availability assumptions isolated (DefaultModels have `Available=false`
      but no indicator shown until provider data arrives)
