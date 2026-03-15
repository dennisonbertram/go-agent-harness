package tooluse

import (
	"os"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// stripANSI removes ANSI escape sequences from a string for width measurement.
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(s); {
		b := s[i]
		if b == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			i += 2
			continue
		}
		if inEscape {
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
				inEscape = false
			}
			i++
			continue
		}
		result.WriteByte(b)
		i++
	}
	return result.String()
}

// TestTUI026_CollapsedRunningShowsEllipsis verifies that a running tool call
// has a trailing "…" ellipsis in its rendered output.
func TestTUI026_CollapsedRunningShowsEllipsis(t *testing.T) {
	v := CollapsedView{
		ToolName: "ReadFile",
		Args:     "main.go",
		State:    StateRunning,
		Width:    80,
	}
	out := v.View()
	visible := stripANSI(out)
	if !strings.Contains(visible, "…") {
		t.Errorf("running state must contain '…' ellipsis, got: %q", visible)
	}
}

// TestTUI026_CollapsedCompletedNoEllipsis verifies that a completed tool call
// does NOT have a trailing "…" ellipsis.
func TestTUI026_CollapsedCompletedNoEllipsis(t *testing.T) {
	v := CollapsedView{
		ToolName: "ReadFile",
		Args:     "main.go",
		State:    StateCompleted,
		Width:    80,
	}
	out := v.View()
	visible := stripANSI(out)
	// Must not have ellipsis suffix in completed state
	if strings.HasSuffix(strings.TrimSpace(visible), "…") {
		t.Errorf("completed state must not end with '…', got: %q", visible)
	}
	// Must not have error cross either
	if strings.Contains(visible, "✗") {
		t.Errorf("completed state must not contain '✗', got: %q", visible)
	}
}

// TestTUI026_CollapsedErrorShowsCross verifies that an error tool call
// shows the "✗" error indicator.
func TestTUI026_CollapsedErrorShowsCross(t *testing.T) {
	v := CollapsedView{
		ToolName: "BashExec",
		Args:     "ls -la",
		State:    StateError,
		Width:    80,
	}
	out := v.View()
	visible := stripANSI(out)
	if !strings.Contains(visible, "✗") {
		t.Errorf("error state must contain '✗', got: %q", visible)
	}
}

// TestTUI026_CollapsedDotPrefix verifies that the ⏺ dot symbol appears in
// every rendered state.
func TestTUI026_CollapsedDotPrefix(t *testing.T) {
	states := []State{StateRunning, StateCompleted, StateError}
	for _, s := range states {
		v := CollapsedView{
			ToolName: "SomeTool",
			Args:     "arg1",
			State:    s,
			Width:    80,
		}
		out := v.View()
		if !strings.Contains(out, "⏺") {
			t.Errorf("state %d: expected '⏺' in output, got: %q", s, out)
		}
	}
}

// TestTUI026_CollapsedArgsTruncation verifies that very long args are truncated
// with "…" so the total line width stays within Width.
func TestTUI026_CollapsedArgsTruncation(t *testing.T) {
	longArgs := strings.Repeat("a", 200)
	v := CollapsedView{
		ToolName: "ReadFile",
		Args:     longArgs,
		State:    StateCompleted,
		Width:    80,
	}
	out := v.View()
	visible := stripANSI(out)
	runeCount := utf8.RuneCountInString(strings.TrimRight(visible, "\n"))
	if runeCount > 80 {
		t.Errorf("truncated output must fit within Width=80, got %d runes: %q", runeCount, visible)
	}
	// The output must still contain "…" indicating truncation occurred
	if !strings.Contains(visible, "…") {
		t.Errorf("long args must be truncated with '…', got: %q", visible)
	}
}

// TestTUI026_CollapsedEmptyArgs verifies that empty args render as "ToolName()"
// with empty parentheses.
func TestTUI026_CollapsedEmptyArgs(t *testing.T) {
	v := CollapsedView{
		ToolName: "ListDir",
		Args:     "",
		State:    StateCompleted,
		Width:    80,
	}
	out := v.View()
	visible := stripANSI(out)
	if !strings.Contains(visible, "ListDir()") {
		t.Errorf("empty args must render as 'ListDir()', got: %q", visible)
	}
}

// TestTUI026_CollapsedBoundaryWidths verifies rendering at boundary widths
// (10, 80, 200) does not panic and produces non-empty output.
func TestTUI026_CollapsedBoundaryWidths(t *testing.T) {
	cases := []struct {
		name  string
		width int
	}{
		{"width_10", 10},
		{"width_80", 80},
		{"width_200", 200},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := CollapsedView{
				ToolName: "BashExec",
				Args:     "go test ./...",
				State:    StateRunning,
				Width:    tc.width,
			}
			// Must not panic
			out := v.View()
			if out == "" {
				t.Errorf("[%s] View() returned empty string", tc.name)
			}
			// Must contain ⏺
			if !strings.Contains(out, "⏺") {
				t.Errorf("[%s] output must contain '⏺', got: %q", tc.name, out)
			}
		})
	}
}

// TestTUI026_CollapsedDefaultWidth verifies that Width=0 defaults to 80.
func TestTUI026_CollapsedDefaultWidth(t *testing.T) {
	v := CollapsedView{
		ToolName: "ReadFile",
		Args:     "main.go",
		State:    StateCompleted,
		Width:    0,
	}
	// Must not panic
	out := v.View()
	if out == "" {
		t.Fatal("View() must not return empty string for Width=0")
	}
	visible := stripANSI(out)
	runeCount := utf8.RuneCountInString(strings.TrimRight(visible, "\n"))
	if runeCount > 80 {
		t.Errorf("Width=0 defaults to 80, line must fit: got %d runes: %q", runeCount, visible)
	}
}

