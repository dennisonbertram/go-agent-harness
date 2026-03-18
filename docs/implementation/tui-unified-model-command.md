# TUI Unified /model Command with Config Panel

## Summary

Implemented a unified `/model` command that combines model selection, gateway routing,
and API key management into a two-level overlay:

- **Level-0**: Model list (existing behavior, augmented with key status indicators)
- **Level-1**: Config panel showing Gateway, API Key status, and Reasoning Effort (for reasoning models)

## Changes

### `cmd/harnesscli/tui/components/modelswitcher/model.go`

- Added `keyStatus func(string) bool` field to `Model` struct
- Added `WithKeyStatus(fn func(string) bool) Model` setter method

### `cmd/harnesscli/tui/components/modelswitcher/view.go`

- Updated `viewModelList()` to append `● ` (configured) or `○ ` (not configured)
  indicator after each model name when `m.keyStatus != nil`

### `cmd/harnesscli/tui/model.go`

**New state fields on `Model` struct:**
```go
modelConfigMode            bool
modelConfigEntry           modelswitcher.ModelEntry
modelConfigSection         int    // 0=gateway, 1=apikey, 2=reasoning
modelConfigGatewayCursor   int
modelConfigReasoningCursor int
modelConfigKeyInputMode    bool
modelConfigKeyInput        string
```

**New helper functions:**
- `gatewayIndex(id string) int` — find gateway option index by ID
- `reasoningLevelIndex(effort string) int` — find reasoning level index by effort ID
- `(m Model) providerKeyConfigured(providerKey string) bool` — check if provider has API key

**New accessor methods (for testing):**
- `ModelConfigMode() bool`
- `ModelConfigEntry() modelswitcher.ModelEntry`
- `ModelConfigSection() int`
- `ModelConfigGatewayCursor() int`
- `ModelConfigReasoningCursor() int`
- `ModelConfigKeyInputMode() bool`
- `ModelConfigKeyInput() string`

**`/model` command handler** — now also fetches providers alongside models:
```go
cmds = append(cmds, fetchModelsCmd(m.config.BaseURL))
cmds = append(cmds, fetchProvidersCmd(m.config.BaseURL))
```

**`ProvidersLoadedMsg` handler** — wires key status to the model switcher:
```go
m.modelSwitcher = m.modelSwitcher.WithKeyStatus(m.providerKeyConfigured)
```

**Enter key behavior** — replaces old 2-level reasoning flow:
- Enter at Level-0: always opens the config panel (all models, not just reasoning)
- Enter at config panel (not in key input): emits `ModelSelectedMsg` + `GatewaySelectedMsg` batch and closes
- Enter at config panel in key input mode: confirms and saves API key

**Escape cascade** (inserted before the Level-0 escape block):
1. Config panel + key input mode → exit key input, keep panel open
2. Config panel (no key input) → exit config panel, return to Level-0
3. Level-0 + search → clear search
4. Level-0 no search → close overlay entirely

**Config panel navigation** (new switch case):
- When in section 2 (reasoning) AND model is reasoning-capable: `j`/`down` and `k`/`up` move reasoning cursor
- Otherwise: `j`/`down` and `k`/`up` move between sections
- `h`/`left` and `l`/`right` in section 0 (gateway): move gateway cursor
- `K` or Enter in section 1 (apikey): enter key input mode

**Config panel key input** (new switch case):
- Backspace/Delete: removes last char
- Ctrl+U: clears input
- Runes: appends characters

**`View()`** — dispatches to config panel when active:
```go
case "model":
    if m.modelConfigMode {
        mainContent = m.viewModelConfigPanel()
    } else {
        mainContent = m.modelSwitcher.View(m.width)
    }
```

**`viewModelConfigPanel()`** — new render function showing:
- Model display name and provider label
- Gateway section with `▶` cursor on selected option
- API Key section with `●`/`○` status and optional key input field
- Reasoning Effort section (reasoning models only) with cursor navigation
- Footer with keyboard hint

**Command registry updates:**
- `/model` description: `"Switch model, gateway, and API keys"`
- `/provider` description: `"Switch routing gateway (use /model for per-model config)"`

### `cmd/harnesscli/tui/model_command_test.go`

Updated two tests to reflect new config panel behavior:

- `TestTUI137_ModelOverlayEnterNonReasoningEmitsMsg` — now verifies Enter at Level-0
  opens the config panel first, then Enter at config panel emits a `tea.BatchMsg`
  containing `ModelSelectedMsg`

- `TestTUI137_ModelOverlayEnterAtLevel1ClosesAndSetsModel` — updated to use the new
  config panel navigation (section + reasoning cursor), expects `tea.BatchMsg`

Added new test `TestTUI137_ModelOverlayEnterAtConfigPanelClosesAndSetsModel` as a
more explicit test of the full config panel flow with reasoning effort selection.

## Backward Compatibility

All existing tests pass. The following tests that tested the OLD direct-accept behavior
were updated to test the equivalent NEW config panel behavior:
- `TestTUI137_ModelOverlayEnterNonReasoningEmitsMsg`
- `TestTUI137_ModelOverlayEnterAtLevel1ClosesAndSetsModel`

The following tests that happened to describe behavior that remains correct (just via
different internal path) pass unchanged:
- `TestTUI137_ModelOverlayEscapeLevel1ReturnsToLevel0` (Escape from config panel = old Level-1)
- `TestTUI137_ModelOverlayEnterReasoningModelEntersLevel1` (config panel shows "Reasoning Effort")
