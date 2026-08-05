//go:build !aix && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !solaris

package main

import (
	"fmt"
	"net"
)

func listenerFromInheritedFD(_, _ string) (net.Listener, error) {
	return nil, fmt.Errorf("HARNESS_LISTEN_FD is unsupported on this platform")
}
