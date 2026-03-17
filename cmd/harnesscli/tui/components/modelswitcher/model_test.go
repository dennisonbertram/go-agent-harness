package modelswitcher_test

import (
	"os"
	"strings"
	"sync"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/modelswitcher"
)

// writeSnapshot writes a visual snapshot to the package-local testdata/snapshots directory.
func writeSnapshot(t *testing.T, name, content string) {
	t.Helper()
	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating snapshot dir: %v", err)
	}
	path := dir + "/" + name
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing snapshot %s: %v", path, err)
	}
	t.Logf("snapshot written to %s", path)
}

// ─── New() tests ─────────────────────────────────────────────────────────────

// TestTUI057_NewStartsClosed verifies that a freshly created model starts closed.
func TestTUI057_NewStartsClosed(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini")
	if m.IsVisible() {
		t.Fatal("New() model should start closed (IsVisible() == false)")
	}
}

// TestTUI057_NewMarksCurrent verifies that New() marks the supplied ID as current.
func TestTUI057_NewMarksCurrent(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini")
	cur := m.CurrentModel()
	if !cur.IsCurrent {
		t.Errorf("CurrentModel().IsCurrent should be true, got false; ID=%q", cur.ID)
	}
	if cur.ID != "gpt-4.1-mini" {
		t.Errorf("CurrentModel().ID = %q, want %q", cur.ID, "gpt-4.1-mini")
	}
}

// TestTUI057_NewUnknownIDFallsBackToFirst verifies that when currentModelID does not
// match any entry, CurrentModel() returns the first entry.
func TestTUI057_NewUnknownIDFallsBackToFirst(t *testing.T) {
	m := modelswitcher.New("nonexistent-model")
	cur := m.CurrentModel()
	if cur.ID != modelswitcher.DefaultModels[0].ID {
		t.Errorf("with unknown ID, CurrentModel().ID = %q, want %q", cur.ID, modelswitcher.DefaultModels[0].ID)
	}
}

// TestTUI057_NewEmptyIDFallsBackToFirst verifies empty string currentModelID falls back to first.
func TestTUI057_NewEmptyIDFallsBackToFirst(t *testing.T) {
	m := modelswitcher.New("")
	cur := m.CurrentModel()
	if cur.ID != modelswitcher.DefaultModels[0].ID {
		t.Errorf("with empty ID, CurrentModel().ID = %q, want %q", cur.ID, modelswitcher.DefaultModels[0].ID)
	}
}

// TestTUI057_NewHasDefaultModels verifies the default models list is populated.
func TestTUI057_NewHasDefaultModels(t *testing.T) {
	if len(modelswitcher.DefaultModels) == 0 {
		t.Fatal("DefaultModels must not be empty")
	}
	// Verify expected models are present (from the catalog).
	want := []string{
		"gpt-4.1", "gpt-4.1-mini",
		"claude-sonnet-4-6", "claude-opus-4-6", "claude-haiku-4-5-20251001",
		"gemini-2.5-flash", "gemini-2.0-flash",
		"deepseek-chat", "deepseek-reasoner",
		"grok-3-mini", "grok-4-1-fast-reasoning",
		"llama-3.3-70b-versatile", "qwen-qwq-32b",
		"qwen-plus", "qwen-turbo",
		"kimi-k2.5",
	}
	ids := make(map[string]bool)
	for _, dm := range modelswitcher.DefaultModels {
		ids[dm.ID] = true
	}
	for _, id := range want {
		if !ids[id] {
			t.Errorf("expected model ID %q not found in DefaultModels", id)
		}
	}
}

// ─── Open/Close/IsVisible tests ──────────────────────────────────────────────

// TestTUI057_OpenSetsVisible verifies Open() makes IsVisible() true.
func TestTUI057_OpenSetsVisible(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").Open()
	if !m.IsVisible() {
		t.Fatal("Open() should set IsVisible() to true")
	}
}

// TestTUI057_CloseSetsNotVisible verifies Close() makes IsVisible() false.
func TestTUI057_CloseSetsNotVisible(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").Open().Close()
	if m.IsVisible() {
		t.Fatal("Close() should set IsVisible() to false")
	}
}

