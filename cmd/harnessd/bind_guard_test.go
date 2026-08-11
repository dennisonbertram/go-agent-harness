package main

import "testing"

// TestPublicBindWithoutAuthIsRefused is the fail-closed half of issue #1328.
// Binding beyond loopback without authentication is the configuration that
// exposed an open agent-execution service to the local network, so it must be
// refused at startup rather than accepted silently.
func TestPublicBindWithoutAuthIsRefused(t *testing.T) {
	err := checkBindSafety("0.0.0.0:8080", false, false)
	if err == nil {
		t.Fatal("a public bind with no auth must be refused")
	}
	// The message has to tell an operator what to do, not just that it failed.
	for _, want := range []string{"HARNESS_AUTH_DISABLED", "HARNESS_ADDR"} {
		if !contains(err.Error(), want) {
			t.Errorf("error must mention %q so the operator knows the way out; got: %v", want, err)
		}
	}
}

// The remaining cases are the false-positive controls: the guard must block only
// the genuinely dangerous configuration.
func TestBindSafetyAllowsEverythingElse(t *testing.T) {
	for _, tc := range []struct {
		name         string
		addr         string
		authDisabled bool
		hasStore     bool
	}{
		{"loopback without auth is normal local development", "127.0.0.1:8080", false, false},
		{"loopback with the explicit opt-out", "127.0.0.1:8080", true, false},
		{"public bind with auth configured", "0.0.0.0:8080", false, true},
		{"public bind with an explicit opt-out", "0.0.0.0:8080", true, false},
		{"localhost by name", "localhost:8080", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkBindSafety(tc.addr, tc.authDisabled, tc.hasStore); err != nil {
				t.Errorf("checkBindSafety(%q, authDisabled=%v, hasStore=%v) = %v, want nil",
					tc.addr, tc.authDisabled, tc.hasStore, err)
			}
		})
	}
}

func contains(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}
