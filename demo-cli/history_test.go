package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestHistory_AddAndNavigate tests basic add and navigation.
func TestHistory_AddAndNavigate(t *testing.T) {
	h := NewHistory(5)

	// Empty history: Prev returns "", false
	got, ok := h.Prev()
	if ok {
		t.Fatalf("expected Prev on empty history to return false, got %q", got)
	}

	h.Add("first")
	h.Add("second")
	h.Add("third")

	// Reset position for navigation
	h.ResetPos()

	// Prev moves backward: third, second, first
	entry, ok := h.Prev()
	if !ok || entry != "third" {
		t.Errorf("expected 'third', got %q ok=%v", entry, ok)
	}
	entry, ok = h.Prev()
	if !ok || entry != "second" {
		t.Errorf("expected 'second', got %q ok=%v", entry, ok)
	}
	entry, ok = h.Prev()
	if !ok || entry != "first" {
		t.Errorf("expected 'first', got %q ok=%v", entry, ok)
	}

	// At oldest: Prev returns false
	_, ok = h.Prev()
	if ok {
		t.Errorf("expected Prev at oldest to return false")
	}

	// Next moves forward: second, third
	entry, ok = h.Next()
	if !ok || entry != "second" {
		t.Errorf("expected 'second', got %q ok=%v", entry, ok)
	}
	entry, ok = h.Next()
	if !ok || entry != "third" {
		t.Errorf("expected 'third', got %q ok=%v", entry, ok)
	}

	// At newest: Next returns false (end of history, restore draft)
	_, ok = h.Next()
	if ok {
		t.Errorf("expected Next at newest to return false (signal restore draft)")
	}
}

// TestHistory_DraftPreservation tests that the draft is preserved during navigation.
func TestHistory_DraftPreservation(t *testing.T) {
	h := NewHistory(10)
	h.Add("first")
	h.Add("second")

	draft := "my unfinished draft"
	h.SetDraft(draft)
	h.ResetPos()

	// Navigate up
	_, ok := h.Prev()
	if !ok {
		t.Fatal("expected Prev to return ok=true")
	}

	// Navigate back to end — Next should signal done (ok=false) and draft should be retrievable
	h.Next() // second -> end of history
	got := h.GetDraft()
	if got != draft {
		t.Errorf("expected draft %q, got %q", draft, got)
	}
}

// TestHistory_RingBuffer tests that the ring buffer evicts oldest entries.
func TestHistory_RingBuffer(t *testing.T) {
	h := NewHistory(3) // max 3 entries

	h.Add("a")
	h.Add("b")
	h.Add("c")
	h.Add("d") // evicts "a"

	entries := h.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(entries), entries)
	}
	if entries[0] != "b" {
		t.Errorf("expected oldest to be 'b', got %q", entries[0])
	}
	if entries[2] != "d" {
		t.Errorf("expected newest to be 'd', got %q", entries[2])
	}
}

// TestHistory_AddEmpty tests that empty or whitespace-only entries are not added.
func TestHistory_AddEmpty(t *testing.T) {
	h := NewHistory(10)
	h.Add("")
	h.Add("   ")
	h.Add("\t\n")

	entries := h.Entries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after adding empty/whitespace, got %d: %v", len(entries), entries)
	}
}

// TestHistory_NoDuplicates is not required but the ring buffer should NOT deduplicate
// (bash history does not deduplicate by default).
// This test verifies duplicates are preserved.
func TestHistory_DuplicatesAllowed(t *testing.T) {
	h := NewHistory(10)
	h.Add("same")
	h.Add("same")

	entries := h.Entries()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (duplicates allowed), got %d", len(entries))
	}
}

// TestHistory_PersistLoad tests save and load from disk.
func TestHistory_PersistLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")

	h := NewHistory(100)
	h.Add("line one")
	h.Add("line two")
	h.Add("line three")

	if err := h.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	h2 := NewHistory(100)
	if err := h2.Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	entries := h2.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries after load, got %d", len(entries))
	}
	if entries[0] != "line one" || entries[1] != "line two" || entries[2] != "line three" {
		t.Errorf("unexpected entries: %v", entries)
	}
}

// TestHistory_DiskCap tests that save caps to 1000 entries on disk.
func TestHistory_DiskCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")

	h := NewHistory(2000) // in-memory can hold 2000
	for i := 0; i < 1500; i++ {
		h.Add(strings.Repeat("x", 10)) // add 1500 entries
	}

	if err := h.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Count lines in file
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	// Filter empty
	var nonEmpty []string
	for _, l := range lines {
		if l != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) != 1000 {
		t.Errorf("expected 1000 lines on disk (cap), got %d", len(nonEmpty))
	}
}

// TestHistory_LoadMissingFile tests that loading a non-existent file is a no-op.
func TestHistory_LoadMissingFile(t *testing.T) {
	h := NewHistory(10)
	err := h.Load("/nonexistent/path/history")
	if err != nil {
		t.Errorf("expected no error loading missing file, got: %v", err)
	}
}

// TestHistory_SaveCreatesDir tests that Save creates parent directories.
func TestHistory_SaveCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "history")

	h := NewHistory(10)
	h.Add("hello")
	if err := h.Save(path); err != nil {
		t.Fatalf("Save with nested dirs failed: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected history file to exist at %s", path)
	}
}

