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
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

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

type terminalHTTPProvider struct {
	result  harness.CompletionResult
	err     error
	started chan struct{}
	hang    bool
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
