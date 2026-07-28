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

// ---------------------------------------------------------------------------
// read tool: hash_lines parameter (ported from internal/harness/tools/line_hash_test.go)
// ---------------------------------------------------------------------------

func TestReadHashLinesFormat(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	content := "first line\nsecond line\n"
	if err := os.WriteFile(filepath.Join(workspace, "f.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":       "f.txt",
		"hash_lines": true,
	})
	out, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("read with hash_lines: %v", err)
	}

	h1 := tools.LineHash("first line")
	h2 := tools.LineHash("second line")

	// Format must match legacy exactly: [hash] linenum→content
	if !strings.Contains(out, "["+h1+"] 1→first line") {
		t.Errorf("expected formatted first line with hash [%s], got: %s", h1, out)
	}
	if !strings.Contains(out, "["+h2+"] 2→second line") {
		t.Errorf("expected formatted second line with hash [%s], got: %s", h2, out)
	}
}

func TestReadHashLinesFalseOrAbsentNoPrefix(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	content := "hello\nworld\n"
	if err := os.WriteFile(filepath.Join(workspace, "hw.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{"path": "hw.txt"})
	out, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	helloHash := tools.LineHash("hello")
	if strings.Contains(out, "["+helloHash+"]") {
		t.Errorf("hash prefix should not appear when hash_lines is absent: %s", out)
	}
}

// ---------------------------------------------------------------------------
// read → edit hash pairing: a hash produced by read must be accepted by edit
// ---------------------------------------------------------------------------

func TestReadHashAcceptedByEditAsStartLineHash(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	content := "line one\nline two\nline three\n"
	if err := os.WriteFile(filepath.Join(workspace, "e.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	readTool := ReadTool(tools.BuildOptions{WorkspaceRoot: workspace})
	readArgs, _ := json.Marshal(map[string]any{"path": "e.txt", "hash_lines": true})
	readOut, err := readTool.Handler(context.Background(), json.RawMessage(readArgs))
	if err != nil {
		t.Fatalf("read with hash_lines: %v", err)
	}

	// Decode the read result the same way an agent would, then extract the
	// hash for "line two" out of the "[hash] 2→line two" formatted content.
	var parsed struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(readOut), &parsed); err != nil {
		t.Fatalf("unmarshal read result: %v", err)
	}
	var hash string
	for _, line := range strings.Split(parsed.Content, "\n") {
		if strings.Contains(line, "line two") {
			start := strings.Index(line, "[")
			end := strings.Index(line, "]")
			if start >= 0 && end > start {
				hash = line[start+1 : end]
			}
		}
	}
	if hash == "" {
		t.Fatalf("could not extract hash for 'line two' from read output: %s", readOut)
	}

	editTool := EditTool(tools.BuildOptions{WorkspaceRoot: workspace})
	editArgs, _ := json.Marshal(map[string]any{
		"path":            "e.txt",
		"old_text":        "line two",
		"new_text":        "LINE TWO",
		"start_line_hash": hash,
	})
	out, err := editTool.Handler(context.Background(), json.RawMessage(editArgs))
	if err != nil {
		t.Fatalf("edit with hash from read: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in output: %s", out)
	}

	got, _ := os.ReadFile(filepath.Join(workspace, "e.txt"))
	if !strings.Contains(string(got), "LINE TWO") {
		t.Errorf("replacement not applied; got: %q", string(got))
	}
}

// ---------------------------------------------------------------------------
// edit tool: start_line_hash / end_line_hash (ported from legacy tests)
// ---------------------------------------------------------------------------

func TestEditStartLineHashNotFound(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	content := "alpha\nbeta\n"
	if err := os.WriteFile(filepath.Join(workspace, "e2.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tool := EditTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":            "e2.txt",
		"old_text":        "alpha",
		"new_text":        "ALPHA",
		"start_line_hash": "deadbeef0000", // nonexistent hash
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for nonexistent start_line_hash")
	}
	if !strings.Contains(err.Error(), "start_line_hash") {
		t.Errorf("error should mention 'start_line_hash', got: %v", err)
	}
}

func TestEditEndLineHashNotFound(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	content := "alpha\nbeta\n"
	if err := os.WriteFile(filepath.Join(workspace, "e3.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tool := EditTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":          "e3.txt",
		"old_text":      "alpha",
		"new_text":      "ALPHA",
		"end_line_hash": "deadbeef0000", // nonexistent hash
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for nonexistent end_line_hash")
	}
	if !strings.Contains(err.Error(), "end_line_hash") {
		t.Errorf("error should mention 'end_line_hash', got: %v", err)
	}
}

func TestEditWithoutHashFieldsPreservesExistingBehavior(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	content := "foo\nbar\nbaz\n"
	if err := os.WriteFile(filepath.Join(workspace, "legacy.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tool := EditTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":     "legacy.txt",
		"old_text": "bar",
		"new_text": "BAR",
	})
	out, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("legacy edit failed: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in legacy edit: %s", out)
	}

	got, _ := os.ReadFile(filepath.Join(workspace, "legacy.txt"))
	if !strings.Contains(string(got), "BAR") {
		t.Errorf("legacy replacement not applied; got: %q", got)
	}
}

func TestEditEndLineHashMismatchWithLastLineOfOldText(t *testing.T) {
	t.Parallel()
	// Regression: end_line_hash exists in the file but does NOT match the
	// last line of old_text. The validator must reject this, not silently allow it.
	workspace := t.TempDir()
	content := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(filepath.Join(workspace, "endmismatch.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	gammaHash := tools.LineHash("gamma") // exists in file, but not the last line of old_text

	tool := EditTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":          "endmismatch.txt",
		"old_text":      "alpha\nbeta", // last line is "beta", not "gamma"
		"new_text":      "REPLACED",
		"end_line_hash": gammaHash,
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error: end_line_hash points to 'gamma' but last line of old_text is 'beta'")
	}
	if !strings.Contains(err.Error(), "end_line_hash") {
		t.Errorf("error should mention 'end_line_hash', got: %v", err)
	}
	if !strings.Contains(err.Error(), "does not match last line of old_text") {
		t.Errorf("error should describe mismatch with last line of old_text, got: %v", err)
	}
}

// TestEditStartLineHashTargetsAnchoredOccurrenceNotFirst is a regression test
// for position-aware replacement: a file with DUPLICATE identical lines (two
// "}" closing braces). Without the anchor fix, strings.Replace would replace
// the FIRST occurrence regardless of which line the hash points to. With the
// fix, the replacement is anchored to the byte offset of the matched line.
func TestEditStartLineHashTargetsAnchoredOccurrenceNotFirst(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	// File layout:
	//   0: func foo() {
	//   1: }                ← first "}"
	//   2: func bar() {
	//   3: }                ← second "}" — targeted via start_line_hash on line 2
	content := "func foo() {\n}\nfunc bar() {\n}\n"
	if err := os.WriteFile(filepath.Join(workspace, "dup.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	startHash := tools.LineHash("func bar() {")

	tool := EditTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":            "dup.go",
		"old_text":        "func bar() {\n}",
		"new_text":        "func bar() {\n\treturn\n}",
		"start_line_hash": startHash,
	})
	out, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("edit with start_line_hash on second block: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error in output: %s", out)
	}

	got, _ := os.ReadFile(filepath.Join(workspace, "dup.go"))
	gotStr := string(got)
	if !strings.Contains(gotStr, "func foo() {\n}") {
		t.Errorf("foo() block was incorrectly modified; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "func bar() {\n\treturn\n}") {
		t.Errorf("bar() block replacement not applied; got:\n%s", gotStr)
	}
}

// TestReadHashLinesWithOffsetAndLimit verifies hash_lines composes correctly
// with offset/limit paging — the hashes for the paged-in lines must still be
// present and correct. Ported from internal/harness/tools/line_hash_test.go.
func TestReadHashLinesWithOffsetAndLimit(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	content := "line1\nline2\nline3\nline4\n"
	if err := os.WriteFile(filepath.Join(workspace, "multi.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":       "multi.txt",
		"hash_lines": true,
		"offset":     1,
		"limit":      2,
	})
	out, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("read with offset+limit+hash_lines: %v", err)
	}

	h2 := tools.LineHash("line2")
	h3 := tools.LineHash("line3")
	if !strings.Contains(out, "["+h2+"]") {
		t.Errorf("expected hash for line2, got: %s", out)
	}
	if !strings.Contains(out, "["+h3+"]") {
		t.Errorf("expected hash for line3, got: %s", out)
	}
}

// TestEditEndLineHashReplacesBlockCorrectly verifies the SUCCESS path of
// end_line_hash — a multi-line block anchored by the hash of its last line is
// replaced correctly. The existing core coverage only exercised end_line_hash
// failure paths (NotFound, MismatchWithLastLineOfOldText); this closes that
// gap. Ported from internal/harness/tools/line_hash_test.go.
func TestEditEndLineHashReplacesBlockCorrectly(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	// Multiline content where end_line_hash points to the end of old_text
	content := "header\nstart here\nmiddle content\nend here\nfooter\n"
	if err := os.WriteFile(filepath.Join(workspace, "block.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	endHash := tools.LineHash("end here")

	tool := EditTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":          "block.txt",
		"old_text":      "start here\nmiddle content\nend here",
		"new_text":      "REPLACED BLOCK",
		"end_line_hash": endHash,
	})
	out, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("edit with end_line_hash: %v", err)
	}
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unexpected error: %s", out)
	}

	got, _ := os.ReadFile(filepath.Join(workspace, "block.txt"))
	if !strings.Contains(string(got), "REPLACED BLOCK") {
		t.Errorf("block replacement not applied; got: %q", got)
	}
	if strings.Contains(string(got), "middle content") {
		t.Errorf("old content still present; got: %q", got)
	}
}

func TestEditStartLineHashAnchorPositionMismatchReturnsError(t *testing.T) {
	t.Parallel()
	// Regression: start_line_hash points to a line that IS in the file, but
	// old_text does not begin at that byte offset.
	workspace := t.TempDir()
	content := "}\n}\n"
	if err := os.WriteFile(filepath.Join(workspace, "anchor.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	closingHash := tools.LineHash("}")
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: workspace})
	args, _ := json.Marshal(map[string]any{
		"path":            "anchor.go",
		"old_text":        "}\nNOT_IN_FILE",
		"new_text":        "REPLACED",
		"start_line_hash": closingHash,
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error: old_text does not match at anchor position")
	}
	if !strings.Contains(err.Error(), "anchor found at line") {
		t.Errorf("error should mention 'anchor found at line', got: %v", err)
	}
	if !strings.Contains(err.Error(), "old_text does not match at that position") {
		t.Errorf("error should mention 'old_text does not match at that position', got: %v", err)
	}
}
