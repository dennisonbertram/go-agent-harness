package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/deferred"
)

// mockCronClient implements tools.CronClient for testing.
type mockCronClient struct {
	createJobFn func(ctx context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error)
	listJobsFn  func(ctx context.Context) ([]tools.CronJob, error)
	getJobFn    func(ctx context.Context, id string) (tools.CronJob, error)
	updateJobFn func(ctx context.Context, id string, req tools.CronUpdateJobRequest) (tools.CronJob, error)
	deleteJobFn func(ctx context.Context, id string) error
	listExecsFn func(ctx context.Context, jobID string, limit, offset int) ([]tools.CronExecution, error)
	healthFn    func(ctx context.Context) error
}

func (m *mockCronClient) CreateJob(ctx context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error) {
	if m.createJobFn != nil {
		return m.createJobFn(ctx, req)
	}
	return tools.CronJob{}, nil
}

func (m *mockCronClient) ListJobs(ctx context.Context) ([]tools.CronJob, error) {
	if m.listJobsFn != nil {
		return m.listJobsFn(ctx)
	}
	return nil, nil
}

func (m *mockCronClient) GetJob(ctx context.Context, id string) (tools.CronJob, error) {
	if m.getJobFn != nil {
		return m.getJobFn(ctx, id)
	}
	return tools.CronJob{}, nil
}

func (m *mockCronClient) UpdateJob(ctx context.Context, id string, req tools.CronUpdateJobRequest) (tools.CronJob, error) {
	if m.updateJobFn != nil {
		return m.updateJobFn(ctx, id, req)
	}
	return tools.CronJob{}, nil
}

func (m *mockCronClient) DeleteJob(ctx context.Context, id string) error {
	if m.deleteJobFn != nil {
		return m.deleteJobFn(ctx, id)
	}
	return nil
}

func (m *mockCronClient) ListExecutions(ctx context.Context, jobID string, limit, offset int) ([]tools.CronExecution, error) {
	if m.listExecsFn != nil {
		return m.listExecsFn(ctx, jobID, limit, offset)
	}
	return nil, nil
}

func (m *mockCronClient) Health(ctx context.Context) error {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return nil
}

var testJob = tools.CronJob{
	ID:         "job-1",
	Name:       "test-job",
	Schedule:   "*/5 * * * *",
	ExecType:   "shell",
	ExecConfig: `{"command":"echo hello"}`,
	Status:     "active",
	TimeoutSec: 30,
	NextRunAt:  time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC),
	CreatedAt:  time.Date(2026, 3, 8, 11, 0, 0, 0, time.UTC),
	UpdatedAt:  time.Date(2026, 3, 8, 11, 0, 0, 0, time.UTC),
}

var testExecution = tools.CronExecution{
	ID:            "exec-1",
	JobID:         "job-1",
	StartedAt:     time.Date(2026, 3, 8, 11, 30, 0, 0, time.UTC),
	FinishedAt:    time.Date(2026, 3, 8, 11, 30, 1, 0, time.UTC),
	Status:        "success",
	OutputSummary: "hello",
	DurationMs:    1000,
}

