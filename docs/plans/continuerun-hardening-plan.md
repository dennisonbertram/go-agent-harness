# ContinueRun Hardening Plan

**Date:** 2026-03-31
**Scope:** `internal/harness/runner.go` + `internal/harness/runner_continuerun_security_test.go`
**Approach:** TDD — failing tests written first, then implementation.

---

## Goal

Fix all 7 state propagation gaps in `ContinueRun` so that continuation runs
fully inherit security controls, configuration, and rule state from the source
run. The existing pattern (snapshot under lock, assign to contState) is extended
to cover the 6 missing fields, plus a shallow-copy is converted to a deep copy
for `allowedTools`, and the doc comment is corrected.

---

## Current State

`ContinueRun` (~lines 928–1067 of `runner.go`) already propagates:
- `maxCostUSD` — snapshot at line ~963, assigned at ~1025
- `permissions` — snapshot at line ~964, assigned at ~1026
- `allowedTools` — snapshot at line ~969 **(shallow copy — bug #1)**
- `resolvedRoleModels` — snapshot at line ~975, assigned at ~1028

Missing propagation (the 7 findings):
1. `allowedTools` deep copy (shallow copy is a security bug)
2. `auditWriter` — not created for continuations at all
3. `profileName` — not snapshotted, not set in contState or req
4. `dynamicRules` — not snapshotted, not set in contState
5. `firedOnceRules` — not snapshotted, not set in contState
6. `forkDepth` — not snapshotted, not set in contState or req
7. Doc comment — describes a status transition that doesn't happen

---

## Task Breakdown

### TASK-001: Write Failing Regression Tests

**Type:** test
**Allowed files:** `internal/harness/runner_continuerun_security_test.go`
**Forbidden files:** `internal/harness/runner.go`
**Dependencies:** none
**Estimated complexity:** low
**Risk level:** low

Add 7 new test functions to `runner_continuerun_security_test.go`. Each test
must compile and demonstrate the bug (either by failing or by asserting the
broken behavior). They all pass after TASK-002.

#### Test 1 — allowedTools deep copy

```go
// TestContinueRun_AllowedToolsDeepCopied verifies that ContinueRun creates an
// independent copy of allowedTools rather than sharing the source slice's
// backing array. If the copy is shallow, mutating the source's slice would
// corrupt the continuation's tool filter.
func TestContinueRun_AllowedToolsDeepCopied(t *testing.T) {
    t.Parallel()

    prov := &continuationProvider{
        turns: []CompletionResult{
            {Content: "first"},
            {Content: "second"},
        },
    }
    runner := NewRunner(prov, NewRegistry(), RunnerConfig{
        DefaultModel: "test-model",
        MaxSteps:     4,
    })

    originalTools := []string{"bash", "read", "compact_history"}
    run1, err := runner.StartRun(RunRequest{
        Prompt:       "initial",
        AllowedTools: originalTools,
    })
    if err != nil {
        t.Fatalf("StartRun: %v", err)
    }
    waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

    run2, err := runner.ContinueRun(run1.ID, "follow up")
    if err != nil {
        t.Fatalf("ContinueRun: %v", err)
    }
    waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

    // Snapshot the continuation's allowedTools before mutation.
    runner.mu.RLock()
    contState, ok := runner.runs[run2.ID]
    if !ok {
        runner.mu.RUnlock()
        t.Fatal("continuation run state not found")
    }
    contToolsBefore := append([]string(nil), contState.allowedTools...)
    runner.mu.RUnlock()

    // Mutate the source run's allowedTools via its state pointer.
    runner.mu.Lock()
    srcState, ok := runner.runs[run1.ID]
    if ok && len(srcState.allowedTools) > 0 {
        srcState.allowedTools[0] = "MUTATED"
    }
    runner.mu.Unlock()

    // Verify the continuation's allowedTools is unaffected.
    runner.mu.RLock()
    contState2, ok := runner.runs[run2.ID]
    if !ok {
        runner.mu.RUnlock()
        t.Fatal("continuation run state not found (post-mutation)")
    }
    contToolsAfter := append([]string(nil), contState2.allowedTools...)
    runner.mu.RUnlock()

    for i, name := range contToolsAfter {
        if i < len(contToolsBefore) && name != contToolsBefore[i] {
            t.Errorf("allowedTools[%d] changed from %q to %q after source mutation (shallow copy detected)",
                i, contToolsBefore[i], name)
        }
    }
}
```

#### Test 2 — auditWriter created for continuation

```go
// TestContinueRun_AuditWriterCreatedForContinuation verifies that when
// AuditTrailEnabled is set and RolloutDir is configured, ContinueRun creates
// a fresh auditWriter for the continuation run. Without the fix, the
// continuation's auditWriter remains nil and no audit records are written for
// the continuation.
func TestContinueRun_AuditWriterCreatedForContinuation(t *testing.T) {
    t.Parallel()

    dir := t.TempDir()
    prov := &continuationProvider{
        turns: []CompletionResult{
            {Content: "first"},
            {Content: "second"},
        },
    }
    runner := NewRunner(prov, NewRegistry(), RunnerConfig{
        DefaultModel:      "test-model",
        MaxSteps:          4,
        RolloutDir:        dir,
        AuditTrailEnabled: true,
    })

    run1, err := runner.StartRun(RunRequest{Prompt: "initial"})
    if err != nil {
        t.Fatalf("StartRun: %v", err)
    }
    waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

    run2, err := runner.ContinueRun(run1.ID, "follow up")
    if err != nil {
        t.Fatalf("ContinueRun: %v", err)
    }
    waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

    runner.mu.RLock()
    contState, ok := runner.runs[run2.ID]
    if !ok {
        runner.mu.RUnlock()
        t.Fatal("continuation run state not found")
    }
    aw := contState.auditWriter
    runner.mu.RUnlock()

    if aw == nil {
        t.Error("ContinueRun: auditWriter is nil; continuation audit trail is silent")
    }
}
```

#### Test 3 — profileName propagated

```go
// TestContinueRun_PropagatesProfileName verifies that ContinueRun copies the
// source run's profileName into the continuation's runState. Without the fix,
// the continuation's profileName is empty and MCP servers defined in the
// profile are not loaded for the continuation.
func TestContinueRun_PropagatesProfileName(t *testing.T) {
    t.Parallel()

    prov := &continuationProvider{
        turns: []CompletionResult{
            {Content: "first"},
            {Content: "second"},
        },
    }
    runner := NewRunner(prov, NewRegistry(), RunnerConfig{
        DefaultModel: "test-model",
        MaxSteps:     4,
    })

    const wantProfile = "my-profile"
    run1, err := runner.StartRun(RunRequest{
        Prompt:      "initial",
        ProfileName: wantProfile,
    })
    if err != nil {
        t.Fatalf("StartRun: %v", err)
    }
    waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

    // Verify source run has the profile name.
    runner.mu.RLock()
    srcState, ok := runner.runs[run1.ID]
    if !ok {
        runner.mu.RUnlock()
        t.Fatal("source run state not found")
    }
    srcProfile := srcState.profileName
    runner.mu.RUnlock()

    if srcProfile != wantProfile {
        t.Fatalf("source run profileName = %q, want %q", srcProfile, wantProfile)
    }

    run2, err := runner.ContinueRun(run1.ID, "follow up")
    if err != nil {
        t.Fatalf("ContinueRun: %v", err)
    }
    waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

    runner.mu.RLock()
    contState, ok := runner.runs[run2.ID]
    if !ok {
        runner.mu.RUnlock()
        t.Fatal("continuation run state not found")
    }
    gotProfile := contState.profileName
    runner.mu.RUnlock()

    if gotProfile != wantProfile {
        t.Errorf("ContinueRun profileName = %q, want %q (profile not propagated)", gotProfile, wantProfile)
    }
}
```

#### Test 4 — dynamicRules propagated

```go
// TestContinueRun_DynamicRulesPropagated verifies that ContinueRun copies the
// source run's dynamicRules into the continuation. Without the fix, the
// continuation has no dynamic rules and pattern-triggered system prompt
// injections are silently disabled for the continuation.
func TestContinueRun_DynamicRulesPropagated(t *testing.T) {
    t.Parallel()

    prov := &continuationProvider{
        turns: []CompletionResult{
            {Content: "first"},
            {Content: "second"},
        },
    }
    runner := NewRunner(prov, NewRegistry(), RunnerConfig{
        DefaultModel: "test-model",
        MaxSteps:     4,
    })

    wantRules := []DynamicRule{
        {
            ID:       "rule-1",
            Trigger:  RuleTrigger{ToolNames: []string{"bash"}},
            Content:  "Be careful with bash.",
            FireOnce: true,
        },
        {
            ID:      "rule-2",
            Trigger: RuleTrigger{ToolNames: []string{"write_file"}},
            Content: "Always check file paths before writing.",
        },
    }
    run1, err := runner.StartRun(RunRequest{
        Prompt:       "initial",
        DynamicRules: wantRules,
    })
    if err != nil {
        t.Fatalf("StartRun: %v", err)
    }
    waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

    run2, err := runner.ContinueRun(run1.ID, "follow up")
    if err != nil {
        t.Fatalf("ContinueRun: %v", err)
    }
    waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

    runner.mu.RLock()
    contState, ok := runner.runs[run2.ID]
    if !ok {
        runner.mu.RUnlock()
        t.Fatal("continuation run state not found")
    }
    gotRules := contState.dynamicRules
    runner.mu.RUnlock()

    if len(gotRules) != len(wantRules) {
        t.Fatalf("ContinueRun dynamicRules len = %d, want %d (rules not propagated)", len(gotRules), len(wantRules))
    }
    for i, rule := range gotRules {
        if rule.ID != wantRules[i].ID {
            t.Errorf("dynamicRules[%d].ID = %q, want %q", i, rule.ID, wantRules[i].ID)
        }
        if rule.Content != wantRules[i].Content {
            t.Errorf("dynamicRules[%d].Content = %q, want %q", i, rule.Content, wantRules[i].Content)
        }
    }
}
```

#### Test 5 — dynamicRules deep copied

```go
// TestContinueRun_DynamicRulesDeepCopied verifies that ContinueRun creates an
// independent copy of dynamicRules. If the copy is shallow, mutating the
// source's rule slice would corrupt the continuation's rules.
func TestContinueRun_DynamicRulesDeepCopied(t *testing.T) {
    t.Parallel()

    prov := &continuationProvider{
        turns: []CompletionResult{
            {Content: "first"},
            {Content: "second"},
        },
    }
    runner := NewRunner(prov, NewRegistry(), RunnerConfig{
        DefaultModel: "test-model",
        MaxSteps:     4,
    })

    run1, err := runner.StartRun(RunRequest{
        Prompt: "initial",
        DynamicRules: []DynamicRule{
            {
                ID:      "rule-1",
                Trigger: RuleTrigger{ToolNames: []string{"bash"}},
                Content: "original content",
            },
        },
    })
    if err != nil {
        t.Fatalf("StartRun: %v", err)
    }
    waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

    run2, err := runner.ContinueRun(run1.ID, "follow up")
    if err != nil {
        t.Fatalf("ContinueRun: %v", err)
    }
    waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

    // Mutate the source run's dynamicRules in-place.
    runner.mu.Lock()
    srcState, ok := runner.runs[run1.ID]
    if ok && len(srcState.dynamicRules) > 0 {
        srcState.dynamicRules[0].Content = "MUTATED"
        if len(srcState.dynamicRules[0].Trigger.ToolNames) > 0 {
            srcState.dynamicRules[0].Trigger.ToolNames[0] = "MUTATED_TOOL"
        }
    }
    runner.mu.Unlock()

    // Verify the continuation's dynamicRules is unaffected.
    runner.mu.RLock()
    contState, ok := runner.runs[run2.ID]
    if !ok {
        runner.mu.RUnlock()
        t.Fatal("continuation run state not found")
    }
    gotContent := ""
    gotTrigger := ""
    if len(contState.dynamicRules) > 0 {
        gotContent = contState.dynamicRules[0].Content
        if len(contState.dynamicRules[0].Trigger.ToolNames) > 0 {
            gotTrigger = contState.dynamicRules[0].Trigger.ToolNames[0]
        }
    }
    runner.mu.RUnlock()

    if gotContent == "MUTATED" {
        t.Error("continuation dynamicRules[0].Content was mutated by source change (shallow copy of rule struct)")
    }
    if gotTrigger == "MUTATED_TOOL" {
        t.Error("continuation dynamicRules[0].Trigger.ToolNames[0] was mutated by source change (shallow copy of ToolNames)")
    }
}
```

#### Test 6 — firedOnceRules propagated

```go
// TestContinueRun_FiredOnceRulesPropagated verifies that ContinueRun copies
// the source run's firedOnceRules into the continuation. Without the fix, a
// FireOnce rule that already fired in the source run would fire again in the
// continuation (because the continuation's firedOnceRules starts empty).
func TestContinueRun_FiredOnceRulesPropagated(t *testing.T) {
    t.Parallel()

    prov := &continuationProvider{
        turns: []CompletionResult{
            {Content: "first"},
            {Content: "second"},
        },
    }
    runner := NewRunner(prov, NewRegistry(), RunnerConfig{
        DefaultModel: "test-model",
        MaxSteps:     4,
    })

    run1, err := runner.StartRun(RunRequest{Prompt: "initial"})
    if err != nil {
        t.Fatalf("StartRun: %v", err)
    }
    waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

    // Inject a fired rule ID into the source run's firedOnceRules to simulate
    // a rule having fired during run 1.
    const firedRuleID = "rule-fired-in-run1"
    runner.mu.Lock()
    srcState, ok := runner.runs[run1.ID]
    if !ok {
        runner.mu.Unlock()
        t.Fatal("source run state not found before inject")
    }
    if srcState.firedOnceRules == nil {
        srcState.firedOnceRules = make(map[string]bool)
    }
    srcState.firedOnceRules[firedRuleID] = true
    runner.mu.Unlock()

    run2, err := runner.ContinueRun(run1.ID, "follow up")
    if err != nil {
        t.Fatalf("ContinueRun: %v", err)
    }
    waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

    runner.mu.RLock()
    contState, ok := runner.runs[run2.ID]
    if !ok {
        runner.mu.RUnlock()
        t.Fatal("continuation run state not found")
    }
    gotFired := contState.firedOnceRules[firedRuleID]
    runner.mu.RUnlock()

    if !gotFired {
        t.Errorf("ContinueRun firedOnceRules[%q] = false, want true (previously-fired rule would re-fire)", firedRuleID)
    }
}
```

#### Test 7 — forkDepth propagated

```go
// TestContinueRun_ForkDepthPropagated verifies that ContinueRun copies the
// source run's forkDepth into the continuation. Without the fix, the
// continuation's forkDepth defaults to 0 (root agent), incorrectly enabling
// task_complete and disabling depth-pressure messages for child agents.
func TestContinueRun_ForkDepthPropagated(t *testing.T) {
    t.Parallel()

    prov := &continuationProvider{
        turns: []CompletionResult{
            {Content: "first"},
            {Content: "second"},
        },
    }
    runner := NewRunner(prov, NewRegistry(), RunnerConfig{
        DefaultModel: "test-model",
        MaxSteps:     4,
    })

    run1, err := runner.StartRun(RunRequest{Prompt: "initial"})
    if err != nil {
        t.Fatalf("StartRun: %v", err)
    }
    waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

    // Set forkDepth on the source run to simulate it being a child agent.
    const wantForkDepth = 3
    runner.mu.Lock()
    srcState, ok := runner.runs[run1.ID]
    if !ok {
        runner.mu.Unlock()
        t.Fatal("source run state not found before inject")
    }
    srcState.forkDepth = wantForkDepth
    runner.mu.Unlock()

    run2, err := runner.ContinueRun(run1.ID, "follow up")
    if err != nil {
        t.Fatalf("ContinueRun: %v", err)
    }
    waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

    runner.mu.RLock()
    contState, ok := runner.runs[run2.ID]
    if !ok {
        runner.mu.RUnlock()
        t.Fatal("continuation run state not found")
    }
    gotDepth := contState.forkDepth
    runner.mu.RUnlock()

    if gotDepth != wantForkDepth {
        t.Errorf("ContinueRun forkDepth = %d, want %d (depth-gating broken for continued child agent)", gotDepth, wantForkDepth)
    }
}
```

---

### TASK-002: Implement All 7 Fixes in runner.go

**Type:** bugfix
**Allowed files:** `internal/harness/runner.go`
**Forbidden files:** `internal/harness/runner_continuerun_security_test.go`
**Dependencies:** TASK-001
**Estimated complexity:** medium
**Risk level:** medium

**Pre-implementation verification checklist (read before coding):**

Before writing any code, read these sections of `runner.go`:

1. `execute()` function — does it re-assign `state.dynamicRules` from
   `req.DynamicRules`? If yes, `req.DynamicRules` must be populated
   instead of (or in addition to) `contState.dynamicRules`. Based on
   reading `mergeDynamicRules` usage, it appears `execute()` does NOT
   overwrite `state.dynamicRules`, but **verify** by searching for
   `dynamicRules =` in `execute()`.

2. `runPreflight()` — does it use `req.ProfileName` to load MCP servers?
   If yes, the continuation RunRequest must carry `ProfileName`. Based on
   reading `runPreflight` at line ~660, it reads `req.ProfileName` to load
   the profile. **Confirm** before adding `ProfileName` to req.

3. `audittrail.AuditWriter` — is the writer goroutine-safe? Multiple runs
   can write to the same daily `audit.jsonl`. **Confirm** by reading
   `internal/forensics/audittrail/`.

#### Fix 1 — Deep copy allowedTools (line ~969 in ContinueRun, line ~539 in StartRun)

In `ContinueRun` snapshot block, change:
```go
srcAllowedTools := state.allowedTools
```
to:
```go
srcAllowedTools := append([]string(nil), state.allowedTools...)
```

In `StartRun` state construction (line ~539), change:
```go
allowedTools: req.AllowedTools,
```
to:
```go
allowedTools: append([]string(nil), req.AllowedTools...),
```

The `contState` assignment at line ~1027 (`allowedTools: srcAllowedTools`) is
already correct once the snapshot is a true copy.

#### Fix 2 — Create auditWriter for continuation

**Location:** After the rollout recorder block, before `contState` construction
(between lines ~1009 and ~1015).

Add:
```go
// Create audit writer for the continuation run, mirroring StartRun's behavior.
// The audit log is shared across all runs in the session (per-day file).
var contAW *audittrail.AuditWriter
if r.config.AuditTrailEnabled && r.config.RolloutDir != "" {
    auditPath := auditLogPath(r.config.RolloutDir)
    var awErr error
    contAW, awErr = audittrail.NewAuditWriter(auditPath)
    if awErr != nil && r.config.Logger != nil {
        r.config.Logger.Error("audit trail: failed to create writer for continuation",
            "run_id", newRun.ID, "error", awErr)
    }
}
```

In `contState` struct literal, add:
```go
auditWriter: contAW,
```

#### Fix 3 — Propagate profileName

In the snapshot block (after ~line 975), add:
```go
srcProfileName := state.profileName
```

In `contState` struct literal, add:
```go
profileName: srcProfileName,
```

In the `RunRequest` construction (~line 1052), add:
```go
ProfileName: srcProfileName,
```

#### Fix 4 — Deep copy and propagate dynamicRules

In the snapshot block, add:
```go
srcDynamicRules := deepCopyDynamicRules(state.dynamicRules)
```

In `contState` struct literal, add:
```go
dynamicRules: srcDynamicRules,
```

Add helper function near `mergeDynamicRules` (~line 2865):
```go
// deepCopyDynamicRules returns an independent copy of a DynamicRule slice.
// Each rule's Trigger.ToolNames slice is also deep-copied so that mutations
// to the source cannot affect the copy and vice versa. Returns nil when
// rules is nil, preserving the nil/empty distinction.
func deepCopyDynamicRules(rules []DynamicRule) []DynamicRule {
    if rules == nil {
        return nil
    }
    out := make([]DynamicRule, len(rules))
    for i, r := range rules {
        out[i] = r
        out[i].Trigger.ToolNames = append([]string(nil), r.Trigger.ToolNames...)
    }
    return out
}
```

Note: `req.DynamicRules` is intentionally left nil in the continuation
RunRequest. The rules are already fully merged in `contState.dynamicRules`.
If `execute()` were to call `mergeDynamicRules(r.config.DynamicRules,
req.DynamicRules)` and overwrite `state.dynamicRules`, that would re-merge
runner-level rules with an empty req — producing only runner-level rules and
losing the per-request rules from the original run. This is why setting
`contState.dynamicRules` directly is correct (assuming `execute()` does not
re-assign the field, which must be verified per the pre-implementation
checklist).

#### Fix 5 — Deep copy and propagate firedOnceRules

In the snapshot block (while the lock is held — CRITICAL: must happen before
`r.mu.Unlock()` at line ~997), add:
```go
srcFiredOnce := make(map[string]bool, len(state.firedOnceRules))
for k, v := range state.firedOnceRules {
    srcFiredOnce[k] = v
}
```

In `contState` struct literal, add:
```go
firedOnceRules: srcFiredOnce,
```

Note: An empty `srcFiredOnce` map (from an empty source) is different from
`firedOnceRules: make(map[string]bool)` that StartRun uses — both are
empty maps. The distinction (nil vs empty map) doesn't matter for the
`firedOnce[id]` lookups in `evaluateDynamicRules` because Go map lookups on
nil maps return the zero value (false). However, for consistency with StartRun,
if `state.firedOnceRules` is nil, produce `make(map[string]bool)` rather than
a nil map:
```go
var srcFiredOnce map[string]bool
if state.firedOnceRules != nil {
    srcFiredOnce = make(map[string]bool, len(state.firedOnceRules))
    for k, v := range state.firedOnceRules {
        srcFiredOnce[k] = v
    }
} else {
    srcFiredOnce = make(map[string]bool)
}
```

#### Fix 6 — Propagate forkDepth

In the snapshot block, add:
```go
srcForkDepth := state.forkDepth
```

In `contState` struct literal, add:
```go
forkDepth: srcForkDepth,
```

In the `RunRequest` construction, add:
```go
ForkDepth: srcForkDepth,
```

#### Fix 7 — Update doc comment (~line 913)

Replace the existing `ContinueRun` godoc block with:
```go
// ContinueRun appends a follow-up user message to a completed run and starts a
// new execution under the same conversation_id. The original run state is kept
// intact (its Status remains RunStatusCompleted); the source run is marked with
// state.continued = true to prevent any second continuation of the same run.
// The new run inherits all security controls and configuration from the source:
//
//   - maxCostUSD          — per-run spending ceiling
//   - permissions         — sandbox scope and approval policy
//   - allowedTools        — per-run tool filter (deep-copied)
//   - profileName         — named profile for MCP server loading
//   - dynamicRules        — pattern-triggered system prompt injections (deep-copied)
//   - firedOnceRules      — set of FireOnce rule IDs already fired (deep-copied)
//   - forkDepth           — agent nesting depth for task_complete gating
//   - resolvedRoleModels  — per-request model overrides
//
// Errors:
//   - ErrRunNotFound      — the source run does not exist.
//   - ErrRunNotCompleted  — the source run has not reached RunStatusCompleted
//     (it is still running, queued, waiting for user, or has failed).
//   - "already continued" — the source run has already been continued once.
//   - validation error    — message is empty.
//
// The method is safe for concurrent use. Only one goroutine can successfully
// continue a given completed run: the first to acquire the lock sets
// state.continued = true, so subsequent callers see the "already continued"
// error.
```

---

### TASK-003: Verify Green Suite

**Type:** test (verification)
**Allowed files:** read-only
**Dependencies:** TASK-002
**Estimated complexity:** low
**Risk level:** low

Run:
```bash
# Targeted: all ContinueRun tests (should be 15 total: 8 pre-existing + 7 new)
go test -race -count=1 -run TestContinueRun ./internal/harness/...

# Full suite: no regressions anywhere
go test -race -count=1 ./internal/harness/...

# Vet: no static errors
go vet ./internal/harness/...
```

Expected results:
- All 15 `TestContinueRun*` tests pass.
- Full `./internal/harness/...` suite passes.
- `go vet` exits 0.
- `-race` reports no data races.

---

## Behavioral Test Specification

| ID | Behavior | Condition | Expected Outcome | Covered by Task |
|----|----------|-----------|-----------------|-----------------|
| BT-001 | allowedTools deep copy | Source allowedTools mutated post-ContinueRun | contState.allowedTools unaffected | TASK-001, TASK-002 |
| BT-002 | auditWriter created | AuditTrailEnabled=true, RolloutDir set | contState.auditWriter != nil | TASK-001, TASK-002 |
| BT-003 | profileName propagated | Source profileName="my-profile" | contState.profileName=="my-profile" | TASK-001, TASK-002 |
| BT-004 | dynamicRules propagated | Source has 2 DynamicRules | contState.dynamicRules has same 2 rules | TASK-001, TASK-002 |
| BT-005 | dynamicRules deep copied | Source dynamicRules mutated post-ContinueRun | contState.dynamicRules unaffected | TASK-001, TASK-002 |
| BT-006 | firedOnceRules propagated | Source has firedOnceRules={'rule-1':true} | contState.firedOnceRules['rule-1']==true | TASK-001, TASK-002 |
| BT-007 | forkDepth propagated | Source forkDepth set to 3 | contState.forkDepth==3 | TASK-001, TASK-002 |
| BT-008 | StartRun allowedTools deep copy | Caller mutates AllowedTools post-StartRun | state.allowedTools unaffected | TASK-002 |
| BT-009 | No regressions | Full suite after all changes | All pre-existing tests pass | TASK-003 |
| BT-010 | Race-free | -race flag during full suite | No data races | TASK-003 |

---

## Dependency Graph

```
TASK-001 (write failing tests)
    |
    v
TASK-002 (implement fixes)
    |
    v
TASK-003 (verify green)
```

All tasks are strictly sequential. All implementation is in `runner.go`; all
tests are in `runner_continuerun_security_test.go`. No parallelization is
possible without file overlap.

## Parallelization Plan

Wave 1: [TASK-001]
Wave 2: [TASK-002] — depends on Wave 1
Wave 3: [TASK-003] — depends on Wave 2

## File Boundary Map

| Task | Owned Files |
|------|-------------|
| TASK-001 | `internal/harness/runner_continuerun_security_test.go` |
| TASK-002 | `internal/harness/runner.go` |
| TASK-003 | None (read-only verification) |

Zero overlap across all waves.

---

## Risks and Open Questions

### 1. firedOnceRules inheritance semantics (needs design decision)
The plan copies the source run's `firedOnceRules` into the continuation so
FireOnce rules that already fired in run 1 don't re-fire in run 2. This is the
conservative interpretation of "continuation". However, if the intended design
is "each continuation is an independent run segment that sees all rules fresh",
the copy would be wrong and `firedOnceRules` should be initialized empty
(matching `StartRun`). **Clarify before implementing Fix 5.**

### 2. execute() and dynamicRules re-assignment (must verify)
If `execute()` calls `mergeDynamicRules(r.config.DynamicRules, req.DynamicRules)`
and overwrites `state.dynamicRules`, then setting `contState.dynamicRules`
directly in ContinueRun would be ineffective. Search for
`state.dynamicRules =` (or `dynamicRules =`) in `execute()` before coding Fix 4.
If `execute()` does overwrite, populate `req.DynamicRules` with the raw
per-request rules instead (which requires de-merging runner-level rules —
non-trivial). Based on current code review, `execute()` does NOT re-assign
`state.dynamicRules`.

### 3. AuditWriter concurrent access safety
`auditLogPath` returns a shared daily file. Multiple `AuditWriter` instances
can point at the same file path. The audit writer must be goroutine-safe for
concurrent appends. Verify by reading `internal/forensics/audittrail/` before
implementing Fix 2.

### 4. profileName and runPreflight interaction
`runPreflight` uses `req.ProfileName` to load the profile (confirmed at line
~675). If `req.ProfileName` is empty, no profile is loaded for the
continuation — MCP servers, isolation mode, etc. defined in the profile are
silently dropped. Fix 3 must set both `contState.profileName` and
`req.ProfileName`.

### 5. forkDepth and req.ForkDepth interaction
`execute()` may read `req.ForkDepth` to enforce depth limits or compute the
effective fork depth. If it does, `req.ForkDepth` must be set. If `execute()`
only reads from `state.forkDepth`, setting `contState.forkDepth` is sufficient.
**Verify before implementing Fix 6.**

---

## Exact Code Location Reference

| Finding | File | Approx. Lines | Action |
|---------|------|---------------|--------|
| 1 (allowedTools shallow copy) | runner.go | ~969 (ContinueRun snapshot), ~539 (StartRun state) | Change to `append([]string(nil), ...)` |
| 2 (auditWriter) | runner.go | ~1009–1015 (before contState) | Add audit writer creation block |
| 3 (profileName) | runner.go | ~975 (snapshot), ~1015 (contState), ~1052 (req) | Add snapshot + assignment |
| 4 (dynamicRules) | runner.go | ~975 (snapshot), ~1015 (contState); ~2865 (helper) | Add deep copy + helper |
| 5 (firedOnceRules) | runner.go | ~975 (snapshot, while lock held), ~1015 (contState) | Add map copy |
| 6 (forkDepth) | runner.go | ~975 (snapshot), ~1015 (contState), ~1052 (req) | Add snapshot + assignment |
| 7 (doc comment) | runner.go | ~913 | Replace godoc block |

---

## Review Triggers

TASK-002 requires review before merge:

- **Security review**: Verify deep copy patterns eliminate all aliasing paths.
  Specifically check `firedOnceRules` map copy and `dynamicRules`
  slice-of-struct copy (ToolNames sub-slice).
- **Correctness review**: Confirm `firedOnceRules` inheritance semantics
  match design intent (copy vs fresh map).
- **Concurrency review**: Confirm `audittrail.AuditWriter` is goroutine-safe
  when multiple run instances write to the same daily file.
- **Correctness review**: Confirm `execute()` does not overwrite
  `state.dynamicRules` after the contState is registered in `r.runs`.
