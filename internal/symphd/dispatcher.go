package symphd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go-agent-harness/internal/workspace"
)

// HarnessClient is the interface for interacting with a harnessd instance.
// Implementations may use HTTP or be mocked for testing.
type HarnessClient interface {
	// StartRun posts a prompt to harnessd and returns a run ID.
	StartRun(ctx context.Context, prompt string, workspacePath string) (string, error)
	// RunStatus returns the current status of a run: "running", "completed", "failed", or "queued".
	RunStatus(ctx context.Context, runID string) (string, error)
}

// DispatchConfig holds dispatcher settings.
type DispatchConfig struct {
	// MaxConcurrent is the maximum number of parallel agent runs.
	MaxConcurrent int
	// StallTimeout is the time with no status change before a run is declared stalled.
	// Defaults to 5 minutes if zero.
	StallTimeout time.Duration
	// HarnessURL is the base URL of the harnessd instance.
	HarnessURL string
	// PollInterval controls how often RunStatus is polled.
	// Defaults to 5 seconds if zero.
	PollInterval time.Duration
}

// RunResult holds the outcome of a dispatched run.
type RunResult struct {
	IssueNumber int
	Success     bool
	Error       error
	Duration    time.Duration
}

// Dispatcher orchestrates workspace provisioning and harness dispatch.
// It claims issues from the tracker, provisions a workspace per issue, starts
// a harness run, monitors progress, detects stalls, and marks issues complete
// or failed.
type Dispatcher struct {
	config    DispatchConfig
	workspace workspace.Workspace
	tracker   Tracker
	client    HarnessClient

	sem     chan struct{} // semaphore limiting MaxConcurrent concurrent runs
	results chan RunResult

	mu      sync.Mutex
	running map[int]context.CancelFunc // issue number → cancel func
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(cfg DispatchConfig, ws workspace.Workspace, tracker Tracker, client HarnessClient) *Dispatcher {
	if cfg.StallTimeout <= 0 {
		cfg.StallTimeout = 5 * time.Minute
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	return &Dispatcher{
		config:    cfg,
		workspace: ws,
		tracker:   tracker,
		client:    client,
		sem:       make(chan struct{}, maxConcurrent),
		results:   make(chan RunResult, 64),
		running:   make(map[int]context.CancelFunc),
	}
}

// Results returns the channel on which completed RunResults are published.
// The caller should drain this channel to avoid blocking dispatched goroutines.
func (d *Dispatcher) Results() <-chan RunResult {
	return d.results
}

// Dispatch provisions a workspace and starts a harness run for the given issue.
// It calls tracker.Start() immediately, then workspace.Provision(), then
// client.StartRun(). Progress is polled at PollInterval; if StallTimeout elapses
// with no terminal status, the run is cancelled and marked failed.
// On completion, tracker.Complete() or tracker.Fail() is called and a RunResult
// is sent to the Results channel.
//
// Dispatch acquires a semaphore slot before launching the goroutine, so callers
// can call Dispatch sequentially and rely on backpressure from MaxConcurrent.
func (d *Dispatcher) Dispatch(ctx context.Context, issue *TrackedIssue) error {
	// Transition tracker state: Claimed → Running.
	if err := d.tracker.Start(issue.Number); err != nil {
		return fmt.Errorf("dispatcher: start issue #%d: %w", issue.Number, err)
	}

	// Acquire a semaphore slot (blocks until a slot is free or ctx is done).
	select {
	case d.sem <- struct{}{}:
	case <-ctx.Done():
		_ = d.tracker.Fail(issue.Number, "context cancelled before semaphore acquired")
		return ctx.Err()
	}

	// Create a per-run cancellable context derived from the parent.
	runCtx, cancel := context.WithCancel(ctx)

	d.mu.Lock()
	d.running[issue.Number] = cancel
	d.mu.Unlock()

	go func() {
		defer func() {
			cancel()
			<-d.sem // release slot
			d.mu.Lock()
			delete(d.running, issue.Number)
			d.mu.Unlock()
		}()

		start := time.Now()
		result := d.runIssue(runCtx, issue)
		result.Duration = time.Since(start)
		d.results <- result
	}()

	return nil
}

// Shutdown cancels all in-flight dispatches and waits for them to drain.
func (d *Dispatcher) Shutdown(ctx context.Context) {
	d.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(d.running))
	for _, cancel := range d.running {
		cancels = append(cancels, cancel)
	}
	d.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}

	// Drain the semaphore: wait until all slots are free.
	// Each running goroutine releases a slot when it exits.
	for i := 0; i < cap(d.sem); i++ {
		select {
		case d.sem <- struct{}{}:
			// slot acquired means it was free
		case <-ctx.Done():
			return
		}
	}
	// Release the slots we just acquired.
	for i := 0; i < cap(d.sem); i++ {
		<-d.sem
	}
}

