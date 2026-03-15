package costdisplay_test

import (
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/costdisplay"
)

// stripANSI removes ANSI escape sequences for plain-text assertions.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// FormatTokens
// ---------------------------------------------------------------------------

func TestTUI056_FormatTokens_Zero(t *testing.T) {
	got := costdisplay.FormatTokens(0)
	if got != "0" {
		t.Errorf("FormatTokens(0) = %q, want %q", got, "0")
	}
}

func TestTUI056_FormatTokens_BelowThousand(t *testing.T) {
	got := costdisplay.FormatTokens(999)
	if got != "999" {
		t.Errorf("FormatTokens(999) = %q, want %q", got, "999")
	}
}

func TestTUI056_FormatTokens_ExactThousand(t *testing.T) {
	got := costdisplay.FormatTokens(1000)
	if got != "1,000" {
		t.Errorf("FormatTokens(1000) = %q, want %q", got, "1,000")
	}
}

func TestTUI056_FormatTokens_NineThousandNineHundredNinetyNine(t *testing.T) {
	got := costdisplay.FormatTokens(9999)
	if got != "9,999" {
		t.Errorf("FormatTokens(9999) = %q, want %q", got, "9,999")
	}
}

func TestTUI056_FormatTokens_TenThousand(t *testing.T) {
	got := costdisplay.FormatTokens(10000)
	if got != "10,000" {
		t.Errorf("FormatTokens(10000) = %q, want %q", got, "10,000")
	}
}

func TestTUI056_FormatTokens_OneMillion(t *testing.T) {
	got := costdisplay.FormatTokens(1000000)
	if got != "1,000,000" {
		t.Errorf("FormatTokens(1000000) = %q, want %q", got, "1,000,000")
	}
}

// ---------------------------------------------------------------------------
// FormatCost
// ---------------------------------------------------------------------------

func TestTUI056_FormatCost_Zero(t *testing.T) {
	got := costdisplay.FormatCost(0.0)
	if got != "$0.0000" {
		t.Errorf("FormatCost(0.0) = %q, want %q", got, "$0.0000")
	}
}

func TestTUI056_FormatCost_TinyAmount(t *testing.T) {
	got := costdisplay.FormatCost(0.0001)
	if got != "$0.0001" {
		t.Errorf("FormatCost(0.0001) = %q, want %q", got, "$0.0001")
	}
}

func TestTUI056_FormatCost_SmallAmount(t *testing.T) {
	got := costdisplay.FormatCost(0.0123)
	if got != "$0.0123" {
		t.Errorf("FormatCost(0.0123) = %q, want %q", got, "$0.0123")
	}
}

func TestTUI056_FormatCost_LargerAmount(t *testing.T) {
	got := costdisplay.FormatCost(1.2345)
	if got != "$1.2345" {
		t.Errorf("FormatCost(1.2345) = %q, want %q", got, "$1.2345")
	}
}

// ---------------------------------------------------------------------------
// Model state transitions
// ---------------------------------------------------------------------------

func TestTUI056_New_DefaultsToHidden(t *testing.T) {
	m := costdisplay.New()
	if m.IsVisible() {
		t.Error("New() model must be hidden by default")
	}
}

func TestTUI056_Show_MakesVisible(t *testing.T) {
	m := costdisplay.New().Show()
	if !m.IsVisible() {
		t.Error("Show() must make model visible")
	}
}

func TestTUI056_Hide_MakesInvisible(t *testing.T) {
	m := costdisplay.New().Show().Hide()
	if m.IsVisible() {
		t.Error("Hide() must make model invisible")
	}
}

func TestTUI056_Toggle_TogglesVisibility(t *testing.T) {
	m := costdisplay.New()
	if m.IsVisible() {
		t.Fatal("New() must start hidden")
	}
	m = m.Toggle()
	if !m.IsVisible() {
		t.Error("Toggle() from hidden must make visible")
	}
	m = m.Toggle()
	if m.IsVisible() {
		t.Error("Toggle() from visible must make hidden")
	}
}

func TestTUI056_ImmutableValueSemantics(t *testing.T) {
	original := costdisplay.New()
	shown := original.Show()

	// original must remain unchanged
	if original.IsVisible() {
		t.Error("Show() must not mutate the original model")
	}
	if !shown.IsVisible() {
		t.Error("Show() must return a visible copy")
	}
}

func TestTUI056_Update_ReplacesSnapshot(t *testing.T) {
	snap := costdisplay.CostSnapshot{
		InputTokens:  1234,
		OutputTokens: 567,
		TotalCostUSD: 0.0123,
		Model:        "gpt-4.1-mini",
	}
	m := costdisplay.New().Show().Update(snap)
	if m.Snapshot.InputTokens != 1234 {
		t.Errorf("Update() InputTokens = %d, want 1234", m.Snapshot.InputTokens)
	}
	if m.Snapshot.OutputTokens != 567 {
		t.Errorf("Update() OutputTokens = %d, want 567", m.Snapshot.OutputTokens)
	}
	if m.Snapshot.TotalCostUSD != 0.0123 {
		t.Errorf("Update() TotalCostUSD = %f, want 0.0123", m.Snapshot.TotalCostUSD)
	}
	if m.Snapshot.Model != "gpt-4.1-mini" {
		t.Errorf("Update() Model = %q, want %q", m.Snapshot.Model, "gpt-4.1-mini")
	}
}

