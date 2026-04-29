package core

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/harness/tools"
)

func TestGitDiffTool_Basic(t *testing.T) {
	opts := tools.BuildOptions{WorkspaceRoot: "."}
	gitDiff := GitDiffTool(opts)

	// Test default call with no arguments
	resultStr, err := gitDiff.Handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("GitDiffTool handler failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Check keys and types
	if _, ok := result["diff"]; !ok {
		t.Error("Result missing 'diff' key")
	}
	if _, ok := result["exit_code"]; !ok {
		t.Error("Result missing 'exit_code' key")
	}
	if _, ok := result["timed_out"]; !ok {
		t.Error("Result missing 'timed_out' key")
	}
}

func TestGitDiffTool_MaxBytes(t *testing.T) {
	repoDir := initGitDiffTestRepo(t)
	filePath := filepath.Join(repoDir, "tracked.txt")
	if err := os.WriteFile(filePath, []byte("this line is intentionally much longer than ten bytes\n"), 0o644); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}

	opts := tools.BuildOptions{WorkspaceRoot: repoDir}
	gitDiff := GitDiffTool(opts)

	args := []byte(`{"max_bytes":10}`)
	resultStr, err := gitDiff.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("GitDiffTool handler failed with max_bytes: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	diff, ok := result["diff"].(string)
	if !ok {
		t.Fatal("Diff field is not a string")
	}
	if len(diff) > 10 {
		t.Errorf("Diff output longer than max_bytes: got %d", len(diff))
	}
	if truncated, ok := result["truncated"].(bool); !ok || !truncated {
		t.Error("Truncated flag not set when output truncated")
	}
}

func TestGitDiffTool_BadJSON(t *testing.T) {
	opts := tools.BuildOptions{WorkspaceRoot: "."}
	gitDiff := GitDiffTool(opts)

	// Passing malformed JSON should cause an error
	_, err := gitDiff.Handler(context.Background(), []byte(`{"max_bytes": "notanint"}`))
	if err == nil {
		t.Error("Expected error for malformed JSON input, got nil")
	}
}

func initGitDiffTestRepo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	runGitDiffTestGit(t, dir, "init")
	runGitDiffTestGit(t, dir, "config", "user.email", "test@example.com")
	runGitDiffTestGit(t, dir, "config", "user.name", "Test User")

	filePath := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(filePath, []byte("short\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	runGitDiffTestGit(t, dir, "add", "tracked.txt")
	runGitDiffTestGit(t, dir, "commit", "-m", "init")

	return dir
}

func runGitDiffTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}
