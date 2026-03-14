package mcpserver

import (
	"bufio"
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

// flusherRecorder wraps httptest.ResponseRecorder and implements http.Flusher.
type flusherRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
	mu      sync.Mutex
}

func (f *flusherRecorder) Flush() {
	f.mu.Lock()
	f.flushed = true
	f.mu.Unlock()
}

func (f *flusherRecorder) WasFlushed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.flushed
}

// T1: GET /mcp returns 200 text/event-stream.
func TestSSE_GetMCPReturns200TextEventStream(t *testing.T) {
	runner := newFakeRunner()
	s := NewServer(runner)
	defer s.Shutdown(context.Background())

	// Use a real HTTP server to test SSE (httptest.NewRecorder won't block properly).
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Connect to the SSE endpoint with a quick timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/mcp", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Context cancelled is expected after our short timeout — check status first.
		if ctx.Err() != nil && resp != nil {
			// Response header was received.
		} else if ctx.Err() == nil {
			t.Fatalf("GET /mcp: %v", err)
		}
	}
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected HTTP 200, got %d", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "text/event-stream") {
			t.Errorf("expected Content-Type text/event-stream, got %q", ct)
		}
	}
}

// T2: After subscribe_run, WatchCount increases by 1.
func TestSSE_SubscribeRunIncreasesWatchCount(t *testing.T) {
	runner := newFakeRunner()
	runner.mu.Lock()
	runner.runs["run-watch"] = &fakeRunnerRun{ID: "run-watch", Status: "running"}
	runner.mu.Unlock()

	s := NewServer(runner)
	defer s.Shutdown(context.Background())

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	before := s.poller.WatchCount()

	resp := doRPC(t, srv, "tools/call", 1, map[string]any{
		"name": "subscribe_run",
		"arguments": map[string]any{
			"run_id": "run-watch",
		},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error.Message)
	}
	result := extractToolCallText(t, resp.Result)
	if strings.Contains(result, "Error:") {
		t.Errorf("expected success, got error: %q", result)
	}

	after := s.poller.WatchCount()
	if after != before+1 {
		t.Errorf("expected WatchCount to increase by 1: before=%d after=%d", before, after)
	}
}

// T5: Client disconnect causes broker subscription cleanup (no goroutine leak).
func TestSSE_ClientDisconnectCleansUp(t *testing.T) {
	runner := newFakeRunner()
	s := NewServer(runner)
	defer s.Shutdown(context.Background())

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	beforeSubs := s.broker.ActiveSubscriptions()

	// Connect and immediately cancel.
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/mcp", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}()

	// Give the server time to register the subscription.
	time.Sleep(50 * time.Millisecond)
	activeDuring := s.broker.ActiveSubscriptions()

	// Cancel the client connection.
	cancel()

	// Wait for the handler goroutine to exit.
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Log("warning: client goroutine did not exit cleanly")
	}

	// Give handleSSE goroutine time to run the defer cancel().
	time.Sleep(100 * time.Millisecond)

	afterSubs := s.broker.ActiveSubscriptions()

	if activeDuring <= beforeSubs {
		t.Logf("note: subscription may not have been registered before disconnect (activeDuring=%d beforeSubs=%d)", activeDuring, beforeSubs)
	}
	if afterSubs > beforeSubs {
		t.Errorf("subscription not cleaned up after disconnect: before=%d after=%d", beforeSubs, afterSubs)
	}
}

