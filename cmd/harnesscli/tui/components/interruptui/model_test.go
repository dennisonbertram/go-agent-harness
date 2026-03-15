package interruptui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ─── State Transition Tests ───────────────────────────────────────────────────

// TestTUI054_NewIsHidden verifies that New() returns a model in the Hidden state.
func TestTUI054_NewIsHidden(t *testing.T) {
	m := New()
	if m.State != StateHidden {
		t.Errorf("New() state = %v, want StateHidden", m.State)
	}
	if m.IsVisible() {
		t.Error("New() IsVisible() = true, want false")
	}
}

// TestTUI054_ShowTransitionsHiddenToConfirm verifies Hidden → Confirm.
func TestTUI054_ShowTransitionsHiddenToConfirm(t *testing.T) {
	m := New()
	m2 := m.Show()

	if m2.State != StateConfirm {
		t.Errorf("Show() state = %v, want StateConfirm", m2.State)
	}
	if !m2.IsVisible() {
		t.Error("Show() IsVisible() = false, want true")
	}
	// Original unchanged (immutable).
	if m.State != StateHidden {
		t.Error("Show() must not modify receiver; original state changed")
	}
}

// TestTUI054_ShowNoopFromNonHidden verifies Show() is a no-op from non-Hidden states.
func TestTUI054_ShowNoopFromNonHidden(t *testing.T) {
	cases := []struct {
		name  string
		start State
	}{
		{"from Confirm", StateConfirm},
		{"from Waiting", StateWaiting},
		{"from Done", StateDone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{State: tc.start}
			m2 := m.Show()
			if m2.State != tc.start {
				t.Errorf("Show() from %v: state = %v, want unchanged %v", tc.start, m2.State, tc.start)
			}
		})
	}
}

// TestTUI054_ConfirmTransitionsConfirmToWaiting verifies Confirm → Waiting.
func TestTUI054_ConfirmTransitionsConfirmToWaiting(t *testing.T) {
	m := New().Show()
	if m.State != StateConfirm {
		t.Fatalf("precondition: expected StateConfirm, got %v", m.State)
	}

	m2 := m.Confirm()
	if m2.State != StateWaiting {
		t.Errorf("Confirm() state = %v, want StateWaiting", m2.State)
	}
	// Original unchanged.
	if m.State != StateConfirm {
		t.Error("Confirm() must not modify receiver")
	}
}

// TestTUI054_ConfirmNoopFromNonConfirm verifies Confirm() is a no-op from non-Confirm states.
func TestTUI054_ConfirmNoopFromNonConfirm(t *testing.T) {
	cases := []struct {
		name  string
		start State
	}{
		{"from Hidden", StateHidden},
		{"from Waiting", StateWaiting},
		{"from Done", StateDone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{State: tc.start}
			m2 := m.Confirm()
			if m2.State != tc.start {
				t.Errorf("Confirm() from %v: state = %v, want unchanged %v", tc.start, m2.State, tc.start)
			}
		})
	}
}

// TestTUI054_MarkDoneTransitionsWaitingToDone verifies Waiting → Done.
func TestTUI054_MarkDoneTransitionsWaitingToDone(t *testing.T) {
	m := New().Show().Confirm()
	if m.State != StateWaiting {
		t.Fatalf("precondition: expected StateWaiting, got %v", m.State)
	}

	m2 := m.MarkDone()
	if m2.State != StateDone {
		t.Errorf("MarkDone() state = %v, want StateDone", m2.State)
	}
	// Original unchanged.
	if m.State != StateWaiting {
		t.Error("MarkDone() must not modify receiver")
	}
}

// TestTUI054_MarkDoneNoopFromNonWaiting verifies MarkDone() is a no-op from non-Waiting states.
func TestTUI054_MarkDoneNoopFromNonWaiting(t *testing.T) {
	cases := []struct {
		name  string
		start State
	}{
		{"from Hidden", StateHidden},
		{"from Confirm", StateConfirm},
		{"from Done", StateDone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{State: tc.start}
			m2 := m.MarkDone()
			if m2.State != tc.start {
				t.Errorf("MarkDone() from %v: state = %v, want unchanged %v", tc.start, m2.State, tc.start)
			}
		})
	}
}

// TestTUI054_HideFromAnyState verifies Hide() transitions any state to Hidden.
func TestTUI054_HideFromAnyState(t *testing.T) {
	cases := []struct {
		name  string
		start State
	}{
		{"from Hidden", StateHidden},
		{"from Confirm", StateConfirm},
		{"from Waiting", StateWaiting},
		{"from Done", StateDone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{State: tc.start}
			m2 := m.Hide()
			if m2.State != StateHidden {
				t.Errorf("Hide() from %v: state = %v, want StateHidden", tc.start, m2.State)
			}
			if m2.IsVisible() {
				t.Errorf("Hide() from %v: IsVisible() = true, want false", tc.start)
			}
			// Original unchanged.
			if m.State != tc.start {
				t.Errorf("Hide() must not modify receiver (started at %v)", tc.start)
			}
		})
	}
}

