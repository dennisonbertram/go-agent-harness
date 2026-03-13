//go:build !unix

package rollout

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// maxPendingOpenTimeouts caps how many os.Open goroutines can be blocked
// (pending timeout) simultaneously. Each timed-out call leaves one goroutine
// blocked on os.Open until the target path eventually unblocks (which may
// never happen for named pipes with no writer). Without a cap, repeated calls
// on blocking paths would accumulate unbounded goroutines. After this limit
// is reached, further calls return an error immediately.
const maxPendingOpenTimeouts = 3

// pendingOpenTimeouts is the count of timed-out goroutines currently blocked
// inside os.Open. It is decremented when the goroutine eventually unblocks.
var pendingOpenTimeouts int32

// openRegularFile opens path for reading on non-Unix systems.
// Since O_NONBLOCK is not available, a goroutine with a 5-second timeout is
// used to prevent indefinitely blocking on a FIFO or other blocking special
// file. A hard cap (maxPendingOpenTimeouts) on concurrently blocked goroutines
// is enforced to prevent resource exhaustion from repeated calls on blocking
// paths. A fstat-after-open then confirms the file is regular before returning.
func openRegularFile(path string) (*os.File, error) {
	// Fail fast if too many prior timed-out opens are still blocking.
	if atomic.LoadInt32(&pendingOpenTimeouts) >= maxPendingOpenTimeouts {
		return nil, fmt.Errorf("rollout: open %q rejected: %d prior open attempt(s) still pending (too many blocked opens)", path, maxPendingOpenTimeouts)
	}

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
		// Increment the pending counter. The cleanup goroutine decrements it
		// when os.Open eventually unblocks, then closes any returned fd.
		atomic.AddInt32(&pendingOpenTimeouts, 1)
		go func() {
			if r := <-ch; r.f != nil {
				r.f.Close()
			}
			atomic.AddInt32(&pendingOpenTimeouts, -1)
		}()
		return nil, fmt.Errorf("rollout: open %q timed out (possible FIFO or blocked device)", path)
	}
}
