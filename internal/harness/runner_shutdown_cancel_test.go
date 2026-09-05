//go:build unix

package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// readPidFile polls for a pid file written by a spawned tool child and
// returns the parsed pid, failing the test if it does not appear in time.
// Mirrors internal/harness/tools/groupkill_unix_test.go's helper of the same
// name; duplicated here because that one is unexported in a different
// package and this test needs to observe the runner-level (not tool-level)
// cancellation path.
func readPidFile(t *testing.T, path string, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if convErr == nil && pid > 0 {
				return pid
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid file %q not readable within %v", path, timeout)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// processGone reports whether pid is dead. See groupkill_unix_test.go for the
// zombie-on-Linux rationale this mirrors.
func processGone(pid int) bool {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return true
	}
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return true // no /proc entry: process is gone
	}
	if idx := strings.LastIndexByte(string(data), ')'); idx >= 0 && idx+2 < len(data) {
		return data[idx+2] == 'Z'
	}
	return false
}

// assertProcessGone polls until the process is dead, proving the tool child
// (and not merely the run's own bookkeeping) was actually killed.
func assertProcessGone(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if processGone(pid) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("process %d still alive after %v", pid, timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestCancelAllActiveRuns_ShutdownKillsToolChildAndMarksCancelled reproduces
// issue #1373 at the runner level: a run blocked in a long-running bash tool
// call (standing in for `sleep 30`) must be cancelled — and its tool child
// killed — by the same bounded, runner-level primitive the daemon's shutdown
// path is required to call before the HTTP server drains and stores close.
// Before this fix, nothing in cmd/harnessd/main.go's shutdown sequence ever
// cancelled in-flight runs, so SIGTERM left the run "running" forever and
// orphaned the bash child.
func TestCancelAllActiveRuns_ShutdownKillsToolChildAndMarksCancelled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "child.pid")

	provider := &scriptedAskProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:   "call-bash-1",
					Name: "bash",
					Arguments: mustJSON(map[string]any{
						"command":         fmt.Sprintf("echo $$ > %s; sleep 30", pidFile),
						"timeout_seconds": 60,
					}),
				}},
			},
			{Content: "done"},
		},
	}

	registry := NewDefaultRegistryWithOptions(tmpDir, DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
	})
	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     5,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait for the bash tool child to actually be running before cancelling,
	// so this proves cancellation interrupts a live tool call rather than a
	// run that never got that far.
	childPID := readPidFile(t, pidFile, 5*time.Second)

	// This is the shutdown-time primitive main.go's signal handler must call
	// (issue #1373): cancel every still-active run via the same path as
	// CancelRun (already proven fast/reliable by TestCancelRun_ActiveRun),
	// and give it a bounded window to reach a terminal status before the
	// daemon process exits.
	waitCtx, cancelWait := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelWait()
	n := runner.CancelAllActiveRuns(waitCtx)
	if n != 1 {
		t.Fatalf("CancelAllActiveRuns reported %d active run(s), want 1", n)
	}

	deadline := time.Now().Add(3 * time.Second)
	var status RunStatus
	for time.Now().Before(deadline) {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatalf("run %s vanished", run.ID)
		}
		status = state.Status
		if status == RunStatusCancelled {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status != RunStatusCancelled {
		t.Fatalf("run status = %q after shutdown cancel-all, want %q", status, RunStatusCancelled)
	}

	// Prove the bash tool's child (and not merely the run bookkeeping) died:
	// an orphaned `sleep 30` surviving the daemon is exactly what issue #1373
	// reports.
	assertProcessGone(t, childPID, 3*time.Second)
}

// TestCancelAllActiveRuns_NoActiveRuns verifies the shutdown primitive is a
// safe no-op (returns 0, does not block or panic) when there is nothing to
// cancel — the common case on every clean shutdown.
func TestCancelAllActiveRuns_NoActiveRuns(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{MaxSteps: 2})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if n := runner.CancelAllActiveRuns(ctx); n != 0 {
		t.Fatalf("CancelAllActiveRuns on an idle runner reported %d, want 0", n)
	}
}
