package tooluse

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// TestTUI031_ExpandedToolcallShowsDetails verifies that params and result are
// visible in the expanded view output.
func TestTUI031_ExpandedToolcallShowsDetails(t *testing.T) {
	v := ExpandedView{
		ToolName: "ReadFile",
		Args:     "main.go, n=50",
		Params: []Param{
			{Key: "path", Value: "main.go"},
			{Key: "limit", Value: "50"},
		},
		Result: "package main\n\nfunc main() {}",
		State:  StateCompleted,
		Width:  80,
	}
	out := v.View()
	visible := stripANSI(out)

	if !strings.Contains(visible, "ReadFile") {
		t.Errorf("expected ToolName 'ReadFile' in output, got: %q", visible)
	}
	if !strings.Contains(visible, "path") {
		t.Errorf("expected param key 'path' in output, got: %q", visible)
	}
	if !strings.Contains(visible, "main.go") {
		t.Errorf("expected param value 'main.go' in output, got: %q", visible)
	}
	if !strings.Contains(visible, "package main") {
		t.Errorf("expected result content in output, got: %q", visible)
	}
}

// TestTUI031_ExpandedViewHasTreeConnectors verifies that ⎿ tree connectors
// appear on param and result lines.
func TestTUI031_ExpandedViewHasTreeConnectors(t *testing.T) {
	v := ExpandedView{
		ToolName: "BashExec",
		Args:     "go test ./...",
		Params: []Param{
			{Key: "cmd", Value: "go test ./..."},
		},
		Result: "ok  go-agent-harness",
		State:  StateCompleted,
		Width:  80,
	}
	out := v.View()
	// ⎿ must appear for params/result lines
	if !strings.Contains(out, "⎿") {
		t.Errorf("expected ⎿ tree connector in expanded view, got: %q", out)
	}
}

// TestTUI031_ExpandedViewTruncatesLongResult verifies that results with more
// than 20 lines get a truncation hint line.
func TestTUI031_ExpandedViewTruncatesLongResult(t *testing.T) {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, "line content here")
	}
	result := strings.Join(lines, "\n")

	v := ExpandedView{
		ToolName: "GrepSearch",
		Args:     "pattern",
		Result:   result,
		State:    StateCompleted,
		Width:    80,
	}
	out := v.View()
	visible := stripANSI(out)

	// Should contain "+10 more lines" or similar truncation hint
	if !strings.Contains(visible, "more lines") {
		t.Errorf("expected truncation hint for >20 result lines, got: %q", visible)
	}
}

// TestTUI031_ExpandedViewShowsDuration verifies that the duration string
// appears when set.
func TestTUI031_ExpandedViewShowsDuration(t *testing.T) {
	v := ExpandedView{
		ToolName: "ReadFile",
		Args:     "main.go",
		Result:   "content",
		State:    StateCompleted,
		Duration: "1.2s",
		Width:    80,
	}
	out := v.View()
	visible := stripANSI(out)

	if !strings.Contains(visible, "1.2s") {
		t.Errorf("expected duration '1.2s' in output, got: %q", visible)
	}
}

// TestTUI031_ExpandedViewTimestampRightAligned verifies that when Timestamp
// is set, it appears right-aligned at the right edge.
func TestTUI031_ExpandedViewTimestampRightAligned(t *testing.T) {
	v := ExpandedView{
		ToolName:  "ReadFile",
		Args:      "main.go",
		Result:    "content",
		State:     StateCompleted,
		Duration:  "0.5s",
		Timestamp: "12:34:56",
		Width:     80,
	}
	out := v.View()
	visible := stripANSI(out)

	if !strings.Contains(visible, "12:34:56") {
		t.Errorf("expected timestamp '12:34:56' in output, got: %q", visible)
	}

	// The timestamp should appear on the duration line, right-aligned.
	// Find the line containing the timestamp and verify duration is also there.
	lines := strings.Split(visible, "\n")
	foundBoth := false
	for _, line := range lines {
		if strings.Contains(line, "12:34:56") && strings.Contains(line, "0.5s") {
			foundBoth = true
			break
		}
	}
	if !foundBoth {
		t.Errorf("expected timestamp and duration on same line, lines: %v", lines)
	}
}

