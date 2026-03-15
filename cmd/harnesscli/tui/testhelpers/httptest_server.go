package testhelpers

import (
	"net/http"
	"net/http/httptest"
)

// NewTestServer creates a test HTTP server that serves SSE events.
// Used by TUI tests to simulate harnessd responses.
func NewTestServer(handler http.Handler) *httptest.Server {
	return httptest.NewServer(handler)
}

// SSEHandler returns an http.Handler that streams canned SSE events.
// Stub for now; will be populated in TUI-007.
func SSEHandler(events []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	})
}
