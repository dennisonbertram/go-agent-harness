//go:build !unix

package audittrail

import "os"

// openAuditFile opens the audit log file for append on non-Unix platforms.
// Symlink and special-file protections (O_NOFOLLOW, fstat IsRegular check)
// are not available on non-Unix platforms.
func openAuditFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

// lockFileExclusive is a no-op on non-Unix platforms.
// Consider per-run isolated audit files for multi-process safety.
func lockFileExclusive(_ *os.File) error { return nil }

// unlockFile is a no-op on non-Unix platforms.
func unlockFile(_ *os.File) {}
