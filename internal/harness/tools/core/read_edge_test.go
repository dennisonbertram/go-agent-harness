package core

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestReadTool_OffsetLimit verifies offset/limit slice the file to the
// requested line window and populate per-line "lines" objects with correct
// 1-based line numbers.
func TestReadTool_OffsetLimit(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "l1\nl2\nl3\nl4\nl5\n")

	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","offset":1,"limit":2}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	content, _ := result["content"].(string)
	if content != "l2\nl3" {
		t.Errorf("expected content 'l2\\nl3' for offset=1 limit=2, got %q", content)
	}
	lines, _ := result["lines"].([]any)
	if len(lines) != 2 {
		t.Fatalf("expected 2 line objects, got %d", len(lines))
	}
	first := lines[0].(map[string]any)
	if ln, _ := first["line_number"].(float64); ln != 2 {
		t.Errorf("expected first line_number 2, got %v", first["line_number"])
	}
}

// TestReadTool_HashLines verifies hash_lines:true prefixes each line with a
// content hash and a 1-based line number arrow, rather than returning plain
// content.
func TestReadTool_HashLines(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "alpha\nbeta\n")

	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","hash_lines":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	content, _ := result["content"].(string)
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 hash-prefixed lines, got %d: %q", len(lines), content)
	}
	if !strings.Contains(lines[0], "1→alpha") {
		t.Errorf("expected first hashed line to contain '1→alpha', got %q", lines[0])
	}
	if !strings.Contains(lines[1], "2→beta") {
		t.Errorf("expected second hashed line to contain '2→beta', got %q", lines[1])
	}
	if !strings.HasPrefix(lines[0], "[") {
		t.Errorf("expected hashed line to start with a bracketed hash, got %q", lines[0])
	}
}

// TestReadTool_MaxBytesTruncation verifies content longer than max_bytes is
// truncated and truncated:true is reported.
func TestReadTool_MaxBytesTruncation(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), strings.Repeat("x", 100))

	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","max_bytes":10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	content, _ := result["content"].(string)
	if len(content) != 10 {
		t.Errorf("expected content truncated to 10 bytes, got %d bytes", len(content))
	}
	if truncated, _ := result["truncated"].(bool); !truncated {
		t.Error("expected truncated:true")
	}
}

// TestReadTool_NegativeMaxBytesClampsToDefault verifies a non-positive
// max_bytes falls back to the 16KiB default rather than reading zero bytes.
func TestReadTool_NegativeMaxBytesClampsToDefault(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "short content")

	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","max_bytes":-5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	if content, _ := result["content"].(string); content != "short content" {
		t.Errorf("expected full content with negative max_bytes clamped to default, got %q", content)
	}
}

// TestReadTool_NegativeOffsetAndLimitClampToZero verifies negative
// offset/limit are clamped to 0 (meaning "no windowing") rather than
// producing an out-of-range slice or error.
func TestReadTool_NegativeOffsetAndLimitClampToZero(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "l1\nl2\n")

	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","offset":-1,"limit":-1}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	if content, _ := result["content"].(string); content != "l1\nl2\n" {
		t.Errorf("expected full unwindowed content, got %q", content)
	}
}

// TestReadTool_NonexistentFile verifies opening a nonexistent file surfaces
// an "open file" error.
func TestReadTool_NonexistentFile(t *testing.T) {
	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"missing.txt"}`))
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "open file") {
		t.Errorf("expected open file error, got %q", err.Error())
	}
}

// TestReadTool_BadJSON verifies malformed JSON args produce a parse error.
func TestReadTool_BadJSON(t *testing.T) {
	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
}
