package harness

// Tests for security properties of ContinueRun: budget propagation and
// permission propagation.
//
// Regression tests for GitHub issue #222:
// ContinueRun drops maxCostUSD and permissions from source run.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func toolMessagePayload(t *testing.T, runner *Runner, runID, toolName string) map[string]any {
	t.Helper()

	msgs := runner.GetRunMessages(runID)
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if msg.Role != "tool" || msg.Name != toolName {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(msg.Content), &payload); err != nil {
			t.Fatalf("unmarshal tool payload for %s: %v", toolName, err)
		}
		return payload
	}
	t.Fatalf("tool message for %q not found in run %s", toolName, runID)
	return nil
}

// TestCopyStringSlice_NilPreserved verifies that copyStringSlice returns nil
// for a nil input (nil means "no restriction" in allowedTools semantics).
func TestCopyStringSlice_NilPreserved(t *testing.T) {
	t.Parallel()
	got := copyStringSlice(nil)
	if got != nil {
		t.Errorf("copyStringSlice(nil) = %v, want nil", got)
	}
}

// TestCopyStringSlice_EmptyNonNilPreserved verifies that copyStringSlice
// returns a non-nil empty slice for a non-nil empty input.
func TestCopyStringSlice_EmptyNonNilPreserved(t *testing.T) {
	t.Parallel()
	src := []string{}
	got := copyStringSlice(src)
	if got == nil {
		t.Error("copyStringSlice([]string{}) = nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("copyStringSlice([]string{}) len = %d, want 0", len(got))
	}
}

// TestCopyStringSlice_CopiesElements verifies that elements are copied and
// that mutations to the original do not affect the copy.
func TestCopyStringSlice_CopiesElements(t *testing.T) {
	t.Parallel()
	src := []string{"read", "write", "bash"}
	got := copyStringSlice(src)
	if len(got) != len(src) {
		t.Fatalf("copyStringSlice len = %d, want %d", len(got), len(src))
	}
	for i, v := range src {
		if got[i] != v {
			t.Errorf("got[%d] = %q, want %q", i, got[i], v)
		}
	}
	// Mutate original — copy must be independent.
	src[0] = "mutated"
	if got[0] == "mutated" {
		t.Error("copyStringSlice copy shares backing array with source")
	}
}

// TestContinueRunPropagatesAllowedTools verifies that ContinueRun copies the
// source run's allowedTools into the continuation runState. Without this fix,
// the continuation defaults to nil (unrestricted), bypassing any per-run tool
// filter set on the original run.
func TestContinueRunPropagatesAllowedTools(t *testing.T) {
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

	wantAllowedTools := []string{"read", "bash"}
	run1, err := runner.StartRun(RunRequest{
		Prompt:       "initial",
		AllowedTools: wantAllowedTools,
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
	srcAllowed := srcState.allowedTools
	runner.mu.RUnlock()

	if len(srcAllowed) != len(wantAllowedTools) {
		t.Fatalf("source run allowedTools = %v, want %v", srcAllowed, wantAllowedTools)
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
	gotAllowed := contState.allowedTools
	runner.mu.RUnlock()

	if len(gotAllowed) != len(wantAllowedTools) {
		t.Fatalf("ContinueRun allowedTools len = %d, want %d (tool filter bypass detected)", len(gotAllowed), len(wantAllowedTools))
	}
	for i, want := range wantAllowedTools {
		if gotAllowed[i] != want {
			t.Errorf("ContinueRun allowedTools[%d] = %q, want %q", i, gotAllowed[i], want)
		}
	}
}

func TestContinueRunWithOptions_OverridesAllowedToolsAndPermissions(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "read",
		Description: "read a file",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{}`, nil
	}); err != nil {
		t.Fatalf("register read: %v", err)
	}
	if err := registry.Register(ToolDefinition{
		Name:        "bash",
		Description: "run a shell command",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{}`, nil
	}); err != nil {
		t.Fatalf("register bash: %v", err)
	}

	prov := &capturingContinuationProvider{
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

	sourcePerms := PermissionConfig{
		Sandbox:  SandboxScopeWorkspace,
		Approval: ApprovalPolicyDestructive,
	}
	run1, err := runner.StartRun(RunRequest{
		Prompt:       "initial",
		AllowedTools: []string{"read", "bash"},
		Permissions:  &sourcePerms,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	overrideTools := []string{"read"}
	overridePerms := PermissionConfig{
		Sandbox:  SandboxScopeLocal,
		Approval: ApprovalPolicyAll,
	}
	run2, err := runner.ContinueRunWithOptions(run1.ID, ContinueRunRequest{
		Prompt:       "follow up",
		AllowedTools: &overrideTools,
		Permissions:  &overridePerms,
	})
	if err != nil {
		t.Fatalf("ContinueRunWithOptions: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	runner.mu.RLock()
	contState, ok := runner.runs[run2.ID]
	if !ok {
		runner.mu.RUnlock()
		t.Fatal("continuation run state not found")
	}
	gotAllowed := copyStringSlice(contState.allowedTools)
	gotPerms := contState.permissions
	runner.mu.RUnlock()

	if !stringSlicesEqual(gotAllowed, overrideTools) {
		t.Fatalf("continuation allowedTools = %v, want %v", gotAllowed, overrideTools)
	}
	if gotPerms != overridePerms {
		t.Fatalf("continuation permissions = %+v, want %+v", gotPerms, overridePerms)
	}

	reqs := prov.captured()
	if len(reqs) < 2 {
		t.Fatalf("expected at least 2 provider calls, got %d", len(reqs))
	}
	secondReq := reqs[1]
	toolNames := make(map[string]bool, len(secondReq.Tools))
	for _, tool := range secondReq.Tools {
		toolNames[tool.Name] = true
	}
	if toolNames["bash"] {
		t.Fatalf("bash should not be offered after continuation override, got tools: %+v", secondReq.Tools)
	}
	if !toolNames["read"] {
		t.Fatalf("read should remain available after continuation override, got tools: %+v", secondReq.Tools)
	}

	foundNotice := false
	for _, msg := range secondReq.Messages {
		if msg.Role == "system" && msg.IsMeta && strings.Contains(msg.Content, "Runtime policy changed for this continuation") {
			foundNotice = true
			break
		}
	}
	if !foundNotice {
		t.Fatal("expected continuation policy notice in provider messages when controls change")
	}
}

func TestStartRunPermissionsSandboxOverridesRegistryDefault(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	registry := NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		SandboxScope: SandboxScopeWorkspace,
	})

	prov := &continuationProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call_1",
					Name:      "bash",
					Arguments: `{"command":"cat /etc/hosts"}`,
				}},
			},
			{Content: "done"},
		},
	}

	runner := NewRunner(prov, registry, RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})

	perms := PermissionConfig{
		Sandbox:  SandboxScopeUnrestricted,
		Approval: ApprovalPolicyNone,
	}
	run, err := runner.StartRun(RunRequest{
		Prompt:      "read host file",
		Permissions: &perms,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	payload := toolMessagePayload(t, runner, run.ID, "bash")
	if errMsg, ok := payload["error"].(string); ok {
		t.Fatalf("expected unrestricted run sandbox to allow bash, got error %q", errMsg)
	}
	if exitCode, ok := payload["exit_code"].(float64); !ok || int(exitCode) != 0 {
		t.Fatalf("expected bash exit_code 0, got %v", payload["exit_code"])
	}
}

func TestContinueRunWithOptions_UpdatesSandboxBoundaryAtExecutionTime(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	registry := NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		SandboxScope: SandboxScopeWorkspace,
	})

	prov := &continuationProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call_1",
					Name:      "bash",
					Arguments: `{"command":"cat /etc/hosts"}`,
				}},
			},
			{Content: "first done"},
			{
				ToolCalls: []ToolCall{{
					ID:        "call_2",
					Name:      "bash",
					Arguments: `{"command":"cat /etc/hosts"}`,
				}},
			},
			{Content: "second done"},
		},
	}

	runner := NewRunner(prov, registry, RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})

	sourcePerms := PermissionConfig{
		Sandbox:  SandboxScopeWorkspace,
		Approval: ApprovalPolicyNone,
	}
	run1, err := runner.StartRun(RunRequest{
		Prompt:      "first run",
		Permissions: &sourcePerms,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	payload1 := toolMessagePayload(t, runner, run1.ID, "bash")
	errMsg1, ok := payload1["error"].(string)
	if !ok || !strings.Contains(errMsg1, "sandbox violation") {
		t.Fatalf("expected workspace sandbox violation on source run, got payload %+v", payload1)
	}

	overridePerms := PermissionConfig{
		Sandbox:  SandboxScopeUnrestricted,
		Approval: ApprovalPolicyNone,
	}
	run2, err := runner.ContinueRunWithOptions(run1.ID, ContinueRunRequest{
		Prompt:      "same conversation, broader sandbox",
		Permissions: &overridePerms,
	})
	if err != nil {
		t.Fatalf("ContinueRunWithOptions: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	payload2 := toolMessagePayload(t, runner, run2.ID, "bash")
	if errMsg2, ok := payload2["error"].(string); ok {
		t.Fatalf("expected continuation sandbox override to allow bash, got error %q", errMsg2)
	}
	if exitCode, ok := payload2["exit_code"].(float64); !ok || int(exitCode) != 0 {
		t.Fatalf("expected continuation bash exit_code 0, got %v", payload2["exit_code"])
	}
}

// TestContinueRunNilAllowedToolsPreserved verifies that when a source run has
// no per-run tool filter (allowedTools == nil, meaning unrestricted), the
// continuation also inherits nil (unrestricted) — not an empty slice.
func TestContinueRunNilAllowedToolsPreserved(t *testing.T) {
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

	// Start with no AllowedTools (nil = unrestricted).
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
	gotAllowed := contState.allowedTools
	runner.mu.RUnlock()

	if gotAllowed != nil {
		t.Errorf("ContinueRun allowedTools = %v, want nil (unrestricted)", gotAllowed)
	}
}

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
