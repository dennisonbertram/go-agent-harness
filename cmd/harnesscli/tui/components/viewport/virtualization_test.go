package viewport_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/viewport"
)

// TestTUI030_VirtualizationSkipsOffscreenMessages verifies that View() returns
// only the visible window, not all 100 lines.
func TestTUI030_VirtualizationSkipsOffscreenMessages(t *testing.T) {
	vm := viewport.NewVirtualizedModel(10, 0)
	for i := 0; i < 100; i++ {
		vm = vm.AppendLine(fmt.Sprintf("line %d", i))
	}
	visible := vm.View()
	if len(visible) != 10 {
		t.Errorf("expected exactly 10 visible lines, got %d", len(visible))
	}
	if vm.TotalLines() != 100 {
		t.Errorf("expected 100 total lines, got %d", vm.TotalLines())
	}
}

// TestTUI030_ScrollRevealsOffscreenMessage verifies that scrolling up reveals
// lines that were previously offscreen at the bottom.
func TestTUI030_ScrollRevealsOffscreenMessage(t *testing.T) {
	vm := viewport.NewVirtualizedModel(5, 0)
	for i := 0; i < 20; i++ {
		vm = vm.AppendLine(fmt.Sprintf("line %d", i))
	}
	// At bottom, we should see lines 15-19
	bottomVisible := vm.View()
	if !strings.Contains(strings.Join(bottomVisible, "\n"), "line 19") {
		t.Errorf("expected line 19 in bottom view, got: %v", bottomVisible)
	}

	// Scroll up by 10 to see lines 5-9
	vm = vm.ScrollUp(10)
	afterScroll := vm.View()
	joined := strings.Join(afterScroll, "\n")
	if strings.Contains(joined, "line 19") {
		t.Errorf("after scrolling up, line 19 should not be visible, got: %v", afterScroll)
	}
	// Should see earlier lines now
	foundEarlier := false
	for _, l := range afterScroll {
		if strings.Contains(l, "line 5") || strings.Contains(l, "line 6") ||
			strings.Contains(l, "line 7") || strings.Contains(l, "line 8") ||
			strings.Contains(l, "line 9") {
			foundEarlier = true
			break
		}
	}
	if !foundEarlier {
		t.Errorf("expected earlier lines visible after scroll up, got: %v", afterScroll)
	}
}

// TestTUI030_WindowSliceEmpty verifies WindowSlice does not panic on empty input.
func TestTUI030_WindowSliceEmpty(t *testing.T) {
	result := viewport.WindowSlice(nil, 0, 10)
	if result == nil {
		result = []string{}
	}
	if len(result) != 0 {
		t.Errorf("expected empty result for nil lines, got %d items", len(result))
	}

	result2 := viewport.WindowSlice([]string{}, 0, 10)
	if len(result2) != 0 {
		t.Errorf("expected empty result for empty lines, got %d items", len(result2))
	}
}

