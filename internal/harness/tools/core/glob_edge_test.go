package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestGlobTool_MatchesSortedAndFiltered verifies glob returns only files
// matching the pattern, expressed as workspace-relative slash paths, sorted
// alphabetically.
func TestGlobTool_MatchesSortedAndFiltered(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "z.txt"), "x")
	mustWrite(t, filepath.Join(dir, "a.txt"), "x")
	mustWrite(t, filepath.Join(dir, "other.md"), "x")

	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"pattern":"*.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	raw, _ := result["matches"].([]any)
	if len(raw) != 2 {
		t.Fatalf("expected 2 .txt matches, got %d (%v)", len(raw), raw)
	}
	if raw[0].(string) != "a.txt" || raw[1].(string) != "z.txt" {
		t.Errorf("expected sorted [a.txt z.txt], got %v", raw)
	}
}

// TestGlobTool_MaxMatchesTruncation verifies the match list is capped at
// max_matches.
func TestGlobTool_MaxMatchesTruncation(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		mustWrite(t, filepath.Join(dir, "f"+string(rune('a'+i))+".txt"), "x")
	}

	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"pattern":"*.txt","max_matches":2}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	raw, _ := result["matches"].([]any)
	if len(raw) != 2 {
		t.Fatalf("expected exactly 2 matches when capped, got %d", len(raw))
	}
}

// TestGlobTool_AbsolutePatternRejected verifies an absolute glob pattern is
// rejected up front.
func TestGlobTool_AbsolutePatternRejected(t *testing.T) {
	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"pattern":"/etc/*"}`))
	if err == nil {
		t.Fatal("expected error for absolute pattern")
	}
}

// TestGlobTool_EscapingPatternRejected verifies a pattern that walks above
// the workspace root ("..") is rejected.
func TestGlobTool_EscapingPatternRejected(t *testing.T) {
	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"pattern":"../*"}`))
	if err == nil {
		t.Fatal("expected error for pattern escaping the workspace")
	}
}

// TestGlobTool_NestedPattern verifies a subdirectory glob pattern
// ("sub/*.txt") matches nested files.
func TestGlobTool_NestedPattern(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "sub"))
	mustWrite(t, filepath.Join(dir, "sub", "nested.txt"), "x")
	mustWrite(t, filepath.Join(dir, "top.txt"), "x")

	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"pattern":"sub/*.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	raw, _ := result["matches"].([]any)
	if len(raw) != 1 || raw[0].(string) != "sub/nested.txt" {
		t.Errorf("expected [sub/nested.txt], got %v", raw)
	}
}

// TestGlobTool_SymlinkEscapeFiltered verifies a match reached only via a
// symlink pointing outside the workspace is dropped even though it matches
// the glob pattern lexically, under SandboxScopeWorkspace.
func TestGlobTool_SymlinkEscapeFiltered(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on windows")
	}
	dir := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.txt"), "x")
	linkPath := filepath.Join(dir, "escape-link")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: dir, SandboxScope: tools.SandboxScopeWorkspace})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"pattern":"escape-link/*.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	raw, _ := result["matches"].([]any)
	if len(raw) != 0 {
		t.Errorf("expected symlink-escaped match to be filtered out under workspace scope, got %v", raw)
	}
}

// TestGlobTool_NoMatches verifies a pattern matching nothing returns an
// empty (not nil-crashing) matches list.
func TestGlobTool_NoMatches(t *testing.T) {
	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"pattern":"*.nonexistent"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	raw, _ := result["matches"].([]any)
	if len(raw) != 0 {
		t.Errorf("expected 0 matches, got %v", raw)
	}
}

// TestGlobTool_BadJSON verifies malformed JSON args produce a parse error.
func TestGlobTool_BadJSON(t *testing.T) {
	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"pattern": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
}
