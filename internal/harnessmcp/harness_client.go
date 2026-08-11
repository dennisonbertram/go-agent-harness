package harnessmcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// HarnessClient is an HTTP client for the harnessd REST API.
type HarnessClient struct {
	baseURL    string
	httpClient *http.Client
}

// RunEvent is the typed payload delivered by harnessd's existing SSE endpoint.
type RunEvent struct {
	Type string
	Data map[string]any
}

func (c *HarnessClient) ContinueRun(ctx context.Context, runID, prompt string) (StartRunResponse, error) {
	return c.postRun(ctx, "/v1/runs/"+url.PathEscape(runID)+"/continue", map[string]string{"prompt": prompt})
}
func (c *HarnessClient) CancelRun(ctx context.Context, runID string) error {
	_, err := c.postRun(ctx, "/v1/runs/"+url.PathEscape(runID)+"/cancel", nil)
	return err
}
func (c *HarnessClient) ApproveRun(ctx context.Context, runID string) error {
	_, err := c.postRun(ctx, "/v1/runs/"+url.PathEscape(runID)+"/approve", nil)
	return err
}
func (c *HarnessClient) DenyRun(ctx context.Context, runID string) error {
	_, err := c.postRun(ctx, "/v1/runs/"+url.PathEscape(runID)+"/deny", nil)
	return err
}
func (c *HarnessClient) postRun(ctx context.Context, path string, bodyValue any) (StartRunResponse, error) {
	var body io.Reader
	if bodyValue != nil {
		raw, err := json.Marshal(bodyValue)
		if err != nil {
			return StartRunResponse{}, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return StartRunResponse{}, err
	}
	if bodyValue != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return StartRunResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return StartRunResponse{}, fmt.Errorf("harness_client: post %s: status %d", path, resp.StatusCode)
	}
	var result StartRunResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

// StreamRunEvents parses harnessd SSE blocks into typed events. It is shared by protocol adapters.
func (c *HarnessClient) StreamRunEvents(ctx context.Context, runID string, receive func(RunEvent) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/runs/"+url.PathEscape(runID)+"/events", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("harness_client: events status %d", resp.StatusCode)
	}
	s := bufio.NewScanner(resp.Body)
	var typ string
	var data []byte
	for s.Scan() {
		line := s.Text()
		if line == "" {
			if typ != "" {
				var payload map[string]any
				if err := json.Unmarshal(data, &payload); err != nil {
					return err
				}
				if err := receive(RunEvent{Type: typ, Data: payload}); err != nil {
					return err
				}
				typ = ""
				data = nil
			}
			continue
		}
		if len(line) > 7 && line[:7] == "event: " {
			typ = line[7:]
		}
		if len(line) > 6 && line[:6] == "data: " {
			data = append(data, line[6:]...)
		}
	}
	return s.Err()
}

