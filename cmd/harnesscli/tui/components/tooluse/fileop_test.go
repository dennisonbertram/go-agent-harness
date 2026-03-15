package tooluse

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// TestTUI034_FileOpSummaryRead verifies that a read operation produces "Read N lines" format.
func TestTUI034_FileOpSummaryRead(t *testing.T) {
	result := "line1\nline2\nline3\nline4\nline5"
	s := ParseFileOp("read_file", "main.go", result)

	if s.Kind != FileOpRead {
		t.Errorf("expected FileOpRead, got %d", s.Kind)
	}
	if s.LineCount != 5 {
		t.Errorf("expected LineCount=5, got %d", s.LineCount)
	}

	line := s.Line()
	visible := stripANSI(line)
	if !strings.Contains(visible, "Read") {
		t.Errorf("expected 'Read' in summary line, got: %q", visible)
	}
	if !strings.Contains(visible, "5") {
		t.Errorf("expected line count '5' in summary line, got: %q", visible)
	}
	if !strings.Contains(visible, "⎿") {
		t.Errorf("expected '⎿' prefix in summary line, got: %q", visible)
	}
}

// TestTUI034_FileOpSummaryWrite verifies that a write operation produces "Wrote N lines to {file}" format.
func TestTUI034_FileOpSummaryWrite(t *testing.T) {
	result := "a\nb\nc\nd\ne\nf\ng"
	s := ParseFileOp("write_file", "utils.go", result)

	if s.Kind != FileOpWrite {
		t.Errorf("expected FileOpWrite, got %d", s.Kind)
	}
	if s.LineCount != 7 {
		t.Errorf("expected LineCount=7, got %d", s.LineCount)
	}
	if s.FileName != "utils.go" {
		t.Errorf("expected FileName='utils.go', got %q", s.FileName)
	}

	line := s.Line()
	visible := stripANSI(line)
	if !strings.Contains(visible, "Wrote") {
		t.Errorf("expected 'Wrote' in summary line, got: %q", visible)
	}
	if !strings.Contains(visible, "7") {
		t.Errorf("expected line count '7' in summary line, got: %q", visible)
	}
	if !strings.Contains(visible, "utils.go") {
		t.Errorf("expected filename 'utils.go' in summary line, got: %q", visible)
	}
}

// TestTUI034_FileOpSummaryEdit verifies that an edit operation produces
// "Added N lines to {file}" for + lines or "Edited {file}" with no + lines.
func TestTUI034_FileOpSummaryEdit(t *testing.T) {
	// Edit with + lines (diff-style)
	result := "+added line one\n+added line two\n context line\n-removed line"
	s := ParseFileOp("edit_file", "main.go", result)

	if s.Kind != FileOpEdit {
		t.Errorf("expected FileOpEdit, got %d", s.Kind)
	}

	line := s.Line()
	visible := stripANSI(line)
	if !strings.Contains(visible, "Added") {
		t.Errorf("expected 'Added' in edit summary line, got: %q", visible)
	}
	if !strings.Contains(visible, "2") {
		t.Errorf("expected '+' line count '2' in summary line, got: %q", visible)
	}

	// Edit with no + lines — fallback to "Edited {file}"
	resultNoPlus := "context only\nno changes marked"
	s2 := ParseFileOp("edit_file", "server.go", resultNoPlus)
	line2 := s2.Line()
	visible2 := stripANSI(line2)
	if !strings.Contains(visible2, "Edited") {
		t.Errorf("expected 'Edited' for edit with no + lines, got: %q", visible2)
	}
	if !strings.Contains(visible2, "server.go") {
		t.Errorf("expected filename in 'Edited' line, got: %q", visible2)
	}
}

// TestTUI034_FileOpSummaryUnknown verifies that an unknown tool name returns "" from Line().
func TestTUI034_FileOpSummaryUnknown(t *testing.T) {
	s := ParseFileOp("bash_exec", "main.go", "some output")
	if s.Kind != FileOpUnknown {
		t.Errorf("expected FileOpUnknown for 'bash_exec', got %d", s.Kind)
	}
	line := s.Line()
	if line != "" {
		t.Errorf("expected empty Line() for FileOpUnknown, got: %q", line)
	}
}

// TestTUI034_FileOpLongFileNameEllipsized verifies that a very long filename
// is truncated with "…" in the summary line.
func TestTUI034_FileOpLongFileNameEllipsized(t *testing.T) {
	longName := strings.Repeat("a", 200) + ".go"
	s := ParseFileOp("write_file", longName, "line1\nline2")
	line := s.Line()
	visible := stripANSI(line)

	// The visible line should not contain the full 200-char name
	if strings.Contains(visible, strings.Repeat("a", 100)) {
		t.Errorf("expected long filename to be truncated, got: %q", visible)
	}
	if !strings.Contains(visible, "…") {
		t.Errorf("expected '…' in truncated filename, got: %q", visible)
	}
}

// TestTUI034_FileOpZeroLineCount verifies that LineCount=0 returns "" from Line().
func TestTUI034_FileOpZeroLineCount(t *testing.T) {
	s := FileOpSummary{
		Kind:      FileOpRead,
		FileName:  "main.go",
		LineCount: 0,
	}
	line := s.Line()
	if line != "" {
		t.Errorf("expected empty Line() for LineCount=0, got: %q", line)
	}
}

