//go:build unix

package rollout

import (
	"fmt"
	"os"
	"syscall"
)

// openRegularFile opens path for reading without blocking on FIFOs/devices,
// and verifies the opened file descriptor refers to a regular file.
// Using O_NONBLOCK prevents blocking if path is a FIFO with no writer,
// eliminating the TOCTOU race where Stat shows regular but open blocks on
// a swapped FIFO. O_NONBLOCK is cleared after the fd-level Stat so
// subsequent reads block normally.
func openRegularFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("rollout: stat fd %q: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("rollout: %q is not a regular file (mode: %s)", path, fi.Mode().Type())
	}
	// Clear O_NONBLOCK so subsequent reads block normally.
	if err := syscall.SetNonblock(fd, false); err != nil {
		f.Close()
		return nil, fmt.Errorf("rollout: clear O_NONBLOCK on %q: %w", path, err)
	}
	return f, nil
}