func TestCronCreate(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		var gotReq tools.CronCreateJobRequest
		client := &mockCronClient{
			createJobFn: func(_ context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error) {
				gotReq = req
				return testJob, nil
			},
		}
		tool := deferred.CronCreateTool(client)

		if tool.Definition.Name != "cron_create" {
			t.Fatalf("expected name cron_create, got %s", tool.Definition.Name)
		}
		if tool.Definition.Action != tools.ActionExecute {
			t.Fatalf("expected action execute, got %s", tool.Definition.Action)
		}
		if !tool.Definition.Mutating {
			t.Fatal("expected mutating=true")
		}

		args := `{"name":"test-job","schedule":"*/5 * * * *","command":"echo hello"}`
		result, err := tool.Handler(context.Background(), json.RawMessage(args))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if gotReq.Name != "test-job" {
			t.Errorf("expected name test-job, got %s", gotReq.Name)
		}
		if gotReq.Schedule != "*/5 * * * *" {
			t.Errorf("expected schedule */5 * * * *, got %s", gotReq.Schedule)
		}
		if gotReq.ExecType != "shell" {
			t.Errorf("expected exec type shell, got %s", gotReq.ExecType)
		}
		if gotReq.TimeoutSec != 30 {
			t.Errorf("expected default timeout 30, got %d", gotReq.TimeoutSec)
		}
		if !strings.Contains(gotReq.ExecConfig, "echo hello") {
			t.Errorf("exec config should contain command, got %s", gotReq.ExecConfig)
		}
		if !strings.Contains(result, "test-job") {
			t.Errorf("result should contain job name, got %s", result)
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		var gotReq tools.CronCreateJobRequest
		client := &mockCronClient{
			createJobFn: func(_ context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error) {
				gotReq = req
				return testJob, nil
			},
		}
		tool := deferred.CronCreateTool(client)
		args := `{"name":"test","schedule":"* * * * *","command":"sleep 60","timeout_seconds":120}`
		_, err := tool.Handler(context.Background(), json.RawMessage(args))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.TimeoutSec != 120 {
			t.Errorf("expected timeout 120, got %d", gotReq.TimeoutSec)
		}
	})

	t.Run("client error", func(t *testing.T) {
		client := &mockCronClient{
			createJobFn: func(_ context.Context, _ tools.CronCreateJobRequest) (tools.CronJob, error) {
				return tools.CronJob{}, errors.New("connection refused")
			},
		}
		tool := deferred.CronCreateTool(client)
		args := `{"name":"test","schedule":"* * * * *","command":"echo hi"}`
		_, err := tool.Handler(context.Background(), json.RawMessage(args))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "connection refused") {
			t.Errorf("expected connection refused in error, got %v", err)
		}
	})

	t.Run("invalid json args", func(t *testing.T) {
		client := &mockCronClient{}
		tool := deferred.CronCreateTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{invalid`))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestCronCreate_UsesRunScopeAndIgnoresModelScopeOverrides(t *testing.T) {
	var gotReq tools.CronCreateJobRequest
	client := &mockCronClient{
		createJobFn: func(_ context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error) {
			gotReq = req
			return tools.CronJob{ID: "job-1", Name: req.Name}, nil
		},
	}
	tool := deferred.CronCreateTool(client)
	ctx := context.WithValue(context.Background(), tools.ContextKeyRunMetadata, tools.RunMetadata{
		TenantID:       "tenant-a",
		ConversationID: "conversation-a",
		AgentID:        "agent-a",
	})

	_, err := tool.Handler(ctx, json.RawMessage(`{"name":"scoped","schedule":"* * * * *","command":"echo hi","tenant_id":"spoof-tenant","conversation_id":"spoof-conversation","agent_id":"spoof-agent"}`))
	if err != nil {
		t.Fatalf("cron_create: %v", err)
	}
	if gotReq.TenantID != "tenant-a" || gotReq.ConversationID != "conversation-a" || gotReq.AgentID != "agent-a" {
		t.Fatalf("cron_create did not preserve run scope: %+v", gotReq)
	}
}

func TestCronList(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		client := &mockCronClient{
			listJobsFn: func(_ context.Context) ([]tools.CronJob, error) {
				return []tools.CronJob{testJob}, nil
			},
		}
		tool := deferred.CronListTool(client)

		if tool.Definition.Name != "cron_list" {
			t.Fatalf("expected name cron_list, got %s", tool.Definition.Name)
		}
		if tool.Definition.Action != tools.ActionList {
			t.Fatalf("expected action list, got %s", tool.Definition.Action)
		}
		if !tool.Definition.ParallelSafe {
			t.Fatal("expected parallel safe")
		}

		result, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "test-job") {
			t.Errorf("result should contain job name, got %s", result)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		client := &mockCronClient{
			listJobsFn: func(_ context.Context) ([]tools.CronJob, error) {
				return []tools.CronJob{}, nil
			},
		}
		tool := deferred.CronListTool(client)
		result, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "[]") {
			t.Errorf("expected empty array, got %s", result)
		}
	})

	t.Run("client error", func(t *testing.T) {
		client := &mockCronClient{
			listJobsFn: func(_ context.Context) ([]tools.CronJob, error) {
				return nil, errors.New("timeout")
			},
		}
		tool := deferred.CronListTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCronGet(t *testing.T) {
	t.Run("happy path with executions", func(t *testing.T) {
		client := &mockCronClient{
			getJobFn: func(_ context.Context, id string) (tools.CronJob, error) {
				if id != "job-1" {
					t.Errorf("expected id job-1, got %s", id)
				}
				return testJob, nil
			},
			listExecsFn: func(_ context.Context, jobID string, limit, offset int) ([]tools.CronExecution, error) {
				if jobID != "job-1" {
					t.Errorf("expected jobID job-1, got %s", jobID)
				}
				if limit != 5 {
					t.Errorf("expected limit 5, got %d", limit)
				}
				if offset != 0 {
					t.Errorf("expected offset 0, got %d", offset)
				}
				return []tools.CronExecution{testExecution}, nil
			},
		}
		tool := deferred.CronGetTool(client)

		if tool.Definition.Name != "cron_get" {
			t.Fatalf("expected name cron_get, got %s", tool.Definition.Name)
		}
		if tool.Definition.Action != tools.ActionRead {
			t.Fatalf("expected action read, got %s", tool.Definition.Action)
		}

		result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "test-job") {
			t.Errorf("result should contain job name")
		}
		if !strings.Contains(result, "exec-1") {
			t.Errorf("result should contain execution ID")
		}
		if !strings.Contains(result, "recent_executions") {
			t.Errorf("result should contain recent_executions key")
		}
	})

	t.Run("executions error degrades gracefully", func(t *testing.T) {
		client := &mockCronClient{
			getJobFn: func(_ context.Context, _ string) (tools.CronJob, error) {
				return testJob, nil
			},
			listExecsFn: func(_ context.Context, _ string, _, _ int) ([]tools.CronExecution, error) {
				return nil, errors.New("db error")
			},
		}
		tool := deferred.CronGetTool(client)
		result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
		if err != nil {
			t.Fatalf("should not error when only executions fail: %v", err)
		}
		if !strings.Contains(result, "test-job") {
			t.Errorf("result should still contain job")
		}
		// Should have empty executions array
		if !strings.Contains(result, "recent_executions") {
			t.Errorf("result should contain recent_executions key")
		}
	})

	t.Run("job not found", func(t *testing.T) {
		client := &mockCronClient{
			getJobFn: func(_ context.Context, _ string) (tools.CronJob, error) {
				return tools.CronJob{}, errors.New("not found")
			},
		}
		tool := deferred.CronGetTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"nonexistent"}`))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected not found error, got %v", err)
		}
	})

	t.Run("invalid args", func(t *testing.T) {
		client := &mockCronClient{}
		tool := deferred.CronGetTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{bad`))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestCronDelete(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		var gotID string
		client := &mockCronClient{
			deleteJobFn: func(_ context.Context, id string) error {
				gotID = id
				return nil
			},
		}
		tool := deferred.CronDeleteTool(client)

		if tool.Definition.Name != "cron_delete" {
			t.Fatalf("expected name cron_delete, got %s", tool.Definition.Name)
		}
		if !tool.Definition.Mutating {
			t.Fatal("expected mutating=true")
		}

		result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotID != "job-1" {
			t.Errorf("expected id job-1, got %s", gotID)
		}
		if !strings.Contains(result, "true") {
			t.Errorf("result should contain deleted:true")
		}
		if !strings.Contains(result, "job-1") {
			t.Errorf("result should contain job ID")
		}
	})

	t.Run("client error", func(t *testing.T) {
		client := &mockCronClient{
			deleteJobFn: func(_ context.Context, _ string) error {
				return errors.New("forbidden")
			},
		}
		tool := deferred.CronDeleteTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "forbidden") {
			t.Errorf("expected forbidden error, got %v", err)
		}
	})
}

func TestCronPause(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		var gotID string
		var gotReq tools.CronUpdateJobRequest
		pausedJob := testJob
		pausedJob.Status = "paused"

		client := &mockCronClient{
			updateJobFn: func(_ context.Context, id string, req tools.CronUpdateJobRequest) (tools.CronJob, error) {
				gotID = id
				gotReq = req
				return pausedJob, nil
			},
		}
		tool := deferred.CronPauseTool(client)

		if tool.Definition.Name != "cron_pause" {
			t.Fatalf("expected name cron_pause, got %s", tool.Definition.Name)
		}

		result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotID != "job-1" {
			t.Errorf("expected id job-1, got %s", gotID)
		}
		if gotReq.Status == nil || *gotReq.Status != "paused" {
			t.Error("expected status=paused in update request")
		}
		if !strings.Contains(result, "paused") {
			t.Errorf("result should contain paused status")
		}
	})

	t.Run("client error", func(t *testing.T) {
		client := &mockCronClient{
			updateJobFn: func(_ context.Context, _ string, _ tools.CronUpdateJobRequest) (tools.CronJob, error) {
				return tools.CronJob{}, errors.New("not found")
			},
		}
		tool := deferred.CronPauseTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCronResume(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		var gotID string
		var gotReq tools.CronUpdateJobRequest
		activeJob := testJob
		activeJob.Status = "active"

		client := &mockCronClient{
			updateJobFn: func(_ context.Context, id string, req tools.CronUpdateJobRequest) (tools.CronJob, error) {
				gotID = id
				gotReq = req
				return activeJob, nil
			},
		}
		tool := deferred.CronResumeTool(client)

		if tool.Definition.Name != "cron_resume" {
			t.Fatalf("expected name cron_resume, got %s", tool.Definition.Name)
		}

		result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotID != "job-1" {
			t.Errorf("expected id job-1, got %s", gotID)
		}
		if gotReq.Status == nil || *gotReq.Status != "active" {
			t.Error("expected status=active in update request")
		}
		if !strings.Contains(result, "active") {
			t.Errorf("result should contain active status")
		}
	})

	t.Run("client error", func(t *testing.T) {
		client := &mockCronClient{
			updateJobFn: func(_ context.Context, _ string, _ tools.CronUpdateJobRequest) (tools.CronJob, error) {
				return tools.CronJob{}, errors.New("server error")
			},
		}
		tool := deferred.CronResumeTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// --- Regression tests: invalid JSON args for delete/pause/resume ---

func TestCronDeleteInvalidArgs(t *testing.T) {
	client := &mockCronClient{}
	tool := deferred.CronDeleteTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCronPauseInvalidArgs(t *testing.T) {
	client := &mockCronClient{}
	tool := deferred.CronPauseTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCronResumeInvalidArgs(t *testing.T) {
	client := &mockCronClient{}
	tool := deferred.CronResumeTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- Regression tests: empty/invalid field values ---

func TestCronCreateEmptyFields(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		var gotReq tools.CronCreateJobRequest
		client := &mockCronClient{
			createJobFn: func(_ context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error) {
				gotReq = req
				return testJob, nil
			},
		}
		tool := deferred.CronCreateTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"","schedule":"* * * * *","command":"echo hi"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.Name != "" {
			t.Errorf("expected empty name to pass through, got %q", gotReq.Name)
		}
	})

	t.Run("empty schedule", func(t *testing.T) {
		var gotReq tools.CronCreateJobRequest
		client := &mockCronClient{
			createJobFn: func(_ context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error) {
				gotReq = req
				return testJob, nil
			},
		}
		tool := deferred.CronCreateTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"test","schedule":"","command":"echo hi"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.Schedule != "" {
			t.Errorf("expected empty schedule to pass through, got %q", gotReq.Schedule)
		}
	})

	t.Run("empty command", func(t *testing.T) {
		var gotReq tools.CronCreateJobRequest
		client := &mockCronClient{
			createJobFn: func(_ context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error) {
				gotReq = req
				return testJob, nil
			},
		}
		tool := deferred.CronCreateTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"test","schedule":"* * * * *","command":""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(gotReq.ExecConfig, `"command":""`) {
			t.Errorf("expected exec config to contain empty command, got %s", gotReq.ExecConfig)
		}
	})

	t.Run("negative timeout", func(t *testing.T) {
		var gotReq tools.CronCreateJobRequest
		client := &mockCronClient{
			createJobFn: func(_ context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error) {
				gotReq = req
				return testJob, nil
			},
		}
		tool := deferred.CronCreateTool(client)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"test","schedule":"* * * * *","command":"echo","timeout_seconds":-1}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.TimeoutSec != -1 {
			t.Errorf("expected negative timeout to pass through, got %d", gotReq.TimeoutSec)
		}
	})
}

// --- Regression tests: empty ID ---

func TestCronGetEmptyID(t *testing.T) {
	client := &mockCronClient{
		getJobFn: func(_ context.Context, id string) (tools.CronJob, error) {
			return tools.CronJob{}, errors.New("not found: empty id")
		},
	}
	tool := deferred.CronGetTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":""}`))
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "empty id") {
		t.Errorf("expected empty id error, got %v", err)
	}
}

