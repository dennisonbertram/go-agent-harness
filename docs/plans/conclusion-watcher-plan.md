# Plan: Conclusion Watcher Plugin

## Context

- **Problem**: LLM agents in the harness frequently produce "conclusion jumps" — asserting
  facts, claiming completion, or proposing mutations without first doing the diagnostic
  work that would justify those claims. This pattern produces incorrect patches, false
  done signals, and architecture assumptions built on unread code.
- **User impact**: Agents claim tasks are done when they are not, write to files they
  have never read, and describe architecture they have not explored.
- **Constraints**:
  - Zero changes to `internal/harness/`. The watcher is a self-contained plugin.
  - No new `EventType` constants in `events.go`, no `AllEventTypes()` changes.
  - No new fields added to `RunnerConfig`.
  - Plugin wires itself in by appending to the existing `PreToolUseHooks` and
    `PostMessageHooks` slices in `RunnerConfig`. These slices already exist and accept
    any value implementing the respective interface.

---

## Scope

- **In scope**:
  - Package `plugins/conclusion-watcher` (root-sibling to `internal/`, `cmd/`)
  - 5 detection patterns implemented as pure functions
  - 3 intervention modes
  - Thread-safe `ObservationLedger`
  - Plugin event emission via caller-supplied callback (no harness event bus coupling)
  - Full TDD — failing tests written first for every component
- **Out of scope**:
  - Persistence of detections across runs (single-run, in-memory only)
  - UI integration or SSE streaming of plugin events
  - Any changes to `internal/harness/`

---

## Module / Import Path

The `go-agent-harness` repository uses the single root module `go-agent-harness` (see
`go.mod`). The plugin lives at `plugins/conclusion-watcher/` inside that module, so its
import path is:

```
go-agent-harness/plugins/conclusion-watcher
```

**No separate `go.mod` is needed.** The plugin is a regular package in the root module.
This is identical to how `internal/forensics/costanomaly` is structured — a subdirectory
package that imports `go-agent-harness/internal/harness` directly.

If the plugin is later extracted to a standalone repo, a `go.mod` would be added then,
but for now staying in the root module avoids import cycle risk and keeps the test runner
unified.

Import cycle check: `plugins/conclusion-watcher` imports
`go-agent-harness/internal/harness`. Nothing in `internal/harness` imports
`plugins/conclusion-watcher`. No cycle.

---

## Package Structure

```
plugins/
└── conclusion-watcher/
    ├── plugin.go          — ConclusionWatcher, New(), Register()
    ├── types.go           — PatternType, DetectionResult, WatcherConfig,
    │                        InterventionMode, CritiqueProvider, event constants
    ├── ledger.go          — ObservationLedger (thread-safe)
    ├── detectors.go       — 5 pure detector functions
    ├── interventions.go   — InjectValidationPrompt, PauseForUser, RequestCritique
    └── plugin_test.go     — unit tests for all components
```

---

## Full Type and Method Signatures

### `types.go`

