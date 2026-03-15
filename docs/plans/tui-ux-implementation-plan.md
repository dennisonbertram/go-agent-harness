# TUI UX Implementation Plan

## Section 1: Architecture Decisions

### 1A. Package Location: cmd/harnesscli/tui/ vs internal/tui/

Recommend: create the production TUI code under `cmd/harnesscli/tui/` initially, with shared render/util helpers extracted to `internal/tui/` only if another consumer appears. The CLI is the only caller today, and the current code path is already terminal-centric (`dispatch` -> `run` in `cmd/harnesscli/main.go`), so colocating TUI avoids indirection and reduces coupling for the first release. The repository already has no reusable terminal shell layer (`run` is a single command entrypoint and all tests are package `main` focused), so the fastest production path is a command-scoped package aligned with the present ownership model. If `internal/tui/` is added too early, we risk freezing unstable abstractions around BubbleTea model messages and view composition before event semantics are stabilized. This also aligns with Go conventions for command-specific UI behavior while preserving a clean future migration path if another binary later needs the same renderer.

### 1B. SSE Streaming Bridge Design

SSE events will stream from `/api/v1/runs/{id}/events` and be converted to BubbleTea messages through a long-lived bridge. The bridge will own a buffered channel `chan tea.Msg` with capacity 256 to absorb short bursts and prevent transient stalls when rendering stalls for a single frame; this keeps the model loop responsive under tool-call bursts. The bridge goroutine starts in `Init` after run creation and consumes HTTP events in a dedicated scanner loop; it exits on terminal event (`run.completed`, `run.failed`), context cancellation, or parse/read error. Errors are propagated as `SSEErrorMsg` with terminal state in model; no panic path is used. The main `Update` handler will return `tea.Batch(pollMsgCmd(bridgeChan))` after each event so the event loop can continue non-blocking while rearming polling.

```go
func StartSSEBridge(ctx context.Context, client *http.Client, baseURL, runID string) (<-chan tea.Msg, func()) {
    ch := make(chan tea.Msg, 256)
    done := make(chan struct{})
    cctx, cancel := context.WithCancel(ctx)
    go func() {
        defer close(ch)
        req, _ := http.NewRequestWithContext(cctx, http.MethodGet, baseURL+"/api/v1/runs/"+runID+"/events", nil)
        resp, err := client.Do(req)
        if err != nil { ch <- SSEErrorMsg{err}; return }
        defer resp.Body.Close()
        scanEvents(resp.Body, func(ev harness.Event) {
            if !nonBlockingSend(ch, SSEEventMsg{Event: ev}) { ch <- SSEDropMsg{Count: 1} }
            if harness.IsTerminalEvent(ev.Type) { ch <- SSEDoneMsg{RunID: runID} }
        }, func(err error) { ch <- SSEErrorMsg{err} })
    }()
    return ch, cancel
}

func PollSSECmd(ch <-chan tea.Msg) tea.Cmd {
    return func() tea.Msg {
        msg, ok := <-ch
        if !ok { return SSEDoneMsg{} }
        return msg
    }
}

func nonBlockingSend(ch chan<- tea.Msg, msg tea.Msg) bool {
    select { case ch <- msg: return true
    default: return false }
}
```

Goland: this pattern guarantees the renderer never blocks on the bridge while allowing explicit overflow telemetry and deterministic shutdown semantics.

### 1C. Concurrent Tool Call Handling in Model State

Use a map keyed by `callID` as the canonical state store: `map[string]ToolCallState` keyed by tool call ID.

Rationale: tool-call deltas arrive independently and may interleave between calls; a map provides constant-time upsert and avoids fragile positional drift in concurrent deltas, while still preserving identity semantics for resumable rendering. For display ordering, maintain a separate `[]string` `toolCallOrder` to preserve stream order by first-seen event. A pure slice cannot efficiently handle out-of-order delta arrival and merge-by-ID without repeated scans, which becomes expensive for long sessions with many calls and nested branches.

```go
type ToolCallState struct {
	ID             string
	Name           string
	ParentCallID   string
	Status         string
	StartedAt      time.Time
	UpdatedAt      time.Time
	Expanded       bool
	StartedText    string
	StreamingText  strings.Builder
	Lines          []string
	ErrorText      string
	TokenUsage     int64
	Duration       time.Duration
	Sequence       int
	ProgressHints   []string
	Command        string
	NestedCalls    []string
}
```

When a `ToolCallStarted` arrives: create or rehydrate state keyed by ID and append ID to `toolCallOrder` if new. For each `ToolCallChunk`/`ToolCallPartial`, append to `StreamingText`, update `UpdatedAt`, and mark `Status=running`. On `ToolCallResult`, copy final output into `Lines`, finalize `Duration`, update status, and mark completion. `toolcall` component rendering always reads from map state and map + order list prevents UI flicker when multiple deltas race.

### 1D. Test Strategy

Use `teatest` as the primary integration harness and `tea.NewTestProgram` for unit-level message/state verification. `teatest` is the right default for snapshot-based render assertions because it validates real terminal output and supports deterministic key sequences; `tea.NewTestProgram` remains useful for state transitions and bridge command behavior where full rendering is not the target. A custom mock layer remains necessary for SSE parsing and transport faults because upstream HTTP behavior must be deterministic and latency-injected.

Patterns:
- layout components: isolate pure view assembly in `View()` helpers and validate with `teatest.NewTestModel` against golden snapshots at 80x24, 120x40, and 200x50.
- streaming renderer: feed model with synthetic stream chunks via command emissions and verify incremental text, spinner evolution, and truncation heuristics; combine with fake clock for tip timing.
- input area: verify multiline semantics (`Enter` vs `Ctrl+J`), history navigation, overlay triggers (`/`, `@`, `!`), and paste behavior via key sequences.

```go
func TestStreamingTextRenderer_RendersChunkedAssistantOutput(t *testing.T) {
	testCtx := newStreamRendererTestContext(t)
	model := testCtx.NewModel(120)
	p := tea.NewProgram(model)
	go func() {
		time.Sleep(10 * time.Millisecond)
		p.Send(streamMsg{CallID: "c1", Text: "Hello"})
		time.Sleep(5 * time.Millisecond)
		p.Send(streamMsg{CallID: "c1", Text: " world"})
		time.Sleep(5 * time.Millisecond)
		p.Send(streamDoneMsg{CallID: "c1", Final: true})
	}()
	view := waitUntil(t, p, func(s string) bool { return strings.Contains(s, "Hello world") })
	if !strings.Contains(view, "Hello world") {
		t.Fatalf("expected streamed text in viewport")
	}
	testCtx.RequireSnapshot(t, "stream_renderer_120x40")
}
```

## Section 2: Component Tree

```text
cmd/harnesscli/tui/
  model.go
  theme.go
  keys.go
  messages.go
  bridge.go
  init.go
  run_flow.go
  config.go
  render.go
  state.go
  cmd_parser.go
  cmd_result.go
  test_helpers_runtime.go
  testhelpers/
    fixtures.go
    httptest_server.go
    assert.go
    golden.go
    snapshot.go
    events.go
  components/
    statusbar/
      model.go
      view.go
      status.go
      state.go
    input/
      model.go
      view.go
      history.go
      multiline.go
    viewport/
      model.go
      layout.go
      virtualization.go
      state.go
    streamrenderer/
      model.go
      view.go
      styles.go
      tokenizer.go
    messagebubble/
      user.go
      assistant.go
      shared.go
      markdown.go
    spinner/
      model.go
      verbs.go
      view.go
    toolcall/
      model.go
      view.go
      state.go
      nested.go
    diffviewer/
      model.go
      view.go
      formatter.go
    permissionprompt/
      model.go
      view.go
      actions.go
    autocomplete/
      model.go
      view.go
      matcher.go
      source.go
    helpdialog/
      model.go
      view.go
      tabs.go
      keys.go
    contextgrid/
      model.go
      view.go
      palette.go
    statsheatmap/
      model.go
      view.go
      ascii.go
      legend.go
    configpanel/
      model.go
      view.go
      schema.go
    permissionspanel/
      model.go
      view.go
      rule.go
    sessionpicker/
      model.go
      view.go
      search.go
    planmode/
      model.go
      view.go
      command.go
    overlay/
      manager.go
    layout/
      container.go
      constraints.go
```

Directory/files notes:
- `cmd/harnesscli/tui/model.go` defines the root BubbleTea model, command queue, and high-level `Update` orchestration.
- `cmd/harnesscli/tui/theme.go` centralizes color tokens, lipgloss palettes, spacing, and symbol definitions for deterministic visual parity.
- `cmd/harnesscli/tui/keys.go` defines all key maps and explicit key->command bindings for overlays, navigation, and editing.
- `cmd/harnesscli/tui/messages.go` contains all custom `tea.Msg` types including SSE events, input actions, tool-call deltas, and command results.
- `cmd/harnesscli/tui/bridge.go` owns the SSE bridge lifecycle and channel/polling glue.
- `cmd/harnesscli/tui/init.go` isolates `Init()` behavior such as initial fetch, window-size warmup, and bridge startup for clean test injection.
- `cmd/harnesscli/tui/run_flow.go` maps run lifecycle states and cancellation semantics from creation through terminal event.
- `cmd/harnesscli/tui/config.go` stores session-level rendering flags, feature toggles, and user-configurable UX mode options.
- `cmd/harnesscli/tui/render.go` composes terminal sections for deterministic rendering and avoids duplicate layout logic.
- `cmd/harnesscli/tui/state.go` holds normalized domain state with immutable snapshots to avoid partial mutation hazards.
- `cmd/harnesscli/tui/cmd_parser.go` parses slash commands, arguments, and command intents from multiline input.
- `cmd/harnesscli/tui/cmd_result.go` formats inline command outputs and routes to status/viewport/history.
- `cmd/harnesscli/tui/test_helpers_runtime.go` exposes production-safe hooks for dependency injection and deterministic timers.
- `cmd/harnesscli/tui/components/statusbar/model.go` stores status data: identity, path, git branch, MCP health, and mode indicators.
- `cmd/harnesscli/tui/components/statusbar/view.go` renders fixed footer with hints and plan/edit mode overlays.
- `cmd/harnesscli/tui/components/statusbar/status.go` computes status segments and state transitions from model events.
- `cmd/harnesscli/tui/components/statusbar/state.go` owns the pure status-state transitions and comparison helpers.
- `cmd/harnesscli/tui/components/input/model.go` handles multiline text buffer, cursor offsets, and editing transforms.
- `cmd/harnesscli/tui/components/input/view.go` renders prompt line, cursor block, and inline overlay hints beneath separators.
- `cmd/harnesscli/tui/components/input/history.go` stores and retrieves command history with bounded ring buffer and branch-aware persistence.
- `cmd/harnesscli/tui/components/input/multiline.go` implements line continuation, indentation, and paste semantics.
- `cmd/harnesscli/tui/components/viewport/model.go` tracks scrollback messages, visible window, and cursor anchoring.
- `cmd/harnesscli/tui/components/viewport/layout.go` computes viewport heights from live window-size messages.
- `cmd/harnesscli/tui/components/viewport/virtualization.go` handles large history culling and off-screen message virtualization.
- `cmd/harnesscli/tui/components/viewport/state.go` stores normalized entries, message metadata, and virtualization indexes.
- `cmd/harnesscli/tui/components/streamrenderer/model.go` streams assistant/text/tool chunks and token counters.
- `cmd/harnesscli/tui/components/streamrenderer/view.go` draws streamed content with indentation and spinner timing.
- `cmd/harnesscli/tui/components/streamrenderer/styles.go` maps markdown or plaintext segments to lipgloss/glamour style outputs.
- `cmd/harnesscli/tui/components/streamrenderer/tokenizer.go` supports token-aware truncation and long-output summary behavior.
- `cmd/harnesscli/tui/components/messagebubble/user.go` renders user turns with full-width dark-gray background and trailing blank row.
- `cmd/harnesscli/tui/components/messagebubble/assistant.go` renders assistant turns including titles, markdown, bullets, tables, and code blocks.
- `cmd/harnesscli/tui/components/messagebubble/shared.go` provides shared bubble helpers, indentation math, and connector symbols.
- `cmd/harnesscli/tui/components/messagebubble/markdown.go` integrates glamour rendering and post-processing for CLI-safe output.
- `cmd/harnesscli/tui/components/spinner/model.go` drives spinner frame timing, verb rotation, and working timers.
- `cmd/harnesscli/tui/components/spinner/verbs.go` defines whimsical verb pool and deterministic seeding for testability.
- `cmd/harnesscli/tui/components/spinner/view.go` renders spinner plus duration/token meta and tip lines.
- `cmd/harnesscli/tui/components/toolcall/model.go` stores per-call collapsed/expanded state and partial result chunks.
- `cmd/harnesscli/tui/components/toolcall/view.go` renders `⏺` and `⎿` rows for collapsed/expanded modes.
- `cmd/harnesscli/tui/components/toolcall/state.go` defines shared tool metadata and status transitions.
- `cmd/harnesscli/tui/components/toolcall/nested.go` renders agent/sub-agent nested call trees and indentation.
- `cmd/harnesscli/tui/components/diffviewer/model.go` accepts diff payloads and tracks truncation plus line limits.
- `cmd/harnesscli/tui/components/diffviewer/view.go` renders unified diff with `╌` bordered layout and line numbering.
- `cmd/harnesscli/tui/components/diffviewer/formatter.go` normalizes path headers, hunks, and +/- coloring.
- `cmd/harnesscli/tui/components/permissionprompt/model.go` tracks prompt lifecycle, scope choices, and default actions.
- `cmd/harnesscli/tui/components/permissionprompt/view.go` builds modal overlay with option rows and context text.
- `cmd/harnesscli/tui/components/permissionprompt/actions.go` resolves keypress actions to model updates and command dispatch.
- `cmd/harnesscli/tui/components/autocomplete/model.go` handles fuzzy filtering and ranking for command/file suggestions.
- `cmd/harnesscli/tui/components/autocomplete/view.go` draws dropdown panel with selected row and metadata columns.
- `cmd/harnesscli/tui/components/autocomplete/matcher.go` implements scoring, prefix matching, and abbreviation logic.
- `cmd/harnesscli/tui/components/autocomplete/source.go` defines providers for slash commands, files, sessions, and models.
- `cmd/harnesscli/tui/components/helpdialog/model.go` owns three-tab dialog state and active section index.
- `cmd/harnesscli/tui/components/helpdialog/view.go` renders help card with command/keybinding/about tabs.
- `cmd/harnesscli/tui/components/helpdialog/tabs.go` hosts static tab schemas and per-row metadata.
- `cmd/harnesscli/tui/components/helpdialog/keys.go` wires tab switching and close shortcuts.
- `cmd/harnesscli/tui/components/contextgrid/model.go` calculates token and context usage bins for `/context` output.
- `cmd/harnesscli/tui/components/contextgrid/view.go` draws the 10x10 icon matrix with category legends.
- `cmd/harnesscli/tui/components/contextgrid/palette.go` maps categories to icon/color pairs and density levels.
- `cmd/harnesscli/tui/components/statsheatmap/model.go` stores usage buckets and heatmap cell metadata.
- `cmd/harnesscli/tui/components/statsheatmap/view.go` renders `░▒▓█` heatmap and axis labels.
- `cmd/harnesscli/tui/components/statsheatmap/ascii.go` handles line-chart ASCII fallback and legend generation.
- `cmd/harnesscli/tui/components/statsheatmap/legend.go` renders key with day/month scales and fun-fact text.
- `cmd/harnesscli/tui/components/configpanel/model.go` represents config entries and inline search/focus state.
- `cmd/harnesscli/tui/components/configpanel/view.go` renders key-value rows, edit states, and validation markers.
- `cmd/harnesscli/tui/components/configpanel/schema.go` defines config schema, types, and defaults.
- `cmd/harnesscli/tui/components/permissionspanel/model.go` stores permission rule rows and scope transitions.
- `cmd/harnesscli/tui/components/permissionspanel/view.go` renders `/permissions`-style tabs and status icons.
- `cmd/harnesscli/tui/components/permissionspanel/rule.go` composes rule rows and validation logic.
- `cmd/harnesscli/tui/components/sessionpicker/model.go` stores resumable sessions and search filters.
- `cmd/harnesscli/tui/components/sessionpicker/view.go` shows scrollable rows with time, branch, size, and markers.
- `cmd/harnesscli/tui/components/sessionpicker/search.go` applies incremental query constraints and keyboard shortcuts.
- `cmd/harnesscli/tui/components/planmode/model.go` handles compact/detailed plan mode indicator and overlay states.
- `cmd/harnesscli/tui/components/planmode/view.go` renders compact mode card with instructions and risk warnings.
- `cmd/harnesscli/tui/components/planmode/command.go` maps model-level cycle actions to commands and status updates.
- `cmd/harnesscli/tui/components/overlay/manager.go` coordinates mutually exclusive overlays and focus priority.
- `cmd/harnesscli/tui/components/layout/container.go` arranges component frames and separator hierarchy.
- `cmd/harnesscli/tui/components/layout/constraints.go` computes responsive breakpoints and orientation rules.
- `cmd/harnesscli/testhelpers/fixtures.go` centralizes static sample events, message payloads, and symbol palettes for tests.
- `cmd/harnesscli/testhelpers/httptest_server.go` supplies deterministic SSE and dialog endpoints via `httptest.Server`.
- `cmd/harnesscli/testhelpers/assert.go` provides test assertions for viewport strings, style tokens, and parse equivalence.
- `cmd/harnesscli/testhelpers/golden.go` loads/saves snapshot files and compares canonicalized render strings.
- `cmd/harnesscli/testhelpers/snapshot.go` wraps teatest program snapshots for fixed-size terminal buffers.
- `cmd/harnesscli/testhelpers/events.go` provides event builders for `harness.Event` and synthetic delta payloads.

