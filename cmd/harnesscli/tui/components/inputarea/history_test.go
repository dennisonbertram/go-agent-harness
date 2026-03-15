package inputarea_test

import (
	"os"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

// TestTUI046_HistoryNavigatesBackwardAndForward pushes 3 items and navigates.
func TestTUI046_HistoryNavigatesBackwardAndForward(t *testing.T) {
	h := inputarea.NewHistory(100)
	h = h.Push("first")
	h = h.Push("second")
	h = h.Push("third")
	// Entries are stored newest-first: ["third", "second", "first"]

	var text string
	h, text = h.Up("")
	if text != "third" {
		t.Errorf("Up#1: want 'third', got %q", text)
	}
	h, text = h.Up("")
	if text != "second" {
		t.Errorf("Up#2: want 'second', got %q", text)
	}
	h, text = h.Up("")
	if text != "first" {
		t.Errorf("Up#3: want 'first', got %q", text)
	}
	// Navigate forward.
	h, text = h.Down()
	if text != "second" {
		t.Errorf("Down#1: want 'second', got %q", text)
	}
	h, text = h.Down()
	if text != "third" {
		t.Errorf("Down#2: want 'third', got %q", text)
	}
	// One more down returns to draft.
	h, text = h.Down()
	if text != "" {
		t.Errorf("Down#3 (draft): want '', got %q", text)
	}
	if !h.AtDraft() {
		t.Error("should be at draft after Down past newest entry")
	}
}

// TestTUI046_DraftPreservedAcrossNavigation verifies draft is saved/restored.
func TestTUI046_DraftPreservedAcrossNavigation(t *testing.T) {
	h := inputarea.NewHistory(100)
	h = h.Push("history-entry")

	// Start with some text in input.
	const draft = "my draft text"
	h, _ = h.Up(draft) // saves draft, goes to "history-entry"
	h, text := h.Down()
	if text != draft {
		t.Errorf("draft not restored: want %q, got %q", draft, text)
	}
	if !h.AtDraft() {
		t.Error("should be at draft position after returning from history")
	}
}

// TestTUI046_HistoryCappedAtMaxSize verifies oldest entries are dropped.
func TestTUI046_HistoryCappedAtMaxSize(t *testing.T) {
	h := inputarea.NewHistory(3)
	h = h.Push("a")
	h = h.Push("b")
	h = h.Push("c")
	h = h.Push("d") // should drop "a"

	if h.Len() != 3 {
		t.Errorf("Len: want 3, got %d", h.Len())
	}
	// Navigate all the way back; should see d, c, b (not a).
	texts := []string{}
	hNav := h
	var text string
	for i := 0; i < 3; i++ {
		hNav, text = hNav.Up("")
		texts = append(texts, text)
	}
	for _, want := range []string{"d", "c", "b"} {
		found := false
		for _, got := range texts {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in history, got %v", want, texts)
		}
	}
	for _, notWant := range []string{"a"} {
		for _, got := range texts {
			if got == notWant {
				t.Errorf("expected %q to be dropped from history, but found it", notWant)
			}
		}
	}
}

// TestTUI046_DuplicateSkipped verifies same text is not added twice consecutively.
func TestTUI046_DuplicateSkipped(t *testing.T) {
	h := inputarea.NewHistory(100)
	h = h.Push("hello")
	h = h.Push("hello") // duplicate; should be skipped

	if h.Len() != 1 {
		t.Errorf("Len: want 1, got %d", h.Len())
	}
}

// TestTUI046_EmptyHistoryUpIsNoOp verifies Up on empty history returns same text.
func TestTUI046_EmptyHistoryUpIsNoOp(t *testing.T) {
	h := inputarea.NewHistory(100)
	h2, text := h.Up("current")
	if text != "current" {
		t.Errorf("Up on empty history: want 'current', got %q", text)
	}
	if !h2.AtDraft() {
		t.Error("should remain at draft on empty history Up")
	}
}

// TestTUI046_DownPastStartReturnsDraft verifies Down at pos=-1 returns saved draft.
func TestTUI046_DownPastStartReturnsDraft(t *testing.T) {
	h := inputarea.NewHistory(100)
	// Down when already at draft returns draft (empty here).
	h2, text := h.Down()
	if text != "" {
		t.Errorf("Down at draft: want '', got %q", text)
	}
	if !h2.AtDraft() {
		t.Error("should still be at draft")
	}
}

// TestTUI046_ClearResetsHistory verifies Len()==0 and AtDraft after Clear.
func TestTUI046_ClearResetsHistory(t *testing.T) {
	h := inputarea.NewHistory(100)
	h = h.Push("a")
	h = h.Push("b")
	h = h.Clear()

	if h.Len() != 0 {
		t.Errorf("Len after Clear: want 0, got %d", h.Len())
	}
	if !h.AtDraft() {
		t.Error("should be at draft after Clear")
	}
}

// TestTUI046_ConcurrentHistories verifies 10 goroutines each with their own History have no race.
func TestTUI046_ConcurrentHistories(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			h := inputarea.NewHistory(10)
			for j := 0; j < 5; j++ {
				h = h.Push("entry")
			}
			h, _ = h.Up("draft")
			h, _ = h.Down()
			_ = h.Len()
			_ = h.AtDraft()
			_ = h.Clear()
		}(i)
	}
	wg.Wait()
}

// Helper to type text into a model.
func typeText(m inputarea.Model, text string) inputarea.Model {
	for _, r := range text {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// TestTUI046_VisualSnapshot_80x24 renders the input area with history navigation at 80x24.
func TestTUI046_VisualSnapshot_80x24(t *testing.T) {
	m := inputarea.New(80)
	m = typeText(m, "first command")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = typeText(m, "second command")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Navigate up once to show history.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	output := m.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-046-history-80x24.txt"
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)

	if !strings.Contains(output, "second command") {
		t.Errorf("snapshot must contain 'second command', got:\n%s", output)
	}
}

// TestTUI046_VisualSnapshot_120x40 renders the input area with history navigation at 120x40.
func TestTUI046_VisualSnapshot_120x40(t *testing.T) {
	m := inputarea.New(120)
	m = typeText(m, "first command")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = typeText(m, "second command")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	output := m.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-046-history-120x40.txt"
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI046_VisualSnapshot_200x50 renders the input area with history navigation at 200x50.
func TestTUI046_VisualSnapshot_200x50(t *testing.T) {
	m := inputarea.New(200)
	m = typeText(m, "first command")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = typeText(m, "second command")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	output := m.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-046-history-200x50.txt"
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
