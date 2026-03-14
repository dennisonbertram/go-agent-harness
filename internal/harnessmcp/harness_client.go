package harnessmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// HarnessClient is an HTTP client for the harnessd REST API.
type HarnessClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHarnessClient creates a new HarnessClient pointing at baseURL.
func NewHarnessClient(baseURL string) *HarnessClient {
	return &HarnessClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// StartRunRequest is the request body for POST /v1/runs.
type StartRunRequest struct {
	Prompt         string  `json:"prompt"`
	Model          string  `json:"model,omitempty"`
	ConversationID string  `json:"conversation_id,omitempty"`
	MaxSteps       int     `json:"max_steps,omitempty"`
	MaxCostUSD     float64 `json:"max_cost_usd,omitempty"`
}

// StartRunResponse is the response body from POST /v1/runs.
type StartRunResponse struct {
	RunID string `json:"run_id"`
}

// RunStatus is the full run state returned by GET /v1/runs/{id}.
type RunStatus struct {
	RunID          string    `json:"run_id"`
	Status         string    `json:"status"`
	ConversationID string    `json:"conversation_id"`
	Messages       []Message `json:"messages"`
	CostUSD        float64   `json:"cost_usd"`
	Error          string    `json:"error,omitempty"`
}

// Message is a single message in a run's conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RunSummary is a summary of a run, as returned by list_runs.
type RunSummary struct {
	RunID   string  `json:"run_id"`
	Status  string  `json:"status"`
	CostUSD float64 `json:"cost_usd"`
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
			RunID   string  `json:"run_id"`
			Status  string  `json:"status"`
			CostUSD float64 `json:"cost_usd"`
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
			CostUSD: r.CostUSD,
		})
	}
	return summaries, nil
}