## Section 3: GitHub Tickets (TUI-001 through TUI-060)

### TUI-001: Add terminal UI dependencies to module definition

**Phase**: Phase 0 — Foundations
**Labels**: tui, phase-0, component:dependencies
**Dependencies**: none

**Description**:
Add direct `charmbracelet/bubbletea`, `charmbracelet/lipgloss`, `charmbracelet/glamour`, and `charmbracelet/x/teatest` dependencies to `go.mod`. The project currently has `glamour` as indirect and no BubbleTea or teatest dependency, so all UI work must be pinned explicitly. This change unblocks deterministic rendering, keyboard mapping, and snapshot testing. Keep versions compatible with Go 1.25 and existing indirect dependency graph.

**Acceptance Criteria**:
- [ ] `go.mod` contains direct requirements for `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `github.com/charmbracelet/glamour`, and `github.com/charmbracelet/x/teatest`.
- [ ] `go.sum` is updated with valid checksums after `go mod tidy`.
- [ ] Existing dependency resolution remains unchanged for non-TUI code.
- [ ] `go test ./cmd/harnesscli` remains green with current tests.

**Files to Create/Modify**:
- `go.mod` — add direct require lines for the new dependencies and explicit version strategy.
- `go.sum` — add checksums for the direct dependencies.

**TDD Requirements** (write these tests FIRST):
- `TestTUI001_DependenciesPinned` — validate go.mod contains required module paths.
- `TestTUI001_GoModTidyNoUnintendedRemoval` — verify unrelated dependencies remain present after tidy.

**Regression Test Requirements**:
- Concurrent access: none (dependency-only ticket), add a parallel package-level import check in CI-style script simulation.
- Boundary conditions: verify minimum supported Go version compiles with dependency module constraints.
- Error paths: verify unresolved dependency names or private module fetch errors are surfaced by `go mod tidy` in CI.

**Visual Similarity Tests**:
- 80x24: not applicable for visual diff; validate no render files are introduced by this ticket.
- 120x40: same as 80x24; no TUI output assertions.
- 200x50: same as 80x24.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-001-80x24.txt

### TUI-002: Create package scaffold and file placeholders

**Phase**: Phase 0 — Foundations
**Labels**: tui, phase-0, component:scaffold
**Dependencies**: none

**Description**:
Create `cmd/harnesscli/tui` and all subpackages for components listed in this plan. Add minimal package declarations, exported type stubs, and compile-safe compile-time boundaries without behavioral logic. The scaffold should include the overlay manager and component directories so ticket dependencies can be implemented in order. This step makes the repository structure explicit and prevents drift across tickets.

**Acceptance Criteria**:
- [ ] New folder structure under `cmd/harnesscli/tui` matches the implementation tree.
- [ ] Each package compiles with placeholder types but no unfinished imports.
- [ ] `cmd/harnesscli` still compiles with unchanged non-TUI command path.
- [ ] No cyclical package import graph from new packages.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/model.go` — add `package tui` and stub root model.
- `cmd/harnesscli/tui/components/statusbar/model.go` — add statusbar package skeleton.

**TDD Requirements** (write these tests FIRST):
- `TestTUI002_PackageTreeCanBeBuilt` — compile-only check across all new directories.
- `TestTUI002_NoCyclicPackages` — static import graph test if possible, or go list import validation.

**Regression Test Requirements**:
- Concurrent access: compile with `t.Parallel()` for each scaffold test to ensure safe package initialization.
- Boundary conditions: verify empty stub files do not produce zero-value panics due to nil methods.
- Error paths: intentionally build with duplicate package names should fail early.

**Visual Similarity Tests**:
- 80x24: no layout output expected; expected capture remains empty shell.
- 120x40: same as 80x24.
- 200x50: same as 80x24.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-002-80x24.txt

### TUI-003: Root model skeleton with init/update/view stubs

**Phase**: Phase 0 — Foundations
**Labels**: tui, phase-0, component:root
**Dependencies**: TUI-002

**Description**:
Implement a minimal BubbleTea root model in `cmd/harnesscli/tui/model.go` with `Init`, `Update`, and `View`. Add placeholder message handling for key events, window resize messages, and run lifecycle states. The goal is a non-panicking shell that can be instantiated and rendered as a stable baseline. This is the anchor for all downstream components.

**Acceptance Criteria**:
- [ ] `model struct` defined with fields for width/height and `overlayMode`.
- [ ] `Init()` returns `nil` or queued bootstrap command.
- [ ] `Update()` handles `tea.KeyMsg`, `tea.WindowSizeMsg`, and unknown messages conservatively.
- [ ] `View()` returns a non-empty string with debug-safe placeholders.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/model.go` — define root struct, interfaces, and skeleton methods.
- `cmd/harnesscli/main.go` — create optional compile switch to instantiate TUI model.

**TDD Requirements** (write these tests FIRST):
- `TestTUI003_RootModelImplementsTeaModel` — compile assertion that model implements `tea.Model`.
- `TestTUI003_InitReturnsCmdOrNil` — validate no nil dereference in Update loop.

**Regression Test Requirements**:
- Concurrent access: test running two `tea.NewProgram` instances from same package simultaneously to catch shared global state.
- Boundary conditions: handle nil pointers in root model fields without panic.
- Error paths: invalid messages (`nil` key msg or zero terminal size) should not panic.

**Visual Similarity Tests**:
- 80x24: show placeholder root content in a single centered region.
- 120x40: placeholder root content centered and unchanged.
- 200x50: placeholder root content with stable margins.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-003-80x24.txt

### TUI-004: SSE-to-channel bridge implementation

**Phase**: Phase 0 — Foundations
**Labels**: tui, phase-0, component:bridge
**Dependencies**: TUI-003

**Description**:
Introduce asynchronous SSE bridge in `cmd/harnesscli/tui/bridge.go` that reads from `/api/v1/runs/{id}/events` and emits typed `tea.Msg`s.
The function starts a goroutine on run creation, parses SSE envelopes, and sends decoded events to a buffered channel.
When terminal event or error is encountered, bridge emits completion/error messages and closes channel.
This removes current linear scanner coupling from `main.go` and makes streaming reactive.

**Acceptance Criteria**:
- [ ] `StartSSEBridge` accepts context and base URL and returns a channel plus stop function.
- [ ] Channel capacity is documented and non-blocking when bridge can outpace the UI loop.
- [ ] `SSEOverflowMsg` is emitted when channel backpressure is hit.
- [ ] Bridge cancels cleanly with context.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/bridge.go` — implement parser+goroutine bridge.
- `cmd/harnesscli/main.go` — route run mode into TUI command path where applicable.

**TDD Requirements** (write these tests FIRST):
- `TestTUI004_BridgeEmitsAssistantEvents` — verify event decoding on channel.
- `TestTUI004_BridgeStopsOnContextCancel` — ensure stop function prevents goroutine leaks.

**Regression Test Requirements**:
- Concurrent access: start two bridges for different runs in parallel with different contexts and verify no channel cross-talk.
- Boundary conditions: verify handling of zero-byte frames, multi-line data fields, and trailing block without newline.
- Error paths: server close, parse failure, and terminal-event termination should all propagate as messages.

**Visual Similarity Tests**:
- 80x24: no direct layout assertion; show first assistant token only after bridge-driven update.
- 120x40: first token appears in top conversation region with no extra blank lines.
- 200x50: streaming updates appear with stable viewport at bottom input.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-004-80x24.txt

### TUI-005: Window resize handling and layout math helpers

**Phase**: Phase 0 — Foundations
**Labels**: tui, phase-0, component:layout
**Dependencies**: TUI-003, TUI-004

**Description**:
Add deterministic layout helpers in `cmd/harnesscli/tui/components/layout` for converting terminal size to component heights and widths.
Functions must compute input, status, separator, and viewport regions without magic numbers and should support very small terminals. Resize events from BubbleTea should update root measurements and trigger full reflow.
This ticket prevents content clipping and ensures consistent behavior at 80x24 and larger terminals.

**Acceptance Criteria**:
- [ ] `layoutState` stores width/height and derived component heights.
- [ ] Input region enforces minimum lines and handles narrow widths.
- [ ] On `tea.WindowSizeMsg`, view recomputes widths without panic.
- [ ] Helpers are unit-tested independently of terminal I/O.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/layout/constraints.go` — dimension formulae.
- `cmd/harnesscli/tui/components/layout/container.go` — container composition helper.

**TDD Requirements** (write these tests FIRST):
- `TestTUI005_ComputeLayoutFor80x24` — assert component heights.
- `TestTUI005_LayoutStaysStableAtTinySizes` — verify clamped values for minimum terminal dimensions.

**Regression Test Requirements**:
- Concurrent access: apply rapid resize bursts on a background goroutine while update loop renders.
- Boundary conditions: widths of 20, 40, 80 and heights of 10, 24, 40.
- Error paths: zero/negative resize values default to last-known valid geometry.

**Visual Similarity Tests**:
- 80x24: view shows status bar + two separators + input area without overlap.
- 120x40: viewport grows while maintaining fixed input/status area.
- 200x50: side regions align exactly and right-aligned session chip present.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-005-80x24.txt

### TUI-006: Lipgloss theme/palette setup

**Phase**: Phase 0 — Foundations
**Labels**: tui, phase-0, component:theme
**Dependencies**: TUI-002, TUI-005

**Description**:
Create a single `theme.go` source of truth for colors, symbols, spacing, and styles.
Map research-derived tokens to actual lipgloss styles and reuse Glamour for markdown code spans.
This phase ensures every component emits consistent `user`, `assistant`, `tool`, and warning styles.
No rendering divergence should remain across components.

**Acceptance Criteria**:
- [ ] Theme exports palette names for all symbols in research summary.
- [ ] User message, spinner, status, and tree connector styles are defined and tested.
- [ ] Theme supports low-color fallback to avoid render breaks.
- [ ] Theme file contains deterministic symbol constants.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/theme.go` — all shared styles.
- `cmd/harnesscli/tui/components/messagebubble/shared.go` — consume shared theme definitions.