// TestTUI030_WindowSliceAtEnd verifies that offset near the end doesn't go past slice boundary.
func TestTUI030_WindowSliceAtEnd(t *testing.T) {
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	// offset=12, height=10 would try to end at 15+2=17 if unclamped
	result := viewport.WindowSlice(lines, 12, 10)
	if len(result) > 10 {
		t.Errorf("WindowSlice returned %d lines, max height is 10", len(result))
	}
	for _, l := range result {
		if l == "" {
			continue
		}
		// Verify we don't have out-of-range content
		found := false
		for _, orig := range lines {
			if l == orig {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("WindowSlice returned line not in original: %q", l)
		}
	}
}

// TestTUI030_ClampOffsetBelowZero verifies that negative offset is clamped to 0.
func TestTUI030_ClampOffsetBelowZero(t *testing.T) {
	result := viewport.ClampOffset(-5, 10, 20)
	if result != 0 {
		t.Errorf("expected ClampOffset(-5, 10, 20) = 0, got %d", result)
	}
}

// TestTUI030_ClampOffsetPastEnd verifies that offset+height > totalLines is clamped correctly.
func TestTUI030_ClampOffsetPastEnd(t *testing.T) {
	// totalLines=20, height=10 => maxOffset = 20-10 = 10
	result := viewport.ClampOffset(15, 10, 20)
	if result != 10 {
		t.Errorf("expected ClampOffset(15, 10, 20) = 10, got %d", result)
	}

	// Edge: totalLines <= height => maxOffset = 0
	result2 := viewport.ClampOffset(5, 10, 8)
	if result2 != 0 {
		t.Errorf("expected ClampOffset(5, 10, 8) = 0, got %d", result2)
	}
}

// TestTUI030_MaxHistoryPrunesOldLines verifies that lines exceeding maxHistory are pruned.
func TestTUI030_MaxHistoryPrunesOldLines(t *testing.T) {
	vm := viewport.NewVirtualizedModel(5, 10) // maxHistory=10
	for i := 0; i < 20; i++ {
		vm = vm.AppendLine(fmt.Sprintf("line %d", i))
	}
	if vm.TotalLines() > 10 {
		t.Errorf("expected at most 10 lines after pruning, got %d", vm.TotalLines())
	}
	// The last 10 lines (10-19) should be present
	visible := strings.Join(vm.ScrollToBottom().View(), "\n")
	if !strings.Contains(visible, "line 19") {
		t.Errorf("expected most recent line visible after pruning, got: %q", visible)
	}
}

// TestTUI030_ExactlyFullScreenLines verifies that exactly height lines renders all, no extra.
func TestTUI030_ExactlyFullScreenLines(t *testing.T) {
	vm := viewport.NewVirtualizedModel(5, 0)
	for i := 0; i < 5; i++ {
		vm = vm.AppendLine(fmt.Sprintf("line %d", i))
	}
	visible := vm.View()
	if len(visible) != 5 {
		t.Errorf("expected exactly 5 visible lines, got %d", len(visible))
	}
}

// TestTUI030_OneOverFullScreen verifies that height+1 lines at bottom shows lines 1-5,
// not line 0 (since line 0 is scrolled off the top).
func TestTUI030_OneOverFullScreen(t *testing.T) {
	vm := viewport.NewVirtualizedModel(5, 0)
	for i := 0; i < 6; i++ {
		vm = vm.AppendLine(fmt.Sprintf("line %d", i))
	}
	// At bottom, should see lines 1-5, not line 0
	visible := vm.View()
	joined := strings.Join(visible, "\n")
	if strings.Contains(joined, "line 0") {
		t.Errorf("line 0 should not be visible when 6 lines exist with height=5, got: %v", visible)
	}
	if !strings.Contains(joined, "line 5") {
		t.Errorf("line 5 should be visible, got: %v", visible)
	}
}

// TestTUI030_ConcurrentAppendAndScroll verifies no race conditions when appending and scrolling concurrently.
// Note: VirtualizedModel is immutable (value type). This test verifies that concurrent creation
// of new model values from a shared baseline doesn't cause data races.
func TestTUI030_ConcurrentAppendAndScroll(t *testing.T) {
	vm := viewport.NewVirtualizedModel(10, 100)
	for i := 0; i < 50; i++ {
		vm = vm.AppendLine(fmt.Sprintf("init %d", i))
	}

	var wg sync.WaitGroup
	// 5 goroutines appending
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			local := vm
			for j := 0; j < 20; j++ {
				local = local.AppendLine(fmt.Sprintf("goroutine %d line %d", id, j))
			}
			_ = local.View()
		}(i)
	}
	// 5 goroutines scrolling
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			local := vm
			for j := 0; j < 20; j++ {
				local = local.ScrollUp(1)
				local = local.ScrollDown(1)
			}
			_ = local.View()
		}(i)
	}
	wg.Wait()
}