// TestTUI057_OpenClosedImmutable verifies Open() returns a new Model without mutating original.
func TestTUI057_OpenClosedImmutable(t *testing.T) {
	m1 := modelswitcher.New("gpt-4.1-mini")
	m2 := m1.Open()
	if m1.IsVisible() {
		t.Error("Open() must not mutate the original model")
	}
	if !m2.IsVisible() {
		t.Error("m2 should be visible after Open()")
	}
}

// ─── SelectUp / SelectDown tests ─────────────────────────────────────────────

// TestTUI057_SelectDownMovesForward verifies SelectDown() advances the selection.
func TestTUI057_SelectDownMovesForward(t *testing.T) {
	m := modelswitcher.New(modelswitcher.DefaultModels[0].ID)
	m2 := m.SelectDown()
	entry, _ := m2.Accept()
	if entry.ID == modelswitcher.DefaultModels[0].ID {
		t.Error("SelectDown() should advance past the first entry")
	}
	if entry.ID != modelswitcher.DefaultModels[1].ID {
		t.Errorf("SelectDown() from index 0: ID = %q, want %q", entry.ID, modelswitcher.DefaultModels[1].ID)
	}
}

// TestTUI057_SelectUpMovesBack verifies SelectUp() moves the selection back.
func TestTUI057_SelectUpMovesBack(t *testing.T) {
	m := modelswitcher.New(modelswitcher.DefaultModels[0].ID).SelectDown() // at index 1
	m2 := m.SelectUp()
	entry, _ := m2.Accept()
	if entry.ID != modelswitcher.DefaultModels[0].ID {
		t.Errorf("SelectUp() from index 1: ID = %q, want %q", entry.ID, modelswitcher.DefaultModels[0].ID)
	}
}

// TestTUI057_SelectDownWrapsAround verifies SelectDown() wraps from last to first.
func TestTUI057_SelectDownWrapsAround(t *testing.T) {
	last := len(modelswitcher.DefaultModels) - 1
	m := modelswitcher.New(modelswitcher.DefaultModels[0].ID)
	for i := 0; i < last; i++ {
		m = m.SelectDown()
	}
	// Now at last index — one more SelectDown should wrap.
	m = m.SelectDown()
	entry, _ := m.Accept()
	if entry.ID != modelswitcher.DefaultModels[0].ID {
		t.Errorf("SelectDown() wrap: ID = %q, want %q", entry.ID, modelswitcher.DefaultModels[0].ID)
	}
}

// TestTUI057_SelectUpWrapsAround verifies SelectUp() wraps from first to last.
func TestTUI057_SelectUpWrapsAround(t *testing.T) {
	m := modelswitcher.New(modelswitcher.DefaultModels[0].ID)
	m = m.SelectUp() // from 0 → last
	entry, _ := m.Accept()
	last := modelswitcher.DefaultModels[len(modelswitcher.DefaultModels)-1]
	if entry.ID != last.ID {
		t.Errorf("SelectUp() wrap: ID = %q, want %q", entry.ID, last.ID)
	}
}

// ─── Accept() tests ───────────────────────────────────────────────────────────

// TestTUI057_AcceptReturnsSelectedEntry verifies Accept() returns the selected entry.
func TestTUI057_AcceptReturnsSelectedEntry(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").SelectDown() // index 1
	entry, changed := m.Accept()
	if entry.ID != modelswitcher.DefaultModels[1].ID {
		t.Errorf("Accept(): entry.ID = %q, want %q", entry.ID, modelswitcher.DefaultModels[1].ID)
	}
	// gpt-4.1 is current; index 1 is gpt-4.1-mini — that is a change.
	if !changed {
		t.Error("Accept() should return changed=true when selected != current")
	}
}

