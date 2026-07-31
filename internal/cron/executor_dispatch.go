package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RunStartRequest is the immutable, typed boundary between a scheduled
// execution and the harness runner. Scope comes from the persisted job, while
// JobID and ExecutionID identify the lifecycle records for this fire.
type RunStartRequest struct {
	Prompt         string
	ConversationID string
	TenantID       string
	AgentID        string
	JobID          string
	ExecutionID    string
}

// RunStarter starts a harness run and returns its id.
//
// Cron takes this rather than a *harness.Runner so this package stays free of
// a dependency on the harness, and so the daemon can bind the real runner
// after the scheduler is already constructed.
type RunStarter interface {
	StartRun(req RunStartRequest) (runID string, err error)
}

type executionAwareExecutor interface {
	ExecuteWithID(ctx context.Context, job Job, executionID string) (string, error)
}

// harnessConfig is the JSON structure for harness execution config.
//
// The prompt lives here rather than on Job: the HTTP create route stores only
// execution_config, and a top-level "prompt" in the request body is silently
// discarded — which is how a harness job ends up with empty config and fails
// to parse.
type harnessConfig struct {
	Prompt string `json:"prompt,omitempty"`
	// ConversationID pins the run to an existing conversation so its output
	// lands in a transcript someone is watching. Empty starts a fresh one.
	ConversationID string `json:"conversation_id,omitempty"`
}

// HarnessExecutor runs a job as a harness agent run.
type HarnessExecutor struct {
	Starter RunStarter
}

// Execute starts a run for the job and returns its id.
//
// It returns as soon as the run is accepted rather than waiting for it to
// finish: a cron job's timeout bounds how long *scheduling* may take, and an
// agent run can legitimately outlive it. The run's own lifecycle is observable
// through the normal run and conversation streams.
func (e *HarnessExecutor) Execute(ctx context.Context, job Job) (string, error) {
	return e.ExecuteWithID(ctx, job, "")
}

// ExecuteWithID is the execution-aware harness path used by Scheduler. The
// job's persisted scope wins over any legacy scope-shaped fields in config.
func (e *HarnessExecutor) ExecuteWithID(ctx context.Context, job Job, executionID string) (string, error) {
	if e == nil || e.Starter == nil {
		return "", fmt.Errorf("harness execution is not configured on this daemon")
	}

	cfg := harnessConfig{}
	if trimmed := strings.TrimSpace(job.ExecConfig); trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
			return "", fmt.Errorf("parse execution config: %w", err)
		}
	}
	prompt := cfg.Prompt
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("harness job %q has no prompt to run", job.Name)
	}

	conversationID := job.ConversationID
	if conversationID == "" {
		// Preserve the pre-scope-column format for existing harness jobs. New
		// jobs always store conversation scope on Job and therefore cannot be
		// overridden by execution config updates.
		conversationID = cfg.ConversationID
	}
	runID, err := e.Starter.StartRun(RunStartRequest{
		Prompt:         prompt,
		ConversationID: conversationID,
		TenantID:       job.TenantID,
		AgentID:        job.AgentID,
		JobID:          job.ID,
		ExecutionID:    executionID,
	})
	if err != nil {
		return "", fmt.Errorf("start run: %w", err)
	}
	return "started run " + runID, nil
}

// DispatchExecutor routes a job to the executor for its execution type.
//
// Without this the shell executor received every job, including ones declared
// as harness work — so a harness job was accepted by the API, stored,
// scheduled, and stamped with last_run_at on every fire, while never being
// able to succeed. The type was validated at the edge and unimplemented at the
// centre.
type DispatchExecutor struct {
	Shell   Executor
	Harness Executor
}

func (d *DispatchExecutor) Execute(ctx context.Context, job Job) (string, error) {
	return d.execute(ctx, job, "")
}

func (d *DispatchExecutor) ExecuteWithID(ctx context.Context, job Job, executionID string) (string, error) {
	return d.execute(ctx, job, executionID)
}

func (d *DispatchExecutor) execute(ctx context.Context, job Job, executionID string) (string, error) {
	switch job.ExecType {
	case ExecTypeHarness:
		if d.Harness == nil {
			return "", fmt.Errorf("execution type %q is not available on this daemon", job.ExecType)
		}
		if aware, ok := d.Harness.(executionAwareExecutor); ok {
			return aware.ExecuteWithID(ctx, job, executionID)
		}
		return d.Harness.Execute(ctx, job)
	case ExecTypeShell, "":
		if d.Shell == nil {
			return "", fmt.Errorf("execution type %q is not available on this daemon", job.ExecType)
		}
		return d.Shell.Execute(ctx, job)
	default:
		// Reaching here means the API accepted a type this dispatcher does
		// not know about — the exact class of gap this type exists to close,
		// so it is reported rather than silently treated as a shell command.
		return "", fmt.Errorf("unknown execution type %q", job.ExecType)
	}
}
