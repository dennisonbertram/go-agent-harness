package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	runstore "go-agent-harness/internal/store"
)

func TestTerminalSSEReplayWaitsForMatchingStatusSettlement(t *testing.T) {
	tests := []struct {
		name       string
		provider   *terminalHTTPProvider
		wantStatus harness.RunStatus
		wantEvent  harness.EventType
		cancel     bool
	}{
		{
			name:       "completed",
			provider:   &terminalHTTPProvider{result: harness.CompletionResult{Content: "done"}},
			wantStatus: harness.RunStatusCompleted,
			wantEvent:  harness.EventRunCompleted,
		},
		{
			name:       "failed",
			provider:   &terminalHTTPProvider{err: errors.New("provider unavailable")},
			wantStatus: harness.RunStatusFailed,
			wantEvent:  harness.EventRunFailed,
		},
		{
			name:       "cancelled",
			provider:   &terminalHTTPProvider{started: make(chan struct{}), hang: true},
			wantStatus: harness.RunStatusCancelled,
			wantEvent:  harness.EventRunCancelled,
			cancel:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTerminalUpdateBarrierStore()
			runner := harness.NewRunner(tt.provider, harness.NewRegistry(), harness.RunnerConfig{
				DefaultModel: "test-model",
				MaxSteps:     1,
				Store:        store,
			})
			ts := httptest.NewServer(New(runner))
			t.Cleanup(func() {
				store.Release()
				ts.Close()
				_ = runner.Shutdown(context.Background())
			})

			runID := startTerminalHTTPRun(t, ts)
			if tt.cancel {
				select {
				case <-tt.provider.started:
				case <-time.After(2 * time.Second):
					t.Fatal("provider did not start")
				}
				res, err := http.Post(ts.URL+"/v1/runs/"+runID+"/cancel", "application/json", nil)
				if err != nil {
					t.Fatalf("POST cancel: %v", err)
				}
				_ = res.Body.Close()
				if res.StatusCode != http.StatusOK {
					t.Fatalf("POST cancel status=%d, want %d", res.StatusCode, http.StatusOK)
				}
			}

			select {
			case <-store.terminalUpdateEntered:
			case <-time.After(2 * time.Second):
				t.Fatal("terminal UpdateRun did not reach barrier after terminal event journal")
			}

			// The Last-Event-ID cursor skips run.started, so any completed HTTP
			// response before the store barrier is released necessarily exposes the
			// replayed terminal event while GET remains non-terminal.
			eventResult := make(chan terminalSSEHTTPResult, 1)
			go func() {
				req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/runs/"+runID+"/events", nil)
				if err != nil {
					eventResult <- terminalSSEHTTPResult{err: err}
					return
				}
				req.Header.Set("Last-Event-ID", runID+":0")
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					eventResult <- terminalSSEHTTPResult{err: err}
					return
				}
				body, readErr := io.ReadAll(res.Body)
				_ = res.Body.Close()
				eventResult <- terminalSSEHTTPResult{statusCode: res.StatusCode, body: string(body), err: readErr}
			}()

			if got := getRunHTTPStatus(t, ts, runID); got == string(tt.wantStatus) {
				t.Fatalf("GET status=%q while terminal UpdateRun is blocked, want non-terminal", got)
			}
			select {
			case result := <-eventResult:
				t.Fatalf("terminal SSE escaped before matching status settlement: status=%d body=%q err=%v",
					result.statusCode, result.body, result.err)
			case <-time.After(100 * time.Millisecond):
			}

			store.Release()
			var result terminalSSEHTTPResult
			select {
			case result = <-eventResult:
			case <-time.After(2 * time.Second):
				t.Fatal("terminal SSE did not resume after matching status settlement")
			}
			if result.err != nil {
				t.Fatalf("GET terminal events: %v", result.err)
			}
			if result.statusCode != http.StatusOK {
				t.Fatalf("GET terminal events status=%d body=%s", result.statusCode, result.body)
			}
			if count := strings.Count(result.body, "event: "+string(tt.wantEvent)+"\n"); count != 1 {
				t.Fatalf("terminal event count=%d, want 1; body=%s", count, result.body)
			}
			if strings.Contains(result.body, "event: run.started\n") {
				t.Fatalf("Last-Event-ID replay included run.started; body=%s", result.body)
			}
			if got := getRunHTTPStatus(t, ts, runID); got != string(tt.wantStatus) {
				t.Fatalf("GET status after terminal SSE=%q, want %q", got, tt.wantStatus)
			}
		})
	}
}

