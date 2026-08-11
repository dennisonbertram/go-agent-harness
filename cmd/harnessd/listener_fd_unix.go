//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// listenerFromInheritedFD adopts the descriptor passed by a trusted parent.
// It intentionally rejects malformed, non-TCP, and address-mismatched files:
// accepting any open FD would let an ambient process choose the health server.
func listenerFromInheritedFD(rawFD, expectedAddress string) (net.Listener, error) {
	fd, err := strconv.Atoi(strings.TrimSpace(rawFD))
	if err != nil || fd < 3 {
		return nil, fmt.Errorf("must name an inherited descriptor >= 3")
	}
	file := os.NewFile(uintptr(fd), "HARNESS_LISTEN_FD")
	if file == nil {
		return nil, fmt.Errorf("cannot access inherited descriptor %d", fd)
	}
	listener, err := net.FileListener(file)
	_ = file.Close()
	if err != nil {
		return nil, fmt.Errorf("adopt inherited descriptor %d: %w", fd, err)
	}
	if _, ok := listener.(*net.TCPListener); !ok {
		_ = listener.Close()
		return nil, fmt.Errorf("inherited descriptor %d is not a TCP listener", fd)
	}
	if listener.Addr().String() != expectedAddress {
		actual := listener.Addr().String()
		_ = listener.Close()
		return nil, fmt.Errorf("inherited listener address %q does not match HARNESS_ADDR %q", actual, expectedAddress)
	}
	return listener, nil
}
