package audittrail

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditRecord is the caller-supplied input for a single audit log entry.
type AuditRecord struct {
	// RunID identifies the run this event belongs to.
	RunID string
	// EventType is the event type string, e.g. "run.started" or "audit.action".
	EventType string
	// Payload contains event-specific data. May be nil.
	Payload map[string]any
	// Timestamp, if non-zero, overrides the wall-clock time used for hashing
	// and the on-disk timestamp. Useful for deterministic tests.
	// When zero, time.Now().UTC() is used.
	Timestamp time.Time
}

// AuditEntry is the on-disk JSON representation of one audit log line.
type AuditEntry struct {
	// Timestamp is when the entry was written.
	Timestamp time.Time `json:"timestamp"`
	// RunID identifies the run.
	RunID string `json:"run_id"`
	// EventType is the event type string.
	EventType string `json:"event_type"`
	// Payload contains event-specific data. May be nil/omitted.
	Payload map[string]any `json:"payload,omitempty"`
	// PrevHash is the EntryHash of the previous entry, or "genesis" for the first.
	PrevHash string `json:"prev_hash"`
	// EntryHash is hex-encoded SHA-256 of the JSON-encoded auditHashPreimage struct.
	// Using a JSON struct avoids concatenation collisions where shifting characters
	// between adjacent fields produces an identical hash input.
	//
	// WARNING: The hash chain provides tamper DETECTION, not tamper PREVENTION.
	// Anyone with write access to the audit file can rewrite the chain. For
	// stronger guarantees, anchor the chain externally (HMAC with a key stored
	// outside the file, or publish the chain root to an immutable log).
	EntryHash string `json:"entry_hash"`
}

// auditHashPreimage is the canonical hash input for each audit entry.
// JSON encoding provides unambiguous field separation, preventing concatenation
// collisions where characters shift between adjacent fields (CRITICAL-1 fix).
type auditHashPreimage struct {
	Timestamp string `json:"ts"`
	RunID     string `json:"run_id"`
	EventType string `json:"event_type"`
	Payload   string `json:"payload_json"`
	PrevHash  string `json:"prev_hash"`
}

// AuditWriter writes an append-only, hash-chained JSONL audit log.
// It is safe for concurrent use.
type AuditWriter struct {
	mu       sync.Mutex
	file     *os.File
	enc      *json.Encoder
	lastHash string // hash of the last written entry ("genesis" before first write)
	closed   bool
}

// maxPayloadBytes caps the marshaled payload size per entry to prevent
// a single oversized entry from causing DoS on chain resume at startup.
// readLastEntryHash reads up to maxAuditTailBytes (4 MiB); entries close to
// that limit cause slow or failed resume. Capping at 2 MiB provides headroom.
//
// HIGH-3 fix: without this cap, a caller can write an arbitrarily large
// payload, producing an entry that exceeds maxAuditTailBytes and permanently
// prevents chain resume (the writer fails closed on every subsequent startup).
const maxPayloadBytes = 2 * 1024 * 1024 // 2 MiB

