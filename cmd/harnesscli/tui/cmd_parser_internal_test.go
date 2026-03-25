package tui

import "testing"

func TestNewEmptyCommandRegistryStartsEmpty(t *testing.T) {
	t.Parallel()

	reg := newEmptyCommandRegistry()
	if reg == nil {
		t.Fatal("expected registry")
	}
	if reg.index == nil {
		t.Fatal("expected index map to be initialized")
	}
	if len(reg.index) != 0 {
		t.Fatalf("expected empty index, got %d entries", len(reg.index))
	}
	if len(reg.entries) != 0 {
		t.Fatalf("expected no pre-registered entries, got %d", len(reg.entries))
	}
}
