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
// appear on the gpt-4.1 row. (It can appear on other rows for o3/o4-mini.)
func TestTUI137_ViewLevel0NoReasoningBadgeForGPT41(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open()
	v := m.View(80)
	// gpt-4.1 row should NOT have [R] next to it — but other rows may have it.
	// We verify that the view contains model names without [R] on the gpt-4.1 line.
	lines := strings.Split(v, "\n")
	for _, line := range lines {
		if strings.Contains(line, "GPT-4.1") && !strings.Contains(line, "GPT-4.1 Mini") && !strings.Contains(line, "GPT-4.1 Nano") {
			if strings.Contains(line, "[R]") {
				t.Errorf("gpt-4.1 row should not contain '[R]': %q", line)
			}
		}
	}
}

// TestTUI137_ViewLevel1ContainsReasoningEffortTitle verifies "Reasoning Effort" title.
func TestTUI137_ViewLevel1ContainsReasoningEffortTitle(t *testing.T) {
	m := modelswitcher.New("o3").Open().EnterReasoningMode()
	v := m.View(80)
	if !strings.Contains(v, "Reasoning Effort") {
		t.Errorf("Level-1 view should contain 'Reasoning Effort' title:\n%s", v)
	}
}

// TestTUI137_ViewLevel1ListsAllFourLevels verifies all 4 reasoning levels are shown.
func TestTUI137_ViewLevel1ListsAllFourLevels(t *testing.T) {
	m := modelswitcher.New("o3").Open().EnterReasoningMode()
	v := m.View(80)
	for _, rl := range modelswitcher.ReasoningLevels {
		if !strings.Contains(v, rl.DisplayName) {
			t.Errorf("Level-1 view missing reasoning level %q:\n%s", rl.DisplayName, v)
		}
	}
}

// TestTUI137_ViewLevel1FooterContainsEscBack verifies "esc back" is in the footer.
func TestTUI137_ViewLevel1FooterContainsEscBack(t *testing.T) {
	m := modelswitcher.New("o3").Open().EnterReasoningMode()
	v := m.View(80)
	if !strings.Contains(v, "esc back") {
		t.Errorf("Level-1 view footer should contain 'esc back':\n%s", v)
	}
}

// TestTUI137_ViewLevel1ReturnsEmptyWhenClosed verifies View("") when not open.
func TestTUI137_ViewLevel1ReturnsEmptyWhenClosed(t *testing.T) {
	m := modelswitcher.New("o3").EnterReasoningMode() // not opened
	v := m.View(80)
	if v != "" {
		t.Errorf("View() should return empty when not open, got %q", v)
	}
}

// TestTUI137_ViewLevel1ShowsModelNameInTitle verifies model name appears in Level-1 title.
func TestTUI137_ViewLevel1ShowsModelNameInTitle(t *testing.T) {
	m := modelswitcher.New("o3").Open().EnterReasoningMode()
	v := m.View(80)
	if !strings.Contains(v, "o3") {
		t.Errorf("Level-1 view title should contain model name 'o3':\n%s", v)
	}
}

// TestTUI137_ViewLevel1NoPanicAtExtremeWidths verifies no panic at boundary widths.
func TestTUI137_ViewLevel1NoPanicAtExtremeWidths(t *testing.T) {
	m := modelswitcher.New("o3").Open().EnterReasoningMode()
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
