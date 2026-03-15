package tooluse

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestTUI040_TimerFormatMilliseconds verifies <1s formats as "Nms".
func TestTUI040_TimerFormatMilliseconds(t *testing.T) {
	t.Run("500ms", func(t *testing.T) {
		timer := NewTimer()
		timer = timer.start(time.Now().Add(-500 * time.Millisecond))
		timer = timer.stop(time.Now())
		got := timer.FormatDuration()
		if !strings.HasSuffix(got, "ms") {
			t.Errorf("expected suffix 'ms' for <1s duration, got: %q", got)
		}
	})
	t.Run("1ms", func(t *testing.T) {
		timer := NewTimer()
		timer = timer.start(time.Now().Add(-1 * time.Millisecond))
		timer = timer.stop(time.Now())
		got := timer.FormatDuration()
		if !strings.HasSuffix(got, "ms") {
			t.Errorf("expected suffix 'ms' for 1ms duration, got: %q", got)
		}
	})
	t.Run("999ms", func(t *testing.T) {
		timer := NewTimer()
		timer = timer.start(time.Now().Add(-999 * time.Millisecond))
		timer = timer.stop(time.Now())
		got := timer.FormatDuration()
		if !strings.HasSuffix(got, "ms") {
			t.Errorf("expected suffix 'ms' for 999ms duration, got: %q", got)
		}
	})
}

// TestTUI040_TimerFormatSeconds verifies 1–59s formats as "N.Ns".
func TestTUI040_TimerFormatSeconds(t *testing.T) {
	t.Run("2.3s", func(t *testing.T) {
		timer := NewTimer()
		timer = timer.start(time.Now().Add(-2300 * time.Millisecond))
		timer = timer.stop(time.Now())
		got := timer.FormatDuration()
		if !strings.HasSuffix(got, "s") {
			t.Errorf("expected suffix 's' for 2.3s duration, got: %q", got)
		}
		if strings.HasSuffix(got, "ms") {
			t.Errorf("must not be 'ms' for >=1s duration, got: %q", got)
		}
	})
	t.Run("59s", func(t *testing.T) {
		timer := NewTimer()
		timer = timer.start(time.Now().Add(-59 * time.Second))
		timer = timer.stop(time.Now())
		got := timer.FormatDuration()
		if !strings.HasSuffix(got, "s") {
			t.Errorf("expected suffix 's' for 59s duration, got: %q", got)
		}
		if strings.Contains(got, "m") {
			t.Errorf("must not contain 'm' for <60s duration, got: %q", got)
		}
	})
}

// TestTUI040_TimerFormatMinutes verifies >=60s formats as "Nm Ns".
func TestTUI040_TimerFormatMinutes(t *testing.T) {
	t.Run("2m30s", func(t *testing.T) {
		timer := NewTimer()
		timer = timer.start(time.Now().Add(-150 * time.Second))
		timer = timer.stop(time.Now())
		got := timer.FormatDuration()
		if !strings.Contains(got, "m") {
			t.Errorf("expected 'm' in output for >=60s, got: %q", got)
		}
		if !strings.Contains(got, "s") {
			t.Errorf("expected 's' in output for >=60s, got: %q", got)
		}
	})
	t.Run("60s_exact", func(t *testing.T) {
		timer := NewTimer()
		timer = timer.start(time.Now().Add(-60 * time.Second))
		timer = timer.stop(time.Now())
		got := timer.FormatDuration()
		if !strings.Contains(got, "m") {
			t.Errorf("expected 'm' in output for exactly 60s, got: %q", got)
		}
	})
}

// TestTUI040_TimerElapsedAfterStop verifies Elapsed() is reasonable after Stop().
func TestTUI040_TimerElapsedAfterStop(t *testing.T) {
	timer := NewTimer()
	start := time.Now()
	timer = timer.start(start)
	time.Sleep(5 * time.Millisecond)
	stop := time.Now()
	timer = timer.stop(stop)

	elapsed := timer.Elapsed()
	if elapsed <= 0 {
		t.Errorf("Elapsed() must be > 0 after Stop, got: %v", elapsed)
	}
	if elapsed > time.Second {
		t.Errorf("Elapsed() too large (expected <1s for a short sleep), got: %v", elapsed)
	}
}

// TestTUI040_TimerElapsedZeroBeforeStart verifies Elapsed()==0 before Start.
func TestTUI040_TimerElapsedZeroBeforeStart(t *testing.T) {
	timer := NewTimer()
	elapsed := timer.Elapsed()
	if elapsed != 0 {
		t.Errorf("Elapsed() must be 0 before Start(), got: %v", elapsed)
	}
}

// TestTUI040_TimerIsRunning verifies IsRunning() tracks the lifecycle correctly.
func TestTUI040_TimerIsRunning(t *testing.T) {
	timer := NewTimer()

	if timer.IsRunning() {
		t.Error("newly created Timer must not be running")
	}

	timer = timer.Start()
	if !timer.IsRunning() {
		t.Error("Timer must be running after Start()")
	}

	timer = timer.Stop()
	if timer.IsRunning() {
		t.Error("Timer must not be running after Stop()")
	}
}

// TestTUI040_CollapsedShowsDuration verifies completed CollapsedView with timer shows timing.
func TestTUI040_CollapsedShowsDuration(t *testing.T) {
	timer := NewTimer()
	timer = timer.start(time.Now().Add(-2300 * time.Millisecond))
	timer = timer.stop(time.Now())

	v := CollapsedView{
		ToolName: "ReadFile",
		Args:     "main.go",
		State:    StateCompleted,
		Width:    80,
		Timer:    timer,
	}
	out := v.View()
	visible := stripANSI(out)

	// The output must contain a timing indicator like "(2.3s)"
	if !strings.Contains(visible, "(") || !strings.Contains(visible, "s)") {
		t.Errorf("completed CollapsedView with timer must show duration, got: %q", visible)
	}
}

