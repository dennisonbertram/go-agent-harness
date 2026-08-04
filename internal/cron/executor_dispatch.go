package cron

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RunStartRequest is the immutable, typed boundary between a scheduled
// execution and the harness runner. Scope comes from the persisted job, while
// JobID and ExecutionID identify the lifecycle records for this fire.
type RunStartRequest struct {
	Prompt            string
	Model             string
	ProviderName      string
	AllowFallback     bool
	FallbackProviders []string
	ConversationID    string
	TenantID          string
	AgentID           string
	JobID             string
	ExecutionID       string
}

// RunStarter starts a harness run and returns its id.
//
// Cron takes this rather than a *harness.Runner so this package stays free of
// a dependency on the harness, and so the daemon can bind the real runner
// after the scheduler is already constructed.
type RunStarter interface {
	StartRun(req RunStartRequest) (runID string, err error)
}

type contextAwareRunStarter interface {
	StartRunContext(ctx context.Context, req RunStartRequest) (runID string, err error)
}

// RunObservation is the terminal, transport-neutral view of a harness run.
// A starter that also implements RunObserver lets cron retain the scoped
// overlap lease until the scheduled conversation has actually finished.
type RunObservation struct {
	Succeeded     bool
	OutputSummary string
	Error         string
}

type RunObserver interface {
	ObserveRun(ctx context.Context, runID string) (RunObservation, error)
}

type executionAwareExecutor interface {
	ExecuteWithID(ctx context.Context, job Job, executionID string) (string, error)
}

// JobValidator lets daemon wiring reject jobs that cannot be dispatched before
// they are persisted or scheduled. It is intentionally optional so existing
// third-party Executor implementations keep their behavior.
type JobValidator interface {
	ValidateJob(job Job) error
}

// ExecutionOutcome separates machine-readable lifecycle data from a bounded
// user/operator-facing output summary. In particular, RunID must never be
// reconstructed by parsing the output text.
type ExecutionOutcome struct {
	RunID         string
	OutputSummary string
}

type executionOutcomeExecutor interface {
	ExecuteOutcomeWithID(ctx context.Context, job Job, executionID string) (ExecutionOutcome, error)
}

type executionOutcomeObserver interface {
	ObserveExecution(ctx context.Context, job Job, outcome ExecutionOutcome) (RunObservation, bool, error)
}

// harnessConfig is the JSON structure for harness execution config.
//
// The prompt lives here rather than on Job: the HTTP create route stores only
// execution_config, and a top-level "prompt" in the request body is silently
// discarded — which is how a harness job ends up with empty config and fails
// to parse.
type harnessConfig struct {
	Prompt            string   `json:"prompt,omitempty"`
	Model             string   `json:"model,omitempty"`
	ProviderName      string   `json:"provider_name,omitempty"`
	AllowFallback     bool     `json:"allow_fallback,omitempty"`
	FallbackProviders []string `json:"fallback_providers,omitempty"`
	// ConversationID pins the run to an existing conversation so its output
	// lands in a transcript someone is watching. Empty starts a fresh one.
	ConversationID string `json:"conversation_id,omitempty"`
}

// HarnessExecutor runs a job as a harness agent run.
type HarnessExecutor struct {
	Starter  RunStarter
	Observer RunObserver
}

// ObserveExecution waits for the configured harness observer when available.
// The StartRun boundary remains separate, so Scheduler persists RunID before
// this potentially long observation begins.
func (e *HarnessExecutor) ObserveExecution(ctx context.Context, job Job, outcome ExecutionOutcome) (RunObservation, bool, error) {
	if e == nil || e.Observer == nil {
		return RunObservation{}, false, nil
	}
	if outcome.RunID == "" {
		return RunObservation{}, true, fmt.Errorf("cannot observe empty harness run ID")
	}
	// Job timeout bounds start/dispatch, not the lifetime of the accepted
	// harness conversation. An agent can legitimately run longer than that
	// limit; cancelling observation here would misreport a live continuation as
	// timed out and prematurely release its overlap lease.
	observation, err := e.Observer.ObserveRun(ctx, outcome.RunID)
	if errors.Is(err, ErrRunObservationUnavailable) {
		return RunObservation{}, false, nil
	}
	return observation, true, err
}

func (e *HarnessExecutor) ValidateJob(job Job) error {
	if job.ExecType != ExecTypeHarness {
		return nil
	}
	if e == nil || e.Starter == nil {
		return fmt.Errorf("harness execution is not configured on this daemon")
	}
	if validator, ok := e.Starter.(JobValidator); ok {
		return validator.ValidateJob(job)
	}
	return nil
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
	outcome, err := e.ExecuteOutcomeWithID(ctx, job, executionID)
	return outcome.OutputSummary, err
}

