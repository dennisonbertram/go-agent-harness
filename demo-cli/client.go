package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Event mirrors harness.Event for SSE decoding.
type Event struct {
	ID        string         `json:"id"`
	RunID     string         `json:"run_id"`
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}

// Option is a single answer choice in a Question.
type Option struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// Question mirrors harness/tools.AskUserQuestion as returned by GET /v1/runs/{id}/input.
type Question struct {
	QuestionText string   `json:"question"`
	Options      []Option `json:"options"`
	MultiSelect  bool     `json:"multiSelect"`
}

// PendingInputResponse is the body returned by GET /v1/runs/{id}/input.
type PendingInputResponse struct {
	Questions []Question `json:"questions"`
}

// RunResponse is the body returned by POST /v1/runs.
type RunResponse struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

// Client talks to the harness HTTP API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	sseClient  *http.Client
}

// NewClient creates a Client targeting the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		sseClient: &http.Client{
			Transport: &http.Transport{
				IdleConnTimeout:       0,
				ResponseHeaderTimeout: 0,
				DisableKeepAlives:     false,
				Proxy:                 http.ProxyFromEnvironment,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          10,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}
}

// HealthCheck returns an error if the server is unreachable.
func (c *Client) HealthCheck() error {
	resp, err := c.httpClient.Get(c.baseURL + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// CreateRun starts a new run and returns the run metadata.
func (c *Client) CreateRun(prompt, model, conversationID string) (RunResponse, error) {
	body := map[string]any{
		"prompt": prompt,
	}
	if model != "" {
		body["model"] = model
	}
	if conversationID != "" {
		body["conversation_id"] = conversationID
	}

	data, err := json.Marshal(body)
	if err != nil {
		return RunResponse{}, fmt.Errorf("encode request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/v1/runs", "application/json", bytes.NewReader(data))
	if err != nil {
		return RunResponse{}, fmt.Errorf("post run: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return RunResponse{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return RunResponse{}, fmt.Errorf("server error %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var run RunResponse
	if err := json.Unmarshal(raw, &run); err != nil {
		return RunResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return run, nil
}

// StreamEvents subscribes to the SSE event stream for a run, calling handler
// for each event until a terminal event is received.
func (c *Client) StreamEvents(runID string, handler func(Event) error) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v1/runs/"+runID+"/events", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := c.sseClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect to event stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var lines []string
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			if len(lines) == 0 {
				continue
			}
			ev, done, err := parseSSEBlock(lines)
			if err != nil {
				lines = lines[:0]
				continue
			}
			if err := handler(ev); err != nil {
				return err
			}
			if done {
				return nil
			}
			lines = lines[:0]
			continue
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan stream: %w", err)
	}

	// Flush any trailing block.
	if len(lines) > 0 {
		ev, _, err := parseSSEBlock(lines)
		if err == nil {
			_ = handler(ev)
		}
	}

	return fmt.Errorf("stream ended before terminal event")
}

// GetPendingInput fetches the current pending user-input request for a run.
func (c *Client) GetPendingInput(runID string) (PendingInputResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v1/runs/" + runID + "/input")
	if err != nil {
		return PendingInputResponse{}, fmt.Errorf("get pending input: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return PendingInputResponse{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return PendingInputResponse{}, fmt.Errorf("server error %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var pending PendingInputResponse
	if err := json.Unmarshal(raw, &pending); err != nil {
		return PendingInputResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return pending, nil
}

// SubmitInput posts answers to a run's pending user-input request.
func (c *Client) SubmitInput(runID string, answers map[string]string) error {
	body := map[string]any{"answers": answers}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/v1/runs/"+runID+"/input", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("submit input: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

// terminalEvents is the set of event types that end a run's SSE stream.
var terminalEvents = map[string]bool{
	"run.completed": true,
	"run.failed":    true,
	"run.cancelled": true,
}

func parseSSEBlock(lines []string) (Event, bool, error) {
	var eventType, data string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data += strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	if eventType == "" || data == "" {
		return Event{}, false, fmt.Errorf("incomplete SSE block")
	}

	var ev Event
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		return Event{}, false, fmt.Errorf("decode event: %w", err)
	}
	if ev.Type == "" {
		ev.Type = eventType
	}

	return ev, terminalEvents[ev.Type], nil
}
