package harness

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// staticProviderWS is a minimal Provider for workspace selection tests.
type staticProviderWS struct {
	result CompletionResult
}

func (s *staticProviderWS) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	return s.result, nil
}

// alwaysFailProviderWS is a Provider that always returns an error.
type alwaysFailProviderWS struct{}

func (p *alwaysFailProviderWS) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	return CompletionResult{}, errors.New("provider always fails")
}

// drainRunEventsWS subscribes to a run and collects all events until a
// terminal event is received or the deadline elapses.
func drainRunEventsWS(t *testing.T, runner *Runner, runID string) []Event {
	t.Helper()

	history, stream, cancel, err := runner.Subscribe(runID)
	if err != nil {
		t.Fatalf("Subscribe(%q): %v", runID, err)
	}
	defer cancel()

	var events []Event
	events = append(events, history...)

	// Check if already terminated in history.
	for _, ev := range history {
		if IsTerminalEvent(ev.Type) {
			return events
		}
	}

	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-stream:
			if !ok {
				return events
			}
			events = append(events, ev)
			if IsTerminalEvent(ev.Type) {
				return events
			}
		case <-deadline:
			t.Logf("drainRunEventsWS timed out; collected %d events", len(events))
			return events
		}
	}
}

// initGitRepoForWS creates a temp directory with a minimal git repo.
func initGitRepoForWS(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runGitWS := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	runGitWS("init")
	runGitWS("config", "user.email", "test@test.com")
	runGitWS("config", "user.name", "Test")

	// Create a README and initial commit so HEAD exists (required for worktrees).
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("test"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGitWS("add", ".")
	runGitWS("commit", "-m", "init")

	return dir
}

// TestRunRequest_WorkspaceType_Default verifies that a RunRequest with no
// workspace_type field is accepted and runs without error (local default).
func TestRunRequest_WorkspaceType_Default(t *testing.T) {
	t.Parallel()

	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{MaxSteps: 1},
	)

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun with no workspace_type failed: %v", err)
	}
	if run.ID == "" {
		t.Fatal("expected non-empty run ID")
	}
}

// TestRunRequest_WorkspaceType_Local verifies that workspace_type="local" is
// accepted and behaves identically to the default (no workspace isolation).
func TestRunRequest_WorkspaceType_Local(t *testing.T) {
	t.Parallel()

	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{MaxSteps: 1},
	)

	run, err := runner.StartRun(RunRequest{
		Prompt:        "hello",
		WorkspaceType: "local",
	})
	if err != nil {
		t.Fatalf("StartRun with workspace_type=local failed: %v", err)
	}
	if run.ID == "" {
		t.Fatal("expected non-empty run ID")
	}

	events := drainRunEventsWS(t, runner, run.ID)
	var gotCompleted bool
	for _, ev := range events {
		if ev.Type == EventRunCompleted {
			gotCompleted = true
		}
	}
	if !gotCompleted {
		t.Error("expected run.completed event for local workspace run")
	}
}

// TestRunRequest_WorkspaceType_Unknown verifies that an unknown workspace_type
// is rejected immediately by StartRun with a descriptive validation error.
func TestRunRequest_WorkspaceType_Unknown(t *testing.T) {
	t.Parallel()

	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{MaxSteps: 1},
	)

	_, err := runner.StartRun(RunRequest{
		Prompt:        "hello",
		WorkspaceType: "bogus-type-xyz",
	})
	if err == nil {
		t.Fatal("expected error for unknown workspace_type, got nil")
	}
	if !strings.Contains(err.Error(), "bogus-type-xyz") {
		t.Errorf("error should mention the invalid workspace_type, got: %v", err)
	}
}

// TestRunRequest_WorkspaceType_Worktree verifies that workspace_type="worktree"
// provisions a real git worktree, emits workspace events, and cleans up on completion.
func TestRunRequest_WorkspaceType_Worktree(t *testing.T) {
	t.Parallel()

	repoDir := initGitRepoForWS(t)
	worktreeRootDir := t.TempDir()

	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{
			MaxSteps: 1,
			WorkspaceBaseOptions: WorkspaceProvisionOptions{
				RepoPath:        repoDir,
				WorktreeRootDir: worktreeRootDir,
			},
		},
	)

	run, err := runner.StartRun(RunRequest{
		Prompt:        "hello",
		WorkspaceType: "worktree",
	})
	if err != nil {
		t.Fatalf("StartRun with workspace_type=worktree failed: %v", err)
	}

	events := drainRunEventsWS(t, runner, run.ID)

	var provisionedEv *Event
	var destroyedEv *Event
	for i := range events {
		switch events[i].Type {
		case EventWorkspaceProvisioned:
			provisionedEv = &events[i]
		case EventWorkspaceDestroyed:
			destroyedEv = &events[i]
		}
	}

	if provisionedEv == nil {
		t.Error("expected workspace.provisioned event")
	} else {
		if provisionedEv.Payload["workspace_type"] != "worktree" {
			t.Errorf("workspace.provisioned workspace_type = %v, want worktree",
				provisionedEv.Payload["workspace_type"])
		}
		wsPath, _ := provisionedEv.Payload["workspace_path"].(string)
		if wsPath == "" {
			t.Error("workspace.provisioned should have non-empty workspace_path")
		}
	}

	if destroyedEv == nil {
		t.Error("expected workspace.destroyed event after run completion")
	}
}

