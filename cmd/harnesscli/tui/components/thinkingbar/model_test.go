package thinkingbar

import "testing"

func TestNewStartsInactive(t *testing.T) {
	t.Parallel()

	m := New()
	if m.Active {
		t.Fatal("expected thinking bar to start inactive")
	}
	if got := m.View(); got != "" {
		t.Fatalf("expected inactive view to return empty string, got %q", got)
	}
}

func TestViewReturnsDefaultLabelWhenActiveAndUnset(t *testing.T) {
	t.Parallel()

	m := Model{Active: true}
	if got := m.View(); got != "Thinking..." {
		t.Fatalf("expected default active label, got %q", got)
	}
}

func TestViewReturnsCustomLabelWhenActive(t *testing.T) {
	t.Parallel()

	m := Model{Active: true, Label: "Reasoning"}
	if got := m.View(); got != "Reasoning..." {
		t.Fatalf("expected custom active label, got %q", got)
	}
}

func TestViewActiveIsNeverEmpty(t *testing.T) {
	t.Parallel()

	if got := (Model{Active: true}).View(); got == "" {
		t.Fatal("expected active thinking bar view to be non-empty")
	}
}
