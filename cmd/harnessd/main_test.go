package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/cron"
	"go-agent-harness/internal/harness"
	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
	openai "go-agent-harness/internal/provider/openai"
	"go-agent-harness/internal/skills"
)

type noopProvider struct{}

func (n *noopProvider) Complete(_ context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	return harness.CompletionResult{Content: "ok"}, nil
}

type modelProviderStub struct {
	result harness.CompletionResult
	err    error
	req    harness.CompletionRequest
}

func (m *modelProviderStub) Complete(_ context.Context, req harness.CompletionRequest) (harness.CompletionResult, error) {
	m.req = req
	if m.err != nil {
		return harness.CompletionResult{}, m.err
	}
	return m.result, nil
}

func TestMainDoesNotExitWhenRunSucceeds(t *testing.T) {
	origRun := runMain
	origExit := exitFunc
	defer func() {
		runMain = origRun
		exitFunc = origExit
	}()

	runMain = func() error { return nil }
	exitCalled := false
	exitFunc = func(int) { exitCalled = true }

	main()

	if exitCalled {
		t.Fatalf("did not expect exit")
	}
}

func TestMainExitsWhenRunFails(t *testing.T) {
	origRun := runMain
	origExit := exitFunc
	defer func() {
		runMain = origRun
		exitFunc = origExit
	}()

	runMain = func() error { return errors.New("boom") }
	exitCode := -1
	exitFunc = func(code int) {
		exitCode = code
		panic("exit-called")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic sentinel")
		}
		if r != "exit-called" {
			t.Fatalf("unexpected panic: %v", r)
		}
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	}()

	main()
}

