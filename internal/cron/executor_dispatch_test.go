package cron

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordingRunStarter struct {
	req   RunStartRequest
	runID string
	err   error
}

type deadlineRecordingRunStarter struct {
	deadline time.Time
	err      error
}

func (s *deadlineRecordingRunStarter) StartRun(RunStartRequest) (string, error) {
	return "", errors.New("context-aware start required")
}

func (s *deadlineRecordingRunStarter) StartRunContext(ctx context.Context, _ RunStartRequest) (string, error) {
	s.deadline, _ = ctx.Deadline()
	if s.err != nil {
		return "", s.err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "run-deadline", nil
}

type deadlineRecordingObserver struct{ hasDeadline bool }

func (o *deadlineRecordingObserver) ObserveRun(ctx context.Context, _ string) (RunObservation, error) {
	_, o.hasDeadline = ctx.Deadline()
	return RunObservation{Succeeded: true}, nil
}

func (s *recordingRunStarter) StartRun(req RunStartRequest) (string, error) {
	s.req = req
	return s.runID, s.err
}

func TestHarnessExecutorStartsConfiguredRun(t *testing.T) {
	starter := &recordingRunStarter{runID: "run-123"}
	executor := &HarnessExecutor{Starter: starter}

	got, err := executor.Execute(context.Background(), Job{
		ID:             "job-7",
		Name:           "daily review",
		ExecConfig:     `{"prompt":"review the queue","conversation_id":"legacy-conversation"}`,
		ConversationID: "conv-7",
		TenantID:       "tenant-7",
		AgentID:        "agent-7",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "started run run-123" {
		t.Fatalf("result = %q, want started run id", got)
	}
	if starter.req.Prompt != "review the queue" ||
		starter.req.ConversationID != "conv-7" ||
		starter.req.TenantID != "tenant-7" ||
		starter.req.AgentID != "agent-7" ||
		starter.req.JobID != "job-7" {
		t.Fatalf("starter received request %+v", starter.req)
	}
}

func TestHarnessExecutorExplainsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		executor *HarnessExecutor
		job      Job
		want     string
	}{
		{name: "missing starter", executor: &HarnessExecutor{}, job: Job{Name: "job"}, want: "not configured"},
		{name: "invalid json", executor: &HarnessExecutor{Starter: &recordingRunStarter{}}, job: Job{ExecConfig: "{"}, want: "parse execution config"},
		{name: "missing prompt", executor: &HarnessExecutor{Starter: &recordingRunStarter{}}, job: Job{Name: "job"}, want: "has no prompt"},
		{name: "start failure", executor: &HarnessExecutor{Starter: &recordingRunStarter{err: errors.New("offline")}}, job: Job{ExecConfig: `{"prompt":"run"}`}, want: "start run: offline"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.executor.Execute(context.Background(), tt.job)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Execute error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestHarnessExecutorAppliesEarliestJobOrParentDeadline(t *testing.T) {
	t.Run("job deadline is earlier", func(t *testing.T) {
		starter := &deadlineRecordingRunStarter{}
		executor := &HarnessExecutor{Starter: starter}
		parent, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		before := time.Now()
		if _, err := executor.Execute(parent, Job{
			ID:         "job-short",
			Name:       "short scheduling deadline",
			ExecConfig: `{"prompt":"start quickly"}`,
			TimeoutSec: 1,
		}); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		remaining := starter.deadline.Sub(before)
		if remaining <= 0 || remaining > 1100*time.Millisecond {
			t.Fatalf("job-derived deadline remaining = %v, want about 1s", remaining)
		}
	})

	t.Run("parent deadline is earlier", func(t *testing.T) {
		starter := &deadlineRecordingRunStarter{}
		executor := &HarnessExecutor{Starter: starter}
		parent, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		parentDeadline, _ := parent.Deadline()

		if _, err := executor.Execute(parent, Job{
			ID:         "job-long",
			Name:       "parent bounded start",
			ExecConfig: `{"prompt":"respect parent"}`,
			TimeoutSec: 60,
		}); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !starter.deadline.Equal(parentDeadline) {
			t.Fatalf("starter deadline = %v, want parent deadline %v", starter.deadline, parentDeadline)
		}
	})

	t.Run("parent cancellation remains typed", func(t *testing.T) {
		starter := &deadlineRecordingRunStarter{}
		executor := &HarnessExecutor{Starter: starter}
		parent, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := executor.Execute(parent, Job{
			ID:         "job-cancelled",
			Name:       "cancelled start",
			ExecConfig: `{"prompt":"do not start"}`,
			TimeoutSec: 60,
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Execute error = %v, want wrapped context.Canceled", err)
		}
	})
}

func TestHarnessExecutor_ObservationDoesNotTurnDispatchTimeoutIntoRunDeadline(t *testing.T) {
	observer := &deadlineRecordingObserver{}
	executor := &HarnessExecutor{Observer: observer}
	_, observed, err := executor.ObserveExecution(context.Background(), Job{TimeoutSec: 1}, ExecutionOutcome{RunID: "run-long-lived"})
	if err != nil || !observed {
		t.Fatalf("ObserveExecution = observed:%v err:%v", observed, err)
	}
	if observer.hasDeadline {
		t.Fatal("terminal observation inherited a dispatch timeout deadline")
	}
}

func TestDispatchExecutorRoutesByExecutionType(t *testing.T) {
	shell := &mockExecutor{ExecuteFunc: func(_ context.Context, _ Job) (string, error) {
		return "shell", nil
	}}
	harness := &mockExecutor{ExecuteFunc: func(_ context.Context, _ Job) (string, error) {
		return "harness", nil
	}}
	dispatch := &DispatchExecutor{Shell: shell, Harness: harness}

	if got, err := dispatch.Execute(context.Background(), Job{ExecType: ExecTypeShell}); err != nil || got != "shell" {
		t.Fatalf("shell route = %q, %v", got, err)
	}
	if got, err := dispatch.Execute(context.Background(), Job{ExecType: ExecTypeHarness}); err != nil || got != "harness" {
		t.Fatalf("harness route = %q, %v", got, err)
	}
	if _, err := dispatch.Execute(context.Background(), Job{ExecType: "future"}); err == nil || !strings.Contains(err.Error(), "unknown execution type") {
		t.Fatalf("unknown route error = %v", err)
	}
}

func TestDispatchExecutorNeverFallsBackFromHarnessToShell(t *testing.T) {
	shellCalls := 0
	dispatch := &DispatchExecutor{
		Shell: &mockExecutor{ExecuteFunc: func(_ context.Context, _ Job) (string, error) {
			shellCalls++
			return "shell", nil
		}},
	}
	_, err := dispatch.Execute(context.Background(), Job{ExecType: ExecTypeHarness, ExecConfig: `{"prompt":"must not execute in shell"}`})
	if err == nil || shellCalls != 0 {
		t.Fatalf("harness dispatch error=%v shellCalls=%d", err, shellCalls)
	}
}
