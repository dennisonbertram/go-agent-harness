package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/store"

	"golang.org/x/crypto/bcrypt"
)

// mockCronClient is a simple in-memory mock for CronClient.
type mockCronClient struct {
	mu            sync.Mutex
	jobs          map[string]tools.CronJob
	executions    map[string][]tools.CronExecution
	listExecCalls []cronExecutionListCall
	seq           int
	fail          bool // if true, all operations return an error
	createErr     error
	updateErr     error
	listExecErr   error
}

type cronExecutionListCall struct {
	jobID  string
	limit  int
	offset int
}

func newMockCronClient() *mockCronClient {
	return &mockCronClient{
		jobs:       make(map[string]tools.CronJob),
		executions: make(map[string][]tools.CronExecution),
	}
}

func (m *mockCronClient) nextID() string {
	m.seq++
	return fmt.Sprintf("job-%d", m.seq)
}

func (m *mockCronClient) CreateJob(_ context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return tools.CronJob{}, fmt.Errorf("mock error")
	}
	if m.createErr != nil {
		return tools.CronJob{}, m.createErr
	}
	now := time.Now().UTC()
	job := tools.CronJob{
		ID:             m.nextID(),
		TenantID:       req.TenantID,
		ConversationID: req.ConversationID,
		Name:           req.Name,
		Schedule:       req.Schedule,
		ExecType:       req.ExecType,
		ExecConfig:     req.ExecConfig,
		Status:         "active",
		TimeoutSec:     req.TimeoutSec,
		Tags:           req.Tags,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	m.jobs[job.ID] = job
	return job, nil
}

func (m *mockCronClient) ListJobs(_ context.Context) ([]tools.CronJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return nil, fmt.Errorf("mock error")
	}
	jobs := make([]tools.CronJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (m *mockCronClient) GetJob(_ context.Context, id string) (tools.CronJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return tools.CronJob{}, fmt.Errorf("mock error")
	}
	j, ok := m.jobs[id]
	if !ok {
		return tools.CronJob{}, tools.ErrCronJobNotFound
	}
	return j, nil
}

func (m *mockCronClient) UpdateJob(_ context.Context, id string, req tools.CronUpdateJobRequest) (tools.CronJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return tools.CronJob{}, fmt.Errorf("mock error")
	}
	if m.updateErr != nil {
		return tools.CronJob{}, m.updateErr
	}
	j, ok := m.jobs[id]
	if !ok {
		return tools.CronJob{}, tools.ErrCronJobNotFound
	}
	if req.ExpectedUpdatedAt != nil && !j.UpdatedAt.Equal(*req.ExpectedUpdatedAt) {
		return tools.CronJob{}, tools.ErrCronJobConflict
	}
	if req.Status != nil {
		j.Status = *req.Status
	}
	if req.Schedule != nil {
		j.Schedule = *req.Schedule
	}
	if req.ExecConfig != nil {
		j.ExecConfig = *req.ExecConfig
	}
	if req.TimeoutSec != nil {
		j.TimeoutSec = *req.TimeoutSec
	}
	if req.Tags != nil {
		j.Tags = *req.Tags
	}
	j.UpdatedAt = time.Now().UTC()
	m.jobs[id] = j
	return j, nil
}

func (m *mockCronClient) DeleteJob(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return fmt.Errorf("mock error")
	}
	if _, ok := m.jobs[id]; !ok {
		return tools.ErrCronJobNotFound
	}
	delete(m.jobs, id)
	return nil
}

func (m *mockCronClient) DeleteJobCAS(_ context.Context, id string, expectedUpdatedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return fmt.Errorf("mock error")
	}
	job, ok := m.jobs[id]
	if !ok {
		return tools.ErrCronJobNotFound
	}
	if !job.UpdatedAt.Equal(expectedUpdatedAt) {
		return tools.ErrCronJobConflict
	}
	delete(m.jobs, id)
	return nil
}