// TestTUI040_CollapsedRunningHidesTimerDuration verifies running state does not show duration.
func TestTUI040_CollapsedRunningHidesTimerDuration(t *testing.T) {
	timer := NewTimer()
	timer = timer.Start()

	v := CollapsedView{
		ToolName: "ReadFile",
		Args:     "main.go",
		State:    StateRunning,
		Width:    80,
		Timer:    timer,
	}
	out := v.View()
	visible := stripANSI(out)

	// Running state should not show "(Ns)" duration suffix
	if strings.Contains(visible, "(") && strings.Contains(visible, "s)") {
		t.Errorf("running CollapsedView must not show duration suffix, got: %q", visible)
	}
}

// TestTUI040_ExpandedUsesFallbackTimerDuration verifies ExpandedView uses Timer when Duration is empty.
func TestTUI040_ExpandedUsesFallbackTimerDuration(t *testing.T) {
	timer := NewTimer()
	timer = timer.start(time.Now().Add(-1500 * time.Millisecond))
	timer = timer.stop(time.Now())

	v := ExpandedView{
		ToolName: "BashExec",
		Args:     "go test ./...",
		State:    StateCompleted,
		Duration: "", // empty — should use Timer
		Width:    80,
		Timer:    timer,
	}
	out := v.View()
	visible := stripANSI(out)

	// Should contain the duration from the timer
	if !strings.Contains(visible, "s") {
		t.Errorf("ExpandedView must show timer duration when Duration is empty, got: %q", visible)
	}
}

// TestTUI040_ExpandedExplicitDurationTakesPrecedence verifies that explicit Duration takes precedence over Timer.
func TestTUI040_ExpandedExplicitDurationTakesPrecedence(t *testing.T) {
	timer := NewTimer()
	timer = timer.start(time.Now().Add(-2300 * time.Millisecond))
	timer = timer.stop(time.Now())

	v := ExpandedView{
		ToolName: "BashExec",
		Args:     "go test ./...",
		State:    StateCompleted,
		Duration: "explicit-dur",
		Width:    80,
		Timer:    timer,
	}
	out := v.View()
	visible := stripANSI(out)

	if !strings.Contains(visible, "explicit-dur") {
		t.Errorf("ExpandedView must use explicit Duration when set, got: %q", visible)
	}
}

// TestTUI040_ConcurrentTimers verifies 10 goroutines each with own Timer show no race.
func TestTUI040_ConcurrentTimers(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			timer := NewTimer()
			timer = timer.Start()
			time.Sleep(time.Duration(id) * time.Millisecond)
			timer = timer.Stop()
			_ = timer.FormatDuration()
			_ = timer.Elapsed()
			_ = timer.IsRunning()
		}(i)
	}
	wg.Wait()
}

// TestTUI040_VisualSnapshot_80x24 renders a tool call with timing at 80 width.
func TestTUI040_VisualSnapshot_80x24(t *testing.T) {
	timer := NewTimer()
	timer = timer.start(time.Now().Add(-2300 * time.Millisecond))
	timer = timer.stop(time.Now())

	views := []string{
		CollapsedView{
			ToolName: "ReadFile",
			Args:     "cmd/harnesscli/tui/model.go",
			State:    StateCompleted,
			Width:    80,
			Timer:    timer,
		}.View(),
		CollapsedView{
			ToolName: "BashExec",
			Args:     "go test ./...",
			State:    StateRunning,
			Width:    80,
			Timer:    NewTimer().Start(),
		}.View(),
		ExpandedView{
			ToolName: "GrepSearch",
			Args:     "pattern",
			State:    StateCompleted,
			Duration: "",
			Width:    80,
			Timer:    timer,
		}.View(),
	}

	output := strings.Join(views, "")

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-040-timing-80x24.txt"
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI040_VisualSnapshot_120x40 renders a tool call with timing at 120 width.
func TestTUI040_VisualSnapshot_120x40(t *testing.T) {
	timer := NewTimer()
	timer = timer.start(time.Now().Add(-2300 * time.Millisecond))
	timer = timer.stop(time.Now())

	views := []string{
		CollapsedView{
			ToolName: "ReadFile",
			Args:     "cmd/harnesscli/tui/model.go",
			State:    StateCompleted,
			Width:    120,
			Timer:    timer,
		}.View(),
		ExpandedView{
			ToolName: "GrepSearch",
			Args:     "pattern",
			State:    StateCompleted,
			Duration: "",
			Width:    120,
			Timer:    timer,
		}.View(),
	}

	output := strings.Join(views, "")

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-040-timing-120x40.txt"
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI040_VisualSnapshot_200x50 renders a tool call with timing at 200 width.
func TestTUI040_VisualSnapshot_200x50(t *testing.T) {
	timer := NewTimer()
	timer = timer.start(time.Now().Add(-2300 * time.Millisecond))
	timer = timer.stop(time.Now())

	views := []string{
		CollapsedView{
			ToolName: "ReadFile",
			Args:     "cmd/harnesscli/tui/model.go",
			State:    StateCompleted,
			Width:    200,
			Timer:    timer,
		}.View(),
		ExpandedView{
			ToolName: "GrepSearch",
			Args:     "pattern",
			State:    StateCompleted,
			Duration: "",
			Width:    200,
			Timer:    timer,
		}.View(),
	}

	output := strings.Join(views, "")

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-040-timing-200x50.txt"
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
