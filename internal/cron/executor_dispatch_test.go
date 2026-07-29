package cron

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type recordingRunStarter struct {
	prompt         string
	conversationID string
	runID          string
	err            error
}

func (s *recordingRunStarter) StartRun(prompt, conversationID string) (string, error) {
	s.prompt = prompt
	s.conversationID = conversationID
	return s.runID, s.err
}

func TestHarnessExecutorStartsConfiguredRun(t *testing.T) {
	starter := &recordingRunStarter{runID: "run-123"}
	executor := &HarnessExecutor{Starter: starter}

	got, err := executor.Execute(context.Background(), Job{
		Name:       "daily review",
		ExecConfig: `{"prompt":"review the queue","conversation_id":"conv-7"}`,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "started run run-123" {
		t.Fatalf("result = %q, want started run id", got)
	}
	if starter.prompt != "review the queue" || starter.conversationID != "conv-7" {
		t.Fatalf("starter received prompt=%q conversation=%q", starter.prompt, starter.conversationID)
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
