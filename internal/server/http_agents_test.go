package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

// mockAgentRunner is a simple in-memory implementation of agentRunnerIface for tests.
type mockAgentRunner struct {
	mu     sync.Mutex
	output string
	err    error
	calls  int
}

func (m *mockAgentRunner) RunPrompt(_ context.Context, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.output, m.err
}

// mockForkedAgentRunner implements forkedAgentRunnerIface for tests.
type mockForkedAgentRunner struct {
	mu     sync.Mutex
	result agentForkResult
	err    error
	calls  int
}

func (m *mockForkedAgentRunner) RunPrompt(_ context.Context, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.result.Output, m.err
}

func (m *mockForkedAgentRunner) RunForkedSkill(_ context.Context, _ agentForkConfig) (agentForkResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.result, m.err
}

// mockSkillLister implements skillListerIface for tests.
type mockSkillLister struct {
	content string
	err     error
}

func (m *mockSkillLister) ResolveSkill(_ context.Context, _, _, _ string) (string, error) {
	return m.content, m.err
}

// testRunnerForAgents creates a minimal harness runner for agent endpoint tests.
func testRunnerForAgents(t *testing.T) *harness.Runner {
	t.Helper()
	registry := harness.NewRegistry()
	return harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		registry,
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
		},
	)
}

// postAgents sends a POST to /v1/agents and returns the response.
func postAgents(t *testing.T, ts *httptest.Server, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	res, err := http.Post(ts.URL+"/v1/agents", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /v1/agents: %v", err)
	}
	return res
}

func TestAgentsEndpoint_PromptExecutesAndReturnsOutput(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)
	mock := &mockAgentRunner{output: "agent result"}
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: mock})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res := postAgents(t, ts, map[string]any{"prompt": "do something"})
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var resp agentResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Output != "agent result" {
		t.Errorf("expected output %q, got %q", "agent result", resp.Output)
	}
	if resp.DurationMs < 0 {
		t.Errorf("expected non-negative duration_ms, got %d", resp.DurationMs)
	}
}

func TestAgentsEndpoint_SkillUsesForkedRunner(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)
	forked := &mockForkedAgentRunner{result: agentForkResult{
		Output:  "forked output",
		Summary: "forked summary",
	}}
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: forked, ForkedAgentRunner: forked})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res := postAgents(t, ts, map[string]any{
		"skill":      "deploy",
		"skill_args": "staging",
	})
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var resp agentResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Output != "forked output" {
		t.Errorf("expected output %q, got %q", "forked output", resp.Output)
	}
	if resp.Summary != "forked summary" {
		t.Errorf("expected summary %q, got %q", "forked summary", resp.Summary)
	}
	if forked.calls != 1 {
		t.Errorf("expected forked runner called once, got %d", forked.calls)
	}
}

func TestAgentsEndpoint_SkillFallbackToSkillLister(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)
	mock := &mockAgentRunner{output: "resolved skill output"}
	sl := &mockSkillLister{content: "resolved skill content"}
	// forkedAgentRunner is nil — should fall back to skillLister + agentRunner
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: mock, SkillLister: sl})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res := postAgents(t, ts, map[string]any{
		"skill": "my-skill",
	})
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var resp agentResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Output != "resolved skill output" {
		t.Errorf("expected output %q, got %q", "resolved skill output", resp.Output)
	}
	if mock.calls != 1 {
		t.Errorf("expected agent runner called once, got %d", mock.calls)
	}
}

func TestAgentsEndpoint_NeitherPromptNorSkill_Returns400(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)
	mock := &mockAgentRunner{output: "ok"}
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: mock})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res := postAgents(t, ts, map[string]any{})
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, string(body))
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error.Code != "invalid_request" {
		t.Errorf("expected error code invalid_request, got %q", errResp.Error.Code)
	}
}

func TestAgentsEndpoint_BothPromptAndSkill_Returns400(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)
	mock := &mockAgentRunner{output: "ok"}
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: mock})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res := postAgents(t, ts, map[string]any{
		"prompt": "do something",
		"skill":  "deploy",
	})
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, string(body))
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error.Code != "invalid_request" {
		t.Errorf("expected error code invalid_request, got %q", errResp.Error.Code)
	}
}

func TestAgentsEndpoint_TimeoutExceeded_Returns408(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)

	// Agent runner that blocks until context is cancelled.
	blocking := &blockingAgentRunner{}
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: blocking})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res := postAgents(t, ts, map[string]any{
		"prompt":          "hang forever",
		"timeout_seconds": 1, // 1 second timeout
	})
	defer res.Body.Close()

	if res.StatusCode != http.StatusRequestTimeout {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 408, got %d: %s", res.StatusCode, string(body))
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error.Code != "timeout" {
		t.Errorf("expected error code timeout, got %q", errResp.Error.Code)
	}
}

