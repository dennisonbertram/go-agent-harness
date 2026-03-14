package harnessmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHarnessClient_StartRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var req StartRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Prompt == "" {
			t.Error("expected non-empty prompt")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"run_id": "run-123"})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	resp, err := client.StartRun(context.Background(), StartRunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if resp.RunID != "run-123" {
		t.Errorf("got run_id %q, want %q", resp.RunID, "run-123")
	}
}

func TestHarnessClient_StartRun_WithOptionalFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req StartRunRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "gpt-4.1-mini" {
			t.Errorf("got model %q, want %q", req.Model, "gpt-4.1-mini")
		}
		if req.ConversationID != "conv-abc" {
			t.Errorf("got conversation_id %q, want %q", req.ConversationID, "conv-abc")
		}
		if req.MaxSteps != 5 {
			t.Errorf("got max_steps %d, want 5", req.MaxSteps)
		}
		if req.MaxCostUSD != 1.5 {
			t.Errorf("got max_cost_usd %f, want 1.5", req.MaxCostUSD)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"run_id": "run-456"})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	_, err := client.StartRun(context.Background(), StartRunRequest{
		Prompt:         "test",
		Model:          "gpt-4.1-mini",
		ConversationID: "conv-abc",
		MaxSteps:       5,
		MaxCostUSD:     1.5,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
}

func TestHarnessClient_StartRun_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	_, err := client.StartRun(context.Background(), StartRunRequest{Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error for bad status, got nil")
	}
}

func TestHarnessClient_GetRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/runs/run-999" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(RunStatus{
			RunID:          "run-999",
			Status:         "completed",
			ConversationID: "conv-1",
			CostUSD:        0.5,
		})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	status, err := client.GetRun(context.Background(), "run-999")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if status.Status != "completed" {
		t.Errorf("got status %q, want %q", status.Status, "completed")
	}
	if status.RunID != "run-999" {
		t.Errorf("got run_id %q, want %q", status.RunID, "run-999")
	}
}

func TestHarnessClient_GetRun_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	_, err := client.GetRun(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestHarnessClient_ListRuns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/runs" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("conversation_id") != "conv-1" {
			t.Errorf("got conversation_id %q, want %q", q.Get("conversation_id"), "conv-1")
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"runs": []map[string]any{
				{"run_id": "run-1", "status": "completed", "cost_usd": 0.1},
				{"run_id": "run-2", "status": "running", "cost_usd": 0.2},
			},
		})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	runs, err := client.ListRuns(context.Background(), ListRunsParams{ConversationID: "conv-1", Limit: 20})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("got %d runs, want 2", len(runs))
	}
	if runs[0].RunID != "run-1" {
		t.Errorf("got run_id %q, want %q", runs[0].RunID, "run-1")
	}
}

func TestHarnessClient_ListRuns_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"runs": []any{}})
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	runs, err := client.ListRuns(context.Background(), ListRunsParams{})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("got %d runs, want 0", len(runs))
	}
}
