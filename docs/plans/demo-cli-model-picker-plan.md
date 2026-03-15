# Plan: demo-cli Interactive Model Picker

## Problem

The `/models` command currently only prints a static list. Users must manually
type `/model <name>` after looking up the exact model key — two steps where one
should suffice. Arrow key navigation in the go-prompt dropdown also needs the
`OptionCompletionOnDown` option for reliable first-keystroke behaviour.

## Scope

**In scope:**
1. Add `prompt.OptionCompletionOnDown()` to `prompt.New(...)` — makes Down arrow
   immediately navigate the dropdown on first press
2. `/models` → interactive full-screen picker (Up/Down/Enter/Esc) that sets
   `currentModel` on selection
3. `demo-cli/picker.go` — new file with all picker logic
4. `demo-cli/picker_test.go` — unit tests for pure/testable logic
5. Update `/help` text

**Out of scope:** bubbletea full rewrite, fuzzy search, mouse support.

## Dependency decision

Use `golang.org/x/term` (already in `go.mod`) + ANSI escape sequences directly.
Zero new dependencies. bubbletea is heavier and creates terminal ownership
conflicts with go-prompt.

## Terminal state contract

go-prompt calls `p.in.TearDown()` (restores cooked mode) before the executor,
and `p.in.Setup()` (re-enters raw mode) after. The executor therefore runs with
the terminal in normal mode. `selectModel` then calls `term.MakeRaw` for the
picker and `defer term.Restore` on exit. Same pattern as `handleUserInput`.

---

## Files to change

| File | Change |
|------|--------|
| `demo-cli/main.go` | Add `OptionCompletionOnDown()` to `prompt.New()`; modify `/models` case in `handleCommand` to call `selectModel` |
| `demo-cli/display.go` | Update `PrintHelp()` `/models` description |
| `demo-cli/picker.go` | **NEW** — `buildPickerItems`, `selectModel`, helpers |
| `demo-cli/picker_test.go` | **NEW** — 11 unit tests |

No `go.mod` / `go.sum` changes needed.

---

## Step 1 — `OptionCompletionOnDown` (1-line change)

In `demo-cli/main.go`, add one option to `prompt.New(...)`:

```go
p := prompt.New(
    executor,
    completer,
    prompt.OptionLivePrefix(livePrefix),
    prompt.OptionTitle("go-agent-harness"),
    prompt.OptionPrefix(""),
    prompt.OptionPrefixTextColor(prompt.Green),
    prompt.OptionPreviewSuggestionTextColor(prompt.Blue),
    prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
    prompt.OptionSuggestionBGColor(prompt.DarkGray),
    prompt.OptionCompletionOnDown(), // makes Down open/navigate dropdown
)
```

---

## Step 2 — `demo-cli/picker.go`

### Data structures

```go
// pickerItem is one entry in the model picker.
// Header rows (provider names) have empty modelKey and are not selectable.
type pickerItem struct {
    modelKey    string // "" = non-selectable header
    displayLine string // rendered line shown to the user
}
```

### `buildPickerItems(cat *catalog.Catalog) []pickerItem`

- Returns nil for nil catalog.
- For each provider (alphabetical via existing `providerOrder`):
  - Appends a header item `{modelKey: "", displayLine: "  [ProviderName]"}`
  - For each model (alphabetical via existing `modelOrder`):
    - Appends `{modelKey: key, displayLine: "    key  $in/$out"}` (pricing optional)

### `firstSelectable(items []pickerItem, from, direction int) int`

Walks from `from` in `direction` (+1 or -1), wrapping at boundaries, until it
finds an item with non-empty `modelKey`. Returns -1 if none exist (all headers).
Maximum iterations = `len(items)` to prevent infinite loop.

### `selectModel(cat *catalog.Catalog) string`

```
1. buildPickerItems → if empty, return ""
2. selected = firstSelectable(items, 0, +1)  (first model item)
3. if selected < 0, return ""
4. term.MakeRaw(stdin) ; defer term.Restore
5. renderPicker(items, selected)
6. loop:
     read up to 3 bytes from os.Stdin
     Up / k   → selected = firstSelectable(items, selected, -1)
     Down / j → selected = firstSelectable(items, selected, +1)
     Enter/CR → clearPicker ; return items[selected].modelKey
     Esc / q  → clearPicker ; return ""
     renderPicker(items, selected)
```

### Rendering helpers

