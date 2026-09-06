package tui_test

import (
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui"
)

// Issue #1407: plan mode must be reachable and visible. ctrl+o is overloaded
// (it expands tool calls whenever any tool has run), so a /plan command
// toggles it explicitly and the status bar shows the mode.
func TestPlanCommand_TogglesAndShowsInStatusBar(t *testing.T) {
	m := initModel(t, 120, 40)
	if m.PlanMode() {
		t.Fatal("plan mode must start off")
	}
	m = sendSlashCommand(m, "/plan")
	if !m.PlanMode() {
		t.Fatal("/plan must turn plan mode on")
	}
	if !strings.Contains(m.StatusBarModelLabel(), "PLAN") {
		t.Fatalf("status bar must show the PLAN badge while plan mode is on, got %q", m.StatusBarModelLabel())
	}
	if !strings.Contains(m.StatusMsg(), "Plan mode") {
		t.Fatalf("status must confirm the toggle, got %q", m.StatusMsg())
	}
	m = sendSlashCommand(m, "/plan")
	if m.PlanMode() || strings.Contains(m.StatusBarModelLabel(), "PLAN") {
		t.Fatal("/plan again must turn plan mode off and drop the badge")
	}
}

func TestPlanCommand_InSlashMenu(t *testing.T) {
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "/pla")
	if !strings.Contains(m.View(), "/plan ") && !strings.Contains(m.View(), "/plan\t") && !strings.Contains(m.View(), "/plan  ") {
		t.Fatalf("/plan must appear in the slash menu:\n%s", m.View())
	}
}

// harnesscli --tui --plan-mode must start the TUI in plan mode.
func TestTUIConfig_PlanModeFlag(t *testing.T) {
	cfg := tui.DefaultTUIConfig()
	cfg.PlanMode = true
	m := tui.New(cfg)
	if !m.PlanMode() {
		t.Fatal("TUIConfig.PlanMode must start the TUI in plan mode")
	}
}
