package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	// historyDiskCap is the maximum number of entries persisted to disk.
	historyDiskCap = 1000
	// historyDefaultMax is the default in-memory ring buffer capacity.
	historyDefaultMax = 100
)

// History is a thread-safe in-memory ring buffer for prompt history with disk persistence.
//
// In the demo-cli, interactive navigation (Up/Down arrows) is handled by go-prompt's
// built-in history, which is seeded at startup via prompt.OptionHistory(hist.Entries()).
// This struct provides the ring buffer logic, disk persistence, and testable navigation
// primitives.
//
// Navigation model (for direct callers / tests):
//   - ResetPos() resets the cursor to "after the newest entry" (i.e., draft position).
//   - Prev() moves backward (toward older entries), returning (entry, true); returns
//     ("", false) when already at the oldest entry (no further navigation possible).
//   - Next() moves forward (toward newer entries), returning (entry, true); returns
//     ("", false) when passing the newest entry and arriving at draft position —
//     the caller should use GetDraft() to restore the saved draft text.
//   - SetDraft / GetDraft store the transient draft text while navigating.
type History struct {
	mu      sync.RWMutex
	saveMu  sync.Mutex // serializes Save calls so the newest snapshot always wins
	entries []string   // ring buffer; oldest at index 0, newest at len-1
	maxSize int
	pos     int    // current navigation index; -1 means "at draft position"
	draft   string // text in the input area before navigation started
}

// NewHistory creates a new History with the given in-memory capacity.
// If maxSize <= 0, historyDefaultMax is used.
func NewHistory(maxSize int) *History {
	if maxSize <= 0 {
		maxSize = historyDefaultMax
	}
	return &History{
		maxSize: maxSize,
		pos:     -1,
	}
}

// Add appends input to history, ignoring empty or whitespace-only strings.
// If the ring buffer is full, the oldest entry is evicted.
func (h *History) Add(input string) {
	if strings.TrimSpace(input) == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.entries) >= h.maxSize {
		// Evict oldest
		h.entries = h.entries[1:]
	}
	h.entries = append(h.entries, input)
	// Reset navigation state after a new entry is added.
	h.pos = -1
	h.draft = ""
}

// SetDraft stores the current in-progress draft before navigation begins.
func (h *History) SetDraft(draft string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.draft = draft
}

// GetDraft returns the stored draft text.
func (h *History) GetDraft() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.draft
}

// ResetPos resets the navigation cursor to the draft position (after newest entry).
func (h *History) ResetPos() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pos = -1
}

// Prev moves to the previous (older) history entry.
// Returns (entry, true) if there is an older entry; (_, false) if already at oldest.
func (h *History) Prev() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := len(h.entries)
	if n == 0 {
		return "", false
	}
	// pos == -1 means at draft; start from the newest entry.
	if h.pos == -1 {
		h.pos = n - 1
		return h.entries[h.pos], true
	}
	if h.pos == 0 {
		// Already at oldest
		return "", false
	}
	h.pos--
	return h.entries[h.pos], true
}

// Next moves to the next (newer) history entry.
// Returns (entry, true) if there is a newer entry.
// Returns ("", false) when the cursor passes the newest entry (caller should use GetDraft()).
func (h *History) Next() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.pos == -1 {
		// Already at draft position; nothing to advance to.
		return "", false
	}
	n := len(h.entries)
	if h.pos >= n-1 {
		// Move past newest → back to draft position.
		h.pos = -1
		return "", false
	}
	h.pos++
	return h.entries[h.pos], true
}

// Entries returns a copy of all history entries, oldest first.
func (h *History) Entries() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.entries) == 0 {
		return []string{}
	}
	result := make([]string, len(h.entries))
	copy(result, h.entries)
	return result
}

