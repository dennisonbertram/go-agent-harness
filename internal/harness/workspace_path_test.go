package harness

// workspace_path_test.go — proves RunRequest.WorkspacePath (issue #1372) routes
// file tools through the existing per-run tool registry seam (the same one
// used for provisioned workspace_type runs) WITHOUT provisioning anything,
// and that the sandbox still confines writes to that root. It also proves the
// effective root is recorded on the conversation, matching the pre-existing
// behavior for RepoPath-configured runs (see TestIssue1256...).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// findToolCallCompletedFor returns the tool.call.completed event whose payload
// call_id matches callID, or nil if none was emitted.
func findToolCallCompletedFor(events []Event, callID string) *Event {
	for i := range events {
		ev := events[i]
		if ev.Type != EventToolCallCompleted {
			continue
		}
		if ev.Payload["call_id"] == callID {
			return &ev
		}
	}
	return nil
}

// TestStartRun_WorkspacePathRootsToolsAndSandboxConfinement is the primary
// regression test named in issue #1372: a write tool call with WorkspacePath
// set must land under that root (BT-001), and a write targeting an absolute
// path outside that root must be refused by the sandbox (BT-002) rather than
// silently escaping to the daemon's own workspace.
func TestStartRun_WorkspacePathRootsToolsAndSandboxConfinement(t *testing.T) {
	t.Parallel()

	wsRoot := t.TempDir()
	daemonCwd := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "escape.txt")
	if err := os.WriteFile(outsideFile, []byte("original"), 0o644); err != nil {
		t.Fatalf("seed outside file: %v", err)
	}

	runner := NewRunner(
		&stubProvider{turns: []CompletionResult{
			{ToolCalls: []ToolCall{{ID: "call-in", Name: "write", Arguments: `{"path":"hello.txt","content":"hi"}`}}},
			{ToolCalls: []ToolCall{{ID: "call-out", Name: "write", Arguments: fmt.Sprintf(`{"path":%q,"content":"pwned"}`, outsideFile)}}},
			{Content: "done"},
		}},
		NewDefaultRegistryWithOptions(daemonCwd, DefaultRegistryOptions{ApprovalMode: ToolApprovalModeFullAuto}),
		RunnerConfig{
			DefaultModel: "test-model",
			MaxSteps:     3,
		},
	)

	run, err := runner.StartRun(RunRequest{
		Prompt:        "write files",
		WorkspacePath: wsRoot,
		AllowedTools:  []string{"write"},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)
	events := getRunEvents(t, runner, run.ID)

	// BT-001: the relative-path write lands under the requested workspace_path.
	data, err := os.ReadFile(filepath.Join(wsRoot, "hello.txt"))
	if err != nil {
		t.Fatalf("expected hello.txt under workspace_path %s: %v", wsRoot, err)
	}
	if string(data) != "hi" {
		t.Fatalf("hello.txt content = %q, want %q", data, "hi")
	}
	if _, err := os.Stat(filepath.Join(daemonCwd, "hello.txt")); err == nil {
		t.Fatal("hello.txt leaked into the daemon's default registry root; workspace_path was not honored")
	}

	// BT-002: a write to an absolute path outside workspace_path is refused.
	completed := findToolCallCompletedFor(events, "call-out")
	if completed == nil {
		t.Fatal("expected a tool.call.completed event for call-out")
	}
	if completed.Payload["error"] == nil {
		t.Fatal("expected the write outside workspace_path to be refused by the sandbox, got no error")
	}
	stillOriginal, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatalf("outside file should remain readable: %v", err)
	}
	if string(stillOriginal) != "original" {
		t.Fatalf("outside file was modified despite sandbox confinement: %q", stillOriginal)
	}

	// The workspace.provisioned event (or equivalent) should carry the explicit
	// root, matching the provisioned-workspace behavior.
	provisioned := findEventByType(events, EventWorkspaceProvisioned)
	if provisioned == nil {
		t.Fatal("expected a workspace.provisioned event for explicit workspace_path")
	}
	if got := provisioned.Payload["workspace_path"]; got != wsRoot {
		t.Fatalf("workspace.provisioned workspace_path = %v, want %v", got, wsRoot)
	}
}

// TestStartRun_WorkspacePathRecordedOnConversation is the regression test:
// it proves the conversation store records the EFFECTIVE workspace root (the
// explicit workspace_path), not just the daemon's configured RepoPath, so a
// future rewind/restore or /conversations listing reflects where the run
// actually operated. This is a different angle from the tool-routing and
// sandbox-refusal behavioral tests above: it exercises the conversation
// persistence integration point and would fail if a future change reverted
// completeRun to always record rc.WorkspaceBaseOptions.RepoPath.
func TestStartRun_WorkspacePathRecordedOnConversation(t *testing.T) {
	t.Parallel()

	wsRoot := t.TempDir()
	store := newTestConversationStore(t)

	runner := NewRunner(
		&stubProvider{turns: []CompletionResult{{Content: "done"}}},
		NewRegistry(),
		RunnerConfig{
			DefaultModel:      "test-model",
			MaxSteps:          1,
			ConversationStore: store,
		},
	)

	run, err := runner.StartRun(RunRequest{
		Prompt:         "hello",
		WorkspacePath:  wsRoot,
		ConversationID: "conv-wspath-1372",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	owner, err := store.GetConversationOwner(context.Background(), "conv-wspath-1372")
	if err != nil {
		t.Fatalf("GetConversationOwner: %v", err)
	}
	if owner == nil {
		t.Fatal("expected a conversation owner row after completion")
	}
	if owner.Workspace != wsRoot {
		t.Fatalf("conversation workspace = %q, want the effective workspace_path %q", owner.Workspace, wsRoot)
	}
}
