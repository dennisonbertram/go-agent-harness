package main

import (
	"fmt"

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
func checkBindSafety(addr string, authDisabled, hasAuthStore bool) error {
	if !config.IsPubliclyReachableAddr(addr) {
		return nil
	}
	if authDisabled || hasAuthStore {
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
