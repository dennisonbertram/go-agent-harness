package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

type staticProvider struct {
	result harness.CompletionResult
}

func (s *staticProvider) Complete(_ context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	return s.result, nil
}

type scriptedProvider struct {
	mu    sync.Mutex
	turns []harness.CompletionResult
	calls int
}

func (s *scriptedProvider) Complete(_ context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls >= len(s.turns) {
		return harness.CompletionResult{}, nil
	}
	out := s.turns[s.calls]
	s.calls++
	return out, nil
}

func TestRunLifecycleEndpoints(t *testing.T) {
	t.Parallel()

	registry := harness.NewRegistry()
	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "done"}}, registry, harness.RunnerConfig{
		DefaultModel:        "gpt-4.1-mini",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
	})

	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":"Hello"}`))
	if err != nil {
		t.Fatalf("create run request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status %d: %s", res.StatusCode, string(body))
	}

	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.RunID == "" {
		t.Fatalf("expected run id")
	}

	eventsRes, err := http.Get(ts.URL + "/v1/runs/" + created.RunID + "/events")
	if err != nil {
		t.Fatalf("events request: %v", err)
	}
	defer eventsRes.Body.Close()

	if got := eventsRes.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected event stream content type, got %q", got)
	}

	eventBody, err := io.ReadAll(eventsRes.Body)
	if err != nil {
		t.Fatalf("read events body: %v", err)
	}
	bodyStr := string(eventBody)

	if !strings.Contains(bodyStr, "event: run.completed") {
		t.Fatalf("expected run.completed event in body: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "event: assistant.message") {
		t.Fatalf("expected assistant.message event in body: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "event: usage.delta") {
		t.Fatalf("expected usage.delta event in body: %s", bodyStr)
	}

	statusRes, err := http.Get(ts.URL + "/v1/runs/" + created.RunID)
	if err != nil {
		t.Fatalf("get run request: %v", err)
	}
	defer statusRes.Body.Close()

	if statusRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected run status code: %d", statusRes.StatusCode)
	}
	var runState struct {
		Status      string                  `json:"status"`
		Output      string                  `json:"output"`
		UsageTotals *harness.RunUsageTotals `json:"usage_totals"`
		CostTotals  *harness.RunCostTotals  `json:"cost_totals"`
	}
	if err := json.NewDecoder(statusRes.Body).Decode(&runState); err != nil {
		t.Fatalf("decode run state: %v", err)
	}
	if runState.Status != string(harness.RunStatusCompleted) {
		t.Fatalf("expected completed run, got %q", runState.Status)
	}
	if runState.Output != "done" {
		t.Fatalf("unexpected output %q", runState.Output)
	}
	if runState.UsageTotals == nil || runState.CostTotals == nil {
		t.Fatalf("expected usage/cost totals, got %+v", runState)
	}
	if runState.UsageTotals.TotalTokens != 0 {
		t.Fatalf("expected zero totals for provider-unreported usage, got %+v", runState.UsageTotals)
	}
	if runState.CostTotals.CostStatus != harness.CostStatusProviderUnreported {
		t.Fatalf("expected provider_unreported cost status, got %+v", runState.CostTotals)
	}
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("unexpected health status: %q", payload.Status)
	}
}

func TestRunsEndpointMethodNotAllowedAndInvalidJSON(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	getRes, err := http.Get(ts.URL + "/v1/runs")
	if err != nil {
		t.Fatalf("GET /v1/runs: %v", err)
	}
	defer getRes.Body.Close()
	if getRes.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", getRes.StatusCode)
	}
	if got := getRes.Header.Get("Allow"); got != http.MethodPost {
		t.Fatalf("expected Allow POST, got %q", got)
	}

	invalidRes, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString("{"))
	if err != nil {
		t.Fatalf("invalid json request: %v", err)
	}
	defer invalidRes.Body.Close()
	if invalidRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", invalidRes.StatusCode)
	}
	var payload map[string]map[string]string
	if err := json.NewDecoder(invalidRes.Body).Decode(&payload); err != nil {
		t.Fatalf("decode invalid response: %v", err)
	}
	if payload["error"]["code"] != "invalid_json" {
		t.Fatalf("unexpected error payload: %+v", payload)
	}
}

