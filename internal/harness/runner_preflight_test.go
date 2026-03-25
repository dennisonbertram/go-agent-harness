package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-agent-harness/internal/systemprompt"
)

func TestRunPreflight_UsesProfileIsolationModeFallback(t *testing.T) {
	repoDir := initGitRepoForWS(t)
	worktreeRootDir := t.TempDir()
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
			DefaultModel: "gpt-5-nano",
			MaxSteps:     1,
			ProfilesDir:  profilesDir,
			WorkspaceBaseOptions: WorkspaceProvisionOptions{
				RepoPath:        repoDir,
				WorktreeRootDir: worktreeRootDir,
			},
		},
	)

	runID := "run-preflight-profile"
	runner.runs[runID] = newRunStateForPreflightTest(runID)

	preflight, err := runner.runPreflight(context.Background(), runID, RunRequest{
		Prompt:      "hello",
		ProfileName: "isolated-worktree",
	})
	if err != nil {
		t.Fatalf("runPreflight: %v", err)
	}

	if preflight.effectiveWorkspaceType != "worktree" {
		t.Fatalf("effectiveWorkspaceType = %q, want worktree", preflight.effectiveWorkspaceType)
	}
	if got := preflight.messages[len(preflight.messages)-1].Content; got != "hello" {
		t.Fatalf("last preflight message = %q, want hello", got)
	}

	provisioned := findEventByType(runner.runs[runID].events, EventWorkspaceProvisioned)
	if provisioned == nil {
		t.Fatal("expected workspace.provisioned event")
	}
	if got := provisioned.Payload["workspace_type"]; got != "worktree" {
		t.Fatalf("workspace.provisioned workspace_type = %v, want worktree", got)
	}
	if runner.runs[runID].workspaceCleanup == nil {
		t.Fatal("expected workspace cleanup to be registered")
	}
}

func TestRunPreflight_ProvisionFailureEmitsWorkspaceProvisionFailed(t *testing.T) {
	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{
			DefaultModel: "gpt-5-nano",
			MaxSteps:     1,
		},
	)

	runID := "run-preflight-failure"
	runner.runs[runID] = newRunStateForPreflightTest(runID)

	_, err := runner.runPreflight(context.Background(), runID, RunRequest{
		Prompt:        "hello",
		WorkspaceType: "worktree",
	})
	if err == nil {
		t.Fatal("expected workspace provisioning error")
	}

	failed := findEventByType(runner.runs[runID].events, EventWorkspaceProvisionFailed)
	if failed == nil {
		t.Fatal("expected workspace.provision_failed event")
	}
	if got := failed.Payload["workspace_type"]; got != "worktree" {
		t.Fatalf("workspace.provision_failed workspace_type = %v, want worktree", got)
	}
}

func TestRunPreflight_ReResolvesSystemPromptWithWorkspacePath(t *testing.T) {
	repoDir := initGitRepoForWS(t)
	worktreeRootDir := t.TempDir()
	engine := &promptEngineStub{
		resolved: systemprompt.ResolvedPrompt{
			StaticPrompt:         "workspace-system",
			ResolvedIntent:       "general",
			ResolvedModelProfile: "default",
		},
	}

	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{
			DefaultModel:       "gpt-5-nano",
			DefaultAgentIntent: "general",
			MaxSteps:           1,
			PromptEngine:       engine,
			WorkspaceBaseOptions: WorkspaceProvisionOptions{
				RepoPath:        repoDir,
				WorktreeRootDir: worktreeRootDir,
			},
		},
	)

	runID := "run-preflight-prompt"
	state := newRunStateForPreflightTest(runID)
	state.staticSystemPrompt = "initial-system"
	state.promptResolved = &systemprompt.ResolvedPrompt{StaticPrompt: "initial-system"}
	runner.runs[runID] = state

	preflight, err := runner.runPreflight(context.Background(), runID, RunRequest{
		Prompt:        "hello",
		WorkspaceType: "worktree",
	})
	if err != nil {
		t.Fatalf("runPreflight: %v", err)
	}

	if preflight.systemPrompt != "workspace-system" {
		t.Fatalf("systemPrompt = %q, want workspace-system", preflight.systemPrompt)
	}
	if state.staticSystemPrompt != "workspace-system" {
		t.Fatalf("state.staticSystemPrompt = %q, want workspace-system", state.staticSystemPrompt)
	}
	provisioned := findEventByType(state.events, EventWorkspaceProvisioned)
	if provisioned == nil {
		t.Fatal("expected workspace.provisioned event")
	}
	if len(engine.resolveReqs) != 1 {
		t.Fatalf("resolve calls = %d, want 1", len(engine.resolveReqs))
	}
	if got := engine.resolveReqs[0].WorkspacePath; got != provisioned.Payload["workspace_path"] {
		t.Fatalf("resolve workspace path = %v, want %v", got, provisioned.Payload["workspace_path"])
	}
}

func TestRunPreflight_BuildsScopedMCPRegistry(t *testing.T) {
	runner := NewRunner(
		&staticProviderWS{result: CompletionResult{Content: "done"}},
		NewRegistry(),
		RunnerConfig{
			DefaultModel: "gpt-5-nano",
			MaxSteps:     1,
		},
	)

	runID := "run-preflight-mcp"
	state := newRunStateForPreflightTest(runID)
	runner.runs[runID] = state

	preflight, err := runner.runPreflight(context.Background(), runID, RunRequest{
		Prompt: "hello",
		MCPServers: []MCPServerConfig{{
			Name: "run-srv",
			URL:  "http://example.com/mcp",
		}},
	})
	if err != nil {
		t.Fatalf("runPreflight: %v", err)
	}

	if preflight.model != "gpt-5-nano" {
		t.Fatalf("model = %q, want gpt-5-nano", preflight.model)
	}
	if state.scopedMCPRegistry == nil {
		t.Fatal("expected scoped MCP registry on run state")
	}
	if !state.scopedMCPRegistry.isPerRun("run-srv") {
		t.Fatal("expected run-srv to be registered as a per-run MCP server")
	}
}

func newRunStateForPreflightTest(runID string) *runState {
	now := time.Now().UTC()
	return &runState{
		run: Run{
			ID:             runID,
			ConversationID: "conv-" + runID,
			CreatedAt:      now,
			UpdatedAt:      now,
			Status:         RunStatusQueued,
		},
		messages:       make([]Message, 0, 16),
		events:         make([]Event, 0, 16),
		subscribers:    make(map[chan Event]struct{}),
		steeringCh:     make(chan string, steeringBufferSize),
		firedOnceRules: make(map[string]bool),
	}
}

func findEventByType(events []Event, eventType EventType) *Event {
	for i := range events {
		if events[i].Type == eventType {
			return &events[i]
		}
	}
	return nil
}