// NewAuditWriter creates an AuditWriter that appends to the JSONL file at
// the given path. The parent directory is created if it does not exist.
// If the file already contains entries, the hash chain is resumed from the
// last valid entry — appending with lastHash="genesis" would create a second
// chain starting mid-file, undermining tamper evidence for the full log.
// If the file exists but its last line is unreadable or lacks an entry_hash,
// NewAuditWriter fails closed to prevent silent chain corruption.
// Call Close when done to flush and release the file handle.
func NewAuditWriter(path string) (*AuditWriter, error) {
	dir := filepath.Dir(path)
	// CRITICAL-1 fix: restrict directory permissions to owner-only.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("audittrail: create directory %s: %w", dir, err)
	}
	// CRITICAL-3 fix: chmod existing directories too. MkdirAll only sets
	// permissions on newly-created directories; a pre-existing 0755 directory
	// remains 0755. Fail closed if the directory is world-accessible and chmod
	// fails (e.g., we don't own it).
	if err := os.Chmod(dir, 0o700); err != nil {
		if fi, statErr := os.Stat(dir); statErr == nil {
			if fi.Mode().Perm()&0o005 != 0 {
				return nil, fmt.Errorf("audittrail: directory %s has world-accessible permissions and chmod failed: %w", dir, err)
			}
		}
	}

	// CRITICAL-2 fix: use openAuditFile (OS-specific) to prevent symlink
	// attacks and verify the path refers to a regular file. On Unix this uses
	// O_NOFOLLOW + fstat; on non-Unix it falls back to os.OpenFile.
	//
	// HIGH-1 fix (round 29): open the file BEFORE reading the chain-resume
	// hash to eliminate the TOCTOU window. The previous code called os.Stat +
	// readLastEntryHash (both opening path via os.Open) and then openAuditFile
	// — a rename attack between the two opens could cause chain-resume to read
	// file A's hash while writes go to file B. By opening once and reading via
	// the same fd, both operations reference the same inode.
	f, err := openAuditFile(path)
	if err != nil {
		return nil, fmt.Errorf("audittrail: open file %s: %w", path, err)
	}

	// CRITICAL-3 fix: chmod the file after opening to correct pre-existing
	// too-permissive modes. OpenFile only sets 0o600 for new files; an existing
	// 0644 file is opened as-is. Fail closed if the file is world-readable and
	// chmod fails.
	if err := f.Chmod(0o600); err != nil {
		if fi, statErr := f.Stat(); statErr == nil {
			if fi.Mode().Perm()&0o044 != 0 {
				f.Close()
				return nil, fmt.Errorf("audittrail: file %s has world-readable permissions and chmod failed: %w", path, err)
			}
		}
	}

	// Resume hash chain from last entry via the already-open fd.
	// HIGH-1 fix: using the same fd ensures chain-resume reads the same inode
	// that will be written to, eliminating the rename-based TOCTOU.
	lastHash, err := readLastEntryHashFromFd(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("audittrail: resume chain from %s: %w", path, err)
	}

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	return &AuditWriter{
		file:     f,
		enc:      enc,
		lastHash: lastHash,
	}, nil
}

// deepCopyPayload returns a deep copy of a map[string]any, recursing into
// nested maps and slices. This prevents the caller from mutating nested
// structures after Write returns and causing hash/content mismatches.
//
// HIGH-1 fix: the previous shallow copy shared references to nested maps
// and slices with the caller. A concurrent mutation of nested data between
// json.Marshal (hash input) and enc.Encode (written bytes) produces an entry
// whose entry_hash does not match its content, breaking the chain.
func deepCopyPayload(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyPayload(val)
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = deepCopyValue(elem)
		}
		return out
	default:
		// Primitive types (string, bool, float64, nil) are immutable in Go.
		return v
	}
}

// maxAuditTailBytes is the maximum number of bytes read from the end of the
// audit file to find the last entry. Any single audit entry is bounded by its
// payload (≤64KiB) plus fixed field overhead; 4MiB is ample for any realistic
// entry while bounding the startup read size. (HIGH-6 fix: scanner-based
// reading with a fixed 1MiB buffer fails on entries larger than the limit.)
const maxAuditTailBytes = 4 * 1024 * 1024

