package thinkingbar

import "testing"

func TestNewAndViewStub(t *testing.T) {
	t.Parallel()

	m := New()
	if m.Active {
		t.Fatal("expected thinking bar to start inactive")
	}
	if m.Label != "" {
		t.Fatalf("expected empty default label, got %q", m.Label)
	}
	if got := m.View(); got != "" {
		t.Fatalf("expected stub view to return empty string, got %q", got)
	}
}