type terminalSSEHTTPResult struct {
	statusCode int
	body       string
	err        error
}

type terminalUpdateBarrierStore struct {
	runstore.Store
	terminalUpdateEntered chan struct{}
	release               chan struct{}
	once                  sync.Once
}

func newTerminalUpdateBarrierStore() *terminalUpdateBarrierStore {
	return &terminalUpdateBarrierStore{
		Store:                 runstore.NewMemoryStore(),
		terminalUpdateEntered: make(chan struct{}),
		release:               make(chan struct{}),
	}
}

func (s *terminalUpdateBarrierStore) Release() { s.once.Do(func() { close(s.release) }) }

func (s *terminalUpdateBarrierStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	if run.Status == runstore.RunStatusCompleted || run.Status == runstore.RunStatusFailed || run.Status == runstore.RunStatus("cancelled") {
		select {
		case <-s.terminalUpdateEntered:
		default:
			close(s.terminalUpdateEntered)
		}
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.Store.UpdateRun(ctx, run)
}

func getRunHTTPStatus(t *testing.T, ts *httptest.Server, runID string) string {
	t.Helper()
	res, err := http.Get(ts.URL + "/v1/runs/" + runID)
	if err != nil {
		t.Fatalf("GET run: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("GET run status=%d body=%s", res.StatusCode, body)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode GET run: %v", err)
	}
	return body.Status
}

func TestTerminalStatusPollImmediatelyReplaysMatchingTerminalEvent(t *testing.T) {
	tests := []struct {
		name       string
		provider   *terminalHTTPProvider
		wantStatus harness.RunStatus
		wantEvent  harness.EventType
		cancel     bool
	}{
		{
			name:       "completed",
			provider:   &terminalHTTPProvider{result: harness.CompletionResult{Content: "done"}},
			wantStatus: harness.RunStatusCompleted,
			wantEvent:  harness.EventRunCompleted,
		},
		{
			name:       "failed",
			provider:   &terminalHTTPProvider{err: errors.New("provider unavailable")},
			wantStatus: harness.RunStatusFailed,
			wantEvent:  harness.EventRunFailed,
		},
		{
			name:       "cancelled",
			provider:   &terminalHTTPProvider{started: make(chan struct{}), hang: true},
			wantStatus: harness.RunStatusCancelled,
			wantEvent:  harness.EventRunCancelled,
			cancel:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := harness.NewRunner(tt.provider, harness.NewRegistry(), harness.RunnerConfig{
				DefaultModel: "test-model",
				MaxSteps:     1,
			})
			ts := httptest.NewServer(New(runner))
			t.Cleanup(func() {
				ts.Close()
				_ = runner.Shutdown(context.Background())
			})

			runID := startTerminalHTTPRun(t, ts)
			if tt.cancel {
				select {
				case <-tt.provider.started:
				case <-time.After(2 * time.Second):
					t.Fatal("provider did not start")
				}
				res, err := http.Post(ts.URL+"/v1/runs/"+runID+"/cancel", "application/json", nil)
				if err != nil {
					t.Fatalf("POST cancel: %v", err)
				}
				_ = res.Body.Close()
				if res.StatusCode != http.StatusOK {
					t.Fatalf("POST cancel status=%d, want %d", res.StatusCode, http.StatusOK)
				}
			}

			if got := waitForRunStatus(t, ts, runID, string(tt.wantStatus)); got != string(tt.wantStatus) {
				t.Fatalf("terminal status=%s, want %s", got, tt.wantStatus)
			}

			req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/runs/"+runID+"/events", nil)
			if err != nil {
				t.Fatalf("build reconnect request: %v", err)
			}
			req.Header.Set("Last-Event-ID", runID+":0")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("GET event replay: %v", err)
			}
			body, readErr := io.ReadAll(res.Body)
			_ = res.Body.Close()
			if readErr != nil {
				t.Fatalf("read event replay: %v", readErr)
			}
			if res.StatusCode != http.StatusOK {
				t.Fatalf("GET event replay status=%d body=%s", res.StatusCode, body)
			}

			replay := string(body)
			matchingFrame := "event: " + string(tt.wantEvent) + "\n"
			if strings.Count(replay, matchingFrame) != 1 {
				t.Fatalf("terminal status %s replay count for %s=%d, want 1; replay=%s",
					tt.wantStatus, tt.wantEvent, strings.Count(replay, matchingFrame), replay)
			}
			for _, other := range []harness.EventType{
				harness.EventRunCompleted,
				harness.EventRunFailed,
				harness.EventRunCancelled,
			} {
				if other != tt.wantEvent && strings.Contains(replay, "event: "+string(other)+"\n") {
					t.Fatalf("terminal status %s replay included mismatched event %s; replay=%s",
						tt.wantStatus, other, replay)
				}
			}
		})
	}
}

