package tui_test

// statusbar_regression_test.go — regression tests for BUG-9 and BUG-11.
//
// BUG-9: Status bar cost was not updated when usage.delta SSE events arrived.
// BUG-11: Status bar lost model name after a tea.WindowSizeMsg (resize) event.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

// TestBUG9_StatusBarCostUpdatesOnUsageDelta verifies that after a usage.delta
// SSE event, the status bar directly contains the cost information.
// We use StatusBarView() to bypass the transient status message overlay.
func TestBUG9_StatusBarCostUpdatesOnUsageDelta(t *testing.T) {
	m := initModel(t, 120, 40)
	m = m.WithCancelRun(func() {})
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-bug9-1"})
	m = m2.(tui.Model)

	// Send a usage.delta event with a non-zero cost.
	m3, _ := m.Update(tui.SSEEventMsg{
		EventType: "usage.delta",
		Raw:       []byte(`{"cumulative_usage":{"total_tokens":1000},"cumulative_cost_usd":0.005}`),
	})
	m = m3.(tui.Model)

	// The statusbar component itself (not the main view with status message overlay)
	// should contain the cost. The statusbar renders cost as "$0.0050" for 0.005 USD.
	statusBarView := m.StatusBarView()
	if !strings.Contains(statusBarView, "0.005") {
		t.Errorf("status bar should contain cost after usage.delta; StatusBarView()=%q", statusBarView)
	}
}

// TestBUG9_StatusBarCostAccumulatesWithMultipleEvents verifies that the most
// recent cumulative cost from sequential usage.delta events is shown.
func TestBUG9_StatusBarCostAccumulatesWithMultipleEvents(t *testing.T) {
	m := initModel(t, 120, 40)
	m = m.WithCancelRun(func() {})
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-bug9-2"})
	m = m2.(tui.Model)

	// First event: cost 0.001
	m3, _ := m.Update(tui.SSEEventMsg{
		EventType: "usage.delta",
		Raw:       []byte(`{"cumulative_usage":{"total_tokens":500},"cumulative_cost_usd":0.001}`),
	})
	m = m3.(tui.Model)

	// Second event: cost 0.0075 (cumulative)
	m4, _ := m.Update(tui.SSEEventMsg{
		EventType: "usage.delta",
		Raw:       []byte(`{"cumulative_usage":{"total_tokens":1500},"cumulative_cost_usd":0.0075}`),
	})
	m = m4.(tui.Model)

	// The status bar should reflect the most recent cost.
	statusBarView := m.StatusBarView()
	if !strings.Contains(statusBarView, "0.0075") {
		t.Errorf("status bar should show latest cumulative cost 0.0075; StatusBarView()=%q", statusBarView)
	}
	// The previous cost (0.001) should NOT appear (was replaced, not accumulated).
	// Note: 0.001 is a substring of 0.0075 numerically but $0.0010 != $0.0075.
	if strings.Contains(statusBarView, "$0.0010") {
		t.Errorf("status bar should NOT show old cost $0.0010; StatusBarView()=%q", statusBarView)
	}
}

// TestBUG11_StatusBarPreservesModelNameOnResize verifies that after a
// tea.WindowSizeMsg (terminal resize), the status bar still shows the model name.
// Uses StatusBarView() to bypass the transient status message overlay.
// Note: displayModelName() converts "gpt-4.1" → "GPT-4.1", so we check for
// the display form "GPT-4.1".
func TestBUG11_StatusBarPreservesModelNameOnResize(t *testing.T) {
	m := initModel(t, 80, 24)

	// Set a model. displayModelName converts "gpt-4.1" → "GPT-4.1".
	m2, _ := m.Update(tui.ModelSelectedMsg{ModelID: "gpt-4.1", Provider: "openai"})
	m = m2.(tui.Model)

	// Verify model name is in the status bar before resize.
	// The display name for "gpt-4.1" is "GPT-4.1".
	statusBarBefore := m.StatusBarView()
	if !strings.Contains(strings.ToLower(statusBarBefore), "gpt-4.1") {
		t.Fatalf("precondition: model name not in status bar before resize; StatusBarView()=%q", statusBarBefore)
	}

	// Simulate resize.
	m3, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = m3.(tui.Model)

	// Model name must still appear in the status bar after resize.
	statusBarAfter := m.StatusBarView()
	if !strings.Contains(strings.ToLower(statusBarAfter), "gpt-4.1") {
		t.Errorf("status bar lost model name after resize; StatusBarView()=%q", statusBarAfter)
	}
}

// TestBUG11_StatusBarPreservesCostOnResize verifies that after a resize, the
// status bar still shows the cumulative cost set by a prior usage.delta event.
func TestBUG11_StatusBarPreservesCostOnResize(t *testing.T) {
	m := initModel(t, 80, 24)
	m = m.WithCancelRun(func() {})
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-bug11-cost-1"})
	m = m2.(tui.Model)

	// Set cost via usage.delta.
	m3, _ := m.Update(tui.SSEEventMsg{
		EventType: "usage.delta",
		Raw:       []byte(`{"cumulative_usage":{"total_tokens":800},"cumulative_cost_usd":0.0042}`),
	})
	m = m3.(tui.Model)

	// Verify cost is in the status bar before resize.
	statusBarBefore := m.StatusBarView()
	if !strings.Contains(statusBarBefore, "0.0042") {
		t.Fatalf("precondition: cost not in status bar before resize; StatusBarView()=%q", statusBarBefore)
	}

	// Simulate resize.
	m4, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = m4.(tui.Model)

	// Cost must still appear in the status bar after resize.
	statusBarAfter := m.StatusBarView()
	if !strings.Contains(statusBarAfter, "0.0042") {
		t.Errorf("status bar lost cost after resize; StatusBarView()=%q", statusBarAfter)
	}
}

// TestBUG11_StatusBarPreservesModelAndCostTogether verifies that both model name
// and cost are preserved after a resize (combined scenario).
// Note: displayModelName() converts "gpt-4.1-mini" → "GPT-4.1 Mini", so we
// do a case-insensitive check for the model ID substring.
func TestBUG11_StatusBarPreservesModelAndCostTogether(t *testing.T) {
	m := initModel(t, 120, 40)
	m = m.WithCancelRun(func() {})

	// Set model. displayModelName converts "gpt-4.1-mini" → "GPT-4.1 Mini".
	m2, _ := m.Update(tui.ModelSelectedMsg{ModelID: "gpt-4.1-mini", Provider: "openai"})
	m = m2.(tui.Model)

	// Start run and set cost.
	m3, _ := m.Update(tui.RunStartedMsg{RunID: "run-bug11-both-1"})
	m = m3.(tui.Model)
	m4, _ := m.Update(tui.SSEEventMsg{
		EventType: "usage.delta",
		Raw:       []byte(`{"cumulative_usage":{"total_tokens":2000},"cumulative_cost_usd":0.0120}`),
	})
	m = m4.(tui.Model)

	// Resize.
	m5, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	m = m5.(tui.Model)

	// Both must still be present in the status bar after resize.
	// Use case-insensitive check for the model name.
	statusBarAfter := m.StatusBarView()
	if !strings.Contains(strings.ToLower(statusBarAfter), "gpt-4.1") {
		t.Errorf("status bar lost model name after resize; StatusBarView()=%q", statusBarAfter)
	}
	if !strings.Contains(statusBarAfter, "0.012") {
		t.Errorf("status bar lost cost after resize; StatusBarView()=%q", statusBarAfter)
	}
}
