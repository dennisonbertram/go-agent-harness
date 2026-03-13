//go:build !unix

package rollout

import (
	"fmt"
	"os"
	"time"
)

// openRegularFile opens path for reading on non-Unix systems.
// Since O_NONBLOCK is not available, a goroutine with a 5-second timeout is
// used to prevent indefinitely blocking on a FIFO or other blocking special
// file. If the open blocks past the deadline, the goroutine is abandoned
// (it will be collected when the process exits — acceptable for a short-lived
// CLI tool) and an error is returned. A fstat-after-open then confirms the
// file is regular before returning, catching static swaps.
func openRegularFile(path string) (*os.File, error) {
	type openResult struct {
		f   *os.File
		err error
	}
	ch := make(chan openResult, 1)
	go func() {
		f, err := os.Open(path)
		ch <- openResult{f, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		fi, err := r.f.Stat()
		if err != nil {
			r.f.Close()
			return nil, fmt.Errorf("rollout: stat fd %q: %w", path, err)
		}
		if !fi.Mode().IsRegular() {
			r.f.Close()
			return nil, fmt.Errorf("rollout: %q is not a regular file (mode: %s)", path, fi.Mode().Type())
		}
		return r.f, nil
	case <-time.After(5 * time.Second):
		// The goroutine may eventually unblock and send on ch after the
		// timeout fires. Spin a cleanup receiver to drain the channel and
		// close any returned *os.File, preventing the fd leak.
		// The goroutine itself is leaked (os.Open cannot be canceled on
		// non-Unix without platform-specific syscalls), but it is bounded
		// to at most one leaked goroutine per timed-out call.
		go func() {
			if r := <-ch; r.f != nil {
				r.f.Close()
			}
		}()
		return nil, fmt.Errorf("rollout: open %q timed out (possible FIFO or blocked device)", path)
	}
}
