package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/cron"
	"go-agent-harness/internal/harness"
)

func TestObserveCronRunReplayTerminalWaitsForAuthoritativeStatus(t *testing.T) {
	t.Parallel()

	for _, terminal := range []harness.RunStatus{
		harness.RunStatusCompleted,
		harness.RunStatusFailed,
		harness.RunStatusCancelled,
	} {
		t.Run(string(terminal), func(t *testing.T) {
			calls := 0
			getRun := func(string) (harness.Run, bool) {
				calls++
				if calls < 3 {
					return harness.Run{Status: harness.RunStatusRunning}, true
				}
				return harness.Run{Status: terminal, Output: "authoritative output", Error: "authoritative error"}, true
			}
			silent := make(chan harness.Event)
			defer close(silent)
			cancelled := false
			poll := make(chan time.Time, 1)
			poll <- time.Now()
			run, err := observeCronRun(context.Background(), "run-replay", getRun, func(string) ([]harness.Event, <-chan harness.Event, func(), error) {
				return []harness.Event{{Type: harness.EventRunCompleted, Payload: map[string]any{"output": "untrusted"}}}, silent, func() { cancelled = true }, nil
			}, poll)
			if err != nil {
				t.Fatalf("observe replay terminal: %v", err)
			}
			if run.Status != terminal || run.Output != "authoritative output" || run.Error != "authoritative error" {
				t.Fatalf("run = %#v, want authoritative %s result", run, terminal)
			}
			if calls != 3 {
				t.Fatalf("GetRun calls = %d, want 3 (initial, post-subscribe, replay confirmation)", calls)
			}
			if !cancelled {
				t.Fatal("subscription cancel was not called")
			}
		})
	}
}

func TestObserveCronRunReplayTerminalHonorsCancellationWhenStatusNeverCommits(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	getRun := func(string) (harness.Run, bool) { return harness.Run{Status: harness.RunStatusRunning}, true }
	silent := make(chan harness.Event)
	defer close(silent)
	_, err := observeCronRun(ctx, "run-replay-cancel", getRun, func(string) ([]harness.Event, <-chan harness.Event, func(), error) {
		return []harness.Event{{Type: harness.EventRunCompleted}}, silent, func() {}, nil
	}, nil)
	if err != context.Canceled {
		t.Fatalf("observe error = %v, want context.Canceled", err)
	}
}

func TestObserveCronRunClosedStreamBeforeTerminalFails(t *testing.T) {
	t.Parallel()
	closed := make(chan harness.Event)
	close(closed)
	getRun := func(string) (harness.Run, bool) { return harness.Run{Status: harness.RunStatusRunning}, true }
	_, err := observeCronRun(context.Background(), "run-closed", getRun, func(string) ([]harness.Event, <-chan harness.Event, func(), error) {
		return nil, closed, func() {}, nil
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "stream closed before terminal status") {
		t.Fatalf("observe error = %v, want closed-stream terminal-status error", err)
	}
}

func TestObserveCronRunLiveTerminalUsesAuthoritativeStatus(t *testing.T) {
	t.Parallel()
	calls := 0
	getRun := func(string) (harness.Run, bool) {
		calls++
		if calls < 4 {
			return harness.Run{Status: harness.RunStatusRunning}, true
		}
		return harness.Run{Status: harness.RunStatusCompleted, Output: "authoritative live output"}, true
	}
	events := make(chan harness.Event, 1)
	events <- harness.Event{Type: harness.EventRunCompleted, Payload: map[string]any{"output": "untrusted live output"}}
	defer close(events)
	poll := make(chan time.Time, 1)
	poll <- time.Now()
	run, err := observeCronRun(context.Background(), "run-live", getRun, func(string) ([]harness.Event, <-chan harness.Event, func(), error) {
		return nil, events, func() {}, nil
	}, poll)
	if err != nil {
		t.Fatalf("observe live terminal: %v", err)
	}
	if run.Status != harness.RunStatusCompleted || run.Output != "authoritative live output" {
		t.Fatalf("run = %#v, want authoritative completed result", run)
	}
	if calls != 4 {
		t.Fatalf("GetRun calls = %d, want 4 (initial, post-subscribe, pre-event, event confirmation)", calls)
	}
}

func TestObserveCronRunPollsAuthoritativeStatusWhenTerminalEventIsSuppressed(t *testing.T) {
	t.Parallel()
	calls := 0
	getRun := func(string) (harness.Run, bool) {
		calls++
		if calls < 4 {
			return harness.Run{Status: harness.RunStatusRunning}, true
		}
		return harness.Run{Status: harness.RunStatusCompleted, Output: "status-only output"}, true
	}
	silent := make(chan harness.Event)
	defer close(silent)
	poll := make(chan time.Time, 1)
	poll <- time.Now()
	run, err := observeCronRun(context.Background(), "run-status-only", getRun, func(string) ([]harness.Event, <-chan harness.Event, func(), error) {
		return nil, silent, func() {}, nil
	}, poll)
	if err != nil {
		t.Fatalf("observe status-only terminal: %v", err)
	}
	if run.Status != harness.RunStatusCompleted || run.Output != "status-only output" {
		t.Fatalf("run = %#v, want authoritative status-only result", run)
	}
	if calls != 4 {
		t.Fatalf("GetRun calls = %d, want 4 (initial, post-subscribe, pre-wait, poll)", calls)
	}
}

func TestCronRunStarterRequiresBoundRunner(t *testing.T) {
	_, err := (&cronRunStarter{}).StartRun(cron.RunStartRequest{
		Prompt:         "prompt",
		ConversationID: "conversation",
	})
	if err == nil || !strings.Contains(err.Error(), "not yet initialized") {
		t.Fatalf("StartRun error = %v", err)
	}
}

func TestCronRunStarterStartsHarnessRun(t *testing.T) {
	runner := harness.NewRunner(&noopProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     1,
	})
	starter := &cronRunStarter{runner: runner}

	runID, err := starter.StartRun(cron.RunStartRequest{
		Prompt:         "scheduled work",
		ConversationID: "conv-cron",
		TenantID:       "tenant-cron",
		AgentID:        "agent-cron",
		JobID:          "job-cron",
		ExecutionID:    "execution-cron",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if runID == "" {
		t.Fatal("StartRun returned an empty run id")
	}
	run, ok := runner.GetRun(runID)
	if !ok {
		t.Fatalf("runner does not know started run %q", runID)
	}
	if run.ConversationID != "conv-cron" {
		t.Fatalf("conversation id = %q, want conv-cron", run.ConversationID)
	}
	if run.TenantID != "tenant-cron" {
		t.Fatalf("tenant id = %q, want tenant-cron", run.TenantID)
	}
	if run.AgentID != "agent-cron" {
		t.Fatalf("agent id = %q, want agent-cron", run.AgentID)
	}
}
