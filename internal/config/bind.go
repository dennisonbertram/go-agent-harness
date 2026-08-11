package config

import (
	"net"
	"strings"
)

// IsPubliclyReachableAddr reports whether a listen address accepts connections
// from beyond the local machine.
//
// This gates the startup check that refuses an unauthenticated public bind
// (issue #1328), so the two failure directions are not symmetric: wrongly
// reporting "loopback" would let an exposed daemon skip the auth requirement,
// while wrongly reporting "public" only asks an operator to configure auth.
// An address we cannot parse is therefore treated as public — fail closed.
func IsPubliclyReachableAddr(addr string) bool {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		// Empty means the Go default of every interface.
		return true
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return true
	}
	host = strings.TrimSpace(host)
	if host == "" {
		// ":8080" — every interface.
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// A hostname we cannot resolve to a literal: assume it is routable.
		return true
	}
	if ip.IsUnspecified() {
		// 0.0.0.0 or ::
		return true
	}
	return !ip.IsLoopback()
}