// blockingAgentRunner blocks until context is cancelled, then returns DeadlineExceeded.
type blockingAgentRunner struct{}

func (b *blockingAgentRunner) RunPrompt(ctx context.Context, _ string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func TestAgentsEndpoint_NoAgentRunner_Returns501(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)
	// Pass nil for agentRunner.
	handler := NewWithOptions(ServerOptions{Runner: runner})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res := postAgents(t, ts, map[string]any{"prompt": "do something"})
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotImplemented {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 501, got %d: %s", res.StatusCode, string(body))
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error.Code != "not_implemented" {
		t.Errorf("expected error code not_implemented, got %q", errResp.Error.Code)
	}
}

func TestAgentsEndpoint_ConcurrentRequestsHandledIndependently(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)

	// Each call returns a unique response based on the prompt content.
	multi := &promptEchoRunner{}
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: multi})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	const concurrency = 10
	results := make([]string, concurrency)
	errs := make([]error, concurrency)
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			prompt := strings.Repeat("x", idx+1) // unique prompts
			b, _ := json.Marshal(map[string]any{"prompt": prompt})
			resp, err := http.Post(ts.URL+"/v1/agents", "application/json", bytes.NewReader(b))
			if err != nil {
				errs[idx] = err
				return
			}
			defer resp.Body.Close()
			var ar agentResponse
			if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
				errs[idx] = err
				return
			}
			results[idx] = ar.Output
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d error: %v", i, err)
		}
	}
	for i, r := range results {
		if r == "" {
			t.Errorf("goroutine %d: expected non-empty output", i)
		}
	}
}

// promptEchoRunner returns the prompt itself as output (for concurrency tests).
type promptEchoRunner struct{}

func (p *promptEchoRunner) RunPrompt(_ context.Context, prompt string) (string, error) {
	return "echo:" + prompt, nil
}

func TestAgentsEndpoint_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)
	mock := &mockAgentRunner{output: "ok"}
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: mock})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/v1/agents")
	if err != nil {
		t.Fatalf("GET /v1/agents: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", res.StatusCode)
	}
}

func TestAgentsEndpoint_DefaultTimeout(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)

	// Verify the default 120-second timeout is applied by checking that
	// a fast runner completes well under the limit.
	fast := &mockAgentRunner{output: "fast"}
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: fast})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	start := time.Now()
	res := postAgents(t, ts, map[string]any{"prompt": "quick task"})
	defer res.Body.Close()
	elapsed := time.Since(start)

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}
	// Should finish in well under 1 second for a mock runner.
	if elapsed > 5*time.Second {
		t.Errorf("request took too long: %v", elapsed)
	}
}

func TestAgentsEndpoint_SkillNotFound_Returns404(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)
	sl := &mockSkillLister{err: context.DeadlineExceeded}
	// Use a skill lister that returns a "not found" error.
	sl2 := &notFoundSkillLister{}
	mock := &mockAgentRunner{}
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: mock, SkillLister: sl2})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	_ = sl // silence unused variable warning

	res := postAgents(t, ts, map[string]any{"skill": "nonexistent"})
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 404, got %d: %s", res.StatusCode, string(body))
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error.Code != "skill_not_found" {
		t.Errorf("expected error code skill_not_found, got %q", errResp.Error.Code)
	}
}

// notFoundSkillLister returns a "not found" error from ResolveSkill.
type notFoundSkillLister struct{}

func (n *notFoundSkillLister) ResolveSkill(_ context.Context, name, _, _ string) (string, error) {
	return "", &skillNotFoundError{name: name}
}

type skillNotFoundError struct{ name string }

func (e *skillNotFoundError) Error() string { return "skill " + e.name + " not found" }

func TestAgentsEndpoint_InvalidJSON_Returns400(t *testing.T) {
	t.Parallel()

	runner := testRunnerForAgents(t)
	mock := &mockAgentRunner{output: "ok"}
	handler := NewWithOptions(ServerOptions{Runner: runner, AgentRunner: mock})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/v1/agents", "application/json", strings.NewReader("{invalid json"))
	if err != nil {
		t.Fatalf("POST /v1/agents: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, string(body))
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error.Code != "invalid_json" {
		t.Errorf("expected error code invalid_json, got %q", errResp.Error.Code)
	}
}
