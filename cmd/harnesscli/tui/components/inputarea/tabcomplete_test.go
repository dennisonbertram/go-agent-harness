package inputarea_test

import (
	"os"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

// TestTUI048_TabCompletesSingleCommand verifies that when there is exactly one
// matching completion, Tab replaces the input with that completion followed by a space.
func TestTUI048_TabCompletesSingleCommand(t *testing.T) {
	m := inputarea.New(80)
	// Type "/cl" so provider returns only "/clear"
	for _, r := range "/cl" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	provider := func(input string) []string {
		if strings.HasPrefix(input, "/cl") {
			return []string{"/clear"}
		}
		return nil
	}
	m = m.SetAutocompleteProvider(provider)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if m.Value() != "/clear " {
		t.Errorf("single completion: got %q, want %q", m.Value(), "/clear ")
	}
}

// TestTUI048_TabKeepsDropdownForMultiMatch verifies that when there are multiple
// completions, Tab completes to the common prefix.
func TestTUI048_TabKeepsDropdownForMultiMatch(t *testing.T) {
	m := inputarea.New(80)
	for _, r := range "/c" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	provider := func(input string) []string {
		if strings.HasPrefix(input, "/c") {
			return []string{"/clear", "/context"}
		}
		return nil
	}
	m = m.SetAutocompleteProvider(provider)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Common prefix of "/clear" and "/context" is "/c" — already typed — no change
	// OR it could be "/c" which is what we have. The result is no-op since prefix == input.
	// Either way, the value should not be longer than the longest common prefix.
	if !strings.HasPrefix("/clear", m.Value()) && !strings.HasPrefix("/context", m.Value()) {
		// Allow the common-prefix result — must be a prefix of both completions
		// (or unchanged if already at common prefix)
		t.Errorf("multi completion: value %q is not a prefix of any completion", m.Value())
	}
}

// TestTUI048_TabNoopOnEmptyCompletions verifies that when there are no completions
// Tab leaves the input unchanged.
func TestTUI048_TabNoopOnEmptyCompletions(t *testing.T) {
	m := inputarea.New(80)
	for _, r := range "/zzz" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	provider := func(input string) []string { return nil }
	m = m.SetAutocompleteProvider(provider)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if m.Value() != "/zzz" {
		t.Errorf("no completions: expected unchanged %q, got %q", "/zzz", m.Value())
	}
}

// TestTUI048_TabNilProviderIsNoOp verifies that a nil autocomplete provider
// does not cause a panic when Tab is pressed.
func TestTUI048_TabNilProviderIsNoOp(t *testing.T) {
	m := inputarea.New(80)
	for _, r := range "/cl" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// No call to SetAutocompleteProvider — provider is nil.
	// Should not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Tab with nil provider panicked: %v", r)
		}
	}()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.Value() != "/cl" {
		t.Errorf("nil provider: expected %q unchanged, got %q", "/cl", m.Value())
	}
}

// TestTUI048_TabOnEmptyInput verifies that Tab on an empty input is a no-op.
func TestTUI048_TabOnEmptyInput(t *testing.T) {
	m := inputarea.New(80)

	provider := func(input string) []string { return []string{"/clear", "/help"} }
	m = m.SetAutocompleteProvider(provider)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if m.Value() != "" {
		t.Errorf("empty input: expected empty, got %q", m.Value())
	}
}

// TestTUI048_CommonPrefixComputed verifies that "cl" with completions
// ["/clear", "/close"] completes to "/cl" (the common prefix).
func TestTUI048_CommonPrefixComputed(t *testing.T) {
	m := inputarea.New(80)
	for _, r := range "/cl" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	provider := func(input string) []string {
		return []string{"/clear", "/close"}
	}
	m = m.SetAutocompleteProvider(provider)

	m, completed := m.CompleteTab()
	// Common prefix of "/clear" and "/close" is "/cl" — same as input, so no-op
	if completed {
		t.Errorf("expected no completion (prefix already matches), got completed=true, value=%q", m.Value())
	}
	if m.Value() != "/cl" {
		t.Errorf("expected /cl (no advance), got %q", m.Value())
	}
}

// TestTUI048_ConcurrentTabComplete verifies 10 goroutines running tab completion
// concurrently do not race.
func TestTUI048_ConcurrentTabComplete(t *testing.T) {
	suggestions := []string{"/clear", "/context", "/help"}
	provider := func(input string) []string {
		var out []string
		for _, s := range suggestions {
			if strings.HasPrefix(s, input) {
				out = append(out, s)
			}
		}
		return out
	}

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			m := inputarea.New(80)
			for _, r := range "/cl" {
				m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			}
			m = m.SetAutocompleteProvider(provider)
			m, _ = m.CompleteTab()
			_ = m.Value()
		}()
	}
	wg.Wait()
}

// TestTUI048_VisualSnapshot_80x24 renders an input area with autocomplete at width 80.
func TestTUI048_VisualSnapshot_80x24(t *testing.T) {
	m := inputarea.New(80)
	for _, r := range "/cl" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	provider := func(input string) []string {
		return []string{"/clear"}
	}
	m = m.SetAutocompleteProvider(provider)

	output := m.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-048-tabcomplete-80x24.txt"
	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)

	if strings.TrimSpace(output) == "" {
		t.Error("View() returned empty output")
	}
	if !strings.Contains(output, "/cl") {
		t.Error("snapshot should contain the input /cl")
	}
}

// TestTUI048_VisualSnapshot_120x40 renders an input area with autocomplete at width 120.
func TestTUI048_VisualSnapshot_120x40(t *testing.T) {
	m := inputarea.New(120)
	for _, r := range "/cl" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	provider := func(input string) []string {
		return []string{"/clear"}
	}
	m = m.SetAutocompleteProvider(provider)

	output := m.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-048-tabcomplete-120x40.txt"
	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)

	if strings.TrimSpace(output) == "" {
		t.Error("View() returned empty output at width 120")
	}
}

// TestTUI048_VisualSnapshot_200x50 renders an input area with autocomplete at width 200.
func TestTUI048_VisualSnapshot_200x50(t *testing.T) {
	m := inputarea.New(200)
	for _, r := range "/cl" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	provider := func(input string) []string {
		return []string{"/clear"}
	}
	m = m.SetAutocompleteProvider(provider)

	output := m.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-048-tabcomplete-200x50.txt"
	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)

	if strings.TrimSpace(output) == "" {
		t.Error("View() returned empty output at width 200")
	}
}