func TestCronDeleteEmptyID(t *testing.T) {
	client := &mockCronClient{
		deleteJobFn: func(_ context.Context, id string) error {
			return errors.New("not found: empty id")
		},
	}
	tool := deferred.CronDeleteTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":""}`))
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "empty id") {
		t.Errorf("expected empty id error, got %v", err)
	}
}

func TestCronPauseEmptyID(t *testing.T) {
	client := &mockCronClient{
		updateJobFn: func(_ context.Context, id string, _ tools.CronUpdateJobRequest) (tools.CronJob, error) {
			return tools.CronJob{}, errors.New("not found: empty id")
		},
	}
	tool := deferred.CronPauseTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":""}`))
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "empty id") {
		t.Errorf("expected empty id error, got %v", err)
	}
}

func TestCronResumeEmptyID(t *testing.T) {
	client := &mockCronClient{
		updateJobFn: func(_ context.Context, id string, _ tools.CronUpdateJobRequest) (tools.CronJob, error) {
			return tools.CronJob{}, errors.New("not found: empty id")
		},
	}
	tool := deferred.CronResumeTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":""}`))
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "empty id") {
		t.Errorf("expected empty id error, got %v", err)
	}
}

// --- Regression test: concurrency ---

func TestCronToolsConcurrentAccess(t *testing.T) {
	client := &mockCronClient{
		createJobFn: func(_ context.Context, _ tools.CronCreateJobRequest) (tools.CronJob, error) {
			return testJob, nil
		},
		listJobsFn: func(_ context.Context) ([]tools.CronJob, error) {
			return []tools.CronJob{testJob}, nil
		},
		getJobFn: func(_ context.Context, _ string) (tools.CronJob, error) {
			return testJob, nil
		},
		updateJobFn: func(_ context.Context, _ string, _ tools.CronUpdateJobRequest) (tools.CronJob, error) {
			return testJob, nil
		},
		deleteJobFn: func(_ context.Context, _ string) error {
			return nil
		},
		listExecsFn: func(_ context.Context, _ string, _, _ int) ([]tools.CronExecution, error) {
			return []tools.CronExecution{testExecution}, nil
		},
	}

	cronTools := []tools.Tool{
		deferred.CronCreateTool(client),
		deferred.CronListTool(client),
		deferred.CronGetTool(client),
		deferred.CronDeleteTool(client),
		deferred.CronPauseTool(client),
		deferred.CronResumeTool(client),
	}

	argsPerTool := []string{
		`{"name":"test","schedule":"* * * * *","command":"echo hi"}`,
		`{}`,
		`{"id":"job-1"}`,
		`{"id":"job-1"}`,
		`{"id":"job-1"}`,
		`{"id":"job-1"}`,
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		for j, tool := range cronTools {
			wg.Add(1)
			go func(t tools.Tool, args string) {
				defer wg.Done()
				_, _ = t.Handler(context.Background(), json.RawMessage(args))
			}(tool, argsPerTool[j])
		}
	}
	wg.Wait()
}