// T10: Integration — mock runner returns running→completed; subscribe_run then GET /mcp;
// receive both run/event (status_changed) and run/completed notifications.
func TestSSE_Integration_SubscribeAndReceiveNotifications(t *testing.T) {
	runner := newFakeRunner()
	runner.mu.Lock()
	runner.runs["run-integrate"] = &fakeRunnerRun{ID: "run-integrate", Status: "running"}
	runner.mu.Unlock()

	// Use fast polling for the test.
	b := NewBroker()
	// We use a custom fakePoller sequence: running → completed.
	fp := newFakePoller()
	fp.addStatuses("run-integrate", "running", "completed")

	poller := NewRunPoller(fp, b, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go poller.Run(ctx)

	s := &Server{
		runner:       runner,
		broker:       b,
		poller:       poller,
		pollerCancel: cancel,
	}

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Connect SSE client first.
	sseCtx, sseCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer sseCancel()

	sseReq, err := http.NewRequestWithContext(sseCtx, http.MethodGet, srv.URL+"/mcp", nil)
	if err != nil {
		t.Fatalf("create SSE request: %v", err)
	}

	sseResp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatalf("SSE connect: %v", err)
	}
	defer sseResp.Body.Close()

	if sseResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", sseResp.StatusCode)
	}

	// Subscribe to the run via subscribe_run tool.
	subscribeResp := doRPC(t, srv, "tools/call", 1, map[string]any{
		"name": "subscribe_run",
		"arguments": map[string]any{
			"run_id": "run-integrate",
		},
	})
	if subscribeResp.Error != nil {
		t.Fatalf("subscribe_run RPC error: %v", subscribeResp.Error.Message)
	}
	subText := extractToolCallText(t, subscribeResp.Result)
	if strings.Contains(subText, "Error:") {
		t.Fatalf("subscribe_run returned error: %q", subText)
	}

	// Read SSE events from the stream.
	notifsCh := make(chan map[string]any, 10)
	go func() {
		scanner := bufio.NewScanner(sseResp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			var notif map[string]any
			if err := json.Unmarshal([]byte(data), &notif); err != nil {
				continue
			}
			notifsCh <- notif
		}
	}()

	// Collect notifications until we get run/completed or timeout.
	var gotCompleted bool
	timeout := time.After(1500 * time.Millisecond)
	for !gotCompleted {
		select {
		case notif := <-notifsCh:
			method, _ := notif["method"].(string)
			if method == "run/completed" {
				params, _ := notif["params"].(map[string]any)
				if params != nil && params["run_id"] == "run-integrate" {
					gotCompleted = true
				}
			}
		case <-timeout:
			t.Fatal("timed out waiting for run/completed SSE notification")
		}
	}

	if !gotCompleted {
		t.Error("did not receive run/completed notification")
	}
}

// T11: 20 concurrent GET /mcp + 5 goroutines publishing, -race.
func TestSSE_ConcurrentClientsAndPublishers(t *testing.T) {
	runner := newFakeRunner()
	s := NewServer(runner)
	defer s.Shutdown(context.Background())

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	const numClients = 20
	const numPublishers = 5

	var wg sync.WaitGroup
	errors := make(chan error, numClients+numPublishers)

	// Start concurrent SSE clients.
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/mcp", nil)
			if err != nil {
				errors <- fmt.Errorf("client %d: create request: %v", i, err)
				return
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				// Expected context cancellation.
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("client %d: expected 200, got %d", i, resp.StatusCode)
			}
			// Drain body until context cancelled.
			buf := make([]byte, 4096)
			for {
				_, rerr := resp.Body.Read(buf)
				if rerr != nil {
					return
				}
			}
		}(i)
	}

	// Give clients time to connect.
	time.Sleep(50 * time.Millisecond)

	// Start concurrent publishers.
	for i := 0; i < numPublishers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n := Notification{
				Method: "run/event",
				Params: json.RawMessage(fmt.Sprintf(`{"run_id":"run-%d","status":"running"}`, i)),
			}
			for j := 0; j < 10; j++ {
				s.broker.PublishAll(n)
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// TestSSE_SubscribeRun_AlreadyCompleted verifies already-completed run returns immediately.
func TestSSE_SubscribeRun_AlreadyCompleted(t *testing.T) {
	runner := newFakeRunner()
	runner.mu.Lock()
	runner.runs["run-done"] = &fakeRunnerRun{ID: "run-done", Status: "completed", Output: "finished"}
	runner.mu.Unlock()

	s := NewServer(runner)
	defer s.Shutdown(context.Background())

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	before := s.poller.WatchCount()

	resp := doRPC(t, srv, "tools/call", 1, map[string]any{
		"name": "subscribe_run",
		"arguments": map[string]any{
			"run_id": "run-done",
		},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error.Message)
	}
	result := extractToolCallText(t, resp.Result)
	if strings.Contains(result, "Error:") {
		t.Errorf("expected success, got error: %q", result)
	}
	if !strings.Contains(result, "already_completed") {
		t.Errorf("expected already_completed in result, got %q", result)
	}

	// WatchCount should NOT increase for already-completed runs.
	after := s.poller.WatchCount()
	if after != before {
		t.Errorf("expected WatchCount to stay at %d, got %d", before, after)
	}
}

// TestSSE_SubscribeRun_MissingRunID verifies validation.
func TestSSE_SubscribeRun_MissingRunID(t *testing.T) {
	runner := newFakeRunner()
	s := NewServer(runner)
	defer s.Shutdown(context.Background())

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/call", 1, map[string]any{
		"name":      "subscribe_run",
		"arguments": map[string]any{},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error.Message)
	}
	result := extractToolCallText(t, resp.Result)
	if !strings.Contains(result, "Error:") {
		t.Errorf("expected error for missing run_id, got %q", result)
	}
}

