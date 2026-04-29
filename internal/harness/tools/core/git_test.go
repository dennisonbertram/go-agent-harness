package core

import (
	"context"
	"encoding/json"
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
	opts := tools.BuildOptions{WorkspaceRoot: "."}
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
