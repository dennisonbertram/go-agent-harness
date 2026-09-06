package tui_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"go-agent-harness/cmd/harnesscli/tui"
)

// Issue #1405: settings overlays must read correctly to a first-time user.

// /cost must show prompt tokens as "in" and completion tokens as "out".
func TestCost_ShowsPromptAndCompletionTokens(t *testing.T) {
	m := initModel(t, 120, 40)
	raw := `{"turn_usage":{"prompt_tokens":15000,"completion_tokens":700,"total_tokens":15700},` +
		`"cumulative_usage":{"prompt_tokens":15000,"completion_tokens":700,"total_tokens":15700},"cumulative_cost_usd":0.0069}`
	m2, _ := m.Update(tui.SSEEventMsg{EventType: "usage.delta", Raw: json.RawMessage(raw), RunID: "run-1"})
	m = m2.(tui.Model)
	m = sendSlashCommand(m, "/cost")
	view := m.View()
	if !strings.Contains(view, "15,000 in") || !strings.Contains(view, "700 out") {
		t.Fatalf("/cost must show 15,000 in and 700 out, got:\n%s", view)
	}
	if strings.Contains(view, "↑ 0 in") {
		t.Fatalf("/cost must not report 0 input tokens after a run with prompt tokens:\n%s", view)
	}
}

// /profiles must never wrap its highlighted row.
func TestProfilePicker_SelectedRowFitsWidth(t *testing.T) {
	m := initModel(t, 120, 40)
	m = sendSlashCommand(m, "/profiles")
	m2, _ := m.Update(tui.ProfilesLoadedMsg{Entries: []tui.ProfileEntry{
		{Name: "bash-runner", Model: "gpt-4.1-mini", SourceTier: "built-in", Description: "Script execution, pipeline tasks"},
		{Name: "full", Model: "gpt-4.1-mini", SourceTier: "built-in", Description: "Default — all tools available"},
	}})
	m = m2.(tui.Model)
	for _, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > 120 {
			t.Fatalf("profiles row wider than the terminal (%d): %q", w, line)
		}
	}
	if !strings.Contains(m.View(), "built-in") || strings.Contains(m.View(), "built-\n") {
		t.Fatalf("highlighted profile row must not wrap mid-word:\n%s", m.View())
	}
}

// /config must not cut values silently and must explain [RO].
func TestConfigPanel_ValuesEllipsisAndROLegend(t *testing.T) {
	m := initModel(t, 120, 40)
	m2, _ := m.Update(tui.ModelSelectedMsg{ModelID: "deepseek/deepseek-v4-pro-with-a-long-suffix-x", Provider: "openrouter"})
	m = m2.(tui.Model)
	m = sendSlashCommand(m, "/config")
	view := m.View()
	if strings.Contains(view, "deepseek/deepseek-v4 ") && !strings.Contains(view, "…") {
		t.Fatalf("/config must not cut the model id silently:\n%s", view)
	}
	if !strings.Contains(view, "deepseek/deepseek-v4-pro") {
		t.Fatalf("/config should have room for the model id at 120 columns:\n%s", view)
	}
	if !strings.Contains(view, "read-only") {
		t.Fatalf("/config must explain the [RO] badge:\n%s", view)
	}
}

// /permissions must not draw a stray separator.
func TestPermissionsPanel_NoStraySeparator(t *testing.T) {
	m := initModel(t, 120, 40)
	m = sendSlashCommand(m, "/permissions")
	for _, line := range strings.Split(m.View(), "\n") {
		trimmed := strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "│"))
		if trimmed == "──" || trimmed == "─" {
			t.Fatalf("stray separator line in /permissions: %q\n%s", line, m.View())
		}
	}
}
