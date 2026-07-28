package core

// Unit tests for the unified-patch parser/applier internals
// (parseUnifiedPatch, applyUnifiedPatch, parseUnifiedPatchHunk).
//
// Moved here from the deleted duplicate tool package: these functions now live
// in core/apply_patch.go, which had no direct coverage of them.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

func TestParseUnifiedPatchBasic(t *testing.T) {
	t.Parallel()

	patch := "*** Begin Patch\n*** Add File: new.txt\n+hello\n+world\n*** End Patch"
	files, err := parseUnifiedPatch(patch)
	if err != nil {
		t.Fatalf("parseUnifiedPatch: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "new.txt" {
		t.Fatalf("Path: got %q", files[0].Path)
	}
	if files[0].Kind != "add" {
		t.Fatalf("Kind: got %q", files[0].Kind)
	}
}

func TestParseUnifiedPatchMissingBegin(t *testing.T) {
	t.Parallel()

	_, err := parseUnifiedPatch("not a patch")
	if err == nil || !strings.Contains(err.Error(), "Begin Patch") {
		t.Fatalf("expected Begin Patch error, got: %v", err)
	}
}

func TestParseUnifiedPatchMissingEnd(t *testing.T) {
	t.Parallel()

	_, err := parseUnifiedPatch("*** Begin Patch\n*** Add File: new.txt\n+hello\n")
	if err == nil || !strings.Contains(err.Error(), "missing terminator") {
		t.Fatalf("expected missing terminator error, got: %v", err)
	}
}

func TestParseUnifiedPatchDeleteFile(t *testing.T) {
	t.Parallel()

	patch := "*** Begin Patch\n*** Delete File: old.txt\n*** End Patch"
	files, err := parseUnifiedPatch(patch)
	if err != nil {
		t.Fatalf("parseUnifiedPatch: %v", err)
	}
	if len(files) != 1 || files[0].Kind != "delete" {
		t.Fatalf("expected delete, got: %+v", files)
	}
}

func TestParseUnifiedPatchUpdateFile(t *testing.T) {
	t.Parallel()

	patch := "*** Begin Patch\n*** Update File: src.txt\n@@ section\n-old line\n+new line\n*** End Patch"
	files, err := parseUnifiedPatch(patch)
	if err != nil {
		t.Fatalf("parseUnifiedPatch: %v", err)
	}
	if len(files) != 1 || files[0].Kind != "update" {
		t.Fatalf("expected update, got: %+v", files)
	}
	if len(files[0].Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(files[0].Hunks))
	}
	if !strings.Contains(files[0].Hunks[0].OldText, "old line") {
		t.Fatalf("OldText: got %q", files[0].Hunks[0].OldText)
	}
	if !strings.Contains(files[0].Hunks[0].NewText, "new line") {
		t.Fatalf("NewText: got %q", files[0].Hunks[0].NewText)
	}
}

func TestParseUnifiedPatchUnsupportedLine(t *testing.T) {
	t.Parallel()

	patch := "*** Begin Patch\nbogus line\n*** End Patch"
	_, err := parseUnifiedPatch(patch)
	if err == nil || !strings.Contains(err.Error(), "unsupported patch line") {
		t.Fatalf("expected unsupported patch line error, got: %v", err)
	}
}

func TestApplyUnifiedPatchAddFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	patch := "*** Begin Patch\n*** Add File: newfile.txt\n+hello world\n*** End Patch"

	result, err := applyUnifiedPatch(context.Background(), workspace, tools.SandboxScopeUnrestricted, patch)
	if err != nil {
		t.Fatalf("applyUnifiedPatch: %v", err)
	}
	if !strings.Contains(result, "newfile.txt") {
		t.Fatalf("expected newfile.txt in result: %s", result)
	}
	if !strings.Contains(result, `"add"`) {
		t.Fatalf("expected add action in result: %s", result)
	}

	content, err := os.ReadFile(filepath.Join(workspace, "newfile.txt"))
	if err != nil {
		t.Fatalf("read new file: %v", err)
	}
	if !strings.Contains(string(content), "hello world") {
		t.Fatalf("file content: got %q", string(content))
	}
}

func TestApplyUnifiedPatchDeleteFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "todelete.txt"), []byte("bye"), 0o644); err != nil {
		t.Fatal(err)
	}

	patch := "*** Begin Patch\n*** Delete File: todelete.txt\n*** End Patch"
	result, err := applyUnifiedPatch(context.Background(), workspace, tools.SandboxScopeUnrestricted, patch)
	if err != nil {
		t.Fatalf("applyUnifiedPatch: %v", err)
	}
	if !strings.Contains(result, `"delete"`) {
		t.Fatalf("expected delete action in result: %s", result)
	}

	if _, err := os.Stat(filepath.Join(workspace, "todelete.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted")
	}
}

func TestApplyUnifiedPatchUpdateFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("old line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	patch := "*** Begin Patch\n*** Update File: src.txt\n@@ section\n-old line\n+new line\n*** End Patch"
	result, err := applyUnifiedPatch(context.Background(), workspace, tools.SandboxScopeUnrestricted, patch)
	if err != nil {
		t.Fatalf("applyUnifiedPatch: %v", err)
	}
	if !strings.Contains(result, `"update"`) {
		t.Fatalf("expected update action in result: %s", result)
	}

	content, err := os.ReadFile(filepath.Join(workspace, "src.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "new line") {
		t.Fatalf("expected 'new line' in content, got: %q", string(content))
	}
}

func TestApplyUnifiedPatchUpdateMissingOldText(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("something\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	patch := "*** Begin Patch\n*** Update File: src.txt\n@@ section\n+only new\n*** End Patch"
	_, err := applyUnifiedPatch(context.Background(), workspace, tools.SandboxScopeUnrestricted, patch)
	if err == nil || !strings.Contains(err.Error(), "missing old text") {
		t.Fatalf("expected missing old text error, got: %v", err)
	}
}

func TestApplyUnifiedPatchUpdateHunkNotPresent(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("actual content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	patch := "*** Begin Patch\n*** Update File: src.txt\n@@ section\n-nonexistent content\n+replacement\n*** End Patch"
	_, err := applyUnifiedPatch(context.Background(), workspace, tools.SandboxScopeUnrestricted, patch)
	if err == nil || !strings.Contains(err.Error(), "not present") {
		t.Fatalf("expected hunk not present error, got: %v", err)
	}
}

func TestParseUnifiedPatchHunkContextLines(t *testing.T) {
	t.Parallel()

	// Test context lines (space prefix), removed lines, and added lines.
	lines := []string{
		" context before",
		"-removed",
		"+added",
		" context after",
		"*** End Patch",
	}
	hunk, next, err := parseUnifiedPatchHunk(lines, 0)
	if err != nil {
		t.Fatalf("parseUnifiedPatchHunk: %v", err)
	}
	if next != 4 {
		t.Fatalf("next: got %d, want 4", next)
	}
	if !strings.Contains(hunk.OldText, "context before") {
		t.Fatalf("OldText missing context: %q", hunk.OldText)
	}
	if !strings.Contains(hunk.OldText, "removed") {
		t.Fatalf("OldText missing removed line: %q", hunk.OldText)
	}
	if !strings.Contains(hunk.NewText, "added") {
		t.Fatalf("NewText missing added line: %q", hunk.NewText)
	}
	if !strings.Contains(hunk.NewText, "context after") {
		t.Fatalf("NewText missing context: %q", hunk.NewText)
	}
}

func TestParseUnifiedPatchHunkBadLine(t *testing.T) {
	t.Parallel()

	lines := []string{"xbad line"}
	_, _, err := parseUnifiedPatchHunk(lines, 0)
	if err == nil || !strings.Contains(err.Error(), "unexpected hunk line") {
		t.Fatalf("expected unexpected hunk line error, got: %v", err)
	}
}