// runIssue is the core per-issue dispatch logic executed in a goroutine.
func (d *Dispatcher) runIssue(ctx context.Context, issue *TrackedIssue) RunResult {
	result := RunResult{IssueNumber: issue.Number}

	// Provision workspace.
	opts := workspace.Options{
		ID:      fmt.Sprintf("issue-%d", issue.Number),
		BaseDir: "", // caller may configure via workspace implementation
	}
	if err := d.workspace.Provision(ctx, opts); err != nil {
		reason := fmt.Sprintf("workspace provision failed: %v", err)
		_ = d.tracker.Fail(issue.Number, reason)
		result.Error = fmt.Errorf("dispatcher: %s", reason)
		return result
	}

	workspacePath := d.workspace.WorkspacePath()

	// Build prompt from issue content.
	prompt := buildPrompt(issue)

	// Start run on harnessd.
	runID, err := d.client.StartRun(ctx, prompt, workspacePath)
	if err != nil {
		reason := fmt.Sprintf("harness start failed: %v", err)
		_ = d.tracker.Fail(issue.Number, reason)
		result.Error = fmt.Errorf("dispatcher: %s", reason)
		return result
	}

	// Poll for completion with stall detection.
	// The stall deadline is set once at dispatch time and NOT reset while the
	// run keeps returning "running" or "queued". A constant non-terminal status
	// IS the stall condition — the deadline only resets on a genuine status
	// transition (e.g. queued → running), not on repeated identical statuses.
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()

	stallTimer := time.NewTimer(d.config.StallTimeout)
	defer stallTimer.Stop()

	lastStatus := ""

	for {
		select {
		case <-ctx.Done():
			reason := "context cancelled"
			_ = d.tracker.Fail(issue.Number, reason)
			result.Error = fmt.Errorf("dispatcher: %s", reason)
			return result

		case <-stallTimer.C:
			reason := fmt.Sprintf("stall timeout (%v) exceeded for run %s", d.config.StallTimeout, runID)
			_ = d.tracker.Fail(issue.Number, reason)
			result.Error = fmt.Errorf("dispatcher: %s", reason)
			return result

		case <-ticker.C:
			status, err := d.client.RunStatus(ctx, runID)
			if err != nil {
				// Transient error — keep polling.
				continue
			}

			// Reset the stall timer when we see a genuine status transition.
			if status != lastStatus {
				lastStatus = status
				if !stallTimer.Stop() {
					select {
					case <-stallTimer.C:
					default:
					}
				}
				stallTimer.Reset(d.config.StallTimeout)
			}

			switch status {
			case "completed":
				if err := d.tracker.Complete(issue.Number); err != nil {
					result.Error = fmt.Errorf("dispatcher: complete tracker: %w", err)
					return result
				}
				result.Success = true
				return result

			case "failed":
				reason := fmt.Sprintf("harness run %s reported failed", runID)
				_ = d.tracker.Fail(issue.Number, reason)
				result.Error = fmt.Errorf("dispatcher: %s", reason)
				return result

			case "running", "queued":
				// Still in progress — stall timer handles timeout.

			default:
				// Unknown status — keep polling until stall timeout.
			}
		}
	}
}

// buildPrompt constructs the agent prompt for an issue.
func buildPrompt(issue *TrackedIssue) string {
	return fmt.Sprintf("Implement GitHub issue #%d: %s\n\n%s", issue.Number, issue.Title, issue.Body)
}

// HTTPHarnessClient implements HarnessClient using real HTTP calls to harnessd.
type HTTPHarnessClient struct {
	baseURL string
	client  *http.Client
}

// NewHTTPHarnessClient creates an HTTPHarnessClient pointing at the given base URL.
func NewHTTPHarnessClient(baseURL string) *HTTPHarnessClient {
	return &HTTPHarnessClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// startRunRequest is the JSON body for POST /v1/runs.
type startRunRequest struct {
	Prompt    string `json:"prompt"`
	Workspace string `json:"workspace,omitempty"`
}

// startRunResponse is the JSON body returned by POST /v1/runs.
type startRunResponse struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

// runStatusResponse is the JSON body returned by GET /v1/runs/{id}.
type runStatusResponse struct {
	Status string `json:"status"`
}

// StartRun posts a new run to POST /v1/runs and returns the run ID.
func (c *HTTPHarnessClient) StartRun(ctx context.Context, prompt string, workspacePath string) (string, error) {
	body := startRunRequest{Prompt: prompt, Workspace: workspacePath}
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("harness client: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/runs", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("harness client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("harness client: post /v1/runs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("harness client: /v1/runs returned status %d", resp.StatusCode)
	}

	var out startRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("harness client: decode response: %w", err)
	}
	if out.RunID == "" {
		return "", fmt.Errorf("harness client: empty run_id in response")
	}
	return out.RunID, nil
}

// RunStatus queries GET /v1/runs/{id} and returns the status string.
func (c *HTTPHarnessClient) RunStatus(ctx context.Context, runID string) (string, error) {
	url := fmt.Sprintf("%s/v1/runs/%s", c.baseURL, runID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("harness client: build request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("harness client: get /v1/runs/%s: %w", runID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("harness client: /v1/runs/%s returned status %d", runID, resp.StatusCode)
	}

	var out runStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("harness client: decode response: %w", err)
	}
	return out.Status, nil
}