// TestTUI054_FullCycle verifies the complete lifecycle: Hidden → Confirm → Waiting → Done → Hidden.
func TestTUI054_FullCycle(t *testing.T) {
	m := New()
	if m.CurrentState() != StateHidden {
		t.Error("initial state should be Hidden")
	}

	m = m.Show()
	if m.CurrentState() != StateConfirm {
		t.Errorf("after Show(): want StateConfirm, got %v", m.CurrentState())
	}

	m = m.Confirm()
	if m.CurrentState() != StateWaiting {
		t.Errorf("after Confirm(): want StateWaiting, got %v", m.CurrentState())
	}

	m = m.MarkDone()
	if m.CurrentState() != StateDone {
		t.Errorf("after MarkDone(): want StateDone, got %v", m.CurrentState())
	}

	m = m.Hide()
	if m.CurrentState() != StateHidden {
		t.Errorf("after Hide(): want StateHidden, got %v", m.CurrentState())
	}
}

// ─── IsVisible Tests ─────────────────────────────────────────────────────────

// TestTUI054_IsVisible verifies IsVisible() for all states.
func TestTUI054_IsVisible(t *testing.T) {
	cases := []struct {
		state   State
		visible bool
	}{
		{StateHidden, false},
		{StateConfirm, true},
		{StateWaiting, true},
		{StateDone, true},
	}
	for _, tc := range cases {
		m := Model{State: tc.state}
		got := m.IsVisible()
		if got != tc.visible {
			t.Errorf("state %v: IsVisible() = %v, want %v", tc.state, got, tc.visible)
		}
	}
}

// ─── Edge Case Tests ─────────────────────────────────────────────────────────

// TestTUI054_DoubleConfirm verifies double-calling Confirm() is idempotent from Waiting.
func TestTUI054_DoubleConfirm(t *testing.T) {
	m := New().Show().Confirm()
	// Second Confirm() from Waiting → no-op (Waiting).
	m2 := m.Confirm()
	if m2.State != StateWaiting {
		t.Errorf("double Confirm(): state = %v, want StateWaiting (no-op)", m2.State)
	}
}

// TestTUI054_DoubleShow verifies double-calling Show() stays at Confirm.
func TestTUI054_DoubleShow(t *testing.T) {
	m := New().Show()
	m2 := m.Show()
	if m2.State != StateConfirm {
		t.Errorf("double Show(): state = %v, want StateConfirm (no-op)", m2.State)
	}
}

// TestTUI054_ImmutableReceivers verifies every method returns a new copy.
func TestTUI054_ImmutableReceivers(t *testing.T) {
	original := New()

	// Show
	shown := original.Show()
	if &shown == &original {
		t.Error("Show() returned same pointer as receiver")
	}
	if original.State != StateHidden {
		t.Error("Show() mutated receiver")
	}

	// Confirm
	confirmed := shown.Confirm()
	if confirmed.State != StateWaiting {
		t.Error("Confirm() did not transition to Waiting")
	}
	if shown.State != StateConfirm {
		t.Error("Confirm() mutated receiver")
	}

	// MarkDone
	done := confirmed.MarkDone()
	if done.State != StateDone {
		t.Error("MarkDone() did not transition to Done")
	}
	if confirmed.State != StateWaiting {
		t.Error("MarkDone() mutated receiver")
	}

	// Hide
	hidden := done.Hide()
	if hidden.State != StateHidden {
		t.Error("Hide() did not transition to Hidden")
	}
	if done.State != StateDone {
		t.Error("Hide() mutated receiver")
	}
}

// ─── View Tests ──────────────────────────────────────────────────────────────

// TestTUI054_ViewHiddenIsEmpty verifies that StateHidden returns "".
func TestTUI054_ViewHiddenIsEmpty(t *testing.T) {
	m := New()
	m.Width = 80
	if got := m.View(); got != "" {
		t.Errorf("StateHidden View() = %q, want empty", got)
	}
}

// TestTUI054_ViewDoneIsEmpty verifies that StateDone returns "".
func TestTUI054_ViewDoneIsEmpty(t *testing.T) {
	m := Model{State: StateDone, Width: 80}
	if got := m.View(); got != "" {
		t.Errorf("StateDone View() = %q, want empty", got)
	}
}