**TDD Requirements** (write these tests FIRST):
- `TestTUI006_ThemeHasRequiredStyles` — validates style map completeness.
- `TestTUI006_ThemeSupportsNoColorMode` — verify fallback rendering for monochrome terminals.

**Regression Test Requirements**:
- Concurrent access: multiple components reading theme concurrently should never mutate shared structures.
- Boundary conditions: unknown theme key should return default style.
- Error paths: nil style references should be detected in tests with `mustHave` guards.

**Visual Similarity Tests**:
- 80x24: confirm symbols and color accents remain visually mapped in snapshot (ANSI-safe).
- 120x40: same tokens with cleaner table/box spacing.
- 200x50: verify long lines preserve color/style boundaries.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-006-80x24.txt

### TUI-007: Test harness setup for TUI execution

**Phase**: Phase 0 — Foundations
**Labels**: tui, phase-0, component:testhelpers
**Dependencies**: TUI-002

**Description**:
Create the `cmd/harnesscli/testhelpers` package to support deterministic unit/integration testing.
Add fixtures, event builders, mock HTTP endpoints, snapshot loaders, and assertion helpers.
This package should avoid importing test logic into production code while sharing canonical event types.
It is required by every advanced UI ticket for stable tests.

**Acceptance Criteria**:
- [ ] Test helper package compiles in `go test ./cmd/harnesscli/...`.
- [ ] Mock SSE server can send multi-event streams with controlled delays.
- [ ] Snapshot loader uses relative paths from repo root.
- [ ] Helper assertions include viewport, status, and message order utilities.

**Files to Create/Modify**:
- `cmd/harnesscli/testhelpers/httptest_server.go` — mock SSE and session endpoints.
- `cmd/harnesscli/testhelpers/golden.go` — snapshot read/write helpers.

**TDD Requirements** (write these tests FIRST):
- `TestTUI007_MockSSERelayEmitsEvents` — verify deterministic event sequence.
- `TestTUI007_SnapshotRoundTrip` — write and read golden fixture.

**Regression Test Requirements**:
- Concurrent access: concurrent tests must not share global ports or temp files.
- Boundary conditions: stream with zero events and very long events are supported.
- Error paths: broken fixture path and malformed JSON payload handling.

**Visual Similarity Tests**:
- 80x24: helper test logs are not visual; no direct output.
- 120x40: no direct output.
- 200x50: no direct output.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-007-80x24.txt

### TUI-008: Key bindings definition with BubbleTea KeyMap

**Phase**: Phase 0 — Foundations
**Labels**: tui, phase-0, component:keys
**Dependencies**: TUI-003, TUI-002

**Description**:
Define a typed `keyMap` in `cmd/harnesscli/tui/keys.go` covering model shortcuts and command shortcuts.
Include bindings for `/`, `@`, Escape, Enter, Ctrl+O, Ctrl+E, Shift+Tab, Ctrl+T, Ctrl+V, Meta+P, Meta+O, and tab completion controls.
Include short help strings for help overlay.
This ticket decouples key intent from model logic and supports discoverability.

**Acceptance Criteria**:
- [ ] All required keys are represented with unambiguous `Binding` structs.
- [ ] `ShortHelp()` includes at least 8 primary actions.
- [ ] Unknown keys are ignored and do not alter state.
- [ ] Conflicting chords resolve to explicit precedence.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/keys.go` — key map and helper methods.
- `cmd/harnesscli/tui/helpdialog/tabs.go` — consume key metadata.

**TDD Requirements** (write these tests FIRST):
- `TestTUI008_HelpTextContainsRequiredShortcuts` — assert key binding map includes all required commands.
- `TestTUI008_ShorthandBindingsAreStable` — verify chord serialization for known keys.

**Regression Test Requirements**:
- Concurrent access: run input key simulation concurrently across two test programs.
- Boundary conditions: non-printable and unsupported keys should not crash.
- Error paths: duplicate shortcuts should fail validation.

**Visual Similarity Tests**:
- 80x24: help line shows at least one key legend.
- 120x40: help legend wraps without clipping.
- 200x50: expanded legend remains aligned with status line.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-008-80x24.txt

### TUI-009: Unified message type definitions

**Phase**: Phase 0 — Foundations
**Labels**: tui, phase-0, component:messages
**Dependencies**: TUI-008

**Description**:
Define all internal `tea.Msg` structs and interfaces in `cmd/harnesscli/tui/messages.go`, including terminal events, stream chunks, UI actions, tool start/result/error, clipboard copies, and overlay toggles.
Normalize names to align with `internal/harness.Event` types while preserving TUI-specific fields.
This prevents ad-hoc type creation and enables strict switch-based update handling.

**Acceptance Criteria**:
- [ ] Message types include stream, tool-call, spinner, error, and command-result variants.
- [ ] Messages are minimal and immutable enough for deterministic tests.
- [ ] Update handler can switch on message type with no default-case swallowing.
- [ ] JSON-like `harness.Event` decoding path remains separate from UI wrappers.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/messages.go` — all msg definitions.
- `cmd/harnesscli/tui/model.go` — consume new messages in update switch.

**TDD Requirements** (write these tests FIRST):
- `TestTUI009_MsgTypeCoverage` — ensure all expected message constructors exist.
- `TestTUI009_SSEEventMsgRoundTrip` — validate data preservation through message wrapping.

**Regression Test Requirements**:
- Concurrent access: emit many message types in parallel and ensure no data races when pooled.
- Boundary conditions: zero-value messages should be safe and testable.
- Error paths: invalid event payload should map to typed error message.

**Visual Similarity Tests**:
- 80x24: no dedicated visual assertion; message plumbing not directly rendered.
- 120x40: no dedicated visual assertion.
- 200x50: no dedicated visual assertion.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-009-80x24.txt

### TUI-010: Build tags and conditional compilation

**Phase**: Phase 0 — Foundations
**Labels**: tui, phase-0, component:build
**Dependencies**: TUI-001, TUI-002

**Description**:
Introduce build tags and runtime flags that keep existing non-interactive CLI behavior unchanged unless TUI mode is active.
Add a compile-safe fallback for interactive path when terminal detection fails, defaulting to legacy streaming output.
This guards compatibility with script-based callers and CI.

**Acceptance Criteria**:
- [ ] Existing `main` command path remains unchanged without `--tui` flag.
- [ ] New `--tui` gate allows BubbleTea launch and event-loop behavior.
- [ ] Non-tty and forced non-interactive tests continue to pass.
- [ ] Build tags prevent importing `x/term`/tty-specific code in unsupported environments.

**Files to Create/Modify**:
- `cmd/harnesscli/main.go` — gate selection and mode flag.
- `cmd/harnesscli/tui/config.go` — runtime feature toggles.

**TDD Requirements** (write these tests FIRST):
- `TestTUI010_DefaultModeIsNonInteractive` — legacy JSON output preserved by default.
- `TestTUI010_TuiFlagSelectsInteractiveProgram` — instantiate TUI when requested.

**Regression Test Requirements**:
- Concurrent access: toggle mode flag in parallel with distinct run flows.
- Boundary conditions: missing stdout tty should force legacy mode.
- Error paths: explicit CLI parsing errors should still return parse failure with code 1.

**Visual Similarity Tests**:
- 80x24: no visual output in legacy mode; TUI in interactive mode only for this ticket.
- 120x40: same as above.
- 200x50: same as above.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-010-80x24.txt

### TUI-011: Status bar component

**Phase**: Phase 1 — Core Layout
**Labels**: tui, phase-1, component:statusbar
**Dependencies**: TUI-006, TUI-008, TUI-009

**Description**:
Implement fixed status line with host, timestamp, path, branch, MCP alerts, and second-line mode hints.
Use info from existing run context and test stubs.
This component is always visible and should never scroll away even when viewport content changes.

**Acceptance Criteria**:
- [ ] Renders default, dirty git, MCP failure, and accept-edits/plan-mode variants.
- [ ] Timestamp updates every second in HH:MM:SS format.
- [ ] Supports optional second status line when permission mode active.
- [ ] Keeps width-safe truncation for very short terminals.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/statusbar/model.go` — status state and update methods.
- `cmd/harnesscli/tui/components/statusbar/view.go` — renderer.

**TDD Requirements** (write these tests FIRST):
- `TestTUI011_StatusbarShowsDefaultState` — baseline render.
- `TestTUI011_StatusbarShowsMCPFailureCount` — alert rendering.

**Regression Test Requirements**:
- Concurrent access: rapid status updates from multiple goroutines should serialize.
- Boundary conditions: empty path, missing branch, and 1-char terminal width.
- Error paths: git command failure should show fallback branch text.

**Visual Similarity Tests**:
- 80x24: status occupies the final fixed line and truncates long path.
- 120x40: full status fields visible with `ctrl+g` hint optional.
- 200x50: branch and MCP alerts all visible with right-aligned session token.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-011-80x24.txt

### TUI-012: Multiline input area component

**Phase**: Phase 1 — Core Layout
**Labels**: tui, phase-1, component:input
**Dependencies**: TUI-002, TUI-003, TUI-009

**Description**: Implement text input with visible prompt, multi-line editing support, and newline insertion behavior for Enter and return semantics.
Support `Ctrl+G` handoff as a command action and preserve history navigation.
Input area must render between separator bars exactly as design indicates.

**Acceptance Criteria**:
- [ ] Enter inserts newline by default and submits only on selected submit chord.
- [ ] Prompt cursor and reverse-video cursor indicator are visible.
- [ ] Input area supports paste and multi-byte UTF-8 text correctly.
- [ ] Empty prompt behavior remains stable in all overlay states.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/input/model.go` — input state and text transforms.
- `cmd/harnesscli/tui/components/input/view.go` — rendering of prompt/input/content.

**TDD Requirements** (write these tests FIRST):
- `TestTUI012_MultilineInputCapturesNewline` — newline insertion.
- `TestTUI012_HistoryNavigationInInput` — up/down arrow selection.

**Regression Test Requirements**:
- Concurrent access: typing and async status updates concurrently should not corrupt buffer.
- Boundary conditions: huge line length beyond width with wrapping.
- Error paths: invalid cursor movement requests when buffer empty.

**Visual Similarity Tests**:
- 80x24: input block at bottom enclosed by separator lines.
- 120x40: multi-line prompt and wrapped text visible.
- 200x50: two-line input with cursor clearly visible.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-012-80x24.txt

### TUI-013: Scrollable viewport component

**Phase**: Phase 1 — Core Layout
**Labels**: tui, phase-1, component:viewport
**Dependencies**: TUI-005, TUI-012

**Description**:
Introduce scrollback viewport that renders messages between input and status bar.
Use a stable frame-based container with cursor-independent scrolling and deterministic truncation policy for non-visible history.
The viewport should handle message overflow without losing metadata.

**Acceptance Criteria**:
- [ ] Viewport can append messages and preserve order.
- [ ] Auto-scroll-to-bottom is enabled by default.
- [ ] Manual scroll prevents jump on incoming data if user positioned mid-history.
- [ ] Works without external terminal mouse support.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/viewport/model.go` — viewport state.
- `cmd/harnesscli/tui/components/viewport/layout.go` — derived dimensions and offsets.

**TDD Requirements** (write these tests FIRST):
- `TestTUI013_ViewportAppendsAndKeepsOrder` — sequential insert order.
- `TestTUI013_UserManualScrollPausesAutoScroll` — manual scroll state respected.

**Regression Test Requirements**:
- Concurrent access: send viewport writes while manual scrolling thread adjusts offset.
- Boundary conditions: zero-height viewport and zero-message state.
- Error paths: scroll beyond min/max should clamp.

**Visual Similarity Tests**:
- 80x24: viewport shows 3-4 lines and no overlap with status/input regions.
- 120x40: viewport taller with at least 10 message rows.
- 200x50: full-scroll interactions visible with top/bottom hints.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-013-80x24.txt

### TUI-014: Streaming text renderer component

**Phase**: Phase 1 — Core Layout
**Labels**: tui, phase-1, component:streamrenderer
**Dependencies**: TUI-013, TUI-009

**Description**:
Implement renderer that consumes partial assistant/text chunks and produces progressive blocks.
The renderer should avoid flicker by using full block updates and stable indentation.
Include token-count-aware completion summary and placeholder display when stream is empty.

**Acceptance Criteria**:
- [ ] Incremental chunks concatenate preserving order.
- [ ] Renderer can transition from streaming state to complete state.
- [ ] Works with spinner and tip lines while tool calls are pending.
- [ ] Truncates very large messages and exposes expand affordance markers.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/streamrenderer/model.go` — model for streamed content.
- `cmd/harnesscli/tui/components/streamrenderer/view.go` — output rendering.

**TDD Requirements** (write these tests FIRST):
- `TestTUI014_StreamRendererAccumulatesChunks` — verify ordering and whitespace handling.
- `TestTUI014_StreamRendererShowsCompletionSummary` — verify completion footer string.

**Regression Test Requirements**:
- Concurrent access: parallel chunk sends from multiple goroutines should not reorder for same message ID.
- Boundary conditions: zero-length chunks and empty complete signal.
- Error paths: out-of-order chunk IDs should be isolated by call map and not crash.

**Visual Similarity Tests**:
- 80x24: first streamed paragraph visible as soon as first chunk arrives.
- 120x40: text wraps cleanly across two columns of width and spacing preserved.
- 200x50: long stream remains readable with minimal reflow flicker.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-014-80x24.txt

### TUI-015: Layout composition in root model

**Phase**: Phase 1 — Core Layout
**Labels**: tui, phase-1, component:layout
**Dependencies**: TUI-011, TUI-012, TUI-013, TUI-014

**Description**:
Compose status bar, viewport, and input areas into a single root view with deterministic ordering.
Implement separator lines and session-name chip logic from design research.
Add minimal fallback behavior when component heights exceed terminal limits.