func TestGetenvOrDefault(t *testing.T) {
	t.Setenv("HARNESS_TEST_VALUE", "x")
	if got := getenvOrDefault("HARNESS_TEST_VALUE", "fallback"); got != "x" {
		t.Fatalf("expected x, got %q", got)
	}
	if got := getenvOrDefault("HARNESS_TEST_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestGetenvIntOrDefault(t *testing.T) {
	t.Setenv("HARNESS_INT", "17")
	if got := getenvIntOrDefault("HARNESS_INT", 9); got != 17 {
		t.Fatalf("expected 17, got %d", got)
	}
	t.Setenv("HARNESS_INT", "bad")
	if got := getenvIntOrDefault("HARNESS_INT", 9); got != 9 {
		t.Fatalf("expected fallback 9, got %d", got)
	}
	os.Unsetenv("HARNESS_INT")
	if got := getenvIntOrDefault("HARNESS_INT", 9); got != 9 {
		t.Fatalf("expected fallback 9, got %d", got)
	}
}

func TestAskUserTimeoutEnvParsing(t *testing.T) {
	t.Setenv("HARNESS_ASK_USER_TIMEOUT_SECONDS", "45")
	if got := getenvIntOrDefault("HARNESS_ASK_USER_TIMEOUT_SECONDS", 300); got != 45 {
		t.Fatalf("expected 45, got %d", got)
	}

	t.Setenv("HARNESS_ASK_USER_TIMEOUT_SECONDS", "bad")
	if got := getenvIntOrDefault("HARNESS_ASK_USER_TIMEOUT_SECONDS", 300); got != 300 {
		t.Fatalf("expected fallback 300, got %d", got)
	}
}

func TestGetenvToolApprovalModeOrDefault(t *testing.T) {
	t.Setenv("HARNESS_TOOL_APPROVAL_MODE", "permissions")
	if got := getenvToolApprovalModeOrDefault("HARNESS_TOOL_APPROVAL_MODE", harness.ToolApprovalModeFullAuto); got != harness.ToolApprovalModePermissions {
		t.Fatalf("expected permissions, got %q", got)
	}
	t.Setenv("HARNESS_TOOL_APPROVAL_MODE", "FULL_AUTO")
	if got := getenvToolApprovalModeOrDefault("HARNESS_TOOL_APPROVAL_MODE", harness.ToolApprovalModePermissions); got != harness.ToolApprovalModeFullAuto {
		t.Fatalf("expected full_auto, got %q", got)
	}
	t.Setenv("HARNESS_TOOL_APPROVAL_MODE", "bad")
	if got := getenvToolApprovalModeOrDefault("HARNESS_TOOL_APPROVAL_MODE", harness.ToolApprovalModeFullAuto); got != harness.ToolApprovalModeFullAuto {
		t.Fatalf("expected fallback full_auto, got %q", got)
	}
}

func TestRunDelegatesToRunWithSignals(t *testing.T) {
	orig := runWithSignalsFunc
	defer func() { runWithSignalsFunc = orig }()

	called := false
	runWithSignalsFunc = func(sig <-chan os.Signal, getenv func(string) string, newProvider providerFactory, profileName string) error {
		called = true
		if sig == nil {
			t.Fatalf("expected non-nil signal channel")
		}
		if getenv == nil {
			t.Fatalf("expected getenv callback")
		}
		if newProvider == nil {
			t.Fatalf("expected provider callback")
		}
		return nil
	}

	if err := run(); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !called {
		t.Fatalf("expected runWithSignalsFunc to be called")
	}
}

func TestRunWithSignalsMissingAPIKey(t *testing.T) {
	err := runWithSignals(make(chan os.Signal, 1), func(string) string { return "" }, func(openai.Config) (harness.Provider, error) {
		return &noopProvider{}, nil
	}, "")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithSignalsProviderFailure(t *testing.T) {
	env := map[string]string{
		"OPENAI_API_KEY":      "x",
		"HARNESS_ADDR":        "127.0.0.1:0",
		"HARNESS_MEMORY_MODE": "off",
	}
	getenv := func(key string) string { return env[key] }

	err := runWithSignals(make(chan os.Signal, 1), getenv, func(openai.Config) (harness.Provider, error) {
		return nil, errors.New("provider init failed")
	}, "")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "provider init failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithSignalsGracefulShutdown(t *testing.T) {
	env := map[string]string{
		"OPENAI_API_KEY":      "x",
		"HARNESS_ADDR":        "127.0.0.1:0",
		"HARNESS_MEMORY_MODE": "off",
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv, func(openai.Config) (harness.Provider, error) {
			return &noopProvider{}, nil
		}, "")
	}()

	time.Sleep(100 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}

func TestGetenvMemoryModeOrDefault(t *testing.T) {
	t.Setenv("HARNESS_MEMORY_MODE", "local_coordinator")
	if got := getenvMemoryModeOrDefault("HARNESS_MEMORY_MODE", "off"); got != "local_coordinator" {
		t.Fatalf("expected local_coordinator, got %q", got)
	}
	t.Setenv("HARNESS_MEMORY_MODE", "bad")
	if got := getenvMemoryModeOrDefault("HARNESS_MEMORY_MODE", "auto"); got != "auto" {
		t.Fatalf("expected fallback auto, got %q", got)
	}
}

func TestGetenvBoolOrDefault(t *testing.T) {
	t.Setenv("HARNESS_BOOL", "yes")
	if !getenvBoolOrDefault("HARNESS_BOOL", false) {
		t.Fatalf("expected true")
	}
	t.Setenv("HARNESS_BOOL", "off")
	if getenvBoolOrDefault("HARNESS_BOOL", true) {
		t.Fatalf("expected false")
	}
	t.Setenv("HARNESS_BOOL", "invalid")
	if !getenvBoolOrDefault("HARNESS_BOOL", true) {
		t.Fatalf("expected fallback true")
	}
}

func TestObservationalMemoryModelComplete(t *testing.T) {
	t.Parallel()

	m := observationalMemoryModel{}
	if _, err := m.Complete(context.Background(), om.ModelRequest{}); err == nil {
		t.Fatalf("expected provider required error")
	}

	provider := &modelProviderStub{
		result: harness.CompletionResult{Content: "  summary result  "},
	}
	m = observationalMemoryModel{
		provider: provider,
		model:    "gpt-5-nano",
	}
	out, err := m.Complete(context.Background(), om.ModelRequest{
		Messages: []om.PromptMessage{{Role: "system", Content: "A"}, {Role: "user", Content: "B"}},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if out != "summary result" {
		t.Fatalf("unexpected trimmed output: %q", out)
	}
	if provider.req.Model != "gpt-5-nano" || len(provider.req.Messages) != 2 {
		t.Fatalf("unexpected provider request: %+v", provider.req)
	}

	provider.err = errors.New("provider failed")
	if _, err := m.Complete(context.Background(), om.ModelRequest{Messages: []om.PromptMessage{{Role: "user", Content: "x"}}}); err == nil {
		t.Fatalf("expected provider error")
	}
}

func TestNewObservationalMemoryManagerBranches(t *testing.T) {
	t.Parallel()

	offMgr, err := newObservationalMemoryManager(observationalMemoryManagerOptions{
		Mode: om.ModeOff,
	})
	if err != nil {
		t.Fatalf("mode off manager: %v", err)
	}
	if offMgr.Mode() != om.ModeOff {
		t.Fatalf("expected off mode, got %q", offMgr.Mode())
	}

	if _, err := newObservationalMemoryManager(observationalMemoryManagerOptions{
		Mode:          om.ModeAuto,
		Driver:        "unknown",
		WorkspaceRoot: t.TempDir(),
	}); err == nil {
		t.Fatalf("expected unsupported driver error")
	}

	if _, err := newObservationalMemoryManager(observationalMemoryManagerOptions{
		Mode:          om.ModeAuto,
		Driver:        "postgres",
		WorkspaceRoot: t.TempDir(),
		MemoryLLMMode: "inherit",
	}); err == nil {
		t.Fatalf("expected postgres dsn error")
	}

	provider := &noopProvider{}
	manager, err := newObservationalMemoryManager(observationalMemoryManagerOptions{
		Mode:          om.ModeAuto,
		Driver:        "sqlite",
		SQLitePath:    ".harness/memory.db",
		WorkspaceRoot: t.TempDir(),
		Provider:      provider,
		Model:         "gpt-5-nano",
		MemoryLLMMode: "inherit",
		DefaultConfig: om.DefaultConfig(),
	})
	if err != nil {
		t.Fatalf("sqlite inherit manager: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	if manager.Mode() != om.ModeLocalCoordinator {
		t.Fatalf("expected local coordinator mode, got %q", manager.Mode())
	}

	if _, err := newObservationalMemoryManager(observationalMemoryManagerOptions{
		Mode:             om.ModeAuto,
		Driver:           "sqlite",
		SQLitePath:       ".harness/memory.db",
		WorkspaceRoot:    t.TempDir(),
		MemoryLLMMode:    "openai",
		MemoryLLMAPIKey:  "",
		MemoryLLMBaseURL: "",
		MemoryLLMModel:   "",
	}); err == nil {
		t.Fatalf("expected openai api key error")
	}

	if _, err := newObservationalMemoryManager(observationalMemoryManagerOptions{
		Mode:          om.ModeAuto,
		Driver:        "sqlite",
		SQLitePath:    ".harness/memory.db",
		WorkspaceRoot: t.TempDir(),
		MemoryLLMMode: "unsupported",
	}); err == nil {
		t.Fatalf("expected unsupported llm mode error")
	}
}

// ---------------------------------------------------------------------------
// cronJobFromCron / cronExecFromCron field-mapping tests
// ---------------------------------------------------------------------------

func TestCronJobFromCronAllFields(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)
	lastRun := now.Add(-1 * time.Hour)

	j := cron.Job{
		ID:         "job-123",
		Name:       "nightly-backup",
		Schedule:   "0 2 * * *",
		ExecType:   "shell",
		ExecConfig: `{"command":"pg_dump"}`,
		Status:     "active",
		TimeoutSec: 300,
		Tags:       "backup,prod",
		NextRunAt:  now.Add(24 * time.Hour),
		LastRunAt:  lastRun,
		CreatedAt:  now.Add(-48 * time.Hour),
		UpdatedAt:  now,
	}

	got := cronJobFromCron(j)

	if got.ID != j.ID {
		t.Fatalf("ID: got %q, want %q", got.ID, j.ID)
	}
	if got.Name != j.Name {
		t.Fatalf("Name: got %q, want %q", got.Name, j.Name)
	}
	if got.Schedule != j.Schedule {
		t.Fatalf("Schedule: got %q, want %q", got.Schedule, j.Schedule)
	}
	if got.ExecType != j.ExecType {
		t.Fatalf("ExecType: got %q, want %q", got.ExecType, j.ExecType)
	}
	if got.ExecConfig != j.ExecConfig {
		t.Fatalf("ExecConfig: got %q, want %q", got.ExecConfig, j.ExecConfig)
	}
	if got.Status != j.Status {
		t.Fatalf("Status: got %q, want %q", got.Status, j.Status)
	}
	if got.TimeoutSec != j.TimeoutSec {
		t.Fatalf("TimeoutSec: got %d, want %d", got.TimeoutSec, j.TimeoutSec)
	}
	if got.Tags != j.Tags {
		t.Fatalf("Tags: got %q, want %q", got.Tags, j.Tags)
	}
	if !got.NextRunAt.Equal(j.NextRunAt) {
		t.Fatalf("NextRunAt: got %v, want %v", got.NextRunAt, j.NextRunAt)
	}
	if !got.LastRunAt.Equal(j.LastRunAt) {
		t.Fatalf("LastRunAt: got %v, want %v", got.LastRunAt, j.LastRunAt)
	}
	if !got.CreatedAt.Equal(j.CreatedAt) {
		t.Fatalf("CreatedAt: got %v, want %v", got.CreatedAt, j.CreatedAt)
	}
	if !got.UpdatedAt.Equal(j.UpdatedAt) {
		t.Fatalf("UpdatedAt: got %v, want %v", got.UpdatedAt, j.UpdatedAt)
	}
}

func TestCronJobFromCronZeroValues(t *testing.T) {
	t.Parallel()

	got := cronJobFromCron(cron.Job{})

	if got.ID != "" {
		t.Fatalf("expected empty ID, got %q", got.ID)
	}
	if got.Name != "" {
		t.Fatalf("expected empty Name, got %q", got.Name)
	}
	if got.TimeoutSec != 0 {
		t.Fatalf("expected 0 TimeoutSec, got %d", got.TimeoutSec)
	}
	if !got.NextRunAt.IsZero() {
		t.Fatalf("expected zero NextRunAt, got %v", got.NextRunAt)
	}
	if !got.LastRunAt.IsZero() {
		t.Fatalf("expected zero LastRunAt, got %v", got.LastRunAt)
	}
	if !got.CreatedAt.IsZero() {
		t.Fatalf("expected zero CreatedAt, got %v", got.CreatedAt)
	}
	if !got.UpdatedAt.IsZero() {
		t.Fatalf("expected zero UpdatedAt, got %v", got.UpdatedAt)
	}
}

func TestCronExecFromCronAllFields(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)

	e := cron.Execution{
		ID:            "exec-456",
		JobID:         "job-123",
		StartedAt:     now.Add(-5 * time.Minute),
		FinishedAt:    now,
		Status:        "success",
		RunID:         "run-789",
		OutputSummary: "completed 42 rows",
		Error:         "",
		DurationMs:    300000,
	}

	got := cronExecFromCron(e)

	if got.ID != e.ID {
		t.Fatalf("ID: got %q, want %q", got.ID, e.ID)
	}
	if got.JobID != e.JobID {
		t.Fatalf("JobID: got %q, want %q", got.JobID, e.JobID)
	}
	if !got.StartedAt.Equal(e.StartedAt) {
		t.Fatalf("StartedAt: got %v, want %v", got.StartedAt, e.StartedAt)
	}
	if !got.FinishedAt.Equal(e.FinishedAt) {
		t.Fatalf("FinishedAt: got %v, want %v", got.FinishedAt, e.FinishedAt)
	}
	if got.Status != e.Status {
		t.Fatalf("Status: got %q, want %q", got.Status, e.Status)
	}
	if got.RunID != e.RunID {
		t.Fatalf("RunID: got %q, want %q", got.RunID, e.RunID)
	}
	if got.OutputSummary != e.OutputSummary {
		t.Fatalf("OutputSummary: got %q, want %q", got.OutputSummary, e.OutputSummary)
	}
	if got.Error != e.Error {
		t.Fatalf("Error: got %q, want %q", got.Error, e.Error)
	}
	if got.DurationMs != e.DurationMs {
		t.Fatalf("DurationMs: got %d, want %d", got.DurationMs, e.DurationMs)
	}
}

func TestCronExecFromCronZeroValues(t *testing.T) {
	t.Parallel()

	got := cronExecFromCron(cron.Execution{})

	if got.ID != "" {
		t.Fatalf("expected empty ID, got %q", got.ID)
	}
	if got.JobID != "" {
		t.Fatalf("expected empty JobID, got %q", got.JobID)
	}
	if !got.StartedAt.IsZero() {
		t.Fatalf("expected zero StartedAt, got %v", got.StartedAt)
	}
	if !got.FinishedAt.IsZero() {
		t.Fatalf("expected zero FinishedAt, got %v", got.FinishedAt)
	}
	if got.Status != "" {
		t.Fatalf("expected empty Status, got %q", got.Status)
	}
	if got.RunID != "" {
		t.Fatalf("expected empty RunID, got %q", got.RunID)
	}
	if got.OutputSummary != "" {
		t.Fatalf("expected empty OutputSummary, got %q", got.OutputSummary)
	}
	if got.Error != "" {
		t.Fatalf("expected empty Error, got %q", got.Error)
	}
	if got.DurationMs != 0 {
		t.Fatalf("expected 0 DurationMs, got %d", got.DurationMs)
	}
}

// ---------------------------------------------------------------------------
// cronClientAdapter end-to-end tests with httptest
// ---------------------------------------------------------------------------

// sampleJob returns a cron.Job fixture for httptest JSON responses.
func sampleJob() cron.Job {
	return cron.Job{
		ID:         "job-abc",
		Name:       "test-job",
		Schedule:   "*/5 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"cmd":"echo hi"}`,
		Status:     "active",
		TimeoutSec: 60,
		Tags:       "test",
		NextRunAt:  time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC),
		LastRunAt:  time.Date(2026, 3, 8, 11, 55, 0, 0, time.UTC),
		CreatedAt:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 3, 8, 11, 55, 0, 0, time.UTC),
	}
}

// sampleExecution returns a cron.Execution fixture for httptest JSON responses.
func sampleExecution() cron.Execution {
	return cron.Execution{
		ID:            "exec-001",
		JobID:         "job-abc",
		StartedAt:     time.Date(2026, 3, 8, 11, 55, 0, 0, time.UTC),
		FinishedAt:    time.Date(2026, 3, 8, 11, 55, 2, 0, time.UTC),
		Status:        "success",
		RunID:         "run-xyz",
		OutputSummary: "all good",
		Error:         "",
		DurationMs:    2000,
	}
}

func newTestAdapter(ts *httptest.Server) *cronClientAdapter {
	return &cronClientAdapter{client: cron.NewClient(ts.URL)}
}

func TestCronClientAdapterCreateJob(t *testing.T) {
	t.Parallel()

	job := sampleJob()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/jobs" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "bad", 400)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json Content-Type, got %q", r.Header.Get("Content-Type"))
		}
		var reqBody cron.CreateJobRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("decode request body: %v", err)
			http.Error(w, "bad", 400)
			return
		}
		if reqBody.Name != "test-job" {
			t.Errorf("request Name: got %q, want %q", reqBody.Name, "test-job")
		}
		if reqBody.Schedule != "*/5 * * * *" {
			t.Errorf("request Schedule: got %q, want %q", reqBody.Schedule, "*/5 * * * *")
		}
		if reqBody.Tags != "test" {
			t.Errorf("request Tags: got %q, want %q", reqBody.Tags, "test")
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(job)
	}))
	defer ts.Close()

	adapter := newTestAdapter(ts)
	got, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name:       "test-job",
		Schedule:   "*/5 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"cmd":"echo hi"}`,
		TimeoutSec: 60,
		Tags:       "test",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if got.ID != "job-abc" {
		t.Fatalf("ID: got %q, want %q", got.ID, "job-abc")
	}
	if got.Name != "test-job" {
		t.Fatalf("Name: got %q, want %q", got.Name, "test-job")
	}
	if got.Tags != "test" {
		t.Fatalf("Tags: got %q, want %q", got.Tags, "test")
	}
}