// Save writes history to the given file path, capped at historyDiskCap entries (newest kept).
// Parent directories are created as needed.
//
// Concurrent Save calls from within the same process are serialized by saveMu so that
// the most recently started Save always completes last and writes the newest snapshot —
// preventing a signal-handler save from overwriting a prompt-executor save that finished later.
func (h *History) Save(path string) error {
	// Serialize all Save calls: the caller that acquires saveMu last will hold the newest
	// snapshot (entries are snapshotted under mu.RLock below, inside saveMu).
	h.saveMu.Lock()
	defer h.saveMu.Unlock()

	h.mu.RLock()
	entries := make([]string, len(h.entries))
	copy(entries, h.entries)
	h.mu.RUnlock()

	// Cap to disk limit: keep the most recent entries.
	if len(entries) > historyDiskCap {
		entries = entries[len(entries)-historyDiskCap:]
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	// Verify the immediate parent directory is not a symlink.
	// Note: this is a best-effort defense for single-user local use; it does not prevent
	// all possible TOCTOU or parent-chain symlink attacks, which require OS-level primitives
	// (e.g., openat/O_NOFOLLOW) not available portably in the Go stdlib.
	if dirInfo, err := os.Lstat(dir); err == nil {
		if dirInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("history directory is a symlink, refusing to write: %s", dir)
		}
	}

	// Write to a temp file in the same directory, then atomically rename.
	// os.CreateTemp uses O_EXCL so it will not open an existing file or follow symlinks
	// (any pre-existing name conflict causes a retry with a fresh random name).
	tmp, err := os.CreateTemp(dir, ".history-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	// Validate the created temp file: ensure it is a regular file (not a symlink).
	// This catches edge cases where O_EXCL semantics differ from expectations.
	if tmpInfo, lerr := os.Lstat(tmpPath); lerr != nil || !tmpInfo.Mode().IsRegular() {
		tmp.Close()
		os.Remove(tmpPath)
		if lerr != nil {
			return lerr
		}
		return fmt.Errorf("temp file is not a regular file after creation: %s", tmpPath)
	}

	// Ensure restricted permissions before writing any content.
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}

	w := bufio.NewWriter(tmp)
	for _, entry := range entries {
		// Use strconv.Quote to safely encode entries (handles newlines, backslashes, etc.).
		// Each line is a quoted string; Unquote reverses it exactly.
		line := strconv.Quote(entry)
		if _, err := w.WriteString(line + "\n"); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return err
		}
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Verify the target path is safe to replace: must be absent, a regular file,
	// and not a symlink. This guards against clobber attacks via path manipulation.
	if info, lstatErr := os.Lstat(path); lstatErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			os.Remove(tmpPath)
			return fmt.Errorf("history path is a symlink, refusing to write: %s", path)
		}
		if !info.Mode().IsRegular() {
			os.Remove(tmpPath)
			return fmt.Errorf("history path is not a regular file, refusing to write: %s", path)
		}
	} else if !os.IsNotExist(lstatErr) {
		os.Remove(tmpPath)
		return lstatErr
	}

	// Atomic rename replaces the target safely.
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // best-effort cleanup of orphaned temp file
		return err
	}
	// Enforce permissions on the final file (rename may inherit them from temp, but be explicit).
	_ = os.Chmod(path, 0o600) // best-effort; non-fatal
	return nil
}