**Acceptance Criteria**:
- [ ] Root `View()` returns components in fixed order.
- [ ] Top session-line and bottom status/input lines remain fixed.
- [ ] Layout switches cleanly across width changes.
- [ ] Overlay modes do not destroy base composition.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/model.go` — top-level layout assembly.
- `cmd/harnesscli/tui/components/layout/container.go` — shared composition helper.

**TDD Requirements** (write these tests FIRST):
- `TestTUI015_RootViewOrdersComponents` — exact region ordering.
- `TestTUI015_SessionNameRendersRightAligned` — header alignment.

**Regression Test Requirements**:
- Concurrent access: simultaneous window-size and message updates should not produce inconsistent region bounds.
- Boundary conditions: one-line terminal and extremely wide terminal behaviors.
- Error paths: panic should not occur when session name is empty.

**Visual Similarity Tests**:
- 80x24: all three major regions visible and non-overlapping.
- 120x40: expanded viewport region with stable separators.
- 200x50: top and bottom bars anchored and center content reflows.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-015-80x24.txt

### TUI-016: Line wrapping and word-wrap utilities

**Phase**: Phase 1 — Core Layout
**Labels**: tui, phase-1, component:viewport
**Dependencies**: TUI-005, TUI-014

**Description**:
Create shared wrapping helpers for plain text, markdown lines, and status snippets.
Ensure continuation rules match the observed indentation conventions (user gray block, assistant two-space offset, tool result tree offset).
This utility is shared across message bubbles, tool outputs, and diff viewers.

**Acceptance Criteria**:
- [ ] `WrapText` returns deterministic segments for same width and unicode width.
- [ ] ANSI-safe width handling avoids breaking escape sequences.
- [ ] Supports hard wraps for code and no-wrap regions.
- [ ] Exposes helper for bubble indentation.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/streamrenderer/tokenizer.go` — wrapping rules for streamed text.
- `cmd/harnesscli/tui/components/messagebubble/shared.go` — apply message-specific indentation.

**TDD Requirements** (write these tests FIRST):
- `TestTUI016_WrapEmojiAndWideChars` — validate rune width handling.
- `TestTUI016_ToolResultIndentPreserved` — verify tree connector indentation.

**Regression Test Requirements**:
- Concurrent access: parallel wrapping calls should be reentrant and safe.
- Boundary conditions: zero width and one-character width fallback.
- Error paths: malformed ANSI spans should be handled gracefully.

**Visual Similarity Tests**:
- 80x24: wrapped assistant text aligns under prefix and no horizontal overflow.
- 120x40: long prompt lines break without tearing in message body.
- 200x50: long code lines keep indentation and avoid wrapping unexpectedly.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-016-80x24.txt

### TUI-017: Auto-scroll-to-bottom on new content

**Phase**: Phase 1 — Core Layout
**Labels**: tui, phase-1, component:viewport
**Dependencies**: TUI-013, TUI-014

**Description**:
Implement automatic scroll strategy that follows new messages unless user has explicitly scrolled up.
When the viewport is pinned to bottom, incoming tool and assistant updates should keep latest content visible.
When user is browsing history, incoming messages set a “new content available” indicator.

**Acceptance Criteria**:
- [ ] Auto-scroll enabled by default after each append.
- [ ] Manual scroll overrides auto-scroll until user returns bottom.
- [ ] New-message indicator appears when not at bottom.
- [ ] Indicator clears when user jumps to latest.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/viewport/model.go` — scroll position logic.
- `cmd/harnesscli/tui/components/viewport/state.go` — add marker for pending new messages.

**TDD Requirements** (write these tests FIRST):
- `TestTUI017_AutoScrollPinsToBottomOnAppend` — default behavior.
- `TestTUI017_ManualScrollStopsAutoScroll` — user-control behavior.

**Regression Test Requirements**:
- Concurrent access: rapid append while scroll actions occur.
- Boundary conditions: append while viewport empty.
- Error paths: invalid scroll position resets to valid range.

**Visual Similarity Tests**:
- 80x24: latest turn always visible while no manual scroll.
- 120x40: scroll indicator appears when not at bottom.
- 200x50: jump-to-bottom marker shown and works.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-017-80x24.txt

### TUI-018: Text cursor and caret display

**Phase**: Phase 1 — Core Layout
**Labels**: tui, phase-1, component:input
**Dependencies**: TUI-012

**Description**:
Render explicit caret in input area as reverse-video block in prompt line.
Cursor should track insertion point across wrapped lines and survive edit operations.
Caret rendering must not alter prompt semantics or interfere with ANSI styling.

**Acceptance Criteria**:
- [ ] Caret visible and positioned according to cursor index.
- [ ] Cursor survives line breaks and multiline prompt.
- [ ] Caret color/style follows theme tokens.
- [ ] Hidden only when input overlay intentionally disables editing.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/input/view.go` — render caret.
- `cmd/harnesscli/tui/components/input/multiline.go` — cursor index mapping.

**TDD Requirements** (write these tests FIRST):
- `TestTUI018_CursorMovesWithMultilineText` — verify coordinate mapping.
- `TestTUI018_CaretShowsOnBlankInput` — caret appears on empty line.

**Regression Test Requirements**:
- Concurrent access: typing and history navigation concurrently should not desync cursor.
- Boundary conditions: cursor at start and end positions.
- Error paths: negative cursor index corrected to zero.

**Visual Similarity Tests**:
- 80x24: visible cursor block in prompt line.
- 120x40: cursor remains with cursor movement keys.
- 200x50: cursor remains on wrapped continuation lines.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-018-80x24.txt

### TUI-019: Border and separator styles

**Phase**: Phase 1 — Core Layout
**Labels**: tui, phase-1, component:layout
**Dependencies**: TUI-006, TUI-015

**Description**:
Implement canonical separators and borders using unicode line characters and dim styles.
Use top/bottom separators around input area and optional boxed borders for dialogs and pickers.
Borders should degrade gracefully to plain ascii when unsupported.

**Acceptance Criteria**:
- [ ] Input separators render as `─` where supported.
- [ ] Status and session chip lines use full-width align and fallback for narrow terminals.
- [ ] Dialog borders and box styles follow requested characters.
- [ ] Fallback to `-`/`|` when terminal does not support unicode.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/theme.go` — separator token definitions.
- `cmd/harnesscli/tui/components/layout/container.go` — border composition.

**TDD Requirements** (write these tests FIRST):
- `TestTUI019_SeparatorsRenderInOrder` — verify top and bottom lines.
- `TestTUI019_BorderFallbackAscii` — non-unicode fallback.

**Regression Test Requirements**:
- Concurrent access: many view renders should not mutate global border state.
- Boundary conditions: tiny width uses minimal separator length.
- Error paths: unsupported font/terminal runes should not panic.

**Visual Similarity Tests**:
- 80x24: two separator lines with proper placement around input.
- 120x40: no clipping and full-width separators.
- 200x50: dotted/dashed separators used where configured.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-019-80x24.txt

### TUI-020: Integration test: full layout renders without panic

**Phase**: Phase 1 — Core Layout
**Labels**: tui, phase-1, component:root
**Dependencies**: TUI-011, TUI-012, TUI-013, TUI-014, TUI-015, TUI-016, TUI-017, TUI-018, TUI-019

**Description**:
Add an end-to-end layout test that renders the entire screen at 80x24, 120x40, and 200x50 with synthetic conversation and tool events.
The test uses teatest snapshot baseline enforcement to prove no render panic.
This is the first full-stack visual acceptance gate.

**Acceptance Criteria**:
- [ ] No panics during program start, input, event injection, and resize sequence.
- [ ] Snapshots stabilize with deterministic output.
- [ ] Layout remains valid under empty and populated conversation states.
- [ ] Terminal sizes from section requirements all pass.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/integration_layout_test.go` — full layout smoke test.
- `cmd/harnesscli/testhelpers/snapshot.go` — terminal-size-aware snapshot helper.

**TDD Requirements** (write these tests FIRST):
- `TestTUI020_LayoutStableAt80x24` — smoke at minimum size.
- `TestTUI020_LayoutStableAt120x40And200x50` — wider terminal validation.

**Regression Test Requirements**:
- Concurrent access: parallel test execution across three layout sizes.
- Boundary conditions: empty data + active overlay + terminal resize.
- Error paths: invalid resize values from mocked messages.

**Visual Similarity Tests**:
- 80x24: complete baseline with header separators and controls.
- 120x40: complete baseline with wider viewport and clean whitespace.
- 200x50: complete baseline with expanded scrollback region and spacing.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-020-80x24.txt

### TUI-021: User message bubble component

**Phase**: Phase 2 — Chat UX
**Labels**: tui, phase-2, component:messagebubble
**Dependencies**: TUI-014, TUI-019

**Description**:
Implement user bubble with dark-gray full-width background and correct prompt symbol prefix.
Add trailing blank line and continuation indentation behavior.
Ensure message text wrapping and background fill remain stable across widths.

**Acceptance Criteria**:
- [ ] User messages render with background style and `❯` prefix.
- [ ] Continuation lines align with two-space continuation rule.
- [ ] Blank line after each user block is always present.
- [ ] Empty message handled as minimal bubble with cursor only.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/messagebubble/user.go` — message rendering.
- `cmd/harnesscli/tui/components/messagebubble/shared.go` — shared bubble utilities.

**TDD Requirements** (write these tests FIRST):
- `TestTUI021_UserBubbleRendersPromptAndBackground` — style validation.
- `TestTUI021_UserBubbleIncludesBlankTrailingLine` — spacing validation.

**Regression Test Requirements**:
- Concurrent access: concurrent user bubble renders for different messages.
- Boundary conditions: long single words and very short terminal widths.
- Error paths: nil/invalid message content falls back safely.

**Visual Similarity Tests**:
- 80x24: user bubble occupies fixed width with no bleed.
- 120x40: multiline user message with continuation and trailing gap.
- 200x50: user bubble style remains obvious in dense history.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-021-80x24.txt

### TUI-022: AI response bubble component

**Phase**: Phase 2 — Chat UX
**Labels**: tui, phase-2, component:messagebubble
**Dependencies**: TUI-021

**Description**:
Create assistant response bubble with white `⏺` and consistent two-space content indentation.
Render titles in bold/italic/underline style when provided and preserve plain text fallback.
No background by default.

**Acceptance Criteria**:
- [ ] Assistant bubble renders title and content with appropriate emphasis.
- [ ] Symbol color remains bright white for text and bright green for tool-use prefix is delegated from tool component.
- [ ] Body lines wrap according to viewport width.
- [ ] Empty response shows no artifacts or placeholder junk.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/messagebubble/assistant.go` — full bubble rendering.
- `cmd/harnesscli/tui/components/streamrenderer/styles.go` — emphasis mapping.

**TDD Requirements** (write these tests FIRST):
- `TestTUI022_AssistantBubbleRendersTitle` — title styling is present.
- `TestTUI022_AssistantBubbleWrapsBody` — width-sensitive wrapping.

**Regression Test Requirements**:
- Concurrent access: two assistant bubbles added in rapid succession.
- Boundary conditions: empty title and huge body text.
- Error paths: malformed markdown should degrade to plain text.

**Visual Similarity Tests**:
- 80x24: assistant bubble appears directly below prior tool call.
- 120x40: body and title are separated and readable.
- 200x50: long responses with multiple paragraphs do not overflow.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-022-80x24.txt

### TUI-023: Markdown rendering via glamour

**Phase**: Phase 2 — Chat UX
**Labels**: tui, phase-2, component:markdown
**Dependencies**: TUI-022

**Description**:
Integrate `glamour` rendering for headings, bold, inline code, bullets, tables, and code blocks in assistant messages.
The existing direct markdown-like token use in tests will move from ANSI manual formatting to normalized renderer output.
Ensure deterministic style output for terminal compatibility.

**Acceptance Criteria**:
- [ ] Headings render with requested emphasis.
- [ ] Inline code uses bright blue foreground.
- [ ] Tables and code blocks preserve structure.
- [ ] Renderer can be bypassed when disabled in config.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/messagebubble/markdown.go` — glamour adapter.
- `cmd/harnesscli/tui/components/messagebubble/assistant.go` — switch to markdown renderer.

**TDD Requirements** (write these tests FIRST):
- `TestTUI023_MarkdownHeadingsRender` — headings have expected style tokens.
- `TestTUI023_MarkdownTableRendersAsciiBorders` — table border output.

**Regression Test Requirements**:
- Concurrent access: render markdown in parallel with different payloads.
- Boundary conditions: malformed markdown should still render partial text.
- Error paths: renderer error falls back to raw markdown text.

**Visual Similarity Tests**:
- 80x24: markdown heading + inline code remain visible and styled.
- 120x40: one markdown table with aligned columns.
- 200x50: code block with syntax color in 3+ lines.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-023-80x24.txt

### TUI-024: Thinking spinner with rotating verbs

**Phase**: Phase 2 — Chat UX
**Labels**: tui, phase-2, component:spinner
**Dependencies**: TUI-001, TUI-006, TUI-009

**Description**:
Implement spinner sequence (`✶ · ✻ ✽ ✳ ✢`) with whimsical verbs and optional token-speed metrics.
Behavior includes working-duration suffix and tip lines during long thinking.
This should operate with a deterministic ticker for testability.

**Acceptance Criteria**:
- [ ] Spinner frames animate at testable interval.
- [ ] Verb pool randomization is seedable.
- [ ] Shows duration and token deltas when provided.
- [ ] Stops cleanly and emits completion line.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/spinner/model.go` — timer and tick state.
- `cmd/harnesscli/tui/components/spinner/verbs.go` — verb catalog and selection rules.

**TDD Requirements** (write these tests FIRST):
- `TestTUI024_SpinnerCyclesFrames` — sequence order.
- `TestTUI024_SpinnerAddsDurationAfterThreshold` — duration formatting.

**Regression Test Requirements**:
- Concurrent access: multiple concurrent runs should keep independent spinner state.
- Boundary conditions: no available verbs should fallback to default list.
- Error paths: timer cancellation should stop ticker immediately.

**Visual Similarity Tests**:
- 80x24: spinner line appears above tip while tool active.
- 120x40: tip line appears below spinner with connector indentation.
- 200x50: spinner persists across state changes without duplicate lines.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-024-80x24.txt

