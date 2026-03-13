//go:build !unix

package rollout

import (
	"fmt"
	"os"
)

// openRegularFile opens path for reading and verifies the opened fd refers
// to a regular file. On non-Unix systems O_NONBLOCK is unavailable, so the
// TOCTOU race between Stat and Open cannot be fully eliminated; however, the
// fstat-after-open check catches the case where a swap already occurred
// before os.Open() was called.
func openRegularFile(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("rollout: stat fd %q: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("rollout: %q is not a regular file (mode: %s)", path, fi.Mode().Type())
	}
	return f, nil
}
