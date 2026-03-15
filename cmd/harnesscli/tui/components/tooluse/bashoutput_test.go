package tooluse

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// TestTUI035_BashOutputTruncatesLongText verifies that output with more than
// MaxLines gets a "+N more lines (ctrl+o to expand)" hint.
func TestTUI035_BashOutputTruncatesLongText(t *testing.T) {
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "output line here")
	}
	output := strings.Join(lines, "\n")

	b := BashOutput{
		Command:  "echo test",
		Output:   output,
		MaxLines: 10,
		Width:    80,
	}
	out := b.View()
	visible := StripANSI(out)

	if !strings.Contains(visible, "more lines") {
		t.Errorf("expected truncation hint for >MaxLines output, got: %q", visible)
	}
	if !strings.Contains(visible, "ctrl+o to expand") {
		t.Errorf("expected 'ctrl+o to expand' in truncation hint, got: %q", visible)
	}
}

// TestTUI035_BashOutputShowsCommandLabel verifies that the command line
// is prefixed with "$ command" when Command is non-empty.
func TestTUI035_BashOutputShowsCommandLabel(t *testing.T) {
	b := BashOutput{
		Command: "echo hello",
		Output:  "hello",
		Width:   80,
	}
	out := b.View()
	visible := StripANSI(out)

	if !strings.Contains(visible, "$ echo hello") {
		t.Errorf("expected '$ echo hello' command label, got: %q", visible)
	}
	if !strings.Contains(visible, "hello") {
		t.Errorf("expected output 'hello' in view, got: %q", visible)
	}
}

// TestTUI035_BashOutputStripsANSI verifies that ANSI escape sequences in
// the output are removed before rendering.
func TestTUI035_BashOutputStripsANSI(t *testing.T) {
	ansiOutput := "\x1b[32mgreen text\x1b[0m and \x1b[31mred text\x1b[0m"
	b := BashOutput{
		Command: "ls --color",
		Output:  ansiOutput,
		Width:   80,
	}
	out := b.View()
	// The raw output should not contain ANSI escapes (they should be stripped)
	if strings.Contains(out, "\x1b[32m") {
		t.Errorf("expected ANSI codes to be stripped, got raw ANSI in output: %q", out)
	}
	if strings.Contains(out, "\x1b[31m") {
		t.Errorf("expected ANSI codes to be stripped, got raw ANSI in output: %q", out)
	}
	// But the text content should remain
	if !strings.Contains(StripANSI(out), "green text") {
		t.Errorf("expected 'green text' after ANSI stripping, got: %q", StripANSI(out))
	}
}

// TestTUI035_BashOutputEmptyOutput verifies that empty output does not panic
// and produces some output (at least the command label if present).
func TestTUI035_BashOutputEmptyOutput(t *testing.T) {
	b := BashOutput{
		Command: "true",
		Output:  "",
		Width:   80,
	}
	// Must not panic
	out := b.View()
	if out == "" {
		t.Fatal("View() must not return empty string for empty output")
	}
}

// TestTUI035_BashOutputOneLineOutput verifies that single-line output renders
// without a truncation hint.
func TestTUI035_BashOutputOneLineOutput(t *testing.T) {
	b := BashOutput{
		Command:  "echo hello",
		Output:   "hello",
		MaxLines: 10,
		Width:    80,
	}
	out := b.View()
	visible := StripANSI(out)

	if strings.Contains(visible, "more lines") {
		t.Errorf("single-line output must not have truncation hint, got: %q", visible)
	}
	if !strings.Contains(visible, "hello") {
		t.Errorf("output content must appear, got: %q", visible)
	}
}

// TestTUI035_BashOutputConcurrent verifies that 10 goroutines calling View()
// concurrently produces no data races (run with -race).
func TestTUI035_BashOutputConcurrent(t *testing.T) {
	b := BashOutput{
		Command:  "go test ./...",
		Output:   "ok  github.com/foo/bar\nok  github.com/foo/baz\nFAIL github.com/foo/qux",
		MaxLines: 10,
		Width:    80,
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				out := b.View()
				if out == "" {
					panic("View() returned empty string")
				}
			}
		}()
	}
	wg.Wait()
}

