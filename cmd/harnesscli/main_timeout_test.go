package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestStreamHTTPClientHasNoClientLevelTimeout verifies that the stream HTTP client
// has no client-level timeout that could prematurely terminate long-running SSE streams.
// A 60-second timeout on the requestHTTPClient is fine for short requests, but the
// stream client must never cut off a run that is still making progress.
func TestStreamHTTPClientHasNoClientLevelTimeout(t *testing.T) {
	t.Parallel()
	if streamHTTPClient.Timeout != 0 {
		t.Fatalf("streamHTTPClient must have no client-level timeout (Timeout=0), got %v", streamHTTPClient.Timeout)
	}
}

// TestStreamHTTPClientUsesStreamingTransport verifies that the streaming client
// uses a dedicated transport with IdleConnTimeout=0 so that long-running SSE
// connections are never dropped due to the transport's idle connection reaper.
func TestStreamHTTPClientUsesStreamingTransport(t *testing.T) {
	t.Parallel()
	transport, ok := streamHTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("streamHTTPClient.Transport must be *http.Transport, got %T", streamHTTPClient.Transport)
	}
	if transport.IdleConnTimeout != 0 {
		t.Fatalf("streaming transport must have IdleConnTimeout=0 to prevent idle reaping of SSE connections, got %v", transport.IdleConnTimeout)
	}
	if transport.ResponseHeaderTimeout != 0 {
		t.Fatalf("streaming transport must have ResponseHeaderTimeout=0, got %v", transport.ResponseHeaderTimeout)
	}
}

// TestStreamRunEventsCompletesAfterPauseLongerThanRequestTimeout verifies that a
// stream with a pause longer than the requestHTTPClient timeout (60s) is not
// terminated prematurely by the streaming client. We simulate this with a short
// configurable delay to keep tests fast while proving the absence of a premature
// cut-off at the stream-client level.
func TestStreamRunEventsCompletesAfterPauseLongerThanRequestTimeout(t *testing.T) {
	t.Parallel()

	// Use a small pause to keep the test fast while still proving the client
	// does not impose a timeout shorter than a real task would need.
	// The pause is 150ms — long enough that a client with a 100ms timeout would fail,
	// fast enough not to slow down the test suite materially.
	const pauseBetweenEvents = 150 * time.Millisecond

	var once sync.Once
	ready := make(chan struct{})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("ResponseWriter does not implement Flusher")
			return
		}

		// Send first event immediately.
		_, _ = io.WriteString(w, "event: run.started\n")
		_, _ = io.WriteString(w, "data: {\"id\":\"e1\",\"run_id\":\"run_pause\",\"type\":\"run.started\"}\n\n")
		flusher.Flush()

		once.Do(func() { close(ready) })

		// Pause to simulate a long-running tool call between events.
		time.Sleep(pauseBetweenEvents)

		// Send terminal event after the pause.
		_, _ = io.WriteString(w, "event: run.completed\n")
		_, _ = io.WriteString(w, "data: {\"id\":\"e2\",\"run_id\":\"run_pause\",\"type\":\"run.completed\"}\n\n")
		flusher.Flush()
	}))
	defer ts.Close()

	// Build a client whose timeout is shorter than the pause, to prove the streaming
	// client does not use this restrictive timeout. If streamRunEvents used this client
	// it would fail; the real streamHTTPClient must have no such restriction.
	shortTimeoutClient := &http.Client{
		Timeout: 50 * time.Millisecond,
		Transport: &http.Transport{
			IdleConnTimeout:       0,
			ResponseHeaderTimeout: 0,
		},
	}

	// Verify the short-timeout client actually fails — confirming our test logic is sound.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, shortErr := streamRunEvents(ctx, shortTimeoutClient, ts.URL, "run_pause", io.Discard)
	if shortErr == nil {
		t.Fatal("expected short-timeout client to fail, but it succeeded — test logic is broken")
	}

	// Now verify the actual streamHTTPClient (with no timeout) succeeds.
	term, err := streamRunEvents(context.Background(), streamHTTPClient, ts.URL, "run_pause", io.Discard)
	if err != nil {
		t.Fatalf("streamRunEvents with streamHTTPClient failed after pause: %v", err)
	}
	if term != "run.completed" {
		t.Fatalf("expected terminal event 'run.completed', got %q", term)
	}
}

// TestRequestHTTPClientHas60SecondTimeout verifies that the request client (used for
// non-streaming POST /v1/runs calls) retains its 60-second timeout. This protects
// against accidentally removing the timeout from the wrong client.
func TestRequestHTTPClientHas60SecondTimeout(t *testing.T) {
	t.Parallel()
	expected := 60 * time.Second
	if requestHTTPClient.Timeout != expected {
		t.Fatalf("requestHTTPClient must retain %v timeout for short request protection, got %v", expected, requestHTTPClient.Timeout)
	}
}
