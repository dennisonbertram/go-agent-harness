package contextgrid

import (
	"strings"
	"testing"
)

func TestViewDefaultsTotalTokensAndWidth(t *testing.T) {
	t.Parallel()

	view := New().View()

	if !strings.Contains(view, "Context Window Usage") {
		t.Fatalf("expected header in view, got %q", view)
	}
	if !strings.Contains(view, "Used:  0 tokens") {
		t.Fatalf("expected zero used tokens in default view, got %q", view)
	}
	if !strings.Contains(view, "Total: 200000 tokens") {
		t.Fatalf("expected default total token fallback, got %q", view)
	}
	if !strings.Contains(view, "Usage: 0.0%") {
		t.Fatalf("expected default usage percentage, got %q", view)
	}

	barWidth := progressBarWidth(t, view)
	if barWidth != 60 {
		t.Fatalf("expected default width fallback to cap the bar at 60 cells, got %d", barWidth)
	}
}

func TestViewClampsUsedTokensIntoValidRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		model        Model
		wantUsedLine string
		wantUsage    string
		wantFilled   int
	}{
		{
			name: "negative used tokens clamp to zero",
			model: Model{
				TotalTokens: 100,
				UsedTokens:  -25,
				Width:       20,
			},
			wantUsedLine: "Used:  0 tokens",
			wantUsage:    "Usage: 0.0%",
			wantFilled:   0,
		},
		{
			name: "used tokens above total clamp to total",
			model: Model{
				TotalTokens: 100,
				UsedTokens:  120,
				Width:       20,
			},
			wantUsedLine: "Used:  100 tokens",
			wantUsage:    "Usage: 100.0%",
			wantFilled:   18,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			view := tt.model.View()
			if !strings.Contains(view, tt.wantUsedLine) {
				t.Fatalf("expected %q in view, got %q", tt.wantUsedLine, view)
			}
			if !strings.Contains(view, tt.wantUsage) {
				t.Fatalf("expected %q in view, got %q", tt.wantUsage, view)
			}

			barLine := progressBarLine(t, view)
			if filled := strings.Count(barLine, "█"); filled != tt.wantFilled {
				t.Fatalf("expected %d filled cells, got %d in %q", tt.wantFilled, filled, barLine)
			}
		})
	}
}

func TestViewCapsProgressBarAtMaximumWidth(t *testing.T) {
	t.Parallel()

	view := Model{
		TotalTokens: 100,
		UsedTokens:  50,
		Width:       200,
	}.View()

	if barWidth := progressBarWidth(t, view); barWidth != 60 {
		t.Fatalf("expected large widths to cap the progress bar at 60 cells, got %d", barWidth)
	}
	if !strings.Contains(view, "Usage: 50.0%") {
		t.Fatalf("expected percentage formatting to remain stable, got %q", view)
	}
}

func TestViewKeepsProgressBarWithinRequestedWidth(t *testing.T) {
	t.Parallel()

	const width = 8
	view := Model{
		TotalTokens: 100,
		UsedTokens:  50,
		Width:       width,
	}.View()

	barLine := progressBarLine(t, view)
	if len([]rune(barLine)) > width {
		t.Fatalf("expected bar line %q to fit within width %d", barLine, width)
	}
}

func progressBarWidth(t *testing.T, view string) int {
	t.Helper()

	barLine := progressBarLine(t, view)
	return len([]rune(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(barLine), "["), "]")))
}

func progressBarLine(t *testing.T, view string) string {
	t.Helper()

	for _, line := range strings.Split(view, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			return trimmed
		}
	}

	t.Fatalf("progress bar line not found in view %q", view)
	return ""
}
