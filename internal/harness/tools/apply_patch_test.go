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

// TestApplyPatchOccurrenceReplacesNthMatch verifies that occurrence:2 replaces
// only the 2nd match, leaving the 1st and 3rd unchanged.
func TestApplyPatchOccurrenceReplacesNthMatch(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	// File has 3 occurrences of "TODO"
	original := "TODO first\nTODO second\nTODO third\n"
	if err := os.WriteFile(filepath.Join(workspace, "notes.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	args, _ := json.Marshal(map[string]any{
		"path":       "notes.txt",
		"find":       "TODO",
		"replace":    "DONE",
		"occurrence": 2,
	})
	out, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("apply_patch occurrence:2: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in output: %s", out)
	}

	got, err := os.ReadFile(filepath.Join(workspace, "notes.txt"))
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	content := string(got)
	// 1st occurrence should still be TODO
	if !strings.HasPrefix(content, "TODO first\n") {
		t.Errorf("1st occurrence should be unchanged; got: %q", content)
	}
	// 2nd occurrence should be DONE
	if !strings.Contains(content, "DONE second\n") {
		t.Errorf("2nd occurrence should be replaced; got: %q", content)
	}
	// 3rd occurrence should still be TODO
	if !strings.Contains(content, "TODO third\n") {
		t.Errorf("3rd occurrence should be unchanged; got: %q", content)
	}
}

// TestApplyPatchOccurrenceNotFound verifies that occurrence:99 on a file with
// only 2 occurrences returns an error containing "occurrence".
func TestApplyPatchOccurrenceNotFound(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "TODO first\nTODO second\n"
	if err := os.WriteFile(filepath.Join(workspace, "notes.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	args, _ := json.Marshal(map[string]any{
		"path":       "notes.txt",
		"find":       "TODO",
		"replace":    "DONE",
		"occurrence": 99,
	})
	_, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for occurrence:99 with only 2 occurrences")
	}
	if !strings.Contains(err.Error(), "occurrence") {
		t.Errorf("error should mention 'occurrence', got: %v", err)
	}
}

// TestApplyPatchOccurrenceNegative verifies that occurrence:-1 returns a validation error.
func TestApplyPatchOccurrenceNegative(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "hello\n"
	if err := os.WriteFile(filepath.Join(workspace, "test.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	args, _ := json.Marshal(map[string]any{
		"path":       "test.txt",
		"find":       "hello",
		"replace":    "bye",
		"occurrence": -1,
	})
	_, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for negative occurrence")
	}
	if !strings.Contains(err.Error(), "non-negative") {
		t.Errorf("error should mention 'non-negative', got: %v", err)
	}
}

// TestApplyPatchOccurrenceWithReplaceAll verifies that occurrence + replace_all
// together returns a validation error (mutually exclusive).
func TestApplyPatchOccurrenceWithReplaceAll(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "hello\nhello\n"
	if err := os.WriteFile(filepath.Join(workspace, "test.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	args, _ := json.Marshal(map[string]any{
		"path":        "test.txt",
		"find":        "hello",
		"replace":     "bye",
		"occurrence":  2,
		"replace_all": true,
	})
	_, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for occurrence + replace_all")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention 'mutually exclusive', got: %v", err)
	}
}

// TestApplyPatchEditsOccurrence verifies the occurrence field works in the edits[] path.
func TestApplyPatchEditsOccurrence(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	// File has 3 occurrences of "marker"
	original := "marker-A\nmarker-B\nmarker-C\n"
	if err := os.WriteFile(filepath.Join(workspace, "edits.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := findApplyPatchTool(t, workspace)

	args, _ := json.Marshal(map[string]any{
		"path": "edits.txt",
		"edits": []map[string]any{
			{
				"old_text":   "marker",
				"new_text":   "REPLACED",
				"occurrence": 2,
			},
		},
	})
	out, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("apply_patch edits occurrence:2: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in output: %s", out)
	}

	got, err := os.ReadFile(filepath.Join(workspace, "edits.txt"))
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	content := string(got)
	// 1st occurrence should still be "marker"
	if !strings.HasPrefix(content, "marker-A\n") {
		t.Errorf("1st occurrence should be unchanged; got: %q", content)
	}
	// 2nd occurrence should be "REPLACED"
	if !strings.Contains(content, "REPLACED-B\n") {
		t.Errorf("2nd occurrence should be replaced; got: %q", content)
	}
	// 3rd occurrence should still be "marker"
	if !strings.Contains(content, "marker-C\n") {
		t.Errorf("3rd occurrence should be unchanged; got: %q", content)
	}
}

// TestApplyPatchOccurrenceZeroMeansFirstMatch verifies that occurrence:0 (or absent)
// falls back to replacing the first match, consistent with the documented default.
func TestApplyPatchOccurrenceZeroMeansFirstMatch(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	original := "foo\nfoo\nfoo\n"
	if err := os.WriteFile(filepath.Join(workspace, "f.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	patch := findApplyPatchTool(t, workspace)
	args, _ := json.Marshal(map[string]any{
		"path":       "f.txt",
		"find":       "foo",
		"replace":    "bar",
		"occurrence": 0, // explicitly 0 → same as absent → first match
	})
	if _, err := patch.Handler(context.Background(), json.RawMessage(args)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(workspace, "f.txt"))
	if string(got) != "bar\nfoo\nfoo\n" {
		t.Errorf("expected only first foo replaced; got %q", got)
	}
}

// TestApplyPatchOccurrenceAtMax verifies that occurrence=maxOccurrence (10000) is
// accepted as a valid value; it will fail gracefully if the file doesn't have that
// many matches.
func TestApplyPatchOccurrenceAtMax(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	patch := findApplyPatchTool(t, workspace)
	args, _ := json.Marshal(map[string]any{
		"path":       "f.txt",
		"find":       "x",
		"replace":    "y",
		"occurrence": 10000, // at the cap — valid, but occurrence not found
	})
	_, err := patch.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("expected error for occurrence=10000 with only 1 match")
	}
	if !strings.Contains(err.Error(), "occurrence") {
		t.Errorf("expected 'occurrence' in error, got: %v", err)
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
