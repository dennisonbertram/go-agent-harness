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

// TestPublicBindWithImplicitlyDisabledAuthIsRefused is the case the first version
// of this guard missed.
//
// harnessd sets its authDisabled flag automatically when it auto-creates a run
// store without an explicit HARNESS_RUN_DB, so that default local installs do not
// suddenly require API keys. Passing that derived flag in as "the operator opted
// out" meant the guard never fired on a default configuration — the daemon
// started wide open on 0.0.0.0 exactly as before. Only an explicit
// HARNESS_AUTH_DISABLED counts as consent.
func TestPublicBindWithImplicitlyDisabledAuthIsRefused(t *testing.T) {
	// explicitOptOut=false, authEnforced=false: auth is off, but nobody asked for it.
	if err := checkBindSafety("0.0.0.0:8080", false, false); err == nil {
		t.Fatal("a public bind with implicitly-disabled auth must be refused")
	}
}

// The remaining cases are the false-positive controls: the guard must block only
// the genuinely dangerous configuration.
func TestBindSafetyAllowsEverythingElse(t *testing.T) {
	for _, tc := range []struct {
		name           string
		addr           string
		explicitOptOut bool
		authEnforced   bool
	}{
		{"loopback without auth is normal local development", "127.0.0.1:8080", false, false},
		{"loopback with the explicit opt-out", "127.0.0.1:8080", true, false},
		{"public bind with auth actually enforced", "0.0.0.0:8080", false, true},
		{"public bind with an explicit opt-out", "0.0.0.0:8080", true, false},
		{"localhost by name", "localhost:8080", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkBindSafety(tc.addr, tc.explicitOptOut, tc.authEnforced); err != nil {
				t.Errorf("checkBindSafety(%q, explicitOptOut=%v, authEnforced=%v) = %v, want nil",
					tc.addr, tc.explicitOptOut, tc.authEnforced, err)
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

// TestSelfBaseURL — the /mcp handler calls the daemon over loopback, so a
// wildcard or empty host must become a dialable one. "http://:8080" is not.
func TestSelfBaseURL(t *testing.T) {
	for _, tc := range []struct{ addr, want string }{
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{":8080", "http://127.0.0.1:8080"},
		{"0.0.0.0:9000", "http://127.0.0.1:9000"},
		{"[::]:9000", "http://127.0.0.1:9000"},
		{"192.168.0.5:8080", "http://192.168.0.5:8080"},
		{"garbage", "http://127.0.0.1:8080"},
		{"", "http://127.0.0.1:8080"},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			if got := selfBaseURL(tc.addr); got != tc.want {
				t.Errorf("selfBaseURL(%q) = %q, want %q", tc.addr, got, tc.want)
			}
		})
	}
}