func (m *mockCronClient) ListExecutions(_ context.Context, jobID string, limit, offset int) ([]tools.CronExecution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listExecErr != nil {
		return nil, m.listExecErr
	}
	if m.fail {
		return nil, fmt.Errorf("mock error")
	}
	m.listExecCalls = append(m.listExecCalls, cronExecutionListCall{jobID: jobID, limit: limit, offset: offset})
	return append([]tools.CronExecution(nil), m.executions[jobID]...), nil
}

func (m *mockCronClient) Health(_ context.Context) error {
	if m.fail {
		return fmt.Errorf("mock error")
	}
	return nil
}

// testRunnerForCron builds a minimal runner suitable for cron HTTP handler tests.
func testRunnerForCron(t *testing.T) *harness.Runner {
	t.Helper()
	registry := harness.NewRegistry()
	return harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		registry,
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
		},
	)
}

// cronTestServer builds a test server with a mock cron client.
func cronTestServer(t *testing.T, client CronClient) *httptest.Server {
	t.Helper()
	runner := testRunnerForCron(t)
	s := NewWithCron(runner, nil, client)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func cronTenantTestServer(t *testing.T, client CronClient) (*httptest.Server, string, string, string, string, string) {
	t.Helper()

	ms := store.NewMemoryStore()
	tenantA := "tenant-cron-alpha"
	tokenA, keyA := cronTestAPIKey(t, tenantA, "cron A", []string{store.ScopeRunsRead, store.ScopeRunsWrite})
	if err := ms.CreateAPIKey(context.Background(), keyA); err != nil {
		t.Fatalf("CreateAPIKey A: %v", err)
	}
	tenantB := "tenant-cron-bravo"
	tokenB, keyB := cronTestAPIKey(t, tenantB, "cron B", []string{store.ScopeRunsRead, store.ScopeRunsWrite})
	if err := ms.CreateAPIKey(context.Background(), keyB); err != nil {
		t.Fatalf("CreateAPIKey B: %v", err)
	}
	tokenNoRunScope, noRunScopeKey := cronTestAPIKey(t, tenantA, "cron no-run scope", nil)
	if err := ms.CreateAPIKey(context.Background(), noRunScopeKey); err != nil {
		t.Fatalf("CreateAPIKey no-run-scope: %v", err)
	}

	h := NewWithOptions(ServerOptions{
		Store:      ms,
		Runner:     testRunnerForCron(t),
		CronClient: client,
	})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts, tokenA, tenantA, tokenB, tenantB, tokenNoRunScope
}

func cronTestAPIKey(t *testing.T, tenantID, name string, scopes []string) (string, store.APIKey) {
	t.Helper()

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand.Read raw: %v", err)
	}
	suffix := base64.RawURLEncoding.EncodeToString(raw)
	rawToken := "harness_sk_" + suffix

	hash, err := bcrypt.GenerateFromPassword([]byte(rawToken), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}

	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		t.Fatalf("rand.Read id: %v", err)
	}

	prefix := suffix
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}

	return rawToken, store.APIKey{
		ID:        base64.RawURLEncoding.EncodeToString(id),
		KeyHash:   string(hash),
		KeyPrefix: prefix,
		TenantID:  tenantID,
		Name:      name,
		Scopes:    append([]string(nil), scopes...),
		CreatedAt: time.Now().UTC(),
	}
}

func doCronJSON(t *testing.T, method, url, token, body string) (*http.Response, []byte) {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	data, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, data
}

func requireCronStatus(t *testing.T, res *http.Response, body []byte, want int) {
	t.Helper()
	if res.StatusCode != want {
		t.Fatalf("expected status %d, got %d: %s", want, res.StatusCode, string(body))
	}
}