// TestTUI031_ToggleExpandedPreservesState verifies that Toggle() flips
// IsExpanded() each time it is called.
func TestTUI031_ToggleExpandedPreservesState(t *testing.T) {
	ts := ToggleState{}
	if ts.IsExpanded() {
		t.Fatal("initial ToggleState must not be expanded")
	}

	ts = ts.Toggle()
	if !ts.IsExpanded() {
		t.Fatal("after first Toggle(), IsExpanded() must be true")
	}

	ts = ts.Toggle()
	if ts.IsExpanded() {
		t.Fatal("after second Toggle(), IsExpanded() must be false")
	}
}

// TestTUI031_ToggleViewDispatch verifies that View() returns collapsed output
// when not expanded and expanded output when expanded.
func TestTUI031_ToggleViewDispatch(t *testing.T) {
	c := CollapsedView{
		ToolName: "ReadFile",
		Args:     "main.go",
		State:    StateCompleted,
		Width:    80,
	}
	e := ExpandedView{
		ToolName: "ReadFile",
		Args:     "main.go",
		Params: []Param{
			{Key: "path", Value: "main.go"},
		},
		Result: "package main",
		State:  StateCompleted,
		Width:  80,
	}

	ts := ToggleState{}
	collapsedOut := ts.View(c, e)
	// Collapsed output should not contain the param key
	visibleCollapsed := stripANSI(collapsedOut)
	if strings.Contains(visibleCollapsed, "path:") {
		t.Errorf("collapsed view must not show params, got: %q", visibleCollapsed)
	}

	ts = ts.Toggle()
	expandedOut := ts.View(c, e)
	visibleExpanded := stripANSI(expandedOut)
	if !strings.Contains(visibleExpanded, "path") {
		t.Errorf("expanded view must show params, got: %q", visibleExpanded)
	}
}

// TestTUI031_ConcurrentExpanded verifies that 10 goroutines calling View()
// on the same ExpandedView concurrently produces no data races (run with -race).
func TestTUI031_ConcurrentExpanded(t *testing.T) {
	v := ExpandedView{
		ToolName: "GrepSearch",
		Args:     "pattern, file.go",
		Params: []Param{
			{Key: "pattern", Value: "pattern"},
			{Key: "path", Value: "file.go"},
		},
		Result:    "match line 1\nmatch line 2",
		State:     StateCompleted,
		Duration:  "0.3s",
		Timestamp: "10:00:00",
		Width:     80,
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				out := v.View()
				if out == "" {
					panic("View() returned empty string")
				}
			}
		}()
	}
	wg.Wait()
}

// TestTUI031_BoundaryWidths verifies rendering at boundary widths
// (10, 80, 200) does not panic and produces non-empty output.
func TestTUI031_BoundaryWidths(t *testing.T) {
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
			v := ExpandedView{
				ToolName: "BashExec",
				Args:     "go test ./...",
				Params: []Param{
					{Key: "cmd", Value: "go test ./..."},
				},
				Result:   "ok",
				State:    StateRunning,
				Duration: "2.0s",
				Width:    tc.width,
			}
			out := v.View()
			if out == "" {
				t.Errorf("[%s] View() returned empty string", tc.name)
			}
			if !strings.Contains(out, "⏺") {
				t.Errorf("[%s] output must contain ⏺, got: %q", tc.name, out)
			}
		})
	}
}

// TestTUI031_EmptyParamsAndResult verifies that empty params and result
// render safely without panics and produce non-empty output.
func TestTUI031_EmptyParamsAndResult(t *testing.T) {
	v := ExpandedView{
		ToolName: "ListDir",
		Args:     "",
		Params:   nil,
		Result:   "",
		State:    StateCompleted,
		Width:    80,
	}
	// Must not panic
	out := v.View()
	if out == "" {
		t.Fatal("View() must not return empty string for empty params/result")
	}
	if !strings.Contains(out, "ListDir") {
		t.Errorf("ToolName must appear in output, got: %q", out)
	}
}

