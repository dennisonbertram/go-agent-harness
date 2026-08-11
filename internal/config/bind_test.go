package config

import "testing"

// TestDefaultAddrIsLoopback is part of the regression for issue #1328.
//
// The default was ":8080" — every interface — and auth is implicitly off when no
// key store is configured, so a default daemon was an open agent-execution
// service on whatever network the machine was attached to.
func TestDefaultAddrIsLoopback(t *testing.T) {
	got := Defaults().Addr
	if got != "127.0.0.1:8080" {
		t.Errorf("default Addr = %q, want 127.0.0.1:8080 — a wildcard bind exposes the daemon to the local network", got)
	}
}

// TestIsPubliclyReachableAddr classifies bind addresses. Getting this wrong in
// either direction is harmful: a false "loopback" would let a public bind skip
// the auth requirement, and a false "public" would block local development.
func TestIsPubliclyReachableAddr(t *testing.T) {
	for _, tc := range []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8080", false},
		{"localhost:8080", false},
		{"[::1]:8080", false},
		{"127.0.0.1:0", false},
		{":8080", true},
		{"0.0.0.0:8080", true},
		{"[::]:8080", true},
		{"192.168.0.103:8080", true},
		{"example.com:8080", true},
		// An unparseable address is treated as public: failing closed is the
		// safe direction when we cannot tell.
		{"garbage", true},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			if got := IsPubliclyReachableAddr(tc.addr); got != tc.want {
				t.Errorf("IsPubliclyReachableAddr(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}