// --- Regression test: context cancellation ---

func TestCronToolsContextCancellation(t *testing.T) {
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := &mockCronClient{
		createJobFn: func(ctx context.Context, _ tools.CronCreateJobRequest) (tools.CronJob, error) {
			return tools.CronJob{}, ctx.Err()
		},
		listJobsFn: func(ctx context.Context) ([]tools.CronJob, error) {
			return nil, ctx.Err()
		},
		getJobFn: func(ctx context.Context, _ string) (tools.CronJob, error) {
			return tools.CronJob{}, ctx.Err()
		},
		updateJobFn: func(ctx context.Context, _ string, _ tools.CronUpdateJobRequest) (tools.CronJob, error) {
			return tools.CronJob{}, ctx.Err()
		},
		deleteJobFn: func(ctx context.Context, _ string) error {
			return ctx.Err()
		},
		listExecsFn: func(ctx context.Context, _ string, _, _ int) ([]tools.CronExecution, error) {
			return nil, ctx.Err()
		},
	}

	toolsAndArgs := []struct {
		name string
		tool tools.Tool
		args string
	}{
		{"cron_create", deferred.CronCreateTool(client), `{"name":"t","schedule":"*","command":"x"}`},
		{"cron_list", deferred.CronListTool(client), `{}`},
		{"cron_get", deferred.CronGetTool(client), `{"id":"1"}`},
		{"cron_delete", deferred.CronDeleteTool(client), `{"id":"1"}`},
		{"cron_pause", deferred.CronPauseTool(client), `{"id":"1"}`},
		{"cron_resume", deferred.CronResumeTool(client), `{"id":"1"}`},
	}

	for _, tc := range toolsAndArgs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.tool.Handler(cancelledCtx, json.RawMessage(tc.args))
			if err == nil {
				t.Fatal("expected error for cancelled context")
			}
			if !strings.Contains(err.Error(), "canceled") {
				t.Errorf("expected canceled in error, got %v", err)
			}
		})
	}
}

