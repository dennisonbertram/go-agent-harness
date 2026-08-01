package cron

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

const (
	testIngressKey    = "cron_ingress_test_secret"
	testIngressTenant = "tenant-a"
)

func TestCronServerAuthenticatedTenantCRUDOverHTTP(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "cron.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	clock := newMockClock(time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &ShellExecutor{}, clock, SchedulerConfig{MaxConcurrent: 1})
	handler := NewServer(store, scheduler, clock, IngressAuthConfig{
		APIKey:   testIngressKey,
		TenantID: testIngressTenant,
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	response := doIngressRequest(t, http.MethodGet, server.URL+"/healthz", "", "")
	assertIngressStatus(t, response, http.StatusOK)
	response = doIngressRequest(t, http.MethodGet, server.URL+"/readyz", "", "")
	assertIngressStatus(t, response, http.StatusUnauthorized)
	response = doIngressRequest(t, http.MethodGet, server.URL+"/readyz", testIngressKey, "")
	assertIngressStatus(t, response, http.StatusOK)
	response = doIngressRequest(t, http.MethodGet, server.URL+"/v1/jobs", "", "")
	assertIngressStatus(t, response, http.StatusUnauthorized)
	response = doIngressRequest(t, http.MethodGet, server.URL+"/v1/jobs", "wrong-secret", "")
	assertIngressStatus(t, response, http.StatusUnauthorized)

	response = doIngressRequest(t, http.MethodPost, server.URL+"/v1/jobs", testIngressKey,
		`{"tenant_id":"tenant-b","name":"spoofed","schedule":"*/5 * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo no\"}"}`)
	assertIngressStatus(t, response, http.StatusForbidden)

	client := NewClient(server.URL, WithAPIKey(testIngressKey))
	job, err := client.CreateJob(context.Background(), CreateJobRequest{
		Name:       "owned-job",
		Schedule:   "*/5 * * * *",
		ExecType:   ExecTypeShell,
		ExecConfig: `{"command":"echo ok"}`,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.TenantID != testIngressTenant {
		t.Fatalf("created tenant = %q, want %q", job.TenantID, testIngressTenant)
	}

	other := testJob("other-tenant-job")
	other.TenantID = "tenant-b"
	other, err = store.CreateJob(context.Background(), other)
	if err != nil {
		t.Fatalf("create other tenant job: %v", err)
	}

	jobs, err := client.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != job.ID {
		t.Fatalf("listed jobs = %#v, want only %q", jobs, job.ID)
	}

	got, err := client.GetJob(context.Background(), job.ID)
	if err != nil || got.ID != job.ID {
		t.Fatalf("GetJob = %#v, %v", got, err)
	}
	if _, err := client.GetJob(context.Background(), other.ID); !IsJobNotFound(err) {
		t.Fatalf("cross-tenant GetJob error = %v, want not found", err)
	}

	paused := StatusPaused
	updated, err := client.UpdateJob(context.Background(), job.ID, UpdateJobRequest{Status: &paused})
	if err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	if updated.Status != StatusPaused || updated.TenantID != testIngressTenant {
		t.Fatalf("updated job = %#v", updated)
	}
	response = doIngressRequest(t, http.MethodPatch, server.URL+"/v1/jobs/"+other.ID, testIngressKey, `{"status":"paused"}`)
	assertIngressStatus(t, response, http.StatusNotFound)

	execution := Execution{
		ID:        "execution-owned",
		JobID:     job.ID,
		StartedAt: clock.Now(),
		Status:    ExecStatusSuccess,
	}
	if _, err := store.CreateExecution(context.Background(), execution); err != nil {
		t.Fatalf("CreateExecution: %v", err)
	}
	executions, err := client.ListExecutions(context.Background(), job.ID, 10, 0)
	if err != nil || len(executions) != 1 || executions[0].ID != execution.ID {
		t.Fatalf("ListExecutions = %#v, %v", executions, err)
	}
	response = doIngressRequest(t, http.MethodGet, server.URL+"/v1/jobs/"+other.ID+"/history", testIngressKey, "")
	assertIngressStatus(t, response, http.StatusNotFound)

	response = doIngressRequest(t, http.MethodDelete, server.URL+"/v1/jobs/"+other.ID, testIngressKey, "")
	assertIngressStatus(t, response, http.StatusNotFound)
	if _, err := store.GetJob(context.Background(), other.ID); err != nil {
		t.Fatalf("cross-tenant delete changed job: %v", err)
	}
	if err := client.DeleteJob(context.Background(), job.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	if _, err := client.GetJob(context.Background(), job.ID); !IsJobNotFound(err) {
		t.Fatalf("deleted GetJob error = %v, want not found", err)
	}
}

func TestIngressAuthConfigValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  IngressAuthConfig
	}{
		{name: "missing key", cfg: IngressAuthConfig{TenantID: "tenant-a"}},
		{name: "missing tenant", cfg: IngressAuthConfig{APIKey: "secret"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Fatal("Validate returned nil")
			}
		})
	}
	if err := (IngressAuthConfig{APIKey: "secret", TenantID: "tenant-a"}).Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
}

func TestCronServerDoesNotExposeLegacyShellJobWithoutDurableClaimSupport(t *testing.T) {
	legacy := testJob("legacy-without-claim-store")
	legacy.TenantID = ""
	store := &mockStore{
		GetJobFunc:   func(context.Context, string) (Job, error) { return legacy, nil },
		ListJobsFunc: func(context.Context) ([]Job, error) { return []Job{legacy}, nil },
	}
	clock := newMockClock(time.Now().UTC())
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{MaxConcurrent: 1})
	handler := NewServer(store, scheduler, clock, testIngressAuthConfig())

	request := httptest.NewRequest(http.MethodGet, "/v1/jobs/"+legacy.ID, nil)
	request.Header.Set("Authorization", "Bearer "+testIngressKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("GET status = %d, want 404; body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	request.Header.Set("Authorization", "Bearer "+testIngressKey)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("LIST status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	var listed struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.NewDecoder(response.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Jobs) != 0 {
		t.Fatalf("listed unclaimed jobs = %#v", listed.Jobs)
	}
}

func doIngressRequest(t *testing.T, method, url, apiKey, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return response
}

func assertIngressStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != want {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, want, body)
	}
	if response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", response.Header.Get("Content-Type"))
	}
	if len(body) == 0 {
		t.Fatal("expected bounded JSON response body")
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
}
