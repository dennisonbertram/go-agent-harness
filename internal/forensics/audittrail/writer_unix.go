//go:build unix

package audittrail

import (
	"fmt"
	"os"
	"syscall"
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
	fd, err := syscall.Open(path,
		syscall.O_WRONLY|syscall.O_CREAT|syscall.O_APPEND|
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

// lockFileExclusive acquires an exclusive advisory flock on f.
//
// HIGH-2 fix: sync.Mutex serializes writes within a single process but cannot
// protect against two processes appending to the same audit file concurrently
// (interleaved lines invalidate prev_hash, creating split-brain chains). An
// exclusive flock serializes cross-process writes at the OS level.
//
// Note: flock is advisory. Processes that do not call flock can still
// interleave writes. For stronger guarantees use per-run isolated audit files.
func lockFileExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlockFile releases the advisory flock on f acquired by lockFileExclusive.
func unlockFile(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
