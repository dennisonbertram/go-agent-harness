//go:build unix

package audittrail

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// openAuditFile opens the audit log file securely on Unix platforms.
//
// CRITICAL-2 fix: O_NOFOLLOW prevents following symlinks at the final path
// component, blocking symlink-redirect attacks where an attacker pre-creates
// path as a symlink to an arbitrary target file (e.g., /etc/passwd). Without
// O_NOFOLLOW, a privileged harness process could overwrite sensitive files.
//
// After open, fstat verifies the fd refers to a regular file (not a FIFO,
// device, or socket), preventing DoS via blocking-on-open for special files.
func openAuditFile(path string) (*os.File, error) {
	// HIGH-1 fix (round 29): use O_RDWR so that readLastEntryHashFromFd can
	// seek and read the tail for chain resume via the same fd used for writing.
	// This eliminates the dual-open TOCTOU window (os.Open for reading, then
	// openAuditFile for writing) where a rename attack could cause chain-resume
	// to read file A's hash while all writes go to file B.
	fd, err := syscall.Open(path,
		syscall.O_RDWR|syscall.O_CREAT|syscall.O_APPEND|
			syscall.O_NOFOLLOW|syscall.O_CLOEXEC,
		0o600)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("audittrail: stat fd %q: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("audittrail: %q is not a regular file (mode: %s)", path, fi.Mode().Type())
	}
	return f, nil
}

// lockFileExclusive acquires an exclusive advisory flock on f with a timeout.
//
// HIGH-2 fix: sync.Mutex serializes writes within a single process but cannot
// protect against two processes appending to the same audit file concurrently
// (interleaved lines invalidate prev_hash, creating split-brain chains). An
// exclusive flock serializes cross-process writes at the OS level.
//
// HIGH-3 fix (round 30): LOCK_EX without LOCK_NB blocks the calling goroutine
// forever when a co-process holds the lock indefinitely. Using LOCK_NB with
// exponential-backoff retry up to lockTimeout prevents goroutine starvation
// while still serializing concurrent writers in the common (low-contention) case.
//
// Note: flock is advisory. Processes that do not call flock can still
// interleave writes. For stronger guarantees use per-run isolated audit files.
func lockFileExclusive(f *os.File) error {
	const lockTimeout = 30 * time.Second
	deadline := time.Now().Add(lockTimeout)
	sleep := time.Millisecond
	fd := int(f.Fd())
	for {
		err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if err != syscall.EWOULDBLOCK {
			return &os.PathError{Op: "flock", Path: f.Name(), Err: err}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("audittrail: timed out waiting for exclusive lock on %s after %s", f.Name(), lockTimeout)
		}
		time.Sleep(sleep)
		if sleep < 500*time.Millisecond {
			sleep *= 2
		}
	}
}

// unlockFile releases the advisory flock on f acquired by lockFileExclusive.
// Returns an error if the unlock syscall fails (e.g., EBADF on a closed fd),
// allowing callers to propagate the failure rather than silently leaving peer
// processes stalled on LOCK_EX acquisition indefinitely.
func unlockFile(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		return &os.PathError{Op: "flock(LOCK_UN)", Path: f.Name(), Err: err}
	}
	return nil
}