// TestTUI030_CorruptedHeightClamped verifies that height=0 and height=-1 don't panic.
func TestTUI030_CorruptedHeightClamped(t *testing.T) {
	vm0 := viewport.NewVirtualizedModel(0, 0)
	vm0 = vm0.AppendLine("test line")
	visible0 := vm0.View()
	// height=0 => no visible lines
	if len(visible0) != 0 {
		t.Errorf("height=0 should return 0 visible lines, got %d", len(visible0))
	}

	vmNeg := viewport.NewVirtualizedModel(-1, 0)
	vmNeg = vmNeg.AppendLine("test line")
	visibleNeg := vmNeg.View()
	// height=-1 => no visible lines
	if len(visibleNeg) != 0 {
		t.Errorf("height=-1 should return 0 visible lines, got %d", len(visibleNeg))
	}
}

// TestTUI030_VisualSnapshot_80x24 writes a snapshot at 80x24 terminal size.
func TestTUI030_VisualSnapshot_80x24(t *testing.T) {
	writeVirtualizationSnapshot(t, 80, 24, "TUI-030-virtualization-80x24.txt")
}

// TestTUI030_VisualSnapshot_120x40 writes a snapshot at 120x40 terminal size.
func TestTUI030_VisualSnapshot_120x40(t *testing.T) {
	writeVirtualizationSnapshot(t, 120, 40, "TUI-030-virtualization-120x40.txt")
}

// TestTUI030_VisualSnapshot_200x50 writes a snapshot at 200x50 terminal size.
func TestTUI030_VisualSnapshot_200x50(t *testing.T) {
	writeVirtualizationSnapshot(t, 200, 50, "TUI-030-virtualization-200x50.txt")
}

func writeVirtualizationSnapshot(t *testing.T, width, height int, filename string) {
	t.Helper()

	vm := viewport.NewVirtualizedModel(height, 200)
	for i := 0; i < 150; i++ {
		vm = vm.AppendLine(fmt.Sprintf("[%3d] Message content for line %d — width=%d height=%d", i, i, width, height))
	}

	// Capture bottom view
	bottomLines := vm.ScrollToBottom().View()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== TUI-030 Virtualization Snapshot: %dx%d ===\n", width, height))
	sb.WriteString(fmt.Sprintf("Total lines: %d, Visible lines: %d\n", vm.TotalLines(), vm.VisibleLines()))
	sb.WriteString(strings.Repeat("-", width) + "\n")
	for _, line := range bottomLines {
		// truncate to width for display
		runes := []rune(line)
		if len(runes) > width {
			line = string(runes[:width])
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString(strings.Repeat("-", width) + "\n")

	// Capture mid-scroll view
	scrolled := vm.ScrollUp(height / 2)
	scrolledLines := scrolled.View()
	sb.WriteString(fmt.Sprintf("--- Scrolled up %d lines ---\n", height/2))
	for _, line := range scrolledLines {
		runes := []rune(line)
		if len(runes) > width {
			line = string(runes[:width])
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString(strings.Repeat("-", width) + "\n")

	dir := filepath.Join("testdata", "snapshots")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	snapshotPath := filepath.Join(dir, filename)
	if err := os.WriteFile(snapshotPath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot %s: %v", snapshotPath, err)
	}
	t.Logf("snapshot written: %s", snapshotPath)
}

// TestTUI030_TotalHeight verifies TotalHeight helper function.
func TestTUI030_TotalHeight(t *testing.T) {
	lines := []string{"a", "b", "c"}
	if viewport.TotalHeight(lines) != 3 {
		t.Errorf("TotalHeight(%v) = %d, want 3", lines, viewport.TotalHeight(lines))
	}
	if viewport.TotalHeight(nil) != 0 {
		t.Errorf("TotalHeight(nil) = %d, want 0", viewport.TotalHeight(nil))
	}
}
