package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"go-agent-harness/internal/config"
)

// checkBindSafety refuses to start an unauthenticated daemon that listens beyond
// loopback.
//
// Auth is implicitly off when no key store is configured, and the bind address
// used to default to every interface. Together those made a default daemon an
// open agent-execution service on the local network: anyone who could reach the
// port could start a run in the workspace using the daemon's provider
// credentials, then read the results (issue #1328).
//
// Rather than demand a token for every local workflow — friction that gets
// switched off, leaving operators worse off — this asks for one only when it
// matters. Loopback development is untouched; a public bind must either
// configure auth or opt out on purpose.
// explicitAuthOptOut must be the operator's own HARNESS_AUTH_DISABLED, not
// harnessd's derived authDisabled flag: that flag is set automatically when a run
// store is auto-created, so treating it as consent made this guard a no-op on the
// default configuration. authEnforced means requests are actually challenged.
func checkBindSafety(addr string, explicitAuthOptOut, authEnforced bool) error {
	if !config.IsPubliclyReachableAddr(addr) {
		return nil
	}
	if explicitAuthOptOut || authEnforced {
		return nil
	}
	return fmt.Errorf(
		"refusing to start: %s listens beyond this machine but no authentication is configured, "+
			"so anyone who can reach the port could start agent runs with this daemon's credentials. "+
			"Configure an API key store, or set HARNESS_AUTH_DISABLED=true to accept an open daemon "+
			"deliberately, or set HARNESS_ADDR=127.0.0.1:8080 to listen locally only",
		addr,
	)
}

// explicitAuthOptOut reports whether the operator set HARNESS_AUTH_DISABLED
// themselves. This is deliberately read here rather than taken from the derived
// authDisabled flag, which harnessd also sets on its own.
func explicitAuthOptOut() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("HARNESS_AUTH_DISABLED")), "true")
}

// selfBaseURL turns this daemon's listen address into a URL it can call itself
// on over loopback.
//
// The /mcp handler runs inside harnessd and reaches the REST API this way, so a
// wildcard or empty host has to become an explicit loopback host — "http://:8080"
// is not dialable.
func selfBaseURL(addr string) string {
	addr = strings.TrimSpace(addr)
	host, port, err := net.SplitHostPort(addr)
	if err != nil || strings.TrimSpace(port) == "" {
		return "http://127.0.0.1:8080"
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return "http://" + net.JoinHostPort(strings.Trim(host, "[]"), port)
}
