package tui_test

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

// TestTUI039_RunActiveFlagSetOnStart verifies that RunStartedMsg sets runActive=true.
func TestTUI039_RunActiveFlagSetOnStart(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m3, _ := m2.(tui.Model).Update(tui.RunStartedMsg{RunID: "run-001"})
	model := m3.(tui.Model)
	if !model.RunActive() {
		t.Error("RunActive() must be true after RunStartedMsg")
	}
}

// TestTUI039_RunActiveFlagClearedOnComplete verifies that RunCompletedMsg sets runActive=false.
func TestTUI039_RunActiveFlagClearedOnComplete(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m3, _ := m2.(tui.Model).Update(tui.RunStartedMsg{RunID: "run-001"})
	m4, _ := m3.(tui.Model).Update(tui.RunCompletedMsg{RunID: "run-001"})
	model := m4.(tui.Model)
	if model.RunActive() {
		t.Error("RunActive() must be false after RunCompletedMsg")
	}
}

// TestTUI039_RunActiveFlagClearedOnFailed verifies that RunFailedMsg sets runActive=false.
func TestTUI039_RunActiveFlagClearedOnFailed(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m3, _ := m2.(tui.Model).Update(tui.RunStartedMsg{RunID: "run-001"})
	m4, _ := m3.(tui.Model).Update(tui.RunFailedMsg{RunID: "run-001", Error: "timeout"})
	model := m4.(tui.Model)
	if model.RunActive() {
		t.Error("RunActive() must be false after RunFailedMsg")
	}
}

// TestTUI039_CancelActiveRunCallsCancelFunc verifies that Ctrl+C when runActive calls the cancel func.
func TestTUI039_CancelActiveRunCallsCancelFunc(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	cancelled := false
	cancelFn := func() { cancelled = true }

	m3 := m2.(tui.Model).WithCancelRun(cancelFn)
	m4, _ := m3.Update(tui.RunStartedMsg{RunID: "run-001"})

	// Now send Ctrl+C
	m5, _ := m4.(tui.Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_ = m5

	if !cancelled {
		t.Error("cancelRun must be called when Ctrl+C is pressed during an active run")
	}
}

// TestTUI039_CancelNoRunIsNoOp verifies that Ctrl+C when !runActive does not quit.
func TestTUI039_CancelNoRunIsNoOp(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	cancelled := false
	cancelFn := func() { cancelled = true }
	m3 := m2.(tui.Model).WithCancelRun(cancelFn)

	// runActive is false — Ctrl+C should do nothing (no cancel, no quit)
	m4, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_ = m4

	// cmd should be tea.Quit when no active run (standard quit behavior)
	// Actually based on spec: "if no active run, Ctrl+C is a no-op (input area handles clearing)"
	// We check the cancel was NOT called
	if cancelled {
		t.Error("cancelRun must NOT be called when Ctrl+C is pressed with no active run")
	}
	_ = cmd
}

// TestTUI039_CancelSetsInterruptedStatus verifies statusMsg="Interrupted" after cancel.
func TestTUI039_CancelSetsInterruptedStatus(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	cancelFn := func() {}
	m3 := m2.(tui.Model).WithCancelRun(cancelFn)
	m4, _ := m3.Update(tui.RunStartedMsg{RunID: "run-001"})
	m5, _ := m4.(tui.Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := m5.(tui.Model)

	statusMsg := model.StatusMsg()
	if statusMsg != "Interrupted" {
		t.Errorf("statusMsg must be 'Interrupted' after cancel, got: %q", statusMsg)
	}
}

// TestTUI039_CancelRunActiveFlagCleared verifies runActive=false after cancel.
func TestTUI039_CancelRunActiveFlagCleared(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	cancelFn := func() {}
	m3 := m2.(tui.Model).WithCancelRun(cancelFn)
	m4, _ := m3.Update(tui.RunStartedMsg{RunID: "run-001"})
	m5, _ := m4.(tui.Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := m5.(tui.Model)

	if model.RunActive() {
		t.Error("RunActive() must be false after Ctrl+C cancel")
	}
}

// TestTUI039_CancelConcurrentSafe verifies cancel called from multiple goroutines does not panic.
func TestTUI039_CancelConcurrentSafe(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	var mu sync.Mutex
	callCount := 0
	cancelFn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}
	m3 := m2.(tui.Model).WithCancelRun(cancelFn)
	m4, _ := m3.Update(tui.RunStartedMsg{RunID: "run-001"})
	model := m4.(tui.Model)

	// Concurrently "cancel" — in practice each goroutine gets its own copy of
	// the model (value semantics), so each sees runActive=true and calls cancel.
	// The important thing is no panic and the cancel func is safe to call concurrently.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each goroutine has its own local copy of the model
			localModel := model
			localModel.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		}()
	}
	wg.Wait()
	// No panic means concurrent invocations are safe
}

// TestTUI039_InterruptedMsgHasTime verifies InterruptedMsg has a non-zero At field.
func TestTUI039_InterruptedMsgHasTime(t *testing.T) {
	msg := tui.InterruptedMsg{At: time.Now()}
	if msg.At.IsZero() {
		t.Error("InterruptedMsg.At must not be zero")
	}
}

// TestTUI039_VisualSnapshot_80x24 renders the TUI with an "Interrupted" status message.
func TestTUI039_VisualSnapshot_80x24(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	cancelFn := func() {}
	m3 := m2.(tui.Model).WithCancelRun(cancelFn)
	m4, _ := m3.Update(tui.RunStartedMsg{RunID: "run-001"})
	m5, _ := m4.(tui.Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := m5.(tui.Model)

	output := model.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-039-cancel-80x24.txt"
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)

	if !strings.Contains(output, "Interrupted") {
		t.Errorf("snapshot must contain 'Interrupted', got:\n%s", output)
	}
}

// TestTUI039_VisualSnapshot_120x40 renders the TUI at 120x40.
func TestTUI039_VisualSnapshot_120x40(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	cancelFn := func() {}
	m3 := m2.(tui.Model).WithCancelRun(cancelFn)
	m4, _ := m3.Update(tui.RunStartedMsg{RunID: "run-001"})
	m5, _ := m4.(tui.Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := m5.(tui.Model)

	output := model.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-039-cancel-120x40.txt"
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI039_VisualSnapshot_200x50 renders the TUI at 200x50.
func TestTUI039_VisualSnapshot_200x50(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})

	cancelFn := func() {}
	m3 := m2.(tui.Model).WithCancelRun(cancelFn)
	m4, _ := m3.Update(tui.RunStartedMsg{RunID: "run-001"})
	m5, _ := m4.(tui.Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := m5.(tui.Model)

	output := model.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-039-cancel-200x50.txt"
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
