package contextgrid_test

import (
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/contextgrid"
)

func TestContextGridView_DefaultFallbackAndFormatting(t *testing.T) {
	m := contextgrid.Model{
		UsedTokens: 1000,
	}

	view := m.View()

	for _, want := range []string{
		"Context Window Usage",
		"Used:  1000 tokens",
		"Total: 200000 tokens",
		"Usage: 0.5%",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
	}

	barLine := strings.Split(view, "\n")[2]
	if got := strings.Count(barLine, "░") + strings.Count(barLine, "█"); got != 60 {
		t.Fatalf("default width must clamp progress bar to 60 cells, got %d in %q", got, barLine)
	}
}

func TestContextGridView_ClampsUsageAndRespectsNarrowWidth(t *testing.T) {
	t.Run("negative usage clamps to zero", func(t *testing.T) {
		m := contextgrid.Model{
			TotalTokens: 100,
			UsedTokens:  -50,
			Width:       30,
		}

		view := m.View()
		if !strings.Contains(view, "Used:  0 tokens") {
			t.Fatalf("View() must clamp negative usage to zero:\n%s", view)
		}
		if !strings.Contains(view, "Usage: 0.0%") {
			t.Fatalf("View() must report zero percent after negative clamp:\n%s", view)
		}
	})

	t.Run("usage above total clamps to total", func(t *testing.T) {
		m := contextgrid.Model{
			TotalTokens: 100,
			UsedTokens:  250,
			Width:       30,
		}

		view := m.View()
		if !strings.Contains(view, "Used:  100 tokens") {
			t.Fatalf("View() must clamp usage to total tokens:\n%s", view)
		}
		if !strings.Contains(view, "Usage: 100.0%") {
			t.Fatalf("View() must report full usage after clamp:\n%s", view)
		}
	})

	t.Run("narrow width does not overflow the requested width", func(t *testing.T) {
		m := contextgrid.Model{
			TotalTokens: 100,
			UsedTokens:  50,
			Width:       8,
		}

		view := m.View()
		for _, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
			if len([]rune(line)) > m.Width {
				t.Fatalf("line %q exceeds requested width %d", line, m.Width)
			}
		}
	})
}