// readLastEntryHashFromFd reads the last entry hash from an already-open file
// descriptor by seeking to the tail and parsing the last JSONL line.
//
// HIGH-1 fix (round 29): using the same fd for chain-resume reading and writing
// eliminates the TOCTOU window. The previous pattern opened a separate os.Open
// handle to read the tail, then a second handle via openAuditFile for writing;
// a rename between the two opens could cause chain-resume to read file A's hash
// while writes target file B's inode — silent broken chain.
//
// HIGH-5 fix (round 29): called under flock in Write() to get the true on-disk
// prevHash before each write. Two AuditWriter processes sharing the same file
// both resume with the same in-memory lastHash; flock serializes byte writes but
// cannot synchronize in-memory lastHash values. Re-reading under flock ensures
// each write sees the chain tail as left by the previous writer, regardless of
// process.
//
// After reading, the fd is seeked to the end so that json.Encoder (which uses
// O_APPEND semantics) can continue writing without seeking itself.
func readLastEntryHashFromFd(f *os.File) (string, error) {
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat: %w", err)
	}
	size := info.Size()
	if size == 0 {
		// File is empty — seek to EOF for consistency before returning.
		// HIGH-1 fix (round 30): always restore fd to EOF before returning so
		// O_APPEND writes see a consistent position on non-atomic filesystems.
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			return "", fmt.Errorf("seek to end (empty file): %w", err)
		}
		return "genesis", nil
	}

	// Read the last up to maxAuditTailBytes from the file.
	readSize := size
	if readSize > maxAuditTailBytes {
		readSize = maxAuditTailBytes
	}
	offset := size - readSize
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek: %w", err)
	}
	tail := make([]byte, readSize)
	if _, err := io.ReadFull(f, tail); err != nil {
		return "", fmt.Errorf("read tail: %w", err)
	}

	// HIGH-1 fix (round 30): seek to EOF after reading so the O_APPEND write
	// path sees a consistent fd position regardless of filesystem semantics.
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return "", fmt.Errorf("seek to end after read: %w", err)
	}

	// Trim trailing newlines to find the end of the last entry.
	tail = bytes.TrimRight(tail, "\n\r")

	// Find the start of the last line (after the last newline).
	start := bytes.LastIndexByte(tail, '\n') + 1 // +1: if -1, start=0

	// If we seeked past the start of the file and there's no newline in the
	// tail, the entire tail may be a partial line — fail safe.
	if offset > 0 && start == 0 {
		return "", fmt.Errorf("last entry exceeds maximum read window (%d bytes): cannot resume chain", maxAuditTailBytes)
	}

	lastLine := tail[start:]
	if len(lastLine) == 0 {
		return "genesis", nil
	}

	var entry AuditEntry
	if err := json.Unmarshal(lastLine, &entry); err != nil {
		return "", fmt.Errorf("parse last line: %w", err)
	}
	if entry.EntryHash == "" {
		return "", fmt.Errorf("last entry has empty entry_hash: cannot resume chain")
	}
	return entry.EntryHash, nil
}