// TestRunRequest_WorkspaceType_WorktreeMissingRepoPath verifies that a run
// with workspace_type="worktree" but no repo path configured fails the run.
func TestRunRequest_WorkspaceType_WorktreeMissingRepoPath(t *testing.T) {
	t.Parallel()

	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{
			MaxSteps: 1,
			// WorkspaceBaseOptions is zero — no RepoPath for worktree.
		},
	)

	run, err := runner.StartRun(RunRequest{
		Prompt:        "hello",
		WorkspaceType: "worktree",
	})
	// StartRun should succeed (validates type name only, not provisioning).
	if err != nil {
		t.Fatalf("StartRun unexpectedly failed: %v", err)
	}

	events := drainRunEventsWS(t, runner, run.ID)

	var gotFailed bool
	for _, ev := range events {
		if ev.Type == EventRunFailed {
			gotFailed = true
		}
	}
	if !gotFailed {
		t.Error("expected run.failed when worktree provisioning lacks a repo path")
	}
}

// TestRunRequest_WorkspaceType_WorkspaceDestroyed_OnFailure verifies that the
// workspace is cleaned up even when the run itself fails.
func TestRunRequest_WorkspaceType_WorkspaceDestroyed_OnFailure(t *testing.T) {
	t.Parallel()

	repoDir := initGitRepoForWS(t)
	worktreeRootDir := t.TempDir()

	runner := NewRunner(
		&alwaysFailProviderWS{},
		NewRegistry(),
		RunnerConfig{
			MaxSteps: 1,
			WorkspaceBaseOptions: WorkspaceProvisionOptions{
				RepoPath:        repoDir,
				WorktreeRootDir: worktreeRootDir,
			},
		},
	)

	run, err := runner.StartRun(RunRequest{
		Prompt:        "hello",
		WorkspaceType: "worktree",
	})
	if err != nil {
		t.Fatalf("StartRun failed: %v", err)
	}

	events := drainRunEventsWS(t, runner, run.ID)

	var gotProvisioned, gotDestroyed bool
	for _, ev := range events {
		switch ev.Type {
		case EventWorkspaceProvisioned:
			gotProvisioned = true
		case EventWorkspaceDestroyed:
			gotDestroyed = true
		}
	}

	if !gotProvisioned {
		t.Error("expected workspace.provisioned event")
	}
	if !gotDestroyed {
		t.Error("expected workspace.destroyed event even when run fails")
	}
}

// TestRunRequest_WorkspaceType_ValidTypes verifies that the known-valid
// workspace types are all accepted without validation error.
func TestRunRequest_WorkspaceType_ValidTypes(t *testing.T) {
	t.Parallel()

	validTypes := []string{"", "local", "worktree"}
	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{MaxSteps: 1},
	)

	for _, wsType := range validTypes {
		wsType := wsType
		t.Run("type="+wsType, func(t *testing.T) {
			t.Parallel()
			_, err := runner.StartRun(RunRequest{
				Prompt:        "hello",
				WorkspaceType: wsType,
			})
			if err != nil {
				t.Errorf("StartRun with workspace_type=%q rejected unexpectedly: %v", wsType, err)
			}
		})
	}
}

// TestRunRequest_WorkspaceType_JSONField verifies the JSON field name is
// "workspace_type" with omitempty semantics.
func TestRunRequest_WorkspaceType_JSONField(t *testing.T) {
	t.Parallel()

	// Unmarshal with workspace_type present.
	raw := `{"prompt":"hi","workspace_type":"local"}`
	var req RunRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if req.WorkspaceType != "local" {
		t.Errorf("WorkspaceType = %q, want local", req.WorkspaceType)
	}
	if req.Prompt != "hi" {
		t.Errorf("Prompt = %q, want hi", req.Prompt)
	}

	// Marshal omits workspace_type when empty.
	empty := RunRequest{Prompt: "hi"}
	b, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(b), "workspace_type") {
		t.Errorf("empty WorkspaceType should be omitted from JSON, got: %s", string(b))
	}
}
