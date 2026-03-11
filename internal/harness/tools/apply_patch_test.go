package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findApplyPatchTool builds a catalog and returns the apply_patch tool.
func findApplyPatchTool(t *testing.T, workspace string) Tool {
	t.Helper()
	list, err := BuildCatalog(BuildOptions{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	return findToolByName(t, list, "apply_patch")
}

// TestApplyPatchUnifiedDiffAutoExtractsPath verifies that when a model sends a
// valid unified diff in the `patch` field without an explicit `path`, the tool
// auto-extracts the target path from the --- a/file header and applies the patch.
func TestApplyPatchUnifiedDiffAutoExtractsPath(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	// Write the original file
	original := "package main\n\nfunc hello() string {\n\treturn \"hello\"\n}\n"
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	// Model sends a unified diff with git-style a/ b/ prefixes in the patch field,
	// but no explicit path field.
	diff := `--- a/main.go
+++ b/main.go
@@ -1,5 +1,5 @@
 package main

 func hello() string {
-	return "hello"
+	return "world"
 }
`
	args, _ := json.Marshal(map[string]any{"patch": diff})
	out, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("apply_patch with unified diff: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in output: %s", out)
	}

	got, err := os.ReadFile(filepath.Join(workspace, "main.go"))
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	if !strings.Contains(string(got), `return "world"`) {
		t.Errorf("patch not applied; file contents: %s", string(got))
	}
	if strings.Contains(string(got), `return "hello"`) {
		t.Errorf("old content still present; file contents: %s", string(got))
	}
}

// TestApplyPatchUnifiedDiffGitStylePrefix verifies handling of git-style a/ b/
// prefixes in the --- and +++ headers.
func TestApplyPatchUnifiedDiffGitStylePrefix(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(filepath.Join(workspace, "words.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	// git diff style with a/ b/ prefixes
	diff := `--- a/words.txt
+++ b/words.txt
@@ -1,3 +1,3 @@
 alpha
-beta
+delta
 gamma
`
	args, _ := json.Marshal(map[string]any{"patch": diff})
	out, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("apply_patch git-style: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in output: %s", out)
	}

	got, err := os.ReadFile(filepath.Join(workspace, "words.txt"))
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	if !strings.Contains(string(got), "delta") {
		t.Errorf("patch not applied; got: %q", string(got))
	}
	if strings.Contains(string(got), "beta") {
		t.Errorf("old content still present; got: %q", string(got))
	}
}

// TestApplyPatchUnifiedDiffPlainPath verifies handling of diffs where the ---
// and +++ headers use plain paths without a/ b/ prefixes.
func TestApplyPatchUnifiedDiffPlainPath(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "one\ntwo\nthree\n"
	if err := os.WriteFile(filepath.Join(workspace, "nums.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	// Plain path style (no a/ b/ prefix) as produced by diff -u
	diff := `--- nums.txt
+++ nums.txt
@@ -1,3 +1,3 @@
 one
-two
+TWO
 three
`
	args, _ := json.Marshal(map[string]any{"patch": diff})
	out, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("apply_patch plain path: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in output: %s", out)
	}

	got, err := os.ReadFile(filepath.Join(workspace, "nums.txt"))
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	if !strings.Contains(string(got), "TWO") {
		t.Errorf("patch not applied; got: %q", string(got))
	}
}

// TestApplyPatchDiffFieldAlias verifies that the tool accepts unified diffs
// sent in the `diff` field (a common alias models use instead of `patch`).
func TestApplyPatchDiffFieldAlias(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "foo\nbar\nbaz\n"
	if err := os.WriteFile(filepath.Join(workspace, "test.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	// Model sends diff in the `diff` field (not `patch`)
	diff := `--- a/test.txt
+++ b/test.txt
@@ -1,3 +1,3 @@
 foo
-bar
+BAR
 baz
`
	args, _ := json.Marshal(map[string]any{"diff": diff})
	out, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("apply_patch with diff field alias: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in output: %s", out)
	}

	got, err := os.ReadFile(filepath.Join(workspace, "test.txt"))
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	if !strings.Contains(string(got), "BAR") {
		t.Errorf("patch not applied via diff alias; got: %q", string(got))
	}
}

// TestApplyPatchUnifiedDiffFieldAlias verifies that the tool accepts unified
// diffs sent in the `unified_diff` field.
func TestApplyPatchUnifiedDiffFieldAlias(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "x\ny\nz\n"
	if err := os.WriteFile(filepath.Join(workspace, "letters.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	diff := `--- a/letters.txt
+++ b/letters.txt
@@ -1,3 +1,3 @@
 x
-y
+Y
 z
`
	args, _ := json.Marshal(map[string]any{"unified_diff": diff})
	out, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("apply_patch with unified_diff field: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in output: %s", out)
	}

	got, err := os.ReadFile(filepath.Join(workspace, "letters.txt"))
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	if !strings.Contains(string(got), "Y") {
		t.Errorf("patch not applied via unified_diff alias; got: %q", string(got))
	}
}

// TestApplyPatchNoPathNoUnifiedDiff verifies that the tool still returns
// "path is required" when no path is provided and the patch is not a unified diff.
func TestApplyPatchNoPathNoUnifiedDiff(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	patch := findApplyPatchTool(t, workspace)

	// No path, no patch — should return "path is required"
	_, err := patch.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing path and no unified diff")
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Errorf("expected 'path is required' error, got: %v", err)
	}
}

// TestApplyPatchMultiFileUnifiedDiff verifies that a unified diff affecting
// multiple files is applied correctly, with each file auto-discovered from headers.
func TestApplyPatchMultiFileUnifiedDiff(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("apple\n"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "b.txt"), []byte("banana\n"), 0o644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	diff := `--- a/a.txt
+++ b/a.txt
@@ -1 +1 @@
-apple
+APPLE
--- a/b.txt
+++ b/b.txt
@@ -1 +1 @@
-banana
+BANANA
`
	args, _ := json.Marshal(map[string]any{"patch": diff})
	out, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("apply_patch multi-file: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in output: %s", out)
	}

	gotA, _ := os.ReadFile(filepath.Join(workspace, "a.txt"))
	gotB, _ := os.ReadFile(filepath.Join(workspace, "b.txt"))
	if !strings.Contains(string(gotA), "APPLE") {
		t.Errorf("a.txt not patched; got: %q", string(gotA))
	}
	if !strings.Contains(string(gotB), "BANANA") {
		t.Errorf("b.txt not patched; got: %q", string(gotB))
	}
}
