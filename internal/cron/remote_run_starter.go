package cron

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultRemoteConnectTimeout = 5 * time.Second
	defaultRemoteRequestTimeout = 15 * time.Second
	maxRemoteResponseBytes      = 64 << 10
	remoteObservationPoll       = 25 * time.Millisecond
)

// RemoteRunStarterConfig configures the authenticated cronsd-to-harnessd
// start boundary. The API key is retained only by the HTTP client and is
// never included in errors or logs.
type RemoteRunStarterConfig struct {
	BaseURL           string
	APIKey            string
	ConnectTimeout    time.Duration
	RequestTimeout    time.Duration
	EndpointClass     string
	CorrelationPrefix string
}

// RemoteRunStarter starts harness runs through harnessd's dedicated cron
// endpoint. It intentionally does not implement cron.Executor: HarnessExecutor
// remains the only owner of harness execution-config decoding and dispatch.
type RemoteRunStarter struct {
	config RemoteRunStarterConfig
	client *http.Client
}

// NewRemoteRunStarter constructs a starter. Configuration is validated lazily
// by ValidateJob and StartRun so a shell-only cronsd can boot without remote
// harness settings while still rejecting harness jobs deterministically.
func NewRemoteRunStarter(config RemoteRunStarterConfig) *RemoteRunStarter {
	// Keep validation and request construction on the exact same canonical
	// values. In particular, a padded URL must not validate successfully and
	// then fail during http.NewRequestWithContext.
	config.BaseURL = strings.TrimSpace(config.BaseURL)
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.EndpointClass = strings.TrimSpace(config.EndpointClass)
	config.CorrelationPrefix = strings.TrimSpace(config.CorrelationPrefix)
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = defaultRemoteConnectTimeout
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = defaultRemoteRequestTimeout
	}
	if config.EndpointClass == "" {
		config.EndpointClass = "harnessd"
	}
	if config.CorrelationPrefix == "" {
		config.CorrelationPrefix = "cron"
	}
	dialer := &net.Dialer{Timeout: config.ConnectTimeout}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   config.ConnectTimeout,
		ResponseHeaderTimeout: config.RequestTimeout,
	}
	return &RemoteRunStarter{
		config: config,
		client: &http.Client{
			Transport: transport,
			// A remote harness start must terminate at the configured
			// authenticated boundary. Following redirects can change POST
			// semantics and can forward the bearer credential to an unintended
			// endpoint.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// ValidateJob reports whether a harness job can be dispatched by this
// starter. Shell jobs deliberately require no remote configuration.
func (s *RemoteRunStarter) ValidateJob(job Job) error {
	if job.ExecType != ExecTypeHarness {
		return nil
	}
	return s.validateConfig()
}

// StartRun starts a run using a background context. Scheduler calls the
// context-aware form so cancellation and request deadlines remain bounded.
func (s *RemoteRunStarter) StartRun(req RunStartRequest) (string, error) {
	return s.StartRunContext(context.Background(), req)
}

// StartRunContext sends a typed, authenticated remote start request and
// returns harnessd's stable run ID.
func (s *RemoteRunStarter) StartRunContext(ctx context.Context, req RunStartRequest) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.validateConfig(); err != nil {
		return "", &RemoteRunError{Code: "not_configured", Retryable: false, Err: err}
	}

	correlationKey := s.correlationKey(req)
	body, err := json.Marshal(remoteRunRequest{
		Prompt:         req.Prompt,
		TenantID:       req.TenantID,
		AgentID:        req.AgentID,
		ConversationID: req.ConversationID,
		JobID:          req.JobID,
		ExecutionID:    req.ExecutionID,
		CorrelationKey: correlationKey,
	})
	if err != nil {
		return "", &RemoteRunError{Code: "request_encode", Retryable: false, Err: err}
	}

	requestCtx, cancel := context.WithTimeout(ctx, s.config.RequestTimeout)
	defer cancel()
	endpoint := strings.TrimRight(s.config.BaseURL, "/") + "/v1/cron/runs"
	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", &RemoteRunError{Code: "request_create", Retryable: false, Err: err}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	httpReq.Header.Set("Idempotency-Key", correlationKey)
	httpReq.Header.Set("X-Cron-Job-ID", req.JobID)
	httpReq.Header.Set("X-Cron-Execution-ID", req.ExecutionID)

	startedAt := time.Now()
	resp, err := s.client.Do(httpReq)
	latency := time.Since(startedAt)
	if err != nil {
		code, retryable := "transport_error", true
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			code = "timeout"
		} else if errors.Is(err, context.Canceled) || errors.Is(requestCtx.Err(), context.Canceled) {
			code, retryable = "cancelled", false
		}
		logRemoteStart(s.config.EndpointClass, req, 0, latency, retryable)
		return "", &RemoteRunError{Code: code, Retryable: retryable, Err: err}
	}
	defer resp.Body.Close()

	retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		logRemoteStart(s.config.EndpointClass, req, resp.StatusCode, latency, retryable)
		return "", &RemoteRunError{Code: remoteStatusCode(resp.StatusCode), StatusCode: resp.StatusCode, Retryable: retryable}
	}

	var result remoteRunResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRemoteResponseBytes)).Decode(&result); err != nil {
		code, decodeRetryable, cause := classifyRemoteContextError(requestCtx, err)
		if code != "" {
			logRemoteStart(s.config.EndpointClass, req, resp.StatusCode, time.Since(startedAt), decodeRetryable)
			return "", &RemoteRunError{Code: code, StatusCode: resp.StatusCode, Retryable: decodeRetryable, Err: cause}
		}
		logRemoteStart(s.config.EndpointClass, req, resp.StatusCode, time.Since(startedAt), false)
		return "", &RemoteRunError{Code: "malformed_response", StatusCode: resp.StatusCode, Retryable: false, Err: err}
	}
	if strings.TrimSpace(result.RunID) == "" {
		logRemoteStart(s.config.EndpointClass, req, resp.StatusCode, time.Since(startedAt), false)
		return "", &RemoteRunError{Code: "malformed_response", StatusCode: resp.StatusCode, Retryable: false, Err: fmt.Errorf("response did not include run_id")}
	}
	logRemoteStart(s.config.EndpointClass, req, resp.StatusCode, time.Since(startedAt), false)
	return result.RunID, nil
}