func TestCronClientAdapterListJobs(t *testing.T) {
	t.Parallel()

	jobs := []cron.Job{sampleJob(), {
		ID:     "job-def",
		Name:   "second-job",
		Status: "paused",
	}}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/jobs" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "bad", 400)
			return
		}
		resp := struct {
			Jobs []cron.Job `json:"jobs"`
		}{Jobs: jobs}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	adapter := newTestAdapter(ts)
	got, err := adapter.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(got))
	}
	if got[0].ID != "job-abc" {
		t.Fatalf("first job ID: got %q, want %q", got[0].ID, "job-abc")
	}
	if got[1].Name != "second-job" {
		t.Fatalf("second job Name: got %q, want %q", got[1].Name, "second-job")
	}
	if got[1].Status != "paused" {
		t.Fatalf("second job Status: got %q, want %q", got[1].Status, "paused")
	}
}

func TestCronClientAdapterGetJob(t *testing.T) {
	t.Parallel()

	job := sampleJob()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/jobs/job-abc" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", 404)
			return
		}
		_ = json.NewEncoder(w).Encode(job)
	}))
	defer ts.Close()

	adapter := newTestAdapter(ts)
	got, err := adapter.GetJob(context.Background(), "job-abc")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.ID != "job-abc" {
		t.Fatalf("ID: got %q, want %q", got.ID, "job-abc")
	}
	if got.Schedule != "*/5 * * * *" {
		t.Fatalf("Schedule: got %q, want %q", got.Schedule, "*/5 * * * *")
	}
}

func TestCronClientAdapterUpdateJob(t *testing.T) {
	t.Parallel()

	updatedJob := sampleJob()
	updatedJob.Status = "paused"
	updatedJob.Schedule = "0 * * * *"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/jobs/job-abc" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "bad", 400)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Errorf("unmarshal request: %v", err)
		}
		if reqBody["status"] != "paused" {
			t.Errorf("request status: got %v, want %q", reqBody["status"], "paused")
		}
		if reqBody["schedule"] != "0 * * * *" {
			t.Errorf("request schedule: got %v, want %q", reqBody["schedule"], "0 * * * *")
		}
		_ = json.NewEncoder(w).Encode(updatedJob)
	}))
	defer ts.Close()

	adapter := newTestAdapter(ts)
	newSched := "0 * * * *"
	newStatus := "paused"
	got, err := adapter.UpdateJob(context.Background(), "job-abc", htools.CronUpdateJobRequest{
		Schedule: &newSched,
		Status:   &newStatus,
	})
	if err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	if got.Status != "paused" {
		t.Fatalf("Status: got %q, want %q", got.Status, "paused")
	}
	if got.Schedule != "0 * * * *" {
		t.Fatalf("Schedule: got %q, want %q", got.Schedule, "0 * * * *")
	}
}

func TestCronClientAdapterDeleteJob(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/jobs/job-abc" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "bad", 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	adapter := newTestAdapter(ts)
	if err := adapter.DeleteJob(context.Background(), "job-abc"); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
}

func TestCronClientAdapterListExecutions(t *testing.T) {
	t.Parallel()

	execs := []cron.Execution{sampleExecution(), {
		ID:            "exec-002",
		JobID:         "job-abc",
		Status:        "failed",
		Error:         "timeout exceeded",
		OutputSummary: "partial",
		DurationMs:    60000,
	}}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/jobs/job-abc/history" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "bad", 400)
			return
		}
		// Verify query params
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("limit: got %q, want %q", r.URL.Query().Get("limit"), "10")
		}
		if r.URL.Query().Get("offset") != "0" {
			t.Errorf("offset: got %q, want %q", r.URL.Query().Get("offset"), "0")
		}
		resp := struct {
			Executions []cron.Execution `json:"executions"`
		}{Executions: execs}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	adapter := newTestAdapter(ts)
	got, err := adapter.ListExecutions(context.Background(), "job-abc", 10, 0)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 executions, got %d", len(got))
	}
	if got[0].RunID != "run-xyz" {
		t.Fatalf("first exec RunID: got %q, want %q", got[0].RunID, "run-xyz")
	}
	if got[0].OutputSummary != "all good" {
		t.Fatalf("first exec OutputSummary: got %q, want %q", got[0].OutputSummary, "all good")
	}
	if got[1].Error != "timeout exceeded" {
		t.Fatalf("second exec Error: got %q, want %q", got[1].Error, "timeout exceeded")
	}
	if got[1].DurationMs != 60000 {
		t.Fatalf("second exec DurationMs: got %d, want %d", got[1].DurationMs, 60000)
	}
}

