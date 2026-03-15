package diffview

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// sampleDiff is the canonical test fixture used across all tests.
const sampleDiff = `--- a/main.go
+++ b/main.go
@@ -1,5 +1,7 @@
 package main

+import "fmt"
+
 func main() {
-    println("hello")
+    fmt.Println("hello world")
 }`

// TestTUI032_DiffViewerRendersHeadersAndHunks verifies @@ hunk header appears in output.
func TestTUI032_DiffViewerRendersHeadersAndHunks(t *testing.T) {
	v := View{Diff: sampleDiff, MaxLines: 0, Width: 80}
	out := v.Render()

	if !strings.Contains(out, "@@") {
		t.Errorf("expected @@ hunk header in output, got:\n%s", out)
	}
}

// TestTUI032_DiffViewerTruncatesLongDiff verifies that exceeding MaxLines triggers
// the truncation marker.
func TestTUI032_DiffViewerTruncatesLongDiff(t *testing.T) {
	// Build a diff with many lines
	var sb strings.Builder
	sb.WriteString("--- a/big.go\n+++ b/big.go\n@@ -1,50 +1,50 @@\n")
	for i := 0; i < 50; i++ {
		sb.WriteString(" context line\n")
	}

	v := View{Diff: sb.String(), MaxLines: 5, Width: 80}
	out := v.Render()

	if !strings.Contains(out, "more lines") {
		t.Errorf("expected truncation marker '[+N more lines]' in output, got:\n%s", out)
	}
}

// TestTUI032_DiffViewerAddLinesStyled verifies that + lines are present in the output.
func TestTUI032_DiffViewerAddLinesStyled(t *testing.T) {
	v := View{Diff: sampleDiff, MaxLines: 0, Width: 80}
	out := v.Render()

	// Strip ANSI escapes and check for + prefix
	plain := stripANSI(out)
	if !strings.Contains(plain, "+") {
		t.Errorf("expected + prefix for added lines in output, got:\n%s", plain)
	}
}

// TestTUI032_DiffViewerRemoveLinesStyled verifies that - lines are present in the output.
func TestTUI032_DiffViewerRemoveLinesStyled(t *testing.T) {
	v := View{Diff: sampleDiff, MaxLines: 0, Width: 80}
	out := v.Render()

	plain := stripANSI(out)
	if !strings.Contains(plain, "-") {
		t.Errorf("expected - prefix for removed lines in output, got:\n%s", plain)
	}
}

// TestTUI032_DiffViewerEmptyDiff verifies that empty input renders safely without panic.
func TestTUI032_DiffViewerEmptyDiff(t *testing.T) {
	v := View{Diff: "", MaxLines: 0, Width: 80}
	// Must not panic
	out := v.Render()
	// Output may be empty or minimal but must not panic
	_ = out
}

// TestTUI032_DiffViewerSingleLine verifies that a single-line diff renders correctly.
func TestTUI032_DiffViewerSingleLine(t *testing.T) {
	single := "+added line"
	v := View{Diff: single, MaxLines: 0, Width: 80}
	out := v.Render()
	plain := stripANSI(out)
	if !strings.Contains(plain, "+") {
		t.Errorf("single-line diff must contain + prefix, got:\n%s", plain)
	}
}

// TestTUI032_DiffViewerMalformedInput verifies that garbage input renders a fallback
// without panicking.
func TestTUI032_DiffViewerMalformedInput(t *testing.T) {
	garbage := "this is not a diff\x00\x01\x02"
	v := View{Diff: garbage, MaxLines: 0, Width: 80}
	// Must not panic
	out := v.Render()
	_ = out
}

// TestTUI032_DiffViewerConcurrent verifies 10 goroutines rendering the same diff
// have no data races.
func TestTUI032_DiffViewerConcurrent(t *testing.T) {
	v := View{Diff: sampleDiff, MaxLines: 40, Width: 80}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				out := v.Render()
				if out == "" && v.Diff != "" {
					// non-empty diff should produce output
					panic("Render() returned empty string for non-empty diff")
				}
			}
		}()
	}
	wg.Wait()
}

