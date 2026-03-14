package harness

import (
	"context"
	"testing"

	htools "go-agent-harness/internal/harness/tools"
)

// TestRunForkedSkill_InheritsParentProfileName verifies that RunForkedSkill
// propagates the parent run's ProfileName to the child RunRequest.
// This ensures profile MCP servers (loaded by StartRun from the profile file)
// are available in forked sub-runs.
func TestRunForkedSkill_InheritsParentProfileName(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{
		turns: []CompletionResult{
			{Content: "parent done"},
			{Content: "child done"},
		},
	}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	// Start a parent run with ProfileName set.
	parentRun, err := runner.StartRun(RunRequest{
		Prompt:      "parent task",
		ProfileName: "my-profile",
	})
	if err != nil {
		t.Fatalf("start parent run: %v", err)
	}

	_, err = collectRunEvents(t, runner, parentRun.ID)
	if err != nil {
		t.Fatalf("collect parent run events: %v", err)
	}

	// Call RunForkedSkill with the parent run ID in context (simulating being
	// called from within the parent run's tool execution).
	parentMeta := htools.RunMetadata{RunID: parentRun.ID, TenantID: "default"}
	ctx := context.WithValue(context.Background(), htools.ContextKeyRunMetadata, parentMeta)

	forkCfg := htools.ForkConfig{
		Prompt:    "child task",
		SkillName: "test-skill",
	}

	_, err = runner.RunForkedSkill(ctx, forkCfg)
	if err != nil {
		t.Fatalf("RunForkedSkill: %v", err)
	}

	// Find the forked run state (any run that is not the parent).
	runner.mu.RLock()
	var forkedState *runState
	for id, state := range runner.runs {
		if id != parentRun.ID {
			forkedState = state
		}
	}
	runner.mu.RUnlock()

	if forkedState == nil {
		t.Fatal("no forked run state found")
	}

	if forkedState.profileName != "my-profile" {
		t.Errorf("expected forked run profileName %q, got %q", "my-profile", forkedState.profileName)
	}
}

// TestRunForkedSkill_NoParentProfilePropagatesEmpty verifies that when the
// parent run has no ProfileName set, the forked run also gets an empty ProfileName.
func TestRunForkedSkill_NoParentProfilePropagatesEmpty(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{
		turns: []CompletionResult{
			{Content: "parent done"},
			{Content: "child done"},
		},
	}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	// Start a parent run WITHOUT a ProfileName.
	parentRun, err := runner.StartRun(RunRequest{
		Prompt: "parent task",
		// ProfileName not set
	})
	if err != nil {
		t.Fatalf("start parent run: %v", err)
	}

	_, err = collectRunEvents(t, runner, parentRun.ID)
	if err != nil {
		t.Fatalf("collect parent run events: %v", err)
	}

	parentMeta := htools.RunMetadata{RunID: parentRun.ID, TenantID: "default"}
	ctx := context.WithValue(context.Background(), htools.ContextKeyRunMetadata, parentMeta)

	forkCfg := htools.ForkConfig{
		Prompt:    "child task",
		SkillName: "test-skill",
	}

	_, err = runner.RunForkedSkill(ctx, forkCfg)
	if err != nil {
		t.Fatalf("RunForkedSkill: %v", err)
	}

	runner.mu.RLock()
	var forkedState *runState
	for id, state := range runner.runs {
		if id != parentRun.ID {
			forkedState = state
		}
	}
	runner.mu.RUnlock()

	if forkedState == nil {
		t.Fatal("no forked run state found")
	}

	if forkedState.profileName != "" {
		t.Errorf("expected forked run profileName to be empty, got %q", forkedState.profileName)
	}
}

// TestRunState_StoresProfileName verifies that profileName is stored in runState
// when StartRun is called with a ProfileName in the RunRequest.
func TestRunState_StoresProfileName(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{
		turns: []CompletionResult{{Content: "done"}},
	}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     1,
	})

	run, err := runner.StartRun(RunRequest{
		Prompt:      "task",
		ProfileName: "test-profile",
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect run events: %v", err)
	}

	runner.mu.RLock()
	state, ok := runner.runs[run.ID]
	runner.mu.RUnlock()

	if !ok {
		t.Fatal("run state not found")
	}

	if state.profileName != "test-profile" {
		t.Errorf("expected profileName %q in runState, got %q", "test-profile", state.profileName)
	}
}