// ObserveRun waits for harnessd's authenticated run-status resource to reach
// a terminal state. The cron-start credential must therefore carry runs:read
// in addition to its existing runs:write grant; an under-scoped or foreign
// request stays a typed remote error and is never mistaken for a terminal run.
func (s *RemoteRunStarter) ObserveRun(ctx context.Context, runID string) (RunObservation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.validateConfig(); err != nil {
		return RunObservation{}, &RemoteRunError{Code: "not_configured", Retryable: false, Err: err}
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return RunObservation{}, &RemoteRunError{Code: "malformed_response", Retryable: false, Err: fmt.Errorf("run id is required")}
	}
	for {
		state, err := s.getRunState(ctx, runID)
		if err != nil {
			return RunObservation{}, err
		}
		switch strings.ToLower(strings.TrimSpace(state.Status)) {
		case "completed":
			return RunObservation{Succeeded: true, OutputSummary: BoundedExecutionSummary(state.Output)}, nil
		case "failed", "cancelled":
			return RunObservation{Succeeded: false, OutputSummary: BoundedExecutionSummary(state.Output), Error: BoundedExecutionSummary(state.Error)}, nil
		}
		select {
		case <-ctx.Done():
			code, retryable, cause := classifyRemoteContextError(ctx, ctx.Err())
			return RunObservation{}, &RemoteRunError{Code: code, Retryable: retryable, Err: cause}
		case <-time.After(remoteObservationPoll):
		}
	}
}

type remoteRunState struct {
	Status string `json:"status"`
	Output string `json:"output"`
	Error  string `json:"error"`
}

