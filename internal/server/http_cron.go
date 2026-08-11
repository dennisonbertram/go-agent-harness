package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/store"
)

// handleCronJobsRoot handles GET /v1/cron/jobs and POST /v1/cron/jobs.
func (s *Server) handleCronJobsRoot(w http.ResponseWriter, r *http.Request) {
	if s.cronClient == nil {
		writeError(w, http.StatusNotImplemented, "not_configured", "cron not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		// GET /v1/cron/jobs — requires runs:read
		if !hasScope(r.Context(), store.ScopeRunsRead) {
			writeScopeError(w, store.ScopeRunsRead)
			return
		}
		s.handleCronListJobs(w, r)
	case http.MethodPost:
		// POST /v1/cron/jobs — requires runs:write
		if !hasScope(r.Context(), store.ScopeRunsWrite) {
			writeScopeError(w, store.ScopeRunsWrite)
			return
		}
		s.handleCronCreateJob(w, r)
	default:
		writeMethodNotAllowed(w, "GET, POST")
	}
}

// handleCronJobByID handles all /v1/cron/jobs/{id} and sub-path requests.
func (s *Server) handleCronJobByID(w http.ResponseWriter, r *http.Request) {
	if s.cronClient == nil {
		writeError(w, http.StatusNotImplemented, "not_configured", "cron not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/cron/jobs/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	if len(parts) == 2 {
		switch parts[1] {
		case "executions":
			// GET /v1/cron/jobs/{id}/executions — requires runs:read.
			if r.Method != http.MethodGet {
				writeMethodNotAllowed(w, http.MethodGet)
				return
			}
			if !hasScope(r.Context(), store.ScopeRunsRead) {
				writeScopeError(w, store.ScopeRunsRead)
				return
			}
			s.handleCronListExecutions(w, r, id)
		case "pause":
			// POST /v1/cron/jobs/{id}/pause — requires runs:write
			if !hasScope(r.Context(), store.ScopeRunsWrite) {
				writeScopeError(w, store.ScopeRunsWrite)
				return
			}
			s.handleCronPauseJob(w, r, id)
		case "resume":
			// POST /v1/cron/jobs/{id}/resume — requires runs:write
			if !hasScope(r.Context(), store.ScopeRunsWrite) {
				writeScopeError(w, store.ScopeRunsWrite)
				return
			}
			s.handleCronResumeJob(w, r, id)
		default:
			http.NotFound(w, r)
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		// GET /v1/cron/jobs/{id} — requires runs:read
		if !hasScope(r.Context(), store.ScopeRunsRead) {
			writeScopeError(w, store.ScopeRunsRead)
			return
		}
		s.handleCronGetJob(w, r, id)
	case http.MethodPatch:
		// PATCH /v1/cron/jobs/{id} — requires runs:write
		if !hasScope(r.Context(), store.ScopeRunsWrite) {
			writeScopeError(w, store.ScopeRunsWrite)
			return
		}
		s.handleCronUpdateJob(w, r, id)
	case http.MethodDelete:
		// DELETE /v1/cron/jobs/{id} — requires runs:write
		if !hasScope(r.Context(), store.ScopeRunsWrite) {
			writeScopeError(w, store.ScopeRunsWrite)
			return
		}
		s.handleCronDeleteJob(w, r, id)
	default:
		writeMethodNotAllowed(w, "GET, PATCH, DELETE")
	}
}

// handleCronListJobs handles GET /v1/cron/jobs.
func (s *Server) handleCronListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.cronClient.ListJobs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	jobs = filterCronJobsByTenant(jobs, TenantIDFromContext(r.Context()))
	if jobs == nil {
		jobs = []tools.CronJob{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

// handleCronCreateJob handles POST /v1/cron/jobs.
func (s *Server) handleCronCreateJob(w http.ResponseWriter, r *http.Request) {
	var req tools.CronCreateJobRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawFields); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	if req.Schedule == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "schedule is required")
		return
	}
	if _, explicitlySet := rawFields["timeout_seconds"]; explicitlySet && req.TimeoutSec <= 0 {
		writeError(w, http.StatusBadRequest, "validation_error", "timeout_seconds must be positive")
		return
	}
	if req.TimeoutSec == 0 {
		req.TimeoutSec = 30
	}
	req.TenantID = TenantIDFromContext(r.Context())

	job, err := s.cronClient.CreateJob(r.Context(), req)
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

// handleCronGetJob handles GET /v1/cron/jobs/{id}.
func (s *Server) handleCronGetJob(w http.ResponseWriter, r *http.Request, id string) {
	job, err := s.cronJobForTenant(r.Context(), id)
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// handleCronListExecutions handles GET /v1/cron/jobs/{id}/executions.
// It authorizes the job before reading its history so a caller cannot use a
// known ID to learn whether another tenant has executions.
func (s *Server) handleCronListExecutions(w http.ResponseWriter, r *http.Request, id string) {
	if _, err := s.cronJobForTenant(r.Context(), id); err != nil {
		writeCronJobError(w, err)
		return
	}

	limit, offset := cronExecutionsPage(r)
	executions, err := s.cronClient.ListExecutions(r.Context(), id, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if executions == nil {
		executions = []tools.CronExecution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": executions})
}

func cronExecutionsPage(r *http.Request) (limit, offset int) {
	const (
		defaultLimit = 20
		maxLimit     = 100
	)
	limit = defaultLimit
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			limit = min(parsed, maxLimit)
		}
	}
	if value := r.URL.Query().Get("offset"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}

// handleCronUpdateJob handles PATCH /v1/cron/jobs/{id}.
func (s *Server) handleCronUpdateJob(w http.ResponseWriter, r *http.Request, id string) {
	var req tools.CronUpdateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if _, err := s.cronJobForTenant(r.Context(), id); err != nil {
		writeCronJobError(w, err)
		return
	}
	job, err := s.cronClient.UpdateJob(r.Context(), id, req)
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	if !cronJobVisibleToTenant(job, TenantIDFromContext(r.Context())) {
		writeCronJobError(w, tools.ErrCronJobNotFound)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// handleCronDeleteJob handles DELETE /v1/cron/jobs/{id}.
func (s *Server) handleCronDeleteJob(w http.ResponseWriter, r *http.Request, id string) {
	if _, err := s.cronJobForTenant(r.Context(), id); err != nil {
		writeCronJobError(w, err)
		return
	}
	expectedUpdatedAt, err := cronActionExpectedUpdatedAt(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if expectedUpdatedAt != nil {
		err = s.cronClient.DeleteJobCAS(r.Context(), id, *expectedUpdatedAt)
	} else {
		err = s.cronClient.DeleteJob(r.Context(), id)
	}
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleCronPauseJob handles POST /v1/cron/jobs/{id}/pause.
func (s *Server) handleCronPauseJob(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	job, err := s.cronJobForTenant(r.Context(), id)
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	if job.Status != "active" {
		writeCronJobError(w, tools.ErrCronJobConflict)
		return
	}
	expectedUpdatedAt, err := cronActionExpectedUpdatedAt(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	paused := "paused"
	job, err = s.cronClient.UpdateJob(r.Context(), id, tools.CronUpdateJobRequest{
		Status: &paused, ExpectedUpdatedAt: expectedUpdatedAt,
	})
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	if !cronJobVisibleToTenant(job, TenantIDFromContext(r.Context())) {
		writeCronJobError(w, tools.ErrCronJobNotFound)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// handleCronResumeJob handles POST /v1/cron/jobs/{id}/resume.
func (s *Server) handleCronResumeJob(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	job, err := s.cronJobForTenant(r.Context(), id)
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	if job.Status != "paused" {
		writeCronJobError(w, tools.ErrCronJobConflict)
		return
	}
	expectedUpdatedAt, err := cronActionExpectedUpdatedAt(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	active := "active"
	job, err = s.cronClient.UpdateJob(r.Context(), id, tools.CronUpdateJobRequest{
		Status: &active, ExpectedUpdatedAt: expectedUpdatedAt,
	})
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	if !cronJobVisibleToTenant(job, TenantIDFromContext(r.Context())) {
		writeCronJobError(w, tools.ErrCronJobNotFound)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) cronJobForTenant(ctx context.Context, id string) (tools.CronJob, error) {
	job, err := s.cronClient.GetJob(ctx, id)
	if err != nil {
		return tools.CronJob{}, err
	}
	if !cronJobVisibleToTenant(job, TenantIDFromContext(ctx)) {
		return tools.CronJob{}, tools.ErrCronJobNotFound
	}
	return job, nil
}

func filterCronJobsByTenant(jobs []tools.CronJob, tenantID string) []tools.CronJob {
	if tenantID == "" {
		return jobs
	}
	filtered := make([]tools.CronJob, 0, len(jobs))
	for _, job := range jobs {
		if job.TenantID == tenantID {
			filtered = append(filtered, job)
		}
	}
	return filtered
}

func cronJobVisibleToTenant(job tools.CronJob, tenantID string) bool {
	return tenantID == "" || job.TenantID == tenantID
}

func writeCronJobError(w http.ResponseWriter, err error) {
	if errors.Is(err, tools.ErrCronJobValidation) {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if errors.Is(err, tools.ErrCronJobConflict) {
		writeError(w, http.StatusConflict, "conflict", "cron job changed or cannot perform that action")
		return
	}
	if errors.Is(err, tools.ErrCronJobNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
}

// cronActionExpectedUpdatedAt accepts an omitted/empty body for older clients,
// while letting current Activity rows submit their observed version as a CAS
// fence.  Action endpoints deliberately ignore every other field.
func cronActionExpectedUpdatedAt(r *http.Request) (*time.Time, error) {
	var request struct {
		ExpectedUpdatedAt *time.Time `json:"expected_updated_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	return request.ExpectedUpdatedAt, nil
}
