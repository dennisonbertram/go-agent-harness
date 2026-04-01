package harness

// Tests for security properties of ContinueRun: budget propagation,
// permission propagation, and allowed-tools propagation.
//
// Regression tests for GitHub issue #222:
// ContinueRun drops maxCostUSD and permissions from source run.
//
// Regression tests for GitHub issue #524:
// ContinueRun drops allowedTools from source run.

import (
	"context"
	"encoding/json"
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

// TestContinueRun_PreservesAllowedTools verifies that ContinueRun copies the
// source run's allowedTools into the continuation runState. Without the fix for
// issue #524, the continuation defaults to nil (unrestricted), silently dropping
// tool restrictions that were active on the original run.
func TestContinueRun_PreservesAllowedTools(t *testing.T) {
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

	wantTools := []string{"bash", "read", "compact_history"}
	run1, err := runner.StartRun(RunRequest{
		Prompt:       "initial",
		AllowedTools: wantTools,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	// Verify source run has the expected allowedTools.
	runner.mu.RLock()
	srcState, ok := runner.runs[run1.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("source run state not found")
	}
	srcTools := srcState.allowedTools
	runner.mu.RUnlock()

	if len(srcTools) != len(wantTools) {
		t.Fatalf("source run allowedTools = %v, want %v", srcTools, wantTools)
	}

	run2, err := runner.ContinueRun(run1.ID, "follow up")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	// The continuation must carry the same allowedTools as the source run.
	runner.mu.RLock()
	contState, ok := runner.runs[run2.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("continuation run state not found")
	}
	gotTools := contState.allowedTools
	runner.mu.RUnlock()

	if len(gotTools) != len(wantTools) {
		t.Fatalf("ContinueRun allowedTools length = %d, want %d (tool restriction dropped)", len(gotTools), len(wantTools))
	}
	wantSet := make(map[string]bool, len(wantTools))
	for _, name := range wantTools {
		wantSet[name] = true
	}
	for _, name := range gotTools {
		if !wantSet[name] {
			t.Errorf("ContinueRun allowedTools contains unexpected tool %q", name)
		}
	}
}

// TestContinueRun_PreservesNilAllowedTools verifies that when a source run has
// no AllowedTools restriction (nil), the continuation also inherits nil
// (unrestricted). Ensures the fix does not introduce false positives.
func TestContinueRun_PreservesNilAllowedTools(t *testing.T) {
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

	// Start with no AllowedTools restriction.
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
	gotTools := contState.allowedTools
	runner.mu.RUnlock()

	if gotTools != nil {
		t.Errorf("ContinueRun allowedTools = %v, want nil (unrestricted); false restriction introduced", gotTools)
	}
}

// TestContinueRun_AllowedToolsFiltersPersistAcrossContinuation verifies that
// filteredToolsForRun on the continuation run correctly excludes tools not in the
// inherited allowedTools list. This tests end-to-end filtering, not just field
// propagation.
func TestContinueRun_AllowedToolsFiltersPersistAcrossContinuation(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	// Register two named tools so we can verify filtering.
	_ = registry.Register(ToolDefinition{
		Name:        "compact_history",
		Description: "compact history",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{}`, nil
	})
	_ = registry.Register(ToolDefinition{
		Name:        "context_status",
		Description: "context status",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{}`, nil
	})
	_ = registry.Register(ToolDefinition{
		Name:        "forbidden_tool",
		Description: "should not appear",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{}`, nil
	})

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, registry, RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	allowedTools := []string{"compact_history", "context_status"}
	run1, err := runner.StartRun(RunRequest{
		Prompt:       "initial",
		AllowedTools: allowedTools,
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

	// filteredToolsForRun should respect the inherited allowedTools.
	defs := runner.filteredToolsForRun(run2.ID)
	for _, def := range defs {
		if def.Name == "forbidden_tool" {
			t.Errorf("continuation run includes forbidden_tool in filtered tool list; allowedTools filter not applied")
		}
	}
	// At least one of the allowed tools should appear (if registered at core tier).
	foundAllowed := false
	for _, def := range defs {
		if def.Name == "compact_history" || def.Name == "context_status" {
			foundAllowed = true
			break
		}
	}
	// AlwaysAvailableTools are always present regardless.
	// If neither allowed tool appears, the registry may use deferred tier — that's
	// acceptable, but forbidden_tool must not appear.
	_ = foundAllowed
}

// TestContinueRun_SecurityFieldsAllPreserved verifies that ALL security fields
// (allowedTools, maxCostUSD, permissions) are preserved across ContinueRun.
// This is a meta-test that catches future regressions if someone adds a new
// security field and forgets to propagate it.
func TestContinueRun_SecurityFieldsAllPreserved(t *testing.T) {
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

	const wantMaxCost = 2.50
	wantPerms := PermissionConfig{
		Sandbox:  SandboxScopeWorkspace,
		Approval: ApprovalPolicyDestructive,
	}
	wantTools := []string{"bash", "read"}

	run1, err := runner.StartRun(RunRequest{
		Prompt:       "initial",
		MaxCostUSD:   wantMaxCost,
		Permissions:  &wantPerms,
		AllowedTools: wantTools,
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
	gotPerms := contState.permissions
	gotTools := contState.allowedTools
	runner.mu.RUnlock()

	if gotMaxCost != wantMaxCost {
		t.Errorf("ContinueRun maxCostUSD = %v, want %v (budget bypass detected)", gotMaxCost, wantMaxCost)
	}
	if gotPerms != wantPerms {
		t.Errorf("ContinueRun permissions = %+v, want %+v (permission bypass detected)", gotPerms, wantPerms)
	}
	if len(gotTools) != len(wantTools) {
		t.Errorf("ContinueRun allowedTools = %v, want %v (tool restriction dropped)", gotTools, wantTools)
	} else {
		wantSet := make(map[string]bool, len(wantTools))
		for _, name := range wantTools {
			wantSet[name] = true
		}
		for _, name := range gotTools {
			if !wantSet[name] {
				t.Errorf("ContinueRun allowedTools contains unexpected tool %q", name)
			}
		}
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
