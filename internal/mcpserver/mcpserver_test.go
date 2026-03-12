// Package mcpserver exposes the agent harness as an MCP server over HTTP.
// Tests use in-process HTTP test servers to verify JSON-RPC 2.0 protocol.
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- fake implementations ---

// fakeRunnerRun is a harness run returned by the fake runner.
type fakeRunnerRun struct {
	ID     string
	Status string
	Output string
	Error  string
}

// fakeRunner implements RunnerInterface for testing.
type fakeRunner struct {
	mu      sync.Mutex
	runs    map[string]*fakeRunnerRun
	nextID  int
	startFn func(prompt string) (string, error) // optional override
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{
		runs: make(map[string]*fakeRunnerRun),
	}
}

func (f *fakeRunner) StartRun(prompt string) (string, error) {
	if f.startFn != nil {
		return f.startFn(prompt)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := fmt.Sprintf("run-%d", f.nextID)
	f.runs[id] = &fakeRunnerRun{
		ID:     id,
		Status: "running",
	}
	return id, nil
}

func (f *fakeRunner) GetRunStatus(runID string) (RunStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[runID]
	if !ok {
		return RunStatus{}, fmt.Errorf("run %q not found", runID)
	}
	return RunStatus{
		ID:     r.ID,
		Status: r.Status,
		Output: r.Output,
		Error:  r.Error,
	}, nil
}

func (f *fakeRunner) ListRuns() ([]RunStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]RunStatus, 0, len(f.runs))
	for _, r := range f.runs {
		out = append(out, RunStatus{
			ID:     r.ID,
			Status: r.Status,
			Output: r.Output,
			Error:  r.Error,
		})
	}
	return out, nil
}

// helper: complete a run in the fake runner
func (f *fakeRunner) completeRun(id, output string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.runs[id]; ok {
		r.Status = "completed"
		r.Output = output
	}
}

// --- JSON-RPC helpers ---

