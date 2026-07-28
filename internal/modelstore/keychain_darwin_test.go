//go:build darwin

package modelstore

import (
	"context"
	"strings"
	"testing"
)

// Exercises the real macOS Keychain. The secret goes in over stdin, so it must
// never appear in the process list — that is the reason this path exists
// rather than passing it as a command argument.
func TestKeychainRoundTripAgainstRealKeychain(t *testing.T) {
	if !KeychainAvailable() {
		t.Skip("security(1) not available")
	}
	ref := KeychainRef("modelstore-selftest")
	t.Cleanup(func() { _ = DeleteCredential(context.Background(), ref) })

	const secret = "sk-test-value-do-not-reuse"
	if err := StoreCredential(context.Background(), ref, secret); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := ResolveCredential(context.Background(), ref)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != secret {
		t.Fatalf("round trip returned %q, want %q", got, secret)
	}

	// Overwriting must update in place rather than fail on a duplicate.
	if err := StoreCredential(context.Background(), ref, "sk-rotated"); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if got, _ := ResolveCredential(context.Background(), ref); got != "sk-rotated" {
		t.Fatalf("rotation did not take effect: %q", got)
	}

	if err := DeleteCredential(context.Background(), ref); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := ResolveCredential(context.Background(), ref); err == nil {
		t.Fatal("the entry is still readable after deletion")
	} else if !strings.Contains(err.Error(), "no keychain entry") {
		t.Fatalf("expected a clear not-found error, got: %v", err)
	}
}