// TestTUI026_CollapsedConcurrent verifies that 10 goroutines calling View()
// concurrently produces no data races (run with -race).
func TestTUI026_CollapsedConcurrent(t *testing.T) {
	views := []CollapsedView{
		{ToolName: "ReadFile", Args: "main.go", State: StateRunning, Width: 80},
		{ToolName: "BashExec", Args: "go test ./...", State: StateCompleted, Width: 120},
		{ToolName: "WriteFile", Args: "out.go, content...", State: StateError, Width: 40},
		{ToolName: "GrepSearch", Args: strings.Repeat("x", 200), State: StateRunning, Width: 80},
		{ToolName: "ListDir", Args: "", State: StateCompleted, Width: 80},
		{ToolName: "FindTool", Args: "pattern", State: StateRunning, Width: 10},
		{ToolName: "GitDiff", Args: "HEAD", State: StateError, Width: 200},
		{ToolName: "ApplyPatch", Args: "patch.diff", State: StateCompleted, Width: 80},
		{ToolName: "LSPHover", Args: "file.go:10:5", State: StateRunning, Width: 80},
		{ToolName: "Task", Args: "implement feature", State: StateCompleted, Width: 80},
	}

	var wg sync.WaitGroup
	for _, v := range views {
		v := v // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				out := v.View()
				if out == "" {
					panic("View() returned empty string")
				}
			}
		}()
	}
	wg.Wait()
}

// TestTUI026_CollapsedToolNameAndArgsAppear verifies the tool name and
// (possibly truncated) args appear in the rendered output.
func TestTUI026_CollapsedToolNameAndArgsAppear(t *testing.T) {
	v := CollapsedView{
		ToolName: "ReadFile",
		Args:     "main.go",
		State:    StateCompleted,
		Width:    80,
	}
	out := v.View()
	visible := stripANSI(out)
	if !strings.Contains(visible, "ReadFile") {
		t.Errorf("tool name must appear in output, got: %q", visible)
	}
	if !strings.Contains(visible, "main.go") {
		t.Errorf("args must appear in output when they fit, got: %q", visible)
	}
}

// TestTUI026_CollapsedFormat verifies the overall format:
// "⏺ ToolName(args)" with correct parentheses.
func TestTUI026_CollapsedFormat(t *testing.T) {
	v := CollapsedView{
		ToolName: "GrepSearch",
		Args:     "pattern, file.go",
		State:    StateCompleted,
		Width:    80,
	}
	out := v.View()
	visible := stripANSI(strings.TrimRight(out, "\n"))
	// Must match format: ⏺ GrepSearch(pattern, file.go)
	if !strings.Contains(visible, "GrepSearch(") {
		t.Errorf("format must be 'ToolName(', got: %q", visible)
	}
	if !strings.Contains(visible, ")") {
		t.Errorf("format must include closing ')', got: %q", visible)
	}
}

// TestTUI026_VisualSnapshot_80x24 renders collapsed tool calls at 80 width
// and writes to testdata/snapshots/TUI-026-tooluse-80x24.txt.
func TestTUI026_VisualSnapshot_80x24(t *testing.T) {
	views := []CollapsedView{
		{ToolName: "ReadFile", Args: "cmd/harnesscli/tui/theme.go", State: StateCompleted, Width: 80},
		{ToolName: "BashExec", Args: "go test ./cmd/harnesscli/...", State: StateRunning, Width: 80},
		{ToolName: "GrepSearch", Args: "pattern, path/to/file.go", State: StateError, Width: 80},
		{ToolName: "WriteFile", Args: "cmd/harnesscli/tui/components/tooluse/collapsed.go, <content>", State: StateCompleted, Width: 80},
		{ToolName: "ListDir", Args: "", State: StateCompleted, Width: 80},
	}

	var sb strings.Builder
	for _, v := range views {
		sb.WriteString(v.View())
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-026-tooluse-80x24.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI026_VisualSnapshot_120x40 renders collapsed tool calls at 120 width.
func TestTUI026_VisualSnapshot_120x40(t *testing.T) {
	views := []CollapsedView{
		{ToolName: "ReadFile", Args: "cmd/harnesscli/tui/theme.go", State: StateCompleted, Width: 120},
		{ToolName: "BashExec", Args: "go test ./cmd/harnesscli/...", State: StateRunning, Width: 120},
		{ToolName: "GrepSearch", Args: "pattern, path/to/file.go", State: StateError, Width: 120},
		{ToolName: "WriteFile", Args: "cmd/harnesscli/tui/components/tooluse/collapsed.go, <content>", State: StateCompleted, Width: 120},
		{ToolName: "ListDir", Args: "", State: StateCompleted, Width: 120},
	}

	var sb strings.Builder
	for _, v := range views {
		sb.WriteString(v.View())
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-026-tooluse-120x40.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI026_VisualSnapshot_200x50 renders collapsed tool calls at 200 width.
func TestTUI026_VisualSnapshot_200x50(t *testing.T) {
	views := []CollapsedView{
		{ToolName: "ReadFile", Args: "cmd/harnesscli/tui/theme.go", State: StateCompleted, Width: 200},
		{ToolName: "BashExec", Args: "go test ./cmd/harnesscli/...", State: StateRunning, Width: 200},
		{ToolName: "GrepSearch", Args: "pattern, path/to/file.go", State: StateError, Width: 200},
		{ToolName: "WriteFile", Args: "cmd/harnesscli/tui/components/tooluse/collapsed.go, <content>", State: StateCompleted, Width: 200},
		{ToolName: "ListDir", Args: "", State: StateCompleted, Width: 200},
	}

	var sb strings.Builder
	for _, v := range views {
		sb.WriteString(v.View())
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-026-tooluse-200x50.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
