package checkpoints

import (
	"context"
	"time"
)

type Kind string

const (
	KindApproval       Kind = "approval"
	KindUserInput      Kind = "user_input"
	KindExternalResume Kind = "external_resume"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusApproved  Status = "approved"
	StatusDenied    Status = "denied"
	StatusResumed   Status = "resumed"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

type Record struct {
	ID             string    `json:"id"`
	Kind           Kind      `json:"kind"`
	Status         Status    `json:"status"`
	RunID          string    `json:"run_id,omitempty"`
	WorkflowRunID  string    `json:"workflow_run_id,omitempty"`
	CallID         string    `json:"call_id,omitempty"`
	Tool           string    `json:"tool,omitempty"`
	Args           string    `json:"args,omitempty"`
	Questions      string    `json:"questions,omitempty"`
	SuspendPayload string    `json:"suspend_payload,omitempty"`
	ResumeSchema   string    `json:"resume_schema,omitempty"`
	ResumePayload  string    `json:"resume_payload,omitempty"`
	DeadlineAt     time.Time `json:"deadline_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateRequest struct {
	Kind           Kind
	RunID          string
	WorkflowRunID  string
	CallID         string
	Tool           string
	Args           string
	Questions      string
	SuspendPayload string
	ResumeSchema   string
	DeadlineAt     time.Time
}

type WaitResult struct {
	Status  Status
	Payload map[string]any
}

type Store interface {
	Create(ctx context.Context, record *Record) error
	Update(ctx context.Context, record *Record) error
	Get(ctx context.Context, id string) (*Record, error)
	PendingByRun(ctx context.Context, runID string) (*Record, error)
	PendingByWorkflowRun(ctx context.Context, workflowRunID string) (*Record, error)
	Close() error
}

type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string {
	return "checkpoint not found: " + e.ID
}

func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}