### TUI-025: Thinking spinner with compact completion line

**Phase**: Phase 2 — Chat UX
**Labels**: tui, phase-2, component:spinner
**Dependencies**: TUI-024

**Description**:
After completion, show worked duration summary (`✻ Worked for 1m 0s`) and remove spinner.
The completion should appear in same position as spinner and optionally keep summary for one viewport cycle.

**Acceptance Criteria**:
- [ ] Completion text appears with symbol and duration.
- [ ] Spinner replacement avoids flicker in the transition.
- [ ] Works when token counters are missing.
- [ ] Completion is available to transcript history.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/spinner/model.go` — completion mode.
- `cmd/harnesscli/tui/components/streamrenderer/model.go` — finalize state on done.

**TDD Requirements** (write these tests FIRST):
- `TestTUI025_SpinningToCompletionLine` — transition from spinner to summary.
- `TestTUI025_CompletionPersistsForNFrames` — summary visibility behavior.

**Regression Test Requirements**:
- Concurrent access: completion while tool-call stream still emits late chunks.
- Boundary conditions: zero duration and one-second durations.
- Error paths: malformed timing strings are sanitized.

**Visual Similarity Tests**:
- 80x24: completion line visible after stream end.
- 120x40: summary appears exactly once per thinking cycle.
- 200x50: summary aligns with top of response block.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-025-80x24.txt

### TUI-026: Tool use collapsed display

**Phase**: Phase 2 — Chat UX
**Labels**: tui, phase-2, component:toolcall
**Dependencies**: TUI-021, TUI-014

**Description**:
Render tool calls in collapsed form by default: single-line summary `⏺ Tool (args)` and optional in-progress ellipsis.
Supports quick toggle to expanded view via input/control key.
Ensure right aligned metadata like elapsed and attempt count available for future expansion.

**Acceptance Criteria**:
- [ ] Collapsed view follows symbol color rules.
- [ ] In-progress appends ellipsis and notifies expansion hint.
- [ ] Completed tool call shows summary and status.
- [ ] Tool names and key params are safely truncated.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/toolcall/model.go` — state and render mode.
- `cmd/harnesscli/tui/components/toolcall/view.go` — collapsed row rendering.

**TDD Requirements** (write these tests FIRST):
- `TestTUI026_CollapsedToolcallShowsPrefixAndHint` — summary rendering.
- `TestTUI026_InProgressToolcallRendersEllipsis` — active state check.

**Regression Test Requirements**:
- Concurrent access: two tool calls running concurrently in map.
- Boundary conditions: tool without name/args should not crash.
- Error paths: tool stream failure updates status in collapsed line.

**Visual Similarity Tests**:
- 80x24: collapsed tool row appears below user message with one-line summary.
- 120x40: truncated tool names include hint line.
- 200x50: multiple collapsed calls stack with clear separators.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-026-80x24.txt

### TUI-027: Timestamps on messages

**Phase**: Phase 2 — Chat UX
**Labels**: tui, phase-2, component:messagebubble
**Dependencies**: TUI-011, TUI-013

**Description**:
Add optional timestamp rendering on message rows and expanded tool rows with right-aligned metadata.
Timestamp style should be non-intrusive and theme-controlled.
For expanded tool rows, include `09:09 PM` and model label when available.

**Acceptance Criteria**:
- [ ] Timestamp optional via settings and visible in transcripts.
- [ ] Right alignment honors viewport width.
- [ ] Millisecond precision not required; minute resolution sufficient.
- [ ] Works in compact and verbose modes.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/messagebubble/shared.go` — add timestamp utilities.
- `cmd/harnesscli/tui/components/toolcall/view.go` — right-aligned metadata.

**TDD Requirements** (write these tests FIRST):
- `TestTUI027_MessageTimestampAppears` — baseline message.
- `TestTUI027_ToolTimestampAlignsRight` — alignment under varied widths.

**Regression Test Requirements**:
- Concurrent access: timestamp updates across many messages.
- Boundary conditions: timezone offset and empty timestamp source.
- Error paths: invalid timestamp parse falls back to local rendering.

**Visual Similarity Tests**:
- 80x24: timestamp absent in minimal compact mode.
- 120x40: timestamp visible on tool detail header.
- 200x50: detailed metadata appears aligned right on expanded line.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-027-80x24.txt

### TUI-028: Copy-to-clipboard keybinding

**Phase**: Phase 2 — Chat UX
**Labels**: tui, phase-2, component:input
**Dependencies**: TUI-012, TUI-011

**Description**:
Implement copy action for last assistant response and selected message where supported.
Use OSC52 if terminal permits and fallback to in-memory status hint when unavailable.
This should include status indicator and explicit success/failure messaging.

**Acceptance Criteria**:
- [ ] `Ctrl+S` copies selected content with cross-platform fallback.
- [ ] Status line reflects success/failure.
- [ ] Works for plain text and markdown-expanded outputs.
- [ ] No clipboard call occurs in headless CI mode.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/keys.go` — bind copy key.
- `cmd/harnesscli/tui/model.go` — copy handler and status update.

**TDD Requirements** (write these tests FIRST):
- `TestTUI028_CopyActionCapturesLastAssistantText` — verifies selected buffer.
- `TestTUI028_CopyActionGracefulInHeadless` — fallback behavior in non-interactive mode.

**Regression Test Requirements**:
- Concurrent access: copy while message stream is updating.
- Boundary conditions: empty response buffer.
- Error paths: clipboard write errors update status without panic.

**Visual Similarity Tests**:
- 80x24: status message briefly indicates copied or unavailable.
- 120x40: copy status and key hint visible.
- 200x50: copy status with non-active command overlay.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-028-80x24.txt

### TUI-029: Clear screen command (/clear)

**Phase**: Phase 2 — Chat UX
**Labels**: tui, phase-2, component:input
**Dependencies**: TUI-041, TUI-013

**Description**:
Implement `/clear` command from input command parser and inline result message.
Clearing should prune scrollback while preserving model and session state where possible.
Show inline response `⎿ cleared history` and update internal history counters.

**Acceptance Criteria**:
- [ ] `/clear` removes visible conversation lines.
- [ ] Inline response emits and is rendered as command result row.
- [ ] Session metadata (name, model, etc.) is retained.
- [ ] No destructive change to settings or permissions.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/cmd_parser.go` — parse clear command.
- `cmd/harnesscli/tui/model.go` — command execution and state reset.

**TDD Requirements** (write these tests FIRST):
- `TestTUI029_ClearCommandEmptiesConversation` — clears viewport history.
- `TestTUI029_ClearCommandDisplaysInlineResult` — inline connector shown.

**Regression Test Requirements**:
- Concurrent access: clear while stream is still active.
- Boundary conditions: clear on already-empty history.
- Error paths: clear command during non-ready state returns status hint.

**Visual Similarity Tests**:
- 80x24: conversation area resets to minimal state while headers remain.
- 120x40: clear connector appears in expected format.
- 200x50: no stale tool rows remain after clear.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-029-80x24.txt

### TUI-030: Message history virtualization

**Phase**: Phase 2 — Chat UX
**Labels**: tui, phase-2, component:viewport
**Dependencies**: TUI-013, TUI-016

**Description**:
Render only visible messages to avoid O(N) render cost in long sessions.
Keep a bounded window with page granularity and preserve full history in backing store.
Important for production reliability for sessions with large tool logs and diff outputs.

**Acceptance Criteria**:
- [ ] Viewport rendering time remains bounded with >200 messages.
- [ ] Off-screen messages are not rendered into output.
- [ ] Scroll operations page to hidden regions as expected.
- [ ] Backing store can be pruned by max-history policy.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/viewport/virtualization.go` — virtualization algorithm.
- `cmd/harnesscli/tui/components/viewport/model.go` — cached total and visible window.

**TDD Requirements** (write these tests FIRST):
- `TestTUI030_VirtualizationSkipsOffscreenMessages` — render-size check.
- `TestTUI030_ScrollRevealsOffscreenMessage` — scroll logic.

**Regression Test Requirements**:
- Concurrent access: adding messages while scrolling quickly.
- Boundary conditions: exactly full-screen worth and one over full-screen worth.
- Error paths: corrupted height values should clamp and avoid panic.

**Visual Similarity Tests**:
- 80x24: only tail region visible and no truncation warning.
- 120x40: deep history browse with scroll keys.
- 200x50: large history remains performant and visually stable.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-030-80x24.txt

### TUI-031: Tool use expanded view

**Phase**: Phase 3 — Tool Use
**Labels**: tui, phase-3, component:toolcall
**Dependencies**: TUI-026

**Description**:
Implement expanded tool-call view with params, nested calls, result summary, hooks, timestamps, and truncation info.
Toggle via `Ctrl+O` and preserve collapsed default in normal mode.
This view should show tree connector and right aligned metadata.

**Acceptance Criteria**:
- [ ] Expanded view shows params, tool outputs, and truncated sections.
- [ ] Toggle preserves scroll position and user input focus.
- [ ] Nested calls render with indentation and parent/child relationship.
- [ ] Duration and token summary displayed.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/toolcall/view.go` — expanded row layout.
- `cmd/harnesscli/tui/model.go` — toggle key handling.

**TDD Requirements** (write these tests FIRST):
- `TestTUI031_ExpandedToolcallShowsDetails` — verify detailed payload displayed.
- `TestTUI031_ToggleExpandedPreservesState` — state continuity after toggle.

**Regression Test Requirements**:
- Concurrent access: simultaneous updates to expanded and collapsed rows.
- Boundary conditions: very large payloads should paginate or truncate.
- Error paths: nested tool call without parent should render gracefully.

**Visual Similarity Tests**:
- 80x24: expanded details render within viewport height with no clipping.
- 120x40: nested child calls visible and connected.
- 200x50: expanded block consumes more rows with complete metadata.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-031-80x24.txt

### TUI-032: Diff viewer component

**Phase**: Phase 3 — Tool Use
**Labels**: tui, phase-3, component:diffviewer
**Dependencies**: TUI-031

**Description**:
Implement diff display component for file read/write/edit summaries with `╌` bordered output.
It should render line numbers, plus/minus prefixes, and support truncation with expansion hint.
Integrate with permission flow and direct assistant references.

**Acceptance Criteria**:
- [ ] Diff viewer accepts unified diff payload and renders deterministic format.
- [ ] Line numbers and gutter are aligned.
- [ ] Truncation message for very long diffs appears.
- [ ] Viewer supports copy-ready plain text output.

**Files to Create/modify**:
- `cmd/harnesscli/tui/components/diffviewer/view.go` — diff rendering.
- `cmd/harnesscli/tui/components/diffviewer/formatter.go` — parser and clipping.

**TDD Requirements** (write these tests FIRST):
- `TestTUI032_DiffViewerRendersHeadersAndHunks` — diff structure.
- `TestTUI032_DiffViewerTruncatesLongDiff` — truncation marker appears.

**Regression Test Requirements**:
- Concurrent access: multiple diffs open in same session.
- Boundary conditions: empty diff and single-line diff.
- Error paths: malformed diff text should show fallback message.

**Visual Similarity Tests**:
- 80x24: small diff rendered in bordered box without overflow.
- 120x40: larger diff with plus/minus lines and preserved indentation.
- 200x50: diff viewer with enough rows and right alignment of line numbers.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-032-80x24.txt

### TUI-033: Permission prompt overlay

**Phase**: Phase 3 — Tool Use
**Labels**: tui, phase-3, component:permissionprompt
**Dependencies**: TUI-021, TUI-031

**Description**:
Build modal permission prompt used by file-edit/read/bash actions with Yes/No/Allow-all modes.
Prompts should support tab-to-amend and command-specific extra options.
Render with overlay focus so user cannot edit input until resolved or dismissed.

**Acceptance Criteria**:
- [ ] Prompt shows resource target, tool type, and action options.
- [ ] Supports yes/no and session-scoped allow modes.
- [ ] Supports `Tab` amend path when available.
- [ ] Exits with explicit result and resumes stream state.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/permissionprompt/model.go` — permission state machine.
- `cmd/harnesscli/tui/components/permissionprompt/view.go` — modal rendering.

**TDD Requirements** (write these tests FIRST):
- `TestTUI033_FileEditPromptShowsYesNoScope` — file write prompt.
- `TestTUI033_PermissionPromptConsumesKeyInput` — key-driven option selection.

**Regression Test Requirements**:
- Concurrent access: multiple permission prompts arriving close in time.
- Boundary conditions: prompt timeout and missing tool metadata.
- Error paths: unknown tool action falls back to ask mode.

**Visual Similarity Tests**:
- 80x24: prompt centered and focusable in overlay state.
- 120x40: options and hint lines visible.
- 200x50: prompt content scrolls only when overflowing.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-033-80x24.txt

### TUI-034: File operation display summaries

**Phase**: Phase 3 — Tool Use
**Labels**: tui, phase-3, component:toolcall
**Dependencies**: TUI-031, TUI-032

**Description**:
Add concise summaries for read/write/edit operations like `Added 1 line` and `Wrote 2 lines to utils.go`.
Summaries should appear under collapsed tool rows with `⎿` and consistent icons.
Used for fast scan without opening diff every time.