func TestRunByIDEndpointsNotFoundAndMethodValidation(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{})
	handler := New(runner)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	notFoundRes, err := http.Get(ts.URL + "/v1/runs/missing")
	if err != nil {
		t.Fatalf("GET missing run: %v", err)
	}
	defer notFoundRes.Body.Close()
	if notFoundRes.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", notFoundRes.StatusCode)
	}

	eventsNotFoundRes, err := http.Get(ts.URL + "/v1/runs/missing/events")
	if err != nil {
		t.Fatalf("GET missing events: %v", err)
	}
	defer eventsNotFoundRes.Body.Close()
	if eventsNotFoundRes.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", eventsNotFoundRes.StatusCode)
	}

	rootNotFoundRes, err := http.Get(ts.URL + "/v1/runs/")
	if err != nil {
		t.Fatalf("GET empty run id path: %v", err)
	}
	defer rootNotFoundRes.Body.Close()
	if rootNotFoundRes.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for /v1/runs/, got %d", rootNotFoundRes.StatusCode)
	}

	createRes, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":"x"}`))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	defer createRes.Body.Close()
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	statusPostReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/runs/"+created.RunID, bytes.NewBufferString(`{}`))
	statusPostRes, err := http.DefaultClient.Do(statusPostReq)
	if err != nil {
		t.Fatalf("POST run status: %v", err)
	}
	defer statusPostRes.Body.Close()
	if statusPostRes.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST run status, got %d", statusPostRes.StatusCode)
	}

	eventsPostReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/runs/"+created.RunID+"/events", bytes.NewBufferString(`{}`))
	eventsPostRes, err := http.DefaultClient.Do(eventsPostReq)
	if err != nil {
		t.Fatalf("POST run events: %v", err)
	}
	defer eventsPostRes.Body.Close()
	if eventsPostRes.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST run events, got %d", eventsPostRes.StatusCode)
	}
}

func TestRunInputEndpoints(t *testing.T) {
	t.Parallel()

	broker := harness.NewInMemoryAskUserQuestionBroker(time.Now)
	provider := &scriptedProvider{turns: []harness.CompletionResult{
		{
			ToolCalls: []harness.ToolCall{{
				ID:        "call_input",
				Name:      "AskUserQuestion",
				Arguments: `{"questions":[{"question":"Where next?","header":"Route","options":[{"label":"Docs","description":"Read docs"},{"label":"Code","description":"Read code"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "done"},
	}}
	registry := harness.NewDefaultRegistryWithOptions(t.TempDir(), harness.DefaultRegistryOptions{
		ApprovalMode:   harness.ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: 3 * time.Second,
	})
	runner := harness.NewRunner(provider, registry, harness.RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: 3 * time.Second,
	})

	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":"Need input"}`))
	if err != nil {
		t.Fatalf("create run request: %v", err)
	}
	defer res.Body.Close()
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	var inputRes *http.Response
	deadline := time.Now().Add(2 * time.Second)
	for {
		inputRes, err = http.Get(ts.URL + "/v1/runs/" + created.RunID + "/input")
		if err != nil {
			t.Fatalf("get input request: %v", err)
		}
		if inputRes.StatusCode == http.StatusOK {
			break
		}
		_ = inputRes.Body.Close()
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for pending input")
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer inputRes.Body.Close()

	var pending map[string]any
	if err := json.NewDecoder(inputRes.Body).Decode(&pending); err != nil {
		t.Fatalf("decode pending input: %v", err)
	}
	if pending["tool"] != "AskUserQuestion" {
		t.Fatalf("unexpected pending payload: %+v", pending)
	}

	invalidRes, err := http.Post(ts.URL+"/v1/runs/"+created.RunID+"/input", "application/json", bytes.NewBufferString(`{"answers":{"Where next?":"Nope"}}`))
	if err != nil {
		t.Fatalf("post invalid input: %v", err)
	}
	defer invalidRes.Body.Close()
	if invalidRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid answers, got %d", invalidRes.StatusCode)
	}

	validRes, err := http.Post(ts.URL+"/v1/runs/"+created.RunID+"/input", "application/json", bytes.NewBufferString(`{"answers":{"Where next?":"Docs"}}`))
	if err != nil {
		t.Fatalf("post valid input: %v", err)
	}
	defer validRes.Body.Close()
	if validRes.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 for valid answers, got %d", validRes.StatusCode)
	}

	noPendingRes, err := http.Get(ts.URL + "/v1/runs/" + created.RunID + "/input")
	if err != nil {
		t.Fatalf("get no pending input: %v", err)
	}
	defer noPendingRes.Body.Close()
	if noPendingRes.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for no pending input, got %d", noPendingRes.StatusCode)
	}
}

func TestRunInputEndpointsMissingRunAndInvalidJSON(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	getRes, err := http.Get(ts.URL + "/v1/runs/missing/input")
	if err != nil {
		t.Fatalf("GET missing input: %v", err)
	}
	defer getRes.Body.Close()
	if getRes.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", getRes.StatusCode)
	}

	postMissingRes, err := http.Post(ts.URL+"/v1/runs/missing/input", "application/json", bytes.NewBufferString(`{"answers":{"x":"y"}}`))
	if err != nil {
		t.Fatalf("POST missing input: %v", err)
	}
	defer postMissingRes.Body.Close()
	if postMissingRes.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", postMissingRes.StatusCode)
	}

	createRes, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":"x"}`))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	defer createRes.Body.Close()
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	invalidJSONRes, err := http.Post(ts.URL+"/v1/runs/"+created.RunID+"/input", "application/json", bytes.NewBufferString(`{`))
	if err != nil {
		t.Fatalf("POST invalid json: %v", err)
	}
	defer invalidJSONRes.Body.Close()
	if invalidJSONRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", invalidJSONRes.StatusCode)
	}
}