// --- Regression test: nil slice from cron_list ---

func TestCronListNilSlice(t *testing.T) {
	client := &mockCronClient{
		listJobsFn: func(_ context.Context) ([]tools.CronJob, error) {
			return nil, nil
		},
	}
	tool := deferred.CronListTool(client)
	result, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// json.Marshal(nil slice) produces "null", verify no crash
	if result != "null" && result != "[]" {
		t.Errorf("expected null or [], got %s", result)
	}
}

// --- Regression test: empty executions is array not null ---

func TestCronGetEmptyExecutionsIsArray(t *testing.T) {
	client := &mockCronClient{
		getJobFn: func(_ context.Context, _ string) (tools.CronJob, error) {
			return testJob, nil
		},
		listExecsFn: func(_ context.Context, _ string, _, _ int) ([]tools.CronExecution, error) {
			return nil, errors.New("db error")
		},
	}
	tool := deferred.CronGetTool(client)
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse result JSON: %v", err)
	}

	execsRaw, ok := parsed["recent_executions"]
	if !ok {
		t.Fatal("missing recent_executions key in result")
	}

	// The code sets execs = []tools.CronExecution{} on error, so it should be "[]" not "null"
	if string(execsRaw) != "[]" {
		t.Errorf("expected recent_executions to be [], got %s", string(execsRaw))
	}
}

