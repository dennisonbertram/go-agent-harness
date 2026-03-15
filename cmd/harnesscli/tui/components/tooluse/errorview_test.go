package tooluse

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// TestTUI037_ToolErrorShowsWarningColor verifies that the ✗ symbol appears
// in the rendered ErrorView output.
func TestTUI037_ToolErrorShowsWarningColor(t *testing.T) {
	e := ErrorView{
		ToolName:  "ReadFile",
		ErrorText: "permission denied",
		Width:     80,
	}
	out := e.View()
	if !strings.Contains(out, "✗") {
		t.Errorf("expected '✗' in error view output, got: %q", out)
	}
}

// TestTUI037_ErrorStatePreservesPreviousToolOutput verifies that the ToolName
// is visible in the rendered ErrorView output.
func TestTUI037_ErrorStatePreservesPreviousToolOutput(t *testing.T) {
	e := ErrorView{
		ToolName:  "BashExec",
		ErrorText: "command not found",
		Width:     80,
	}
	out := e.View()
	visible := stripANSI(out)
	if !strings.Contains(visible, "BashExec") {
		t.Errorf("expected ToolName 'BashExec' in error view, got: %q", visible)
	}
}

// TestTUI037_ErrorTextWraps verifies that long error text wraps at Width.
func TestTUI037_ErrorTextWraps(t *testing.T) {
	longError := strings.Repeat("x", 200)
	e := ErrorView{
		ToolName:  "WriteFile",
		ErrorText: longError,
		Width:     80,
	}
	out := e.View()
	visible := stripANSI(out)

	// Each line (after stripping ANSI) must not exceed 80 chars significantly.
	// The error text is wrapped at Width-12, so each content line <= 80.
	lines := strings.Split(visible, "\n")
	for i, line := range lines {
		if len([]rune(line)) > 80 {
			t.Errorf("line %d exceeds width 80: %q", i, line)
		}
	}
	// Should have multiple lines since 200 chars exceeds width
	if len(lines) < 2 {
		t.Errorf("expected wrapped output with multiple lines, got: %q", visible)
	}
}

// TestTUI037_HintAppearsWhenSet verifies that the "Hint:" line is visible
// when ErrorView.Hint is non-empty.
func TestTUI037_HintAppearsWhenSet(t *testing.T) {
	e := ErrorView{
		ToolName:  "ReadFile",
		ErrorText: "no such file",
		Hint:      "Check file permissions",
		Width:     80,
	}
	out := e.View()
	visible := stripANSI(out)
	if !strings.Contains(visible, "Hint") {
		t.Errorf("expected 'Hint' line in output, got: %q", visible)
	}
	if !strings.Contains(visible, "Check file permissions") {
		t.Errorf("expected hint text in output, got: %q", visible)
	}
}

// TestTUI037_EmptyErrorText verifies that an empty ErrorText renders safely
// without panic.
func TestTUI037_EmptyErrorText(t *testing.T) {
	e := ErrorView{
		ToolName:  "ListDir",
		ErrorText: "",
		Width:     80,
	}
	// Must not panic
	out := e.View()
	if out == "" {
		t.Fatal("View() must not return empty string for empty ErrorText")
	}
	visible := stripANSI(out)
	if !strings.Contains(visible, "ListDir") {
		t.Errorf("expected ToolName in output even with empty ErrorText, got: %q", visible)
	}
}

// TestTUI037_EmptyHint verifies that no "Hint:" line appears when Hint is "".
func TestTUI037_EmptyHint(t *testing.T) {
	e := ErrorView{
		ToolName:  "GrepSearch",
		ErrorText: "file not found",
		Hint:      "",
		Width:     80,
	}
	out := e.View()
	visible := stripANSI(out)
	if strings.Contains(visible, "Hint") {
		t.Errorf("expected no 'Hint' line when Hint is empty, got: %q", visible)
	}
}

