package tui

import "testing"

func TestNewEmptyCommandRegistryStartsEmpty(t *testing.T) {
	r := newEmptyCommandRegistry()
	if r == nil {
		t.Fatal("expected registry")
	}
	if len(r.All()) != 0 {
		t.Fatalf("expected empty registry, got %d entries", len(r.All()))
	}
	if r.IsRegistered("help") {
		t.Fatal("empty registry should not report built-ins as registered")
	}
}