```go
package conclusionwatcher

// PatternType identifies which conclusion pattern was detected.
type PatternType string

const (
    PatternHedgeAssertion       PatternType = "hedge_assertion"
    PatternUnverifiedFileClaim  PatternType = "unverified_file_claim"
    PatternPrematureCompletion  PatternType = "premature_completion"
    PatternSkippedDiagnostic    PatternType = "skipped_diagnostic"
    PatternArchitectureAssumption PatternType = "architecture_assumption"
)

// InterventionMode controls what the watcher does when a pattern fires.
type InterventionMode string

const (
    // InterventionInjectPrompt appends a validation request to the LLM
    // response text so that the next LLM turn receives the injected message.
    // This is the default mode.
    InterventionInjectPrompt InterventionMode = "inject_prompt"
    // InterventionPauseForUser blocks the step and surfaces the detection
    // to the user via HookActionBlock.
    InterventionPauseForUser InterventionMode = "pause_for_user"
    // InterventionRequestCritique fires a secondary LLM call through
    // CritiqueProvider and injects the critique into the response.
    InterventionRequestCritique InterventionMode = "request_critique"
)

// DetectionResult describes a single fired pattern.
type DetectionResult struct {
    Pattern     PatternType `json:"pattern"`
    Confidence  float64     `json:"confidence"` // 0.0–1.0
    Evidence    string      `json:"evidence"`    // excerpt that triggered the match
    Step        int         `json:"step"`
    RunID       string      `json:"run_id"`
}

// CritiqueProvider is satisfied by anything that can produce a critique
// for a given piece of content.  The harness Provider interface is NOT
// used directly — callers wire in a thin adapter if needed.
type CritiqueProvider interface {
    Critique(ctx context.Context, content string) (string, error)
}

// WatcherConfig holds all watcher options.  Zero values produce safe defaults.
type WatcherConfig struct {
    // Patterns lists which PatternTypes to arm. Empty means all 5 armed.
    Patterns []PatternType

    // Mode selects the intervention strategy. Defaults to InterventionInjectPrompt.
    Mode InterventionMode

    // CritiqueProvider is required when Mode == InterventionRequestCritique.
    CritiqueProvider CritiqueProvider

    // EventEmitter, if non-nil, is called whenever a detection fires or an
    // intervention executes.  The plugin does NOT register its own event types
    // in the harness event bus; it emits through this callback instead.
    // eventType will be one of the package-level EventConclusionDetected /
    // EventConclusionIntervened constants.
    EventEmitter func(eventType string, runID string, payload map[string]any)

    // ValidationPrompt is the text appended in InterventionInjectPrompt mode.
    // Defaults to DefaultValidationPrompt when empty.
    ValidationPrompt string

    // MaxInterventionsPerRun caps total interventions to avoid runaway injection.
    // 0 means unlimited.
    MaxInterventionsPerRun int
}

// Plugin-scoped event type strings.  These are plain string constants,
// intentionally NOT of type harness.EventType, so the harness events.go
// and AllEventTypes() are never touched.
const (
    EventConclusionDetected   = "conclusion.detected"
    EventConclusionIntervened = "conclusion.intervened"
)

// DefaultValidationPrompt is injected when ValidationPrompt is empty and
// Mode == InterventionInjectPrompt.
const DefaultValidationPrompt = "\n\n[WATCHER] A conclusion-jump pattern was detected. " +
    "Please verify your claim with concrete evidence before proceeding. " +
    "Read the relevant file or run a diagnostic tool first."
```

### `ledger.go`

```go
package conclusionwatcher

import "sync"

// ObservationLedger tracks which file paths and tool names have been
// observed in the current run.  Thread-safe.
type ObservationLedger struct {
    mu            sync.RWMutex
    observedFiles map[string]struct{} // file paths seen via read/grep/glob/ls
    toolHistory   []toolEntry         // ordered list of tool calls this run
}

type toolEntry struct {
    step     int
    toolName string
}

// DiagnosticTools is the set of tool names considered "diagnostic"
// (non-mutating, exploratory).
var DiagnosticTools = map[string]bool{
    "read_file":    true,
    "grep":         true,
    "glob":         true,
    "bash":         true, // treated as diagnostic only when non-destructive
    "list_dir":     true,
    "git_log":      true,
    "git_diff":     true,
    "git_show":     true,
    "search":       true,
}

// ExplorationTools is the subset of DiagnosticTools that indicates
// explicit codebase exploration.
var ExplorationTools = map[string]bool{
    "read_file": true,
    "grep":      true,
    "glob":      true,
    "list_dir":  true,
    "git_log":   true,
    "git_diff":  true,
    "git_show":  true,
}

// MutatingTools is the set of tool names considered mutating / destructive.
var MutatingTools = map[string]bool{
    "write_file":  true,
    "edit_file":   true,
    "bash":        true, // bash can be either; context-dependent heuristic in detector
    "delete_file": true,
    "move_file":   true,
    "patch_file":  true,
}

// NewObservationLedger creates a new empty ledger.
func NewObservationLedger() *ObservationLedger

// RecordFileSeen records that a file path was accessed by a tool.
func (l *ObservationLedger) RecordFileSeen(path string)

// HasSeenFile reports whether path has been recorded.
func (l *ObservationLedger) HasSeenFile(path string) bool

// RecordTool appends a tool call to the ordered history.
func (l *ObservationLedger) RecordTool(step int, toolName string)

// RecentTools returns the tool names used in the last n steps (in order,
// most recent last).  n <= 0 returns the full history.
func (l *ObservationLedger) RecentTools(n int) []string

// LastStepHadDiagnostic reports whether any diagnostic tool was called
// during step or step-1 (the "current or previous step" window).
func (l *ObservationLedger) LastStepHadDiagnostic(currentStep int) bool

// LastStepHadExploration reports whether any exploration tool was called
// during step or step-1.
func (l *ObservationLedger) LastStepHadExploration(currentStep int) bool

// Reset clears the ledger.  Call between runs if a single watcher is
// reused across runs (not the typical pattern; New() is preferred per-run).
func (l *ObservationLedger) Reset()
```

### `detectors.go`

Each detector is a pure function taking the relevant inputs and returning
`*DetectionResult` (nil = no detection). No side effects; no lock held inside.
The caller (plugin.go) holds the ledger and passes it in.

