package tui

import "testing"

func TestNewEmptyCommandRegistryAllStartsEmpty(t *testing.T) {
	t.Parallel()

	r := newEmptyCommandRegistry()

	if got := r.All(); len(got) != 0 {
		t.Fatalf("All() length = %d, want 0", len(got))
	}
	if _, ok := r.Lookup("clear"); ok {
		t.Fatal("Lookup(clear) should fail for an empty registry")
	}
	if r.IsRegistered("clear") {
		t.Fatal("IsRegistered(clear) should be false for an empty registry")
	}
}
