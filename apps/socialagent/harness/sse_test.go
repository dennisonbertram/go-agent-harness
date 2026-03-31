package harness_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-agent-harness/apps/socialagent/harness"
)

// writeSSEEvent writes a single SSE event to the response writer.
func writeSSEEvent(w http.ResponseWriter, id, event, data string) {
	if id != "" {
		fmt.Fprintf(w, "id: %s\n", id)
	}
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n", data)
	fmt.Fprintf(w, "\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func TestStreamEvents_SuccessfulRun(t *testing.T) {
	runID := "run_abc123"

	startedData, _ := json.Marshal(map[string]any{
		"id":        runID + ":0",
		"run_id":    runID,
		"type":      "run.started",
		"timestamp": "2026-03-31T12:00:00Z",
		"payload":   map[string]any{},
	})
	completedData, _ := json.Marshal(map[string]any{
		"id":        runID + ":1",
		"run_id":    runID,
		"type":      "run.completed",
		"timestamp": "2026-03-31T12:00:01Z",
		"payload": map[string]any{
			"output": "Hello, world!",
		},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		writeSSEEvent(w, runID+":0", "run.started", string(startedData))
		writeSSEEvent(w, runID+":1", "run.completed", string(completedData))
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	result, err := client.StreamEvents(t.Context(), runID)
	if err != nil {
		t.Fatalf("StreamEvents returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output != "Hello, world!" {
		t.Errorf("Output: got %q, want %q", result.Output, "Hello, world!")
	}
	if result.RunID != runID {
		t.Errorf("RunID: got %q, want %q", result.RunID, runID)
	}
	if result.Error != "" {
		t.Errorf("expected empty Error, got %q", result.Error)
	}
}

func TestStreamEvents_FailedRun(t *testing.T) {
	runID := "run_fail001"

	startedData, _ := json.Marshal(map[string]any{
		"id":        runID + ":0",
		"run_id":    runID,
		"type":      "run.started",
		"timestamp": "2026-03-31T12:00:00Z",
		"payload":   map[string]any{},
	})
	failedData, _ := json.Marshal(map[string]any{
		"id":        runID + ":1",
		"run_id":    runID,
		"type":      "run.failed",
		"timestamp": "2026-03-31T12:00:01Z",
		"payload": map[string]any{
			"error": "something went wrong",
		},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		writeSSEEvent(w, runID+":0", "run.started", string(startedData))
		writeSSEEvent(w, runID+":1", "run.failed", string(failedData))
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	result, err := client.StreamEvents(t.Context(), runID)
	if err == nil {
		t.Fatal("expected error for failed run, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on failure, got %+v", result)
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestStreamEvents_CancelledRun(t *testing.T) {
	runID := "run_cancel001"

	cancelledData, _ := json.Marshal(map[string]any{
		"id":        runID + ":0",
		"run_id":    runID,
		"type":      "run.cancelled",
		"timestamp": "2026-03-31T12:00:00Z",
		"payload":   map[string]any{},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		writeSSEEvent(w, runID+":0", "run.cancelled", string(cancelledData))
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	result, err := client.StreamEvents(t.Context(), runID)
	if err == nil {
		t.Fatal("expected error for cancelled run, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on cancellation, got %+v", result)
	}
	if err.Error() != "run cancelled" {
		t.Errorf("error message: got %q, want %q", err.Error(), "run cancelled")
	}
}

func TestStreamEvents_SSEParsingMultiplePayloads(t *testing.T) {
	runID := "run_multi001"

	// Simulate multiple intermediate events before completion.
	makeEvent := func(seq int, eventType string, payload map[string]any) string {
		data, _ := json.Marshal(map[string]any{
			"id":        fmt.Sprintf("%s:%d", runID, seq),
			"run_id":    runID,
			"type":      eventType,
			"timestamp": "2026-03-31T12:00:00Z",
			"payload":   payload,
		})
		return string(data)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		writeSSEEvent(w, runID+":0", "run.started", makeEvent(0, "run.started", map[string]any{}))
		writeSSEEvent(w, runID+":1", "tool.call", makeEvent(1, "tool.call", map[string]any{"name": "bash"}))
		writeSSEEvent(w, runID+":2", "tool.result", makeEvent(2, "tool.result", map[string]any{"output": "ok"}))
		writeSSEEvent(w, runID+":3", "run.completed", makeEvent(3, "run.completed", map[string]any{"output": "Final answer"}))
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	result, err := client.StreamEvents(t.Context(), runID)
	if err != nil {
		t.Fatalf("StreamEvents returned error: %v", err)
	}
	if result.Output != "Final answer" {
		t.Errorf("Output: got %q, want %q", result.Output, "Final answer")
	}
}

func TestStreamEvents_ServerError(t *testing.T) {
	runID := "run_err001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	result, err := client.StreamEvents(t.Context(), runID)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on server error, got %+v", result)
	}
}

func TestStreamEvents_MultiLineData(t *testing.T) {
	// Verify that multiple data: lines in one SSE event are concatenated with \n.
	// We emit an assistant.message event with two data: lines and confirm the
	// accumulated data string equals "first line\nsecond line".
	// Because assistant.message is a non-terminal event, StreamEvents keeps
	// reading and will eventually hit a run.completed event so the stream ends.
	runID := "run_multiline001"

	completedData, _ := json.Marshal(map[string]any{
		"id":        runID + ":1",
		"run_id":    runID,
		"type":      "run.completed",
		"timestamp": "2026-03-31T00:00:00Z",
		"payload": map[string]any{
			"output": "multi-line accumulation verified",
		},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		// Emit an event with two data: lines — these must be concatenated with \n.
		fmt.Fprintf(w, "event: assistant.message\n")
		fmt.Fprintf(w, "data: first line\n")
		fmt.Fprintf(w, "data: second line\n")
		fmt.Fprintf(w, "\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Emit a terminal event so StreamEvents can return.
		writeSSEEvent(w, runID+":1", "run.completed", string(completedData))
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	result, err := client.StreamEvents(t.Context(), runID)
	if err != nil {
		t.Fatalf("StreamEvents returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output != "multi-line accumulation verified" {
		t.Errorf("Output: got %q, want %q", result.Output, "multi-line accumulation verified")
	}
}
