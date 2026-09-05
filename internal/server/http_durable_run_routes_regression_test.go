package server

// http_durable_run_routes_regression_test.go adds regression coverage for
// issue #1375 beyond the main happy-path fallback (see
// http_durable_run_routes_test.go): Last-Event-ID resumption against the
// durable replay path, and the still-running status gate on the durable
// summary path. Both would fail if the store-fallback implementation in
// handleDurableRunEvents / durableRunSummary were reverted or narrowed to
// only the unconditional-full-replay / always-200 case.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/store"
)

// TestDurableRunEvents_LastEventIDSkipsSeenEvents catches a revert of the
// Last-Event-ID handling inside handleDurableRunEvents: if that code path
// were replaced with an unconditional full replay (ignoring the header
// entirely, which is the simplest possible -- and wrong -- implementation
// of the store fallback), a reconnecting client would receive duplicate
// events it already has.
func TestDurableRunEvents_LastEventIDSkipsSeenEvents(t *testing.T) {
	t.Parallel()

	memStore := store.NewMemoryStore()
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{DefaultModel: "test-model", MaxSteps: 1, Store: memStore},
	)
	liveServer := httptest.NewServer(NewWithOptions(ServerOptions{Runner: runner, Store: memStore, AuthDisabled: true}))
	defer liveServer.Close()

	res, err := http.Post(liveServer.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":"regression"}`))
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

	restartedRunner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "unused"}},
		harness.NewRegistry(),
		harness.RunnerConfig{Store: memStore},
	)
	restartedServer := httptest.NewServer(NewWithOptions(ServerOptions{Runner: restartedRunner, Store: memStore, AuthDisabled: true}))
	defer restartedServer.Close()

	// First pass: full replay, to learn a real mid-stream event ID.
	fullRes, err := http.Get(restartedServer.URL + "/v1/runs/" + created.RunID + "/events")
	if err != nil {
		t.Fatalf("GET /events (full replay): %v", err)
	}
	fullFrames := readAllDurableSSEFrames(t, fullRes.Body)
	fullRes.Body.Close()
	if len(fullFrames) < 2 {
		t.Fatalf("expected at least 2 durable events to test resumption against, got %d", len(fullFrames))
	}
	seenID := fullFrames[0].id

	// Second pass: reconnect with Last-Event-ID set to the first event. The
	// response must not repeat that event, and must still end on
	// run.completed.
	req, err := http.NewRequest(http.MethodGet, restartedServer.URL+"/v1/runs/"+created.RunID+"/events", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Last-Event-ID", seenID)
	resumeRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /events (resume): %v", err)
	}
	if resumeRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resumeRes.Body)
		resumeRes.Body.Close()
		t.Fatalf("expected 200 on resume, got %d: %s", resumeRes.StatusCode, body)
	}
	resumeFrames := readAllDurableSSEFrames(t, resumeRes.Body)
	resumeRes.Body.Close()

	if len(resumeFrames) != len(fullFrames)-1 {
		t.Fatalf("expected resume to skip the seen event (full=%d, resume=%d)", len(fullFrames), len(resumeFrames))
	}
	for _, f := range resumeFrames {
		if f.id == seenID {
			t.Fatalf("resume replay repeated already-seen event id %q", seenID)
		}
	}
	if resumeFrames[len(resumeFrames)-1].event != string(harness.EventRunCompleted) {
		t.Fatalf("expected resumed stream to still end on run.completed, got %q", resumeFrames[len(resumeFrames)-1].event)
	}
}

// TestDurableRunSummary_StillRunningReturnsConflict catches a revert of the
// status gate inside durableRunSummary: if the fallback ignored the stored
// run's status and always computed a summary from whatever events happen to
// be persisted so far, a client polling a run stuck in "running" (e.g. the
// process crashed mid-run) would get a misleading 200 with a partial,
// silently-final-looking summary instead of the same 409 the live path
// returns for an unfinished run.
func TestDurableRunSummary_StillRunningReturnsConflict(t *testing.T) {
	t.Parallel()

	memStore := store.NewMemoryStore()
	ctx := context.Background()
	seededRun := &store.Run{
		ID:        "still-running-1",
		TenantID:  "",
		AgentID:   "agent-test",
		Model:     "gpt-4",
		Prompt:    "long task",
		Status:    store.RunStatusRunning,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := memStore.CreateRun(ctx, seededRun); err != nil {
		t.Fatalf("seed run in store: %v", err)
	}

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "unused"}},
		harness.NewRegistry(),
		harness.RunnerConfig{Store: memStore},
	)
	ts := httptest.NewServer(NewWithOptions(ServerOptions{Runner: runner, Store: memStore, AuthDisabled: true}))
	defer ts.Close()

	res, err := http.Get(ts.URL + "/v1/runs/still-running-1/summary")
	if err != nil {
		t.Fatalf("GET /summary: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 409 for a still-running store-only run, got %d: %s", res.StatusCode, body)
	}
}
