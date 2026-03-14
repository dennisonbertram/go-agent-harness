package harnessmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestStartRunHandler_MissingPrompt verifies that start_run returns isError when prompt is missing.
func TestStartRunHandler_MissingPrompt(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	handler := newStartRunHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{"model":"gpt-4"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for missing prompt")
	}
}

// TestStartRunHandler_BadJSON verifies that start_run returns isError for malformed args.
func TestStartRunHandler_BadJSON(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	handler := newStartRunHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for bad JSON")
	}
}

// TestStartRunHandler_HTTPError verifies start_run returns isError when harnessd returns error.
func TestStartRunHandler_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	handler := newStartRunHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{"prompt":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for HTTP error")
	}
}

// TestGetRunStatusHandler_MissingRunID verifies get_run_status returns isError when run_id is missing.
func TestGetRunStatusHandler_MissingRunID(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	handler := newGetRunStatusHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for missing run_id")
	}
}

// TestGetRunStatusHandler_BadJSON verifies get_run_status returns isError for malformed args.
func TestGetRunStatusHandler_BadJSON(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	handler := newGetRunStatusHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for bad JSON")
	}
}

// TestGetRunStatusHandler_HTTPError verifies get_run_status returns isError when harnessd returns error.
func TestGetRunStatusHandler_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	handler := newGetRunStatusHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{"run_id":"run-x"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for HTTP 404")
	}
}

// TestWaitForRunHandler_MissingRunID verifies wait_for_run returns isError when run_id is missing.
func TestWaitForRunHandler_MissingRunID(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	handler := newWaitForRunHandler(client, RealClock{})

	result, err := handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for missing run_id")
	}
}

// TestWaitForRunHandler_BadJSON verifies wait_for_run returns isError for malformed args.
func TestWaitForRunHandler_BadJSON(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	handler := newWaitForRunHandler(client, RealClock{})

	result, err := handler(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for bad JSON")
	}
}

// TestWaitForRunHandler_ContextCancel verifies wait_for_run returns isError when context is cancelled.
// We cancel the context before calling the handler so that either GetRun fails with
// context error or the select fires ctx.Done — either way isError should be true.
func TestWaitForRunHandler_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(RunStatus{Status: "running"})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	clock := &mockClockNever{}
	handler := newWaitForRunHandler(client, clock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately — will be detected either in GetRun or in select

	result, err := handler(ctx, json.RawMessage(`{"run_id":"run-cancel","timeout_seconds":300}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for cancelled context")
	}
	// Either "cancelled" (select path) or a context error from GetRun — both are valid.
	if len(result.Content) == 0 {
		t.Fatal("expected content in error result")
	}
}

// TestWaitForRunHandler_HTTPError verifies wait_for_run returns isError when GetRun fails.
func TestWaitForRunHandler_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	handler := newWaitForRunHandler(client, RealClock{})

	result, err := handler(context.Background(), json.RawMessage(`{"run_id":"run-err","timeout_seconds":10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for HTTP error")
	}
}

// TestContinueRunHandler_MissingRunID verifies continue_run returns isError when run_id is missing.
func TestContinueRunHandler_MissingRunID(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	handler := newContinueRunHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{"message":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for missing run_id")
	}
}

// TestContinueRunHandler_MissingMessage verifies continue_run returns isError when message is missing.
func TestContinueRunHandler_MissingMessage(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	handler := newContinueRunHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{"run_id":"run-1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for missing message")
	}
}

// TestContinueRunHandler_BadJSON verifies continue_run returns isError for malformed args.
func TestContinueRunHandler_BadJSON(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	handler := newContinueRunHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for bad JSON")
	}
}

// TestContinueRunHandler_GetRunError verifies continue_run returns isError when GetRun fails.
func TestContinueRunHandler_GetRunError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	handler := newContinueRunHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{"run_id":"run-x","message":"hi"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true when GetRun fails")
	}
}

// TestContinueRunHandler_StartRunError verifies continue_run returns isError when StartRun fails.
func TestContinueRunHandler_StartRunError(t *testing.T) {
	getCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			getCount++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(RunStatus{
				RunID:          "run-x",
				Status:         "completed",
				ConversationID: "conv-1",
			})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	handler := newContinueRunHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{"run_id":"run-x","message":"hi"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true when StartRun fails")
	}
}

