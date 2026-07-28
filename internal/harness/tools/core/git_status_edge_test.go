package core

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// requireGit skips the test if the git binary is unavailable.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

// TestGitStatusTool_CleanRepo verifies a freshly initialized repo with no
// changes reports clean:true and empty porcelain output.
func TestGitStatusTool_CleanRepo(t *testing.T) {
	requireGit(t)
	repo := t.TempDir()
	runGit(t, repo, "init")

	tool := GitStatusTool(tools.BuildOptions{WorkspaceRoot: repo})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if clean, _ := result["clean"].(bool); !clean {
		t.Errorf("expected clean:true for freshly-initialized repo, got %v (output=%q)", result["clean"], result["output"])
	}
	if output, _ := result["output"].(string); output != "" {
		t.Errorf("expected empty output for clean repo, got %q", output)
	}
	if code, ok := result["exit_code"].(float64); !ok || code != 0 {
		t.Errorf("expected exit_code 0, got %v", result["exit_code"])
	}
}

// TestGitStatusTool_DirtyRepo_PorcelainDefault verifies an untracked file
// shows up in porcelain-format output (the default) with clean:false, and
// that the porcelain "??" prefix is present.
func TestGitStatusTool_DirtyRepo_PorcelainDefault(t *testing.T) {
	requireGit(t)
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	writeFile(t, repo+"/untracked.txt", "hello\n")

	tool := GitStatusTool(tools.BuildOptions{WorkspaceRoot: repo})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if clean, _ := result["clean"].(bool); clean {
		t.Error("expected clean:false when an untracked file is present")
	}
	output, _ := result["output"].(string)
	if !strings.Contains(output, "?? untracked.txt") {
		t.Errorf("expected porcelain output to contain '?? untracked.txt', got %q", output)
	}
}

// TestGitStatusTool_NonPorcelain verifies porcelain:false produces the
// human-readable "Untracked files:" format rather than the "??" short form,
// proving the porcelain flag actually changes the git invocation.
func TestGitStatusTool_NonPorcelain(t *testing.T) {
	requireGit(t)
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	writeFile(t, repo+"/untracked.txt", "hello\n")

	tool := GitStatusTool(tools.BuildOptions{WorkspaceRoot: repo})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"porcelain":false}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	output, _ := result["output"].(string)
	if !strings.Contains(output, "Untracked files:") {
		t.Errorf("expected non-porcelain output to contain 'Untracked files:', got %q", output)
	}
	if strings.Contains(output, "?? untracked.txt") {
		t.Errorf("did not expect porcelain '??' marker in non-porcelain output, got %q", output)
	}
}

// TestGitStatusTool_OutsideGitRepo verifies running git_status against a
// directory that is not a git repository surfaces the failure through
// exit_code and output (git exits non-zero but the handler does not error,
// matching runCommand's nil-error-on-nonzero-exit contract) rather than
// silently succeeding.
func TestGitStatusTool_OutsideGitRepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()

	tool := GitStatusTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error from handler (git failure should surface via exit_code): %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	code, ok := result["exit_code"].(float64)
	if !ok || code == 0 {
		t.Fatalf("expected non-zero exit_code outside a git repo, got %v", result["exit_code"])
	}
	output, _ := result["output"].(string)
	if !strings.Contains(output, "not a git repository") {
		t.Errorf("expected output to mention 'not a git repository', got %q", output)
	}
	if clean, _ := result["clean"].(bool); clean {
		t.Error("expected clean:false when git status itself failed")
	}
}

// TestGitStatusTool_BadJSON verifies malformed JSON args produce a parse error.
func TestGitStatusTool_BadJSON(t *testing.T) {
	tool := GitStatusTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"porcelain": "notabool"}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
	if !strings.Contains(err.Error(), "parse git_status args") {
		t.Errorf("expected parse error, got %q", err.Error())
	}
}