// --- Regression tests: constraint enforcement / idempotent operations ---

func TestCronPauseAlreadyPaused(t *testing.T) {
	client := &mockCronClient{
		updateJobFn: func(_ context.Context, _ string, _ tools.CronUpdateJobRequest) (tools.CronJob, error) {
			return tools.CronJob{}, errors.New("job already paused")
		},
	}
	tool := deferred.CronPauseTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "job already paused") {
		t.Errorf("expected 'job already paused' in error, got %v", err)
	}
}

func TestCronResumeAlreadyActive(t *testing.T) {
	client := &mockCronClient{
		updateJobFn: func(_ context.Context, _ string, _ tools.CronUpdateJobRequest) (tools.CronJob, error) {
			return tools.CronJob{}, errors.New("job already active")
		},
	}
	tool := deferred.CronResumeTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "job already active") {
		t.Errorf("expected 'job already active' in error, got %v", err)
	}
}

func TestCronCreateDuplicateName(t *testing.T) {
	client := &mockCronClient{
		createJobFn: func(_ context.Context, _ tools.CronCreateJobRequest) (tools.CronJob, error) {
			return tools.CronJob{}, errors.New("unique constraint violation")
		},
	}
	tool := deferred.CronCreateTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"dup","schedule":"* * * * *","command":"echo"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unique constraint violation") {
		t.Errorf("expected 'unique constraint violation' in error, got %v", err)
	}
}

