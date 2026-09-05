package server

// http_durable_run_routes_test.go covers issue #1375: after a daemon restart
// (same run/conversation stores, but a fresh in-memory Runner with no live
// state for the run), GET /v1/runs/{id}/events and GET /v1/runs/{id}/summary
// must be served from the persistent store, exactly like GET /v1/runs/{id}
// already is (see TestStoreRunFallback).

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/store"
)

// durableSSEFrame is one parsed SSE frame (id/event/data) from an events
// stream, used to assert on the replayed durable history.
type durableSSEFrame struct {
	id    string
	event string
	data  string
}

// readAllDurableSSEFrames reads every frame from r until the stream closes.
// It skips keepalive comment lines. The store-fallback path never emits a
// live tail, so the underlying connection is expected to close on its own.
func readAllDurableSSEFrames(t *testing.T, body io.Reader) []durableSSEFrame {
	t.Helper()
	var frames []durableSSEFrame
	r := bufio.NewReader(body)
	var cur durableSSEFrame
	for {
		line, err := r.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(trimmed, "id: "):
			cur.id = strings.TrimPrefix(trimmed, "id: ")
		case strings.HasPrefix(trimmed, "event: "):
			cur.event = strings.TrimPrefix(trimmed, "event: ")
		case strings.HasPrefix(trimmed, "data: "):
			cur.data = strings.TrimPrefix(trimmed, "data: ")
		case trimmed == "":
			if cur.event != "" || cur.data != "" {
				frames = append(frames, cur)
				cur = durableSSEFrame{}
			}
		}
		if err != nil {
			if err == io.EOF {
				return frames
			}
			t.Fatalf("reading SSE stream: %v", err)
		}
	}
}

// TestRunEventsAndSummary_ServedFromStoreAfterRunnerForgets reproduces the
// restart scenario from issue #1375: a run completes against one Runner
// backed by a persistent store, then a second, unrelated Runner instance
// (simulating the process restarting with the same store) serves the by-ID
// routes. GET /v1/runs/{id} already falls back to the store; /events and
// /summary must too.
func TestRunEventsAndSummary_ServedFromStoreAfterRunnerForgets(t *testing.T) {
	t.Parallel()

	memStore := store.NewMemoryStore()
	registry := harness.NewDefaultRegistryWithOptions(t.TempDir(), harness.DefaultRegistryOptions{
		ApprovalMode: harness.ToolApprovalModeFullAuto,
	})
	cached := 10
	provider := &scriptedProvider{turns: []harness.CompletionResult{
		{
			ToolCalls:  []harness.ToolCall{{ID: "c1", Name: "bash", Arguments: `{"command":"echo hi"}`}},
			Usage:      &harness.CompletionUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, CachedPromptTokens: &cached},
			CostUSD:    ptrFloat64(0.005),
			CostStatus: harness.CostStatusAvailable,
		},
		{Content: "done", Usage: &harness.CompletionUsage{PromptTokens: 200, CompletionTokens: 30, TotalTokens: 230}, CostUSD: ptrFloat64(0.003), CostStatus: harness.CostStatusAvailable},
	}}

	liveRunner := harness.NewRunner(provider, registry, harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
		Store:        memStore,
	})
	liveServer := httptest.NewServer(NewWithOptions(ServerOptions{Runner: liveRunner, Store: memStore, AuthDisabled: true}))
	defer liveServer.Close()

	res, err := http.Post(liveServer.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":"test durable routes"}`))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	defer res.Body.Close()
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.RunID == "" {
		t.Fatal("expected non-empty run_id")
	}

	deadline := time.Now().Add(4 * time.Second)
	for {
		statusRes, err := http.Get(liveServer.URL + "/v1/runs/" + created.RunID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		var state struct {
			Status string `json:"status"`
		}
		json.NewDecoder(statusRes.Body).Decode(&state)
		statusRes.Body.Close()
		if state.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for run completion, status=%s", state.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Simulate a daemon restart: a brand-new Runner with zero live knowledge
	// of created.RunID, sharing the same persistent store.
	restartedRunner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "unused"}},
		harness.NewRegistry(),
		harness.RunnerConfig{Store: memStore},
	)
	restartedServer := httptest.NewServer(NewWithOptions(ServerOptions{Runner: restartedRunner, Store: memStore, AuthDisabled: true}))
	defer restartedServer.Close()

	// --- /events must replay the durable history and close, not 404. ---
	eventsRes, err := http.Get(restartedServer.URL + "/v1/runs/" + created.RunID + "/events")
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer eventsRes.Body.Close()
	if eventsRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(eventsRes.Body)
		t.Fatalf("expected 200 from /events after restart, got %d: %s", eventsRes.StatusCode, body)
	}
	frames := readAllDurableSSEFrames(t, eventsRes.Body)
	if len(frames) == 0 {
		t.Fatal("expected at least one replayed durable event, got none")
	}
	last := frames[len(frames)-1]
	if last.event != string(harness.EventRunCompleted) {
		t.Fatalf("expected stream to end on run.completed, last event was %q", last.event)
	}
	if !strings.HasPrefix(last.id, created.RunID+":") {
		t.Fatalf("expected last event id to be scoped to run %q, got %q", created.RunID, last.id)
	}

	// --- /summary must be computed from the store, matching the live totals. ---
	summaryRes, err := http.Get(restartedServer.URL + "/v1/runs/" + created.RunID + "/summary")
	if err != nil {
		t.Fatalf("GET /summary: %v", err)
	}
	defer summaryRes.Body.Close()
	if summaryRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(summaryRes.Body)
		t.Fatalf("expected 200 from /summary after restart, got %d: %s", summaryRes.StatusCode, body)
	}
	var summary harness.RunSummary
	if err := json.NewDecoder(summaryRes.Body).Decode(&summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.Status != harness.RunStatusCompleted {
		t.Fatalf("expected completed, got %s", summary.Status)
	}
	if summary.StepsTaken != 2 {
		t.Fatalf("expected 2 steps, got %d", summary.StepsTaken)
	}
	if summary.TotalPromptTokens != 300 {
		t.Fatalf("expected 300 prompt tokens, got %d", summary.TotalPromptTokens)
	}
	if summary.TotalCompletionTokens != 80 {
		t.Fatalf("expected 80 completion tokens, got %d", summary.TotalCompletionTokens)
	}
	if summary.TotalCostUSD != 0.008 {
		t.Fatalf("expected 0.008 cost, got %f", summary.TotalCostUSD)
	}
	if len(summary.ToolCalls) != 1 || summary.ToolCalls[0].ToolName != "bash" {
		t.Fatalf("expected 1 bash tool call, got %+v", summary.ToolCalls)
	}
	if summary.CacheHitRate <= 0 {
		t.Fatalf("expected positive cache hit rate, got %f", summary.CacheHitRate)
	}
}