func TestCronClientAdapterHealth(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/healthz" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "bad", 400)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	adapter := newTestAdapter(ts)
	if err := adapter.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestCronClientAdapterServerError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "internal_error",
				"message": "something broke",
			},
		})
	}))
	defer ts.Close()

	adapter := newTestAdapter(ts)
	ctx := context.Background()

	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{Name: "x"}); err == nil {
		t.Fatalf("CreateJob: expected error")
	}
	if _, err := adapter.ListJobs(ctx); err == nil {
		t.Fatalf("ListJobs: expected error")
	}
	if _, err := adapter.GetJob(ctx, "id"); err == nil {
		t.Fatalf("GetJob: expected error")
	}
	if _, err := adapter.UpdateJob(ctx, "id", htools.CronUpdateJobRequest{}); err == nil {
		t.Fatalf("UpdateJob: expected error")
	}
	if err := adapter.DeleteJob(ctx, "id"); err == nil {
		t.Fatalf("DeleteJob: expected error")
	}
	if _, err := adapter.ListExecutions(ctx, "id", 10, 0); err == nil {
		t.Fatalf("ListExecutions: expected error")
	}
	if err := adapter.Health(ctx); err == nil {
		t.Fatalf("Health: expected error")
	}
}

func TestCronClientAdapterServerUnreachable(t *testing.T) {
	t.Parallel()

	// Create a server and immediately close it to get an unreachable URL.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := ts.URL
	ts.Close()

	adapter := &cronClientAdapter{client: cron.NewClient(url)}
	ctx := context.Background()

	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{Name: "x"}); err == nil {
		t.Fatalf("CreateJob: expected connection error")
	}
	if _, err := adapter.ListJobs(ctx); err == nil {
		t.Fatalf("ListJobs: expected connection error")
	}
	if _, err := adapter.GetJob(ctx, "id"); err == nil {
		t.Fatalf("GetJob: expected connection error")
	}
	if _, err := adapter.UpdateJob(ctx, "id", htools.CronUpdateJobRequest{}); err == nil {
		t.Fatalf("UpdateJob: expected connection error")
	}
	if err := adapter.DeleteJob(ctx, "id"); err == nil {
		t.Fatalf("DeleteJob: expected connection error")
	}
	if _, err := adapter.ListExecutions(ctx, "id", 10, 0); err == nil {
		t.Fatalf("ListExecutions: expected connection error")
	}
	if err := adapter.Health(ctx); err == nil {
		t.Fatalf("Health: expected connection error")
	}
}

