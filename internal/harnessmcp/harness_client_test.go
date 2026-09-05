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
			// The server sends id and a nested cost_totals, not run_id/cost_usd.
			// This fixture previously encoded the struct's wrong tags, which is
			// how the mismatch stayed green (issue #1314).
			"runs": []map[string]any{
				{"id": "run-1", "status": "completed", "cost_totals": map[string]any{"cost_usd_total": 0.1}},
				{"id": "run-2", "status": "running", "cost_totals": map[string]any{"cost_usd_total": 0.2}},
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

// realRunResponseBody is copied from a live GET /v1/runs/{id}. The fixture is
// deliberately the server's actual shape rather than one written to match the
// struct — a fixture built from the struct is what let this mismatch survive
// (issue #1314).
const realRunResponseBody = `{
  "id": "run_2adedadc-a0f2-46d2-99a5-60b632b70e95",
  "prompt": "Reply with exactly the word: OK",
  "model": "gpt-4.1-mini",
  "status": "completed",
  "output": "OK",
  "conversation_id": "conv_abc",
  "usage_totals": {"prompt_tokens_total": 14257, "completion_tokens_total": 5, "total_tokens": 14262},
  "cost_totals": {"cost_usd_total": 0.0057072, "last_turn_cost_usd": 0.0057072, "cost_status": "available"},
  "created_at": "2026-08-11T12:00:00Z"
}`

func TestGetRunDecodesServerRunShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realRunResponseBody))
	}))
	defer srv.Close()

	c := NewHarnessClient(srv.URL)
	got, err := c.GetRun(context.Background(), "run_2adedadc-a0f2-46d2-99a5-60b632b70e95")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}

	// False-positive control: status already decoded before the fix, so if this
	// fails the fixture itself is wrong rather than the mapping.
	if got.Status != "completed" {
		t.Fatalf("Status = %q, want completed — fixture is wrong", got.Status)
	}
	if got.RunID != "run_2adedadc-a0f2-46d2-99a5-60b632b70e95" {
		t.Errorf("RunID = %q, want the server's id field", got.RunID)
	}
	if got.ConversationID != "conv_abc" {
		t.Errorf("ConversationID = %q, want conv_abc", got.ConversationID)
	}
	if got.CostUSD != 0.0057072 {
		t.Errorf("CostUSD = %v, want 0.0057072 from cost_totals.cost_usd_total", got.CostUSD)
	}
	if got.Output != "OK" {
		t.Errorf("Output = %q, want the run's output text", got.Output)
	}
}

// TestGetRunStatusHandlesEmptyOutputAndZeroCost is the false-positive control:
// the fix must not be "always report something non-zero".
func TestGetRunHandlesEmptyOutputAndZeroCost(t *testing.T) {
	const body = `{"id":"run_x","status":"running","output":"","cost_totals":{"cost_usd_total":0}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	got, err := NewHarnessClient(srv.URL).GetRun(context.Background(), "run_x")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want running", got.Status)
	}
	if got.Output != "" {
		t.Errorf("Output = %q, want empty", got.Output)
	}
	if got.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0", got.CostUSD)
	}
}

// TestListRunsDecodesServerRunShape covers the same mismatch in list_runs.
func TestListRunsDecodesServerRunShape(t *testing.T) {
	const body = `{"runs":[
	  {"id":"run_a","status":"completed","cost_totals":{"cost_usd_total":0.25}},
	  {"id":"run_b","status":"failed","cost_totals":{"cost_usd_total":0}}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	got, err := NewHarnessClient(srv.URL).ListRuns(context.Background(), ListRunsParams{})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d runs, want 2", len(got))
	}
	if got[0].RunID != "run_a" || got[0].CostUSD != 0.25 {
		t.Errorf("run[0] = %+v, want RunID run_a and CostUSD 0.25", got[0])
	}
	if got[1].RunID != "run_b" || got[1].CostUSD != 0 {
		t.Errorf("run[1] = %+v, want RunID run_b and CostUSD 0", got[1])
	}
}

func TestHarnessClient_SteerRun(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"run_id": "run-42"})
	}))
	defer srv.Close()

	if err := NewHarnessClient(srv.URL).SteerRun(context.Background(), "run-42", "please slow down"); err != nil {
		t.Fatalf("SteerRun: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/runs/run-42/steer" {
		t.Errorf("request = %s %s, want POST /v1/runs/run-42/steer", gotMethod, gotPath)
	}
	if gotBody["prompt"] != "please slow down" {
		t.Errorf("request body prompt = %q, want %q", gotBody["prompt"], "please slow down")
	}
}

func TestHarnessClient_SteerRun_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "run not found"})
	}))
	defer srv.Close()

	if err := NewHarnessClient(srv.URL).SteerRun(context.Background(), "missing-run", "hello"); err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestHarnessClient_ListProviders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/providers" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"providers":[{"name":"openai","configured":true,"health":"ok","model_count":12}]}`))
	}))
	defer srv.Close()

	got, err := NewHarnessClient(srv.URL).ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d providers, want 1", len(got))
	}
	if got[0].Name != "openai" || !got[0].Configured || got[0].Health != "ok" || got[0].ModelCount != 12 {
		t.Errorf("provider = %+v, want name openai, configured true, health ok, model_count 12", got[0])
	}
}

func TestHarnessClient_ListProviders_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "boom"})
	}))
	defer srv.Close()

	if _, err := NewHarnessClient(srv.URL).ListProviders(context.Background()); err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}