func requireOnlyCronJob(t *testing.T, body []byte, wantID, wantTenant string) {
	t.Helper()
	var resp struct {
		Jobs []tools.CronJob `json:"jobs"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode jobs: %v", err)
	}
	if len(resp.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d: %s", len(resp.Jobs), string(body))
	}
	if resp.Jobs[0].ID != wantID {
		t.Fatalf("expected job %q, got %q", wantID, resp.Jobs[0].ID)
	}
	if resp.Jobs[0].TenantID != wantTenant {
		t.Fatalf("expected tenant %q, got %q", wantTenant, resp.Jobs[0].TenantID)
	}
}

// TestCronExecutionHistory_ReturnsOwnedPaginatedExecutions verifies the
// public history API exposes an owned job's durable execution records and
// forwards pagination to the configured cron adapter.
func TestCronExecutionHistory_ReturnsOwnedPaginatedExecutions(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	job, err := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{
		Name:     "history",
		Schedule: "* * * * *",
		ExecType: "agent",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	mock.executions[job.ID] = []tools.CronExecution{{
		ID:            "exec-1",
		JobID:         job.ID,
		Status:        "succeeded",
		RunID:         "run-1",
		OutputSummary: "deployment passed",
	}}
	ts := cronTestServer(t, mock)

	res, body := doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs/"+job.ID+"/executions?limit=2&offset=3", "", "")
	requireCronStatus(t, res, body, http.StatusOK)

	var got struct {
		Executions []tools.CronExecution `json:"executions"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode execution response: %v", err)
	}
	if len(got.Executions) != 1 || got.Executions[0].RunID != "run-1" || got.Executions[0].OutputSummary != "deployment passed" {
		t.Fatalf("executions = %#v, want durable execution", got.Executions)
	}
	mock.mu.Lock()
	calls := append([]cronExecutionListCall(nil), mock.listExecCalls...)
	mock.mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("ListExecutions calls = %d, want 1", len(calls))
	}
	if call := calls[0]; call.jobID != job.ID || call.limit != 2 || call.offset != 3 {
		t.Fatalf("ListExecutions(%q, %d, %d), want (%q, 2, 3)", call.jobID, call.limit, call.offset, job.ID)
	}
}

func TestCronExecutionHistory_NormalizesPagination(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	job, err := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{Name: "page", Schedule: "* * * * *", ExecType: "agent"})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	ts := cronTestServer(t, mock)

	res, body := doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs/"+job.ID+"/executions?limit=999&offset=-4", "", "")
	requireCronStatus(t, res, body, http.StatusOK)
	mock.mu.Lock()
	calls := append([]cronExecutionListCall(nil), mock.listExecCalls...)
	mock.mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("ListExecutions calls = %d, want 1", len(calls))
	}
	if call := calls[0]; call.limit != 100 || call.offset != 0 {
		t.Fatalf("ListExecutions pagination = (%d, %d), want (100, 0)", call.limit, call.offset)
	}
}

func TestCronExecutionHistory_RequiresReadScopeAndHidesForeignJobs(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	ts, tokenA, tenantA, tokenB, _, tokenNoRunScope := cronTenantTestServer(t, mock)
	res, body := doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs", tokenA, `{"name":"tenant-a-history","schedule":"* * * * *","execution_type":"agent"}`)
	requireCronStatus(t, res, body, http.StatusCreated)
	var job tools.CronJob
	if err := json.Unmarshal(body, &job); err != nil {
		t.Fatalf("decode created job: %v", err)
	}
	if job.TenantID != tenantA {
		t.Fatalf("created tenant = %q, want %q", job.TenantID, tenantA)
	}
	mock.mu.Lock()
	mock.executions[job.ID] = []tools.CronExecution{{ID: "tenant-a-exec", JobID: job.ID, Status: "succeeded"}}
	mock.mu.Unlock()

	res, body = doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs/"+job.ID+"/executions", "", "")
	requireCronStatus(t, res, body, http.StatusUnauthorized)

	res, body = doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs/"+job.ID+"/executions", tokenNoRunScope, "")
	requireCronStatus(t, res, body, http.StatusForbidden)
	mock.mu.Lock()
	listCalls := len(mock.listExecCalls)
	mock.mu.Unlock()
	if listCalls != 0 {
		t.Fatalf("ListExecutions calls after unauthorized requests = %d, want 0", listCalls)
	}

	res, body = doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs/"+job.ID+"/executions", tokenB, "")
	requireCronStatus(t, res, body, http.StatusNotFound)

	res, body = doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs/does-not-exist/executions", tokenA, "")
	requireCronStatus(t, res, body, http.StatusNotFound)
}

