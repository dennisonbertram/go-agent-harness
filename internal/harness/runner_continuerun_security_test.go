package harness

// Tests for security properties of ContinueRun: budget propagation and
// permission propagation.
//
// Regression tests for GitHub issue #222:
// ContinueRun drops maxCostUSD and permissions from source run.
//
// Regression tests for GitHub issue #526:
// ContinueRun does not deep-copy allowedTools, does not create auditWriter,
// does not propagate profileName, dynamicRules, firedOnceRules, or forkDepth.

import (
	"fmt"
	"reflect"
	"testing"
)

// TestContinueRunPropagatesMaxCostUSD verifies that ContinueRun copies the
// source run's maxCostUSD into the continuation runState. Without this fix,
// the continuation defaults to 0 (unlimited), allowing budget bypass.
func TestContinueRunPropagatesMaxCostUSD(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	const wantMaxCost = 1.25
	run1, err := runner.StartRun(RunRequest{
		Prompt:     "initial",
		MaxCostUSD: wantMaxCost,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	// Verify source run has the expected maxCostUSD.
	runner.mu.RLock()
	srcState, ok := runner.runs[run1.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("source run state not found")
	}
	srcMaxCost := srcState.maxCostUSD
	runner.mu.RUnlock()

	if srcMaxCost != wantMaxCost {
		t.Fatalf("source run maxCostUSD = %v, want %v", srcMaxCost, wantMaxCost)
	}

	run2, err := runner.ContinueRun(run1.ID, "follow up")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	// The continuation must carry the same maxCostUSD as the source run.
	runner.mu.RLock()
	contState, ok := runner.runs[run2.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("continuation run state not found")
	}
	gotMaxCost := contState.maxCostUSD
	runner.mu.RUnlock()

	if gotMaxCost != wantMaxCost {
		t.Errorf("ContinueRun maxCostUSD = %v, want %v (budget bypass detected)", gotMaxCost, wantMaxCost)
	}
}

// TestContinueRunPropagatesPermissions verifies that ContinueRun copies the
// source run's permissions into the continuation runState. Without this fix,
// the continuation gets a zero-value PermissionConfig, bypassing any sandbox
// or approval constraints set on the original run.
func TestContinueRunPropagatesPermissions(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	wantPerms := PermissionConfig{
		Sandbox:  SandboxScopeWorkspace,
		Approval: ApprovalPolicyDestructive,
	}
	run1, err := runner.StartRun(RunRequest{
		Prompt:      "initial",
		Permissions: &wantPerms,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	// Verify source run has the expected permissions.
	runner.mu.RLock()
	srcState, ok := runner.runs[run1.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("source run state not found")
	}
	srcPerms := srcState.permissions
	runner.mu.RUnlock()

	if srcPerms != wantPerms {
		t.Fatalf("source run permissions = %+v, want %+v", srcPerms, wantPerms)
	}

	run2, err := runner.ContinueRun(run1.ID, "follow up")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	// The continuation must carry the same permissions as the source run.
	runner.mu.RLock()
	contState, ok := runner.runs[run2.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("continuation run state not found")
	}
	gotPerms := contState.permissions
	runner.mu.RUnlock()

	if gotPerms != wantPerms {
		t.Errorf("ContinueRun permissions = %+v, want %+v (permission bypass detected)", gotPerms, wantPerms)
	}
}

// TestContinueRunZeroMaxCostIsPreserved verifies that when a source run has
// maxCostUSD == 0 (unlimited), the continuation also inherits unlimited (0).
// This confirms the propagation doesn't accidentally impose a ceiling.
func TestContinueRunZeroMaxCostIsPreserved(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	// Start with no cost ceiling (MaxCostUSD = 0 = unlimited).
	run1, err := runner.StartRun(RunRequest{
		Prompt:     "initial",
		MaxCostUSD: 0,
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
	gotMaxCost := contState.maxCostUSD
	runner.mu.RUnlock()

	if gotMaxCost != 0 {
		t.Errorf("ContinueRun maxCostUSD = %v, want 0 (unlimited)", gotMaxCost)
	}
}

// TestContinueRunPropagatesResolvedRoleModels verifies that ContinueRun copies
// the source run's resolvedRoleModels into the continuation runState AND into
// the RunRequest so that execute() re-resolves to the same per-request
// RoleModels. Without this fix, per-request Primary or Summarizer overrides
// are silently dropped when a run is continued.
func TestContinueRunPropagatesResolvedRoleModels(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	// Use runner-level role models that differ from the per-request ones so we
	// can distinguish which source won.
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
		RoleModels: RoleModels{
			Primary:    "runner-primary",
			Summarizer: "runner-summarizer",
		},
	})

	wantRoleModels := RoleModels{
		Primary:    "per-request-primary",
		Summarizer: "per-request-summarizer",
	}
	run1, err := runner.StartRun(RunRequest{
		Prompt:     "initial",
		RoleModels: &wantRoleModels,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	// Verify source run has the per-request resolvedRoleModels (not runner-level).
	runner.mu.RLock()
	srcState, ok := runner.runs[run1.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("source run state not found")
	}
	srcRoleModels := srcState.resolvedRoleModels
	runner.mu.RUnlock()

	if srcRoleModels != wantRoleModels {
		t.Fatalf("source run resolvedRoleModels = %+v, want %+v", srcRoleModels, wantRoleModels)
	}

	run2, err := runner.ContinueRun(run1.ID, "follow up")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	// The continuation must carry the same resolvedRoleModels as the source run,
	// not the runner-level fallback values.
	runner.mu.RLock()
	contState, ok := runner.runs[run2.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("continuation run state not found")
	}
	gotRoleModels := contState.resolvedRoleModels
	runner.mu.RUnlock()

	if gotRoleModels != wantRoleModels {
		t.Errorf("ContinueRun resolvedRoleModels = %+v, want %+v (per-request role models dropped)", gotRoleModels, wantRoleModels)
	}
}

// TestContinueRunRoleModelsZeroValuePreserved verifies that when a source run
// has no per-request RoleModels override (resolvedRoleModels is zero-value),
// the continuation also gets a zero-value resolvedRoleModels — the fix does
// not accidentally inject non-empty values.
func TestContinueRunRoleModelsZeroValuePreserved(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	// No runner-level role models, no per-request role models.
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	run1, err := runner.StartRun(RunRequest{
		Prompt: "initial",
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
	gotRoleModels := contState.resolvedRoleModels
	runner.mu.RUnlock()

	zeroRoleModels := RoleModels{}
	if gotRoleModels != zeroRoleModels {
		t.Errorf("ContinueRun resolvedRoleModels = %+v, want zero value %+v", gotRoleModels, zeroRoleModels)
	}
}

// TestContinueRunDefaultPermissionsPreserved verifies that when a source run
// uses the default permissions (unrestricted, no approval), the continuation
// also inherits those exact default permissions.
func TestContinueRunDefaultPermissionsPreserved(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	// No explicit Permissions — should use DefaultPermissionConfig().
	run1, err := runner.StartRun(RunRequest{
		Prompt: "initial",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	runner.mu.RLock()
	srcState, ok := runner.runs[run1.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("source run state not found")
	}
	srcPerms := srcState.permissions
	runner.mu.RUnlock()

	// Source should have defaults.
	defaultPerms := DefaultPermissionConfig()
	if srcPerms != defaultPerms {
		t.Fatalf("source run permissions = %+v, want default %+v", srcPerms, defaultPerms)
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
	gotPerms := contState.permissions
	runner.mu.RUnlock()

	if gotPerms != defaultPerms {
		t.Errorf("ContinueRun permissions = %+v, want default %+v", gotPerms, defaultPerms)
	}
}

// TestContinueRun_AllowedToolsDeepCopy verifies that the continuation's
// allowedTools slice is an independent copy of the source run's slice.
// Mutating the source run's slice after continuation must not affect the
// continuation's allowedTools.
func TestContinueRun_AllowedToolsDeepCopy(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	origTools := []string{"bash", "read_file"}
	run1, err := runner.StartRun(RunRequest{
		Prompt:       "initial",
		AllowedTools: origTools,
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

	// Mutate the source run's allowedTools under the lock.
	runner.mu.Lock()
	srcState := runner.runs[run1.ID]
	if len(srcState.allowedTools) > 0 {
		srcState.allowedTools[0] = "MUTATED"
	}
	runner.mu.Unlock()

	// The continuation's allowedTools must be unaffected by the mutation.
	runner.mu.RLock()
	contState, ok := runner.runs[run2.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("continuation run state not found")
	}
	gotTools := append([]string(nil), contState.allowedTools...)
	runner.mu.RUnlock()

	if len(gotTools) != len(origTools) {
		t.Fatalf("continuation allowedTools length = %d, want %d", len(gotTools), len(origTools))
	}
	if gotTools[0] != origTools[0] {
		t.Errorf("continuation allowedTools[0] = %q, want %q (mutation leaked through)", gotTools[0], origTools[0])
	}
}

// TestContinueRun_AllowedToolsNilPreserved verifies that a nil allowedTools
// slice on the source run results in a nil (not empty) allowedTools on the
// continuation. Nil means "no per-run restriction"; empty would be wrong.
func TestContinueRun_AllowedToolsNilPreserved(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	// No AllowedTools — source run should have nil allowedTools.
	run1, err := runner.StartRun(RunRequest{
		Prompt: "initial",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	// Confirm source has nil allowedTools.
	runner.mu.RLock()
	srcState, ok := runner.runs[run1.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("source run state not found")
	}
	srcTools := srcState.allowedTools
	runner.mu.RUnlock()

	if srcTools != nil {
		t.Fatalf("source run allowedTools should be nil when none specified, got %v", srcTools)
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
	gotTools := contState.allowedTools
	runner.mu.RUnlock()

	if gotTools != nil {
		t.Errorf("ContinueRun allowedTools = %v, want nil (nil != empty slice)", gotTools)
	}
}

// TestContinueRun_ProfileNamePreserved verifies that a continuation inherits
// the profileName from the source run.
func TestContinueRun_ProfileNamePreserved(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
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
		t.Errorf("ContinueRun profileName = %q, want %q", gotProfile, wantProfile)
	}
}

// TestContinueRun_DynamicRulesPreserved verifies that the continuation
// inherits the source run's dynamic rules and that the rules are deep-copied
// (mutating Trigger.ToolNames in the source does not affect the continuation).
func TestContinueRun_DynamicRulesPreserved(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	wantRules := []DynamicRule{
		{
			ID:       "rule-1",
			Content:  "Be careful with files.",
			FireOnce: true,
			Trigger:  RuleTrigger{ToolNames: []string{"bash", "write_file"}},
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

	// Snapshot the original ToolNames value before the continuation so we can
	// verify it is unchanged after mutation of the source run.
	const origToolName = "bash"

	run2, err := runner.ContinueRun(run1.ID, "follow up")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	// Mutate the source run's dynamicRules.Trigger.ToolNames to verify deep copy.
	// Note: wantRules shares backing storage with the source state's ToolNames
	// slice (mergeDynamicRules does a shallow copy), so wantRules is also
	// mutated here — use the pre-snapshot constant origToolName for assertions.
	runner.mu.Lock()
	srcState := runner.runs[run1.ID]
	if len(srcState.dynamicRules) > 0 && len(srcState.dynamicRules[0].Trigger.ToolNames) > 0 {
		srcState.dynamicRules[0].Trigger.ToolNames[0] = "MUTATED"
	}
	runner.mu.Unlock()

	runner.mu.RLock()
	contState, ok := runner.runs[run2.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("continuation run state not found")
	}
	gotRules := contState.dynamicRules
	runner.mu.RUnlock()

	if len(gotRules) != len(wantRules) {
		t.Fatalf("continuation dynamicRules length = %d, want %d", len(gotRules), len(wantRules))
	}
	if gotRules[0].ID != wantRules[0].ID {
		t.Errorf("continuation dynamicRules[0].ID = %q, want %q", gotRules[0].ID, wantRules[0].ID)
	}
	// Verify mutation did not leak through: ToolNames[0] should still be the
	// original value "bash", not "MUTATED".
	if len(gotRules[0].Trigger.ToolNames) > 0 && gotRules[0].Trigger.ToolNames[0] != origToolName {
		t.Errorf("continuation dynamicRules[0].Trigger.ToolNames[0] = %q, want %q (deep-copy failed, mutation leaked through)",
			gotRules[0].Trigger.ToolNames[0], origToolName)
	}
}

// TestContinueRun_FiredOnceRulesPreserved verifies that the continuation
// inherits the source run's firedOnceRules and that the map is a deep copy
// (mutations to either map do not affect the other).
func TestContinueRun_FiredOnceRulesPreserved(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	run1, err := runner.StartRun(RunRequest{
		Prompt: "initial",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	// Inject a fired rule into the source run's firedOnceRules map before continuation.
	runner.mu.Lock()
	srcState := runner.runs[run1.ID]
	srcState.firedOnceRules["rule-already-fired"] = true
	runner.mu.Unlock()

	run2, err := runner.ContinueRun(run1.ID, "follow up")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	// Verify the fired rule was inherited.
	runner.mu.RLock()
	contState, ok := runner.runs[run2.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("continuation run state not found")
	}
	contFired := contState.firedOnceRules
	runner.mu.RUnlock()

	if !contFired["rule-already-fired"] {
		t.Error("continuation firedOnceRules missing inherited fired rule")
	}

	// Verify independence: adding a new entry to the continuation's map should
	// not affect the source's map.
	runner.mu.Lock()
	contState2 := runner.runs[run2.ID]
	contState2.firedOnceRules["new-cont-rule"] = true
	srcState2 := runner.runs[run1.ID]
	if srcState2.firedOnceRules["new-cont-rule"] {
		runner.mu.Unlock()
		t.Error("mutation of continuation firedOnceRules leaked into source run (maps share backing storage)")
		return
	}
	runner.mu.Unlock()
}

// TestContinueRun_ForkDepthPreserved verifies that a continuation inherits
// the source run's forkDepth, preserving subagent nesting depth.
func TestContinueRun_ForkDepthPreserved(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	const wantDepth = 2
	run1, err := runner.StartRun(RunRequest{
		Prompt:    "initial",
		ForkDepth: wantDepth,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	runner.mu.RLock()
	srcState, ok := runner.runs[run1.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("source run state not found")
	}
	srcDepth := srcState.forkDepth
	runner.mu.RUnlock()

	if srcDepth != wantDepth {
		t.Fatalf("source run forkDepth = %d, want %d", srcDepth, wantDepth)
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
	gotDepth := contState.forkDepth
	runner.mu.RUnlock()

	if gotDepth != wantDepth {
		t.Errorf("ContinueRun forkDepth = %d, want %d", gotDepth, wantDepth)
	}
}

// TestContinueRun_ChainedContinuationPreservesAllFields verifies that all
// security fields propagate correctly through 2 levels of continuation:
// run1 → run2 → run3.
func TestContinueRun_ChainedContinuationPreservesAllFields(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
			{Content: "third response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            6,
	})

	const (
		wantMaxCost = 2.50
		wantProfile = "chained-profile"
		wantDepth   = 1
	)
	wantPerms := PermissionConfig{
		Sandbox:  SandboxScopeWorkspace,
		Approval: ApprovalPolicyDestructive,
	}
	wantTools := []string{"read_file", "bash"}
	wantRules := []DynamicRule{
		{ID: "chained-rule", Content: "Be careful.", Trigger: RuleTrigger{ToolNames: []string{"bash"}}},
	}

	run1, err := runner.StartRun(RunRequest{
		Prompt:       "initial",
		MaxCostUSD:   wantMaxCost,
		Permissions:  &wantPerms,
		AllowedTools: wantTools,
		ProfileName:  wantProfile,
		DynamicRules: wantRules,
		ForkDepth:    wantDepth,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	run2, err := runner.ContinueRun(run1.ID, "follow up 1")
	if err != nil {
		t.Fatalf("ContinueRun (level 1): %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	run3, err := runner.ContinueRun(run2.ID, "follow up 2")
	if err != nil {
		t.Fatalf("ContinueRun (level 2): %v", err)
	}
	waitForStatusCont(t, runner, run3.ID, RunStatusCompleted, RunStatusFailed)

	runner.mu.RLock()
	state3, ok := runner.runs[run3.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("run3 state not found")
	}
	gotMaxCost := state3.maxCostUSD
	gotPerms := state3.permissions
	gotTools := append([]string(nil), state3.allowedTools...)
	gotProfile := state3.profileName
	gotDepth := state3.forkDepth
	gotRulesLen := len(state3.dynamicRules)
	runner.mu.RUnlock()

	if gotMaxCost != wantMaxCost {
		t.Errorf("run3 maxCostUSD = %v, want %v", gotMaxCost, wantMaxCost)
	}
	if gotPerms != wantPerms {
		t.Errorf("run3 permissions = %+v, want %+v", gotPerms, wantPerms)
	}
	if !reflect.DeepEqual(gotTools, wantTools) {
		t.Errorf("run3 allowedTools = %v, want %v", gotTools, wantTools)
	}
	if gotProfile != wantProfile {
		t.Errorf("run3 profileName = %q, want %q", gotProfile, wantProfile)
	}
	if gotDepth != wantDepth {
		t.Errorf("run3 forkDepth = %d, want %d", gotDepth, wantDepth)
	}
	if gotRulesLen != len(wantRules) {
		t.Errorf("run3 dynamicRules length = %d, want %d", gotRulesLen, len(wantRules))
	}
}

// TestContinueRun_AllSecurityFieldsEnumerated is a meta-test that checks the
// named set of security-critical runState fields against a known list. If a
// new security field is added to runState but not propagated in ContinueRun,
// the developer who adds the new test entry here will notice the gap.
//
// This test does not use reflection to enumerate all struct fields — that
// approach produces noise for non-security operational fields (e.g. terminated,
// compactMu, steeringCh). Instead it checks a curated allowlist by name using
// reflect.TypeOf so that field renames are caught at compile time via the test.
func TestContinueRun_AllSecurityFieldsEnumerated(t *testing.T) {
	t.Parallel()

	// securityFields is the canonical list of runState fields that must be
	// propagated by ContinueRun. Each entry is the exact Go field name.
	// When adding a new security-critical field to runState, add it here AND
	// ensure ContinueRun copies it in the snapshot block.
	securityFields := []string{
		"maxCostUSD",
		"allowedTools",
		"permissions",
		"resolvedRoleModels",
		"profileName",
		"dynamicRules",
		"firedOnceRules",
		"forkDepth",
	}

	// Verify each field name exists in runState using reflection.
	// This catches renames: if a field is renamed, reflect.TypeOf will report
	// it as not found and the test will fail, prompting an update here.
	rsType := reflect.TypeOf(runState{})
	for _, fieldName := range securityFields {
		if _, found := rsType.FieldByName(fieldName); !found {
			t.Errorf("security field %q not found in runState — was it renamed or removed?", fieldName)
		}
	}

	// Now run a continuation and verify each security field is non-zero
	// (or matches the source) by setting distinct non-zero values and checking
	// they appear in the continuation.
	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	const wantMaxCost = 9.99
	const wantProfile = "meta-profile"
	const wantDepth = 3
	wantPerms := PermissionConfig{
		Sandbox:  SandboxScopeWorkspace,
		Approval: ApprovalPolicyDestructive,
	}
	wantTools := []string{"meta-tool"}
	wantRules := []DynamicRule{
		{ID: "meta-rule", Content: "meta content", Trigger: RuleTrigger{ToolNames: []string{"meta-tool"}}},
	}

	run1, err := runner.StartRun(RunRequest{
		Prompt:       "initial",
		MaxCostUSD:   wantMaxCost,
		Permissions:  &wantPerms,
		AllowedTools: wantTools,
		ProfileName:  wantProfile,
		DynamicRules: wantRules,
		ForkDepth:    wantDepth,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	// Seed firedOnceRules on the source before continuing.
	runner.mu.Lock()
	runner.runs[run1.ID].firedOnceRules["meta-rule"] = true
	runner.mu.Unlock()

	run2, err := runner.ContinueRun(run1.ID, "follow up")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	runner.mu.RLock()
	cs, ok := runner.runs[run2.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("continuation run state not found")
	}
	gotMaxCost := cs.maxCostUSD
	gotPerms := cs.permissions
	gotTools := cs.allowedTools
	gotProfile := cs.profileName
	gotDepth := cs.forkDepth
	gotRules := cs.dynamicRules
	gotFired := cs.firedOnceRules
	runner.mu.RUnlock()

	checks := []struct {
		field string
		ok    bool
		msg   string
	}{
		{"maxCostUSD", gotMaxCost == wantMaxCost, fmt.Sprintf("got %v, want %v", gotMaxCost, wantMaxCost)},
		{"permissions", gotPerms == wantPerms, fmt.Sprintf("got %+v, want %+v", gotPerms, wantPerms)},
		{"allowedTools", reflect.DeepEqual(gotTools, wantTools), fmt.Sprintf("got %v, want %v", gotTools, wantTools)},
		{"profileName", gotProfile == wantProfile, fmt.Sprintf("got %q, want %q", gotProfile, wantProfile)},
		{"forkDepth", gotDepth == wantDepth, fmt.Sprintf("got %d, want %d", gotDepth, wantDepth)},
		{"dynamicRules length", len(gotRules) == len(wantRules), fmt.Sprintf("got %d, want %d", len(gotRules), len(wantRules))},
		{"firedOnceRules[meta-rule]", gotFired["meta-rule"], "meta-rule not present in continuation firedOnceRules"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("security field %s not correctly propagated: %s", c.field, c.msg)
		}
	}
}
