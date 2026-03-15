package testhelpers

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

// SSEEvent represents a single SSE event with an event type and data payload.
type SSEEvent struct {
	Event string
	Data  string
}

// MockSSEServer wraps an httptest.Server that streams canned SSE events.
type MockSSEServer struct {
	*httptest.Server
}

// NewMockSSEServer creates an httptest.Server that serves SSE events on /events.
// Events are sent with a 10ms delay between each, then the connection is closed.
// If events is nil or empty, the server closes the connection immediately.
func NewMockSSEServer(events []SSEEvent) *MockSSEServer {
	handler := http.NewServeMux()
	handler.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		flusher.Flush()

		for _, evt := range events {
			time.Sleep(10 * time.Millisecond)
			fmt.Fprintf(w, "event: %s\n", evt.Event)
			fmt.Fprintf(w, "data: %s\n\n", evt.Data)
			flusher.Flush()
		}
	})

	srv := httptest.NewServer(handler)
	return &MockSSEServer{Server: srv}
}

// CollectSSEEvents connects to the given URL and reads all SSE events until
// the server closes the connection or the context is cancelled.
func CollectSSEEvents(ctx context.Context, url string) ([]SSEEvent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to SSE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var events []SSEEvent
	scanner := bufio.NewScanner(resp.Body)
	var currentEvent string
	var currentData string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line signals end of an SSE block
			if currentEvent != "" || currentData != "" {
				events = append(events, SSEEvent{
					Event: currentEvent,
					Data:  currentData,
				})
				currentEvent = ""
				currentData = ""
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			currentData = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}

	// Handle any trailing event without a final blank line
	if currentEvent != "" || currentData != "" {
		events = append(events, SSEEvent{
			Event: currentEvent,
			Data:  currentData,
		})
	}

	if err := scanner.Err(); err != nil {
		// Context cancellation is not an error for our purposes
		if ctx.Err() != nil {
			return events, nil
		}
		return events, fmt.Errorf("reading SSE stream: %w", err)
	}

	return events, nil
}

// NewTestServer creates a test HTTP server that serves SSE events.
// Used by TUI tests to simulate harnessd responses.
func NewTestServer(handler http.Handler) *httptest.Server {
	return httptest.NewServer(handler)
}

// SSEHandler returns an http.Handler that streams canned SSE events.
func SSEHandler(events []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		flusher.Flush()

		for _, data := range events {
			time.Sleep(10 * time.Millisecond)
			fmt.Fprintf(w, "event: message\n")
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	})
}