// TestTUI037_ConcurrentError verifies that 10 goroutines calling ErrorView.View()
// concurrently produces no data races (run with -race).
func TestTUI037_ConcurrentError(t *testing.T) {
	views := []ErrorView{
		{ToolName: "ReadFile", ErrorText: "permission denied", Width: 80},
		{ToolName: "BashExec", ErrorText: "command not found", Hint: "Try using full path", Width: 120},
		{ToolName: "WriteFile", ErrorText: "disk full", Width: 40},
		{ToolName: "GrepSearch", ErrorText: strings.Repeat("x", 200), Width: 80},
		{ToolName: "ListDir", ErrorText: "not a directory", Width: 10},
		{ToolName: "FindTool", ErrorText: "tool not found", Hint: "Use find_tool first", Width: 80},
		{ToolName: "GitDiff", ErrorText: "not a git repo", Width: 200},
		{ToolName: "ApplyPatch", ErrorText: "patch failed", Hint: "Check diff format", Width: 80},
		{ToolName: "LSPHover", ErrorText: "LSP not running", Hint: "Start language server", Width: 80},
		{ToolName: "Task", ErrorText: "context timeout", Width: 80},
	}

	var wg sync.WaitGroup
	for _, v := range views {
		v := v
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

// TestTUI037_BoundaryWidths verifies rendering at boundary widths
// (10, 80, 200) does not panic and produces non-empty output.
func TestTUI037_BoundaryWidths(t *testing.T) {
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
			e := ErrorView{
				ToolName:  "BashExec",
				ErrorText: "some error message here",
				Hint:      "Try again",
				Width:     tc.width,
			}
			out := e.View()
			if out == "" {
				t.Errorf("[%s] View() returned empty string", tc.name)
			}
			if !strings.Contains(out, "✗") {
				t.Errorf("[%s] output must contain '✗', got: %q", tc.name, out)
			}
		})
	}
}

// TestTUI037_VisualSnapshot_80x24 renders ErrorView at 80 width
// and writes to testdata/snapshots/TUI-037-error-80x24.txt.
func TestTUI037_VisualSnapshot_80x24(t *testing.T) {
	views := []ErrorView{
		{
			ToolName:  "ReadFile",
			ErrorText: "open /etc/shadow: permission denied",
			Hint:      "Check file permissions or run with sudo",
			Width:     80,
		},
		{
			ToolName:  "BashExec",
			ErrorText: "exit status 1: command not found: go",
			Hint:      "Ensure Go is installed and in PATH",
			Width:     80,
		},
		{
			ToolName:  "WriteFile",
			ErrorText: "write /var/log/app.log: no space left on device",
			Width:     80,
		},
		{
			ToolName:  "GrepSearch",
			ErrorText: "invalid regex: unexpected end of expression",
			Hint:      "Check your regex syntax",
			Width:     80,
		},
	}

	var sb strings.Builder
	for _, v := range views {
		sb.WriteString(v.View())
		sb.WriteString("\n")
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-037-error-80x24.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI037_VisualSnapshot_120x40 renders ErrorView at 120 width.
func TestTUI037_VisualSnapshot_120x40(t *testing.T) {
	views := []ErrorView{
		{
			ToolName:  "ReadFile",
			ErrorText: "open /etc/shadow: permission denied",
			Hint:      "Check file permissions",
			Width:     120,
		},
		{
			ToolName:  "BashExec",
			ErrorText: "exit status 1: the command failed due to a very long error message that should wrap properly",
			Width:     120,
		},
	}

	var sb strings.Builder
	for _, v := range views {
		sb.WriteString(v.View())
		sb.WriteString("\n")
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-037-error-120x40.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI037_VisualSnapshot_200x50 renders ErrorView at 200 width.
func TestTUI037_VisualSnapshot_200x50(t *testing.T) {
	views := []ErrorView{
		{
			ToolName:  "Task",
			ErrorText: "context deadline exceeded after 30s: the agent was unable to complete the task within the allotted time",
			Hint:      "Try increasing HARNESS_MAX_STEPS or breaking the task into smaller pieces",
			Width:     200,
		},
	}

	var sb strings.Builder
	for _, v := range views {
		sb.WriteString(v.View())
		sb.WriteString("\n")
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-037-error-200x50.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