// TestTUI032_DiffViewerBoundaryWidths verifies rendering at width=20, 80, 200.
func TestTUI032_DiffViewerBoundaryWidths(t *testing.T) {
	cases := []int{20, 80, 200}
	for _, w := range cases {
		v := View{Diff: sampleDiff, MaxLines: 0, Width: w}
		out := v.Render()
		if out == "" {
			t.Errorf("width=%d: Render() returned empty string", w)
		}
	}
}

// TestTUI032_ParseUnifiedDiff verifies Parse() returns the correct DiffLine kinds.
func TestTUI032_ParseUnifiedDiff(t *testing.T) {
	lines := Parse(sampleDiff)

	if len(lines) == 0 {
		t.Fatal("Parse() returned no lines for non-empty diff")
	}

	// Find each expected kind
	kinds := map[Kind]bool{}
	for _, l := range lines {
		kinds[l.Kind] = true
	}

	if !kinds[KindHeader] {
		t.Error("expected KindHeader lines (--- / +++)")
	}
	if !kinds[KindHunk] {
		t.Error("expected KindHunk lines (@@)")
	}
	if !kinds[KindAdd] {
		t.Error("expected KindAdd lines (+)")
	}
	if !kinds[KindRemove] {
		t.Error("expected KindRemove lines (-)")
	}
	if !kinds[KindContext] {
		t.Error("expected KindContext lines (space prefix)")
	}
}

// TestTUI032_ParseReturnsNilOnEmpty verifies Parse() returns nil for empty input.
func TestTUI032_ParseReturnsNilOnEmpty(t *testing.T) {
	result := Parse("")
	if result != nil {
		t.Errorf("Parse(\"\") should return nil, got %v", result)
	}
}

// TestTUI032_ClipLines verifies Clip() trims at maxLines and returns truncation flag.
func TestTUI032_ClipLines(t *testing.T) {
	lines := Parse(sampleDiff)
	if len(lines) == 0 {
		t.Skip("no lines to clip")
	}

	// Clip to fewer than available
	clipped, truncated := Clip(lines, 3)
	if len(clipped) > 3 {
		t.Errorf("Clip() returned %d lines, expected at most 3", len(clipped))
	}
	if !truncated {
		t.Error("Clip() expected truncated=true when input exceeds maxLines")
	}

	// Clip with maxLines >= len: no truncation
	all, truncated2 := Clip(lines, len(lines)+100)
	if len(all) != len(lines) {
		t.Errorf("Clip() with large maxLines returned %d lines, expected %d", len(all), len(lines))
	}
	if truncated2 {
		t.Error("Clip() expected truncated=false when maxLines >= len(lines)")
	}

	// Clip with maxLines=0: no truncation
	all2, truncated3 := Clip(lines, 0)
	if len(all2) != len(lines) {
		t.Errorf("Clip(lines, 0) returned %d lines, expected %d", len(all2), len(lines))
	}
	if truncated3 {
		t.Error("Clip(lines, 0) expected truncated=false")
	}
}

// TestTUI032_VisualSnapshot_80x24 renders a diff at width=80 and writes snapshot.
func TestTUI032_VisualSnapshot_80x24(t *testing.T) {
	v := View{Diff: sampleDiff, MaxLines: 40, Width: 80}
	out := v.Render()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-032-diff-80x24.txt"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI032_VisualSnapshot_120x40 renders a diff at width=120 and writes snapshot.
func TestTUI032_VisualSnapshot_120x40(t *testing.T) {
	v := View{Diff: sampleDiff, MaxLines: 40, Width: 120}
	out := v.Render()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-032-diff-120x40.txt"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI032_VisualSnapshot_200x50 renders a diff at width=200 and writes snapshot.
func TestTUI032_VisualSnapshot_200x50(t *testing.T) {
	v := View{Diff: sampleDiff, MaxLines: 40, Width: 200}
	out := v.Render()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-032-diff-200x50.txt"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// stripANSI removes ANSI escape sequences for plain-text assertions.
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
