package cron

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is an HTTP client for the cronsd API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// ClientOption configures a cronsd client.
type ClientOption func(*Client)

// WithAPIKey authenticates management requests to cronsd. The credential is
// transmitted only in the Authorization header.
func WithAPIKey(apiKey string) ClientOption {
	return func(client *Client) {
		client.apiKey = apiKey
	}
}

// GetJobByName performs the distinct operator lookup. Model-facing callers use
// GetJob, whose route is ID-only.
func (c *Client) GetJobByName(ctx context.Context, name string) (Job, error) {
	if name == "" {
		return Job{}, fmt.Errorf("name is required")
	}
	query := url.Values{"name": []string{name}}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/jobs/by-name?"+query.Encode(), nil)
	if err != nil {
		return Job{}, fmt.Errorf("create request: %w", err)
	}
	withScopeHeaders(httpReq)
	c.authorize(httpReq)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Job{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Job{}, c.parseError(resp)
	}
	var job Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return Job{}, fmt.Errorf("decode response: %w", err)
	}
	return job, nil
}

func withScopeHeaders(req *http.Request) {
	if scope, ok := ScopeFromContext(req.Context()); ok {
		req.Header.Set("X-Cron-Tenant-ID", scope.TenantID)
		req.Header.Set("X-Cron-Conversation-ID", scope.ConversationID)
		req.Header.Set("X-Cron-Agent-ID", scope.AgentID)
	}
}

// NewClient creates a new Client for the given base URL.
func NewClient(baseURL string, options ...ClientOption) *Client {
	client := &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
	for _, option := range options {
		if option != nil {
			option(client)
		}
	}
	return client
}

func (c *Client) authorize(request *http.Request) {
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

// CreateJob creates a new cron job.
func (c *Client) CreateJob(ctx context.Context, req CreateJobRequest) (Job, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return Job{}, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/jobs", bytes.NewReader(body))
	if err != nil {
		return Job{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	withScopeHeaders(httpReq)
	c.authorize(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Job{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return Job{}, c.parseError(resp)
	}

	var job Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return Job{}, fmt.Errorf("decode response: %w", err)
	}
	return job, nil
}

// ListJobs returns all cron jobs.
func (c *Client) ListJobs(ctx context.Context) ([]Job, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/jobs", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	withScopeHeaders(httpReq)
	c.authorize(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Jobs, nil
}

// GetJob retrieves a cron job by ID. Operator name lookup is GetJobByName.
func (c *Client) GetJob(ctx context.Context, id string) (Job, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/jobs/"+id, nil)
	if err != nil {
		return Job{}, fmt.Errorf("create request: %w", err)
	}
	withScopeHeaders(httpReq)
	c.authorize(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Job{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Job{}, c.parseError(resp)
	}

	var job Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return Job{}, fmt.Errorf("decode response: %w", err)
	}
	return job, nil
}

// UpdateJob updates a cron job.
func (c *Client) UpdateJob(ctx context.Context, id string, req UpdateJobRequest) (Job, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return Job{}, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+"/v1/jobs/"+id, bytes.NewReader(body))
	if err != nil {
		return Job{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	withScopeHeaders(httpReq)
	c.authorize(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Job{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Job{}, c.parseError(resp)
	}

	var job Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return Job{}, fmt.Errorf("decode response: %w", err)
	}
	return job, nil
}

// DeleteJob deletes a cron job.
func (c *Client) DeleteJob(ctx context.Context, id string) error {
	return c.deleteJob(ctx, id, nil)
}

// DeleteJobCAS deletes only when updated_at still matches the version read by
// the caller. DeleteJob remains the unversioned operator API.
func (c *Client) DeleteJobCAS(ctx context.Context, id string, expectedUpdatedAt time.Time) error {
	body, err := json.Marshal(DeleteJobRequest{ExpectedUpdatedAt: &expectedUpdatedAt})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	return c.deleteJob(ctx, id, body)
}

func (c *Client) deleteJob(ctx context.Context, id string, body []byte) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/v1/jobs/"+id, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	withScopeHeaders(httpReq)
	c.authorize(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}
	return nil
}

// ListExecutions returns execution history for a job.
func (c *Client) ListExecutions(ctx context.Context, jobID string, limit, offset int) ([]Execution, error) {
	url := fmt.Sprintf("%s/v1/jobs/%s/history?limit=%d&offset=%d", c.baseURL, jobID, limit, offset)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	withScopeHeaders(httpReq)
	c.authorize(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Executions []Execution `json:"executions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Executions, nil
}

// Health checks authenticated readiness. The unauthenticated /healthz route
// is liveness-only and cannot prove that management requests are usable.
func (c *Client) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/readyz", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	c.authorize(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readiness check failed: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) parseError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		if resp.StatusCode == http.StatusNotFound && errResp.Error.Code == "not_found" {
			return ErrJobNotFound
		}
		if resp.StatusCode == http.StatusConflict && errResp.Error.Code == "conflict" {
			return ErrJobConflict
		}
		if resp.StatusCode == http.StatusConflict && errResp.Error.Code == "ambiguous" {
			return ErrJobAmbiguous
		}
		return fmt.Errorf("HTTP %d: %s: %s", resp.StatusCode, errResp.Error.Code, errResp.Error.Message)
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}