func TestCronDeleteNonexistent(t *testing.T) {
	client := &mockCronClient{
		deleteJobFn: func(_ context.Context, _ string) error {
			return errors.New("not found")
		},
	}
	tool := deferred.CronDeleteTool(client)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"nonexistent"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %v", err)
	}
}

// --- Regression test: all tools.CronJob/tools.CronExecution fields exercised ---

func TestCronGetAllFieldsPresent(t *testing.T) {
	fullJob := tools.CronJob{
		ID:         "job-full",
		Name:       "full-job",
		Schedule:   "0 */2 * * *",
		ExecType:   "shell",
		ExecConfig: `{"command":"echo all fields"}`,
		Status:     "active",
		TimeoutSec: 60,
		Tags:       "tag1,tag2",
		NextRunAt:  time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC),
		LastRunAt:  time.Date(2026, 3, 8, 22, 0, 0, 0, time.UTC),
		CreatedAt:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 3, 8, 22, 0, 0, 0, time.UTC),
	}

	fullExec := tools.CronExecution{
		ID:            "exec-full",
		JobID:         "job-full",
		StartedAt:     time.Date(2026, 3, 8, 22, 0, 0, 0, time.UTC),
		FinishedAt:    time.Date(2026, 3, 8, 22, 0, 5, 0, time.UTC),
		Status:        "failed",
		RunID:         "run-abc-123",
		OutputSummary: "partial output here",
		Error:         "exit code 1",
		DurationMs:    5000,
	}

	client := &mockCronClient{
		getJobFn: func(_ context.Context, _ string) (tools.CronJob, error) {
			return fullJob, nil
		},
		listExecsFn: func(_ context.Context, _ string, _, _ int) ([]tools.CronExecution, error) {
			return []tools.CronExecution{fullExec}, nil
		},
	}

	tool := deferred.CronGetTool(client)
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-full"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all job fields present
	for _, expected := range []string{
		"job-full", "full-job", "0 */2 * * *", "shell",
		"echo all fields", "active", "tag1,tag2",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("result missing expected value %q", expected)
		}
	}

	// Verify all execution fields present
	for _, expected := range []string{
		"exec-full", "failed", "run-abc-123",
		"partial output here", "exit code 1",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("result missing expected execution value %q", expected)
		}
	}

	// Verify numeric fields via JSON parse
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	var job tools.CronJob
	if err := json.Unmarshal(parsed["job"], &job); err != nil {
		t.Fatalf("failed to parse job: %v", err)
	}
	if job.TimeoutSec != 60 {
		t.Errorf("expected timeout 60, got %d", job.TimeoutSec)
	}

	var execs []tools.CronExecution
	if err := json.Unmarshal(parsed["recent_executions"], &execs); err != nil {
		t.Fatalf("failed to parse executions: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].DurationMs != 5000 {
		t.Errorf("expected duration 5000, got %d", execs[0].DurationMs)
	}
}
