//go:build !unix

package rollout

import "os"

// openRegularFile opens path for reading. On non-Unix systems, O_NONBLOCK is
// not available so this falls back to os.Open; the Stat-based IsRegular check
// in LoadFile still applies.
func openRegularFile(path string) (*os.File, error) {
	return os.Open(path)
}
