package diffview

import "testing"

func TestModelNewSetsFieldsAndViewIsStable(t *testing.T) {
	t.Parallel()

	m := New("main.go", "@@")
	if m.FilePath != "main.go" {
		t.Fatalf("unexpected file path: %q", m.FilePath)
	}
	if m.Diff != "@@" {
		t.Fatalf("unexpected diff: %q", m.Diff)
	}
	if got := m.View(); got != "" {
		t.Fatalf("expected stub view to return empty string, got %q", got)
	}
}
