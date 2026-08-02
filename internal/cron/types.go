package cron

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Scope is the immutable ownership tuple for a conversational cron job.
type Scope struct{ TenantID, ConversationID, AgentID string }

func (s Scope) Complete() bool { return s.TenantID != "" && s.ConversationID != "" && s.AgentID != "" }
func (s Scope) Matches(job Job) bool {
	return s.TenantID == job.TenantID && s.ConversationID == job.ConversationID && s.AgentID == job.AgentID
}

type scopeContextKey struct{}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, scope)
}
func ScopeFromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(Scope)
	return scope, ok && scope.Complete()
}

var ErrJobNotFound = errors.New("cron job not found")
var ErrJobConflict = errors.New("cron job update conflict")
var ErrJobAmbiguous = errors.New("cron job name is ambiguous")

// ErrExecutionSkippedOverlap is persisted on a skipped execution when another
// cron-started run already owns the same durable conversation scope. It is a
// stable machine-readable reason, not display prose.
var ErrExecutionSkippedOverlap = errors.New("cron execution skipped: scoped conversation already active")

// ErrRunObservationUnavailable means the runner lifecycle bridge is not bound
// yet. It is nonterminal: startup ordering must not turn a live run into a
// failed history record or release its scoped lease.
var ErrRunObservationUnavailable = errors.New("cron run observation unavailable")

func IsJobNotFound(err error) bool {
	return errors.Is(err, ErrJobNotFound) || errors.Is(err, sql.ErrNoRows)
}

func IsJobConflict(err error) bool {
	return errors.Is(err, ErrJobConflict)
}

func IsJobAmbiguous(err error) bool { return errors.Is(err, ErrJobAmbiguous) }

// Job status constants
const (
	StatusActive  = "active"
	StatusPaused  = "paused"
	StatusDeleted = "deleted"
)

// Execution status constants
const (
	ExecStatusQueued    = "queued"
	ExecStatusStarting  = "starting"
	ExecStatusRunning   = "running"
	ExecStatusSucceeded = "succeeded"
	ExecStatusFailed    = "failed"
	ExecStatusTimeout   = "timeout"
	ExecStatusSkipped   = "skipped"

	// Compatibility names retain existing callers while the wire lifecycle is
	// made explicit. Older persisted `pending`/`success` rows remain readable.
	ExecStatusPending = ExecStatusQueued
	ExecStatusSuccess = ExecStatusSucceeded
)

// Execution type constants
const (
	ExecTypeShell   = "shell"
	ExecTypeHarness = "harness"
)

// Job represents a scheduled job.
type Job struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
	AgentID        string    `json:"agent_id,omitempty"`
	Name           string    `json:"name"`
	Schedule       string    `json:"schedule"`
	ExecType       string    `json:"execution_type"`
	ExecConfig     string    `json:"execution_config"` // JSON blob
	Status         string    `json:"status"`
	TimeoutSec     int       `json:"timeout_seconds"`
	Tags           string    `json:"tags"` // comma-separated
	NextRunAt      time.Time `json:"next_run_at"`
	LastRunAt      time.Time `json:"last_run_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Execution represents a single run of a job.
type Execution struct {
	ID            string    `json:"id"`
	JobID         string    `json:"job_id"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at,omitempty"`
	Status        string    `json:"status"`
	RunID         string    `json:"run_id,omitempty"` // harness run ID
	OutputSummary string    `json:"output_summary,omitempty"`
	Error         string    `json:"error,omitempty"`
	DurationMs    int64     `json:"duration_ms"`
}

// CreateJobRequest is the request payload for creating a job.
type CreateJobRequest struct {
	TenantID       string `json:"tenant_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	Name           string `json:"name"`
	Schedule       string `json:"schedule"`
	ExecType       string `json:"execution_type"`
	ExecConfig     string `json:"execution_config"`
	TimeoutSec     int    `json:"timeout_seconds,omitempty"`
	Tags           string `json:"tags,omitempty"`
}

// UpdateJobRequest is the request payload for updating a job.
type UpdateJobRequest struct {
	Schedule          *string    `json:"schedule,omitempty"`
	ExecConfig        *string    `json:"execution_config,omitempty"`
	Status            *string    `json:"status,omitempty"`
	TimeoutSec        *int       `json:"timeout_seconds,omitempty"`
	Tags              *string    `json:"tags,omitempty"`
	ExpectedUpdatedAt *time.Time `json:"expected_updated_at,omitempty"`
}

// DeleteJobRequest optionally carries an optimistic version. Operator callers
// may keep using an empty DELETE, while model-facing tools always supply the
// updated_at returned by cron_get.
type DeleteJobRequest struct {
	ExpectedUpdatedAt *time.Time `json:"expected_updated_at,omitempty"`
}

// ListExecutionsRequest is the request payload for listing executions.
type ListExecutionsRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