// ExecuteOutcomeWithID starts the durable harness run and returns its stable
// ID independently of the human-readable summary. Scheduler persists RunID
// immediately after this accepted boundary.
func (e *HarnessExecutor) ExecuteOutcomeWithID(ctx context.Context, job Job, executionID string) (ExecutionOutcome, error) {
	if e == nil || e.Starter == nil {
		return ExecutionOutcome{}, fmt.Errorf("harness execution is not configured on this daemon")
	}

	cfg := harnessConfig{}
	if trimmed := strings.TrimSpace(job.ExecConfig); trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
			return ExecutionOutcome{}, fmt.Errorf("parse execution config: %w", err)
		}
	}
	prompt := cfg.Prompt
	if strings.TrimSpace(prompt) == "" {
		return ExecutionOutcome{}, fmt.Errorf("harness job %q has no prompt to run", job.Name)
	}

	conversationID := job.ConversationID
	if conversationID == "" {
		// Preserve the pre-scope-column format for existing harness jobs. New
		// jobs always store conversation scope on Job and therefore cannot be
		// overridden by execution config updates.
		conversationID = cfg.ConversationID
	}
	startRequest := RunStartRequest{
		Prompt:            prompt,
		Model:             cfg.Model,
		ProviderName:      cfg.ProviderName,
		AllowFallback:     cfg.AllowFallback,
		FallbackProviders: append([]string(nil), cfg.FallbackProviders...),
		ConversationID:    conversationID,
		TenantID:          job.TenantID,
		AgentID:           job.AgentID,
		JobID:             job.ID,
		ExecutionID:       executionID,
	}
	var runID string
	var err error
	if aware, ok := e.Starter.(contextAwareRunStarter); ok {
		startCtx := ctx
		cancel := func() {}
		if job.TimeoutSec > 0 {
			startCtx, cancel = context.WithTimeout(ctx, time.Duration(job.TimeoutSec)*time.Second)
		}
		defer cancel()
		runID, err = aware.StartRunContext(startCtx, startRequest)
	} else {
		runID, err = e.Starter.StartRun(startRequest)
	}
	if err != nil {
		return ExecutionOutcome{}, fmt.Errorf("start run: %w", err)
	}
	if strings.TrimSpace(runID) == "" {
		return ExecutionOutcome{}, fmt.Errorf("start run: harness accepted an empty run ID")
	}
	return ExecutionOutcome{RunID: runID, OutputSummary: "started run " + runID}, nil
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

func (d *DispatchExecutor) ValidateJob(job Job) error {
	switch job.ExecType {
	case ExecTypeHarness:
		if d == nil || d.Harness == nil {
			return fmt.Errorf("execution type %q is not available on this daemon", job.ExecType)
		}
		if validator, ok := d.Harness.(JobValidator); ok {
			return validator.ValidateJob(job)
		}
	case ExecTypeShell, "":
		if d == nil || d.Shell == nil {
			return fmt.Errorf("execution type %q is not available on this daemon", job.ExecType)
		}
	default:
		return fmt.Errorf("unknown execution type %q", job.ExecType)
	}
	return nil
}

func (d *DispatchExecutor) Execute(ctx context.Context, job Job) (string, error) {
	return d.execute(ctx, job, "")
}

func (d *DispatchExecutor) ExecuteWithID(ctx context.Context, job Job, executionID string) (string, error) {
	return d.execute(ctx, job, executionID)
}

// ExecuteOutcomeWithID preserves structured lifecycle results for harness
// jobs, while adapting shell and legacy executors without changing their
// terminal output semantics.
func (d *DispatchExecutor) ExecuteOutcomeWithID(ctx context.Context, job Job, executionID string) (ExecutionOutcome, error) {
	var executor Executor
	switch job.ExecType {
	case ExecTypeHarness:
		executor = d.Harness
	case ExecTypeShell, "":
		executor = d.Shell
	default:
		return ExecutionOutcome{}, fmt.Errorf("unknown execution type %q", job.ExecType)
	}
	if executor == nil {
		return ExecutionOutcome{}, fmt.Errorf("execution type %q is not available on this daemon", job.ExecType)
	}
	if aware, ok := executor.(executionOutcomeExecutor); ok {
		return aware.ExecuteOutcomeWithID(ctx, job, executionID)
	}
	if aware, ok := executor.(executionAwareExecutor); ok {
		output, err := aware.ExecuteWithID(ctx, job, executionID)
		return ExecutionOutcome{OutputSummary: output}, err
	}
	output, err := executor.Execute(ctx, job)
	return ExecutionOutcome{OutputSummary: output}, err
}

func (d *DispatchExecutor) ObserveExecution(ctx context.Context, job Job, outcome ExecutionOutcome) (RunObservation, bool, error) {
	if job.ExecType != ExecTypeHarness || d.Harness == nil {
		return RunObservation{}, false, nil
	}
	observer, ok := d.Harness.(executionOutcomeObserver)
	if !ok {
		return RunObservation{}, false, nil
	}
	return observer.ObserveExecution(ctx, job, outcome)
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
