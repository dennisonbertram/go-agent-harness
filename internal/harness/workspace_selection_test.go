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

	"go-agent-harness/internal/profiles"
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

	// "container" and "vm" pass validation (type name is known) but
	// provisioning will fail at execute() time for lack of orchestrator config.
	// StartRun itself must not reject them.
	validTypes := []string{"", "local", "worktree", "container", "vm"}
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

// ---- resolveWorkspaceType unit tests (issue #414) ----

// TestResolveWorkspaceType_ExplicitOverride verifies that a non-empty
// RunRequest.WorkspaceType always wins, regardless of what the profile says.
func TestResolveWorkspaceType_ExplicitOverride(t *testing.T) {
	t.Parallel()

	profile := &profiles.Profile{IsolationMode: "container"}
	got := resolveWorkspaceType("worktree", profile)
	if got != "worktree" {
		t.Errorf("resolveWorkspaceType(\"worktree\", container-profile) = %q, want \"worktree\"", got)
	}
}

// TestResolveWorkspaceType_ProfileFallback_Worktree verifies that when
// RunRequest.WorkspaceType is empty the profile's IsolationMode="worktree"
// is used.
func TestResolveWorkspaceType_ProfileFallback_Worktree(t *testing.T) {
	t.Parallel()

	profile := &profiles.Profile{IsolationMode: "worktree"}
	got := resolveWorkspaceType("", profile)
	if got != "worktree" {
		t.Errorf("resolveWorkspaceType(\"\", worktree-profile) = %q, want \"worktree\"", got)
	}
}

// TestResolveWorkspaceType_ProfileFallback_Container verifies that
// IsolationMode="container" is propagated when WorkspaceType is empty.
func TestResolveWorkspaceType_ProfileFallback_Container(t *testing.T) {
	t.Parallel()

	profile := &profiles.Profile{IsolationMode: "container"}
	got := resolveWorkspaceType("", profile)
	if got != "container" {
		t.Errorf("resolveWorkspaceType(\"\", container-profile) = %q, want \"container\"", got)
	}
}

// TestResolveWorkspaceType_ProfileFallback_VM verifies that
// IsolationMode="vm" is propagated when WorkspaceType is empty.
func TestResolveWorkspaceType_ProfileFallback_VM(t *testing.T) {
	t.Parallel()

	profile := &profiles.Profile{IsolationMode: "vm"}
	got := resolveWorkspaceType("", profile)
	if got != "vm" {
		t.Errorf("resolveWorkspaceType(\"\", vm-profile) = %q, want \"vm\"", got)
	}
}

// TestResolveWorkspaceType_ProfileNone verifies that IsolationMode="none"
// results in "" (no provisioning), since "none" is an explicit opt-out.
func TestResolveWorkspaceType_ProfileNone(t *testing.T) {
	t.Parallel()

	profile := &profiles.Profile{IsolationMode: "none"}
	got := resolveWorkspaceType("", profile)
	if got != "" {
		t.Errorf("resolveWorkspaceType(\"\", none-profile) = %q, want \"\"", got)
	}
}

// TestResolveWorkspaceType_NoProfile verifies that when both WorkspaceType and
// profile are absent the result is "" (no provisioning).
func TestResolveWorkspaceType_NoProfile(t *testing.T) {
	t.Parallel()

	got := resolveWorkspaceType("", nil)
	if got != "" {
		t.Errorf("resolveWorkspaceType(\"\", nil) = %q, want \"\"", got)
	}
}

// TestResolveWorkspaceType_EmptyIsolationMode verifies that a profile with an
// empty IsolationMode field falls back to "" (no provisioning).
func TestResolveWorkspaceType_EmptyIsolationMode(t *testing.T) {
	t.Parallel()

	profile := &profiles.Profile{IsolationMode: ""}
	got := resolveWorkspaceType("", profile)
	if got != "" {
		t.Errorf("resolveWorkspaceType(\"\", empty-isolation-profile) = %q, want \"\"", got)
	}
}

// TestResolveWorkspaceType_ExplicitOverride_VM verifies that an explicit
// WorkspaceType="vm" overrides a profile that specifies "worktree".
func TestResolveWorkspaceType_ExplicitOverride_VM(t *testing.T) {
	t.Parallel()

	profile := &profiles.Profile{IsolationMode: "worktree"}
	got := resolveWorkspaceType("vm", profile)
	if got != "vm" {
		t.Errorf("resolveWorkspaceType(\"vm\", worktree-profile) = %q, want \"vm\"", got)
	}
}

// TestValidateWorkspaceType_Container verifies that "container" is now accepted.
func TestValidateWorkspaceType_Container(t *testing.T) {
	t.Parallel()

	if err := validateWorkspaceType("container"); err != nil {
		t.Errorf("validateWorkspaceType(\"container\") = %v, want nil", err)
	}
}

// TestValidateWorkspaceType_VM verifies that "vm" is now accepted.
func TestValidateWorkspaceType_VM(t *testing.T) {
	t.Parallel()

	if err := validateWorkspaceType("vm"); err != nil {
		t.Errorf("validateWorkspaceType(\"vm\") = %v, want nil", err)
	}
}

// TestValidateWorkspaceType_ErrorMessage verifies that the error message for
// an unknown type lists all four valid options.
func TestValidateWorkspaceType_ErrorMessage(t *testing.T) {
	t.Parallel()

	err := validateWorkspaceType("bogus")
	if err == nil {
		t.Fatal("expected error for unknown workspace type")
	}
	for _, expected := range []string{"local", "worktree", "container", "vm"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("error message should mention %q, got: %v", expected, err)
		}
	}
}