// TestTUI057_AcceptReturnsFalseWhenUnchanged verifies Accept() returns changed=false
// when the selected entry is already current.
func TestTUI057_AcceptReturnsFalseWhenUnchanged(t *testing.T) {
	// New("gpt-4.1-mini") marks gpt-4.1-mini as current and sets Selected to its index.
	// Accept() without any navigation should return changed=false.
	m := modelswitcher.New("gpt-4.1-mini")
	entry, changed := m.Accept()
	if entry.ID != "gpt-4.1-mini" {
		t.Errorf("Accept(): entry.ID = %q, want %q", entry.ID, "gpt-4.1-mini")
	}
	if changed {
		t.Error("Accept() should return changed=false when selected == current")
	}
}

// ─── CurrentModel() tests ─────────────────────────────────────────────────────

// TestTUI057_CurrentModelReturnsCurrent verifies CurrentModel() returns the IsCurrent entry.
func TestTUI057_CurrentModelReturnsCurrent(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner")
	cur := m.CurrentModel()
	if cur.ID != "deepseek-reasoner" {
		t.Errorf("CurrentModel().ID = %q, want %q", cur.ID, "deepseek-reasoner")
	}
	if !cur.IsCurrent {
		t.Error("CurrentModel().IsCurrent should be true")
	}
}

// TestTUI057_CurrentModelFallsBackToFirstWhenNoneMarked verifies that if no entry
// has IsCurrent=true, CurrentModel() returns the first entry.
func TestTUI057_CurrentModelFallsBackToFirstWhenNoneMarked(t *testing.T) {
	m := modelswitcher.New("") // unknown id → none marked
	cur := m.CurrentModel()
	if cur.ID != modelswitcher.DefaultModels[0].ID {
		t.Errorf("CurrentModel() fallback: ID = %q, want %q", cur.ID, modelswitcher.DefaultModels[0].ID)
	}
}

// ─── View tests ───────────────────────────────────────────────────────────────

// TestTUI057_ViewReturnsEmptyWhenNotVisible verifies View() returns "" when not open.
func TestTUI057_ViewReturnsEmptyWhenNotVisible(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini")
	v := m.View(80)
	if v != "" {
		t.Errorf("View() should return empty string when not visible, got %q", v)
	}
}

// TestTUI057_ViewContainsTitle verifies "Switch Model" appears in the visible view.
func TestTUI057_ViewContainsTitle(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").Open()
	v := m.View(80)
	if !strings.Contains(v, "Switch Model") {
		t.Errorf("View() should contain 'Switch Model' title:\n%s", v)
	}
}

// TestTUI057_ViewContainsModelNames verifies that all model display names appear.
func TestTUI057_ViewContainsModelNames(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").Open()
	v := m.View(80)
	for _, dm := range modelswitcher.DefaultModels {
		if !strings.Contains(v, dm.DisplayName) {
			t.Errorf("View() should contain display name %q:\n%s", dm.DisplayName, v)
		}
	}
}

// TestTUI057_ViewContainsCurrentMarker verifies the current model row shows "← current".
func TestTUI057_ViewContainsCurrentMarker(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").Open()
	v := m.View(80)
	if !strings.Contains(v, "← current") {
		t.Errorf("View() should contain '← current' marker:\n%s", v)
	}
}

// TestTUI057_ViewContainsFooter verifies the navigation hint footer is present.
func TestTUI057_ViewContainsFooter(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").Open()
	v := m.View(80)
	if !strings.Contains(v, "navigate") {
		t.Errorf("View() should contain 'navigate' in footer:\n%s", v)
	}
}

// TestTUI057_ViewEmptyModelsShowsMessage verifies "No models available" when models empty.
func TestTUI057_ViewEmptyModelsShowsMessage(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini")
	// Build a model with no entries by accessing the exported struct directly.
	// We do this by building a fresh Model and checking the empty branch via View.
	// The only way to get empty models is through the Model struct directly.
	// Since Model is exported with Models field, we can set it empty.
	m2 := modelswitcher.Model{
		Models:   nil,
		Selected: 0,
		IsOpen:   true,
		Width:    80,
	}
	v := m2.View(80)
	if !strings.Contains(v, "No models available") {
		t.Errorf("View() should show 'No models available' for empty models:\n%s", v)
	}
	_ = m
}