type jsonRPCReq struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      int             `json:"id"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCErrObj  `json:"error,omitempty"`
}

type jsonRPCErrObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func doRPC(t *testing.T, srv *httptest.Server, method string, id int, params any) jsonRPCResp {
	t.Helper()
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		paramsRaw = b
	}
	req := jsonRPCReq{
		JSONRPC: "2.0",
		Method:  method,
		ID:      id,
		Params:  paramsRaw,
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal req: %v", err)
	}
	resp, err := http.Post(srv.URL+"/mcp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http post: %v", err)
	}
	defer resp.Body.Close()
	var rpcResp jsonRPCResp
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rpcResp
}

// --- tests ---

// TestServer_Initialize verifies the MCP initialize handshake.
func TestServer_Initialize(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "initialize", 1, map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo":      map[string]any{"name": "test", "version": "1"},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocolVersion 2024-11-05, got %q", result.ProtocolVersion)
	}
	if result.ServerInfo.Name == "" {
		t.Error("expected non-empty server name")
	}
}

// TestServer_ListTools verifies that tools/list returns start_run, get_run_status, list_runs.
func TestServer_ListTools(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/list", 2, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}

	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	required := []string{"start_run", "get_run_status", "list_runs"}
	for _, name := range required {
		if !toolNames[name] {
			t.Errorf("expected tool %q in tools/list, got: %v", name, result.Tools)
		}
	}

	// Each tool must have a non-empty description.
	for _, tool := range result.Tools {
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %q has nil inputSchema", tool.Name)
		}
	}
}

// TestServer_StartRun verifies the start_run tool.
func TestServer_StartRun(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/call", 3, map[string]any{
		"name": "start_run",
		"arguments": map[string]any{
			"prompt": "hello world",
		},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error.Message)
	}

	result := extractToolCallText(t, resp.Result)
	if result == "" {
		t.Fatal("expected non-empty tool call result")
	}
	// Result should contain the run ID.
	if !strings.Contains(result, "run-1") {
		t.Errorf("expected result to contain run ID, got %q", result)
	}
}

// TestServer_StartRun_MissingPrompt verifies that start_run rejects missing prompt.
func TestServer_StartRun_MissingPrompt(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/call", 4, map[string]any{
		"name":      "start_run",
		"arguments": map[string]any{},
	})
	// Should succeed at RPC level but return an error in the tool result.
	if resp.Error != nil {
		t.Fatalf("unexpected RPC-level error: %v", resp.Error.Message)
	}
	result := extractToolCallText(t, resp.Result)
	if !strings.Contains(strings.ToLower(result), "prompt") {
		t.Errorf("expected error mentioning prompt, got %q", result)
	}
}

// TestServer_GetRunStatus verifies the get_run_status tool.
func TestServer_GetRunStatus(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	// Start a run first.
	startResp := doRPC(t, srv, "tools/call", 5, map[string]any{
		"name":      "start_run",
		"arguments": map[string]any{"prompt": "test"},
	})
	if startResp.Error != nil {
		t.Fatalf("start_run error: %v", startResp.Error.Message)
	}

	// Get its status.
	statusResp := doRPC(t, srv, "tools/call", 6, map[string]any{
		"name":      "get_run_status",
		"arguments": map[string]any{"run_id": "run-1"},
	})
	if statusResp.Error != nil {
		t.Fatalf("get_run_status error: %v", statusResp.Error.Message)
	}

	result := extractToolCallText(t, statusResp.Result)
	if !strings.Contains(result, "run-1") {
		t.Errorf("expected run ID in status result, got %q", result)
	}
	if !strings.Contains(result, "running") {
		t.Errorf("expected status 'running' in result, got %q", result)
	}
}

// TestServer_GetRunStatus_NotFound verifies error for unknown run.
func TestServer_GetRunStatus_NotFound(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/call", 7, map[string]any{
		"name":      "get_run_status",
		"arguments": map[string]any{"run_id": "nonexistent"},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error.Message)
	}
	result := extractToolCallText(t, resp.Result)
	if !strings.Contains(strings.ToLower(result), "not found") &&
		!strings.Contains(strings.ToLower(result), "error") {
		t.Errorf("expected not-found error in result, got %q", result)
	}
}

// TestServer_ListRuns verifies the list_runs tool.
func TestServer_ListRuns(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	// Start two runs.
	for i := 0; i < 2; i++ {
		doRPC(t, srv, "tools/call", 10+i, map[string]any{
			"name":      "start_run",
			"arguments": map[string]any{"prompt": fmt.Sprintf("run %d", i)},
		})
	}

	resp := doRPC(t, srv, "tools/call", 20, map[string]any{
		"name":      "list_runs",
		"arguments": map[string]any{},
	})
	if resp.Error != nil {
		t.Fatalf("list_runs error: %v", resp.Error.Message)
	}

	result := extractToolCallText(t, resp.Result)
	if result == "" {
		t.Fatal("expected non-empty list_runs result")
	}
	// Both run IDs should appear.
	if !strings.Contains(result, "run-1") || !strings.Contains(result, "run-2") {
		t.Errorf("expected both run IDs in result, got %q", result)
	}
}

// TestServer_ListRuns_Empty verifies list_runs returns a useful message when empty.
func TestServer_ListRuns_Empty(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/call", 21, map[string]any{
		"name":      "list_runs",
		"arguments": map[string]any{},
	})
	if resp.Error != nil {
		t.Fatalf("list_runs error: %v", resp.Error.Message)
	}
	result := extractToolCallText(t, resp.Result)
	if result == "" {
		t.Fatal("expected non-empty result even for empty list")
	}
}

// TestServer_UnknownTool verifies that unknown tool calls return an error result.
func TestServer_UnknownTool(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/call", 30, map[string]any{
		"name":      "nonexistent_tool",
		"arguments": map[string]any{},
	})
	// At JSON-RPC level this may succeed or fail; either way the tool result must
	// indicate an error.
	if resp.Error == nil {
		result := extractToolCallText(t, resp.Result)
		if !strings.Contains(strings.ToLower(result), "unknown") &&
			!strings.Contains(strings.ToLower(result), "not found") &&
			!strings.Contains(strings.ToLower(result), "error") {
			t.Errorf("expected error for unknown tool, got %q", result)
		}
	}
}

// TestServer_UnknownMethod verifies that unknown JSON-RPC methods return an error.
func TestServer_UnknownMethod(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "foo/bar", 31, nil)
	if resp.Error == nil {
		t.Error("expected JSON-RPC error for unknown method, got nil")
	}
}

// TestServer_Concurrent verifies thread safety under concurrent requests.
func TestServer_Concurrent(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	const n = 20
	var wg sync.WaitGroup
	errors := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp := doRPC(t, srv, "tools/call", i+100, map[string]any{
				"name":      "start_run",
				"arguments": map[string]any{"prompt": fmt.Sprintf("concurrent %d", i)},
			})
			if resp.Error != nil {
				errors <- fmt.Errorf("goroutine %d: %v", i, resp.Error.Message)
			}
		}(i)
	}
	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// All runs should be listed.
	listResp := doRPC(t, srv, "tools/call", 200, map[string]any{
		"name":      "list_runs",
		"arguments": map[string]any{},
	})
	if listResp.Error != nil {
		t.Fatalf("list_runs: %v", listResp.Error.Message)
	}
	result := extractToolCallText(t, listResp.Result)
	for i := 1; i <= n; i++ {
		if !strings.Contains(result, fmt.Sprintf("run-%d", i)) {
			t.Errorf("expected run-%d in list, result snippet: %q", i, result[:min(len(result), 200)])
		}
	}
}

// TestServer_RunnerError verifies that runner errors are surfaced as tool errors.
func TestServer_RunnerError(t *testing.T) {
	runner := newFakeRunner()
	runner.startFn = func(prompt string) (string, error) {
		return "", fmt.Errorf("runner is overloaded")
	}
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/call", 40, map[string]any{
		"name":      "start_run",
		"arguments": map[string]any{"prompt": "test"},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected RPC-level error: %v", resp.Error.Message)
	}
	result := extractToolCallText(t, resp.Result)
	if !strings.Contains(result, "overloaded") {
		t.Errorf("expected runner error in result, got %q", result)
	}
}

// TestServer_Shutdown verifies that the server shuts down cleanly.
func TestServer_Shutdown(t *testing.T) {
	runner := newFakeRunner()
	s := NewServer(runner)
	httpSrv := httptest.NewServer(s.Handler())

	// Make a request while running.
	resp := doRPC(t, httpSrv, "tools/list", 1, nil)
	if resp.Error != nil {
		t.Fatalf("tools/list error before shutdown: %v", resp.Error.Message)
	}

	// Shut down.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	httpSrv.Close()
	if err := s.Shutdown(ctx); err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
}

// TestServer_InvalidJSON verifies graceful handling of malformed requests.
func TestServer_InvalidJSON(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/mcp", "application/json", strings.NewReader("{invalid json"))
	if err != nil {
		t.Fatalf("http post: %v", err)
	}
	defer resp.Body.Close()

	var rpcResp jsonRPCResp
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rpcResp.Error == nil {
		t.Error("expected JSON-RPC parse error for invalid JSON input")
	}
}

// TestServer_GetRunStatus_MissingRunID verifies validation of missing run_id.
func TestServer_GetRunStatus_MissingRunID(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/call", 50, map[string]any{
		"name":      "get_run_status",
		"arguments": map[string]any{},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error.Message)
	}
	result := extractToolCallText(t, resp.Result)
	if !strings.Contains(strings.ToLower(result), "run_id") &&
		!strings.Contains(strings.ToLower(result), "required") &&
		!strings.Contains(strings.ToLower(result), "error") {
		t.Errorf("expected validation error for missing run_id, got %q", result)
	}
}

// TestServer_NotificationIgnored verifies that JSON-RPC notifications (no ID)
// return HTTP 200 with no body or an empty response.
func TestServer_NotificationIgnored(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	// A notification has no "id" field.
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	body, _ := json.Marshal(notif)
	resp, err := http.Post(srv.URL+"/mcp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http post: %v", err)
	}
	defer resp.Body.Close()
	// HTTP 200 is acceptable for a notification (no response body required).
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 200 or 204 for notification, got %d", resp.StatusCode)
	}
}

// --- helpers ---

// extractToolCallText decodes the MCP tools/call result and returns concatenated text.
func extractToolCallText(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	if raw == nil {
		return ""
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		// May not be a tools/call result — return raw string.
		return string(raw)
	}
	var sb strings.Builder
	for _, c := range result.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestTruncate verifies the truncate helper handles both short and long strings.
func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("short string: got %q, want %q", got, "hello")
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("long string: got %q, want %q", got, "hello...")
	}
	if got := truncate("", 5); got != "" {
		t.Errorf("empty string: got %q, want %q", got, "")
	}
	// Unicode: "日本語" is 3 runes, truncate at 2 should produce "日本..."
	if got := truncate("日本語", 2); got != "日本..." {
		t.Errorf("unicode: got %q, want %q", got, "日本...")
	}
}
