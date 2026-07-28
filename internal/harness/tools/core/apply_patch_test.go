package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestApplyPatchOccurrenceReplacesNthMatch verifies that occurrence:2 replaces
// only the 2nd match, leaving the 1st and 3rd unchanged.
func TestApplyPatchOccurrenceReplacesNthMatch(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "TODO first\nTODO second\nTODO third\n"
	if err := os.WriteFile(filepath.Join(workspace, "notes.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})

	args, _ := json.Marshal(map[string]any{
		"path":       "notes.txt",
		"find":       "TODO",
		"replace":    "DONE",
		"occurrence": 2,
	})
	out, err := tool.Handler(context.Background(), json.RawMessage(args))
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
	if !strings.HasPrefix(content, "TODO first\n") {
		t.Errorf("1st occurrence should be unchanged; got: %q", content)
	}
	if !strings.Contains(content, "DONE second\n") {
		t.Errorf("2nd occurrence should be replaced; got: %q", content)
	}
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

	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})

	args, _ := json.Marshal(map[string]any{
		"path":       "notes.txt",
		"find":       "TODO",
		"replace":    "DONE",
		"occurrence": 99,
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for occurrence:99 with only 2 occurrences")
	}
	if !strings.Contains(err.Error(), "occurrence") {
		t.Errorf("error should mention 'occurrence', got: %v", err)
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

	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})

	args, _ := json.Marshal(map[string]any{
		"path":        "test.txt",
		"find":        "hello",
		"replace":     "bye",
		"occurrence":  2,
		"replace_all": true,
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for occurrence + replace_all")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention 'mutually exclusive', got: %v", err)
	}
}

// TestApplyPatchOccurrenceAboveCap verifies that an occurrence value above
// maxOccurrence (10000) is rejected outright, before any scan is attempted.
func TestApplyPatchOccurrenceAboveCap(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":       "f.txt",
		"find":       "x",
		"replace":    "y",
		"occurrence": maxOccurrence + 1,
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for occurrence above the cap")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("expected 'exceeds maximum' in error, got: %v", err)
	}
}

// TestApplyPatchEditsOccurrence verifies the occurrence field works in the edits[] path.
func TestApplyPatchEditsOccurrence(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "marker-A\nmarker-B\nmarker-C\n"
	if err := os.WriteFile(filepath.Join(workspace, "edits.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})

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
	out, err := tool.Handler(context.Background(), json.RawMessage(args))
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
	if !strings.HasPrefix(content, "marker-A\n") {
		t.Errorf("1st occurrence should be unchanged; got: %q", content)
	}
	if !strings.Contains(content, "REPLACED-B\n") {
		t.Errorf("2nd occurrence should be replaced; got: %q", content)
	}
	if !strings.Contains(content, "marker-C\n") {
		t.Errorf("3rd occurrence should be unchanged; got: %q", content)
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

	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})

	diff := `--- a/test.txt
+++ b/test.txt
@@ -1,3 +1,3 @@
 foo
-bar
+BAR
 baz
`
	args, _ := json.Marshal(map[string]any{"diff": diff})
	out, err := tool.Handler(context.Background(), json.RawMessage(args))
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
// diffs sent in the `unified_diff` field, and that `patch`, `diff`, and
// `unified_diff` all produce identical results for the same payload.
func TestApplyPatchUnifiedDiffFieldAlias(t *testing.T) {
	t.Parallel()

	diff := `--- a/letters.txt
+++ b/letters.txt
@@ -1,3 +1,3 @@
 x
-y
+Y
 z
`
	original := "x\ny\nz\n"

	for _, field := range []string{"patch", "diff", "unified_diff"} {
		field := field
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			workspace := t.TempDir()
			if err := os.WriteFile(filepath.Join(workspace, "letters.txt"), []byte(original), 0o644); err != nil {
				t.Fatalf("write original: %v", err)
			}
			tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})

			args, _ := json.Marshal(map[string]any{field: diff})
			out, err := tool.Handler(context.Background(), json.RawMessage(args))
			if err != nil {
				t.Fatalf("apply_patch with %s field: %v", field, err)
			}
			if strings.Contains(out, `"error"`) {
				t.Fatalf("unexpected error in output: %s", out)
			}

			got, err := os.ReadFile(filepath.Join(workspace, "letters.txt"))
			if err != nil {
				t.Fatalf("read patched file: %v", err)
			}
			if string(got) != "x\nY\nz\n" {
				t.Errorf("patch not applied via %s field; got: %q", field, string(got))
			}
		})
	}
}

// TestApplyPatchUnifiedDiffPlainPath verifies handling of diffs where the ---
// and +++ headers use plain paths without a/ b/ prefixes (as produced by
// `diff -u`). Ported from internal/harness/tools/apply_patch_test.go — this
// header shape is distinct from the a/ b/ prefixed diffs covered by
// TestApplyPatchTool_Handler_StandardUnifiedDiff.
func TestApplyPatchUnifiedDiffPlainPath(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "one\ntwo\nthree\n"
	if err := os.WriteFile(filepath.Join(workspace, "nums.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})

	diff := `--- nums.txt
+++ nums.txt
@@ -1,3 +1,3 @@
 one
-two
+TWO
 three
`
	args, _ := json.Marshal(map[string]any{"patch": diff})
	out, err := tool.Handler(context.Background(), json.RawMessage(args))
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

// TestApplyPatchOccurrenceNegative verifies that occurrence:-1 returns a
// validation error. Ported from internal/harness/tools/apply_patch_test.go.
func TestApplyPatchOccurrenceNegative(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()

	original := "hello\n"
	if err := os.WriteFile(filepath.Join(workspace, "test.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})

	args, _ := json.Marshal(map[string]any{
		"path":       "test.txt",
		"find":       "hello",
		"replace":    "bye",
		"occurrence": -1,
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for negative occurrence")
	}
	if !strings.Contains(err.Error(), "non-negative") {
		t.Errorf("error should mention 'non-negative', got: %v", err)
	}
}

// TestApplyPatchOccurrenceZeroMeansFirstMatch verifies that occurrence:0
// (explicitly set) falls back to replacing the first match, consistent with
// the documented default. Ported from internal/harness/tools/apply_patch_test.go.
func TestApplyPatchOccurrenceZeroMeansFirstMatch(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	original := "foo\nfoo\nfoo\n"
	if err := os.WriteFile(filepath.Join(workspace, "f.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":       "f.txt",
		"find":       "foo",
		"replace":    "bar",
		"occurrence": 0, // explicitly 0 → same as absent → first match
	})
	if _, err := tool.Handler(context.Background(), json.RawMessage(args)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(workspace, "f.txt"))
	if string(got) != "bar\nfoo\nfoo\n" {
		t.Errorf("expected only first foo replaced; got %q", got)
	}
}

// TestApplyPatchOccurrenceAtMax verifies that occurrence=maxOccurrence
// (10000, the cap value itself, not above it) is accepted as a valid input
// and fails gracefully — "occurrence not found" — rather than being rejected
// outright the way an above-cap value is (see TestApplyPatchOccurrenceAboveCap).
// Ported from internal/harness/tools/apply_patch_test.go.
func TestApplyPatchOccurrenceAtMax(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":       "f.txt",
		"find":       "x",
		"replace":    "y",
		"occurrence": maxOccurrence, // at the cap — valid, but occurrence not found
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("expected error for occurrence=maxOccurrence with only 1 match")
	}
	if !strings.Contains(err.Error(), "occurrence") {
		t.Errorf("expected 'occurrence' in error, got: %v", err)
	}
}
