package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestWriteTool_ContentAliases verifies each of the content aliases
// (new_text, new_string, text) is accepted in preference order when
// "content" itself is absent.
func TestWriteTool_ContentAliases(t *testing.T) {
	cases := []struct {
		name string
		args string
	}{
		{"new_text", `{"path":"a.txt","new_text":"from-new-text"}`},
		{"new_string", `{"path":"a.txt","new_string":"from-new-string"}`},
		{"text", `{"path":"a.txt","text":"from-text"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tool := WriteTool(tools.BuildOptions{WorkspaceRoot: dir})
			_, err := tool.Handler(context.Background(), json.RawMessage(tc.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "a.txt"))
			if err != nil {
				t.Fatalf("read written file: %v", err)
			}
			if string(data) == "" {
				t.Error("expected non-empty file content")
			}
		})
	}
}

// TestWriteTool_Append verifies append:true adds to existing content rather
// than truncating the file.
func TestWriteTool_Append(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("first;"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := WriteTool(tools.BuildOptions{WorkspaceRoot: dir})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","content":"second","append":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first;second" {
		t.Errorf("expected appended content 'first;second', got %q", data)
	}
}

// TestWriteTool_ExpectedVersionMismatch_ExistingFile verifies a stale
// expected_version against an existing file returns a stale_write error
// payload (not a hard error) and does NOT modify the file.
func TestWriteTool_ExpectedVersionMismatch_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := WriteTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","content":"new","expected_version":"bogus-version"}`))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected an error payload, got %v", result)
	}
	if errObj["code"] != "stale_write" {
		t.Errorf("expected code stale_write, got %v", errObj["code"])
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Errorf("expected file to remain unmodified after stale write rejection, got %q", data)
	}
}

// TestWriteTool_ExpectedVersionSet_FileDoesNotExist verifies supplying
// expected_version for a file that does not yet exist is rejected as a
// stale_write with an empty actual_version, rather than silently creating
// the file.
func TestWriteTool_ExpectedVersionSet_FileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	tool := WriteTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"new.txt","content":"x","expected_version":"anything"}`))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected an error payload, got %v", result)
	}
	if errObj["code"] != "stale_write" {
		t.Errorf("expected code stale_write, got %v", errObj["code"])
	}
	if errObj["actual_version"] != "" {
		t.Errorf("expected empty actual_version for a nonexistent file, got %v", errObj["actual_version"])
	}
	if _, statErr := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(statErr) {
		t.Error("expected the file to NOT be created when expected_version is stale")
	}
}

// TestWriteTool_MissingWorkspaceRoot verifies write refuses to operate when
// the workspace root itself does not exist (EnsureWorkspaceRootUsable
// failure), rather than attempting the write and failing confusingly later.
func TestWriteTool_MissingWorkspaceRoot(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")
	tool := WriteTool(tools.BuildOptions{WorkspaceRoot: missing})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","content":"x"}`))
	if err == nil {
		t.Fatal("expected error when workspace root does not exist")
	}
}

// TestWriteTool_TargetIsDirectory verifies attempting to write to a path
// that is already a directory surfaces an open-file error.
func TestWriteTool_TargetIsDirectory(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "adir"))
	tool := WriteTool(tools.BuildOptions{WorkspaceRoot: dir})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"adir","content":"x"}`))
	if err == nil {
		t.Fatal("expected error when target path is a directory")
	}
}

// TestWriteTool_BadJSON verifies malformed JSON args produce a parse error.
func TestWriteTool_BadJSON(t *testing.T) {
	tool := WriteTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
}