func TestCronExecutionHistory_ReportsAdapterFailure(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	job, err := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{Name: "broken-history", Schedule: "* * * * *", ExecType: "agent"})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	mock.listExecErr = fmt.Errorf("history unavailable")
	ts := cronTestServer(t, mock)

	res, body := doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs/"+job.ID+"/executions", "", "")
	requireCronStatus(t, res, body, http.StatusInternalServerError)
	if !strings.Contains(string(body), "history unavailable") {
		t.Fatalf("error body = %s, want adapter error", body)
	}
}

// TestCronListJobs_Returns200WithList verifies GET /v1/cron/jobs returns a list.
func TestCronListJobs_Returns200WithList(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	// Pre-seed a job.
	_, _ = mock.CreateJob(context.Background(), tools.CronCreateJobRequest{
		Name:     "test-job",
		Schedule: "* * * * *",
		ExecType: "shell",
	})

	ts := cronTestServer(t, mock)

	res, err := http.Get(ts.URL + "/v1/cron/jobs")
	if err != nil {
		t.Fatalf("GET /v1/cron/jobs: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var resp struct {
		Jobs []tools.CronJob `json:"jobs"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(resp.Jobs))
	}
}

// TestCronCreateJob_Returns201 verifies POST /v1/cron/jobs creates and returns 201.
func TestCronCreateJob_Returns201(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	ts := cronTestServer(t, mock)

	body := `{"name":"my-job","schedule":"0 * * * *","execution_type":"shell","execution_config":"{\"command\":\"echo hi\"}"}`
	res, err := http.Post(ts.URL+"/v1/cron/jobs", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /v1/cron/jobs: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 201, got %d: %s", res.StatusCode, string(b))
	}

	var job tools.CronJob
	if err := json.NewDecoder(res.Body).Decode(&job); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	if job.ID == "" {
		t.Error("expected non-empty job ID")
	}
	if job.Name != "my-job" {
		t.Errorf("expected name my-job, got %q", job.Name)
	}
}

// TestCronCreateJob_ValidatesRequiredFields verifies missing fields return 400.
func TestCronCreateJob_ValidatesRequiredFields(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	ts := cronTestServer(t, mock)

	// Missing schedule.
	body := `{"name":"my-job"}`
	res, err := http.Post(ts.URL+"/v1/cron/jobs", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", res.StatusCode)
	}
}

func TestCronCreateAndUpdateJob_TypedValidationErrorsReturn400(t *testing.T) {
	t.Parallel()

	validation := errors.Join(tools.ErrCronJobValidation, errors.New("invalid schedule: expected five fields"))
	mock := newMockCronClient()
	mock.fail = false
	ts := cronTestServer(t, mock)

	// A client-invalid create must remain a 400 with the stable error code,
	// rather than being presented as a scheduler/dependency outage.
	mock.createErr = validation
	res, body := doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs", "", `{"name":"bad","schedule":"bad","execution_type":"shell","execution_config":"{\"command\":\"echo ok\"}"}`)
	requireCronStatus(t, res, body, http.StatusBadRequest)
	if !strings.Contains(string(body), `"code":"validation_error"`) || !strings.Contains(string(body), "invalid schedule") {
		t.Fatalf("create validation response = %s", body)
	}
	if len(mock.jobs) != 0 {
		t.Fatalf("invalid create persisted %d jobs", len(mock.jobs))
	}

	mock.createErr = nil
	job, err := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{Name: "valid", Schedule: "* * * * *", ExecType: "shell"})
	if err != nil {
		t.Fatalf("seed job: %v", err)
	}
	mock.updateErr = validation
	res, body = doCronJSON(t, http.MethodPatch, ts.URL+"/v1/cron/jobs/"+job.ID, "", `{"timeout_seconds":0}`)
	requireCronStatus(t, res, body, http.StatusBadRequest)
	if !strings.Contains(string(body), `"code":"validation_error"`) || !strings.Contains(string(body), "invalid schedule") {
		t.Fatalf("update validation response = %s", body)
	}
}

func TestCronCreateJob_ExplicitNonPositiveTimeoutReturnsValidationError(t *testing.T) {
	t.Parallel()
	mock := newMockCronClient()
	ts := cronTestServer(t, mock)

	for _, body := range []string{
		`{"name":"zero","schedule":"* * * * *","execution_type":"shell","timeout_seconds":0}`,
		`{"name":"negative","schedule":"* * * * *","execution_type":"shell","timeout_seconds":-1}`,
	} {
		res, response := doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs", "", body)
		requireCronStatus(t, res, response, http.StatusBadRequest)
		if !strings.Contains(string(response), `"code":"validation_error"`) || !strings.Contains(string(response), "timeout_seconds must be positive") {
			t.Fatalf("response = %s", response)
		}
	}
	if len(mock.jobs) != 0 {
		t.Fatalf("invalid timeout creates persisted %d jobs", len(mock.jobs))
	}
}

func TestCronJobError_PreservesNotFoundConflictAndDependencyFailure(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{name: "not found", err: tools.ErrCronJobNotFound, want: http.StatusNotFound},
		{name: "conflict", err: tools.ErrCronJobConflict, want: http.StatusConflict},
		{name: "dependency", err: errors.New("scheduler unavailable"), want: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeCronJobError(rr, tc.err)
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rr.Code, tc.want, rr.Body.String())
			}
		})
	}
}

// TestCronGetJob_Returns200 verifies GET /v1/cron/jobs/{id} returns a specific job.
func TestCronGetJob_Returns200(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	job, _ := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{
		Name:     "find-me",
		Schedule: "* * * * *",
		ExecType: "shell",
	})

	ts := cronTestServer(t, mock)

	res, err := http.Get(ts.URL + "/v1/cron/jobs/" + job.ID)
	if err != nil {
		t.Fatalf("GET /v1/cron/jobs/%s: %v", job.ID, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var got tools.CronJob
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != job.ID {
		t.Errorf("expected job ID %q, got %q", job.ID, got.ID)
	}
	if got.Name != "find-me" {
		t.Errorf("expected name find-me, got %q", got.Name)
	}
}

// TestCronGetJob_Returns404ForUnknown verifies 404 for unknown job.
func TestCronGetJob_Returns404ForUnknown(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	ts := cronTestServer(t, mock)

	res, err := http.Get(ts.URL + "/v1/cron/jobs/does-not-exist")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", res.StatusCode)
	}
}

func TestCronGetJob_Returns500ForBackendError(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	mock.fail = true
	ts := cronTestServer(t, mock)

	res, err := http.Get(ts.URL + "/v1/cron/jobs/job-1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusInternalServerError {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 500, got %d: %s", res.StatusCode, body)
	}
}

func TestCronJobs_AreTenantIsolated(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	ts, tokenA, tenantA, tokenB, tenantB, _ := cronTenantTestServer(t, mock)

	res, body := doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs", tokenA, `{"name":"tenant-a","schedule":"* * * * *","execution_type":"shell"}`)
	requireCronStatus(t, res, body, http.StatusCreated)
	var jobA tools.CronJob
	if err := json.Unmarshal(body, &jobA); err != nil {
		t.Fatalf("decode tenant A job: %v", err)
	}
	if jobA.TenantID != tenantA {
		t.Fatalf("expected tenant A job to be stamped %q, got %q", tenantA, jobA.TenantID)
	}

	res, body = doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs", tokenB, `{"name":"tenant-b","schedule":"0 * * * *","execution_type":"shell"}`)
	requireCronStatus(t, res, body, http.StatusCreated)
	var jobB tools.CronJob
	if err := json.Unmarshal(body, &jobB); err != nil {
		t.Fatalf("decode tenant B job: %v", err)
	}
	if jobB.TenantID != tenantB {
		t.Fatalf("expected tenant B job to be stamped %q, got %q", tenantB, jobB.TenantID)
	}

	res, body = doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs", tokenA, "")
	requireCronStatus(t, res, body, http.StatusOK)
	requireOnlyCronJob(t, body, jobA.ID, tenantA)

	res, body = doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs", tokenB, "")
	requireCronStatus(t, res, body, http.StatusOK)
	requireOnlyCronJob(t, body, jobB.ID, tenantB)

	res, body = doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs/"+jobA.ID, tokenB, "")
	requireCronStatus(t, res, body, http.StatusNotFound)

	res, body = doCronJSON(t, http.MethodDelete, ts.URL+"/v1/cron/jobs/"+jobA.ID, tokenB, "")
	requireCronStatus(t, res, body, http.StatusNotFound)

	res, body = doCronJSON(t, http.MethodGet, ts.URL+"/v1/cron/jobs/"+jobA.ID, tokenA, "")
	requireCronStatus(t, res, body, http.StatusOK)

	res, body = doCronJSON(t, http.MethodDelete, ts.URL+"/v1/cron/jobs/"+jobA.ID, tokenA, "")
	requireCronStatus(t, res, body, http.StatusNoContent)
}

// TestCronUpdateJob_Returns200 verifies PATCH /v1/cron/jobs/{id} updates job.
func TestCronUpdateJob_Returns200(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	job, _ := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{
		Name:     "patch-me",
		Schedule: "* * * * *",
		ExecType: "shell",
	})

	ts := cronTestServer(t, mock)

	newSched := `{"schedule":"0 12 * * *"}`
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/v1/cron/jobs/"+job.ID, bytes.NewBufferString(newSched))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var updated tools.CronJob
	if err := json.NewDecoder(res.Body).Decode(&updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Schedule != "0 12 * * *" {
		t.Errorf("expected updated schedule, got %q", updated.Schedule)
	}
}

// TestCronDeleteJob_Returns204 verifies DELETE /v1/cron/jobs/{id} returns 204.
func TestCronDeleteJob_Returns204(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	job, _ := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{
		Name:     "delete-me",
		Schedule: "* * * * *",
		ExecType: "shell",
	})

	ts := cronTestServer(t, mock)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/v1/cron/jobs/"+job.ID, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 204, got %d: %s", res.StatusCode, string(body))
	}

	// Verify it's gone.
	getRes, _ := http.Get(ts.URL + "/v1/cron/jobs/" + job.ID)
	if getRes != nil {
		defer getRes.Body.Close()
		if getRes.StatusCode != http.StatusNotFound {
			t.Errorf("expected job to be deleted (404), got %d", getRes.StatusCode)
		}
	}
}

// TestCronPauseJob_Returns200 verifies POST /v1/cron/jobs/{id}/pause.
func TestCronPauseJob_Returns200(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	job, _ := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{
		Name:     "pause-me",
		Schedule: "* * * * *",
		ExecType: "shell",
	})

	ts := cronTestServer(t, mock)

	res, err := http.Post(ts.URL+"/v1/cron/jobs/"+job.ID+"/pause", "application/json", nil)
	if err != nil {
		t.Fatalf("POST pause: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var updated tools.CronJob
	if err := json.NewDecoder(res.Body).Decode(&updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Status != "paused" {
		t.Errorf("expected status paused, got %q", updated.Status)
	}
}

// TestCronResumeJob_Returns200 verifies POST /v1/cron/jobs/{id}/resume.
func TestCronResumeJob_Returns200(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	job, _ := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{
		Name:     "resume-me",
		Schedule: "* * * * *",
		ExecType: "shell",
	})
	// Pause it first via mock.
	paused := "paused"
	_, _ = mock.UpdateJob(context.Background(), job.ID, tools.CronUpdateJobRequest{Status: &paused})

	ts := cronTestServer(t, mock)

	res, err := http.Post(ts.URL+"/v1/cron/jobs/"+job.ID+"/resume", "application/json", nil)
	if err != nil {
		t.Fatalf("POST resume: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var updated tools.CronJob
	if err := json.NewDecoder(res.Body).Decode(&updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Status != "active" {
		t.Errorf("expected status active, got %q", updated.Status)
	}
}

// The Activity page can render a cron row just before another client changes
// it. Optional expected_updated_at is a CAS fence: stale actions return 409
// and must not mutate the current job.
func TestCronLifecycleActions_OptionalExpectedUpdatedAtPreventsStaleMutation(t *testing.T) {
	mock := newMockCronClient()
	job, err := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{
		Name: "watch deployment", Schedule: "*/5 * * * *",
	})
	if err != nil {
		t.Fatalf("seed cron job: %v", err)
	}
	ts := cronTestServer(t, mock)
	stale := job.UpdatedAt.Add(-time.Second).Format(time.RFC3339Nano)
	current := job.UpdatedAt.Format(time.RFC3339Nano)

	res, _ := doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs/"+job.ID+"/pause", "", `{"expected_updated_at":"`+stale+`"}`)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("stale pause status = %d, want 409", res.StatusCode)
	}
	got, _ := mock.GetJob(context.Background(), job.ID)
	if got.Status != "active" {
		t.Fatalf("stale pause mutated status = %q, want active", got.Status)
	}

	res, _ = doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs/"+job.ID+"/pause", "", `{"expected_updated_at":"`+current+`"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("current pause status = %d, want 200", res.StatusCode)
	}
	got, _ = mock.GetJob(context.Background(), job.ID)
	if got.Status != "paused" {
		t.Fatalf("current pause status = %q, want paused", got.Status)
	}

	res, _ = doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs/"+job.ID+"/resume", "", `{"expected_updated_at":"`+current+`"}`)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("stale resume status = %d, want 409", res.StatusCode)
	}
	if got, _ = mock.GetJob(context.Background(), job.ID); got.Status != "paused" {
		t.Fatalf("stale resume mutated status = %q, want paused", got.Status)
	}

	current = got.UpdatedAt.Format(time.RFC3339Nano)
	res, _ = doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs/"+job.ID+"/resume", "", `{"expected_updated_at":"`+current+`"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("current resume status = %d, want 200", res.StatusCode)
	}
	got, _ = mock.GetJob(context.Background(), job.ID)
	if got.Status != "active" {
		t.Fatalf("current resume status = %q, want active", got.Status)
	}

	res, _ = doCronJSON(t, http.MethodDelete, ts.URL+"/v1/cron/jobs/"+job.ID, "", `{"expected_updated_at":"`+current+`"}`)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("stale delete status = %d, want 409", res.StatusCode)
	}
	if _, err := mock.GetJob(context.Background(), job.ID); err != nil {
		t.Fatalf("stale delete removed job: %v", err)
	}

	current = got.UpdatedAt.Format(time.RFC3339Nano)
	res, _ = doCronJSON(t, http.MethodDelete, ts.URL+"/v1/cron/jobs/"+job.ID, "", `{"expected_updated_at":"`+current+`"}`)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("current delete status = %d, want 204", res.StatusCode)
	}
	if _, err := mock.GetJob(context.Background(), job.ID); !errors.Is(err, tools.ErrCronJobNotFound) {
		t.Fatalf("current delete error = %v, want not found", err)
	}
}