**Acceptance Criteria**:
- [ ] Tool operation actions produce one-line summary lines.
- [ ] Summary is derived from SSE payload or diff metadata.
- [ ] No summary produced for unsupported tools.
- [ ] Summary style matches tool and warning colors.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/toolcall/model.go` — compute operation summaries.
- `cmd/harnesscli/tui/components/toolcall/view.go` — render summary lines.

**TDD Requirements** (write these tests FIRST):
- `TestTUI034_FileOpSummaryRead` — read operation summary text.
- `TestTUI034_FileOpSummaryWrite` — write operation summary includes file name.

**Regression Test Requirements**:
- Concurrent access: overlapping file operations and ordering under concurrent tool calls.
- Boundary conditions: binary files or empty file names.
- Error paths: missing operation count should be omitted safely.

**Visual Similarity Tests**:
- 80x24: one-line summary immediately below collapsed call.
- 120x40: long file names are ellipsized.
- 200x50: summary remains in one visual block with no wrapping drift.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-034-80x24.txt

### TUI-035: Bash output display with truncation

**Phase**: Phase 3 — Tool Use
**Labels**: tui, phase-3, component:toolcall
**Dependencies**: TUI-031

**Description**:
When tool output includes shell/batch command output, render a concise line and truncated body with line-count hints.
Long outputs should show `+N lines (ctrl+o to expand)` pattern in collapsed mode.
Expanded mode can show complete output or truncated window with continuation.

**Acceptance Criteria**:
- [ ] Detect shell output type and render command summary.
- [ ] Truncate using character and line thresholds.
- [ ] Expansion marker and collapsed summary include exact line counts.
- [ ] No raw control chars in output rendering.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/toolcall/state.go` — output classification.
- `cmd/harnesscli/tui/components/toolcall/view.go` — truncation rendering.

**TDD Requirements** (write these tests FIRST):
- `TestTUI035_BashOutputTruncatesLongText` — truncation marker and count.
- `TestTUI035_BashOutputShowsCommandLabel` — command line shown in header.

**Regression Test Requirements**:
- Concurrent access: simultaneous stdout/stderr-like chunks.
- Boundary conditions: empty output and one-line output.
- Error paths: malformed payload with missing command field.

**Visual Similarity Tests**:
- 80x24: short output one-line summary below tool call.
- 120x40: collapsed mode shows line count badge.
- 200x50: expanded mode displays full output in bounded box.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-035-80x24.txt

### TUI-036: Streaming tool result updates

**Phase**: Phase 3 — Tool Use
**Labels**: tui, phase-3, component:toolcall
**Dependencies**: TUI-014, TUI-031

**Description**:
Handle tool tool-call result chunks that stream after tool start.
Map chunk IDs to tool state and render partial body in expanded/collapsed mode.
Late chunks should still update latest known summary without duplicating line breaks.

**Acceptance Criteria**:
- [ ] Tool result partials append incrementally.
- [ ] No duplicated chunk insertion when same chunk resent.
- [ ] Completed message replaces incremental summary when final arrives.
- [ ] Supports simultaneous chunks for multiple tool calls.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/toolcall/model.go` — chunk accumulator.
- `cmd/harnesscli/tui/messages.go` — new `ToolCallChunkMsg` mapping.

**TDD Requirements** (write these tests FIRST):
- `TestTUI036_ToolChunksAccumulatePerCallID` — map keyed behavior.
- `TestTUI036_ChunkOutOfOrderHandled` — ordering guard tests.

**Regression Test Requirements**:
- Concurrent access: tool calls with interleaved chunks from two IDs.
- Boundary conditions: duplicate chunk IDs and zero-size chunks.
- Error paths: corrupted partial payload should be captured as tool error.

**Visual Similarity Tests**:
- 80x24: first tool chunk appears quickly with partial marker.
- 120x40: streaming tool text increments in stable line sequence.
- 200x50: many chunks render without reflow jump.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-036-80x24.txt

### TUI-037: Tool error state display

**Phase**: Phase 3 — Tool Use
**Labels**: tui, phase-3, component:toolcall
**Dependencies**: TUI-036

**Description**:
Render tool errors explicitly with warning icon and concise reason while preserving stream continuity.
Errors should not drop prior output, but should switch tool state to failed and set actionable hints.

**Acceptance Criteria**:
- [ ] Failed tool rows use warning color and state icon.
- [ ] Error text wraps and does not crash viewport rendering.
- [ ] Suggestion hint can be shown in tooltip-like line.
- [ ] Error can be expanded for log detail.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/toolcall/model.go` — failed status transitions.
- `cmd/harnesscli/tui/components/toolcall/view.go` — warning styling.

**TDD Requirements** (write these tests FIRST):
- `TestTUI037_ToolErrorShowsWarningColor` — style and status.
- `TestTUI037_ErrorStatePreservesPreviousToolOutput` — previous output retained.

**Regression Test Requirements**:
- Concurrent access: error on one call while another is still running.
- Boundary conditions: error message empty and very long.
- Error paths: missing error payload fallback text.

**Visual Similarity Tests**:
- 80x24: warning indicator visible in collapsed row.
- 120x40: failed tool row shows hint on second line.
- 200x50: expanded row includes error details and stack summary.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-037-80x24.txt

### TUI-038: Nested tool call display

**Phase**: Phase 3 — Tool Use
**Labels**: tui, phase-3, component:toolcall
**Dependencies**: TUI-031

**Description**:
Support nested tool calls and sub-agent call rendering in tree form under parent nodes.
Child nodes should visually connect via indentation and markers; each node still supports collapse/expand.
Preserve parent/child relationship when events arrive asynchronously.

**Acceptance Criteria**:
- [ ] Nested relationships render with tree indentation.
- [ ] Parent row remains first with children grouped below.
- [ ] Expanded parent can show only requested descendants.
- [ ] Parent summary updates when child status changes.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/toolcall/nested.go` — tree-building helper.
- `cmd/harnesscli/tui/components/toolcall/view.go` — nested rendering.

**TDD Requirements** (write these tests FIRST):
- `TestTUI038_NestedToolCallsRenderHierarchy` — parent-child ordering.
- `TestTUI038_ChildStateUpdatesParent` — parent status summary update.

**Regression Test Requirements**:
- Concurrent access: nested updates arriving while flattening occurs.
- Boundary conditions: deep nesting >3 levels.
- Error paths: missing parent ID for child should fallback to top-level.

**Visual Similarity Tests**:
- 80x24: at least one child line under parent visible.
- 120x40: two-level nesting with clear tree symbols.
- 200x50: deep nesting remains readable with no truncation.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-038-80x24.txt

### TUI-039: Cancel in-flight tool call

**Phase**: Phase 3 — Tool Use
**Labels**: tui, phase-3, component:toolcall
**Dependencies**: TUI-031, TUI-037

**Description**:
Bind interruption (`Ctrl+C`) and Escape behavior for active tool/assistant streams to cancel current run.
Cancellation should send API-level cancellation message if endpoint exists and emit interruption message in conversation.
A partial response should not remain stale after cancel.

**Acceptance Criteria**:
- [ ] `Ctrl+C` during active tool streaming sends cancel command.
- [ ] User receives `Interrupted` notice under current turn.
- [ ] Input returns active immediately.
- [ ] Cancel while no active run has no effect.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/model.go` — run cancellation event handling.
- `cmd/harnesscli/tui/messages.go` — interruption message type.

**TDD Requirements** (write these tests FIRST):
- `TestTUI039_CtrlCCancelsActiveRun` — cancellation dispatch called once.
- `TestTUI039_InterruptShowsInterruptedNotice` — response content reflects cancellation.

**Regression Test Requirements**:
- Concurrent access: cancel while another run spawn occurs concurrently.
- Boundary conditions: cancel pressed rapidly twice.
- Error paths: cancel endpoint errors set status with retry hint.

**Visual Similarity Tests**:
- 80x24: interrupted notice appears with `⎿` connector.
- 120x40: input line active immediately after cancel.
- 200x50: no partial output beyond interruption notice.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-039-80x24.txt

### TUI-040: Tool timing and duration display

**Phase**: Phase 3 — Tool Use
**Labels**: tui, phase-3, component:toolcall
**Dependencies**: TUI-031

**Description**:
Add duration accounting for each tool call and per-run cumulative timing.
Display should show in collapsed and expanded rows with friendly units (`ms`, `s`, `m`).
Duration is used in prompt summaries and cost computations.

**Acceptance Criteria**:
- [ ] Tool start and completion timestamps produce durations.
- [ ] Duration formatting is stable and localized.
- [ ] Duration shown in tool line and optionally in expanded metadata.
- [ ] Failed tools still show measured duration.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/toolcall/state.go` — timer fields and methods.
- `cmd/harnesscli/tui/components/toolcall/view.go` — duration rendering.

**TDD Requirements** (write these tests FIRST):
- `TestTUI040_ToolCallDurationRendered` — completed tool with duration.
- `TestTUI040_FailedToolDurationRendered` — still shown on failure.

**Regression Test Requirements**:
- Concurrent access: parallel tool calls start/finish with overlapping windows.
- Boundary conditions: zero-duration and long-duration tool calls.
- Error paths: missing start time should defer until completed with safe fallback.

**Visual Similarity Tests**:
- 80x24: duration label appears inline with summary.
- 120x40: expanded metadata includes duration in right side.
- 200x50: long run durations remain readable.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-040-80x24.txt

### TUI-041: Slash command parser

**Phase**: Phase 4 — Commands and Navigation
**Labels**: tui, phase-4, component:input
**Dependencies**: TUI-028, TUI-012

**Description**:
Implement parser that recognizes `/command` with optional arguments and dispatches strongly typed command handlers.
Support alias resolution and quoted arguments where practical.
Parser should reject unknown commands with friendly inline feedback.

**Acceptance Criteria**:
- [ ] Recognizes command token, trim spaces, and argument splitting.
- [ ] Dispatch map includes all built-ins for next tickets.
- [ ] Unknown command returns user-visible error in viewport.
- [ ] Commands are case-insensitive where appropriate.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/cmd_parser.go` — parsing and dispatch table.
- `cmd/harnesscli/tui/cmd_result.go` — command handler interface and result object.

**TDD Requirements** (write these tests FIRST):
- `TestTUI041_ParseClearCommand` — parse command/args.
- `TestTUI041_UnknownCommandReturnsHint` — unknown command error path.

**Regression Test Requirements**:
- Concurrent access: parse commands while stream events update history.
- Boundary conditions: whitespace-only, empty command, multi-arg commands.
- Error paths: malformed command with unmatched quote.

**Visual Similarity Tests**:
- 80x24: parsed command displayed correctly in input submit flow.
- 120x40: command result appears inline and styled.
- 200x50: command + args preserve spacing in output.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-041-80x24.txt

### TUI-042: Autocomplete dropdown component

**Phase**: Phase 4 — Commands and Navigation
**Labels**: tui, phase-4, component:autocomplete
**Dependencies**: TUI-041

**Description**:
Create a dropdown that appears on `/` and supports matching, ranking, and keyboard selection.
Show command names and descriptions with highlight and reverse-video selected row.
Close on Escape and Enter selection.

**Acceptance Criteria**:
- [ ] Dropdown opens when slash command prefix detected.
- [ ] Filtering updates in O(1) for each keystroke.
- [ ] Enter applies highlighted suggestion.
- [ ] Up/Down moves selection with wrap behavior.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/autocomplete/model.go` — overlay state.
- `cmd/harnesscli/tui/components/autocomplete/view.go` — dropdown view.

**TDD Requirements** (write these tests FIRST):
- `TestTUI042_AutocompleteOpensOnSlashPrefix` — overlay open test.
- `TestTUI042_EnterSelectsSuggestion` — selection behavior.

**Regression Test Requirements**:
- Concurrent access: overlay updates while background SSE messages arrive.
- Boundary conditions: zero results and huge result sets.
- Error paths: malformed suggestion data should be ignored.

**Visual Similarity Tests**:
- 80x24: dropdown appears immediately below input area and does not exceed width.
- 120x40: up to 8 rows visible with truncation.
- 200x50: selected row has inverse video and description.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-042-80x24.txt

### TUI-043: Help dialog with 3 tabs

**Phase**: Phase 4 — Commands and Navigation
**Labels**: tui, phase-4, component:helpdialog
**Dependencies**: TUI-008, TUI-042

**Description**:
Implement `/help` modal with Commands, Keybindings, and About tabs.
Support tab switching and static/dynamic content updates from key map and command registry.
Dialog should support close/exit and not block status updates.

**Acceptance Criteria**:
- [ ] Three tabs present and selectable.
- [ ] Commands tab lists slash commands with descriptions.
- [ ] Keybindings tab lists shortcuts from keyMap.
- [ ] About tab includes version and runtime metadata.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/helpdialog/model.go` — tab model.
- `cmd/harnesscli/tui/components/helpdialog/view.go` — 3-tab rendering.

**TDD Requirements** (write these tests FIRST):
- `TestTUI043_HelpdialogHasThreeTabs` — presence test.
- `TestTUI043_HelpdialogTabSwitching` — left/right switching semantics.

**Regression Test Requirements**:
- Concurrent access: open help while tool events still arriving.
- Boundary conditions: empty content lists.
- Error paths: undefined tab index resets safely.

**Visual Similarity Tests**:
- 80x24: help dialog uses border style and readable single-column list.
- 120x40: tab headers and content split with clear separators.
- 200x50: full dialog with more command rows.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-043-80x24.txt

### TUI-044: /context grid view

**Phase**: Phase 4 — Commands and Navigation
**Labels**: tui, phase-4, component:contextgrid
**Dependencies**: TUI-043, TUI-041

**Description**: Implement `/context` command output that generates 10x10 context usage grid with category legend.
Use icon set and colors from research with readable fallback.
Supports concise category counts and right-side breakdown text.

**Acceptance Criteria**:
- [ ] Generates deterministic 10x10 grid for sample input data.
- [ ] Handles low and high density cells with icon transitions.
- [ ] Includes category legend and metadata line.
- [ ] Works as inline command output, not separate overlay.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/contextgrid/model.go` — data computation.
- `cmd/harnesscli/tui/components/contextgrid/view.go` — inline render.

**TDD Requirements** (write these tests FIRST):
- `TestTUI044_ContextGridGenerates10x10Matrix` — exact dimensions.
- `TestTUI044_ContextGridIncludesLegend` — legend content.

**Regression Test Requirements**:
- Concurrent access: data updates while command output rendering.
- Boundary conditions: zero usage and max usage values.
- Error paths: corrupt data points should be sanitized.

**Visual Similarity Tests**:
- 80x24: compact 10x10 matrix with legend and minimal clipping.
- 120x40: category breakdown visible on right.
- 200x50: larger labels and spacing remain stable.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-044-80x24.txt

### TUI-045: /stats heatmap

**Phase**: Phase 4 — Commands and Navigation
**Labels**: tui, phase-4, component:statsheatmap
**Dependencies**: TUI-044

**Description**: Add `/stats` command with heatmap rendering and model metadata.
Render `░▒▓█` intensity bars and activity labels with day/month markers.
Include tabs for “Overview” and “Models” where practical.

**Acceptance Criteria**:
- [ ] Heatmap row and legend render correctly.
- [ ] Supports at least two display periods via `r` key toggle.
- [ ] Handles sparse and dense data.
- [ ] Clipboard copy hook available.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/statsheatmap/model.go` — data transforms.
- `cmd/harnesscli/tui/components/statsheatmap/view.go` — heatmap layout.

