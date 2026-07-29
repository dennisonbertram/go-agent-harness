package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Reproduces the reported failure: a background job that prints after a delay
// reported no output, so the model concluded it had never fired.
func TestBackgroundJobOutputIsReadableAfterItPrints(t *testing.T) {
	m := NewJobManager(t.TempDir(), time.Now)
	defer func() { _ = m.Shutdown(context.Background()) }()

	started, err := m.runBackground(context.Background(), "sleep 1; echo hello dennison", 30, "")
	if err != nil {
		t.Fatalf("start background job: %v", err)
	}
	id, _ := started["shell_id"].(string)
	if id == "" {
		t.Fatal("background start returned no shell_id")
	}

	// Poll the way an agent would, rather than assuming a fixed delay.
	deadline := time.Now().Add(10 * time.Second)
	var out map[string]any
	for time.Now().Before(deadline) {
		out, err = m.output(id, false)
		if err != nil {
			t.Fatalf("read job output: %v", err)
		}
		if running, _ := out["running"].(bool); !running {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if running, _ := out["running"].(bool); running {
		t.Fatal("job never finished within 10s")
	}
	got, _ := out["output"].(string)
	if !strings.Contains(got, "hello dennison") {
		t.Errorf("finished job reported output %q, want it to contain %q — this is the\n"+
			"defect the model hit: the job ran, exited, and its output was unreachable",
			got, "hello dennison")
	}
}

// The trailing-ampersand form the model actually used. `a; b &` backgrounds
// only `b`, so the echo is a detached grandchild of the shell we capture.
func TestBackgroundJobOutputWithTrailingAmpersand(t *testing.T) {
	m := NewJobManager(t.TempDir(), time.Now)
	defer func() { _ = m.Shutdown(context.Background()) }()

	started, err := m.runBackground(context.Background(), "sleep 1; echo hello dennison &", 30, "")
	if err != nil {
		t.Fatalf("start background job: %v", err)
	}
	id, _ := started["shell_id"].(string)

	deadline := time.Now().Add(10 * time.Second)
	var out map[string]any
	for time.Now().Before(deadline) {
		out, _ = m.output(id, false)
		if running, _ := out["running"].(bool); !running {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	got, _ := out["output"].(string)
	t.Logf("trailing-ampersand form produced output %q (running=%v exit=%v)",
		got, out["running"], out["exit_code"])
}

// The defect that made background jobs look broken: the job inherited the
// caller's context, so when the run that started it ended, the job was killed
// mid-flight. A delayed echo never ran, and the job reported exit -1 with no
// output — indistinguishable, from the model's side, from a tool that does
// not work.
func TestBackgroundJobSurvivesItsCallersContext(t *testing.T) {
	m := NewJobManager(t.TempDir(), time.Now)
	defer func() { _ = m.Shutdown(context.Background()) }()

	callerCtx, cancelCaller := context.WithCancel(context.Background())
	started, err := m.runBackground(callerCtx, "sleep 1; echo survived", 30, "")
	if err != nil {
		t.Fatalf("start background job: %v", err)
	}
	id, _ := started["shell_id"].(string)

	// The run ends immediately, as it does in practice.
	cancelCaller()

	deadline := time.Now().Add(10 * time.Second)
	var out map[string]any
	for time.Now().Before(deadline) {
		out, _ = m.output(id, false)
		if running, _ := out["running"].(bool); !running {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	got, _ := out["output"].(string)
	exit, _ := out["exit_code"].(int)
	if !strings.Contains(got, "survived") {
		t.Errorf("output = %q (exit %d), want it to contain %q — the job was killed "+
			"when its caller's context was cancelled", got, exit, "survived")
	}
	if exit != 0 {
		t.Errorf("exit code = %d, want 0 — a non-zero code here means the job was "+
			"terminated rather than allowed to finish", exit)
	}
}
