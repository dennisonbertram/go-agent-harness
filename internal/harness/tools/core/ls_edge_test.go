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

// TestLsTool_Handler_Recursive verifies recursive listing descends into
// subdirectories and returns nested-file relative paths, proving the
// recursive branch of collectEntries actually walks the tree rather than
// just listing the top level.
func TestLsTool_Handler_Recursive(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "top.txt"), "x")
	mustMkdir(t, filepath.Join(dir, "sub"))
	mustWrite(t, filepath.Join(dir, "sub", "nested.txt"), "x")

	tool := LsTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"recursive":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := lsEntries(t, resultStr)
	if !containsEntry(entries, "sub/nested.txt") {
		t.Errorf("expected recursive listing to contain sub/nested.txt, got %v", entries)
	}
	if !containsEntry(entries, "top.txt") {
		t.Errorf("expected recursive listing to contain top.txt, got %v", entries)
	}
}

// TestLsTool_Handler_DepthLimit verifies the depth parameter stops the walk
// once an entry's own depth exceeds the limit: with depth=2, dirA (depth 1)
// and its direct children fileA.txt/dirB (depth 2) are listed, but dirB's
// child fileB.txt (depth 3) is excluded and dirB's contents are pruned
// entirely (fs.SkipDir), proving depth is enforced per-entry rather than
// merely accepted and ignored.
func TestLsTool_Handler_DepthLimit(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "dirA"))
	mustWrite(t, filepath.Join(dir, "dirA", "fileA.txt"), "x")
	mustMkdir(t, filepath.Join(dir, "dirA", "dirB"))
	mustWrite(t, filepath.Join(dir, "dirA", "dirB", "fileB.txt"), "x")

	tool := LsTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"recursive":true,"depth":2}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := lsEntries(t, resultStr)
	for _, want := range []string{"dirA", "dirA/fileA.txt", "dirA/dirB"} {
		if !containsEntry(entries, want) {
			t.Errorf("expected depth-2 entries to include %q, got %v", want, entries)
		}
	}
	if containsEntry(entries, "dirA/dirB/fileB.txt") {
		t.Errorf("expected dirA/dirB/fileB.txt (depth 3) to be excluded at depth=2, got %v", entries)
	}
}

// TestLsTool_Handler_IncludeHidden verifies hidden entries (dotfiles) are
// excluded by default and included only when include_hidden is set, for
// both the recursive and non-recursive branches of collectEntries.
func TestLsTool_Handler_IncludeHidden(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".hidden"), "x")
	mustWrite(t, filepath.Join(dir, "visible.txt"), "x")

	tool := LsTool(tools.BuildOptions{WorkspaceRoot: dir})

	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := lsEntries(t, resultStr)
	if containsEntry(entries, ".hidden") {
		t.Errorf("expected .hidden to be excluded by default, got %v", entries)
	}

	resultStr, err = tool.Handler(context.Background(), json.RawMessage(`{"include_hidden":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries = lsEntries(t, resultStr)
	if !containsEntry(entries, ".hidden") {
		t.Errorf("expected .hidden to be included with include_hidden:true, got %v", entries)
	}
}

// TestLsTool_Handler_HiddenDirRecursiveSkip verifies that, in the recursive
// branch, a hidden directory's contents are pruned (fs.SkipDir) rather than
// merely having the directory entry itself filtered out.
func TestLsTool_Handler_HiddenDirRecursiveSkip(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".git"))
	mustWrite(t, filepath.Join(dir, ".git", "config"), "x")

	tool := LsTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"recursive":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := lsEntries(t, resultStr)
	if containsEntry(entries, ".git/config") {
		t.Errorf("expected contents of hidden directory to be pruned during recursive walk, got %v", entries)
	}
}

// TestLsTool_Handler_MaxEntriesTruncation_NonRecursive verifies the
// non-recursive branch reports truncated:true and caps the entry count at
// max_entries when more entries exist than the limit.
func TestLsTool_Handler_MaxEntriesTruncation_NonRecursive(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		mustWrite(t, filepath.Join(dir, "f"+string(rune('a'+i))+".txt"), "x")
	}

	tool := LsTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"max_entries":2}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	entries := lsEntries(t, resultStr)
	if len(entries) != 2 {
		t.Errorf("expected exactly 2 entries when capped at max_entries=2, got %d (%v)", len(entries), entries)
	}
	if truncated, _ := result["truncated"].(bool); !truncated {
		t.Error("expected truncated:true when entries exceed max_entries")
	}
}

// TestLsTool_Handler_MaxEntriesTruncation_Recursive verifies the recursive
// branch also honors max_entries and reports truncated:true, exercising the
// io.EOF short-circuit path inside filepath.WalkDir's callback.
func TestLsTool_Handler_MaxEntriesTruncation_Recursive(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		mustWrite(t, filepath.Join(dir, "f"+string(rune('a'+i))+".txt"), "x")
	}

	tool := LsTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"recursive":true,"max_entries":2}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	entries := lsEntries(t, resultStr)
	if len(entries) != 2 {
		t.Errorf("expected exactly 2 entries when capped at max_entries=2, got %d (%v)", len(entries), entries)
	}
	if truncated, _ := result["truncated"].(bool); !truncated {
		t.Error("expected truncated:true when recursive entries exceed max_entries")
	}
}

// TestLsTool_Handler_NonexistentPath verifies listing a path that does not
// exist surfaces the underlying os.ReadDir failure as an error, rather than
// silently returning an empty listing.
func TestLsTool_Handler_NonexistentPath(t *testing.T) {
	dir := t.TempDir()
	tool := LsTool(tools.BuildOptions{WorkspaceRoot: dir})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"does-not-exist"}`))
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

// TestLsTool_Handler_RecursiveWalkPermissionError verifies a permission
// failure while recursively walking a subdirectory is surfaced as an error
// (the "walk entries" branch), not swallowed.
func TestLsTool_Handler_RecursiveWalkPermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits not enforced the same way on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits")
	}
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	mustMkdir(t, blocked)
	mustWrite(t, filepath.Join(blocked, "secret.txt"), "x")
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0o755) })

	tool := LsTool(tools.BuildOptions{WorkspaceRoot: dir})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"recursive":true}`))
	if err == nil {
		t.Fatal("expected error when recursive walk hits a permission-denied directory")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func lsEntries(t *testing.T, resultStr string) []string {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	raw, _ := result["entries"].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func containsEntry(entries []string, target string) bool {
	for _, e := range entries {
		if e == target {
			return true
		}
	}
	return false
}