**`renderPicker(items []pickerItem, selected int)`**
Clears screen (`\033[2J\033[H`), prints a header line, then each item:
- Selected row: reverse video (`\033[7m`) + `> ` prefix
- Header row: dim (`\033[2m`)
- Normal row: no colour + `  ` prefix
Prints footer: `↑↓ navigate  Enter select  Esc cancel`

**`clearPicker()`**
Full clear (`\033[2J\033[H`) so go-prompt redraws cleanly after executor returns.

### Key-match helpers (reading raw bytes)

| Helper | Matches |
|--------|---------|
| `isUp(b)` | `\x1b[A`, `k` |
| `isDown(b)` | `\x1b[B`, `j` |
| `isEnter(b)` | `\r` (0x0d), `\n` (0x0a) |
| `isEscOrQ(b)` | `\x1b` alone, `q`, `Q` |

Read buffer size 3 bytes handles three-byte arrow escape sequences.

---

## Step 3 — `handleCommand` change in `main.go`

```go
case "/models":
    if modelCatalog == nil {
        display.PrintModelsList(modelCatalog) // prints "catalog not available"
        return true, ""
    }
    if chosen := selectModel(modelCatalog); chosen != "" {
        *currentModel = chosen
        display.PrintModelSwitched(chosen)
    }
    return true, ""
```

`handleCommand` already takes `*currentModel` as a pointer (used by `/model <name>`).

---

## Step 4 — `PrintHelp` update in `display.go`

```go
fmt.Printf("  %s  open interactive model picker\n", d.color(colorCyan, "/models"))
```

---

## Step 5 — `demo-cli/picker_test.go`

All tests are pure logic (no terminal required):

| # | Test | What it checks |
|---|------|----------------|
| 1 | `TestBuildPickerItems_WithCatalog` | Headers have empty modelKey; models have non-empty; alphabetical order; pricing in displayLine |
| 2 | `TestBuildPickerItems_NilCatalog` | Returns nil, no panic |
| 3 | `TestBuildPickerItems_ModelWithoutPricing` | Item included; no NaN/nil/panic |
| 4 | `TestBuildPickerItems_EmptyCatalog` | Zero providers → empty slice |
| 5 | `TestFirstSelectable_BasicForward` | `[header, m1, m2]`, from=0, dir=+1 → 1 |
| 6 | `TestFirstSelectable_BasicBackward` | `[header, m1, m2]`, from=2, dir=-1 → 1 |
| 7 | `TestFirstSelectable_Wrap` | Forward from last model wraps to first model |
| 8 | `TestFirstSelectable_AllHeaders` | Returns -1 without infinite loop |
| 9 | `TestSelectModel_NilCatalog` | Returns "" without panicking or calling MakeRaw |
| 10 | `TestSelectModel_EmptyCatalog` | Returns "" safely (zero selectable items) |
| 11 | `TestHandleCommand_Models_NilCatalog_NoChange` | Existing test still passes after handleCommand change |

---

## Risks and mitigations

| Risk | Mitigation |
|------|-----------|
| Terminal left in raw mode on panic | `defer term.Restore` is first defer after `term.MakeRaw`; defers run on panic |
| `clearPicker` calculates wrong lines on wide terminal (wrapping) | Use full clear `\033[2J\033[H` instead of line-counting — simpler and always correct |
| go-prompt redraws and clobbers picker output | `clearPicker` runs before returning from `selectModel`; terminal is blank on executor return; go-prompt redraws cleanly |
| Arrow key conflicts with go-prompt during picker | Non-issue: go-prompt stops its `readBuffer` goroutine before calling the executor (`stopReadBufCh <- struct{}{}`); no concurrent stdin reader |
| Provider header accidentally selected | `firstSelectable` skips items with empty `modelKey`; initial selection is always a model |
| Catalog with only headers and no models | `firstSelectable` returns -1; `selectModel` returns "" before entering raw mode |

---

## TDD implementation order

1. Write `picker_test.go` (all tests red)
2. Implement `buildPickerItems` and `firstSelectable` in `picker.go` (tests 1-8 green)
3. Implement `selectModel` early-exit paths (tests 9-10 green)
4. Implement rendering and input loop (manual test only)
5. Modify `handleCommand` in `main.go`
6. Update `PrintHelp` in `display.go`
7. Add `OptionCompletionOnDown()` to `prompt.New()`
8. `go test ./demo-cli/...` — all green
9. `go test ./internal/... ./cmd/... -race` — full regression
10. Manual smoke test in tmux
