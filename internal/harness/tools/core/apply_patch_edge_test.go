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

// TestApplyPatchTool_BadJSON verifies malformed JSON args produce a parse error.
func TestApplyPatchTool_BadJSON(t *testing.T) {
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
}

// TestApplyPatchTool_PathEscapeRejected verifies a path escaping the
// workspace under SandboxScopeWorkspace is rejected before any file access.
func TestApplyPatchTool_PathEscapeRejected(t *testing.T) {
	dir := t.TempDir()
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir, SandboxScope: tools.SandboxScopeWorkspace})
	args, _ := json.Marshal(map[string]any{"path": "../outside.txt", "find": "a", "replace": "b"})
	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for path escaping the workspace")
	}
}

// TestApplyPatchTool_NonexistentFile verifies a nonexistent target file
// surfaces a read error.
func TestApplyPatchTool_NonexistentFile(t *testing.T) {
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	args, _ := json.Marshal(map[string]any{"path": "missing.txt", "find": "a", "replace": "b"})
	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "read patch file") {
		t.Errorf("expected read error, got %q", err.Error())
	}
}

// TestApplyPatchTool_ExpectedVersionMismatch verifies a stale
// expected_version on the direct find/replace path returns a stale_write
// error payload and leaves the file unmodified.
func TestApplyPatchTool_ExpectedVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{"path": "a.txt", "find": "hello", "replace": "bye", "expected_version": "bogus"})
	resultStr, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	errObj, ok := result["error"].(map[string]any)
	if !ok || errObj["code"] != "stale_write" {
		t.Fatalf("expected stale_write error payload, got %v", result)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("expected file unmodified after stale write rejection, got %q", data)
	}
}

// TestApplyPatchTool_DirectReplaceAll verifies the direct (non-edits,
// non-patch) find/replace path honors replace_all:true and reports the
// correct replacement count.
func TestApplyPatchTool_DirectReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("foo bar foo baz foo"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{"path": "a.txt", "find": "foo", "replace": "X", "replace_all": true})
	resultStr, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	if n, _ := result["replacements"].(float64); n != 3 {
		t.Errorf("expected 3 replacements, got %v", result["replacements"])
	}
	data, _ := os.ReadFile(path)
	if string(data) != "X bar X baz X" {
		t.Errorf("expected all occurrences replaced, got %q", data)
	}
}

// TestApplyPatchTool_DirectFindNotPresent verifies the plain (no occurrence,
// no replace_all) direct path rejects a find string absent from the file.
func TestApplyPatchTool_DirectFindNotPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{"path": "a.txt", "find": "nonexistent", "replace": "x"})
	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when find text is not present")
	}
	if !strings.Contains(err.Error(), "find text not present") {
		t.Errorf("expected 'find text not present' error, got %q", err.Error())
	}
}

// TestApplyPatchTool_MissingFind verifies find is required when neither
// edits nor patch is supplied.
func TestApplyPatchTool_MissingFind(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{"path": "a.txt", "replace": "x"})
	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when find is missing")
	}
}

// TestApplyPatchTool_EditsReplaceAll verifies a single edit with
// replace_all:true replaces every occurrence for that edit.
func TestApplyPatchTool_EditsReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("foo foo foo"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{
		"path": "a.txt",
		"edits": []map[string]any{
			{"old_text": "foo", "new_text": "X", "replace_all": true},
		},
	})
	resultStr, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	if n, _ := result["replacements"].(float64); n != 3 {
		t.Errorf("expected 3 replacements, got %v", result["replacements"])
	}
}

// TestApplyPatchTool_EditsValidationErrors verifies each per-edit validation
// failure (missing old_text, negative occurrence, occurrence above the cap,
// occurrence+replace_all mutually exclusive) is captured in failed_edits
// rather than aborting the whole request, and that "partial" reflects it.
func TestApplyPatchTool_EditsValidationErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{
		"path": "a.txt",
		"edits": []map[string]any{
			{"new_text": "x"}, // missing old_text
			{"old_text": "hello", "new_text": "x", "occurrence": -1},                     // negative occurrence
			{"old_text": "hello", "new_text": "x", "occurrence": maxOccurrence + 1},      // exceeds cap
			{"old_text": "hello", "new_text": "x", "occurrence": 1, "replace_all": true}, // mutually exclusive
			{"old_text": "hello", "new_text": "bye"},                                     // this one should succeed
		},
	})
	resultStr, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	failed, _ := result["failed_edits"].([]any)
	if len(failed) != 4 {
		t.Fatalf("expected 4 failed edits, got %d: %v", len(failed), failed)
	}
	if applied, _ := result["applied_edits"].(float64); applied != 1 {
		t.Errorf("expected 1 applied edit, got %v", result["applied_edits"])
	}
	if partial, _ := result["partial"].(bool); !partial {
		t.Error("expected partial:true when some edits failed")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "bye world" {
		t.Errorf("expected the surviving edit to be applied, got %q", data)
	}
}

// TestApplyPatchTool_EditsOldTextNotFound verifies a per-edit old_text that
// is absent from the (possibly already-modified) content is recorded as a
// failed edit with a distinct message from the occurrence-not-found case.
func TestApplyPatchTool_EditsOldTextNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{
		"path": "a.txt",
		"edits": []map[string]any{
			{"old_text": "nonexistent", "new_text": "x"},
		},
	})
	resultStr, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	failed, _ := result["failed_edits"].([]any)
	if len(failed) != 1 {
		t.Fatalf("expected 1 failed edit, got %d", len(failed))
	}
	entry := failed[0].(map[string]any)
	if !strings.Contains(entry["error"].(string), "old_text not found") {
		t.Errorf("expected 'old_text not found', got %v", entry["error"])
	}
}