**TDD Requirements** (write these tests FIRST):
- `TestTUI045_StatsHeatmapRendersLegendAndBars` — baseline view.
- `TestTUI045_StatsHeatmapPeriodToggle` — command key toggles period.

**Regression Test Requirements**:
- Concurrent access: concurrent updates for multiple model series.
- Boundary conditions: empty history and all-on history.
- Error paths: malformed timestamps in input series.

**Visual Similarity Tests**:
- 80x24: overview chart fits one screen without clipping.
- 120x40: two tabs shown with period toggle hint.
- 200x50: model series lines and heatmap coexist clearly.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-045-80x24.txt

### TUI-046: Command history navigation

**Phase**: Phase 4 — Commands and Navigation
**Labels**: tui, phase-4, component:input
**Dependencies**: TUI-012

**Description**: Add input history with up/down navigation and per-session persistent list.
Support replaying commands from history and preserving cursor at end.
History should include both slash commands and normal prompts.

**Acceptance Criteria**:
- [ ] Up/down cycle through persisted commands.
- [ ] Navigating history does not overwrite unsaved draft unless accepted.
- [ ] History capped at configurable size.
- [ ] Supports clear and empty-history edge gracefully.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/input/history.go` — history store.
- `cmd/harnesscli/tui/components/input/model.go` — up/down handling.

**TDD Requirements** (write these tests FIRST):
- `TestTUI046_HistoryNavigatesBackwardAndForward` — baseline navigation.
- `TestTUI046_DraftPreservedAcrossNavigation` — unsent text restored.

**Regression Test Requirements**:
- Concurrent access: input with background event updates.
- Boundary conditions: empty and max-length histories.
- Error paths: history file missing or corrupted fallback behavior.

**Visual Similarity Tests**:
- 80x24: history retrieval visible in input prompt.
- 120x40: no flicker when cycling.
- 200x50: draft restoration remains legible.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-046-80x24.txt

### TUI-047: Fuzzy filter for autocomplete

**Phase**: Phase 4 — Commands and Navigation
**Labels**: tui, phase-4, component:autocomplete
**Dependencies**: TUI-042

**Description**: Enhance autocomplete ranking with fuzzy matching over command names and descriptions.
Use stable scoring to avoid jumpy order across fast typing.
Return at least five top matches and avoid expensive rescans.

**Acceptance Criteria**:
- [ ] Fuzzy matching handles abbreviations and typos.
- [ ] Ranking deterministic for identical scores.
- [ ] Re-renders at input rate without stutter.
- [ ] Works with command prefix and argument context.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/autocomplete/matcher.go` — scoring algorithm.
- `cmd/harnesscli/tui/components/autocomplete/model.go` — apply ranked results.

**TDD Requirements** (write these tests FIRST):
- `TestTUI047_FuzzyMatchRanksCloserTermsHigher` — ranking behavior.
- `TestTUI047_AutocompleteStableOnTyping` — no order jitter.

**Regression Test Requirements**:
- Concurrent access: concurrent query changes while list updates.
- Boundary conditions: no query and huge query.
- Error paths: missing candidate descriptions should still rank by name.

**Visual Similarity Tests**:
- 80x24: top suggestions reorder as query narrows.
- 120x40: top five displayed with short descriptions.
- 200x50: full sorted list with active row in reverse video.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-047-80x24.txt

### TUI-048: Tab completion in input

**Phase**: Phase 4 — Commands and Navigation
**Labels**: tui, phase-4, component:input
**Dependencies**: TUI-042

**Description**: Add Tab completion for command names, overlay suggestions, and filesystem-like suggestions when `@` context enabled.
On single match, auto-complete and append trailing space where appropriate.
On multiple matches, keep overlay open and select first.

**Acceptance Criteria**:
- [ ] Tab completion picks best completion for deterministic suggestions.
- [ ] Multiple matches preserve autocomplete dropdown visibility.
- [ ] Completion does not break cursor for multiline text.
- [ ] Completion works when command includes arguments.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/input/model.go` — tab-key handling.
- `cmd/harnesscli/tui/components/autocomplete/matcher.go` — single-match fast path.

**TDD Requirements** (write these tests FIRST):
- `TestTUI048_TabCompletesSingleCommand` — direct completion.
- `TestTUI048_TabKeepsDropdownForMultiMatch` — overlay remains.

**Regression Test Requirements**:
- Concurrent access: tab completion while command parser mutates context.
- Boundary conditions: tab on empty input and at argument boundary.
- Error paths: completion call with no providers returns no-op safely.

**Visual Similarity Tests**:
- 80x24: completion writes expected text in prompt.
- 120x40: dropdown remains after partial completion.
- 200x50: argument completion with spaces visible.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-048-80x24.txt

### TUI-049: Escape key handling

**Phase**: Phase 4 — Commands and Navigation
**Labels**: tui, phase-4, component:input
**Dependencies**: TUI-010, TUI-033, TUI-039

**Description**: Implement multi-purpose Escape semantics: close overlays first, then clear current input on second press when text is present.
When running, Escape should interrupt stream and enter input-ready state.
Add explicit status hint for two-step clear behavior.

**Acceptance Criteria**:
- [ ] First Escape closes overlay or clears currently waiting state.
- [ ] Second Escape clears input text with prior hint.
- [ ] Escape during active stream sends interrupt.
- [ ] Overlay focus restored after dismissal.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/keys.go` — escape key mapping and docs.
- `cmd/harnesscli/tui/model.go` — escape behavior state machine.

**TDD Requirements** (write these tests FIRST):
- `TestTUI049_EscapeClosesOverlayFirst` — overlay priority.
- `TestTUI049_EscapeDoubleClearBehavior` — two-step clear sequence.

**Regression Test Requirements**:
- Concurrent access: overlay close while model update continues.
- Boundary conditions: empty input and overlay open.
- Error paths: escape on unsupported mode no-op safe.

**Visual Similarity Tests**:
- 80x24: status hint appears after first escape on non-empty input.
- 120x40: overlay closes cleanly with no visual residue.
- 200x50: interrupt and clear transitions maintain prompt.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-049-80x24.txt

### TUI-050: Command result display in viewport

**Phase**: Phase 4 — Commands and Navigation
**Labels**: tui, phase-4, component:viewport
**Dependencies**: TUI-041, TUI-043

**Description**: Ensure all slash command responses render via standardized inline response style in viewport.
Support command-level outputs for clear/config/context/stats and unknown feedback.
Standardize with `⎿` connectors where appropriate.

**Acceptance Criteria**:
- [ ] Command results appear immediately after command submission.
- [ ] Result type maps to status or list formats.
- [ ] Unknown commands show descriptive hint.
- [ ] Results participate in auto-scroll and history virtualization.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/cmd_result.go` — response object formatting.
- `cmd/harnesscli/tui/components/viewport/model.go` — command-result insertion.

**TDD Requirements** (write these tests FIRST):
- `TestTUI050_CommandResultAppearsAfterSubmit` — command output inserted.
- `TestTUI050_UnknownCommandResultHint` — invalid command result formatting.

**Regression Test Requirements**:
- Concurrent access: multiple command outputs near-simultaneously.
- Boundary conditions: command output empty and very long.
- Error paths: command handler panic recovers with error result.

**Visual Similarity Tests**:
- 80x24: command result appears with connector below input area.
- 120x40: clear and context outputs keep expected structure.
- 200x50: multiple command rows visible with stable spacing.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-050-80x24.txt

### TUI-051: /config panel form component

**Phase**: Phase 5 — Advanced UX
**Labels**: tui, phase-5, component:configpanel
**Dependencies**: TUI-043, TUI-041

**Description**: Add interactive config panel within `/config` flow with searchable list and inline editing affordance.
Panel should support key-value rows, type hints, and save/revert semantics.
This is nested in status dialog context but can be reused from command output.

**Acceptance Criteria**:
- [ ] Renders config keys and values in scrollable form.
- [ ] Supports search/filter and selection.
- [ ] Edits are validated before apply.
- [ ] Changes are persisted if supported by backend endpoint.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/configpanel/model.go` — config entries and selection.
- `cmd/harnesscli/tui/components/configpanel/view.go` — list and edit rendering.

**TDD Requirements** (write these tests FIRST):
- `TestTUI051_ConfigPanelRendersSettingsRows` — rows and values.
- `TestTUI051_ConfigPanelSearchFiltersRows` — search behavior.

**Regression Test Requirements**:
- Concurrent access: edits while live tool output appends elsewhere.
- Boundary conditions: unknown config key and read-only field.
- Error paths: save request failure sets status with retry hint.

**Visual Similarity Tests**:
- 80x24: compact form with minimal visible rows.
- 120x40: searchable input + rows visible.
- 200x50: full panel with metadata columns.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-051-80x24.txt

### TUI-052: /permissions panel component

**Phase**: Phase 5 — Advanced UX
**Labels**: tui, phase-5, component:permissionspanel
**Dependencies**: TUI-033, TUI-051

**Description**: Build permissions panel with tabbed allow/ask/deny/workspace semantics.
Provide row-based editing, search, and inline “add new rule” action.
Panel should integrate with permission evaluator and live preview.

**Acceptance Criteria**:
- [ ] Panel has 4 logical tabs and selectable rows.
- [ ] New rule insertion stub exists and validated.
- [ ] Search filtering updates rows in real time.
- [ ] Panel state changes persist until exit.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/permissionspanel/model.go` — rule model.
- `cmd/harnesscli/tui/components/permissionspanel/view.go` — tab/list rendering.

**TDD Requirements** (write these tests FIRST):
- `TestTUI052_PermissionsTabsRender` — tab presence and labels.
- `TestTUI052_AddRuleFlow` — add workflow stub.

**Regression Test Requirements**:
- Concurrent access: update rule while prompt overlay open.
- Boundary conditions: no rules and huge rule list.
- Error paths: malformed rule expression rejected gracefully.

**Visual Similarity Tests**:
- 80x24: panel with first rows and add rule line.
- 120x40: tab headers and status icons visible.
- 200x50: richer rule metadata and search box.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-052-80x24.txt

### TUI-053: Session picker list

**Phase**: Phase 5 — Advanced UX
**Labels**: tui, phase-5, component:sessionpicker
**Dependencies**: TUI-050, TUI-046

**Description**: Implement session picker with resumable sessions, branch info, branch toggles, and short action hints.
Used for `/resume` flow and future branching workflows.
Must support search and selection shortcuts.

**Acceptance Criteria**:
- [ ] Session list shows id, size, branch, and recency.
- [ ] Search field filters sessions incrementally.
- [ ] Selection returns command to restore/resume state.
- [ ] Supports empty set with instructional row.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/sessionpicker/model.go` — session list logic.
- `cmd/harnesscli/tui/components/sessionpicker/view.go` — list rendering.

**TDD Requirements** (write these tests FIRST):
- `TestTUI053_SessionpickerShowsMetadata` — row content.
- `TestTUI053_SessionpickerSearchFilters` — filter semantics.

**Regression Test Requirements**:
- Concurrent access: sessions refreshed while user filters.
- Boundary conditions: no sessions and one session.
- Error paths: API failure to fetch sessions surfaces fallback text.

**Visual Similarity Tests**:
- 80x24: first sessions list with search cue.
- 120x40: branch and timestamp columns visible.
- 200x50: sorted list with highlighted selection.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-053-80x24.txt

### TUI-054: Interrupt UX with graceful stop

**Phase**: Phase 5 — Advanced UX
**Labels**: tui, phase-5, component:statusbar
**Dependencies**: TUI-039, TUI-049

**Description**: Expand interruption flow to include confirmation when running and no active stream.
Support soft-stop semantics with graceful fallback and quick re-entry into input mode.
Display confirmation text and exit path for running background tasks.

**Acceptance Criteria**:
- [ ] Interrupt can stop active run or show no-op if idle.
- [ ] Confirmation path exists for destructive stop.
- [ ] Input remains usable immediately after cancel.
- [ ] Status bar and history indicate stop reason.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/model.go` — interrupt state handling.
- `cmd/harnesscli/tui/bridge.go` — cancel signal integration.

**TDD Requirements** (write these tests FIRST):
- `TestTUI054_GracefulStopSetsStoppedState` — stop event state.
- `TestTUI054_InputUnblockedAfterStop` — input re-enabled.

**Regression Test Requirements**:
- Concurrent access: stop while multiple tool calls pending.
- Boundary conditions: stop on idle state no-op.
- Error paths: stop endpoint failure should display error and keep state stable.

**Visual Similarity Tests**:
- 80x24: stop status appears in fixed status line.
- 120x40: stop result connector appears below user turn.
- 200x50: no ghost spinner after stop.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-054-80x24.txt

### TUI-055: Plan mode overlay

**Phase**: Phase 5 — Advanced UX
**Labels**: tui, phase-5, component:planmode
**Dependencies**: TUI-031, TUI-049

**Description**: Implement `/plan` and Shift+Tab toggling behavior for plan mode.
Plan mode should surface intentful text and prevent automatic execution where applicable.
Show overlay indicators and compact instructions.

**Acceptance Criteria**:
- [ ] Toggle changes root mode and overlay indicator.
- [ ] Status line updates to `⏸ plan mode on` with cycle hint.
- [ ] All plan-mode controls remain localized and non-blocking.
- [ ] Exits cleanly to normal mode.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/planmode/model.go` — plan state and transitions.
- `cmd/harnesscli/tui/components/planmode/view.go` — visual indication.