```go
package conclusionwatcher

// DetectHedgeAssertion fires when the response content contains hedge-assertion
// language: "must be", "clearly", "obviously", "definitely", "I assume",
// "probably is", "should be", "it appears that".
// Returns non-nil when any phrase is found.
func DetectHedgeAssertion(runID string, step int, content string) *DetectionResult

// DetectUnverifiedFileClaim fires when the content mentions a file path AND
// uses an assertion keyword, but the ledger has no record of that file
// being read.
// File path heuristic: sequences matching `[\w./\-]+\.(go|py|ts|js|yaml|json|
// toml|md|sh|txt)` that appear in the content.
// Assertion keywords: same set as HedgeAssertion.
func DetectUnverifiedFileClaim(runID string, step int, content string, ledger *ObservationLedger) *DetectionResult

// DetectPrematureCompletion fires when the content contains completion language
// ("done", "fixed", "complete", "resolved", "implemented", "finished") but the
// recent tool history (last 3 steps) contains no test-or-verification tool
// ("bash" with test pattern, "run_tests", "go_test", "check", "verify").
func DetectPrematureCompletion(runID string, step int, content string, ledger *ObservationLedger) *DetectionResult

// DetectSkippedDiagnostic fires when the proposed tool call is a mutating tool
// (write_file, edit_file, or bash with a destructive command pattern) AND
// neither the current step nor the immediately preceding step included a
// diagnostic tool call.
// toolName is the tool about to be called.
// args is the raw JSON arguments (used to classify bash as destructive or not).
func DetectSkippedDiagnostic(runID string, step int, toolName string, args []byte, ledger *ObservationLedger) *DetectionResult

// DetectArchitectureAssumption fires when the content contains architecture
// assertion phrases ("the design is", "the flow is", "this is a bug",
// "the architecture requires", "the intended flow is") AND no exploration
// tool has been called in the recent history (last 3 steps).
func DetectArchitectureAssumption(runID string, step int, content string, ledger *ObservationLedger) *DetectionResult
```

### `interventions.go`

```go
package conclusionwatcher

import (
    "context"
    "go-agent-harness/internal/harness"
)

// InjectValidationPrompt appends ValidationPrompt to the LLM response content.
// Returns a mutated PostMessageHookResult with HookActionContinue and the
// modified response.
func InjectValidationPrompt(
    result harness.PostMessageHookResult,
    response *harness.CompletionResult,
    prompt string,
    detection DetectionResult,
) harness.PostMessageHookResult

// PauseForUser returns a PostMessageHookResult with HookActionBlock, using
// the detection evidence as the block reason.  The step is halted and the
// reason is surfaced to the runner (and ultimately to the user via SSE).
func PauseForUser(detection DetectionResult) harness.PostMessageHookResult

// RequestCritique calls the CritiqueProvider with the response content,
// then injects the critique using InjectValidationPrompt.
// Returns an error if the provider call fails; callers should fall back to
// InjectValidationPrompt on error.
func RequestCritique(
    ctx context.Context,
    result harness.PostMessageHookResult,
    response *harness.CompletionResult,
    detection DetectionResult,
    provider CritiqueProvider,
) (harness.PostMessageHookResult, error)
```

Note: `SkippedDiagnostic` and `PrematureCompletion` fire in the `PreToolUseHook`, not the
`PostMessageHook`, because they gate on the *next tool to be called*. See the Register
section below for which hook each pattern uses.

### `plugin.go`

```go
package conclusionwatcher

import (
    "context"
    "sync"
    "sync/atomic"

    "go-agent-harness/internal/harness"
)

// ConclusionWatcher is the root plugin object.  Create one per run (or share
// across runs by calling ledger.Reset() between them — but per-run is safer).
type ConclusionWatcher struct {
    cfg              WatcherConfig
    ledger           *ObservationLedger
    interventionCount int64 // atomic counter
    mu               sync.Mutex // guards detections slice
    detections       []DetectionResult
}

// New creates a ConclusionWatcher with the given config.
// WatcherConfig zero values are safe: all patterns armed, inject mode, no emitter.
func New(cfg WatcherConfig) *ConclusionWatcher

// Register wires the watcher into a RunnerConfig's hook slices.
// Call before creating the runner.  Register appends (never replaces) hooks,
// so it is safe to call alongside other plugins.
//
// Hooks appended:
//   - cfg.PostMessageHooks ← postMessageHook (runs HedgeAssertion,
//     UnverifiedFileClaim, PrematureCompletion, ArchitectureAssumption)
//   - cfg.PreToolUseHooks  ← preToolUseHook (runs SkippedDiagnostic;
//     also updates the ledger's tool history and file observations)
//   - cfg.PostToolUseHooks ← postToolUseHook (updates ledger file
//     observations from tool output)
func (w *ConclusionWatcher) Register(cfg *harness.RunnerConfig)

// Detections returns a copy of all DetectionResults recorded so far.
// Safe for concurrent use.
func (w *ConclusionWatcher) Detections() []DetectionResult

// InterventionCount returns the number of interventions executed.
func (w *ConclusionWatcher) InterventionCount() int64

// --- internal hook implementations (unexported) ---

// postMessageHook implements harness.PostMessageHook.
type postMessageHook struct{ w *ConclusionWatcher }

func (h *postMessageHook) Name() string
func (h *postMessageHook) AfterMessage(
    ctx context.Context,
    in harness.PostMessageHookInput,
) (harness.PostMessageHookResult, error)

// preToolUseHook implements harness.PreToolUseHook.
type preToolUseHook struct{ w *ConclusionWatcher }

func (h *preToolUseHook) Name() string
func (h *preToolUseHook) PreToolUse(
    ctx context.Context,
    ev harness.PreToolUseEvent,
) (*harness.PreToolUseResult, error)

// postToolUseHook implements harness.PostToolUseHook.
type postToolUseHook struct{ w *ConclusionWatcher }

func (h *postToolUseHook) Name() string
func (h *postToolUseHook) PostToolUse(
    ctx context.Context,
    ev harness.PostToolUseEvent,
) (*harness.PostToolUseResult, error)
```

---

## Hook Responsibilities Breakdown

### `postMessageHook.AfterMessage`

Fires after the LLM completes a turn.  Inspects `in.Response.Content`.

1. Run `DetectHedgeAssertion` on the content.
2. Run `DetectUnverifiedFileClaim` on the content + ledger.
3. Run `DetectPrematureCompletion` on the content + ledger.
4. Run `DetectArchitectureAssumption` on the content + ledger.
5. For each non-nil result: record to `w.detections`, emit `EventConclusionDetected`
   via the callback, then apply the configured intervention (respecting
   `MaxInterventionsPerRun`).
6. If multiple patterns fire in the same step, interventions are applied in
   pattern order; only the first intervention that produces `HookActionBlock`
   or a mutated response is used (subsequent detections are recorded but not
   double-intervened).
7. Returns `PostMessageHookResult` with the (possibly mutated) response.

### `preToolUseHook.PreToolUse`

Fires before each tool call.

1. Record the tool name in the ledger (`ledger.RecordTool(step, toolName)`).
2. If the tool is a read-type tool (in `ExplorationTools`), extract file paths
   from `ev.Args` and call `ledger.RecordFileSeen` for each.
3. Run `DetectSkippedDiagnostic` using `ev.ToolName`, `ev.Args`, and the ledger.
4. If non-nil result: record + emit `EventConclusionDetected`, apply intervention.
5. For `InterventionPauseForUser` mode: return `&PreToolUseResult{Decision: harness.ToolHookDeny, Reason: detection.Evidence}`.
6. For `InterventionInjectPrompt` mode in a pre-tool hook: cannot mutate the
   LLM response directly at this point, so instead return `ToolHookDeny` with
   the validation text as the denial reason (this causes the runner to return
   the reason as a tool result to the LLM, effectively injecting the prompt for
   the next turn).
7. Returns nil (allow) when no detection fires.

### `postToolUseHook.PostToolUse`

Fires after each tool call completes.

1. If the tool is in `ExplorationTools`, scan `ev.Result` for file paths
   mentioned in the output (lines starting with a path pattern) and call
   `ledger.RecordFileSeen` for each.
2. If tool is `read_file` or similar: extract the `path` from `ev.Args` and
   record it as seen (this is more reliable than parsing output).
3. No detection logic here — this hook is purely ledger maintenance.
4. Always returns `nil, nil` (no modification, no error).

---

## Step Numbering: The Off-By-One Trap

`PreToolUseEvent` and `PostToolUseEvent` do not carry a `Step` field.
`PostMessageHookInput.Step` is the step number of the LLM turn that just
completed.

The plugin extracts step context as follows:
- In `postMessageHook`: `in.Step` is the step number. Pass to detectors directly.
- In `preToolUseHook`: the step is not directly available. Use a per-run
  `currentStep int64` field on `ConclusionWatcher`, incremented atomically
  when `postMessageHook` fires (i.e., after each LLM turn completes). This
  gives a consistent step number for ledger entries and detection results.

Implementation: `ConclusionWatcher` holds `currentStep int64` (atomic). The
`postMessageHook` calls `atomic.AddInt64(&w.currentStep, 1)` at the start of
`AfterMessage`. The `preToolUseHook` reads `atomic.LoadInt64(&w.currentStep)`
to get the current step.

---

## Pattern Matching Details

### HedgeAssertion — phrase set

```go
var hedgePhrases = []string{
    "must be", "clearly", "obviously", "definitely",
    "I assume", "probably is", "should be", "it appears that",
}
```

Case-insensitive substring match. Confidence: 1.0 for exact match.

### UnverifiedFileClaim — file path regex

```go
var filePathRe = regexp.MustCompile(
    `(?i)\b[\w./\-]+\.(?:go|py|ts|js|jsx|tsx|yaml|yml|json|toml|md|sh|txt|env|cfg|conf)\b`,
)
```

Scan all matches. For each match, check `ledger.HasSeenFile(match)`. If the
path is not in the ledger AND the content also contains an assertion keyword
(same `hedgePhrases` list), fire with confidence 0.8 and the path as evidence.

**Trap**: `HasSeenFile` must normalise paths (strip leading `./`, resolve `..`)
before comparison, or the same file will fail the lookup when referred to with a
different prefix.

### PrematureCompletion — term set

```go
var completionTerms = []string{
    "done", "fixed", "complete", "completed",
    "resolved", "implemented", "finished", "all set",
}
var verificationTools = map[string]bool{
    "bash":       true, // only counts when args contain "test" or "go test" or similar
    "run_tests":  true,
    "go_test":    true,
    "check":      true,
    "verify":     true,
}
```

Case-insensitive. Look back 3 steps in ledger. Confidence: 0.9.

The bash tool counts as a verification tool only if the raw arguments contain
`test`, `go test`, `pytest`, `check`, `verify`, or `-run`. This requires the
ledger to store args alongside tool names (extend `toolEntry` to include
`args string`).

### SkippedDiagnostic — destructive bash heuristic

```go
var destructiveBashPatterns = []string{
    "rm ", "mv ", "cp ", "chmod ", "chown ",
    "sed -i", "awk ", "truncate",
    "tee ", "> /", ">> /",
}
```

For `bash` tool calls: scan `args` JSON for a `command` field. If the command
matches any destructive pattern and there was no diagnostic tool in the current
or previous step, fire with confidence 0.85.

For `write_file`, `edit_file`, `patch_file`, `delete_file`: always mutating.
Fire if no diagnostic in current or previous step. Confidence: 1.0.

### ArchitectureAssumption — phrase set

```go
var architecturePhrases = []string{
    "the design is", "the flow is", "this is a bug",
    "the architecture requires", "the intended flow is",
    "this is how it works", "the system does",
}
```

Case-insensitive. Look back 3 steps for any `ExplorationTool` call. If none
found, fire with confidence 0.75.

---

## TDD Task Sequence

Tasks must be worked in this order. Each task starts by writing a failing test
in `plugin_test.go`, then implementing the minimum code to pass.

### Task 1 — ObservationLedger

**Tests to write first:**
- `TestLedger_RecordAndHasSeenFile` — record a path, assert `HasSeenFile` true
- `TestLedger_UnseenFileReturnsFalse` — assert false for path never recorded
- `TestLedger_PathNormalization` — `./foo/bar.go` and `foo/bar.go` are the same
- `TestLedger_RecordTool_RecentTools` — record 5 tools, `RecentTools(3)` returns last 3
- `TestLedger_LastStepHadDiagnostic_True` — record diagnostic tool at step N, assert true for N
- `TestLedger_LastStepHadDiagnostic_False` — no diagnostic at step N, assert false
- `TestLedger_LastStepHadDiagnostic_PrevStep` — diagnostic at step N-1 counts for N
- `TestLedger_ConcurrencySafe` — 100 goroutines calling RecordTool/HasSeenFile concurrently with `-race`
- `TestLedger_Reset` — record items, Reset, assert empty

**Implement**: `ledger.go`

### Task 2 — DetectHedgeAssertion

**Tests to write first:**
- `TestDetectHedgeAssertion_MustBe` — content "the answer must be 42" fires
- `TestDetectHedgeAssertion_Obviously` — content "obviously this is correct" fires
- `TestDetectHedgeAssertion_NoMatch` — neutral content returns nil
- `TestDetectHedgeAssertion_CaseInsensitive` — "CLEARLY this is" fires
- `TestDetectHedgeAssertion_Fields` — result has correct RunID, Step, Pattern, non-empty Evidence

**Implement**: `DetectHedgeAssertion` in `detectors.go`

### Task 3 — DetectUnverifiedFileClaim

**Tests to write first:**
- `TestDetectUnverifiedFileClaim_UnseeFile` — content mentions "foo.go" with assertion keyword, file not in ledger → fires
- `TestDetectUnverifiedFileClaim_SeenFile` — same content but `foo.go` in ledger → nil
- `TestDetectUnverifiedFileClaim_NoAssertionKeyword` — path present but no assertion → nil
- `TestDetectUnverifiedFileClaim_PathNormalization` — `./internal/foo.go` seen, content mentions `internal/foo.go` → nil
- `TestDetectUnverifiedFileClaim_MultipleFiles` — two paths, one seen one not → fires for unseen only

**Implement**: `DetectUnverifiedFileClaim` in `detectors.go`

### Task 4 — DetectPrematureCompletion

**Tests to write first:**
- `TestDetectPrematureCompletion_DoneNoTests` — "all done, fixed it", no test tool in last 3 steps → fires
- `TestDetectPrematureCompletion_DoneWithTests` — "fixed it" but `bash go test` in last step → nil
- `TestDetectPrematureCompletion_NoCompletionTerms` — neutral response → nil
- `TestDetectPrematureCompletion_BashMustHaveTestKeyword` — bash without test keyword does not count as verification

**Implement**: `DetectPrematureCompletion` in `detectors.go`

Note: `toolEntry` must carry `args string` for this test. Update `ledger.go` and
`RecordTool` signature before or during this task: `RecordTool(step int, toolName, args string)`.

### Task 5 — DetectSkippedDiagnostic

**Tests to write first:**
- `TestDetectSkippedDiagnostic_WriteFileNoDiag` — `write_file` with no prior diagnostic → fires
- `TestDetectSkippedDiagnostic_WriteFileWithDiag` — `write_file` but `read_file` in previous step → nil
- `TestDetectSkippedDiagnostic_BashDestructiveNoDiag` — bash `rm -rf foo` with no prior diag → fires
- `TestDetectSkippedDiagnostic_BashNonDestructive` — bash `go build ./...` with no prior diag → nil
- `TestDetectSkippedDiagnostic_EditFileCurrentStepDiag` — diagnostic in current step → nil

**Implement**: `DetectSkippedDiagnostic` in `detectors.go`

### Task 6 — DetectArchitectureAssumption

**Tests to write first:**
- `TestDetectArchitectureAssumption_PhraseNoExploration` — "the design is X", no exploration in last 3 steps → fires
- `TestDetectArchitectureAssumption_PhraseWithExploration` — `read_file` 2 steps ago → nil
- `TestDetectArchitectureAssumption_CaseInsensitive` — "The Flow Is" fires

**Implement**: `DetectArchitectureAssumption` in `detectors.go`

### Task 7 — InjectValidationPrompt intervention

**Tests to write first:**
- `TestInjectValidationPrompt_AppendsText` — response content gets prompt appended
- `TestInjectValidationPrompt_ActionIsContinue` — result action is HookActionContinue
- `TestInjectValidationPrompt_MutatedResponseSet` — MutatedResponse is non-nil

**Implement**: `InjectValidationPrompt` in `interventions.go`

### Task 8 — PauseForUser intervention

**Tests to write first:**
- `TestPauseForUser_ActionIsBlock` — result action is HookActionBlock
- `TestPauseForUser_ReasonContainsEvidence` — Reason field contains Detection.Evidence

**Implement**: `PauseForUser` in `interventions.go`

### Task 9 — RequestCritique intervention

**Tests to write first:**
- `TestRequestCritique_InjectsCritique` — mock CritiqueProvider returns "X is wrong", injected
- `TestRequestCritique_ProviderError` — provider returns error, function returns error
- `TestRequestCritique_ContextPropagated` — ctx cancellation propagates to provider

**Implement**: `RequestCritique` in `interventions.go`

### Task 10 — ConclusionWatcher core + Register

**Tests to write first:**
- `TestNew_Defaults` — zero config, all 5 patterns armed, inject mode
- `TestRegister_AppendsHooks` — after Register, cfg has one more pre + post + post hook
- `TestRegister_DoesNotReplaceExistingHooks` — pre-existing hooks not removed
- `TestWatcher_PostMessageHook_FiresDetection` — synthetic response with "obviously" → detection recorded
- `TestWatcher_PreToolUseHook_FiresOnWriteFile` — write_file with no prior diagnostic → denied
- `TestWatcher_PostToolUseHook_UpdatesLedger` — read_file result scanned for paths
- `TestWatcher_InterventionCount` — 3 detections → InterventionCount() == 3
- `TestWatcher_MaxInterventionsRespected` — MaxInterventionsPerRun=2, 5 detections → only 2 interventions
- `TestWatcher_EventEmitterCalled` — emitter callback invoked for each detection
- `TestWatcher_ConcurrencySafe` — Register + hooks called from multiple goroutines with `-race`
- `TestWatcher_Detections_ReturnsCopy` — mutating returned slice does not affect internal state

**Implement**: `plugin.go`

### Task 11 — Integration smoke test

**Tests to write first:**
- `TestWatcher_EndToEndInjectMode` — wire a watcher into a real `harness.RunnerConfig`,
  exercise the hook chain with a synthetic `PostMessageHookInput` containing hedge
  language and no file in the ledger; assert MutatedResponse contains the validation
  prompt and the EventEmitter was called with `EventConclusionDetected`.

This test uses only the harness types package (no live LLM), so it runs fully offline.

---

## Critical Integration Points and Traps

### 1. `PostMessageHookResult.MutatedResponse` must be a pointer to a new value

From `types.go`:
```go
type PostMessageHookResult struct {
    Action          HookAction
    Reason          string
    MutatedResponse *CompletionResult
}
```

When injecting a prompt, you must copy the `CompletionResult` and set `MutatedResponse`
to a pointer to the copy. Mutating `in.Response` directly causes aliasing — the original
struct is passed by value so the copy is safe, but future code changes could break this.
Always allocate a new `CompletionResult`:

```go
mutated := in.Response // copy (value type fields are safe)
mutated.Content = in.Response.Content + validationPrompt
result.MutatedResponse = &mutated
```

### 2. `PreToolUseResult` nil vs zero value

From `types.go`, `PreToolUse` returns `*PreToolUseResult`. The contract says "return nil
result (with nil error) to allow with no modification." Do not return `&PreToolUseResult{}`
with zero `Decision` (= `ToolHookAllow`) unless you also want to propagate a
`ModifiedArgs`. Return `nil, nil` for the no-op path.

### 3. Ledger file path normalisation

`filepath.Clean` is insufficient for case-insensitive file systems. Use
`filepath.ToSlash(filepath.Clean(path))` and compare lower-cased paths on darwin/windows.
For this plugin's purposes, call `strings.TrimPrefix(filepath.Clean(path), "./")` to
normalize relative prefixes. Do not use `filepath.Abs` — that requires a working
directory and makes tests non-deterministic.

### 4. Bash tool classification

The `bash` tool is simultaneously diagnostic, mutating, and verification-capable.
The heuristic is:
- Destructive: matches any `destructiveBashPatterns` (see above) → treated as mutating
- Verification: command contains "test" substring or matches `go test`, `pytest`,
  `cargo test`, `npm test`, `-run`, `check` → treated as verification
- Otherwise: treated as diagnostic/exploratory

Parse the `command` field from the JSON args. If the args are malformed, treat the tool
as non-diagnostic (fail closed).

### 5. Hook order matters for ledger state

The ledger must be updated in `preToolUseHook` BEFORE running detectors so that
`RecordTool` is current. However, `RecordFileSeen` from the tool's output is only
available in `postToolUseHook`. This means:

- `DetectUnverifiedFileClaim` in `postMessageHook` uses the state from the *end of the
  previous step's* file reads, not the current step. This is correct — the LLM's
  response is generated before it calls any tools, so it can only be claimed to have
  seen files that were read in prior steps.

### 6. MaxInterventionsPerRun and the atomic counter

Use `atomic.AddInt64` and `atomic.LoadInt64` on `w.interventionCount`. Check *before*
applying intervention:

```go
if w.cfg.MaxInterventionsPerRun > 0 {
    count := atomic.LoadInt64(&w.interventionCount)
    if count >= int64(w.cfg.MaxInterventionsPerRun) {
        // record detection but skip intervention
        return harness.PostMessageHookResult{Action: harness.HookActionContinue}, nil
    }
}
atomic.AddInt64(&w.interventionCount, 1)
```

### 7. Multiple patterns firing in the same step

When multiple patterns fire in one `AfterMessage` call, apply interventions in order
but stop at the first `HookActionBlock` — do not block AND inject. Record all
`DetectionResult` values regardless of whether the intervention is applied.

### 8. Step numbering via atomic currentStep

The `currentStep` field on `ConclusionWatcher` starts at 0. The `postMessageHook`
increments it *at the start* of `AfterMessage` (so the first LLM turn is step 1,
matching `PostMessageHookInput.Step`). The `preToolUseHook` reads `currentStep` without
incrementing. This is consistent with the harness's own step numbering which starts at 1.

### 9. Plugin does not need to implement PostMessageHook for PostToolUse

The harness `RunnerConfig.PostToolUseHooks` takes `[]PostToolUseHook`. The plugin
registers a `postToolUseHook` (implements `PostToolUseHook`) there. Do not confuse
`PostMessageHooks` (after LLM turn) and `PostToolUseHooks` (after tool call).

---

## go.mod / Import Path Considerations

```
module go-agent-harness       ← existing module name (confirmed from worktree go.mod)
go 1.22
```

The plugin package declaration:

```go
package conclusionwatcher     ← short, no hyphens in package name
```

Import in consumer code:

```go
import cw "go-agent-harness/plugins/conclusion-watcher"

watcher := cw.New(cw.WatcherConfig{
    Mode: cw.InterventionInjectPrompt,
})
watcher.Register(&cfg)
runner := harness.NewRunner(cfg)
```

The directory is `plugins/conclusion-watcher` (hyphen in path is fine; Go package
names use the package declaration, not the directory name).

No separate `go.mod` is created. The plugin is part of the root module. Running
`go test go-agent-harness/plugins/conclusion-watcher/...` from the repo root works
without any module configuration.

---

## Test Plan (TDD)

- **New failing tests to write first**: see TDD Task Sequence above (11 tasks)
- **Existing tests to update**: none — zero changes to `internal/harness/`
- **Regression tests required**:
  - Concurrency safety: `TestLedger_ConcurrencySafe`, `TestWatcher_ConcurrencySafe`
    (both must pass `go test -race`)
  - Nil pointer safety: every detector receives nil ledger in one test and must not panic
  - Empty content: every detector with `content=""` must return nil (no panic)
  - Empty args: `DetectSkippedDiagnostic` with `args=nil` must return nil (no panic)
  - `MaxInterventionsPerRun=0` means unlimited (not zero interventions)
  - `Register` called twice: hooks are appended twice; this is a user error but must not
    panic (add a note in the Register godoc warning against calling Register twice on
    the same cfg)

---

## Implementation Checklist

- [ ] Write `ledger.go` tests (Task 1), then implement
- [ ] Write `DetectHedgeAssertion` tests (Task 2), then implement
- [ ] Write `DetectUnverifiedFileClaim` tests (Task 3), then implement
- [ ] Extend `toolEntry` to carry `args string`; update tests from Task 1
- [ ] Write `DetectPrematureCompletion` tests (Task 4), then implement
- [ ] Write `DetectSkippedDiagnostic` tests (Task 5), then implement
- [ ] Write `DetectArchitectureAssumption` tests (Task 6), then implement
- [ ] Write `InjectValidationPrompt` tests (Task 7), then implement
- [ ] Write `PauseForUser` tests (Task 8), then implement
- [ ] Write `RequestCritique` tests (Task 9), then implement
- [ ] Write `ConclusionWatcher` + `Register` tests (Task 10), then implement
- [ ] Write end-to-end integration smoke test (Task 11), then verify it passes
- [ ] `go test ./plugins/conclusion-watcher/...` — all pass
- [ ] `go test -race ./plugins/conclusion-watcher/...` — all pass
- [ ] `go test ./...` from repo root — no regressions
- [ ] Add entry to `docs/INDEX.md` under Plugins
- [ ] Append implementation note to `docs/logs/engineering-log.md`

---

## Risks and Mitigations

- **Risk**: False positives from HedgeAssertion ("should be" appears in legitimate
  code comments quoted in responses).
  **Mitigation**: Confidence score < 1.0 lets callers filter. Add a
  `MinConfidence float64` field to `WatcherConfig` that suppresses interventions below
  the threshold (default 0.0 = all fire).

- **Risk**: Ledger misses file reads from MCP tools or non-standard tool names.
  **Mitigation**: `postToolUseHook` scans ALL tool outputs for file path patterns,
  regardless of tool name. This provides a fallback even for unknown tools.

- **Risk**: Bash tool misclassification causes too many SkippedDiagnostic false positives.
  **Mitigation**: Default `MutatingTools["bash"]` to false in the pre-tool hook; only
  promote to mutating when the destructive pattern regex matches. Log evidence in the
  DetectionResult so callers can tune the regex.

- **Risk**: `RequestCritique` intervention adds latency to every flagged step.
  **Mitigation**: Only used when `Mode == InterventionRequestCritique`. Document that
  this mode doubles LLM calls on flagged steps. Provide a timeout via the `ctx` argument.

- **Risk**: Worktree agents produce paths like `/workspace/foo.go` while the watcher
  records `foo.go`. Path mismatch breaks `HasSeenFile`.
  **Mitigation**: Normalize all recorded paths to their basename AND their full path.
  Store both; `HasSeenFile` matches if either is found.