// TestTUI034_ParseFileOpDetectsTool verifies that tool name detection works
// for all supported aliases.
func TestTUI034_ParseFileOpDetectsTool(t *testing.T) {
	cases := []struct {
		toolName string
		wantKind FileOpKind
	}{
		{"read_file", FileOpRead},
		{"Read", FileOpRead},
		{"write_file", FileOpWrite},
		{"Write", FileOpWrite},
		{"Create", FileOpWrite},
		{"edit_file", FileOpEdit},
		{"Edit", FileOpEdit},
		{"str_replace_editor", FileOpEdit},
		{"unknown_tool", FileOpUnknown},
		{"BashExec", FileOpUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.toolName, func(t *testing.T) {
			s := ParseFileOp(tc.toolName, "file.go", "line1\nline2")
			if s.Kind != tc.wantKind {
				t.Errorf("ParseFileOp(%q): expected kind %d, got %d", tc.toolName, tc.wantKind, s.Kind)
			}
		})
	}
}

// TestTUI034_FileOpConcurrent verifies that 10 goroutines calling ParseFileOp
// and Line() concurrently produces no data races (run with -race).
func TestTUI034_FileOpConcurrent(t *testing.T) {
	inputs := []struct {
		tool   string
		file   string
		result string
	}{
		{"read_file", "main.go", "a\nb\nc"},
		{"write_file", "utils.go", "x\ny\nz"},
		{"edit_file", "server.go", "+added\ncontext"},
		{"Read", "theme.go", "line1\nline2\nline3"},
		{"Write", "model.go", "p\nq\nr\ns"},
		{"Edit", "handler.go", "+new1\n+new2\nold"},
		{"str_replace_editor", "router.go", "+route\nother"},
		{"unknown", "junk.go", "output"},
		{"Create", "new_file.go", "a\nb"},
		{"read_file", "config.go", strings.Repeat("line\n", 50)},
	}

	var wg sync.WaitGroup
	for _, inp := range inputs {
		inp := inp
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				s := ParseFileOp(inp.tool, inp.file, inp.result)
				_ = s.Line()
			}
		}()
	}
	wg.Wait()
}

// TestTUI034_BinaryFileNoName verifies that an empty filename is handled safely.
func TestTUI034_BinaryFileNoName(t *testing.T) {
	s := ParseFileOp("write_file", "", "line1\nline2\nline3")
	// Should not panic
	line := s.Line()
	// With empty filename, Line() should still render something or empty
	// but must not panic
	_ = line
}

// TestTUI034_VisualSnapshot_80x24 renders FileOpSummary lines at 80 width
// and writes to testdata/snapshots/TUI-034-fileop-80x24.txt.
func TestTUI034_VisualSnapshot_80x24(t *testing.T) {
	snapshots := []struct {
		tool   string
		file   string
		result string
	}{
		{"read_file", "cmd/harnesscli/tui/theme.go", strings.Repeat("line\n", 42)},
		{"write_file", "cmd/harnesscli/tui/components/tooluse/fileop.go", strings.Repeat("line\n", 7)},
		{"edit_file", "internal/harness/runner.go", "+added line one\n+added line two\ncontext\n-removed"},
		{"str_replace_editor", "cmd/harnessd/main.go", "+new func\n+second line\nold context"},
		{"Read", "go.mod", strings.Repeat("dep\n", 15)},
	}

	var sb strings.Builder
	for _, snap := range snapshots {
		s := ParseFileOp(snap.tool, snap.file, snap.result)
		line := s.Line()
		if line != "" {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-034-fileop-80x24.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI034_VisualSnapshot_120x40 renders FileOpSummary lines at 120 width.
func TestTUI034_VisualSnapshot_120x40(t *testing.T) {
	snapshots := []struct {
		tool   string
		file   string
		result string
	}{
		{"read_file", "cmd/harnesscli/tui/theme.go", strings.Repeat("line\n", 42)},
		{"write_file", "internal/harness/runner.go", strings.Repeat("code\n", 120)},
		{"edit_file", "cmd/harnessd/main.go", "+added\ncontext\n-removed"},
	}

	var sb strings.Builder
	for _, snap := range snapshots {
		s := ParseFileOp(snap.tool, snap.file, snap.result)
		line := s.Line()
		if line != "" {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-034-fileop-120x40.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI034_VisualSnapshot_200x50 renders FileOpSummary lines at 200 width.
func TestTUI034_VisualSnapshot_200x50(t *testing.T) {
	snapshots := []struct {
		tool   string
		file   string
		result string
	}{
		{"read_file", "cmd/harnesscli/tui/theme.go", strings.Repeat("line\n", 42)},
		{"write_file", "internal/harness/runner.go", strings.Repeat("code\n", 200)},
		{"edit_file", "cmd/harnessd/main.go", "+a\n+b\n+c\nctx\n-old"},
		{"Create", "internal/workspace/pool.go", strings.Repeat("func\n", 88)},
	}

	var sb strings.Builder
	for _, snap := range snapshots {
		s := ParseFileOp(snap.tool, snap.file, snap.result)
		line := s.Line()
		if line != "" {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-034-fileop-200x50.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
