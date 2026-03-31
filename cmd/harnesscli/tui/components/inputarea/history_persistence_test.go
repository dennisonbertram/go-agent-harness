package inputarea_test

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

// BT-001: When a user types "hello" and presses Enter, then presses Up, the input shows "hello".
func TestBT001_UpAfterSubmitShowsLastCommand(t *testing.T) {
	m := inputarea.New(80)
	m = typeText(m, "hello")
	m, _ = m.Update(btKeyEnter())
	// Now press Up — should show "hello"
	m, _ = m.Update(btKeyUp())
	if m.Value() != "hello" {
		t.Errorf("BT-001: after Up, want 'hello', got %q", m.Value())
	}
}

// BT-002: When user submits 3 commands and presses Up three times, shows c, b, a.
func TestBT002_ThreeCommandsNavigateBackwardCorrectly(t *testing.T) {
	m := inputarea.New(80)
	for _, cmd := range []string{"a", "b", "c"} {
		m = typeText(m, cmd)
		m, _ = m.Update(btKeyEnter())
	}

	m, _ = m.Update(btKeyUp())
	if m.Value() != "c" {
		t.Errorf("BT-002 Up#1: want 'c', got %q", m.Value())
	}
	m, _ = m.Update(btKeyUp())
	if m.Value() != "b" {
		t.Errorf("BT-002 Up#2: want 'b', got %q", m.Value())
	}
	m, _ = m.Update(btKeyUp())
	if m.Value() != "a" {
		t.Errorf("BT-002 Up#3: want 'a', got %q", m.Value())
	}
}

// BT-003: When a user presses Down after navigating up, it moves forward through entries.
func TestBT003_DownAfterUpMovesForward(t *testing.T) {
	m := inputarea.New(80)
	for _, cmd := range []string{"first", "second", "third"} {
		m = typeText(m, cmd)
		m, _ = m.Update(btKeyEnter())
	}

	// Navigate up to "third", "second"
	m, _ = m.Update(btKeyUp())
	m, _ = m.Update(btKeyUp())
	if m.Value() != "second" {
		t.Fatalf("BT-003 setup: want 'second', got %q", m.Value())
	}

	// Navigate down — should go back to "third"
	m, _ = m.Update(btKeyDown())
	if m.Value() != "third" {
		t.Errorf("BT-003 Down: want 'third', got %q", m.Value())
	}
}

// BT-004: When at the most recent entry and pressing Down, input returns to draft.
func TestBT004_DownFromNewestEntryReturnsDraft(t *testing.T) {
	m := inputarea.New(80)
	m = typeText(m, "mycommand")
	m, _ = m.Update(btKeyEnter())
	// Type a draft but don't submit
	m = typeText(m, "draft text")

	// Navigate up to history
	m, _ = m.Update(btKeyUp())
	if m.Value() != "mycommand" {
		t.Fatalf("BT-004 setup: want 'mycommand', got %q", m.Value())
	}

	// Navigate down — should return to draft
	m, _ = m.Update(btKeyDown())
	if m.Value() != "draft text" {
		t.Errorf("BT-004: Down from newest should restore draft; want 'draft text', got %q", m.Value())
	}
}

// BT-005: After a quit/restart (simulated via WithEntries), previous history is available.
func TestBT005_PersistenceViaWithEntries(t *testing.T) {
	// Simulate a "session" that accumulated history
	m := inputarea.New(80)
	for _, cmd := range []string{"persist-a", "persist-b", "persist-c"} {
		m = typeText(m, cmd)
		m, _ = m.Update(btKeyEnter())
	}

	// Extract the entries (simulating save-to-disk)
	entries := m.HistoryState().Entries()
	if len(entries) == 0 {
		t.Fatal("BT-005: Entries() returned empty slice after submitting commands")
	}

	// Simulate "restart": new model loaded with saved entries
	m2 := inputarea.NewWithHistory(80, inputarea.NewHistoryWithEntries(100, entries))

	// History navigation should work
	m2, _ = m2.Update(btKeyUp())
	if m2.Value() != "persist-c" {
		t.Errorf("BT-005: After restart, Up should show 'persist-c', got %q", m2.Value())
	}
}

// BT-006: When history has 100 entries and a new one is added, the oldest is evicted.
func TestBT006_OldestEntryEvictedAt100(t *testing.T) {
	h := inputarea.NewHistory(100)
	// Push 100 entries
	for i := 0; i < 100; i++ {
		h = h.Push(fmt.Sprintf("entry-%03d", i))
	}
	if h.Len() != 100 {
		t.Fatalf("BT-006: want 100 entries after 100 pushes, got %d", h.Len())
	}

	// Push one more — oldest ("entry-000") should be evicted
	h = h.Push("entry-new")
	if h.Len() != 100 {
		t.Errorf("BT-006: want 100 entries after eviction, got %d", h.Len())
	}

	// Verify "entry-000" is gone via Entries()
	entries := h.Entries()
	for _, e := range entries {
		if e == "entry-000" {
			t.Error("BT-006: oldest entry 'entry-000' should have been evicted")
		}
	}
	// Verify "entry-new" is present as newest
	if entries[0] != "entry-new" {
		t.Errorf("BT-006: newest entry should be 'entry-new', got %q", entries[0])
	}
}

// BT-007: When a user submits the same command twice, only one entry is stored.
func TestBT007_DuplicateCommandSuppressed(t *testing.T) {
	m := inputarea.New(80)
	m = typeText(m, "duplicate")
	m, _ = m.Update(btKeyEnter())
	m = typeText(m, "duplicate")
	m, _ = m.Update(btKeyEnter())

	h := m.HistoryState()
	if h.Len() != 1 {
		t.Errorf("BT-007: duplicate suppression failed; want 1 entry, got %d", h.Len())
	}
}

// BT-008: When user has typed text and presses Down without having pressed Up first, input is preserved.
// Regression guard for: https://github.com/dennisonbertram/go-agent-harness/issues/473
// Before the fix, KeyDown unconditionally called history.Down() which returned the empty draft
// string when pos == -1, wiping whatever the user had typed.
func TestBT008_DownWithoutUpPreservesInput(t *testing.T) {
	m := inputarea.New(80)
	m = typeText(m, "work in progress")

	// Sanity check: we have text and are at the draft position.
	if m.Value() != "work in progress" {
		t.Fatalf("BT-008 setup: want 'work in progress', got %q", m.Value())
	}
	if !m.HistoryState().AtDraft() {
		t.Fatal("BT-008 setup: expected to be at draft position before any navigation")
	}

	// Press Down without ever pressing Up — input must NOT be wiped.
	m, _ = m.Update(btKeyDown())
	if m.Value() != "work in progress" {
		t.Errorf("BT-008: Down at draft position wiped input; want 'work in progress', got %q", m.Value())
	}
}

// helpers — named with bt prefix to avoid collision with history_test.go helpers

func btKeyEnter() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

func btKeyUp() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyUp}
}

func btKeyDown() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyDown}
}
