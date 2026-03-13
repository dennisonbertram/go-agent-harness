package audittrail_test

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/forensics/audittrail"
)

// readEntries reads all JSONL entries from the audit log file.
func readEntries(t *testing.T, path string) []audittrail.AuditEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	defer f.Close()

	var entries []audittrail.AuditEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry audittrail.AuditEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("unmarshal entry: %v", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return entries
}

func TestAuditWriter_BasicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	w, err := audittrail.NewAuditWriter(path)
	if err != nil {
		t.Fatalf("NewAuditWriter: %v", err)
	}

	err = w.Write(audittrail.AuditRecord{
		RunID:     "run_1",
		EventType: "run.started",
		Payload:   map[string]any{"prompt": "hello"},
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.RunID != "run_1" {
		t.Errorf("RunID = %q, want %q", entry.RunID, "run_1")
	}
	if entry.EventType != "run.started" {
		t.Errorf("EventType = %q, want %q", entry.EventType, "run.started")
	}
	if entry.PrevHash != "genesis" {
		t.Errorf("PrevHash = %q, want %q", entry.PrevHash, "genesis")
	}
	if entry.EntryHash == "" {
		t.Error("EntryHash is empty")
	}
	if entry.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
}

func TestAuditWriter_HashChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	w, err := audittrail.NewAuditWriter(path)
	if err != nil {
		t.Fatalf("NewAuditWriter: %v", err)
	}

	records := []audittrail.AuditRecord{
		{RunID: "run_1", EventType: "run.started", Payload: map[string]any{"prompt": "hello"}},
		{RunID: "run_1", EventType: "audit.action", Payload: map[string]any{"tool": "bash"}},
		{RunID: "run_1", EventType: "run.completed", Payload: map[string]any{"output": "done"}},
	}

	for _, rec := range records {
		if err := w.Write(rec); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	// First entry must have prev_hash = "genesis"
	if entries[0].PrevHash != "genesis" {
		t.Errorf("entry[0].PrevHash = %q, want %q", entries[0].PrevHash, "genesis")
	}

	// Each subsequent entry's prev_hash must equal the previous entry's entry_hash
	for i := 1; i < len(entries); i++ {
		if entries[i].PrevHash != entries[i-1].EntryHash {
			t.Errorf("entry[%d].PrevHash = %q, want entry[%d].EntryHash = %q",
				i, entries[i].PrevHash, i-1, entries[i-1].EntryHash)
		}
	}
}

func TestAuditWriter_HashChainIntegrity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	w, err := audittrail.NewAuditWriter(path)
	if err != nil {
		t.Fatalf("NewAuditWriter: %v", err)
	}

	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	payload := map[string]any{"prompt": "hello"}
	payloadJSON, _ := json.Marshal(payload)

	rec := audittrail.AuditRecord{
		RunID:     "run_1",
		EventType: "run.started",
		Payload:   payload,
		Timestamp: ts,
	}

	if err := w.Write(rec); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	entry := entries[0]

	// Verify the hash manually:
	// entry_hash = SHA-256(timestamp + run_id + event_type + payload_json + prev_hash)
	hashInput := entry.Timestamp.UTC().Format(time.RFC3339Nano) +
		entry.RunID +
		entry.EventType +
		string(payloadJSON) +
		entry.PrevHash
	h := sha256.Sum256([]byte(hashInput))
	expectedHash := hex.EncodeToString(h[:])

	if entry.EntryHash != expectedHash {
		t.Errorf("EntryHash = %q, want %q (hash of %q)", entry.EntryHash, expectedHash, hashInput)
	}
}

func TestAuditWriter_FirstEntryGenesisHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	w, err := audittrail.NewAuditWriter(path)
	if err != nil {
		t.Fatalf("NewAuditWriter: %v", err)
	}

	if err := w.Write(audittrail.AuditRecord{
		RunID:     "run_abc",
		EventType: "run.started",
		Payload:   nil,
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].PrevHash != "genesis" {
		t.Errorf("first entry PrevHash = %q, want %q", entries[0].PrevHash, "genesis")
	}
}

