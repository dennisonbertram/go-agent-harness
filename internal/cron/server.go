package cron

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	robfigcron "github.com/robfig/cron/v3"
)

// Server provides REST API handlers for cron job management.
type Server struct {
	store     Store
	scheduler *Scheduler
	clock     Clock
	mu        sync.Mutex
	auth      IngressAuthConfig
}

// IngressAuthConfig binds one cronsd bearer credential to one authoritative
// tenant. A cronsd instance therefore cannot accept a caller-supplied tenant
// identity, even when the caller is another harness service.
type IngressAuthConfig struct {
	APIKey   string
	TenantID string
}

// Validate rejects an unprotected or tenant-ambiguous management API.
func (c IngressAuthConfig) Validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("CRONSD_INGRESS_API_KEY is required")
	}
	if strings.TrimSpace(c.TenantID) == "" {
		return fmt.Errorf("CRONSD_INGRESS_TENANT_ID is required")
	}
	return nil
}

type ingressTenantContextKey struct{}

// NewServer creates an authenticated http.Handler with cron API routes.
// /healthz is deliberately unauthenticated and contains liveness only;
// readiness and every job-management route require the configured bearer.
func NewServer(store Store, scheduler *Scheduler, clock Clock, auth IngressAuthConfig) http.Handler {
	auth.APIKey = strings.TrimSpace(auth.APIKey)
	auth.TenantID = strings.TrimSpace(auth.TenantID)
	s := &Server{store: store, scheduler: scheduler, clock: clock, auth: auth}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.Handle("/readyz", s.requireIngressAuth(http.HandlerFunc(s.handleReady)))
	mux.Handle("/v1/jobs", s.requireIngressAuth(http.HandlerFunc(s.handleJobs)))
	mux.Handle("/v1/jobs/by-name", s.requireIngressAuth(http.HandlerFunc(s.handleGetJobByName)))
	mux.Handle("/v1/jobs/", s.requireIngressAuth(http.HandlerFunc(s.handleJobByID)))
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) requireIngressAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.auth.Validate(); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not_ready", "cron ingress authentication is not configured")
			return
		}
		token := ingressBearerToken(r.Header.Get("Authorization"))
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.auth.APIKey)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authorization required")
			return
		}
		ctx := context.WithValue(r.Context(), ingressTenantContextKey{}, s.auth.TenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ingressBearerToken(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func ingressTenantID(ctx context.Context) string {
	tenantID, _ := ctx.Value(ingressTenantContextKey{}).(string)
	return tenantID
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListJobs(w, r)
	case http.MethodPost:
		s.handleCreateJob(w, r)
	default:
		writeMethodNotAllowed(w, "GET, POST")
	}
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	var req CreateJobRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	tenantID := ingressTenantID(r.Context())
	if scope, scoped, scopeErr := requestScope(r); scopeErr != nil {
		writeError(w, http.StatusBadRequest, "validation_error", scopeErr.Error())
		return
	} else if scoped && (req.TenantID != scope.TenantID || req.ConversationID != scope.ConversationID || req.AgentID != scope.AgentID) {
		writeError(w, http.StatusForbidden, "forbidden", "cron create scope does not match request scope")
		return
	}
	if strings.TrimSpace(req.TenantID) != "" && strings.TrimSpace(req.TenantID) != tenantID {
		writeError(w, http.StatusForbidden, "tenant_mismatch", "request tenant does not match authenticated tenant")
		return
	}
	req.TenantID = tenantID
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawFields); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}

	if req.Schedule == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "schedule is required")
		return
	}

	nextRun, err := NextRunTime(req.Schedule, s.clock.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid schedule: %v", err))
		return
	}

	if req.ExecType != ExecTypeShell && req.ExecType != ExecTypeHarness {
		writeError(w, http.StatusBadRequest, "validation_error", "execution_type must be \"shell\" or \"harness\"")
		return
	}
	if err := ValidateExecutionConfig(req.ExecType, req.ExecConfig); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if _, explicitlySet := rawFields["timeout_seconds"]; explicitlySet && req.TimeoutSec <= 0 {
		writeError(w, http.StatusBadRequest, "validation_error", "timeout_seconds must be positive")
		return
	}
	if req.TimeoutSec == 0 {
		req.TimeoutSec = 30
	}

	now := s.clock.Now()
	job := Job{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		ConversationID: req.ConversationID,
		AgentID:        req.AgentID,
		Name:           req.Name,
		Schedule:       req.Schedule,
		ExecType:       req.ExecType,
		ExecConfig:     req.ExecConfig,
		// Creation is deliberately persisted non-runnable. The live scheduler is
		// registered first and this row is CAS-activated only afterwards; a
		// scheduler or activation failure can therefore never survive restart as
		// an active orphan.
		Status:     StatusPaused,
		TimeoutSec: req.TimeoutSec,
		Tags:       req.Tags,
		NextRunAt:  nextRun,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.scheduler.ValidateJob(job); err != nil {
		writeError(w, http.StatusServiceUnavailable, "executor_not_ready", boundedMessage(err.Error()))
		return
	}

	job, err = s.store.CreateJob(r.Context(), job)
	if err != nil {
		writeInternalError(w, "store_error")
		return
	}

	activeJob := job
	activeJob.Status = StatusActive
	if addErr := s.scheduler.AddJob(activeJob); addErr != nil {
		s.scheduler.RemoveJob(job.ID)
		writeInternalError(w, "scheduler_error")
		return
	}
	activeJob.UpdatedAt = s.clock.Now()
	if !activeJob.UpdatedAt.After(job.UpdatedAt) {
		activeJob.UpdatedAt = job.UpdatedAt.Add(time.Nanosecond)
	}
	if err := s.store.UpdateJobCAS(r.Context(), activeJob, job.UpdatedAt); err != nil {
		s.scheduler.RemoveJob(job.ID)
		writeError(w, http.StatusInternalServerError, "store_error", fmt.Sprintf("activate registered job: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, activeJob)
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	scope, scoped, err := requestScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	var jobs []Job
	if scoped {
		if store, ok := s.store.(ScopedStore); ok {
			jobs, err = store.ListJobsInScope(r.Context(), scope)
		} else {
			jobs, err = s.store.ListJobs(r.Context())
			if err == nil {
				jobs = filterJobsInScope(jobs, scope)
			}
		}
	} else {
		jobs, err = s.store.ListJobs(r.Context())
	}
	if err != nil {
		writeInternalError(w, "store_error")
		return
	}
	tenantID := ingressTenantID(r.Context())
	filtered := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		visible, err := s.jobVisibleToTenant(r.Context(), &job, tenantID)
		if err != nil {
			writeInternalError(w, "store_error")
			return
		}
		if visible {
			filtered = append(filtered, job)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": filtered})
}

func (s *Server) handleJobByID(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/jobs/") {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	id := parts[0]
	if len(parts) == 2 && parts[1] == "history" {
		s.handleHistory(w, r, id)
		return
	}
	if len(parts) > 1 {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetJob(w, r, id)
	case http.MethodPatch:
		s.handleUpdateJob(w, r, id)
	case http.MethodDelete:
		s.handleDeleteJob(w, r, id)
	default:
		writeMethodNotAllowed(w, "GET, PATCH, DELETE")
	}
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request, id string) {
	scope, scoped, scopeErr := requestScope(r)
	if scopeErr != nil {
		writeError(w, http.StatusBadRequest, "validation_error", scopeErr.Error())
		return
	}
	job, err := s.getJob(r.Context(), id, scope, scoped)
	if err == nil {
		job, err = s.authorizeJob(r.Context(), job)
	}
	if err != nil {
		if IsJobNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "job not found")
		} else {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// handleGetJobByName is a distinct query-parameter operator lookup. A query
// value preserves every non-empty job name, including slashes and percent
// signs, while model-facing CRUD remains on the ID-only /v1/jobs/{id} route.
func (s *Server) handleGetJobByName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, "GET")
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	scope, scoped, scopeErr := requestScope(r)
	if scopeErr != nil {
		writeError(w, http.StatusBadRequest, "validation_error", scopeErr.Error())
		return
	}
	var job Job
	var err error
	if store, ok := s.store.(ScopedStore); ok && scoped {
		job, err = store.GetJobByNameInScope(r.Context(), name, scope)
	} else {
		job, err = s.store.GetJobByName(r.Context(), name)
		if err == nil && scoped && !scope.Matches(job) {
			err = ErrJobNotFound
		}
	}
	if err == nil {
		job, err = s.authorizeJob(r.Context(), job)
	}
	if err != nil {
		if IsJobNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "job not found")
		} else if IsJobAmbiguous(err) {
			writeError(w, http.StatusConflict, "ambiguous", "job name matches multiple scopes; use a job ID or provide scope")
		} else {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleUpdateJob(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope, scoped, scopeErr := requestScope(r)
	if scopeErr != nil {
		writeError(w, http.StatusBadRequest, "validation_error", scopeErr.Error())
		return
	}
	job, err := s.getJob(r.Context(), id, scope, scoped)
	if err == nil {
		job, err = s.authorizeJob(r.Context(), job)
	}
	if err != nil {
		if !IsJobNotFound(err) {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	originalJob := job

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	var req UpdateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if req.TimeoutSec != nil && *req.TimeoutSec <= 0 {
		writeError(w, http.StatusBadRequest, "validation_error", "timeout_seconds must be positive")
		return
	}
	expectedUpdatedAt := job.UpdatedAt
	if req.ExpectedUpdatedAt != nil {
		expectedUpdatedAt = req.ExpectedUpdatedAt.UTC()
	}

	if req.Schedule != nil {
		trimmed := strings.TrimSpace(*req.Schedule)
		if trimmed == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "schedule must not be empty")
			return
		}
		nextRun, err := NextRunTime(*req.Schedule, s.clock.Now())
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid schedule: %v", err))
			return
		}
		job.Schedule = *req.Schedule
		job.NextRunAt = nextRun
	}
	if req.ExecConfig != nil {
		job.ExecConfig = *req.ExecConfig
	}
	if req.TimeoutSec != nil {
		job.TimeoutSec = *req.TimeoutSec
	}
	if req.Tags != nil {
		job.Tags = *req.Tags
	}
	if err := ValidateExecutionConfig(job.ExecType, job.ExecConfig); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if req.Status != nil {
		if *req.Status != StatusActive && *req.Status != StatusPaused {
			writeError(w, http.StatusBadRequest, "validation_error", "status must be \"active\" or \"paused\"")
			return
		}
		job.Status = *req.Status
	}

	// Gate on job.Status (the EFFECTIVE post-update status), not on
	// req.Status (the raw request field). A schedule-only PATCH
	// (req.Status == nil) must not re-arm a job whose stored status is
	// paused: job.Status already reflects that live status in that case.
	// For a resume+schedule PATCH, the status block above already set
	// job.Status = StatusActive, so this still correctly re-arms
	// genuinely-active jobs.
	job.UpdatedAt = s.clock.Now()
	if !job.UpdatedAt.After(expectedUpdatedAt) {
		job.UpdatedAt = expectedUpdatedAt.Add(time.Nanosecond)
	}
	scheduleChanged := req.Schedule != nil
	// Prepare an inert scheduler entry before the durable CAS. A failed prepare
	// or CAS leaves the prior active row and its live entry unchanged; after the
	// CAS, the in-memory commit is deliberately infallible.
	twoPhaseActivate := job.Status == StatusActive && (scheduleChanged || originalJob.Status != StatusActive)
	if twoPhaseActivate {
		prepared, err := s.scheduler.PrepareJob(job)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scheduler_error", err.Error())
			return
		}
		if err := s.store.UpdateJobCAS(r.Context(), job, expectedUpdatedAt); err != nil {
			s.scheduler.AbortJob(prepared)
			if IsJobConflict(err) {
				writeError(w, http.StatusConflict, "conflict", "cron job changed; refresh before updating")
				return
			}
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		s.scheduler.CommitJob(prepared)
		writeJSON(w, http.StatusOK, job)
		return
	}

	if err := s.store.UpdateJobCAS(r.Context(), job, expectedUpdatedAt); err != nil {
		if IsJobConflict(err) {
			writeError(w, http.StatusConflict, "conflict", "cron job changed; refresh before updating")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	if job.Status == StatusPaused {
		s.scheduler.RemoveJob(job.ID)
	}

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope, scoped, scopeErr := requestScope(r)
	if scopeErr != nil {
		writeError(w, http.StatusBadRequest, "validation_error", scopeErr.Error())
		return
	}
	job, err := s.getJob(r.Context(), id, scope, scoped)
	if err == nil {
		job, err = s.authorizeJob(r.Context(), job)
	}
	if err != nil {
		if IsJobNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "job not found")
		} else {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		}
		return
	}
	var req DeleteJobRequest
	if r.Body != nil && r.Body != http.NoBody {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
				return
			}
		}
	}
	var deleteErr error
	if req.ExpectedUpdatedAt != nil {
		deleteErr = s.store.DeleteJobCAS(r.Context(), id, req.ExpectedUpdatedAt.UTC())
	} else {
		deleteErr = s.store.DeleteJob(r.Context(), id)
	}
	if deleteErr != nil {
		if IsJobNotFound(deleteErr) {
			writeError(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		if IsJobConflict(deleteErr) {
			writeError(w, http.StatusConflict, "conflict", "cron job changed; call cron_get and retry")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", deleteErr.Error())
		return
	}
	s.scheduler.RemoveJob(job.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, "GET")
		return
	}
	scope, scoped, scopeErr := requestScope(r)
	if scopeErr != nil {
		writeError(w, http.StatusBadRequest, "validation_error", scopeErr.Error())
		return
	}
	job, err := s.getJob(r.Context(), jobID, scope, scoped)
	if err == nil {
		job, err = s.authorizeJob(r.Context(), job)
	}
	if err != nil {
		if IsJobNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "job not found")
		} else {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		}
		return
	}

	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	execs, err := s.store.ListExecutions(r.Context(), job.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if execs == nil {
		execs = []Execution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": execs})
}

func requestScope(r *http.Request) (Scope, bool, error) {
	scope := Scope{TenantID: r.Header.Get("X-Cron-Tenant-ID"), ConversationID: r.Header.Get("X-Cron-Conversation-ID"), AgentID: r.Header.Get("X-Cron-Agent-ID")}
	if scope.TenantID == "" && scope.ConversationID == "" && scope.AgentID == "" {
		return Scope{}, false, nil
	}
	if !scope.Complete() {
		return Scope{}, false, fmt.Errorf("complete cron scope is required")
	}
	return scope, true, nil
}

func filterJobsInScope(jobs []Job, scope Scope) []Job {
	filtered := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		if scope.Matches(job) {
			filtered = append(filtered, job)
		}
	}
	return filtered
}

func (s *Server) getJob(ctx context.Context, id string, scope Scope, scoped bool) (Job, error) {
	if store, ok := s.store.(ScopedStore); ok && scoped {
		return store.GetJobInScope(ctx, id, scope)
	}
	job, err := s.store.GetJob(ctx, id)
	if err == nil && scoped && !scope.Matches(job) {
		return Job{}, ErrJobNotFound
	}
	return job, err
}

func (s *Server) authorizeJob(ctx context.Context, job Job) (Job, error) {
	visible, err := s.jobVisibleToTenant(ctx, &job, ingressTenantID(ctx))
	if err != nil {
		return Job{}, err
	}
	if !visible {
		return Job{}, ErrJobNotFound
	}
	return job, nil
}

func (s *Server) jobVisibleToTenant(ctx context.Context, job *Job, tenantID string) (bool, error) {
	if job.TenantID == tenantID {
		return true, nil
	}
	// Pre-tenant schema rows were shell-only. A configured single-tenant
	// cronsd may expose one only after the store has durably and atomically
	// assigned it. Stores without this contract must keep the row invisible.
	if job.TenantID == "" && job.ExecType == ExecTypeShell {
		claimer, ok := s.store.(JobTenantClaimer)
		if !ok {
			return false, nil
		}
		claimedJob, claimed, err := claimer.ClaimJobTenant(ctx, job.ID, tenantID)
		if err != nil {
			return false, err
		}
		if !claimed || claimedJob.TenantID != tenantID {
			return false, nil
		}
		*job = claimedJob
		return true, nil
	}
	return false, nil
}

func NextRunTime(schedule string, from time.Time) (time.Time, error) {
	parser := robfigcron.NewParser(robfigcron.Minute | robfigcron.Hour | robfigcron.Dom | robfigcron.Month | robfigcron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(from), nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": boundedMessage(message)},
	})
}

func writeInternalError(w http.ResponseWriter, code string) {
	writeError(w, http.StatusInternalServerError, code, "internal error")
}

func boundedMessage(message string) string {
	const maxBytes = 256
	if len(message) <= maxBytes {
		return message
	}
	return message[:maxBytes]
}

func writeMethodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}
