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

// TestEditTool_ReplaceAll verifies replace_all:true replaces every
// occurrence and reports the correct replacement count.
func TestEditTool_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("foo bar foo baz foo"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{"path": "a.txt", "old_text": "foo", "new_text": "X", "replace_all": true})
	resultStr, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if n, _ := result["replacements"].(float64); n != 3 {
		t.Errorf("expected 3 replacements, got %v", result["replacements"])
	}
	data, _ := os.ReadFile(path)
	if string(data) != "X bar X baz X" {
		t.Errorf("expected all occurrences replaced, got %q", data)
	}
}

// TestEditTool_OldTextNotFound verifies an old_text that does not appear in
// the file is rejected rather than silently no-op-succeeding.
func TestEditTool_OldTextNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{"path": "a.txt", "old_text": "nonexistent", "new_text": "x"})
	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when old_text is not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %q", err.Error())
	}
}

// TestEditTool_ExpectedVersionMismatch verifies a stale expected_version is
// rejected with a stale_write payload and the file is left unmodified.
func TestEditTool_ExpectedVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{"path": "a.txt", "old_text": "hello", "new_text": "bye", "expected_version": "bogus"})
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
	if string(data) != "hello world" {
		t.Errorf("expected file unmodified after stale write rejection, got %q", data)
	}
}

// TestEditTool_LineHashAnchoring verifies start_line_hash/end_line_hash
// correctly anchor the replacement to a specific occurrence, using the real
// hash function so a wrong implementation of the anchor check would fail
// this test.
func TestEditTool_LineHashAnchoring(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	original := "dup\nkeep\ndup\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	hash := tools.LineHash("dup")

	tool := EditTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{
		"path": "a.txt", "old_text": "dup", "new_text": "REPLACED",
		"start_line_hash": hash, "end_line_hash": hash,
	})
	resultStr, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	if n, _ := result["replacements"].(float64); n != 1 {
		t.Errorf("expected exactly 1 anchored replacement, got %v", result["replacements"])
	}
	data, _ := os.ReadFile(path)
	// The FIRST occurrence of "dup" is anchored by the hash search (which
	// finds the first matching line), so only the first "dup" becomes REPLACED.
	if string(data) != "REPLACED\nkeep\ndup\n" {
		t.Errorf("expected only the anchored occurrence replaced, got %q", data)
	}
}

// TestEditTool_StartLineHashNotFound verifies an unmatched start_line_hash
// is rejected.
func TestEditTool_StartLineHashNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]any{"path": "a.txt", "old_text": "hello", "new_text": "x", "start_line_hash": "deadbeefcafe"})
	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for unmatched start_line_hash")
	}
	if !strings.Contains(err.Error(), "start_line_hash") {
		t.Errorf("expected start_line_hash error, got %q", err.Error())
	}
}

// TestEditTool_NonexistentFile verifies editing a nonexistent file surfaces
// a read error.
func TestEditTool_NonexistentFile(t *testing.T) {
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	args, _ := json.Marshal(map[string]any{"path": "missing.txt", "old_text": "a", "new_text": "b"})
	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "read file for edit") {
		t.Errorf("expected read error, got %q", err.Error())
	}
}

// TestEditTool_MissingOldText verifies old_text is required.
func TestEditTool_MissingOldText(t *testing.T) {
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","new_text":"x"}`))
	if err == nil {
		t.Fatal("expected error for missing old_text")
	}
}

// TestEditTool_BadJSON verifies malformed JSON args produce a parse error.
func TestEditTool_BadJSON(t *testing.T) {
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
}
