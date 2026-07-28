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

// TestGrepTool_SingleFile verifies grep against a single file path (not a
// directory) still finds and reports matches with correct line numbers.
func TestGrepTool_SingleFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "one\nneedle here\nthree\n")

	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"needle","path":"a.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	matches, _ := result["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d (%v)", len(matches), matches)
	}
	m := matches[0].(map[string]any)
	if ln, _ := m["line_number"].(float64); ln != 2 {
		t.Errorf("expected match on line 2, got %v", m["line_number"])
	}
}

// TestGrepTool_CaseSensitivity verifies case_sensitive:true excludes
// differently-cased matches that the case-insensitive default would find.
func TestGrepTool_CaseSensitivity(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "Needle\n")

	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: dir})

	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"needle"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	matches, _ := result["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("expected case-insensitive match by default, got %d matches", len(matches))
	}

	resultStr, err = tool.Handler(context.Background(), json.RawMessage(`{"query":"needle","case_sensitive":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	json.Unmarshal([]byte(resultStr), &result)
	matches, _ = result["matches"].([]any)
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches with case_sensitive:true against differently-cased text, got %d", len(matches))
	}
}

// TestGrepTool_RegexMatch verifies regex:true compiles the query as a
// regular expression rather than a literal substring.
func TestGrepTool_RegexMatch(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "code123\nnomatch\n")

	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"code[0-9]+","regex":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	matches, _ := result["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("expected 1 regex match, got %d", len(matches))
	}
}

// TestGrepTool_LiteralTextOverridesRegex verifies literal_text:true forces
// the query to be treated as a literal string even when regex:true is also
// set, so regex metacharacters in the query are matched literally.
func TestGrepTool_LiteralTextOverridesRegex(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "a[0-9]+b literal line\n")

	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"a[0-9]+b","regex":true,"literal_text":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	matches, _ := result["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("expected literal_text to match the literal bracket text, got %d matches", len(matches))
	}
}

// TestGrepTool_InvalidRegex verifies an unparsable regex query surfaces a
// compile error rather than silently matching nothing.
func TestGrepTool_InvalidRegex(t *testing.T) {
	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"(unclosed","regex":true}`))
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

// TestGrepTool_MaxMatchesTruncation verifies truncated:true is reported and
// the match count is capped once max_matches is hit.
func TestGrepTool_MaxMatchesTruncation(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("needle\n", 10)
	mustWrite(t, filepath.Join(dir, "a.txt"), content)

	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"needle","max_matches":3}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	matches, _ := result["matches"].([]any)
	if len(matches) != 3 {
		t.Fatalf("expected exactly 3 matches when capped, got %d", len(matches))
	}
	if truncated, _ := result["truncated"].(bool); !truncated {
		t.Error("expected truncated:true")
	}
}

// TestGrepTool_SkipsGitDir verifies the walk prunes ".git" directories
// rather than searching their contents.
func TestGrepTool_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".git"))
	mustWrite(t, filepath.Join(dir, ".git", "config"), "needle\n")
	mustWrite(t, filepath.Join(dir, "real.txt"), "needle\n")

	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"needle"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	matches, _ := result["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("expected only the match outside .git, got %d matches: %v", len(matches), matches)
	}
}

// TestGrepTool_SkipsBinaryFiles verifies a file containing a NUL byte is
// treated as binary and skipped rather than reporting spurious matches.
func TestGrepTool_SkipsBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bin.dat"), []byte("needle\x00trailing"), 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"needle"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal([]byte(resultStr), &result)
	matches, _ := result["matches"].([]any)
	if len(matches) != 0 {
		t.Errorf("expected binary file to be skipped, got %d matches", len(matches))
	}
}

// TestGrepTool_NonexistentPath verifies a nonexistent path surfaces the stat
// failure as an error.
func TestGrepTool_NonexistentPath(t *testing.T) {
	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"x","path":"does-not-exist"}`))
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	if !strings.Contains(err.Error(), "stat grep path") {
		t.Errorf("expected stat error, got %q", err.Error())
	}
}

// TestGrepTool_EmptyQuery verifies an empty/whitespace query is rejected.
func TestGrepTool_EmptyQuery(t *testing.T) {
	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"   "}`))
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

// TestGrepTool_BadJSON verifies malformed JSON args produce a parse error.
func TestGrepTool_BadJSON(t *testing.T) {
	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"query": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
}
