package cron

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRemoteRunStarterSendsAuthenticatedScopedRequest(t *testing.T) {
	var got remoteRunRequest
	var gotIdempotency string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/cron/runs" {
			t.Fatalf("request = %s %s, want POST /v1/cron/runs", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		gotIdempotency = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run_id":"run-remote-1","status":"queued"}`))
	}))
	defer srv.Close()

	starter := NewRemoteRunStarter(RemoteRunStarterConfig{
		BaseURL:           srv.URL,
		APIKey:            "secret-token",
		ConnectTimeout:    time.Second,
		RequestTimeout:    time.Second,
		EndpointClass:     "local-harnessd",
		CorrelationPrefix: "cron",
	})
	runID, err := starter.StartRun(RunStartRequest{
		Prompt:            "private prompt must not be logged",
		Model:             "fixture-model",
		ProviderName:      "missing-primary",
		AllowFallback:     true,
		FallbackProviders: []string{"fake-secondary", "fake-tertiary"},
		TenantID:          "tenant-a",
		AgentID:           "agent-a",
		ConversationID:    "conversation-a",
		JobID:             "job-a",
		ExecutionID:       "execution-a",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if runID != "run-remote-1" {
		t.Fatalf("run id = %q", runID)
	}
	if got.Prompt != "private prompt must not be logged" || got.TenantID != "tenant-a" ||
		got.Model != "fixture-model" || got.ProviderName != "missing-primary" ||
		!got.AllowFallback || !slices.Equal(got.FallbackProviders, []string{"fake-secondary", "fake-tertiary"}) ||
		got.AgentID != "agent-a" || got.ConversationID != "conversation-a" ||
		got.JobID != "job-a" || got.ExecutionID != "execution-a" ||
		got.CorrelationKey != "cron/job-a/execution-a" {
		t.Fatalf("request lost scope/correlation: %+v", got)
	}
	if gotIdempotency != "cron/job-a/execution-a" {
		t.Fatalf("idempotency key = %q", gotIdempotency)
	}
}

func TestRemoteRunStarterNormalizesURLAndCredentialBeforeValidationAndDispatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q, want normalized bearer token", got)
		}
		_, _ = w.Write([]byte(`{"run_id":"run-normalized"}`))
	}))
	defer srv.Close()

	starter := NewRemoteRunStarter(RemoteRunStarterConfig{BaseURL: "  " + srv.URL + "///  ", APIKey: " token ", RequestTimeout: time.Second})
	if err := starter.ValidateJob(Job{ExecType: ExecTypeHarness}); err != nil {
		t.Fatalf("ValidateJob with padded config: %v", err)
	}
	runID, err := starter.StartRun(RunStartRequest{Prompt: "normalized", JobID: "job", ExecutionID: "exec"})
	if err != nil {
		t.Fatalf("StartRun with padded config: %v", err)
	}
	if runID != "run-normalized" {
		t.Fatalf("run id = %q", runID)
	}
}

func TestRemoteRunStarterObservationMapsRemoteScopeFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/runs/run-foreign" {
			t.Fatalf("observation request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer scoped-token" {
			t.Fatalf("authorization = %q", got)
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	starter := NewRemoteRunStarter(RemoteRunStarterConfig{BaseURL: srv.URL, APIKey: "scoped-token", RequestTimeout: time.Second})
	_, err := starter.ObserveRun(context.Background(), "run-foreign")
	var remoteErr *RemoteRunError
	if !errors.As(err, &remoteErr) || remoteErr.Code != "unauthorized" || remoteErr.StatusCode != http.StatusForbidden || remoteErr.Retryable {
		t.Fatalf("ObserveRun scope error = %#v, want non-retryable unauthorized remote error", err)
	}
}

func TestRemoteRunStarterMapsFailuresWithoutSecretsOrPrompt(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantCode  string
		wantRetry bool
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, body: `{"error":"no"}`, wantCode: "unauthorized"},
		{name: "server failure", status: http.StatusBadGateway, body: `secret body`, wantCode: "http_status", wantRetry: true},
		{name: "malformed", status: http.StatusAccepted, body: `{"run_id":`, wantCode: "malformed_response"},
		{name: "missing run id", status: http.StatusAccepted, body: `{"status":"queued"}`, wantCode: "malformed_response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			starter := NewRemoteRunStarter(RemoteRunStarterConfig{BaseURL: srv.URL, APIKey: "top-secret", RequestTimeout: time.Second})
			_, err := starter.StartRun(RunStartRequest{Prompt: "do not expose me", JobID: "job", ExecutionID: "exec"})
			if err == nil {
				t.Fatal("expected error")
			}
			var remoteErr *RemoteRunError
			if !errors.As(err, &remoteErr) {
				t.Fatalf("error = %T %v, want *RemoteRunError", err, err)
			}
			if remoteErr.Code != tt.wantCode || remoteErr.Retryable != tt.wantRetry {
				t.Fatalf("remote error = %+v", remoteErr)
			}
			if strings.Contains(err.Error(), "top-secret") || strings.Contains(err.Error(), "do not expose me") || strings.Contains(err.Error(), "secret body") {
				t.Fatalf("error leaked sensitive data: %v", err)
			}
		})
	}
}