func (s *RemoteRunStarter) getRunState(ctx context.Context, runID string) (remoteRunState, error) {
	requestCtx, cancel := context.WithTimeout(ctx, s.config.RequestTimeout)
	defer cancel()
	endpoint := strings.TrimRight(s.config.BaseURL, "/") + "/v1/runs/" + url.PathEscape(runID)
	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return remoteRunState{}, &RemoteRunError{Code: "request_create", Retryable: false, Err: err}
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	resp, err := s.client.Do(httpReq)
	if err != nil {
		code, retryable, cause := classifyRemoteContextError(requestCtx, err)
		if code == "" {
			code, retryable, cause = "transport_error", true, err
		}
		return remoteRunState{}, &RemoteRunError{Code: code, Retryable: retryable, Err: cause}
	}
	defer resp.Body.Close()
	retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return remoteRunState{}, &RemoteRunError{Code: remoteStatusCode(resp.StatusCode), StatusCode: resp.StatusCode, Retryable: retryable}
	}
	var state remoteRunState
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRemoteResponseBytes)).Decode(&state); err != nil {
		code, decodeRetryable, cause := classifyRemoteContextError(requestCtx, err)
		if code == "" {
			code, decodeRetryable, cause = "malformed_response", false, err
		}
		return remoteRunState{}, &RemoteRunError{Code: code, StatusCode: resp.StatusCode, Retryable: decodeRetryable, Err: cause}
	}
	if strings.TrimSpace(state.Status) == "" {
		return remoteRunState{}, &RemoteRunError{Code: "malformed_response", StatusCode: resp.StatusCode, Retryable: false, Err: fmt.Errorf("response did not include status")}
	}
	return state, nil
}

func classifyRemoteContextError(ctx context.Context, err error) (code string, retryable bool, cause error) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return "timeout", true, err
	}
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return "cancelled", false, err
	}
	return "", false, err
}

func (s *RemoteRunStarter) validateConfig() error {
	if s == nil {
		return fmt.Errorf("CRONSD_HARNESS_URL and CRONSD_HARNESS_API_KEY are required for harness jobs")
	}
	parsed, err := url.Parse(strings.TrimSpace(s.config.BaseURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("CRONSD_HARNESS_URL must be an absolute http(s) URL")
	}
	if strings.TrimSpace(s.config.APIKey) == "" {
		return fmt.Errorf("CRONSD_HARNESS_API_KEY is required for harness jobs")
	}
	if s.config.ConnectTimeout <= 0 || s.config.RequestTimeout <= 0 {
		return fmt.Errorf("remote harness timeouts must be positive")
	}
	return nil
}

func (s *RemoteRunStarter) correlationKey(req RunStartRequest) string {
	prefix := s.config.CorrelationPrefix
	return fmt.Sprintf("%s/%s/%s", prefix, req.JobID, req.ExecutionID)
}

func remoteStatusCode(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "unauthorized"
	default:
		return "http_status"
	}
}

func logRemoteStart(endpointClass string, req RunStartRequest, status int, latency time.Duration, retryable bool) {
	log.Printf("cron: remote harness start endpoint_class=%q job_id=%q execution_id=%q status_code=%d latency_ms=%d retryable=%t", endpointClass, req.JobID, req.ExecutionID, status, latency.Milliseconds(), retryable)
}

// RemoteRunError is a safe, inspectable failure from the remote start
// boundary. It intentionally omits request bodies, response bodies, tokens,
// and prompt contents.
type RemoteRunError struct {
	Code       string
	StatusCode int
	Retryable  bool
	Err        error
}

func (e *RemoteRunError) Error() string {
	if e == nil {
		return "remote harness start failed"
	}
	message := fmt.Sprintf("remote harness start failed: code=%s", e.Code)
	if e.StatusCode != 0 {
		message += fmt.Sprintf(" status=%d", e.StatusCode)
	}
	message += fmt.Sprintf(" retryable=%t", e.Retryable)
	return message
}

func (e *RemoteRunError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type remoteRunRequest struct {
	Prompt         string `json:"prompt"`
	TenantID       string `json:"tenant_id,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	JobID          string `json:"job_id"`
	ExecutionID    string `json:"execution_id"`
	CorrelationKey string `json:"correlation_key"`
}

type remoteRunResponse struct {
	RunID  string `json:"run_id"`
	Status string `json:"status,omitempty"`
}
