package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// runListResponse is the JSON shape returned by GET /v1/runs.
type runListResponse struct {
	Runs []runRecord `json:"runs"`
}

// runRecord is a single run entry from the list or get-by-ID response.
type runRecord struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id,omitempty"`
	TenantID       string    `json:"tenant_id,omitempty"`
	Model          string    `json:"model,omitempty"`
	Prompt         string    `json:"prompt,omitempty"`
	Status         string    `json:"status"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// runList implements "harnesscli list".
// Sends GET /v1/runs (optionally filtered) and prints a table.
func runList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseURL := fs.String("base-url", "http://localhost:8080", "harness API base URL")
	statusFilter := fs.String("status", "", "filter by status (queued, running, completed, failed)")
	convID := fs.String("conversation-id", "", "filter by conversation ID")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "harnesscli list: %v\n", err)
		return 1
	}

	endpoint := strings.TrimRight(*baseURL, "/") + "/v1/runs"
	qv := url.Values{}
	if *statusFilter != "" {
		qv.Set("status", *statusFilter)
	}
	if *convID != "" {
		qv.Set("conversation_id", *convID)
	}
	if len(qv) > 0 {
		endpoint += "?" + qv.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Fprintf(stderr, "harnesscli list: build request: %v\n", err)
		return 1
	}

	resp, err := requestHTTPClient.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "harnesscli list: request failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(stderr, "harnesscli list: read response: %v\n", err)
		return 1
	}

	if resp.StatusCode >= 300 {
		fmt.Fprintf(stderr, "harnesscli list: %v\n", formatAPIError(resp.StatusCode, body))
		return 1
	}

	var lr runListResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		fmt.Fprintf(stderr, "harnesscli list: decode response: %v\n", err)
		return 1
	}

	if len(lr.Runs) == 0 {
		fmt.Fprintln(stdout, "No runs found")
		return 0
	}

	// Print table header.
	fmt.Fprintf(stdout, "%-24s  %-18s  %-20s  %s\n", "ID", "STATUS", "MODEL", "PROMPT")
	fmt.Fprintf(stdout, "%s\n", strings.Repeat("-", 90))
	for _, r := range lr.Runs {
		prompt := r.Prompt
		if len(prompt) > 40 {
			prompt = prompt[:37] + "..."
		}
		model := r.Model
		if model == "" {
			model = "(default)"
		}
		fmt.Fprintf(stdout, "%-24s  %-18s  %-20s  %s\n", r.ID, r.Status, model, prompt)
	}
	return 0
}

// runCancel implements "harnesscli cancel <run-id>".
// Sends POST /v1/runs/{id}/cancel and reports success or failure.
func runCancel(args []string) int {
	fs := flag.NewFlagSet("cancel", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseURL := fs.String("base-url", "http://localhost:8080", "harness API base URL")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "harnesscli cancel: %v\n", err)
		return 1
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "harnesscli cancel: run ID is required")
		return 1
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "harnesscli cancel: too many arguments; accepts exactly one run ID")
		return 1
	}
	runID := fs.Arg(0)

	endpoint := strings.TrimRight(*baseURL, "/") + "/v1/runs/" + url.PathEscape(runID) + "/cancel"
	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		fmt.Fprintf(stderr, "harnesscli cancel: build request: %v\n", err)
		return 1
	}

	resp, err := requestHTTPClient.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "harnesscli cancel: request failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(stderr, "harnesscli cancel: read response: %v\n", err)
		return 1
	}

	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintf(stderr, "harnesscli cancel: run %q not found\n", runID)
		return 1
	}

	if resp.StatusCode >= 300 {
		fmt.Fprintf(stderr, "harnesscli cancel: %v\n", formatAPIError(resp.StatusCode, body))
		return 1
	}

	fmt.Fprintf(stdout, "Run %s cancelling\n", runID)
	return 0
}

// runStatus implements "harnesscli status <run-id>".
// Sends GET /v1/runs/{id} and prints run details.
func runStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseURL := fs.String("base-url", "http://localhost:8080", "harness API base URL")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "harnesscli status: %v\n", err)
		return 1
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "harnesscli status: run ID is required")
		return 1
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "harnesscli status: too many arguments; accepts exactly one run ID")
		return 1
	}
	runID := fs.Arg(0)

	endpoint := strings.TrimRight(*baseURL, "/") + "/v1/runs/" + url.PathEscape(runID)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Fprintf(stderr, "harnesscli status: build request: %v\n", err)
		return 1
	}

	resp, err := requestHTTPClient.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "harnesscli status: request failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(stderr, "harnesscli status: read response: %v\n", err)
		return 1
	}

	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintf(stderr, "harnesscli status: run %q not found\n", runID)
		return 1
	}

	if resp.StatusCode >= 300 {
		fmt.Fprintf(stderr, "harnesscli status: %v\n", formatAPIError(resp.StatusCode, body))
		return 1
	}

	var r runRecord
	if err := json.Unmarshal(body, &r); err != nil {
		fmt.Fprintf(stderr, "harnesscli status: decode response: %v\n", err)
		return 1
	}

	model := r.Model
	if model == "" {
		model = "(default)"
	}
	fmt.Fprintf(stdout, "ID:        %s\n", r.ID)
	fmt.Fprintf(stdout, "Status:    %s\n", r.Status)
	fmt.Fprintf(stdout, "Model:     %s\n", model)
	fmt.Fprintf(stdout, "Created:   %s\n", r.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(stdout, "Updated:   %s\n", r.UpdatedAt.Format(time.RFC3339))
	if r.Prompt != "" {
		prompt := r.Prompt
		if len(prompt) > 80 {
			prompt = prompt[:77] + "..."
		}
		fmt.Fprintf(stdout, "Prompt:    %s\n", prompt)
	}
	if r.Error != "" {
		fmt.Fprintf(stdout, "Error:     %s\n", r.Error)
	}
	return 0
}