func TestTerminalDurabilityBackpressureMapsStartAndContinueToServiceUnavailable(t *testing.T) {
	store := &httpTerminalStatusFailStore{Store: runstore.NewMemoryStore()}
	runner := harness.NewRunner(&terminalHTTPProvider{
		result: harness.CompletionResult{Content: "done"},
	}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
		Store:                 store,
	})
	ts := httptest.NewServer(New(runner))
	t.Cleanup(func() {
		ts.Close()
		_ = runner.Shutdown(context.Background())
	})

	runID := startTerminalHTTPRun(t, ts)
	waitForRunStatus(t, ts, runID, string(harness.RunStatusCompleted))

	tests := []struct {
		name string
		url  string
	}{
		{name: "start", url: ts.URL + "/v1/runs"},
		{name: "continue", url: ts.URL + "/v1/runs/" + runID + "/continue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := http.Post(tt.url, "application/json", bytes.NewBufferString(`{"prompt":"blocked admission"}`))
			if err != nil {
				t.Fatalf("POST: %v", err)
			}
			defer res.Body.Close()
			var response struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if res.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("status=%d code=%q, want 503", res.StatusCode, response.Error.Code)
			}
			if response.Error.Code != "terminal_durability_unavailable" {
				t.Fatalf("error code=%q, want terminal_durability_unavailable", response.Error.Code)
			}
		})
	}
}

type terminalHTTPProvider struct {
	result  harness.CompletionResult
	err     error
	started chan struct{}
	hang    bool
}

type httpTerminalStatusFailStore struct{ runstore.Store }

func (s *httpTerminalStatusFailStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	if run.Status == runstore.RunStatusCompleted || run.Status == runstore.RunStatusFailed ||
		run.Status == runstore.RunStatus("cancelled") {
		return errors.New("terminal status persistence unavailable")
	}
	return s.Store.UpdateRun(ctx, run)
}

func (p *terminalHTTPProvider) Complete(ctx context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	if p.started != nil {
		select {
		case <-p.started:
		default:
			close(p.started)
		}
	}
	if p.hang {
		<-ctx.Done()
		return harness.CompletionResult{}, ctx.Err()
	}
	return p.result, p.err
}

func startTerminalHTTPRun(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":"terminal atomicity"}`))
	if err != nil {
		t.Fatalf("POST run: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("POST run status=%d, want %d: %s", res.StatusCode, http.StatusAccepted, body)
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode run response: %v", err)
	}
	if created.RunID == "" {
		t.Fatal("POST run returned empty run_id")
	}
	return created.RunID
}