// TestTUI057_ViewNoPanicAtExtremeWidths verifies no panic at boundary widths.
func TestTUI057_ViewNoPanicAtExtremeWidths(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").Open()
	for _, w := range []int{10, 20, 80, 200, 0} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("View(%d) panicked: %v", w, r)
				}
			}()
			_ = m.View(w)
		}()
	}
}

// ─── Concurrency test ─────────────────────────────────────────────────────────

// TestTUI057_ConcurrentModels verifies that 10 goroutines each holding their own
// Model instance have no data races.
func TestTUI057_ConcurrentModels(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			m := modelswitcher.New("gpt-4.1-mini")
			m = m.Open()
			m = m.SelectDown()
			m = m.SelectUp()
			_, _ = m.Accept()
			_ = m.CurrentModel()
			_ = m.IsVisible()
			_ = m.View(80)
			m = m.Close()
		}()
	}
	wg.Wait()
}

// ─── Snapshot tests ───────────────────────────────────────────────────────────

// TestTUI057_VisualSnapshot_80x24 captures the 80x24 visual snapshot.
func TestTUI057_VisualSnapshot_80x24(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").Open()
	snapshot := m.View(80)
	writeSnapshot(t, "TUI-057-modelswitcher-80x24.txt", snapshot)

	if strings.TrimSpace(snapshot) == "" {
		t.Error("View() returned empty output at width=80")
	}
	if !strings.Contains(snapshot, "Switch Model") {
		t.Error("snapshot should contain 'Switch Model'")
	}
}

// TestTUI057_VisualSnapshot_120x40 captures the 120x40 visual snapshot.
func TestTUI057_VisualSnapshot_120x40(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").Open()
	snapshot := m.View(120)
	writeSnapshot(t, "TUI-057-modelswitcher-120x40.txt", snapshot)

	if strings.TrimSpace(snapshot) == "" {
		t.Error("View() returned empty output at width=120")
	}
}

// ─── ReasoningMode field tests ────────────────────────────────────────────────

// TestTUI137_ReasoningModeFieldDeepSeekReasoner verifies deepseek-reasoner has ReasoningMode=true.
func TestTUI137_ReasoningModeFieldDeepSeekReasoner(t *testing.T) {
	for _, dm := range modelswitcher.DefaultModels {
		if dm.ID == "deepseek-reasoner" {
			if !dm.ReasoningMode {
				t.Error("deepseek-reasoner should have ReasoningMode=true")
			}
			return
		}
	}
	t.Fatal("deepseek-reasoner not found in DefaultModels")
}

// TestTUI137_ReasoningModeFieldQwQ32B verifies qwen-qwq-32b has ReasoningMode=true.
func TestTUI137_ReasoningModeFieldQwQ32B(t *testing.T) {
	for _, dm := range modelswitcher.DefaultModels {
		if dm.ID == "qwen-qwq-32b" {
			if !dm.ReasoningMode {
				t.Error("qwen-qwq-32b should have ReasoningMode=true")
			}
			return
		}
	}
	t.Fatal("qwen-qwq-32b not found in DefaultModels")
}

// TestTUI137_ReasoningModeFieldGPT41 verifies gpt-4.1 has ReasoningMode=false.
func TestTUI137_ReasoningModeFieldGPT41(t *testing.T) {
	for _, dm := range modelswitcher.DefaultModels {
		if dm.ID == "gpt-4.1" {
			if dm.ReasoningMode {
				t.Error("gpt-4.1 should have ReasoningMode=false")
			}
			return
		}
	}
	t.Fatal("gpt-4.1 not found in DefaultModels")
}

// TestTUI137_ReasoningLevelCount verifies ReasoningLevels has 4 entries.
func TestTUI137_ReasoningLevelCount(t *testing.T) {
	if len(modelswitcher.ReasoningLevels) != 4 {
		t.Errorf("ReasoningLevels should have 4 entries, got %d", len(modelswitcher.ReasoningLevels))
	}
}

// TestTUI137_ReasoningLevelIDs verifies ReasoningLevels has correct IDs.
func TestTUI137_ReasoningLevelIDs(t *testing.T) {
	want := []string{"", "low", "medium", "high"}
	for i, rl := range modelswitcher.ReasoningLevels {
		if rl.ID != want[i] {
			t.Errorf("ReasoningLevels[%d].ID = %q, want %q", i, rl.ID, want[i])
		}
	}
}