// Write appends a single entry to the audit log. It is safe for concurrent use.
// Errors from encoding are returned to the caller.
//
// HIGH-2 fix (round 30): uses a named return so that flock unlock errors are
// propagated to the caller even when the encode path succeeded. A silent unlock
// failure (e.g., EBADF after fd duplication) leaves peer processes stalled on
// LOCK_EX indefinitely; surfacing the error allows the caller to react (e.g.,
// close/reopen the writer).
func (w *AuditWriter) Write(rec AuditRecord) (retErr error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("audittrail: write to closed writer")
	}

	// HIGH-2 fix: acquire exclusive flock before any I/O to serialize writes
	// across processes that share the same audit file. In-process serialization
	// is handled by w.mu above; cross-process serialization requires OS locking.
	// On non-Unix this is a no-op; use per-run isolated files for multi-process safety.
	if err := lockFileExclusive(w.file); err != nil {
		return fmt.Errorf("audittrail: lock file: %w", err)
	}
	// HIGH-2 fix (round 30): propagate unlock errors via named return. A
	// silent unlock failure leaves peer processes stalled forever on LOCK_EX.
	defer func() {
		if unlockErr := unlockFile(w.file); unlockErr != nil && retErr == nil {
			retErr = fmt.Errorf("audittrail: unlock file: %w", unlockErr)
		}
	}()

	// HIGH-1 fix: deep copy the caller's payload to prevent concurrent mutations
	// of nested structures from causing hash/content mismatches. The previous
	// shallow copy shared references to nested maps/slices with the caller.
	var snapshotPayload map[string]any
	if rec.Payload != nil {
		snapshotPayload = deepCopyPayload(rec.Payload)
	}

	ts := rec.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	} else {
		ts = ts.UTC()
	}

	// Marshal payload for hashing. Use a stable JSON encoding.
	var payloadJSON []byte
	var err error
	if snapshotPayload != nil {
		payloadJSON, err = json.Marshal(snapshotPayload)
		if err != nil {
			return fmt.Errorf("audittrail: marshal payload: %w", err)
		}
		// HIGH-3 fix: reject oversized payloads before writing. An entry larger
		// than maxAuditTailBytes (4 MiB) would cause chain resume to fail on
		// next startup, permanently disabling audit logging until manual repair.
		if len(payloadJSON) > maxPayloadBytes {
			return fmt.Errorf("audittrail: payload size %d exceeds maximum %d bytes", len(payloadJSON), maxPayloadBytes)
		}
	} else {
		payloadJSON = []byte("null")
	}

	// HIGH-5 fix (round 29): re-read the true on-disk prevHash under flock.
	// Two AuditWriter instances both resume with lastHash=H0 at startup; flock
	// serializes byte writes but cannot synchronize their in-memory lastHash
	// values. After process A writes E1 (prev=H0, hash=H1), process B would
	// still compute E2 with prev=H0 (its stale in-memory value), breaking the
	// chain. By re-reading under flock we always chain off the true file tail.
	prevHash, err := readLastEntryHashFromFd(w.file)
	if err != nil {
		return fmt.Errorf("audittrail: read prevHash under flock: %w", err)
	}
	w.lastHash = prevHash

	// Compute entry hash using a JSON-encoded canonical struct (CRITICAL-1 fix).
	// Plain concatenation of ts+runID+eventType+payloadJSON+prevHash is ambiguous:
	// "a"+"bc" == "ab"+"c". JSON field quoting and separators make each field
	// boundary unambiguous, eliminating concatenation-collision attacks.
	preimage := auditHashPreimage{
		Timestamp: ts.Format(time.RFC3339Nano),
		RunID:     rec.RunID,
		EventType: rec.EventType,
		Payload:   string(payloadJSON),
		PrevHash:  prevHash,
	}
	preimageBytes, err := json.Marshal(preimage)
	if err != nil {
		return fmt.Errorf("audittrail: marshal hash preimage: %w", err)
	}
	h := sha256.Sum256(preimageBytes)
	entryHash := hex.EncodeToString(h[:])

	// Build on-disk payload using the snapshot to match what was hashed.
	var diskPayload map[string]any
	if snapshotPayload != nil {
		diskPayload = snapshotPayload
	}

	entry := AuditEntry{
		Timestamp: ts,
		RunID:     rec.RunID,
		EventType: rec.EventType,
		Payload:   diskPayload,
		PrevHash:  prevHash,
		EntryHash: entryHash,
	}

	// Record the file size before encoding so we can truncate on partial-write.
	// HIGH-1 fix (round 31): if enc.Encode writes some bytes then returns an
	// error (e.g., a short write on a nearly-full disk), the partial JSON
	// fragment left in the file contains no newline delimiter. readLastEntryHashFromFd
	// cannot distinguish the partial fragment from the previous complete line and
	// fails with "parse last line" on the next Write(), permanently breaking
	// chain-resume. Truncating to preEncodeSize removes the fragment atomically.
	preEncodeInfo, statErr := w.file.Stat()
	var preEncodeSize int64
	if statErr == nil {
		preEncodeSize = preEncodeInfo.Size()
	}

	if err := w.enc.Encode(entry); err != nil {
		// Truncate away any partial bytes written by the failed Encode, then
		// recreate the encoder (its internal buffer is also corrupted by the
		// partial write). On truncate failure we still recreate the encoder and
		// return the original encode error — subsequent reads of the file will
		// fail, prompting the caller to abandon the writer.
		if statErr == nil {
			_ = w.file.Truncate(preEncodeSize)
			_, _ = w.file.Seek(0, io.SeekEnd)
		}
		w.enc = json.NewEncoder(w.file)
		w.enc.SetEscapeHTML(false)
		return fmt.Errorf("audittrail: encode entry: %w", err)
	}

	// Cache the hash for diagnostics; Write() re-reads from disk under flock
	// on each call, so this is informational only and not used for chain chaining.
	w.lastHash = entryHash
	return nil
}

// Close flushes buffered data and closes the underlying file.
// Calling Close more than once is safe and returns nil after the first call.
func (w *AuditWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true
	return w.file.Close()
}