// NewHarnessClient creates a new HarnessClient pointing at baseURL.
func NewHarnessClient(baseURL string) *HarnessClient {
	return &HarnessClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// StartRunRequest is the request body for POST /v1/runs.
//
// Every field beyond Prompt is omitempty: a caller that sends only a prompt must
// post exactly what it posted before this struct grew, so nobody is silently
// opted into isolation or a restricted tool set (issue #1316).
type StartRunRequest struct {
	Prompt         string  `json:"prompt"`
	Model          string  `json:"model,omitempty"`
	ConversationID string  `json:"conversation_id,omitempty"`
	MaxSteps       int     `json:"max_steps,omitempty"`
	MaxCostUSD     float64 `json:"max_cost_usd,omitempty"`

	// WorkspaceType selects the workspace backend: "local", "worktree",
	// "container", or "vm". Empty uses the daemon's own workspace, which is the
	// pre-existing behavior. "worktree" is what makes a delegated write safe.
	WorkspaceType string `json:"workspace_type,omitempty"`
	// ExtraDirs grants read/work access to directory roots beyond the workspace.
	ExtraDirs []string `json:"extra_dirs,omitempty"`
	// AllowedTools restricts the run to the named tools; DeniedTools removes
	// specific ones. Empty means no restriction.
	AllowedTools []string `json:"allowed_tools,omitempty"`
	DeniedTools  []string `json:"denied_tools,omitempty"`
	// ProfileName selects a server-side profile (tool set, isolation, limits).
	ProfileName string `json:"profile,omitempty"`

	SystemPrompt    string `json:"system_prompt,omitempty"`
	ProviderName    string `json:"provider_name,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	MaxTurns        int    `json:"max_turns,omitempty"`
	PlanMode        bool   `json:"plan_mode,omitempty"`
	PlanFile        string `json:"plan_file,omitempty"`
	AgentIntent     string `json:"agent_intent,omitempty"`
	TaskContext     string `json:"task_context,omitempty"`
}

// StartRunResponse is the response body from POST /v1/runs.
type StartRunResponse struct {
	RunID string `json:"run_id"`
}

// RunStatus is the full run state returned by GET /v1/runs/{id}.
//
// The JSON tags mirror what the server actually emits. They previously named
// fields the server does not send (run_id, cost_usd, messages), and because
// encoding/json leaves absent fields at their zero value, the proxy silently
// dropped every run's output and reported every run as free (issue #1314).
type RunStatus struct {
	RunID          string `json:"id"`
	Status         string `json:"status"`
	ConversationID string `json:"conversation_id"`
	// Output is the run's result text. The server sends a single string, not a
	// message list.
	Output   string    `json:"output"`
	Messages []Message `json:"messages"`
	// CostTotals carries the run's spend; CostUSD is projected from it after
	// decoding, since the server nests it rather than exposing a flat field.
	CostTotals runCostTotals `json:"cost_totals"`
	CostUSD    float64       `json:"-"`
	Error      string        `json:"error,omitempty"`
}

// runCostTotals is the server's nested cost object.
type runCostTotals struct {
	CostUSDTotal float64 `json:"cost_usd_total"`
}

// Message is a single message in a run's conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RunSummary is a summary of a run, as returned by list_runs.
type RunSummary struct {
	RunID   string  `json:"id"`
	Status  string  `json:"status"`
	CostUSD float64 `json:"-"`
}

// ListRunsParams are the query parameters for GET /v1/runs.
type ListRunsParams struct {
	ConversationID string
	Limit          int
}

// StartRun calls POST /v1/runs and returns the new run ID.
func (c *HarnessClient) StartRun(ctx context.Context, req StartRunRequest) (StartRunResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return StartRunResponse{}, fmt.Errorf("harness_client: marshal start run request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/runs", bytes.NewReader(body))
	if err != nil {
		return StartRunResponse{}, fmt.Errorf("harness_client: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return StartRunResponse{}, fmt.Errorf("harness_client: post /v1/runs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return StartRunResponse{}, fmt.Errorf("harness_client: post /v1/runs: status %d: %v", resp.StatusCode, errBody)
	}

	var result StartRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return StartRunResponse{}, fmt.Errorf("harness_client: decode start run response: %w", err)
	}
	return result, nil
}

// GetRun calls GET /v1/runs/{runID} and returns the run status.
func (c *HarnessClient) GetRun(ctx context.Context, runID string) (RunStatus, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/runs/"+url.PathEscape(runID), nil)
	if err != nil {
		return RunStatus{}, fmt.Errorf("harness_client: create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return RunStatus{}, fmt.Errorf("harness_client: get /v1/runs/%s: %w", runID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return RunStatus{}, fmt.Errorf("harness_client: get /v1/runs/%s: status %d: %v", runID, resp.StatusCode, errBody)
	}

	var result RunStatus
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return RunStatus{}, fmt.Errorf("harness_client: decode run status: %w", err)
	}
	// The server nests cost; flatten it so callers keep one field to read.
	result.CostUSD = result.CostTotals.CostUSDTotal
	return result, nil
}

// ListRuns calls GET /v1/runs with optional filters and returns a list of run summaries.
func (c *HarnessClient) ListRuns(ctx context.Context, params ListRunsParams) ([]RunSummary, error) {
	u, err := url.Parse(c.baseURL + "/v1/runs")
	if err != nil {
		return nil, fmt.Errorf("harness_client: parse url: %w", err)
	}

	q := u.Query()
	if params.ConversationID != "" {
		q.Set("conversation_id", params.ConversationID)
	}
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("harness_client: create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("harness_client: get /v1/runs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("harness_client: get /v1/runs: status %d: %v", resp.StatusCode, errBody)
	}

	// The server returns {"runs": [...]} with full run objects.
	// We project each to a RunSummary.
	var result struct {
		Runs []struct {
			RunID      string        `json:"id"`
			Status     string        `json:"status"`
			CostTotals runCostTotals `json:"cost_totals"`
		} `json:"runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("harness_client: decode list runs response: %w", err)
	}

	summaries := make([]RunSummary, 0, len(result.Runs))
	for _, r := range result.Runs {
		summaries = append(summaries, RunSummary{
			RunID:   r.RunID,
			Status:  r.Status,
			CostUSD: r.CostTotals.CostUSDTotal,
		})
	}
	return summaries, nil
}

// SteerRun injects a guidance message into an in-flight run.
func (c *HarnessClient) SteerRun(ctx context.Context, runID, prompt string) error {
	_, err := c.postRun(ctx, "/v1/runs/"+url.PathEscape(runID)+"/steer",
		map[string]string{"prompt": prompt})
	return err
}

// CatalogModel is one entry from GET /v1/models.
type CatalogModel struct {
	ID            string `json:"id"`
	Provider      string `json:"provider"`
	ContextWindow int    `json:"context_window,omitempty"`
}

// CatalogProvider is one entry from GET /v1/providers. It reports whether a
// credential exists and whether it is known to work — a provider can be
// configured and still fail, so both are returned.
type CatalogProvider struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	Health     string `json:"health"`
	ModelCount int    `json:"model_count"`
}

// ListModels calls GET /v1/models so a caller can discover what is routable
// without shelling out to curl.
func (c *HarnessClient) ListModels(ctx context.Context) ([]CatalogModel, error) {
	var body struct {
		Models []CatalogModel `json:"models"`
	}
	if err := c.getJSON(ctx, "/v1/models", &body); err != nil {
		return nil, err
	}
	return body.Models, nil
}

// ListProviders calls GET /v1/providers.
func (c *HarnessClient) ListProviders(ctx context.Context) ([]CatalogProvider, error) {
	var body struct {
		Providers []CatalogProvider `json:"providers"`
	}
	if err := c.getJSON(ctx, "/v1/providers", &body); err != nil {
		return nil, err
	}
	return body.Providers, nil
}

// getJSON performs a GET and decodes a JSON body into out.
func (c *HarnessClient) getJSON(ctx context.Context, path string, out any) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("harness_client: create request: %w", err)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("harness_client: get %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return fmt.Errorf("harness_client: get %s: status %d: %v", path, resp.StatusCode, errBody)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("harness_client: decode %s response: %w", path, err)
	}
	return nil
}