// Task versions are opaque server strings. A task listing with nanosecond
// precision must provide the exact token callers need for the action CAS
// fence; a client-side truncated timestamp is stale even in the same second.
func TestCronLifecycleActions_TaskVersionPreservesNanoseconds(t *testing.T) {
	mock := newMockCronClient()
	job, err := mock.CreateJob(context.Background(), tools.CronCreateJobRequest{
		Name: "watch deployment", Schedule: "*/5 * * * *",
	})
	if err != nil {
		t.Fatalf("seed cron job: %v", err)
	}

	updatedAt := time.Date(2026, time.August, 4, 12, 0, 0, 123456789, time.UTC)
	mock.mu.Lock()
	job = mock.jobs[job.ID]
	job.UpdatedAt = updatedAt
	mock.jobs[job.ID] = job
	mock.mu.Unlock()

	ts := cronTestServer(t, mock)
	res, body := doCronJSON(t, http.MethodGet, ts.URL+"/v1/tasks", "", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list tasks status = %d, want 200: %s", res.StatusCode, string(body))
	}
	var listed struct {
		Tasks []struct {
			ID        string `json:"id"`
			UpdatedAt string `json:"updated_at"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("decode task list: %v", err)
	}
	var token string
	for _, task := range listed.Tasks {
		if task.ID == job.ID {
			token = task.UpdatedAt
			break
		}
	}
	want := updatedAt.Format(time.RFC3339Nano)
	if token != want {
		t.Fatalf("listed task version = %q, want %q", token, want)
	}

	truncated := "2026-08-04T12:00:00.123Z"
	res, _ = doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs/"+job.ID+"/pause", "", `{"expected_updated_at":"`+truncated+`"}`)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("truncated pause status = %d, want 409", res.StatusCode)
	}
	if got, _ := mock.GetJob(context.Background(), job.ID); got.Status != "active" {
		t.Fatalf("truncated pause mutated status = %q, want active", got.Status)
	}

	res, _ = doCronJSON(t, http.MethodPost, ts.URL+"/v1/cron/jobs/"+job.ID+"/pause", "", `{"expected_updated_at":"`+token+`"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("exact pause status = %d, want 200", res.StatusCode)
	}
	if got, _ := mock.GetJob(context.Background(), job.ID); got.Status != "paused" {
		t.Fatalf("exact pause status = %q, want paused", got.Status)
	}
}

// TestCronEndpoints_Return501WhenNotConfigured verifies all cron endpoints return 501 when cronClient is nil.
func TestCronEndpoints_Return501WhenNotConfigured(t *testing.T) {
	t.Parallel()

	runner := testRunnerForCron(t)
	// Use NewWithCron with nil cronClient — all cron endpoints should 501.
	s := NewWithCron(runner, nil, nil)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/v1/cron/jobs", ""},
		{http.MethodPost, "/v1/cron/jobs", `{"name":"x","schedule":"* * * * *"}`},
		{http.MethodGet, "/v1/cron/jobs/123", ""},
		{http.MethodGet, "/v1/cron/jobs/123/executions", ""},
		{http.MethodPatch, "/v1/cron/jobs/123", `{}`},
		{http.MethodDelete, "/v1/cron/jobs/123", ""},
		{http.MethodPost, "/v1/cron/jobs/123/pause", ""},
		{http.MethodPost, "/v1/cron/jobs/123/resume", ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			// NOTE: subtests must NOT be parallel here: parent's t.Cleanup
			// runs after all subtests complete, keeping the server alive.
			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = bytes.NewBufferString(tc.body)
			}
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, bodyReader)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusNotImplemented {
				body, _ := io.ReadAll(res.Body)
				t.Errorf("expected 501, got %d: %s", res.StatusCode, string(body))
			}
		})
	}
}

// TestCronJobsRoot_MethodNotAllowed verifies unsupported methods return 405.
func TestCronJobsRoot_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	ts := cronTestServer(t, mock)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/v1/cron/jobs", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /v1/cron/jobs: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", res.StatusCode)
	}
}

// TestCronCreateJob_InvalidJSON returns 400 for malformed JSON.
func TestCronCreateJob_InvalidJSON(t *testing.T) {
	t.Parallel()

	mock := newMockCronClient()
	ts := cronTestServer(t, mock)

	res, err := http.Post(ts.URL+"/v1/cron/jobs", "application/json", bytes.NewBufferString("{bad json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", res.StatusCode)
	}
}
