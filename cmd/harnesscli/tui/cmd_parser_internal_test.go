package tui

import "testing"

func TestNewEmptyCommandRegistryStartsEmpty(t *testing.T) {
	t.Parallel()

	registry := newEmptyCommandRegistry()
	if registry == nil {
		t.Fatal("newEmptyCommandRegistry returned nil")
	}
	if len(registry.entries) != 0 {
		t.Fatalf("entries len = %d, want 0", len(registry.entries))
	}
	if len(registry.index) != 0 {
		t.Fatalf("index len = %d, want 0", len(registry.index))
	}
	if registry.IsRegistered("help") {
		t.Fatal("empty registry should not report built-in commands as registered")
	}
}
