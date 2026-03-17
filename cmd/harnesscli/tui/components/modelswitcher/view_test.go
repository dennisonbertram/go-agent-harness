package modelswitcher_test

import (
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/modelswitcher"
)

// TestTUI137_ViewLevel0ContainsReasoningBadgeO3 verifies [R] badge appears for o3.
func TestTUI137_ViewLevel0ContainsReasoningBadgeO3(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open()
	v := m.View(80)
	if !strings.Contains(v, "[R]") {
		t.Errorf("Level-0 view should contain '[R]' badge for o3/o4-mini:\n%s", v)
	}
}

// TestTUI137_ViewLevel0NoReasoningBadgeForGPT41 verifies [R] badge does NOT
// appear on the gpt-4.1 row. (It can appear on other rows for reasoning models.)
func TestTUI137_ViewLevel0NoReasoningBadgeForGPT41(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open()
	v := m.View(80)
	// gpt-4.1 row should NOT have [R] next to it — but other rows may have it.
	// We verify that the view contains model names without [R] on the gpt-4.1 line.
	lines := strings.Split(v, "\n")
	for _, line := range lines {
		if strings.Contains(line, "GPT-4.1") && !strings.Contains(line, "GPT-4.1 Mini") {
			if strings.Contains(line, "[R]") {
				t.Errorf("gpt-4.1 row should not contain '[R]': %q", line)
			}
		}
	}
}

// TestTUI137_ViewLevel1ContainsReasoningEffortTitle verifies "Reasoning Effort" title.
func TestTUI137_ViewLevel1ContainsReasoningEffortTitle(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner").Open().EnterReasoningMode()
	v := m.View(80)
	if !strings.Contains(v, "Reasoning Effort") {
		t.Errorf("Level-1 view should contain 'Reasoning Effort' title:\n%s", v)
	}
}

// TestTUI137_ViewLevel1ListsAllFourLevels verifies all 4 reasoning levels are shown.
func TestTUI137_ViewLevel1ListsAllFourLevels(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner").Open().EnterReasoningMode()
	v := m.View(80)
	for _, rl := range modelswitcher.ReasoningLevels {
		if !strings.Contains(v, rl.DisplayName) {
			t.Errorf("Level-1 view missing reasoning level %q:\n%s", rl.DisplayName, v)
		}
	}
}

// TestTUI137_ViewLevel1FooterContainsEscBack verifies "esc back" is in the footer.
func TestTUI137_ViewLevel1FooterContainsEscBack(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner").Open().EnterReasoningMode()
	v := m.View(80)
	if !strings.Contains(v, "esc back") {
		t.Errorf("Level-1 view footer should contain 'esc back':\n%s", v)
	}
}

// TestTUI137_ViewLevel1ReturnsEmptyWhenClosed verifies View("") when not open.
func TestTUI137_ViewLevel1ReturnsEmptyWhenClosed(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner").EnterReasoningMode() // not opened
	v := m.View(80)
	if v != "" {
		t.Errorf("View() should return empty when not open, got %q", v)
	}
}

// TestTUI137_ViewLevel1ShowsModelNameInTitle verifies model name appears in Level-1 title.
func TestTUI137_ViewLevel1ShowsModelNameInTitle(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner").Open().EnterReasoningMode()
	v := m.View(80)
	if !strings.Contains(v, "DeepSeek Reasoner") {
		t.Errorf("Level-1 view title should contain model name 'DeepSeek Reasoner':\n%s", v)
	}
}

// TestTUI137_ViewLevel1NoPanicAtExtremeWidths verifies no panic at boundary widths.
func TestTUI137_ViewLevel1NoPanicAtExtremeWidths(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner").Open().EnterReasoningMode()
	for _, w := range []int{0, 10, 20, 80, 200} {
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

// ─── Loading / Error / Search / Star view tests ────────────────────────────────

// TestModelSearchView_LoadingShowsIndicatorAboveList verifies "Loading models..." appears
// in the view while loading but the model list is still rendered.
func TestModelSearchView_LoadingShowsIndicatorAboveList(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open().SetLoading(true)
	v := m.View(80)
	if !strings.Contains(v, "Loading models...") {
		t.Errorf("View should contain 'Loading models...' when loading:\n%s", v)
	}
	// Model list should still be visible (DefaultModels are shown during loading).
	if !strings.Contains(v, "GPT-4.1") {
		t.Errorf("View should still show model list while loading:\n%s", v)
	}
}

// TestModelSearchView_ErrorShowsMessage verifies the error message is shown when loadError is set.
func TestModelSearchView_ErrorShowsMessage(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open().SetLoadError("Error loading models: connection refused")
	v := m.View(80)
	if !strings.Contains(v, "Error loading models") {
		t.Errorf("View should contain error message when loadError is set:\n%s", v)
	}
	// When in error state, models list is NOT shown.
	if strings.Contains(v, "GPT-4.1") {
		t.Errorf("View should NOT show model list when in error state:\n%s", v)
	}
}

// TestModelSearchView_SearchBarVisibleWhenQueryNonEmpty verifies the "Filter:" prefix
// is shown when a search query is active.
func TestModelSearchView_SearchBarVisibleWhenQueryNonEmpty(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open().SetSearch("claude")
	v := m.View(80)
	if !strings.Contains(v, "Filter:") {
		t.Errorf("View should contain 'Filter:' when search query is non-empty:\n%s", v)
	}
	if !strings.Contains(v, "claude") {
		t.Errorf("View should contain the search query text:\n%s", v)
	}
}

// TestModelSearchView_StarSymbolForStarredModel verifies starred models show the ★ prefix.
func TestModelSearchView_StarSymbolForStarredModel(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open().WithStarred([]string{"gpt-4.1"})
	v := m.View(80)
	if !strings.Contains(v, "★") {
		t.Errorf("View should contain '★' for starred model:\n%s", v)
	}
}

// TestModelSearchView_NoModelsMatchMessage verifies "No models match" is shown
// when a search query filters out all models.
func TestModelSearchView_NoModelsMatchMessage(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open().SetSearch("zzznomatch")
	v := m.View(80)
	if !strings.Contains(v, "No models match") {
		t.Errorf("View should show 'No models match' when filter yields no results:\n%s", v)
	}
}

// TestModelSearchView_SearchFilterOnlyShowsMatchingModels verifies that search
// hides non-matching models from view.
func TestModelSearchView_SearchFilterOnlyShowsMatchingModels(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open().SetSearch("claude")
	v := m.View(80)
	// Claude models should be visible.
	if !strings.Contains(v, "Claude") {
		t.Errorf("View should show Claude models when searching 'claude':\n%s", v)
	}
	// Non-Claude models should be hidden.
	if strings.Contains(v, "GPT-4.1") {
		t.Errorf("View should hide GPT models when searching 'claude':\n%s", v)
	}
}

// TestModelSearchView_EmptySearchShowsAllModels verifies all models are visible
// with an empty search query.
func TestModelSearchView_EmptySearchShowsAllModels(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open()
	v := m.View(80)
	for _, dm := range modelswitcher.DefaultModels {
		if !strings.Contains(v, dm.DisplayName) {
			t.Errorf("View should contain %q with empty search:\n%s", dm.DisplayName, v)
		}
	}
}
