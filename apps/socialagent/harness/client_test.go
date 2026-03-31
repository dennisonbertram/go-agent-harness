package harness_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-agent-harness/apps/socialagent/harness"
)

func TestStartRun_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/runs" {
			t.Errorf("expected path /v1/runs, got %s", r.URL.Path)
		}

		var req harness.RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if req.Prompt != "Hello" {
			t.Errorf("Prompt: got %q, want %q", req.Prompt, "Hello")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(harness.RunResponse{
			RunID:  "run_test001",
			Status: "running",
		})
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	resp, err := client.StartRun(context.Background(), harness.RunRequest{
		Prompt:         "Hello",
		ConversationID: "conv_001",
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if resp.RunID != "run_test001" {
		t.Errorf("RunID: got %q, want %q", resp.RunID, "run_test001")
	}
	if resp.Status != "running" {
		t.Errorf("Status: got %q, want %q", resp.Status, "running")
	}
}

func TestStartRun_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid prompt"}`))
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	resp, err := client.StartRun(context.Background(), harness.RunRequest{
		Prompt:         "",
		ConversationID: "conv_001",
	})
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response on error, got %+v", resp)
	}
}

func TestStartRun_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	resp, err := client.StartRun(context.Background(), harness.RunRequest{
		Prompt:         "test",
		ConversationID: "conv_001",
	})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response on error, got %+v", resp)
	}
}

func TestSendAndWait_EndToEnd(t *testing.T) {
	runID := "run_e2e001"
	conversationID := "conv_e2e001"

	completedData, _ := json.Marshal(map[string]any{
		"id":        runID + ":1",
		"run_id":    runID,
		"type":      "run.completed",
		"timestamp": "2026-03-31T12:00:01Z",
		"payload": map[string]any{
			"output": "End to end response",
		},
	})
	startedData, _ := json.Marshal(map[string]any{
		"id":        runID + ":0",
		"run_id":    runID,
		"type":      "run.started",
		"timestamp": "2026-03-31T12:00:00Z",
		"payload":   map[string]any{},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs":
			var req harness.RunRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.ConversationID != conversationID {
				t.Errorf("ConversationID: got %q, want %q", req.ConversationID, conversationID)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(harness.RunResponse{
				RunID:  runID,
				Status: "running",
			})

		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/v1/runs/%s/events", runID):
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			writeSSEEvent(w, runID+":0", "run.started", string(startedData))
			writeSSEEvent(w, runID+":1", "run.completed", string(completedData))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	result, err := client.SendAndWait(context.Background(), harness.RunRequest{
		Prompt:         "What is 2+2?",
		ConversationID: conversationID,
	})
	if err != nil {
		t.Fatalf("SendAndWait returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output != "End to end response" {
		t.Errorf("Output: got %q, want %q", result.Output, "End to end response")
	}
	if result.RunID != runID {
		t.Errorf("RunID: got %q, want %q", result.RunID, runID)
	}
}

func TestSendAndWait_StartRunFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := harness.NewClient(srv.URL)
	result, err := client.SendAndWait(context.Background(), harness.RunRequest{
		Prompt:         "test",
		ConversationID: "conv_001",
	})
	if err == nil {
		t.Fatal("expected error when StartRun fails, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %+v", result)
	}
}