func TestAuditWriter_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	w, err := audittrail.NewAuditWriter(path)
	if err != nil {
		t.Fatalf("NewAuditWriter: %v", err)
	}

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w.Write(audittrail.AuditRecord{ //nolint:errcheck
				RunID:     fmt.Sprintf("run_%d", i),
				EventType: "audit.action",
				Payload:   map[string]any{"seq": i},
			})
		}(i)
	}
	wg.Wait()

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != n {
		t.Errorf("got %d entries, want %d", len(entries), n)
	}
}

func TestAuditWriter_CloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	w, err := audittrail.NewAuditWriter(path)
	if err != nil {
		t.Fatalf("NewAuditWriter: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestAuditWriter_WriteAfterClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	w, err := audittrail.NewAuditWriter(path)
	if err != nil {
		t.Fatalf("NewAuditWriter: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Write after close should not panic and should return an error or be a no-op
	err = w.Write(audittrail.AuditRecord{
		RunID:     "run_1",
		EventType: "audit.action",
		Payload:   nil,
	})
	// Either error or silently dropped is acceptable — just must not panic
	_ = err
}

func TestAuditWriter_EmptyPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	w, err := audittrail.NewAuditWriter(path)
	if err != nil {
		t.Fatalf("NewAuditWriter: %v", err)
	}

	if err := w.Write(audittrail.AuditRecord{
		RunID:     "run_1",
		EventType: "run.started",
		Payload:   nil,
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	// EntryHash must still be valid
	if entries[0].EntryHash == "" {
		t.Error("EntryHash is empty")
	}
}

func TestNewAuditWriter_InvalidDir(t *testing.T) {
	// Try to create a writer in a non-existent deeply nested path without
	// MkdirAll — but since we do MkdirAll in the implementation this should succeed
	// unless the path is truly invalid (e.g. writing to a file as a dir).
	dir := t.TempDir()
	// Create a file where we want a directory to be - this should fail
	conflictPath := filepath.Join(dir, "conflict")
	if err := os.WriteFile(conflictPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Try to write audit.jsonl inside "conflict" (which is a file, not a dir)
	_, err := audittrail.NewAuditWriter(filepath.Join(conflictPath, "audit.jsonl"))
	if err == nil {
		t.Error("expected error creating writer in file-as-directory, got nil")
	}
}

func TestNewAuditWriter_ResumesHashChain(t *testing.T) {
	// HIGH-6 fix: when appending to an existing file, the hash chain must be
	// resumed from the last entry. Writing with lastHash="genesis" mid-file
	// would create a second chain, undermining tamper evidence.
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	// First writer — write one entry.
	w1, err := audittrail.NewAuditWriter(path)
	if err != nil {
		t.Fatalf("NewAuditWriter (first): %v", err)
	}
	if err := w1.Write(audittrail.AuditRecord{
		RunID:     "r1",
		EventType: "run.started",
	}); err != nil {
		t.Fatalf("Write first: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close first: %v", err)
	}

	entries1 := readEntries(t, path)
	if len(entries1) != 1 {
		t.Fatalf("expected 1 entry after first write, got %d", len(entries1))
	}
	firstHash := entries1[0].EntryHash

	// Second writer — must resume chain from firstHash.
	w2, err := audittrail.NewAuditWriter(path)
	if err != nil {
		t.Fatalf("NewAuditWriter (second): %v", err)
	}
	if err := w2.Write(audittrail.AuditRecord{
		RunID:     "r1",
		EventType: "run.completed",
	}); err != nil {
		t.Fatalf("Write second: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close second: %v", err)
	}

	entries2 := readEntries(t, path)
	if len(entries2) != 2 {
		t.Fatalf("expected 2 entries after second write, got %d", len(entries2))
	}

	// The second entry's prev_hash must equal the first entry's entry_hash
	// (chain continuity), not "genesis" (which would indicate chain restart).
	if entries2[1].PrevHash != firstHash {
		t.Errorf("chain broken: second entry prev_hash=%q, want %q (first entry_hash)",
			entries2[1].PrevHash, firstHash)
	}
}
