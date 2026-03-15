package tui_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

func TestTUI003_RootModelImplementsTeaModel(t *testing.T) {
	var _ tea.Model = tui.New(tui.DefaultTUIConfig())
}

func TestTUI003_InitReturnsCmdOrNil(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	cmd := m.Init()
	_ = cmd // nil is valid for Init
}

func TestTUI003_UpdateHandlesWindowSizeMsg(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m2 == nil {
		t.Fatal("Update returned nil model")
	}
}

func TestTUI003_UpdateHandlesUnknownMsgGracefully(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	type unknownMsg struct{}
	m2, _ := m.Update(unknownMsg{})
	if m2 == nil {
		t.Fatal("Update panicked or returned nil on unknown msg")
	}
}

func TestTUI003_ViewReturnsNonEmpty(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	// Set a valid size first
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model := m2.(tui.Model)
	v := model.View()
	if v == "" {
		t.Error("View() returned empty string")
	}
}

func TestTUI003_TeatestRenderAt80x24(t *testing.T) {
	tm := teatest.NewTestModel(t,
		tui.New(tui.DefaultTUIConfig()),
		teatest.WithInitialTermSize(80, 24),
	)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