// TestHistory_ConcurrentAccess tests that concurrent Add+Entries+Save is race-free.
func TestHistory_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	h := NewHistory(100)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			h.Add(strings.Repeat("a", n+1))
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.Entries()
		}()
	}

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.Save(path)
		}()
	}

	wg.Wait()
}

// TestHistory_ResetPosAfterAdd tests that adding a new entry resets navigation.
func TestHistory_ResetPosAfterAdd(t *testing.T) {
	h := NewHistory(10)
	h.Add("first")
	h.Add("second")
	h.ResetPos()

	// Navigate back
	h.Prev()
	h.Prev()

	// Add a new entry — should reset navigation
	h.Add("third")
	h.ResetPos()

	// Prev should now go to the newest entry (third)
	entry, ok := h.Prev()
	if !ok || entry != "third" {
		t.Errorf("expected 'third' after reset, got %q ok=%v", entry, ok)
	}
}

// TestHistory_Entries_EmptyHistory tests Entries on empty history.
func TestHistory_Entries_EmptyHistory(t *testing.T) {
	h := NewHistory(10)
	entries := h.Entries()
	if entries == nil {
		entries = []string{}
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries on new history, got %d", len(entries))
	}
}

// TestHistory_PersistLoad_EscapingRoundTrip tests that entries containing
// literal backslash-n sequences and real newlines survive the save/load cycle.
func TestHistory_PersistLoad_EscapingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")

	h := NewHistory(100)
	// Entry with a literal backslash-n (two characters: '\' and 'n')
	h.Add(`hello\nworld`)
	// Entry with an actual embedded newline
	h.Add("line one\nline two")

	if err := h.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	h2 := NewHistory(100)
	if err := h2.Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	entries := h2.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}
	if entries[0] != `hello\nworld` {
		t.Errorf("literal backslash-n not preserved: got %q", entries[0])
	}
	if entries[1] != "line one\nline two" {
		t.Errorf("real newline not preserved: got %q", entries[1])
	}
}

// TestHistory_Load_RefusesSymlink tests that Load refuses to read a symlink path.
func TestHistory_Load_RefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-file")
	link := filepath.Join(dir, "history-link")

	// Create a real file and a symlink to it
	if err := os.WriteFile(target, []byte("\"hello\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation not supported on this platform: %v", err)
	}

	h := NewHistory(10)
	err := h.Load(link)
	if err == nil {
		t.Error("expected error when loading from symlink, got nil")
	}
}

// TestHistory_Load_OversizedLineSkipped tests that Load does not fail and does not
// drop entries after an oversized line; it skips the giant line and continues reading.
func TestHistory_Load_OversizedLineSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")

	h := NewHistory(100)
	h.Add("before giant")
	if err := h.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Append a line that exceeds 10 MiB and then another normal entry after it.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	// Write a quoted string where the content alone is 11 MiB (exceeds maxLineBytes).
	giant := strings.Repeat("x", 11*1024*1024)
	if _, err := f.WriteString("\"" + giant + "\"\n"); err != nil {
		f.Close()
		t.Fatalf("WriteString giant: %v", err)
	}
	// Write a normal entry AFTER the giant line.
	if _, err := f.WriteString("\"after giant\"\n"); err != nil {
		f.Close()
		t.Fatalf("WriteString after: %v", err)
	}
	f.Close()

	// Load should not return an error; it should skip the giant line and read all others.
	h2 := NewHistory(100)
	err = h2.Load(path)
	if err != nil {
		t.Errorf("expected no error on oversized line, got: %v", err)
	}
	entries := h2.Entries()
	// We expect "before giant" and "after giant" — the oversized line is skipped.
	found := make(map[string]bool)
	for _, e := range entries {
		found[e] = true
	}
	if !found["before giant"] {
		t.Errorf("expected 'before giant' in entries, got: %v", entries)
	}
	if !found["after giant"] {
		t.Errorf("expected 'after giant' in entries (entries after oversized line must be preserved), got: %v", entries)
	}
}

// TestHistory_Save_SerializesConcurrentCalls tests that concurrent Save calls within the
// same process serialize correctly and the final file is not corrupted.
func TestHistory_Save_SerializesConcurrentCalls(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	h := NewHistory(200)

	for i := 0; i < 50; i++ {
		h.Add(strings.Repeat("a", i+1))
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			h.Add(strings.Repeat("b", n+1))
			_ = h.Save(path)
		}(i)
	}
	wg.Wait()

	// File must be loadable and contain entries (not corrupted).
	h2 := NewHistory(200)
	if err := h2.Load(path); err != nil {
		t.Fatalf("Load after concurrent saves failed: %v", err)
	}
	if len(h2.Entries()) == 0 {
		t.Error("expected entries after concurrent saves, got 0")
	}
}

// TestDefaultHistoryPath tests that defaultHistoryPath returns a non-empty path.
func TestDefaultHistoryPath(t *testing.T) {
	path := defaultHistoryPath()
	if path == "" {
		t.Error("expected non-empty defaultHistoryPath")
	}
	if !strings.HasSuffix(path, "history") {
		t.Errorf("expected path to end with 'history', got %q", path)
	}
}
