package messagebubble

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// TestTUI027_MessageTimestampAppears verifies that FormatTimestamp with a
// non-zero time returns a non-empty string.
func TestTUI027_MessageTimestampAppears(t *testing.T) {
	ts := FormatTimestamp(time.Now())
	if ts == "" {
		t.Error("FormatTimestamp(time.Now()) must return non-empty string")
	}
}

// TestTUI027_FormatTimestampZeroTime verifies that zero time returns "".
func TestTUI027_FormatTimestampZeroTime(t *testing.T) {
	ts := FormatTimestamp(time.Time{})
	if ts != "" {
		t.Errorf("FormatTimestamp(zero) must return empty string, got %q", ts)
	}
}

// TestTUI027_RightAlignPadsCorrectly verifies that a 2-rune label in a
// 20-column field has 18 spaces of padding on the left.
func TestTUI027_RightAlignPadsCorrectly(t *testing.T) {
	label := "PM"
	width := 20
	result := RightAlign(label, width)

	// Total rune count must equal width
	runes := utf8.RuneCountInString(result)
	if runes != width {
		t.Errorf("RightAlign(%q, %d): want %d runes, got %d", label, width, width, runes)
	}

	// Must end with label
	if !strings.HasSuffix(result, label) {
		t.Errorf("RightAlign(%q, %d): result must end with label, got %q", label, width, result)
	}

	// Must have (width - len(label)) spaces before the label
	expectedPad := strings.Repeat(" ", width-utf8.RuneCountInString(label))
	if !strings.HasPrefix(result, expectedPad) {
		t.Errorf("RightAlign(%q, %d): expected %d leading spaces, got %q", label, width, width-2, result)
	}
}

// TestTUI027_RightAlignNarrowClamps verifies that when width < len(label),
// the label is truncated to width runes.
func TestTUI027_RightAlignNarrowClamps(t *testing.T) {
	label := "Hello"
	width := 3
	result := RightAlign(label, width)

	runes := utf8.RuneCountInString(result)
	if runes != width {
		t.Errorf("RightAlign narrow: want %d runes, got %d, result=%q", width, runes, result)
	}
	// Should be the first 3 runes of "Hello"
	if result != "Hel" {
		t.Errorf("RightAlign narrow: expected %q, got %q", "Hel", result)
	}
}

// TestTUI027_ToolTimestampAlignsRight verifies that a timestamp string
// right-aligned to a given width fits exactly within that width.
func TestTUI027_ToolTimestampAlignsRight(t *testing.T) {
	ts := FormatTimestamp(time.Now())
	width := 40
	aligned := RightAlign(ts, width)

	runes := utf8.RuneCountInString(aligned)
	if runes != width {
		t.Errorf("right-aligned timestamp: want %d runes, got %d, result=%q", width, runes, aligned)
	}
	if !strings.HasSuffix(aligned, ts) {
		t.Errorf("right-aligned timestamp must end with timestamp string %q, got %q", ts, aligned)
	}
}

// TestTUI027_ConcurrentTimestamp verifies that concurrent calls to
// FormatTimestamp and RightAlign trigger no data races.
func TestTUI027_ConcurrentTimestamp(t *testing.T) {
	now := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ts := FormatTimestamp(now)
				if ts == "" {
					panic("FormatTimestamp returned empty for non-zero time")
				}
				_ = RightAlign(ts, 40)
			}
		}()
	}
	wg.Wait()
}

// TestTUI027_VisualSnapshot_80x24 writes a snapshot of right-aligned
// timestamps at 80 columns to testdata/snapshots/TUI-027-timestamp-80x24.txt.
func TestTUI027_VisualSnapshot_80x24(t *testing.T) {
	width := 80
	ts := FormatTimestamp(time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC))
	line := RightAlign(ts, width)

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-027-timestamp-80x24.txt"
	content := line + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI027_VisualSnapshot_120x40 writes a snapshot at 120 columns.
func TestTUI027_VisualSnapshot_120x40(t *testing.T) {
	width := 120
	ts := FormatTimestamp(time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC))
	line := RightAlign(ts, width)

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-027-timestamp-120x40.txt"
	content := line + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI027_VisualSnapshot_200x50 writes a snapshot at 200 columns.
func TestTUI027_VisualSnapshot_200x50(t *testing.T) {
	width := 200
	ts := FormatTimestamp(time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC))
	line := RightAlign(ts, width)

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-027-timestamp-200x50.txt"
	content := line + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