// TestTUI035_BashOutputBoundaryWidths verifies rendering at boundary widths
// (5, 80, 200) does not panic and produces non-empty output.
func TestTUI035_BashOutputBoundaryWidths(t *testing.T) {
	cases := []struct {
		name  string
		width int
	}{
		{"width_5", 5},
		{"width_80", 80},
		{"width_200", 200},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := BashOutput{
				Command: "ls -la",
				Output:  "file1.go\nfile2.go\nfile3.go",
				Width:   tc.width,
			}
			// Must not panic
			out := b.View()
			if out == "" {
				t.Errorf("[%s] View() returned empty string", tc.name)
			}
		})
	}
}

// TestTUI035_BashOutputNoCommandLabel verifies that when Command is empty,
// no "$ " label line is emitted.
func TestTUI035_BashOutputNoCommandLabel(t *testing.T) {
	b := BashOutput{
		Command: "",
		Output:  "some output",
		Width:   80,
	}
	out := b.View()
	visible := StripANSI(out)

	if strings.Contains(visible, "$ ") {
		t.Errorf("empty Command must not emit '$ ' label, got: %q", visible)
	}
	if !strings.Contains(visible, "some output") {
		t.Errorf("output content must appear, got: %q", visible)
	}
}

// TestTUI035_BashOutputDefaultMaxLines verifies that MaxLines=0 defaults to 10.
func TestTUI035_BashOutputDefaultMaxLines(t *testing.T) {
	// Build 15 lines of output
	var lines []string
	for i := 0; i < 15; i++ {
		lines = append(lines, "line")
	}
	output := strings.Join(lines, "\n")

	b := BashOutput{
		Command:  "cmd",
		Output:   output,
		MaxLines: 0, // should default to 10
		Width:    80,
	}
	out := b.View()
	visible := StripANSI(out)

	// With 15 lines and default MaxLines=10, should see truncation hint
	if !strings.Contains(visible, "more lines") {
		t.Errorf("MaxLines=0 should default to 10, expected truncation hint for 15 lines, got: %q", visible)
	}
}

// TestTUI035_VisualSnapshot_80x24 renders BashOutput at width 80 and writes
// snapshot to testdata/snapshots/TUI-035-bash-80x24.txt.
func TestTUI035_VisualSnapshot_80x24(t *testing.T) {
	views := []BashOutput{
		{
			Command:  "go test ./cmd/harnesscli/...",
			Output:   "ok  go-agent-harness/cmd/harnesscli/tui\nok  go-agent-harness/cmd/harnesscli/tui/components/tooluse\nok  go-agent-harness/cmd/harnesscli/tui/components/spinner\nok  go-agent-harness/cmd/harnesscli",
			MaxLines: 10,
			Width:    80,
		},
		{
			Command:  "ls -la /var/log",
			Output:   "total 128\n-rw-r--r-- 1 root root 1234 Mar 14 10:00 syslog\n-rw-r--r-- 1 root root 5678 Mar 14 09:00 auth.log",
			MaxLines: 10,
			Width:    80,
		},
		{
			Command: "",
			Output:  "stdout line 1\nstdout line 2\nstdout line 3",
			Width:   80,
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
	path := dir + "/TUI-035-bash-80x24.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI035_VisualSnapshot_120x40 renders BashOutput at width 120.
func TestTUI035_VisualSnapshot_120x40(t *testing.T) {
	var manyLines []string
	for i := 1; i <= 25; i++ {
		manyLines = append(manyLines, "output line "+strings.Repeat("x", i))
	}

	views := []BashOutput{
		{
			Command:  "go build ./...",
			Output:   strings.Join(manyLines, "\n"),
			MaxLines: 10,
			Width:    120,
		},
		{
			Command: "echo hello world",
			Output:  "hello world",
			Width:   120,
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
	path := dir + "/TUI-035-bash-120x40.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI035_VisualSnapshot_200x50 renders BashOutput at width 200.
func TestTUI035_VisualSnapshot_200x50(t *testing.T) {
	views := []BashOutput{
		{
			Command:  "go test ./... -race -v",
			Output:   "=== RUN   TestFoo\n--- PASS: TestFoo (0.00s)\n=== RUN   TestBar\n--- PASS: TestBar (0.00s)\nPASS\nok  go-agent-harness/internal/harness",
			MaxLines: 10,
			Width:    200,
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
	path := dir + "/TUI-035-bash-200x50.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
