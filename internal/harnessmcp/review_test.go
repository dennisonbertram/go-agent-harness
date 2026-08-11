package harnessmcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestTransportStopsOnContextCancel — a cancelled context must end the read loop.
// `break` inside a select leaves the select, not the for, so the original check
// fell through and kept dispatching (issue #1319).
func TestTransportStopsOnContextCancel(t *testing.T) {
	// A reader that never reaches EOF, so only cancellation can stop the loop.
	pr, pw := io.Pipe()
	defer pw.Close()

	d := NewDispatcher(NewHarnessClient("http://127.0.0.1:1"), RealClock{})
	tr := NewStdioTransport(pr, io.Discard, d)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tr.Run(ctx) }()

	cancel()

	// Cancellation is observed at the next message boundary: scanner.Scan blocks
	// until a line or EOF arrives, so a cancelled context alone cannot wake it.
	// (In production stdin closes on shutdown, which ends the scan.) Feed lines
	// from a goroutine so this test cannot deadlock on the pipe once Run returns.
	go func() {
		for i := 0; i < 5; i++ {
			if _, err := pw.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")); err != nil {
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

// TestWaitForRunReturnsOnCancelledStatus — cancelled is terminal for the runner
// (isTerminalRunStatus), so the wait must end rather than poll to timeout.
func TestWaitForRunReturnsOnCancelledStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"run-c","status":"cancelled","output":""}`))
	}))
	defer srv.Close()

	h := newWaitForRunHandler(NewHarnessClient(srv.URL), RealClock{})
	done := make(chan ToolResult, 1)
	go func() {
		res, _ := h(context.Background(), json.RawMessage(`{"run_id":"run-c","timeout_seconds":30}`))
		done <- res
	}()

	select {
	case res := <-done:
		if !strings.Contains(res.Content[0].Text, "cancelled") {
			t.Errorf("expected the cancelled status, got: %s", res.Content[0].Text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("wait_for_run did not return on a cancelled run")
	}
}

// TestWaitForRunKeepsPollingNonTerminal is the false-positive control: the fix
// must not be "return on any status".
func TestWaitForRunKeepsPollingNonTerminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"run-r","status":"running"}`))
	}))
	defer srv.Close()

	h := newWaitForRunHandler(NewHarnessClient(srv.URL), RealClock{})
	res, _ := h(context.Background(), json.RawMessage(`{"run_id":"run-r","timeout_seconds":1}`))

	if !res.IsError || !strings.Contains(res.Content[0].Text, "timed out") {
		t.Errorf("a still-running run must poll until timeout, got: %+v", res)
	}
}

// TestClientHasRequestTimeout — a wedged daemon must not block a tool call forever.
func TestClientHasRequestTimeout(t *testing.T) {
	c := NewHarnessClient("http://127.0.0.1:1")
	if c.httpClient.Timeout <= 0 {
		t.Fatal("client has no request timeout; a hung daemon would block indefinitely")
	}
	if c.httpClient.Timeout > 5*time.Minute {
		t.Errorf("timeout %v is too long to be a useful bound", c.httpClient.Timeout)
	}
}

// TestRequestsCarryBearerTokenWhenConfigured — harnessd enforces Bearer auth by
// default, so the proxy must be able to authenticate.
func TestRequestsCarryBearerTokenWhenConfigured(t *testing.T) {
	var gotAuth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"id":"run-1","status":"completed","models":[],"runs":[]}`))
	}))
	defer srv.Close()

	c := NewHarnessClientWithToken(srv.URL, "sekret")
	_, _ = c.GetRun(context.Background(), "run-1") // GET
	_ = c.CancelRun(context.Background(), "run-1") // POST
	_, _ = c.ListModels(context.Background())      // GET via getJSON

	if len(gotAuth) != 3 {
		t.Fatalf("expected 3 requests, saw %d", len(gotAuth))
	}
	for i, h := range gotAuth {
		if h != "Bearer sekret" {
			t.Errorf("request %d Authorization = %q, want Bearer sekret", i, h)
		}
	}
}

// TestNoAuthHeaderWhenTokenUnset is the false-positive control: the fix must not
// be "always send an Authorization header".
func TestNoAuthHeaderWhenTokenUnset(t *testing.T) {
	var seen string
	present := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Authorization"]
		seen = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"run-1","status":"completed"}`))
	}))
	defer srv.Close()

	_, _ = NewHarnessClient(srv.URL).GetRun(context.Background(), "run-1")

	if present || seen != "" {
		t.Errorf("no token configured, but Authorization was sent: %q", seen)
	}
}

// TestPostErrorsIncludeServerMessage — a bare status code does not say why.
func TestPostErrorsIncludeServerMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"run nope does not exist"}}`))
	}))
	defer srv.Close()

	err := NewHarnessClient(srv.URL).CancelRun(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error must carry the server's message, got: %v", err)
	}
}

// TestContinueRunFailsOnMalformedResponse — a discarded decode error let this
// report success while returning no run to track.
func TestContinueRunFailsOnMalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"run_id":`)) // truncated
	}))
	defer srv.Close()

	resp, err := NewHarnessClient(srv.URL).ContinueRun(context.Background(), "run-1", "go on")
	if err == nil {
		t.Fatalf("expected a decode error, got success with run_id %q", resp.RunID)
	}
}

// blockingReader never yields a line and never reaches EOF, so only cancellation
// can end a loop reading from it.
type blockingReader struct{ release chan struct{} }

func (b *blockingReader) Read(p []byte) (int, error) {
	<-b.release
	return 0, io.EOF
}

// TestTransportStopsOnCancelWhileIdle is the regression test for issue #1321.
// scanner.Scan blocks in a read, so an in-loop context check can never fire while
// idle — which made harness-mcp ignore SIGINT with stdin open.
func TestTransportStopsOnCancelWhileIdle(t *testing.T) {
	r := &blockingReader{release: make(chan struct{})}
	defer close(r.release)

	d := NewDispatcher(NewHarnessClient("http://127.0.0.1:1"), RealClock{})
	tr := NewStdioTransport(r, io.Discard, d)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tr.Run(ctx) }()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return on cancellation while idle — no input was written")
	}
}

// TestTransportStopsOnEOF is the false-positive control: EOF must still end the
// loop, since that is the common shutdown path when a host closes stdin.
func TestTransportStopsOnEOF(t *testing.T) {
	d := NewDispatcher(NewHarnessClient("http://127.0.0.1:1"), RealClock{})
	tr := NewStdioTransport(strings.NewReader(""), io.Discard, d)

	done := make(chan error, 1)
	go func() { done <- tr.Run(context.Background()) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return on EOF")
	}
}

// TestTransportCompletesInFlightWorkBeforeReturning — cancelling must not drop a
// message that was already accepted; its response still has to be written.
func TestTransportCompletesInFlightWorkBeforeReturning(t *testing.T) {
	var out lockedBuffer
	d := NewDispatcher(NewHarnessClient("http://127.0.0.1:1"), RealClock{})
	tr := NewStdioTransport(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &out, d)

	if err := tr.Run(context.Background()); err != nil && err != context.Canceled {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "start_run") {
		t.Errorf("in-flight request produced no response: %q", out.String())
	}
}

// lockedBuffer is a concurrency-safe buffer for capturing transport output.
type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