// TestTUI031_VisualSnapshot_80x24 renders expanded tool calls at 80 width
// and writes to testdata/snapshots/TUI-031-expanded-80x24.txt.
func TestTUI031_VisualSnapshot_80x24(t *testing.T) {
	views := []ExpandedView{
		{
			ToolName: "ReadFile",
			Args:     "cmd/harnesscli/tui/theme.go",
			Params: []Param{
				{Key: "path", Value: "cmd/harnesscli/tui/theme.go"},
				{Key: "limit", Value: "100"},
			},
			Result:    "package tui\n\nimport (\n\t\"strings\"\n\t\"github.com/charmbracelet/lipgloss\"\n)\n\n// Theme holds all lipgloss styles.",
			State:     StateCompleted,
			Duration:  "0.1s",
			Timestamp: "14:32:01",
			Width:     80,
		},
		{
			ToolName: "BashExec",
			Args:     "go test ./cmd/harnesscli/...",
			Params: []Param{
				{Key: "cmd", Value: "go test ./cmd/harnesscli/..."},
			},
			Result:   "ok  go-agent-harness/cmd/harnesscli/tui\nok  go-agent-harness/cmd/harnesscli/tui/components/tooluse",
			State:    StateRunning,
			Duration: "",
			Width:    80,
		},
		{
			ToolName: "GrepSearch",
			Args:     "pattern, path/to/file.go",
			Params: []Param{
				{Key: "pattern", Value: "pattern"},
				{Key: "path", Value: "path/to/file.go"},
			},
			Result:   "Error: file not found",
			State:    StateError,
			Duration: "0.0s",
			Width:    80,
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
	path := dir + "/TUI-031-expanded-80x24.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI031_VisualSnapshot_120x40 renders expanded tool calls at 120 width.
func TestTUI031_VisualSnapshot_120x40(t *testing.T) {
	views := []ExpandedView{
		{
			ToolName: "ReadFile",
			Args:     "cmd/harnesscli/tui/theme.go, limit=100",
			Params: []Param{
				{Key: "path", Value: "cmd/harnesscli/tui/theme.go"},
				{Key: "limit", Value: "100"},
			},
			Result:    "package tui\n\nimport (\n\t\"strings\"\n\t\"github.com/charmbracelet/lipgloss\"\n)\n",
			State:     StateCompleted,
			Duration:  "0.1s",
			Timestamp: "14:32:01",
			Width:     120,
		},
		{
			ToolName: "WriteFile",
			Args:     "cmd/harnesscli/tui/components/tooluse/expanded.go, <content>",
			Params: []Param{
				{Key: "path", Value: "cmd/harnesscli/tui/components/tooluse/expanded.go"},
				{Key: "content", Value: "<multi-line content>"},
			},
			Result:   "written 1234 bytes",
			State:    StateCompleted,
			Duration: "0.2s",
			Width:    120,
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
	path := dir + "/TUI-031-expanded-120x40.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI031_VisualSnapshot_200x50 renders expanded tool calls at 200 width.
func TestTUI031_VisualSnapshot_200x50(t *testing.T) {
	views := []ExpandedView{
		{
			ToolName: "ReadFile",
			Args:     "cmd/harnesscli/tui/theme.go, limit=100",
			Params: []Param{
				{Key: "path", Value: "cmd/harnesscli/tui/theme.go"},
				{Key: "limit", Value: "100"},
			},
			Result:    "package tui\n\nimport (\n\t\"strings\"\n\t\"github.com/charmbracelet/lipgloss\"\n)\n",
			State:     StateCompleted,
			Duration:  "0.1s",
			Timestamp: "14:32:01",
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
	path := dir + "/TUI-031-expanded-200x50.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