// Load reads history from the given file path into the ring buffer.
// If the file does not exist, Load is a no-op (no error).
func (h *History) Load(path string) error {
	// Lstat first to verify the path is a regular file (or missing), not a symlink or device.
	lstatInfo, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if lstatInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("history path is a symlink, refusing to read: %s", path)
	}
	if !lstatInfo.Mode().IsRegular() {
		return fmt.Errorf("history path is not a regular file: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Re-verify after open to close the TOCTOU window between Lstat and Open.
	// If path was swapped (e.g., to a symlink pointing elsewhere) after our Lstat,
	// the opened file's Stat will differ from the Lstat result.
	fstatInfo, err := f.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(lstatInfo, fstatInfo) {
		return fmt.Errorf("history path changed between lstat and open (possible TOCTOU): %s", path)
	}
	if !fstatInfo.Mode().IsRegular() {
		return fmt.Errorf("history path is not a regular file after open: %s", path)
	}

	// loadCap is the max lines we accumulate during a Load (prevents memory exhaustion
	// if the history file is unexpectedly large). We keep only the newest entries.
	loadCap := h.maxSize
	if loadCap < historyDiskCap {
		loadCap = historyDiskCap
	}

	// Use a fixed-size ring to avoid unbounded memory growth while reading.
	ring := make([]string, 0, loadCap)
	// Use bufio.Reader (not bufio.Scanner) so that oversized lines can be skipped
	// individually rather than stopping the entire load. A single giant /file attachment
	// in the history file will not drop all subsequent entries.
	const maxLineBytes = 10 * 1024 * 1024 // 10 MiB — entries larger than this are skipped
	br := bufio.NewReaderSize(f, 64*1024)
	// lineBuf and oversized are declared outside the loop so they remain accessible
	// after the loop body (used by the doneReading block for files without a final newline).
	var lineBuf []byte
	var oversized bool
	for {
		lineBuf = lineBuf[:0] // reset without reallocating
		oversized = false
		for {
			chunk, isPrefix, readErr := br.ReadLine()
			if readErr != nil {
				if readErr == io.EOF {
					// EOF with possible final chunk: accumulate it only when not oversized
					// to avoid memory exhaustion on a file that lacks a final newline.
					if len(chunk) > 0 && !oversized {
						lineBuf = append(lineBuf, chunk...)
						if len(lineBuf) > maxLineBytes {
							oversized = true
							lineBuf = lineBuf[:0]
						}
					}
					goto doneReading
				}
				return readErr
			}
			// Only accumulate bytes when below the per-line limit; once oversized is
			// set, keep draining ReadLine chunks (to advance past this line) without
			// appending them — this prevents memory exhaustion on huge lines.
			if !oversized {
				lineBuf = append(lineBuf, chunk...)
				if len(lineBuf) > maxLineBytes {
					oversized = true
					lineBuf = lineBuf[:0] // release accumulated bytes immediately
				}
			}
			if !isPrefix {
				break
			}
		}
		if oversized {
			// Entry exceeded the per-line limit; skip it silently.
			continue
		}
		line := string(lineBuf)
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Unquote the stored entry (reverse of strconv.Quote in Save).
		entry, uErr := strconv.Unquote(line)
		if uErr != nil {
			// Fallback: treat line as plain text (handles legacy plain-text history files).
			entry = line
		}
		if len(ring) >= loadCap {
			// Drop the oldest entry to stay within cap.
			ring = ring[1:]
		}
		ring = append(ring, entry)
	}
doneReading:
	// Process any final line that had no trailing newline (e.g., a truncated or manually
	// edited history file). lineBuf is non-empty only when the last ReadLine chunk arrived
	// at EOF without a preceding newline terminator, and the line was not oversized.
	if len(lineBuf) > 0 && !oversized {
		line := string(lineBuf)
		if strings.TrimSpace(line) != "" {
			entry, uErr := strconv.Unquote(line)
			if uErr != nil {
				entry = line
			}
			if len(ring) >= loadCap {
				ring = ring[1:]
			}
			ring = append(ring, entry)
		}
	}
	lines := ring

	h.mu.Lock()
	defer h.mu.Unlock()
	// Respect in-memory cap: keep the most recent entries.
	if len(lines) > h.maxSize {
		lines = lines[len(lines)-h.maxSize:]
	}
	h.entries = lines
	h.pos = -1
	return nil
}

// defaultHistoryPath returns the default path for persisting prompt history.
// It uses ~/.config/harnesscli/history.
func defaultHistoryPath() string {
	configDir, err := os.UserConfigDir()
	if err == nil && configDir != "" {
		return filepath.Join(configDir, "harnesscli", "history")
	}
	// Fallback: try home dir
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".harnesscli", "history")
	}
	// Last resort: use OS temp dir to avoid writing to working directory
	return filepath.Join(os.TempDir(), "harnesscli", "history")
}