func TestRemoteRunStarterTimeoutAndCancellationAreStructured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	starter := NewRemoteRunStarter(RemoteRunStarterConfig{BaseURL: srv.URL, APIKey: "secret", RequestTimeout: 20 * time.Millisecond})
	_, err := starter.StartRun(RunStartRequest{Prompt: "prompt", JobID: "job", ExecutionID: "exec"})
	var remoteErr *RemoteRunError
	if !errors.As(err, &remoteErr) || remoteErr.Code != "timeout" || !remoteErr.Retryable {
		t.Fatalf("timeout error = %T %+v", err, err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error does not unwrap to context deadline: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = starter.StartRunContext(ctx, RunStartRequest{Prompt: "prompt", JobID: "job", ExecutionID: "exec"})
	if !errors.As(err, &remoteErr) || remoteErr.Code != "cancelled" || remoteErr.Retryable {
		t.Fatalf("cancel error = %T %+v", err, err)
	}
}

func TestRemoteRunStarterClassifiesTimeoutAndCancelWhileReadingAcceptedBody(t *testing.T) {
	t.Run("request timeout", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}))
		defer srv.Close()

		starter := NewRemoteRunStarter(RemoteRunStarterConfig{
			BaseURL:        srv.URL,
			APIKey:         "secret",
			RequestTimeout: 20 * time.Millisecond,
		})
		_, err := starter.StartRun(RunStartRequest{Prompt: "prompt", JobID: "job", ExecutionID: "exec"})
		var remoteErr *RemoteRunError
		if !errors.As(err, &remoteErr) || remoteErr.Code != "timeout" || !remoteErr.Retryable {
			t.Fatalf("stalled body timeout = %T %+v", err, err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("stalled body timeout does not unwrap deadline: %v", err)
		}
	})

	t.Run("parent cancellation", func(t *testing.T) {
		headersFlushed := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.(http.Flusher).Flush()
			close(headersFlushed)
			<-r.Context().Done()
		}))
		defer srv.Close()

		starter := NewRemoteRunStarter(RemoteRunStarterConfig{
			BaseURL:        srv.URL,
			APIKey:         "secret",
			RequestTimeout: time.Second,
		})
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() {
			_, err := starter.StartRunContext(ctx, RunStartRequest{Prompt: "prompt", JobID: "job", ExecutionID: "exec"})
			result <- err
		}()
		select {
		case <-headersFlushed:
			cancel()
		case <-time.After(time.Second):
			t.Fatal("server did not flush accepted headers")
		}
		err := <-result
		var remoteErr *RemoteRunError
		if !errors.As(err, &remoteErr) || remoteErr.Code != "cancelled" || remoteErr.Retryable {
			t.Fatalf("stalled body cancellation = %T %+v", err, err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("stalled body cancellation does not unwrap cancel: %v", err)
		}
	})
}

func TestRemoteRunStarterRejectsRedirectWithoutForwardingCredentials(t *testing.T) {
	var targetCalled bool
	var targetAuthorization string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalled = true
		targetAuthorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"run_id":"redirected-run"}`))
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL+"/capture")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	starter := NewRemoteRunStarter(RemoteRunStarterConfig{
		BaseURL:        redirect.URL,
		APIKey:         "redirect-secret",
		RequestTimeout: time.Second,
	})
	_, err := starter.StartRun(RunStartRequest{Prompt: "prompt", JobID: "job", ExecutionID: "exec"})
	var remoteErr *RemoteRunError
	if !errors.As(err, &remoteErr) || remoteErr.StatusCode != http.StatusTemporaryRedirect || remoteErr.Retryable {
		t.Fatalf("redirect error = %T %+v", err, err)
	}
	if targetCalled || targetAuthorization != "" {
		t.Fatalf("redirect target received request=%t authorization=%q", targetCalled, targetAuthorization)
	}
}

func TestRemoteRunStarterReadinessIsHarnessSpecific(t *testing.T) {
	starter := NewRemoteRunStarter(RemoteRunStarterConfig{})
	if err := starter.ValidateJob(Job{ExecType: ExecTypeShell}); err != nil {
		t.Fatalf("shell readiness = %v", err)
	}
	if err := starter.ValidateJob(Job{ExecType: ExecTypeHarness}); err == nil || !strings.Contains(err.Error(), "CRONSD_HARNESS_URL") {
		t.Fatalf("harness readiness = %v", err)
	}
}
