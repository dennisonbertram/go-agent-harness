package diffview

import (
	"strings"
	"testing"
)

func TestModelNewSetsFieldsAndViewIsStable(t *testing.T) {
	t.Parallel()

	m := New("main.go", "@@")
	if m.FilePath != "main.go" {
		t.Fatalf("unexpected file path: %q", m.FilePath)
	}
	if m.Diff != "@@" {
		t.Fatalf("unexpected diff: %q", m.Diff)
	}
	m.Width = 80
	if got := stripANSI(m.View()); got == "" {
		t.Fatal("View() must render through the diff component")
	}
	if got := stripANSI(New("main.go", sampleDiff).View()); !strings.Contains(got, "main.go") || !strings.Contains(got, "@@") {
		t.Fatalf("View() must surface the rendered diff output, got %q", got)
	}
}
