package audittrail

import (
	"bufio"
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
	// EntryHash is SHA-256(timestamp + run_id + event_type + payload_json + prev_hash),
	// hex-encoded. Provides tamper evidence and chain integrity.
	EntryHash string `json:"entry_hash"`
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
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("audittrail: create directory %s: %w", dir, err)
	}

	// HIGH-6 fix: resume hash chain from last entry when appending to an
	// existing file. Read the file before opening for append so we can seek.
	lastHash := "genesis"
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		rh, err := readLastEntryHash(path)
		if err != nil {
			return nil, fmt.Errorf("audittrail: resume chain from %s: %w", path, err)
		}
		lastHash = rh
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("audittrail: open file %s: %w", path, err)
	}

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	return &AuditWriter{
		file:     f,
		enc:      enc,
		lastHash: lastHash,
	}, nil
}

// readLastEntryHash reads the last non-empty JSONL line from path and returns
// its entry_hash field. Returns an error if the last line cannot be parsed or
// lacks an entry_hash — the caller should fail closed in this case.
func readLastEntryHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lastLine string
	scanner := bufio.NewScanner(f)
	// Increase scanner buffer for large payload lines.
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			lastLine = line
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return "", fmt.Errorf("scan: %w", err)
	}
	if lastLine == "" {
		return "genesis", nil // file exists but has no non-empty lines
	}

	var entry AuditEntry
	if err := json.Unmarshal([]byte(lastLine), &entry); err != nil {
		return "", fmt.Errorf("parse last line: %w", err)
	}
	if entry.EntryHash == "" {
		return "", fmt.Errorf("last entry has empty entry_hash: cannot resume chain")
	}
	return entry.EntryHash, nil
}

// Write appends a single entry to the audit log. It is safe for concurrent use.
// Errors from encoding are returned to the caller.
func (w *AuditWriter) Write(rec AuditRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("audittrail: write to closed writer")
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
	if rec.Payload != nil {
		payloadJSON, err = json.Marshal(rec.Payload)
		if err != nil {
			return fmt.Errorf("audittrail: marshal payload: %w", err)
		}
	} else {
		payloadJSON = []byte("null")
	}

	prevHash := w.lastHash

	// Compute entry hash:
	// entry_hash = SHA-256(timestamp + run_id + event_type + payload_json + prev_hash)
	hashInput := ts.Format(time.RFC3339Nano) +
		rec.RunID +
		rec.EventType +
		string(payloadJSON) +
		prevHash
	h := sha256.Sum256([]byte(hashInput))
	entryHash := hex.EncodeToString(h[:])

	// Build on-disk payload (only include when non-nil for cleaner output)
	var diskPayload map[string]any
	if rec.Payload != nil {
		diskPayload = rec.Payload
	}

	entry := AuditEntry{
		Timestamp: ts,
		RunID:     rec.RunID,
		EventType: rec.EventType,
		Payload:   diskPayload,
		PrevHash:  prevHash,
		EntryHash: entryHash,
	}

	if err := w.enc.Encode(entry); err != nil {
		return fmt.Errorf("audittrail: encode entry: %w", err)
	}

	// Advance the chain.
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