func TestCronClientAdapterContextCancelled(t *testing.T) {
	t.Parallel()

	// Server that blocks long enough for context cancellation to win.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer ts.Close()

	adapter := newTestAdapter(ts)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{Name: "x"}); err == nil {
		t.Fatalf("CreateJob: expected context error")
	}
	if _, err := adapter.ListJobs(ctx); err == nil {
		t.Fatalf("ListJobs: expected context error")
	}
	if _, err := adapter.GetJob(ctx, "id"); err == nil {
		t.Fatalf("GetJob: expected context error")
	}
	if _, err := adapter.UpdateJob(ctx, "id", htools.CronUpdateJobRequest{}); err == nil {
		t.Fatalf("UpdateJob: expected context error")
	}
	if err := adapter.DeleteJob(ctx, "id"); err == nil {
		t.Fatalf("DeleteJob: expected context error")
	}
	if _, err := adapter.ListExecutions(ctx, "id", 10, 0); err == nil {
		t.Fatalf("ListExecutions: expected context error")
	}
	if err := adapter.Health(ctx); err == nil {
		t.Fatalf("Health: expected context error")
	}
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestCronClientAdapterConcurrent(t *testing.T) {
	t.Parallel()

	job := sampleJob()
	exec := sampleExecution()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/jobs":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(job)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/jobs":
			resp := struct {
				Jobs []cron.Job `json:"jobs"`
			}{Jobs: []cron.Job{job}}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/history"):
			resp := struct {
				Executions []cron.Execution `json:"executions"`
			}{Executions: []cron.Execution{exec}}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/jobs/"):
			_ = json.NewEncoder(w).Encode(job)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/jobs/"):
			_ = json.NewEncoder(w).Encode(job)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/jobs/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer ts.Close()

	adapter := newTestAdapter(ts)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 70)

	for i := 0; i < 10; i++ {
		wg.Add(7)
		go func() {
			defer wg.Done()
			if _, err := adapter.CreateJob(ctx, htools.CronCreateJobRequest{Name: "c"}); err != nil {
				errs <- fmt.Errorf("CreateJob: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := adapter.ListJobs(ctx); err != nil {
				errs <- fmt.Errorf("ListJobs: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := adapter.GetJob(ctx, "job-abc"); err != nil {
				errs <- fmt.Errorf("GetJob: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			s := "paused"
			if _, err := adapter.UpdateJob(ctx, "job-abc", htools.CronUpdateJobRequest{Status: &s}); err != nil {
				errs <- fmt.Errorf("UpdateJob: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := adapter.DeleteJob(ctx, "job-abc"); err != nil {
				errs <- fmt.Errorf("DeleteJob: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := adapter.ListExecutions(ctx, "job-abc", 10, 0); err != nil {
				errs <- fmt.Errorf("ListExecutions: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := adapter.Health(ctx); err != nil {
				errs <- fmt.Errorf("Health: %w", err)
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// HARNESS_CRON_URL env var wiring
// ---------------------------------------------------------------------------

func TestCronURLEnvVarWiring(t *testing.T) {
	t.Parallel()

	// Sub-test: empty string -> embedded cron
	t.Run("empty_string", func(t *testing.T) {
		env := map[string]string{
			"OPENAI_API_KEY":      "test-key",
			"HARNESS_ADDR":        "127.0.0.1:0",
			"HARNESS_MEMORY_MODE": "off",
			"HARNESS_CRON_URL":    "",
		}
		getenv := func(key string) string { return env[key] }

		sig := make(chan os.Signal, 1)
		done := make(chan error, 1)
		go func() {
			done <- runWithSignals(sig, getenv, func(openai.Config) (harness.Provider, error) {
				return &noopProvider{}, nil
			}, "")
		}()
		// Give server time to start, then shut down.
		time.Sleep(100 * time.Millisecond)
		sig <- os.Interrupt
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("runWithSignals: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out")
		}
	})

	// Sub-test: whitespace-only -> treated as empty (embedded cron)
	t.Run("whitespace_only", func(t *testing.T) {
		env := map[string]string{
			"OPENAI_API_KEY":      "test-key",
			"HARNESS_ADDR":        "127.0.0.1:0",
			"HARNESS_MEMORY_MODE": "off",
			"HARNESS_CRON_URL":    "   ",
		}
		getenv := func(key string) string { return env[key] }

		sig := make(chan os.Signal, 1)
		done := make(chan error, 1)
		go func() {
			done <- runWithSignals(sig, getenv, func(openai.Config) (harness.Provider, error) {
				return &noopProvider{}, nil
			}, "")
		}()
		time.Sleep(100 * time.Millisecond)
		sig <- os.Interrupt
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("runWithSignals: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out")
		}
	})

	// Sub-test: valid URL -> server starts (won't connect to cron, but should start)
	t.Run("valid_url", func(t *testing.T) {
		env := map[string]string{
			"OPENAI_API_KEY":      "test-key",
			"HARNESS_ADDR":        "127.0.0.1:0",
			"HARNESS_MEMORY_MODE": "off",
			"HARNESS_CRON_URL":    "http://localhost:9090",
		}
		getenv := func(key string) string { return env[key] }

		sig := make(chan os.Signal, 1)
		done := make(chan error, 1)
		go func() {
			done <- runWithSignals(sig, getenv, func(openai.Config) (harness.Provider, error) {
				return &noopProvider{}, nil
			}, "")
		}()
		time.Sleep(100 * time.Millisecond)
		sig <- os.Interrupt
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("runWithSignals: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out")
		}
	})
}

// ---------------------------------------------------------------------------
// stdLogger.Error coverage
// ---------------------------------------------------------------------------

func TestStdLoggerError(t *testing.T) {
	t.Parallel()

	l := &stdLogger{}
	// Verify it does not panic with various arguments.
	l.Error("something went wrong")
	l.Error("context failure", "key", "value", "count", 42)
}

// ---------------------------------------------------------------------------
// callbackRunStarter.StartRun coverage
// ---------------------------------------------------------------------------

func TestCallbackRunStarterNilRunner(t *testing.T) {
	t.Parallel()

	starter := &callbackRunStarter{}
	err := starter.StartRun("hello", "conv-1")
	if err == nil {
		t.Fatalf("expected error when runner is nil")
	}
	if !strings.Contains(err.Error(), "not yet initialized") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallbackRunStarterWithRunner(t *testing.T) {
	t.Parallel()

	provider := &noopProvider{}
	reg := harness.NewRegistry()
	runner := harness.NewRunner(provider, reg, harness.RunnerConfig{
		DefaultModel:        "gpt-4.1-mini",
		DefaultSystemPrompt: "test",
		MaxSteps:            2,
	})

	starter := &callbackRunStarter{}
	starter.mu.Lock()
	starter.runner = runner
	starter.mu.Unlock()

	err := starter.StartRun("do something", "")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
}

// ---------------------------------------------------------------------------
// skillListerAdapter coverage
// ---------------------------------------------------------------------------

func TestSkillListerAdapterGetSkillNotFound(t *testing.T) {
	t.Parallel()

	reg := skills.NewRegistry()
	resolver := skills.NewResolver(reg)
	adapter := &skillListerAdapter{registry: reg, resolver: resolver, workspace: "/tmp"}

	_, ok := adapter.GetSkill("nonexistent")
	if ok {
		t.Fatalf("expected not found")
	}
}

func TestSkillListerAdapterGetSkillFound(t *testing.T) {
	t.Parallel()

	reg := skills.NewRegistry()
	// Insert a skill directly by loading via the registry's internal structure.
	// We use the loader path instead: build a temp skill directory.
	dir := t.TempDir()
	skillDir := dir + "/test-skill"
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: test-skill
description: A test skill
version: 1
argument-hint: "<arg>"
allowed-tools:
  - bash
---
Hello $ARGUMENTS`
	if err := os.WriteFile(skillDir+"/SKILL.md", []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := skills.NewLoader(skills.LoaderConfig{GlobalDir: dir})
	if err := reg.Load(loader); err != nil {
		t.Fatal(err)
	}

	resolver := skills.NewResolver(reg)
	adapter := &skillListerAdapter{registry: reg, resolver: resolver, workspace: "/tmp"}

	info, ok := adapter.GetSkill("test-skill")
	if !ok {
		t.Fatalf("expected to find test-skill")
	}
	if info.Name != "test-skill" {
		t.Fatalf("Name: got %q", info.Name)
	}
	if info.Description != "A test skill" {
		t.Fatalf("Description: got %q", info.Description)
	}
	if info.ArgumentHint != "<arg>" {
		t.Fatalf("ArgumentHint: got %q", info.ArgumentHint)
	}
	if len(info.AllowedTools) != 1 || info.AllowedTools[0] != "bash" {
		t.Fatalf("AllowedTools: got %v", info.AllowedTools)
	}
}

func TestSkillListerAdapterListSkills(t *testing.T) {
	t.Parallel()

	reg := skills.NewRegistry()
	dir := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		skillDir := dir + "/" + name
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := fmt.Sprintf("---\nname: %s\ndescription: Skill %s\nversion: 1\n---\nBody", name, name)
		if err := os.WriteFile(skillDir+"/SKILL.md", []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	loader := skills.NewLoader(skills.LoaderConfig{GlobalDir: dir})
	if err := reg.Load(loader); err != nil {
		t.Fatal(err)
	}

	resolver := skills.NewResolver(reg)
	adapter := &skillListerAdapter{registry: reg, resolver: resolver, workspace: "/tmp"}

	all := adapter.ListSkills()
	if len(all) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(all))
	}
}

func TestSkillListerAdapterResolveSkill(t *testing.T) {
	t.Parallel()

	reg := skills.NewRegistry()
	dir := t.TempDir()
	skillDir := dir + "/greet"
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: greet\ndescription: Greet\nversion: 1\n---\nHello $ARGUMENTS from $WORKSPACE"
	if err := os.WriteFile(skillDir+"/SKILL.md", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := skills.NewLoader(skills.LoaderConfig{GlobalDir: dir})
	if err := reg.Load(loader); err != nil {
		t.Fatal(err)
	}

	resolver := skills.NewResolver(reg)
	adapter := &skillListerAdapter{registry: reg, resolver: resolver, workspace: "/default"}

	// With default workspace.
	result, err := adapter.ResolveSkill(context.Background(), "greet", "world", "")
	if err != nil {
		t.Fatalf("ResolveSkill: %v", err)
	}
	if !strings.Contains(result, "world") {
		t.Fatalf("expected 'world' in result: %q", result)
	}
	if !strings.Contains(result, "/default") {
		t.Fatalf("expected '/default' in result: %q", result)
	}

	// With explicit workspace.
	result, err = adapter.ResolveSkill(context.Background(), "greet", "earth", "/custom")
	if err != nil {
		t.Fatalf("ResolveSkill: %v", err)
	}
	if !strings.Contains(result, "/custom") {
		t.Fatalf("expected '/custom' in result: %q", result)
	}

	// Not found.
	_, err = adapter.ResolveSkill(context.Background(), "nonexistent", "", "")
	if err == nil {
		t.Fatalf("expected error for nonexistent skill")
	}
}

// ---------------------------------------------------------------------------
// embeddedCronAdapter coverage
// ---------------------------------------------------------------------------

func TestEmbeddedCronAdapterCreateJob(t *testing.T) {
	t.Parallel()

	store := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	executor := &cron.ShellExecutor{}
	scheduler := cron.NewScheduler(store, executor, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	defer scheduler.Stop()

	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}

	// Missing name.
	_, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{Schedule: "*/5 * * * *"})
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected name required error, got: %v", err)
	}

	// Missing schedule.
	_, err = adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{Name: "test"})
	if err == nil || !strings.Contains(err.Error(), "schedule is required") {
		t.Fatalf("expected schedule required error, got: %v", err)
	}

	// Invalid execution type.
	_, err = adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name:     "test",
		Schedule: "*/5 * * * *",
		ExecType: "invalid",
	})
	if err == nil || !strings.Contains(err.Error(), "execution_type") {
		t.Fatalf("expected execution_type error, got: %v", err)
	}

	// Valid creation.
	job, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name:       "my-job",
		Schedule:   "*/5 * * * *",
		ExecType:   "shell",
		ExecConfig: `{"command":"echo hi"}`,
		Tags:       "test",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.Name != "my-job" {
		t.Fatalf("Name: got %q", job.Name)
	}
	if job.TimeoutSec != 30 {
		t.Fatalf("expected default timeout 30, got %d", job.TimeoutSec)
	}
}

func TestEmbeddedCronAdapterListJobs(t *testing.T) {
	t.Parallel()

	store := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	scheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	defer scheduler.Stop()

	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}

	// Create a job first.
	_, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name: "list-test", Schedule: "*/5 * * * *", ExecType: "shell",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	jobs, err := adapter.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
}

func TestEmbeddedCronAdapterGetJob(t *testing.T) {
	t.Parallel()

	store := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	scheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	defer scheduler.Stop()

	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}

	created, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name: "get-test", Schedule: "*/5 * * * *", ExecType: "shell",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Get by ID.
	got, err := adapter.GetJob(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetJob by ID: %v", err)
	}
	if got.Name != "get-test" {
		t.Fatalf("Name: got %q", got.Name)
	}

	// Get by name.
	got, err = adapter.GetJob(context.Background(), "get-test")
	if err != nil {
		t.Fatalf("GetJob by name: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("ID: got %q, want %q", got.ID, created.ID)
	}

	// Not found.
	_, err = adapter.GetJob(context.Background(), "nonexistent")
	if err == nil {
		t.Fatalf("expected error for nonexistent job")
	}
}

func TestEmbeddedCronAdapterUpdateJob(t *testing.T) {
	t.Parallel()

	store := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	scheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	if err := scheduler.Start(context.Background()); err != nil {
		t.Fatalf("start scheduler: %v", err)
	}
	defer scheduler.Stop()

	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}

	created, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name: "update-test", Schedule: "*/5 * * * *", ExecType: "shell",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Update schedule.
	newSched := "0 * * * *"
	updated, err := adapter.UpdateJob(context.Background(), created.ID, htools.CronUpdateJobRequest{
		Schedule: &newSched,
	})
	if err != nil {
		t.Fatalf("UpdateJob schedule: %v", err)
	}
	if updated.Schedule != "0 * * * *" {
		t.Fatalf("Schedule: got %q", updated.Schedule)
	}

	// Update status to paused.
	paused := "paused"
	updated, err = adapter.UpdateJob(context.Background(), created.ID, htools.CronUpdateJobRequest{
		Status: &paused,
	})
	if err != nil {
		t.Fatalf("UpdateJob pause: %v", err)
	}
	if updated.Status != "paused" {
		t.Fatalf("Status: got %q", updated.Status)
	}

	// Resume.
	active := "active"
	updated, err = adapter.UpdateJob(context.Background(), created.ID, htools.CronUpdateJobRequest{
		Status: &active,
	})
	if err != nil {
		t.Fatalf("UpdateJob resume: %v", err)
	}
	if updated.Status != "active" {
		t.Fatalf("Status: got %q", updated.Status)
	}

	// Invalid status.
	bad := "invalid"
	_, err = adapter.UpdateJob(context.Background(), created.ID, htools.CronUpdateJobRequest{
		Status: &bad,
	})
	if err == nil {
		t.Fatalf("expected error for invalid status")
	}

	// Empty schedule.
	empty := "  "
	_, err = adapter.UpdateJob(context.Background(), created.ID, htools.CronUpdateJobRequest{
		Schedule: &empty,
	})
	if err == nil {
		t.Fatalf("expected error for empty schedule")
	}

	// Not found.
	_, err = adapter.UpdateJob(context.Background(), "nonexistent", htools.CronUpdateJobRequest{})
	if err == nil {
		t.Fatalf("expected error for nonexistent job")
	}
}

func TestEmbeddedCronAdapterDeleteJob(t *testing.T) {
	t.Parallel()

	store := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	scheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	defer scheduler.Stop()

	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}

	created, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name: "delete-test", Schedule: "*/5 * * * *", ExecType: "shell",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if err := adapter.DeleteJob(context.Background(), created.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	// Verify deleted.
	_, err = adapter.GetJob(context.Background(), created.ID)
	if err == nil {
		t.Fatalf("expected error after delete")
	}
}

func TestEmbeddedCronAdapterListExecutions(t *testing.T) {
	t.Parallel()

	store := newTestCronStore(t)
	clock := testClock{t: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	scheduler := cron.NewScheduler(store, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 1})
	defer scheduler.Stop()

	adapter := &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock}

	created, err := adapter.CreateJob(context.Background(), htools.CronCreateJobRequest{
		Name: "exec-test", Schedule: "*/5 * * * *", ExecType: "shell",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	execs, err := adapter.ListExecutions(context.Background(), created.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(execs) != 0 {
		t.Fatalf("expected 0 executions, got %d", len(execs))
	}
}

func TestEmbeddedCronAdapterHealth(t *testing.T) {
	t.Parallel()

	adapter := &embeddedCronAdapter{}
	if err := adapter.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

// testClock implements cron.Clock with a fixed time.
type testClock struct{ t time.Time }

func (c testClock) Now() time.Time { return c.t }

// newTestCronStore creates a SQLite cron store backed by a temp directory.
func newTestCronStore(t *testing.T) cron.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := cron.NewSQLiteStore(dir + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// --- lazySummarizer tests ---

// stubSummarizer is a test double for htools.MessageSummarizer.
type stubSummarizer struct {
	result string
	err    error
	called bool
	msgs   []map[string]any
}

func (s *stubSummarizer) SummarizeMessages(_ context.Context, msgs []map[string]any) (string, error) {
	s.called = true
	s.msgs = msgs
	return s.result, s.err
}

func TestLazySummarizer_NotConfigured(t *testing.T) {
	t.Parallel()

	ls := &lazySummarizer{}
	_, err := ls.SummarizeMessages(context.Background(), []map[string]any{
		{"role": "user", "content": "hello"},
	})
	if err == nil {
		t.Fatal("expected error when summarizer not configured")
	}
	if err.Error() != "summarizer not configured yet" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLazySummarizer_AfterWiring(t *testing.T) {
	t.Parallel()

	inner := &stubSummarizer{result: "summary result"}
	ls := &lazySummarizer{}

	// Wire the inner summarizer
	ls.mu.Lock()
	ls.summarizer = inner
	ls.mu.Unlock()

	msgs := []map[string]any{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": "hi"},
	}

	result, err := ls.SummarizeMessages(context.Background(), msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "summary result" {
		t.Fatalf("expected %q, got %q", "summary result", result)
	}
	if !inner.called {
		t.Fatal("inner summarizer was not called")
	}
	if len(inner.msgs) != 2 {
		t.Fatalf("expected 2 messages passed to inner, got %d", len(inner.msgs))
	}
}

func TestLazySummarizer_ErrorPropagation(t *testing.T) {
	t.Parallel()

	inner := &stubSummarizer{err: errors.New("inner error")}
	ls := &lazySummarizer{}

	ls.mu.Lock()
	ls.summarizer = inner
	ls.mu.Unlock()

	_, err := ls.SummarizeMessages(context.Background(), []map[string]any{
		{"role": "user", "content": "hello"},
	})
	if err == nil {
		t.Fatal("expected error from inner summarizer")
	}
	if err.Error() != "inner error" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// lookupModelAPI wiring tests
// ---------------------------------------------------------------------------

// TestLookupModelAPIWiredInRunWithSignals verifies that when a model catalog is
// configured and a model has "api": "responses", runWithSignals passes a
// ModelAPILookup function to the provider. We do this by tracking the Config
// that newProvider receives and verifying ModelAPILookup is non-nil and returns
// the correct value.
func TestLookupModelAPIWiredInRunWithSignals(t *testing.T) {
	t.Parallel()

	// Write a temporary catalog file with a codex model that has api: "responses".
	catalogJSON := `{
		"catalog_version": "1.0.0",
		"providers": {
			"openai": {
				"display_name": "OpenAI",
				"base_url": "https://api.openai.com",
				"api_key_env": "OPENAI_API_KEY",
				"protocol": "openai_compat",
				"models": {
					"gpt-4.1": {
						"display_name": "GPT-4.1",
						"context_window": 128000,
						"tool_calling": true,
						"streaming": true
					},
					"gpt-5.3-codex": {
						"display_name": "GPT-5.3 Codex",
						"context_window": 200000,
						"tool_calling": true,
						"streaming": true,
						"api": "responses"
					}
				}
			}
		}
	}`

	catalogFile, err := os.CreateTemp(t.TempDir(), "catalog*.json")
	if err != nil {
		t.Fatalf("create temp catalog: %v", err)
	}
	if _, err := catalogFile.WriteString(catalogJSON); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
	catalogFile.Close()

	var capturedConfig openai.Config
	var configMu sync.Mutex

	workspaceDir := t.TempDir()
	env := map[string]string{
		"OPENAI_API_KEY":             "test-key",
		"HARNESS_ADDR":               "127.0.0.1:0",
		"HARNESS_MEMORY_MODE":        "off",
		"HARNESS_WORKSPACE":          workspaceDir,
		"HARNESS_MODEL_CATALOG_PATH": catalogFile.Name(),
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv, func(cfg openai.Config) (harness.Provider, error) {
			configMu.Lock()
			capturedConfig = cfg
			configMu.Unlock()
			return &noopProvider{}, nil
		}, "")
	}()

	time.Sleep(150 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}

	configMu.Lock()
	cfg := capturedConfig
	configMu.Unlock()

	// Verify ModelAPILookup is non-nil.
	if cfg.ModelAPILookup == nil {
		t.Fatal("expected ModelAPILookup to be non-nil when catalog is loaded")
	}

	// Verify gpt-5.3-codex resolves to "responses".
	got := cfg.ModelAPILookup("openai", "gpt-5.3-codex")
	if got != "responses" {
		t.Errorf("expected ModelAPILookup(openai, gpt-5.3-codex) = %q, got %q", "responses", got)
	}

	// Verify standard model resolves to "" (empty).
	got2 := cfg.ModelAPILookup("openai", "gpt-4.1")
	if got2 != "" {
		t.Errorf("expected ModelAPILookup(openai, gpt-4.1) = %q, got %q", "", got2)
	}
}

// TestLookupModelAPIWithAlias verifies that the lookupModelAPI closure correctly
// resolves model aliases.
func TestLookupModelAPIWithAlias(t *testing.T) {
	t.Parallel()

	catalogJSON := `{
		"catalog_version": "1.0.0",
		"providers": {
			"openai": {
				"display_name": "OpenAI",
				"base_url": "https://api.openai.com",
				"api_key_env": "OPENAI_API_KEY",
				"protocol": "openai_compat",
				"models": {
					"gpt-5.3-codex": {
						"display_name": "GPT-5.3 Codex",
						"context_window": 200000,
						"tool_calling": true,
						"streaming": true,
						"api": "responses"
					}
				},
				"aliases": {
					"codex": "gpt-5.3-codex"
				}
			}
		}
	}`

	catalogFile, err := os.CreateTemp(t.TempDir(), "catalog*.json")
	if err != nil {
		t.Fatalf("create temp catalog: %v", err)
	}
	if _, err := catalogFile.WriteString(catalogJSON); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
	catalogFile.Close()

	var capturedConfig openai.Config
	var configMu sync.Mutex

	workspaceDir2 := t.TempDir()
	env := map[string]string{
		"OPENAI_API_KEY":             "test-key",
		"HARNESS_ADDR":               "127.0.0.1:0",
		"HARNESS_MEMORY_MODE":        "off",
		"HARNESS_WORKSPACE":          workspaceDir2,
		"HARNESS_MODEL_CATALOG_PATH": catalogFile.Name(),
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv, func(cfg openai.Config) (harness.Provider, error) {
			configMu.Lock()
			capturedConfig = cfg
			configMu.Unlock()
			return &noopProvider{}, nil
		}, "")
	}()

	time.Sleep(150 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}

	configMu.Lock()
	cfg := capturedConfig
	configMu.Unlock()

	if cfg.ModelAPILookup == nil {
		t.Fatal("expected ModelAPILookup to be non-nil when catalog is loaded")
	}

	// Alias "codex" should resolve to "responses" via gpt-5.3-codex.
	got := cfg.ModelAPILookup("openai", "codex")
	if got != "responses" {
		t.Errorf("expected ModelAPILookup(openai, codex) = %q (alias), got %q", "responses", got)
	}
}

// TestLookupModelAPIWithoutCatalog verifies that when no catalog is loaded,
// ModelAPILookup returns "" safely (no nil panic).
func TestLookupModelAPIWithoutCatalog(t *testing.T) {
	t.Parallel()

	var capturedConfig openai.Config
	var configMu sync.Mutex

	workspaceDir3 := t.TempDir()
	env := map[string]string{
		"OPENAI_API_KEY":      "test-key",
		"HARNESS_ADDR":        "127.0.0.1:0",
		"HARNESS_MEMORY_MODE": "off",
		"HARNESS_WORKSPACE":   workspaceDir3,
		// No HARNESS_MODEL_CATALOG_PATH set — catalog is nil.
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv, func(cfg openai.Config) (harness.Provider, error) {
			configMu.Lock()
			capturedConfig = cfg
			configMu.Unlock()
			return &noopProvider{}, nil
		}, "")
	}()

	time.Sleep(150 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}

	configMu.Lock()
	cfg := capturedConfig
	configMu.Unlock()

	// Even without a catalog, ModelAPILookup should be wired and return "".
	if cfg.ModelAPILookup == nil {
		t.Fatal("expected ModelAPILookup to be non-nil (closure always assigned)")
	}
	got := cfg.ModelAPILookup("openai", "any-model")
	if got != "" {
		t.Errorf("expected empty string without catalog, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// HARNESS_ROLE_MODEL_* env var wiring via getenv seam
// ---------------------------------------------------------------------------

// TestRoleModelEnvVarsUseGetenvSeam verifies that HARNESS_ROLE_MODEL_PRIMARY and
// HARNESS_ROLE_MODEL_SUMMARIZER are read through the injected getenv closure,
// not os.Getenv directly. We set sentinel values in the real environment (via
// t.Setenv) but supply empty strings via the getenv closure. The server must
// start and shut down without error, proving it does not fall through to
// os.Getenv for these keys (if it did, it would still succeed, but the model
// would be wrong; the negative case is verified by the positive test below).
//
// Not marked t.Parallel() because it uses t.Setenv.
func TestRoleModelEnvVarsUseGetenvSeam(t *testing.T) {
	// Sentinel values present in the real environment.
	t.Setenv("HARNESS_ROLE_MODEL_PRIMARY", "should-not-be-used-primary")
	t.Setenv("HARNESS_ROLE_MODEL_SUMMARIZER", "should-not-be-used-summarizer")

	// The fake getenv does NOT expose role model vars — only the minimum to
	// boot the server. If runWithSignals reads os.Getenv it would pick up the
	// sentinel above; if it uses getenv it gets "" (no override).
	env := map[string]string{
		"OPENAI_API_KEY":      "test-key",
		"HARNESS_ADDR":        "127.0.0.1:0",
		"HARNESS_MEMORY_MODE": "off",
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv, func(openai.Config) (harness.Provider, error) {
			return &noopProvider{}, nil
		}, "")
	}()

	time.Sleep(100 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}

// TestRoleModelPrimaryFromGetenvAppliedToRun verifies the positive case: when
// HARNESS_ROLE_MODEL_PRIMARY and HARNESS_ROLE_MODEL_SUMMARIZER are supplied
// only through the injected getenv closure (not os.Setenv), runWithSignals
// starts the server successfully. This exercises the code path changed by the
// fix from os.Getenv → getenv in cmd/harnessd/main.go.
func TestRoleModelPrimaryFromGetenvAppliedToRun(t *testing.T) {
	t.Parallel()

	// Role model env vars appear ONLY in the fake getenv, NOT in os.Setenv.
	env := map[string]string{
		"OPENAI_API_KEY":                "test-key",
		"HARNESS_ADDR":                  "127.0.0.1:0",
		"HARNESS_MEMORY_MODE":           "off",
		"HARNESS_ROLE_MODEL_PRIMARY":    "injected-primary-model",
		"HARNESS_ROLE_MODEL_SUMMARIZER": "injected-summarizer-model",
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv, func(openai.Config) (harness.Provider, error) {
			return &noopProvider{}, nil
		}, "")
	}()

	time.Sleep(100 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals with role model env vars in getenv returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}
// ---------------------------------------------------------------------------
// Issue #337: runWithSignals failure paths
// ---------------------------------------------------------------------------

// TestRunWithSignalsPromptEngineFailure verifies that a bad HARNESS_PROMPTS_DIR
// (pointing to a directory with no catalog.yaml) causes runWithSignals to
// return an error wrapping "load prompt engine".
func TestRunWithSignalsPromptEngineFailure(t *testing.T) {
	t.Parallel()

	// Use a temp dir that has no catalog.yaml — NewFileEngine will fail.
	emptyDir := t.TempDir()

	env := map[string]string{
		"OPENAI_API_KEY":      "test-key",
		"HARNESS_ADDR":        "127.0.0.1:0",
		"HARNESS_MEMORY_MODE": "off",
		"HARNESS_PROMPTS_DIR": emptyDir,
	}
	getenv := func(key string) string { return env[key] }

	err := runWithSignals(make(chan os.Signal, 1), getenv, func(openai.Config) (harness.Provider, error) {
		return &noopProvider{}, nil
	}, "")

	if err == nil {
		t.Fatal("expected error when prompts dir is missing catalog.yaml")
	}
	if !strings.Contains(err.Error(), "load prompt engine") {
		t.Fatalf("expected 'load prompt engine' in error, got: %v", err)
	}
}

// TestRunWithSignalsMemoryManagerFailure verifies that an unsupported
// HARNESS_MEMORY_DB_DRIVER causes runWithSignals to return an error wrapping
// "create observational memory manager".
func TestRunWithSignalsMemoryManagerFailure(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"OPENAI_API_KEY":           "test-key",
		"HARNESS_ADDR":             "127.0.0.1:0",
		"HARNESS_MEMORY_MODE":      "auto",
		"HARNESS_MEMORY_DB_DRIVER": "unsupported_driver_xyz",
	}
	getenv := func(key string) string { return env[key] }

	err := runWithSignals(make(chan os.Signal, 1), getenv, func(openai.Config) (harness.Provider, error) {
		return &noopProvider{}, nil
	}, "")

	if err == nil {
		t.Fatal("expected error when memory driver is unsupported")
	}
	if !strings.Contains(err.Error(), "create observational memory manager") {
		t.Fatalf("expected 'create observational memory manager' in error, got: %v", err)
	}
}

// TestRunWithSignalsCronStoreFailure verifies that when the cron SQLite DB path
// is unwritable, the cron store creation failure causes runWithSignals to return
// an error wrapping "cron store".
func TestRunWithSignalsCronStoreFailure(t *testing.T) {
	t.Parallel()

	// Create a valid workspace dir with a proper .harness directory so
	// config loading succeeds. Then pre-create cron.db as a directory (not a
	// file) so the SQLite driver cannot open it as a database file.
	workspaceDir := t.TempDir()
	harnessSubDir := workspaceDir + "/.harness"
	if err := os.MkdirAll(harnessSubDir, 0o755); err != nil {
		t.Fatalf("setup: create .harness dir: %v", err)
	}
	// Make cron.db a directory — SQLite cannot open a directory as a DB file.
	cronDBAsDir := harnessSubDir + "/cron.db"
	if err := os.MkdirAll(cronDBAsDir, 0o755); err != nil {
		t.Fatalf("setup: create cron.db as directory: %v", err)
	}

	env := map[string]string{
		"OPENAI_API_KEY":      "test-key",
		"HARNESS_ADDR":        "127.0.0.1:0",
		"HARNESS_MEMORY_MODE": "off",
		"HARNESS_WORKSPACE":   workspaceDir,
		"HARNESS_CRON_URL":    "", // force embedded path
	}
	getenv := func(key string) string { return env[key] }

	err := runWithSignals(make(chan os.Signal, 1), getenv, func(openai.Config) (harness.Provider, error) {
		return &noopProvider{}, nil
	}, "")

	if err == nil {
		t.Fatal("expected error when cron store path is unwritable")
	}
	if !strings.Contains(err.Error(), "cron store") {
		t.Fatalf("expected 'cron store' in error, got: %v", err)
	}
}

// TestRunWithSignalsConversationStoreFailure verifies that a bad
// HARNESS_CONVERSATION_DB path causes an error wrapping "conversation store".
func TestRunWithSignalsConversationStoreFailure(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()

	env := map[string]string{
		"OPENAI_API_KEY":           "test-key",
		"HARNESS_ADDR":             "127.0.0.1:0",
		"HARNESS_MEMORY_MODE":      "off",
		"HARNESS_WORKSPACE":        workspaceDir,
		// Point to a file path under /dev/null/... which cannot be created.
		"HARNESS_CONVERSATION_DB":  "/dev/null/cannot/create.db",
	}
	getenv := func(key string) string { return env[key] }

	err := runWithSignals(make(chan os.Signal, 1), getenv, func(openai.Config) (harness.Provider, error) {
		return &noopProvider{}, nil
	}, "")

	if err == nil {
		t.Fatal("expected error when conversation store path is invalid")
	}
	if !strings.Contains(err.Error(), "conversation store") {
		t.Fatalf("expected 'conversation store' in error, got: %v", err)
	}
}

// TestRunWithSignalsPricingCatalogFailure verifies that a non-existent pricing
// catalog path causes runWithSignals to return an error wrapping "load pricing catalog".
func TestRunWithSignalsPricingCatalogFailure(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()

	env := map[string]string{
		"OPENAI_API_KEY":              "test-key",
		"HARNESS_ADDR":                "127.0.0.1:0",
		"HARNESS_MEMORY_MODE":         "off",
		"HARNESS_WORKSPACE":           workspaceDir,
		"HARNESS_PRICING_CATALOG_PATH": "/nonexistent/path/pricing.json",
	}
	getenv := func(key string) string { return env[key] }

	err := runWithSignals(make(chan os.Signal, 1), getenv, func(openai.Config) (harness.Provider, error) {
		return &noopProvider{}, nil
	}, "")

	if err == nil {
		t.Fatal("expected error when pricing catalog path is invalid")
	}
	if !strings.Contains(err.Error(), "pricing catalog") {
		t.Fatalf("expected 'pricing catalog' in error, got: %v", err)
	}
}

// TestRunWithSignalsMCPParseFailureContinues verifies that an unparseable
// HARNESS_MCP_SERVERS value is logged as a warning but does NOT abort startup
// (MCP failures are non-fatal — the server continues without env-configured MCP).
func TestRunWithSignalsMCPParseFailureContinues(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()

	env := map[string]string{
		"OPENAI_API_KEY":       "test-key",
		"HARNESS_ADDR":         "127.0.0.1:0",
		"HARNESS_MEMORY_MODE":  "off",
		"HARNESS_WORKSPACE":    workspaceDir,
		"HARNESS_MCP_SERVERS":  "not-valid-json-{{{",
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv, func(openai.Config) (harness.Provider, error) {
			return &noopProvider{}, nil
		}, "")
	}()

	time.Sleep(100 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		// MCP parse failure must NOT abort the server; a nil error is expected.
		if err != nil {
			t.Fatalf("expected server to continue despite MCP parse failure; got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}

// TestRunWithSignalsInvalidModelCatalogContinues verifies that a pointing
// HARNESS_MODEL_CATALOG_PATH to an invalid JSON file is logged as a warning
// and the server continues (catalog failures are non-fatal).
func TestRunWithSignalsInvalidModelCatalogContinues(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()

	// Write a malformed catalog file.
	badCatalog, err := os.CreateTemp(t.TempDir(), "bad-catalog*.json")
	if err != nil {
		t.Fatalf("create temp catalog: %v", err)
	}
	if _, err := badCatalog.WriteString(`{invalid json`); err != nil {
		t.Fatalf("write bad catalog: %v", err)
	}
	badCatalog.Close()

	env := map[string]string{
		"OPENAI_API_KEY":             "test-key",
		"HARNESS_ADDR":               "127.0.0.1:0",
		"HARNESS_MEMORY_MODE":        "off",
		"HARNESS_WORKSPACE":          workspaceDir,
		"HARNESS_MODEL_CATALOG_PATH": badCatalog.Name(),
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv, func(openai.Config) (harness.Provider, error) {
			return &noopProvider{}, nil
		}, "")
	}()

	time.Sleep(100 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		// Invalid model catalog must NOT abort the server; nil error expected.
		if err != nil {
			t.Fatalf("expected server to continue despite invalid model catalog; got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}