func TestConversationMessagesEndpoint(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "done"}}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	// Create a run with a specific conversation ID
	res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":"Hello","conversation_id":"conv-http"}`))
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

	// Wait for run to complete
	deadline := time.Now().Add(4 * time.Second)
	for {
		statusRes, err := http.Get(ts.URL + "/v1/runs/" + created.RunID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		var runState struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(statusRes.Body).Decode(&runState); err != nil {
			statusRes.Body.Close()
			t.Fatalf("decode run: %v", err)
		}
		statusRes.Body.Close()
		if runState.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for run to complete, last status: %s", runState.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// GET conversation messages
	convRes, err := http.Get(ts.URL + "/v1/conversations/conv-http/messages")
	if err != nil {
		t.Fatalf("get conversation messages: %v", err)
	}
	defer convRes.Body.Close()

	if convRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(convRes.Body)
		t.Fatalf("expected 200, got %d: %s", convRes.StatusCode, string(body))
	}

	var payload struct {
		Messages []harness.Message `json:"messages"`
	}
	if err := json.NewDecoder(convRes.Body).Decode(&payload); err != nil {
		t.Fatalf("decode conversation messages: %v", err)
	}
	if len(payload.Messages) == 0 {
		t.Fatalf("expected non-empty messages array")
	}
}

func TestConversationMessagesEndpoint404(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	res, err := http.Get(ts.URL + "/v1/conversations/nonexistent/messages")
	if err != nil {
		t.Fatalf("get conversation messages: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.StatusCode)
	}
}

// capturingServerProvider captures all CompletionRequests for inspection.
type capturingServerProvider struct {
	mu     sync.Mutex
	result harness.CompletionResult
	calls  []harness.CompletionRequest
}

func (c *capturingServerProvider) Complete(_ context.Context, req harness.CompletionRequest) (harness.CompletionResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, req)
	return c.result, nil
}

func (c *capturingServerProvider) lastRequest() *harness.CompletionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.calls) == 0 {
		return nil
	}
	last := c.calls[len(c.calls)-1]
	return &last
}

func TestSpecialCharacterPromptsRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		prompt string
	}{
		{"exclamation", "Hello! How are you?"},
		{"single_quotes", "It's a test"},
		{"double_quotes", `She said "hello"`},
		{"backslashes", `path\to\file`},
		{"newlines", "line1\nline2"},
		{"unicode_emoji", "Hello 🌍 world"},
		{"json_in_prompt", `Parse this: {"key": "value"}`},
		{"shell_metacharacters", `echo $HOME && rm -rf /; ls | grep foo`},
		{"backticks", "`code block`"},
		{"mixed", `It's "complex"! path\to\file 🎉 $var`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prov := &capturingServerProvider{result: harness.CompletionResult{Content: "ack"}}
			registry := harness.NewRegistry()
			runner := harness.NewRunner(prov, registry, harness.RunnerConfig{
				DefaultModel: "test-model",
				MaxSteps:     2,
			})
			ts := httptest.NewServer(New(runner))
			defer ts.Close()

			// Marshal prompt into JSON properly
			body, err := json.Marshal(map[string]string{"prompt": tc.prompt})
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}

			res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("create run: %v", err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusAccepted {
				respBody, _ := io.ReadAll(res.Body)
				t.Fatalf("expected 202, got %d: %s", res.StatusCode, respBody)
			}

			var created struct {
				RunID string `json:"run_id"`
			}
			json.NewDecoder(res.Body).Decode(&created)

			// Wait for completion
			deadline := time.Now().Add(4 * time.Second)
			for {
				statusRes, err := http.Get(ts.URL + "/v1/runs/" + created.RunID)
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
					t.Fatalf("timed out, last status: %s", state.Status)
				}
				time.Sleep(20 * time.Millisecond)
			}

			// Assert the provider received the exact prompt
			last := prov.lastRequest()
			if last == nil {
				t.Fatal("provider was never called")
			}
			// Find the user message in the messages slice
			found := false
			for _, msg := range last.Messages {
				if msg.Role == "user" && msg.Content == tc.prompt {
					found = true
					break
				}
			}
			if !found {
				// Show what was actually received for debugging
				var contents []string
				for _, msg := range last.Messages {
					contents = append(contents, fmt.Sprintf("role=%s content=%q", msg.Role, msg.Content))
				}
				t.Fatalf("prompt not found in messages.\nExpected: %q\nGot messages: %v", tc.prompt, contents)
			}
		})
	}
}
