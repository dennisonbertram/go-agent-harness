package core

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestGitDiffTool_PathFilter verifies passing a path restricts the diff to
// that file only, proving the "--" path filter is actually wired into the
// git invocation rather than always diffing the whole tree.
func TestGitDiffTool_PathFilter(t *testing.T) {
	requireGit(t)
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	writeFile(t, filepath.Join(repo, "a.txt"), "a1\n")
	writeFile(t, filepath.Join(repo, "b.txt"), "b1\n")
	runGit(t, repo, "add", "a.txt", "b.txt")
	runGit(t, repo, "commit", "-m", "initial")
	writeFile(t, filepath.Join(repo, "a.txt"), "a2\n")
	writeFile(t, filepath.Join(repo, "b.txt"), "b2\n")

	tool := GitDiffTool(tools.BuildOptions{WorkspaceRoot: repo})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	diff, _ := result["diff"].(string)
	if !strings.Contains(diff, "a.txt") {
		t.Errorf("expected diff to mention a.txt, got %q", diff)
	}
	if strings.Contains(diff, "b.txt") {
		t.Errorf("expected diff to be filtered to a.txt only, but it mentioned b.txt: %q", diff)
	}
}

// TestGitDiffTool_StagedFlag verifies staged:true diffs the index rather
// than the working tree: an unstaged change is invisible with staged:true,
// but visible once staged.
func TestGitDiffTool_StagedFlag(t *testing.T) {
	requireGit(t)
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	writeFile(t, filepath.Join(repo, "a.txt"), "a1\n")
	runGit(t, repo, "add", "a.txt")
	runGit(t, repo, "commit", "-m", "initial")
	writeFile(t, filepath.Join(repo, "a.txt"), "a2\n")

	tool := GitDiffTool(tools.BuildOptions{WorkspaceRoot: repo})

	// Unstaged change, asking for staged diff: expect no diff content.
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"staged":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	if diff, _ := result["diff"].(string); diff != "" {
		t.Errorf("expected empty staged diff before 'git add', got %q", diff)
	}

	runGit(t, repo, "add", "a.txt")
	resultStr, err = tool.Handler(context.Background(), json.RawMessage(`{"staged":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	json.Unmarshal([]byte(resultStr), &result)
	if diff, _ := result["diff"].(string); !strings.Contains(diff, "a2") {
		t.Errorf("expected staged diff to show the staged change, got %q", diff)
	}
}

// TestGitDiffTool_PathEscapeRejected verifies a path that escapes the
// workspace under SandboxScopeWorkspace is rejected before git runs.
func TestGitDiffTool_PathEscapeRejected(t *testing.T) {
	dir := t.TempDir()
	tool := GitDiffTool(tools.BuildOptions{WorkspaceRoot: dir, SandboxScope: tools.SandboxScopeWorkspace})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"../outside.txt"}`))
	if err == nil {
		t.Fatal("expected error for path escaping the workspace")
	}
}