// TestTUI137_EnterExitReasoningModeToggle verifies toggling reasoning mode.
func TestTUI137_EnterExitReasoningModeToggle(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner")
	if m.IsReasoningMode() {
		t.Fatal("New model should not be in reasoning mode")
	}
	m2 := m.EnterReasoningMode()
	if !m2.IsReasoningMode() {
		t.Error("EnterReasoningMode() should set IsReasoningMode() to true")
	}
	m3 := m2.ExitReasoningMode()
	if m3.IsReasoningMode() {
		t.Error("ExitReasoningMode() should set IsReasoningMode() to false")
	}
}

// TestTUI137_ReasoningUpDownWrap verifies ReasoningUp/Down wrap at boundaries.
func TestTUI137_ReasoningUpDownWrap(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner").EnterReasoningMode()
	// reasoningSelected starts at 0 ("Default").
	re, _ := m.AcceptReasoning()
	if re.ID != "" {
		t.Errorf("initial reasoning should be Default (''), got %q", re.ID)
	}

	// Down from 0 → 1 ("low")
	m = m.ReasoningDown()
	re, _ = m.AcceptReasoning()
	if re.ID != "low" {
		t.Errorf("after ReasoningDown: ID = %q, want %q", re.ID, "low")
	}

	// Up from 1 → 0 ("Default")
	m = m.ReasoningUp()
	re, _ = m.AcceptReasoning()
	if re.ID != "" {
		t.Errorf("after ReasoningUp: ID = %q, want Default ('')", re.ID)
	}

	// Up from 0 → wraps to last ("high")
	m = m.ReasoningUp()
	re, _ = m.AcceptReasoning()
	if re.ID != "high" {
		t.Errorf("ReasoningUp wrap: ID = %q, want %q", re.ID, "high")
	}

	// Down from last ("high") → wraps to 0 ("Default")
	m = m.ReasoningDown()
	re, _ = m.AcceptReasoning()
	if re.ID != "" {
		t.Errorf("ReasoningDown wrap: ID = %q, want Default ('')", re.ID)
	}
}

// TestTUI137_AcceptReasoningChangedBool verifies AcceptReasoning changed bool.
func TestTUI137_AcceptReasoningChangedBool(t *testing.T) {
	// Set currentReasoning to "low", cursor at "low" → changed=false.
	m := modelswitcher.New("deepseek-reasoner").WithCurrentReasoning("low").EnterReasoningMode()
	// Cursor should be initialised to "low" (index 1).
	re, changed := m.AcceptReasoning()
	if re.ID != "low" {
		t.Errorf("AcceptReasoning: ID = %q, want %q", re.ID, "low")
	}
	if changed {
		t.Error("AcceptReasoning: changed should be false when cursor == current")
	}

	// Move to "medium" → changed=true.
	m2 := m.ReasoningDown()
	re2, changed2 := m2.AcceptReasoning()
	if re2.ID != "medium" {
		t.Errorf("AcceptReasoning: ID = %q, want %q", re2.ID, "medium")
	}
	if !changed2 {
		t.Error("AcceptReasoning: changed should be true when cursor != current")
	}
}

// TestTUI137_WithCurrentReasoningPersists verifies WithCurrentReasoning sets the value.
func TestTUI137_WithCurrentReasoningPersists(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner").WithCurrentReasoning("high")
	m2 := m.EnterReasoningMode()
	re, _ := m2.AcceptReasoning()
	if re.ID != "high" {
		t.Errorf("WithCurrentReasoning+EnterReasoningMode: cursor should start at 'high', got %q", re.ID)
	}
}

// TestTUI137_ValueSemanticsEnterReasoning verifies EnterReasoningMode does not mutate original.
func TestTUI137_ValueSemanticsEnterReasoning(t *testing.T) {
	m1 := modelswitcher.New("deepseek-reasoner")
	_ = m1.EnterReasoningMode()
	if m1.IsReasoningMode() {
		t.Error("EnterReasoningMode() must not mutate the original model")
	}
}