// TestTUI054_ViewConfirmContainsWarning verifies the Confirm banner contains expected text.
func TestTUI054_ViewConfirmContainsWarning(t *testing.T) {
	m := Model{State: StateConfirm, Width: 80}
	got := m.View()
	plain := stripANSI(got)

	if !strings.Contains(plain, "⚠") {
		t.Errorf("Confirm View() missing ⚠ icon; got: %q", plain)
	}
	if !strings.Contains(plain, "Ctrl+C") {
		t.Errorf("Confirm View() missing 'Ctrl+C'; got: %q", plain)
	}
	if !strings.Contains(plain, "Esc") {
		t.Errorf("Confirm View() missing 'Esc'; got: %q", plain)
	}
}

// TestTUI054_ViewWaitingContainsStopping verifies the Waiting line contains expected text.
func TestTUI054_ViewWaitingContainsStopping(t *testing.T) {
	m := Model{State: StateWaiting, Width: 80}
	got := m.View()
	plain := stripANSI(got)

	if !strings.Contains(plain, "Stopping") {
		t.Errorf("Waiting View() missing 'Stopping'; got: %q", plain)
	}
	if !strings.Contains(plain, "waiting for current tool") {
		t.Errorf("Waiting View() missing 'waiting for current tool'; got: %q", plain)
	}
}

// TestTUI054_ViewDoesNotPanicAtZeroWidth verifies graceful handling of zero width.
func TestTUI054_ViewDoesNotPanicAtZeroWidth(t *testing.T) {
	for _, state := range []State{StateHidden, StateConfirm, StateWaiting, StateDone} {
		m := Model{State: state, Width: 0}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("View() panicked with width=0 at state %v: %v", state, r)
				}
			}()
			_ = m.View()
		}()
	}
}

// TestTUI054_ViewNarrowWidth verifies graceful rendering at very narrow width.
func TestTUI054_ViewNarrowWidth(t *testing.T) {
	for _, state := range []State{StateConfirm, StateWaiting} {
		m := Model{State: state, Width: 10}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("View() panicked with width=10 at state %v: %v", state, r)
				}
			}()
			_ = m.View()
		}()
	}
}

// ─── Concurrency Tests ────────────────────────────────────────────────────────

// TestTUI054_ConcurrentIndependentState verifies 10 goroutines each operating
// on their own Model instance do not race.
func TestTUI054_ConcurrentIndependentState(t *testing.T) {
	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			m := New()
			m = m.Show()
			m = m.Confirm()
			_ = m.View()
			m = m.MarkDone()
			m = m.Hide()
			_ = m.IsVisible()
			_ = m.CurrentState()
		}()
	}

	wg.Wait()
}

// ─── Snapshot Tests ───────────────────────────────────────────────────────────

const snapshotDir = "testdata/snapshots"

func writeSnapshot(t *testing.T, name, content string) {
	t.Helper()
	path := filepath.Join(snapshotDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create snapshot dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write snapshot %s: %v", path, err)
	}
}

func renderSnapshot(width, height int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# TUI-054 InterruptUI Snapshot %dx%d\n", width, height))
	sb.WriteString(strings.Repeat("-", width))
	sb.WriteString("\n")

	// StateHidden
	sb.WriteString("## StateHidden\n")
	m := Model{State: StateHidden, Width: width}
	view := m.View()
	if view == "" {
		sb.WriteString("(empty)\n")
	} else {
		sb.WriteString(view)
		sb.WriteString("\n")
	}

	// StateConfirm
	sb.WriteString("\n## StateConfirm\n")
	m = Model{State: StateConfirm, Width: width}
	sb.WriteString(m.View())
	sb.WriteString("\n")

	// StateWaiting
	sb.WriteString("\n## StateWaiting\n")
	m = Model{State: StateWaiting, Width: width}
	sb.WriteString(m.View())
	sb.WriteString("\n")

	// StateDone
	sb.WriteString("\n## StateDone\n")
	m = Model{State: StateDone, Width: width}
	view = m.View()
	if view == "" {
		sb.WriteString("(empty)\n")
	} else {
		sb.WriteString(view)
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat("-", width))
	sb.WriteString("\n")
	return sb.String()
}

func TestTUI054_VisualSnapshot_80x24(t *testing.T) {
	content := renderSnapshot(80, 24)
	writeSnapshot(t, "TUI-054-interruptui-80x24.txt", content)
	t.Logf("Snapshot written: %s/TUI-054-interruptui-80x24.txt", snapshotDir)
}

func TestTUI054_VisualSnapshot_120x40(t *testing.T) {
	content := renderSnapshot(120, 40)
	writeSnapshot(t, "TUI-054-interruptui-120x40.txt", content)
	t.Logf("Snapshot written: %s/TUI-054-interruptui-120x40.txt", snapshotDir)
}