// ---------------------------------------------------------------------------
// View output
// ---------------------------------------------------------------------------

func TestTUI056_View_HiddenReturnsEmpty(t *testing.T) {
	m := costdisplay.New() // hidden
	got := m.View()
	if got != "" {
		t.Errorf("View() when hidden must return empty string, got: %q", got)
	}
}

func TestTUI056_View_Width80_ContainsExpectedFields(t *testing.T) {
	snap := costdisplay.CostSnapshot{
		InputTokens:  1234,
		OutputTokens: 567,
		TotalCostUSD: 0.0123,
		Model:        "gpt-4.1-mini",
	}
	m := costdisplay.New().Show().Update(snap)
	m.Width = 80

	got := m.View()
	visible := stripANSI(got)

	if !strings.Contains(visible, "1,234") {
		t.Errorf("View() at width 80 must show formatted input tokens '1,234', got: %q", visible)
	}
	if !strings.Contains(visible, "567") {
		t.Errorf("View() at width 80 must show output tokens '567', got: %q", visible)
	}
	if !strings.Contains(visible, "$0.0123") {
		t.Errorf("View() at width 80 must show cost '$0.0123', got: %q", visible)
	}
	if !strings.Contains(visible, "gpt-4.1-mini") {
		t.Errorf("View() at width 80 must show model name, got: %q", visible)
	}
}

func TestTUI056_View_Width80_ContainsUpDownArrows(t *testing.T) {
	snap := costdisplay.CostSnapshot{
		InputTokens:  100,
		OutputTokens: 50,
		TotalCostUSD: 0.001,
		Model:        "gpt-4.1-mini",
	}
	m := costdisplay.New().Show().Update(snap)
	m.Width = 80

	got := m.View()
	visible := stripANSI(got)

	if !strings.Contains(visible, "↑") {
		t.Errorf("View() must contain '↑' for input tokens, got: %q", visible)
	}
	if !strings.Contains(visible, "↓") {
		t.Errorf("View() must contain '↓' for output tokens, got: %q", visible)
	}
}

func TestTUI056_View_Width80_ModelInBrackets(t *testing.T) {
	snap := costdisplay.CostSnapshot{
		InputTokens:  100,
		OutputTokens: 50,
		TotalCostUSD: 0.001,
		Model:        "gpt-4.1-mini",
	}
	m := costdisplay.New().Show().Update(snap)
	m.Width = 80

	got := m.View()
	visible := stripANSI(got)

	if !strings.Contains(visible, "[gpt-4.1-mini]") {
		t.Errorf("View() must show model in brackets '[gpt-4.1-mini]', got: %q", visible)
	}
}

func TestTUI056_View_Width120_ContainsExpectedFields(t *testing.T) {
	snap := costdisplay.CostSnapshot{
		InputTokens:  10000,
		OutputTokens: 5000,
		TotalCostUSD: 1.2345,
		Model:        "gpt-4.1",
	}
	m := costdisplay.New().Show().Update(snap)
	m.Width = 120

	got := m.View()
	visible := stripANSI(got)

	if !strings.Contains(visible, "10,000") {
		t.Errorf("View() at width 120 must show '10,000', got: %q", visible)
	}
	if !strings.Contains(visible, "5,000") {
		t.Errorf("View() at width 120 must show '5,000', got: %q", visible)
	}
	if !strings.Contains(visible, "$1.2345") {
		t.Errorf("View() at width 120 must show cost '$1.2345', got: %q", visible)
	}
	if !strings.Contains(visible, "[gpt-4.1]") {
		t.Errorf("View() at width 120 must show model in brackets, got: %q", visible)
	}
}

func TestTUI056_View_NoModelName_OmitsBrackets(t *testing.T) {
	snap := costdisplay.CostSnapshot{
		InputTokens:  100,
		OutputTokens: 50,
		TotalCostUSD: 0.001,
		Model:        "",
	}
	m := costdisplay.New().Show().Update(snap)
	m.Width = 80

	got := m.View()
	visible := stripANSI(got)

	if strings.Contains(visible, "[]") {
		t.Errorf("View() with no model must not show empty brackets '[]', got: %q", visible)
	}
}

func TestTUI056_View_ZeroTokens_ShowsZeros(t *testing.T) {
	snap := costdisplay.CostSnapshot{
		InputTokens:  0,
		OutputTokens: 0,
		TotalCostUSD: 0.0,
		Model:        "gpt-4.1-mini",
	}
	m := costdisplay.New().Show().Update(snap)
	m.Width = 80

	got := m.View()
	visible := stripANSI(got)

	// Should render without panic and show cost
	if !strings.Contains(visible, "$0.0000") {
		t.Errorf("View() with zero cost must show '$0.0000', got: %q", visible)
	}
}
