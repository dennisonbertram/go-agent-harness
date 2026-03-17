# TUI Model Picker: Dynamic Fetch, Search, Favorites

## Summary

Three new features added to the model picker overlay:

1. **Dynamic model list** — fetched from `/v1/models` on picker open
2. **Search/filter** — printable chars filter visible models by display name
3. **Favorites/Stars** — `s` key toggles star; starred models pin to top; persisted to `~/.config/harnesscli/config.json`

## Files Changed

### New Files

- `cmd/harnesscli/config/config.go` — persistent CLI config (Load/Save with `~/.config/harnesscli/config.json`)
- `cmd/harnesscli/config/config_test.go` — tests for Load/Save round-trip, missing file, dir creation, file mode 0600

### Modified Files

- `cmd/harnesscli/tui/components/modelswitcher/model.go`
  - Added `currentModelID` field (replaces `IsCurrent` baking in `New`)
  - Added `searchQuery`, `starred map[string]bool`, `loading`, `loadError` fields
  - Added `ServerModelEntry` type
  - Added `visibleModels()` — filtered + starred-first ordering
  - Updated `SelectUp/SelectDown/Accept` to use `visibleModels()`
  - Added `WithModels`, `WithStarred`, `StarredIDs`, `ToggleStar`, `SetSearch`, `SearchQuery`, `SetLoading`, `Loading`, `SetLoadError`, `LoadError`, `IsStarred`

- `cmd/harnesscli/tui/components/modelswitcher/view.go`
  - Shows loading indicator above the list (not instead of it)
  - Shows error message (hides list) on load error
  - Shows `Filter: <query>` header when search is active
  - Shows `★` prefix (gold/yellow) for starred models
  - Shows "No models match" when filter yields empty results
  - Skips provider headers when search active or starred models present
  - Updated footer hint

- `cmd/harnesscli/tui/messages.go`
  - Added `ModelsFetchedMsg{Models []ServerModelEntry}`
  - Added `ModelsFetchErrorMsg{Err string}`

- `cmd/harnesscli/tui/api.go`
  - Added `fetchModelsCmd(baseURL string) tea.Cmd`

- `cmd/harnesscli/tui/model.go`
  - Import `harnessconfig "go-agent-harness/cmd/harnesscli/config"`
  - `New()`: loads starred models from persistent config on startup
  - `/model` command: preserves starred models, sets loading=true, dispatches `fetchModelsCmd`
  - Escape handler: clears search query before closing overlay
  - `default:` key handler: routes backspace/delete to trim search; `s` to toggle star (persists); other runes to search query
  - New message cases: `ModelsFetchedMsg`, `ModelsFetchErrorMsg`
  - `ModelSelectedMsg` handler: preserves starred models across model switches

### New Tests

- `cmd/harnesscli/config/config_test.go`: 5 tests
- `cmd/harnesscli/tui/components/modelswitcher/model_test.go`: 16 new tests (stars, search, WithModels, loading/error state)
- `cmd/harnesscli/tui/components/modelswitcher/view_test.go`: 7 new tests (loading indicator, error view, filter bar, star symbol, no-match message)

## Key Design Decisions

**Loading state shows cached DefaultModels** — when `/model` opens and fetch is in-flight, the user sees DefaultModels with a "Loading models..." header. This avoids a blank/blocked picker while waiting for the server. On fetch completion, `WithModels` replaces the list.

**`visibleModels()` is the single source of truth** — `Selected` is always an index into `visibleModels()`, not `m.Models`. This means navigation, Accept, and view rendering are all consistent.

**`s` key intercepted before generic rune handler** — in the `default:` case, when the model overlay is open, `s` is checked before the generic rune-as-search path to avoid accidentally searching for "s".

**Value semantics maintained** — all new methods return copies. `ToggleStar`, `WithStarred`, `SetSearch`, `WithModels` all copy the `starred` map to avoid aliasing.

**Auth** — `/v1/models` uses the same auth as other endpoints (Bearer token or disabled). The current `fetchModelsCmd` uses a plain `http.Get`. Since the default dev setup has `AuthDisabled=true` (no store configured), this works. For auth-enabled deployments, the TUI would need the token wired in.

## Test Results

All 27+ packages in `./cmd/harnesscli/...` pass with `-race -count=1`.
