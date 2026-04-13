package workflows

import "time"

type StepType string

const (
	StepTypeTool       StepType = "tool"
	StepTypeRun        StepType = "run"
	StepTypeCheckpoint StepType = "checkpoint"
	StepTypeBranch     StepType = "branch"
)

type RunStatus string

const (
	RunStatusQueued               RunStatus = "queued"
	RunStatusRunning              RunStatus = "running"
	RunStatusWaitingForCheckpoint RunStatus = "waiting_for_checkpoint"
	RunStatusCompleted            RunStatus = "completed"
	RunStatusFailed               RunStatus = "failed"
)

type StepStatus string

const (
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
)

type Definition struct {
	Name        string           `json:"name" yaml:"name"`
	Description string           `json:"description" yaml:"description"`
	Steps       []StepDefinition `json:"steps" yaml:"steps"`
}

type StepDefinition struct {
	ID         string            `json:"id" yaml:"id"`
	Type       StepType          `json:"type" yaml:"type"`
	Tool       string            `json:"tool,omitempty" yaml:"tool,omitempty"`
	Args       map[string]any    `json:"args,omitempty" yaml:"args,omitempty"`
	Run        *RunStep          `json:"run,omitempty" yaml:"run,omitempty"`
	Checkpoint *CheckpointStep   `json:"checkpoint,omitempty" yaml:"checkpoint,omitempty"`
	Field      string            `json:"field,omitempty" yaml:"field,omitempty"`
	Cases      map[string]string `json:"cases,omitempty" yaml:"cases,omitempty"`
	Default    string            `json:"default,omitempty" yaml:"default,omitempty"`
	Next       string            `json:"next,omitempty" yaml:"next,omitempty"`
}

type RunStep struct {
	Prompt string `json:"prompt" yaml:"prompt"`
	Model  string `json:"model,omitempty" yaml:"model,omitempty"`
}

type CheckpointStep struct {
	SuspendPayload map[string]any `json:"suspend_payload,omitempty" yaml:"suspend_payload,omitempty"`
	ResumeSchema   map[string]any `json:"resume_schema,omitempty" yaml:"resume_schema,omitempty"`
}

type Run struct {
	ID                  string    `json:"id"`
	WorkflowName        string    `json:"workflow_name"`
	Status              RunStatus `json:"status"`
	CurrentStepID       string    `json:"current_step_id,omitempty"`
	CurrentCheckpointID string    `json:"current_checkpoint_id,omitempty"`
	InputJSON           string    `json:"input_json,omitempty"`
	OutputJSON          string    `json:"output_json,omitempty"`
	Error               string    `json:"error,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type StepState struct {
	WorkflowRunID string     `json:"workflow_run_id"`
	StepID        string     `json:"step_id"`
	Status        StepStatus `json:"status"`
	OutputJSON    string     `json:"output_json,omitempty"`
	Error         string     `json:"error,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Event struct {
	Seq           int64          `json:"seq"`
	WorkflowRunID string         `json:"workflow_run_id"`
	Type          string         `json:"type"`
	Payload       map[string]any `json:"payload,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
}