**TDD Requirements** (write these tests FIRST):
- `TestTUI055_PlanModeTogglePersists` — toggling on/off behavior.
- `TestTUI055_PlanModeAffectsCommandRouting` — command-mode gating.

**Regression Test Requirements**:
- Concurrent access: mode toggles while run in progress.
- Boundary conditions: repeated rapid toggles.
- Error paths: unknown mode fallback to default.

**Visual Similarity Tests**:
- 80x24: plan mode indicator appears in status line.
- 120x40: plan mode overlay card with hint visible.
- 200x50: long sessions include mode footer line.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-055-80x24.txt

### TUI-056: Running cost display

**Phase**: Phase 5 — Advanced UX
**Labels**: tui, phase-5, component:statusbar
**Dependencies**: TUI-015, TUI-020

**Description**: Show runtime cost estimate from usage events in status line, with token and USD display.
Display should be low-profile and non-blocking and update in place with deltas.
Should handle missing values gracefully.

**Acceptance Criteria**:
- [ ] Token count and USD estimate displayed when data available.
- [ ] Handles unknown/missing usage values.
- [ ] Cost line truncates gracefully on narrow terminals.
- [ ] Usage updates on assistant/tool events.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/components/statusbar/status.go` — usage accumulation.
- `cmd/harnesscli/tui/model.go` — usage event fan-in.

**TDD Requirements** (write these tests FIRST):
- `TestTUI056_CostDisplayAppearsWhenUsagePresent` — usage formatting.
- `TestTUI056_MissingUsageFallbackSafe` — no panic on nil usage payload.

**Regression Test Requirements**:
- Concurrent access: usage updates from overlapping runs.
- Boundary conditions: zero usage and huge totals.
- Error paths: malformed usage payload.

**Visual Similarity Tests**:
- 80x24: compact usage segment appended to status line.
- 120x40: full `token · USD` text visible.
- 200x50: larger numbers no clipping.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-056-80x24.txt

### TUI-057: Model switcher component

**Phase**: Phase 5 — Advanced UX
**Labels**: tui, phase-5, component:statusbar
**Dependencies**: TUI-041, TUI-055

**Description**: Add quick and dialog model switcher with current model highlight and effort slider placeholder.
Support Meta+P fast shortcut path from key map.
Switching model should reflect immediately in status and current run header.

**Acceptance Criteria**:
- [ ] Model picker opens and lists available options.
- [ ] Current model displayed with `✔`.
- [ ] Active model updates on confirm.
- [ ] Meta+P opens picker from input state.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/run_flow.go` — selected model context.
- `cmd/harnesscli/tui/components/helpdialog/model.go` or dedicated model overlay for picker selection.

**TDD Requirements** (write these tests FIRST):
- `TestTUI057_ModelSwitcherOpens` — picker route and keys.
- `TestTUI057_ModelSelectionPersists` — selected model appears in status.

**Regression Test Requirements**:
- Concurrent access: model switch while active stream in progress.
- Boundary conditions: no models available fallback to current model.
- Error paths: unknown model selection rejected with hint.

**Visual Similarity Tests**:
- 80x24: picker in compact list mode with current model marker.
- 120x40: effort slider placeholder visible if included.
- 200x50: full model table with branch and selection states.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-057-80x24.txt

### TUI-058: Compact/verbose output toggle

**Phase**: Phase 5 — Advanced UX
**Labels**: tui, phase-5, component:overlay
**Dependencies**: TUI-031, TUI-043

**Description**: Add compact mode toggle for conversation density and tool output verbosity.
Compact mode hides some details and summary lines while verbose keeps full tool expansions and diffs.
Shortcut and command path should both work.

**Acceptance Criteria**:
- [ ] Toggle changes renderers globally.
- [ ] In compact mode, collapsed tool lines and shorter snippets are used.
- [ ] In verbose mode, expanded details appear by default.
- [ ] State persists for session if desired.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/config.go` — compact mode flag.
- `cmd/harnesscli/tui/model.go` — apply mode in message render selection.

**TDD Requirements** (write these tests FIRST):
- `TestTUI058_CompactModeHidesToolDetails` — compact effect.
- `TestTUI058_VerboseModeShowsFullDetails` — verbose effect.

**Regression Test Requirements**:
- Concurrent access: switching mode while stream running.
- Boundary conditions: toggling repeatedly while no messages.
- Error paths: unknown mode values fallback to default.

**Visual Similarity Tests**:
- 80x24: compact mode keeps layout density and spacing.
- 120x40: verbose mode shows additional lines and metadata.
- 200x50: mode transition leaves status line updated.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-058-80x24.txt

### TUI-059: Export transcript to file

**Phase**: Phase 5 — Advanced UX
**Labels**: tui, phase-5, component:cmd
**Dependencies**: TUI-030, TUI-050

**Description**: Implement `/export` flow to write visible transcript (or full history) to a file.
Support timestamped default filename, overwrite safety, and success/failure notifications.
Transcript should include formatted content but exclude transient status noise.

**Acceptance Criteria**:
- [ ] Command writes transcript and reports output path.
- [ ] Export supports user-specified path argument.
- [ ] On failure, error is shown inline.
- [ ] Success does not alter active viewport state.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/cmd_result.go` — export command handler.
- `cmd/harnesscli/tui/components/statusbar/view.go` — export feedback in status.

**TDD Requirements** (write these tests FIRST):
- `TestTUI059_ExportWritesTranscript` — writes file with expected marker lines.
- `TestTUI059_ExportRejectsInvalidPath` — invalid path error handling.

**Regression Test Requirements**:
- Concurrent access: export while streaming new messages.
- Boundary conditions: empty transcript and read-only path.
- Error paths: filesystem permission denied.

**Visual Similarity Tests**:
- 80x24: inline result line with saved path or error.
- 120x40: file path fully shown when short.
- 200x50: long path line wraps with connector line.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-059-80x24.txt

### TUI-060: Final polish — animation smoothing and accessibility

**Phase**: Phase 5 — Advanced UX
**Labels**: tui, phase-5, component:root
**Dependencies**: TUI-001, TUI-002, TUI-003, TUI-004, TUI-005, TUI-006, TUI-007, TUI-008, TUI-009, TUI-010, TUI-011, TUI-012, TUI-013, TUI-014, TUI-015, TUI-016, TUI-017, TUI-018, TUI-019, TUI-020, TUI-021, TUI-022, TUI-023, TUI-024, TUI-025, TUI-026, TUI-027, TUI-028, TUI-029, TUI-030, TUI-031, TUI-032, TUI-033, TUI-034, TUI-035, TUI-036, TUI-037, TUI-038, TUI-039, TUI-040, TUI-041, TUI-042, TUI-043, TUI-044, TUI-045, TUI-046, TUI-047, TUI-048, TUI-049, TUI-050, TUI-051, TUI-052, TUI-053, TUI-054, TUI-055, TUI-056, TUI-057, TUI-058, TUI-059

**Description**: Consolidate focus management, minor motion smoothing, and accessibility.
Reduce unnecessary re-renders, add focus-ring semantics, improve screen-reader metadata, and ensure keyboard-only usability.
This is the hardening ticket and should close visual, timing, and behavioral polish.

**Acceptance Criteria**:
- [ ] Focus transitions are deterministic and never jump across unrelated components.
- [ ] Animation frequency avoids CPU spikes on long sessions.
- [ ] Overlay focus and input focus are mutually exclusive and testable.
- [ ] Basic accessibility labels/aria-like hints are visible in status/help.

**Files to Create/Modify**:
- `cmd/harnesscli/tui/model.go` — focus arbitration and motion throttle.
- `cmd/harnesscli/tui/components/layout/container.go` — render ordering optimization.

**TDD Requirements** (write these tests FIRST):
- `TestTUI060_NoExcessiveReRendersInSteadyState` — ensure event loop stabilizes.
- `TestTUI060_FocusManagementRespectsOverlayPriority` — focus arbitration.

**Regression Test Requirements**:
- Concurrent access: heavy synthetic load (200 events/sec) with no frame drops.
- Boundary conditions: tiny terminal and very large history.
- Error paths: malformed overlay close events should not leave focus stuck.

**Visual Similarity Tests**:
- 80x24: all major interactions produce no visual tearing.
- 120x40: smooth overlay transitions and stable cursor.
- 200x50: full interaction path remains responsive and aligned.
- Command: tmux new-session -d -s tui-test -x 80 -y 24 && tmux capture-pane -t tui-test -p > testdata/snapshots/TUI-060-80x24.txt

## Section 4: Implementation Sequence (Wave-based dependency graph)

Wave 0 (no deps): TUI-001, TUI-002

Wave 1 (after Wave 0): TUI-003, TUI-004, TUI-005, TUI-006, TUI-007, TUI-008, TUI-009, TUI-010

Wave 2 (after Wave 1): TUI-011, TUI-012, TUI-013, TUI-014, TUI-015, TUI-016, TUI-017, TUI-018, TUI-019

Wave 3 (after Wave 2): TUI-020

Wave 4 (after Wave 3): TUI-021, TUI-022, TUI-023, TUI-024, TUI-025, TUI-026, TUI-027, TUI-028, TUI-029, TUI-030

Wave 5 (after Wave 4): TUI-031, TUI-032, TUI-033, TUI-034, TUI-035, TUI-036, TUI-037, TUI-038, TUI-039, TUI-040

Wave 6 (after Wave 5): TUI-041, TUI-042, TUI-043, TUI-044, TUI-045, TUI-046, TUI-047, TUI-048, TUI-049, TUI-050

Wave 7 (after Wave 6): TUI-051, TUI-052, TUI-053, TUI-054, TUI-055, TUI-056, TUI-057, TUI-058, TUI-059, TUI-060

Dependencies within waves are parallelizable, and each wave unlocks the next by introducing new stable primitives:
- Waves 0-1 create foundation and bridgeability.
- Wave 2 enables layout and composition.
- Wave 3 validates baseline behavior end-to-end.
- Wave 4 builds chat ergonomics.
- Wave 5 adds advanced tool usability.
- Wave 6 adds navigation and command UX.
- Wave 7 completes advanced panels and polish.

## Section 5: Testing Infrastructure

### 5A. testhelpers package design

Location: `cmd/harnesscli/testhelpers`.

Planned helper functions (non-exhaustive, all production-safe and reusable across tests):
- `func StartMockSSEServer(t *testing.T, handler http.HandlerFunc) *httptest.Server` — starts and returns an `httptest.Server` with deterministic SSE framing helpers.
- `func SSEFrame(t string, payload any) string` — returns a single SSE text block.
- `func SendSSEChunks(t *testing.T, endpoint string, chunks []string, pause time.Duration)` — writes SSE chunks with controlled pacing.
- `func MustReadFixture(t *testing.T, path string) string` — loads expected golden snapshots from `testdata/snapshots`.
- `func RequireViewportContains(t *testing.T, got, want string)` — assertion helper for rendered strings.
- `func CaptureSnapshot(t *testing.T, name string, width, height int) string` — captures fixed-size output for tests.
- `func BuildToolEvent(id, typ string, payload any) harness.Event` — creates typed event fixtures.
- `func BuildMessage(id, content string, ts time.Time) Message` — creates deterministic message fixtures.
- `func WriteGolden(t *testing.T, path string, got string)` — optional golden file update helper.
- `func ParseTokens(t *testing.T, src string) []string` — token approximation helper for duration metrics and truncation thresholds.

### 5B. Mock SSE server design for integration tests

Use `httptest.NewServer` with a handler that writes `text/event-stream; charset=utf-8` and flushes each frame.
The mock server must run a goroutine for long-running events and expose channels for frame control.
Implementation pattern:
1. Create per-test server with a deterministic event script (`[]string`) and optional delays.
2. In handler, assert method and path, set SSE headers, and use `Flusher` if available.
3. Write ordered events with `

`; insert controlled sleeps between frames for concurrency testing.
4. Close channels and cancel client context on test completion to ensure bridge shutdown.
5. Add route variations for `/v1/runs/{id}/events` and `/api/v1/runs/{id}/events` with fallback in bridge adapter.

### 5C. Golden file pattern

Directory layout:
- `testdata/snapshots/` (root)
- `testdata/snapshots/layout/` for layout-specific captures
- `testdata/snapshots/components/` for component-level snapshots
- `testdata/snapshots/integration/` for full TUI flows

File naming pattern:
- `TUI-XXX-<size>.txt` where `<size>` is `80x24`, `120x40`, or `200x50`.
- Example: `testdata/snapshots/TUI-020-120x40.txt` and `testdata/snapshots/TUI-014-200x50.txt`.

Update command:
```bash
mkdir -p testdata/snapshots
cp testdata/snapshots/TUI-XXX-80x24.txt.test testdata/snapshots/TUI-XXX-80x24.txt
```
(Or a dedicated helper that writes only when an `UPDATE_TESTDATA=1` environment flag is set.)

### 5D. Visual regression workflow

1. Record baselines using deterministic tests with fixed terminal sizes and seeded timers.
2. Store snapshots at the three canonical sizes and commit with ticket IDs.
3. On PR, run baseline compare on changed component and integration tests.
4. On mismatches, run `git diff` on snapshot files and only accept diffs with design intent or test fixture updates.
5. Use textual diff tools for line-level inspection and, if needed, side-by-side capture from tmux sessions for human review.
6. Keep the process tool-agnostic but deterministic:
   - `go test ./cmd/harnesscli/... -run TestTUI...`
   - `git diff -- testdata/snapshots/*.txt`
   - If drift is expected, update snapshots in the same PR with changelist references.

Suggested tooling:
- Core tests: `go test ./cmd/harnesscli/...`
- Golden visual capture: `tmux new-session -d -s tui-test -x <w> -y <h> && ...` from each component test runner.
- Diff verification: standard git diff plus optional `colordiff` for terminal review.