// TestListRunsHandler_WithConversationAndLimit verifies list_runs passes correct params.
func TestListRunsHandler_WithConversationAndLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("conversation_id") != "conv-list" {
			t.Errorf("got conversation_id %q, want %q", q.Get("conversation_id"), "conv-list")
		}
		if q.Get("limit") != "5" {
			t.Errorf("got limit %q, want %q", q.Get("limit"), "5")
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"runs": []map[string]any{
				{"run_id": "r1", "status": "completed", "cost_usd": 0.1},
			},
		})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	handler := newListRunsHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{"conversation_id":"conv-list","limit":5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error in result: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}

	var runs []RunSummary
	if err := json.Unmarshal([]byte(result.Content[0].Text), &runs); err != nil {
		t.Fatalf("parse content: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("got %d runs, want 1", len(runs))
	}
}

// TestListRunsHandler_Defaults verifies list_runs defaults limit to 20.
func TestListRunsHandler_Defaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "20" {
			t.Errorf("got limit %q, want %q", q.Get("limit"), "20")
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"runs": []any{}})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	handler := newListRunsHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}
}

// TestListRunsHandler_BadJSON verifies list_runs returns isError for malformed args.
func TestListRunsHandler_BadJSON(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	handler := newListRunsHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for bad JSON")
	}
}

// TestListRunsHandler_HTTPError verifies list_runs returns isError when harnessd returns error.
func TestListRunsHandler_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	handler := newListRunsHandler(client)

	result, err := handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for HTTP error")
	}
}

// TestListRunsHandler_NilArgs verifies list_runs handles nil args gracefully.
func TestListRunsHandler_NilArgs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"runs": []any{}})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	handler := newListRunsHandler(client)

	result, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}
}

// TestRealClock exercises the RealClock implementation for coverage.
func TestRealClock(t *testing.T) {
	c := RealClock{}
	now := c.Now()
	if now.IsZero() {
		t.Error("expected non-zero time")
	}

	ch := c.After(1) // 1 nanosecond
	select {
	case <-ch:
		// fired as expected
	case <-context.Background().Done():
		t.Error("context done unexpectedly")
	}
}

// TestToolCallParams_MissingArgs verifies dispatcher handles tools/call with missing arguments.
func TestToolCallParams_MissingArgs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"run_id": "run-noargs"})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	d := NewDispatcher(client, RealClock{})

	// Call tools/call without arguments field — but with a valid tool name.
	// The handler should set args to {} and fail validation for missing prompt.
	idRaw := json.RawMessage(`20`)
	req := Request{
		JSONRPC: "2.0",
		ID:      &idRaw,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"start_run"}`),
	}

	resp, _ := d.Dispatch(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error)
	}
	var result ToolResult
	_ = json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Error("expected isError=true for missing required prompt")
	}
}

// TestToolCallParams_InvalidJSON verifies dispatcher handles tools/call with invalid params.
func TestToolCallParams_InvalidJSON(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	d := NewDispatcher(client, RealClock{})

	idRaw := json.RawMessage(`21`)
	req := Request{
		JSONRPC: "2.0",
		ID:      &idRaw,
		Method:  "tools/call",
		Params:  json.RawMessage(`{bad json`),
	}

	resp, _ := d.Dispatch(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error)
	}
	var result ToolResult
	_ = json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Error("expected isError=true for invalid params JSON")
	}
}

// TestDispatch_Notification_WithID_Unknown tests an unknown method with an ID — should respond.
func TestDispatch_Notification_WithID_Unknown(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	d := NewDispatcher(client, RealClock{})

	idRaw := json.RawMessage(`42`)
	req := Request{
		JSONRPC: "2.0",
		ID:      &idRaw,
		Method:  "nope/nope",
	}

	resp, shouldRespond := d.Dispatch(context.Background(), req)
	if !shouldRespond {
		t.Error("expected shouldRespond=true for unknown method with ID")
	}
	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("got code %d, want -32601", resp.Error.Code)
	}
}

// TestTransport_EmptyLines verifies empty lines are skipped gracefully.
func TestTransport_EmptyLines(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	d := NewDispatcher(client, RealClock{})

	// Mix empty lines with a valid request.
	input := "\n\n" + `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n\n"
	in := strings.NewReader(input)
	var out strings.Builder

	transport := NewStdioTransport(in, &out, d)
	if err := transport.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(out.String(), `"id":1`) {
		t.Errorf("expected response with id=1, got %q", out.String())
	}
}

// mockClockNever is a clock where After() channels never fire.
type mockClockNever struct{}

func (m *mockClockNever) Now() time.Time { return time.Now() }
func (m *mockClockNever) After(d time.Duration) <-chan time.Time {
	return make(chan time.Time) // never fires
}