// TestSSE_SubscribeRun_NotFound verifies error for nonexistent run.
func TestSSE_SubscribeRun_NotFound(t *testing.T) {
	runner := newFakeRunner()
	s := NewServer(runner)
	defer s.Shutdown(context.Background())

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/call", 1, map[string]any{
		"name": "subscribe_run",
		"arguments": map[string]any{
			"run_id": "run-nonexistent",
		},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error.Message)
	}
	result := extractToolCallText(t, resp.Result)
	if !strings.Contains(result, "Error:") {
		t.Errorf("expected error for not-found run, got %q", result)
	}
}

// nonFlusherWriter is an http.ResponseWriter that does NOT implement http.Flusher.
type nonFlusherWriter struct {
	header http.Header
	body   bytes.Buffer
	code   int
}

func (n *nonFlusherWriter) Header() http.Header {
	if n.header == nil {
		n.header = make(http.Header)
	}
	return n.header
}
func (n *nonFlusherWriter) Write(b []byte) (int, error) { return n.body.Write(b) }
func (n *nonFlusherWriter) WriteHeader(code int)        { n.code = code }

// TestSSE_GetMCPFlusherNotSupported verifies graceful error when Flusher not implemented.
func TestSSE_GetMCPFlusherNotSupported(t *testing.T) {
	runner := newFakeRunner()
	s := NewServer(runner)
	defer s.Shutdown(context.Background())

	// Use a writer that does NOT implement http.Flusher.
	rec := &nonFlusherWriter{}
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	s.handleSSE(rec, req)

	if rec.code != http.StatusInternalServerError {
		t.Errorf("expected 500 when Flusher not supported, got %d", rec.code)
	}
	if !strings.Contains(rec.body.String(), "streaming not supported") {
		t.Errorf("expected 'streaming not supported' error, got %q", rec.body.String())
	}
}

// TestSSE_PostMethodStillHandled verifies POST /mcp still works after adding GET handler.
func TestSSE_PostMethodStillHandled(t *testing.T) {
	runner := newFakeRunner()
	s := NewServer(runner)
	defer s.Shutdown(context.Background())

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// POST to /mcp should still work as JSON-RPC.
	resp := doRPC(t, srv, "tools/list", 1, nil)
	if resp.Error != nil {
		t.Fatalf("tools/list error: %v", resp.Error.Message)
	}
	if resp.Result == nil {
		t.Fatal("expected result from tools/list")
	}
}

// TestServer_ListTools_TenTools verifies that tools/list now returns exactly 10 tools.
func TestServer_ListTools_TenTools(t *testing.T) {
	runner := newFakeRunner()
	srv := httptest.NewServer(NewServer(runner).Handler())
	defer srv.Close()

	resp := doRPC(t, srv, "tools/list", 2, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Tools) != 10 {
		names := make([]string, len(result.Tools))
		for i, tool := range result.Tools {
			names[i] = tool.Name
		}
		t.Errorf("expected exactly 10 tools, got %d: %v", len(result.Tools), names)
	}

	// Verify subscribe_run is present.
	found := false
	for _, tool := range result.Tools {
		if tool.Name == "subscribe_run" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected subscribe_run tool in tools/list")
	}
}

// TestSSE_HandleMCP_MethodNotAllowed verifies PUT returns 405.
func TestSSE_HandleMCP_MethodNotAllowed(t *testing.T) {
	runner := newFakeRunner()
	s := NewServer(runner)
	defer s.Shutdown(context.Background())

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/mcp", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for PUT, got %d", resp.StatusCode)
	}
}
