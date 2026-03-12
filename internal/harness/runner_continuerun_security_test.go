package harness

// Tests for security properties of ContinueRun: budget propagation and
// permission propagation.
//
// Regression tests for GitHub issue #222:
// ContinueRun drops maxCostUSD and permissions from source run.

import (
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