// TestRunRequest_WorkspaceType_ValidTypes_Extended verifies that the full set
// of known workspace types (including container and vm) pass validation.
func TestRunRequest_WorkspaceType_ValidTypes_Extended(t *testing.T) {
	t.Parallel()

	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{MaxSteps: 1},
	)

	// container and vm pass validation but provisioning will fail at execute()
	// time since they require orchestrator config. We only test that StartRun
	// does NOT return a validation error (the error comes later as run.failed).
	for _, wsType := range []string{"container", "vm"} {
		wsType := wsType
		t.Run("type="+wsType, func(t *testing.T) {
			t.Parallel()
			_, err := runner.StartRun(RunRequest{
				Prompt:        "hello",
				WorkspaceType: wsType,
			})
			// StartRun validates the type name only — should not error.
			if err != nil {
				t.Errorf("StartRun with workspace_type=%q rejected by validation: %v", wsType, err)
			}
		})
	}
}

// TestProfile_IsolationMode_Worktree_Integration verifies that when a profile
// with IsolationMode="worktree" is loaded from disk, the runner provisions a
// worktree workspace (workspace.provisioned event is emitted).
func TestProfile_IsolationMode_Worktree_Integration(t *testing.T) {
	t.Parallel()

	// Set up a git repo for the worktree.
	repoDir := initGitRepoForWS(t)
	worktreeRootDir := t.TempDir()

	// Create a profiles dir with a profile that specifies isolation_mode = "worktree".
	// IMPORTANT: isolation_mode is a top-level TOML field — it must appear before
	// any section header ([meta], [runner], [tools]) to avoid being parsed as a
	// nested field under the last section.
	profilesDir := t.TempDir()
	profileTOML := `isolation_mode = "worktree"

[meta]
name = "isolated-worktree"
description = "Test profile with worktree isolation"
version = 1

[runner]
model = ""
max_steps = 1

[tools]
allow = []
`
	if err := os.WriteFile(filepath.Join(profilesDir, "isolated-worktree.toml"), []byte(profileTOML), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{
			MaxSteps:    1,
			ProfilesDir: profilesDir,
			WorkspaceBaseOptions: WorkspaceProvisionOptions{
				RepoPath:        repoDir,
				WorktreeRootDir: worktreeRootDir,
			},
		},
	)

	// No WorkspaceType in the request — the profile's IsolationMode should kick in.
	run, err := runner.StartRun(RunRequest{
		Prompt:      "hello",
		ProfileName: "isolated-worktree",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
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
		t.Error("expected workspace.provisioned event — profile IsolationMode should have triggered provisioning")
	} else {
		if provisionedEv.Payload["workspace_type"] != "worktree" {
			t.Errorf("workspace.provisioned workspace_type = %v, want worktree",
				provisionedEv.Payload["workspace_type"])
		}
	}

	if destroyedEv == nil {
		t.Error("expected workspace.destroyed event after run completion")
	}
}

// TestProfile_IsolationMode_None_NoProvisioning verifies that a profile with
// IsolationMode="none" does NOT trigger workspace provisioning.
func TestProfile_IsolationMode_None_NoProvisioning(t *testing.T) {
	t.Parallel()

	profilesDir := t.TempDir()
	// isolation_mode must appear before section headers in TOML.
	profileTOML := `isolation_mode = "none"

[meta]
name = "no-isolation"
description = "Test profile with no isolation"
version = 1

[runner]
model = ""
max_steps = 1

[tools]
allow = []
`
	if err := os.WriteFile(filepath.Join(profilesDir, "no-isolation.toml"), []byte(profileTOML), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{
			MaxSteps:    1,
			ProfilesDir: profilesDir,
		},
	)

	run, err := runner.StartRun(RunRequest{
		Prompt:      "hello",
		ProfileName: "no-isolation",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events := drainRunEventsWS(t, runner, run.ID)

	for _, ev := range events {
		if ev.Type == EventWorkspaceProvisioned {
			t.Error("unexpected workspace.provisioned event — IsolationMode=none should not provision")
		}
	}
}

// TestProfile_IsolationMode_Explicit_Override verifies that an explicit
// RunRequest.WorkspaceType="local" overrides a profile's IsolationMode.
func TestProfile_IsolationMode_Explicit_Override(t *testing.T) {
	t.Parallel()

	// Profile says "worktree" but request says "local" — "local" wins.
	profilesDir := t.TempDir()
	// isolation_mode must appear before section headers in TOML.
	profileTOML := `isolation_mode = "worktree"

[meta]
name = "wants-worktree"
description = "Profile that prefers worktree"
version = 1

[runner]
model = ""
max_steps = 1

[tools]
allow = []
`
	if err := os.WriteFile(filepath.Join(profilesDir, "wants-worktree.toml"), []byte(profileTOML), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{
			MaxSteps:    1,
			ProfilesDir: profilesDir,
		},
	)

	run, err := runner.StartRun(RunRequest{
		Prompt:        "hello",
		ProfileName:   "wants-worktree",
		WorkspaceType: "local", // explicit override: local, not worktree
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events := drainRunEventsWS(t, runner, run.ID)

	for _, ev := range events {
		if ev.Type == EventWorkspaceProvisioned {
			wsType, _ := ev.Payload["workspace_type"].(string)
			if wsType == "worktree" {
				t.Errorf("workspace.provisioned type = worktree, want local — explicit WorkspaceType should win")
			}
		}
	}
}
