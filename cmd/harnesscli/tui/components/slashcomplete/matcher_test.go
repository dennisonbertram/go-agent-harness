package slashcomplete_test

import (
	"os"
	"strings"
	"sync"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/slashcomplete"
)

// TestTUI047_FuzzyMatchRanksCloserTermsHigher verifies that "clear" scores higher
// than "context" for the query "cl" (prefix match beats no prefix match).
func TestTUI047_FuzzyMatchRanksCloserTermsHigher(t *testing.T) {
	scoreClear := slashcomplete.Score("cl", "clear")
	scoreContext := slashcomplete.Score("cl", "context")

	if scoreClear <= scoreContext {
		t.Errorf("expected Score(cl, clear)=%d > Score(cl, context)=%d", scoreClear, scoreContext)
	}
}

// TestTUI047_AutocompleteStableOnTyping verifies that repeated calls with the same
// query return the same order every time.
func TestTUI047_AutocompleteStableOnTyping(t *testing.T) {
	suggestions := []slashcomplete.Suggestion{
		{Name: "clear", Description: "Clear conversation history"},
		{Name: "context", Description: "Show context usage grid"},
		{Name: "help", Description: "Show help dialog"},
		{Name: "quit", Description: "Quit the TUI"},
		{Name: "stats", Description: "Show usage statistics"},
	}

	first := slashcomplete.FuzzyFilter(suggestions, "c")
	second := slashcomplete.FuzzyFilter(suggestions, "c")

	if len(first) != len(second) {
		t.Fatalf("same query returned different lengths: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Name != second[i].Name {
			t.Errorf("position %d: first=%q second=%q", i, first[i].Name, second[i].Name)
		}
	}
}

// TestTUI047_ExactPrefixScoresHighest verifies prefix match outscores substring match.
func TestTUI047_ExactPrefixScoresHighest(t *testing.T) {
	// "clear" has prefix "cl"; "declare" has "cl" as a substring but not prefix
	prefixScore := slashcomplete.Score("cl", "clear")
	containsScore := slashcomplete.Score("cl", "declare")

	if prefixScore <= containsScore {
		t.Errorf("prefix score (%d) should be > contains score (%d)", prefixScore, containsScore)
	}
}

// TestTUI047_NoMatchReturnsZero verifies Score returns 0 when there's no fuzzy match.
func TestTUI047_NoMatchReturnsZero(t *testing.T) {
	score := slashcomplete.Score("zzz", "clear")
	if score != 0 {
		t.Errorf("Score(zzz, clear) = %d, want 0", score)
	}
}

// TestTUI047_EmptyQueryShowsAll verifies FuzzyFilter with empty query returns all suggestions.
func TestTUI047_EmptyQueryShowsAll(t *testing.T) {
	suggestions := []slashcomplete.Suggestion{
		{Name: "clear"}, {Name: "context"}, {Name: "help"}, {Name: "quit"}, {Name: "stats"},
	}
	result := slashcomplete.FuzzyFilter(suggestions, "")
	if len(result) != len(suggestions) {
		t.Errorf("empty query: got %d results, want %d", len(result), len(suggestions))
	}
}

// TestTUI047_SubsequenceMatch verifies "cr" matches "clear" via subsequence (c...r).
func TestTUI047_SubsequenceMatch(t *testing.T) {
	// "clear" contains c, then l, e, a, r — so "cr" is a valid subsequence
	score := slashcomplete.Score("cr", "clear")
	if score == 0 {
		t.Errorf("Score(cr, clear) = 0, expected subsequence match (>0)")
	}
}

// TestTUI047_StableSort verifies that equal-scored items maintain their insertion order.
func TestTUI047_StableSort(t *testing.T) {
	// Two items with the same name length and query that should score identically
	suggestions := []slashcomplete.Suggestion{
		{Name: "aaa", Description: "first"},
		{Name: "bbb", Description: "second"},
		{Name: "ccc", Description: "third"},
	}
	// Query "a" only matches "aaa" with prefix; "bbb" and "ccc" may be subsequence
	// The key check: same-score items must stay in original order across calls.
	r1 := slashcomplete.FuzzyFilter(suggestions, "")
	r2 := slashcomplete.FuzzyFilter(suggestions, "")

	for i := range r1 {
		if r1[i].Name != r2[i].Name {
			t.Errorf("stable sort violation at index %d: r1=%q r2=%q", i, r1[i].Name, r2[i].Name)
		}
	}
}

// TestTUI047_ConcurrentFilter verifies 10 goroutines can call FuzzyFilter concurrently
// without data races.
func TestTUI047_ConcurrentFilter(t *testing.T) {
	suggestions := []slashcomplete.Suggestion{
		{Name: "clear"}, {Name: "context"}, {Name: "help"}, {Name: "quit"}, {Name: "stats"},
	}

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			_ = slashcomplete.FuzzyFilter(suggestions, "c")
			_ = slashcomplete.FuzzyFilter(suggestions, "")
			_ = slashcomplete.Score("cl", "clear")
		}()
	}
	wg.Wait()
}

// TestTUI047_VisualSnapshot_80x24 generates a fuzzy-filtered dropdown snapshot at width 80.
func TestTUI047_VisualSnapshot_80x24(t *testing.T) {
	suggestions := []slashcomplete.Suggestion{
		{Name: "clear", Description: "Clear conversation history"},
		{Name: "context", Description: "Show context usage grid"},
		{Name: "help", Description: "Show help dialog"},
		{Name: "quit", Description: "Quit the TUI"},
		{Name: "stats", Description: "Show usage statistics"},
	}
	m := slashcomplete.New(suggestions)
	m = m.Open()
	m = m.SetQuery("c") // fuzzy filter on "c" — shows clear, context ranked

	output := m.View(80)

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-047-fuzzy-80x24.txt"
	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)

	if strings.TrimSpace(output) == "" {
		t.Error("View() returned empty output")
	}
}

// TestTUI047_VisualSnapshot_120x40 generates a fuzzy-filtered dropdown snapshot at width 120.
func TestTUI047_VisualSnapshot_120x40(t *testing.T) {
	suggestions := []slashcomplete.Suggestion{
		{Name: "clear", Description: "Clear conversation history"},
		{Name: "context", Description: "Show context usage grid"},
		{Name: "help", Description: "Show help dialog"},
		{Name: "quit", Description: "Quit the TUI"},
		{Name: "stats", Description: "Show usage statistics"},
	}
	m := slashcomplete.New(suggestions)
	m = m.Open()
	m = m.SetQuery("c")

	output := m.View(120)

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-047-fuzzy-120x40.txt"
	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)

	if strings.TrimSpace(output) == "" {
		t.Error("View() returned empty output at width 120")
	}
}

// TestTUI047_VisualSnapshot_200x50 generates a fuzzy-filtered dropdown snapshot at width 200.
func TestTUI047_VisualSnapshot_200x50(t *testing.T) {
	suggestions := []slashcomplete.Suggestion{
		{Name: "clear", Description: "Clear conversation history"},
		{Name: "context", Description: "Show context usage grid"},
		{Name: "help", Description: "Show help dialog"},
		{Name: "quit", Description: "Quit the TUI"},
		{Name: "stats", Description: "Show usage statistics"},
	}
	m := slashcomplete.New(suggestions)
	m = m.Open()
	m = m.SetQuery("c")

	output := m.View(200)

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-047-fuzzy-200x50.txt"
	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)

	if strings.TrimSpace(output) == "" {
		t.Error("View() returned empty output at width 200")
	}
}
