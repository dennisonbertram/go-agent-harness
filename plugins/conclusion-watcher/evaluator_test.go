package conclusionwatcher_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	cw "go-agent-harness/plugins/conclusion-watcher"
)

// ============================================================
// Evaluator tests — Part 1
// ============================================================

func TestNewOpenAIEvaluator_Defaults(t *testing.T) {
	e := cw.NewOpenAIEvaluator("test-key")
	if e == nil {
		t.Fatal("NewOpenAIEvaluator must not return nil")
	}
	if e.APIKey != "test-key" {
		t.Errorf("expected APIKey test-key, got %s", e.APIKey)
	}
	if e.Model == "" {
		t.Error("expected default Model to be set")
	}
	if e.BaseURL == "" {
		t.Error("expected default BaseURL to be set")
	}
	if e.Client == nil {
		t.Error("expected default http.Client to be set")
	}
}

func TestOpenAIEvaluator_Evaluate_HappyPath(t *testing.T) {
	// Serve a mock OpenAI response that returns has_unjustified_conclusion=true.
	mockResponse := map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]any{
					"content": `{"has_unjustified_conclusion":true,"patterns":["hedge_assertion"],"evidence":"probably","explanation":"No read tool was called"}`,
				},
			},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("expected /chat/completions path, got %s", r.URL.Path)
		}
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Errorf("expected Bearer token, got: %s", authHeader)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	e := cw.NewOpenAIEvaluator("test-key")
	e.BaseURL = server.URL
	e.Client = server.Client()

	result, err := e.Evaluate(context.Background(),
		"The file probably contains the main logic",
		[]string{"step 1: bash(ls)"},
		[]string{"write_file"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasUnjustifiedConclusion {
		t.Error("expected HasUnjustifiedConclusion=true")
	}
	if len(result.Patterns) == 0 {
		t.Error("expected at least one pattern")
	}
	if result.Evidence == "" {
		t.Error("expected non-empty Evidence")
	}
}

func TestOpenAIEvaluator_Evaluate_ReturnsFalse(t *testing.T) {
	mockResponse := map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]any{
					"content": `{"has_unjustified_conclusion":false,"patterns":[],"evidence":"","explanation":"all assertions supported"}`,
				},
			},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	e := cw.NewOpenAIEvaluator("test-key")
	e.BaseURL = server.URL
	e.Client = server.Client()

	result, err := e.Evaluate(context.Background(), "I read the file and confirmed the output", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasUnjustifiedConclusion {
		t.Error("expected HasUnjustifiedConclusion=false")
	}
}

func TestOpenAIEvaluator_Evaluate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	e := cw.NewOpenAIEvaluator("test-key")
	e.BaseURL = server.URL
	e.Client = server.Client()

	_, err := e.Evaluate(context.Background(), "some text", nil, nil)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestOpenAIEvaluator_Evaluate_MalformedJSON(t *testing.T) {
	// LLM returns non-JSON content
	mockResponse := map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]any{
					"content": "I cannot determine this",
				},
			},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	e := cw.NewOpenAIEvaluator("test-key")
	e.BaseURL = server.URL
	e.Client = server.Client()

	_, err := e.Evaluate(context.Background(), "some text", nil, nil)
	if err == nil {
		t.Fatal("expected error for malformed JSON from LLM")
	}
}

func TestOpenAIEvaluator_Evaluate_ContextTimeout(t *testing.T) {
	// Serve a slow response to trigger context cancellation.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := cw.NewOpenAIEvaluator("test-key")
	e.BaseURL = server.URL
	e.Client = server.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := e.Evaluate(ctx, "some text", nil, nil)
	if err == nil {
		t.Fatal("expected error for context timeout")
	}
}

func TestOpenAIEvaluator_Evaluate_RequestContainsRequiredFields(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		// Return a minimal valid response.
		mockResponse := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"has_unjustified_conclusion":false,"patterns":[],"evidence":"","explanation":"ok"}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	e := cw.NewOpenAIEvaluator("test-key")
	e.BaseURL = server.URL
	e.Client = server.Client()

	_, err := e.Evaluate(context.Background(), "some text",
		[]string{"step 1: read_file(foo.go)"},
		[]string{"write_file"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify model field is set.
	if capturedBody["model"] == nil {
		t.Error("expected model field in request body")
	}
	// Verify messages field is set.
	messages, ok := capturedBody["messages"].([]any)
	if !ok || len(messages) == 0 {
		t.Error("expected non-empty messages field in request body")
	}
	// Verify response_format is set.
	if capturedBody["response_format"] == nil {
		t.Error("expected response_format field in request body")
	}
}

func TestOpenAIEvaluator_Evaluate_NoChoices(t *testing.T) {
	mockResponse := map[string]any{
		"choices": []any{},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	e := cw.NewOpenAIEvaluator("test-key")
	e.BaseURL = server.URL
	e.Client = server.Client()

	_, err := e.Evaluate(context.Background(), "text", nil, nil)
	if err == nil {
		t.Fatal("expected error when choices is empty")
	}
}

// ============================================================
// Plugin parallel evaluation merge logic — Part 2
// ============================================================

// mockEvaluator is a controllable Evaluator for testing.
type mockEvaluator struct {
	result *cw.EvaluatorResult
	err    error
	sleep  time.Duration
	calls  int
	mu     sync.Mutex
}

func (m *mockEvaluator) Evaluate(_ context.Context, _ string, _ []string, _ []string) (*cw.EvaluatorResult, error) {
	if m.sleep > 0 {
		time.Sleep(m.sleep)
	}
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	return m.result, m.err
}

func (m *mockEvaluator) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func TestWatcher_EvaluatorFalse_SuppressesPhraseDetections(t *testing.T) {
	// Phrase detectors would fire on "obviously must be", but evaluator says false.
	eval := &mockEvaluator{
		result: &cw.EvaluatorResult{
			HasUnjustifiedConclusion: false,
			Patterns:                 []cw.PatternType{},
		},
	}
	w := cw.New(cw.WatcherConfig{
		Evaluator: eval,
	})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	result, err := hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
		RunID: "run-eval-false",
		Step:  1,
		Response: harness.CompletionResult{
			Content: "obviously this must be the right answer",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// LLM said no conclusion jump, so no injection expected.
	if result.MutatedResponse != nil {
		t.Error("expected no MutatedResponse when evaluator returns false")
	}
	if result.Action != harness.HookActionContinue {
		t.Errorf("expected HookActionContinue, got %v", result.Action)
	}
	// Evaluator should have been called.
	if eval.Calls() == 0 {
		t.Error("expected evaluator to be called")
	}
}

func TestWatcher_EvaluatorTrue_UsesLLMDetections(t *testing.T) {
	eval := &mockEvaluator{
		result: &cw.EvaluatorResult{
			HasUnjustifiedConclusion: true,
			Patterns:                 []cw.PatternType{cw.PatternArchitectureAssumption},
			Evidence:                 "The design is clearly X",
			Explanation:              "No exploration tool called",
		},
	}
	w := cw.New(cw.WatcherConfig{
		Evaluator: eval,
	})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	_, err := hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
		RunID: "run-eval-true",
		Step:  1,
		Response: harness.CompletionResult{
			Content: "The design is clearly correct.",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	detections := w.Detections()
	if len(detections) == 0 {
		t.Fatal("expected detections to be recorded from evaluator result")
	}
	// Should have architecture_assumption from LLM.
	found := false
	for _, d := range detections {
		if d.Pattern == cw.PatternArchitectureAssumption {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected architecture_assumption detection from LLM")
	}
}

func TestWatcher_EvaluatorError_FallsBackToPhraseDetections(t *testing.T) {
	eval := &mockEvaluator{
		err: errors.New("evaluator timeout"),
	}
	w := cw.New(cw.WatcherConfig{
		Evaluator: eval,
	})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	result, err := hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
		RunID: "run-eval-err",
		Step:  1,
		Response: harness.CompletionResult{
			Content: "obviously this must be the right answer",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error from hook: %v", err)
	}
	// Phrase detectors should have fired (obviously, must be) → intervention.
	if result.MutatedResponse == nil {
		t.Error("expected MutatedResponse from phrase detectors when evaluator errors")
	}
	if len(w.Detections()) == 0 {
		t.Error("expected phrase detections when evaluator errors")
	}
}

func TestWatcher_EvaluatorNil_BehavesAsBeforePhraseOnly(t *testing.T) {
	// No evaluator — original phrase-only behavior.
	w := cw.New(cw.WatcherConfig{})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	result, err := hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
		RunID: "run-no-eval",
		Step:  1,
		Response: harness.CompletionResult{
			Content: "obviously this must be the right answer",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MutatedResponse == nil {
		t.Error("expected MutatedResponse from phrase detectors when no evaluator")
	}
}

func TestWatcher_EvaluatorRunsConcurrently(t *testing.T) {
	// Evaluator sleeps 50ms; phrase detectors are instant.
	// Both must complete before AfterMessage returns.
	evalSleep := 50 * time.Millisecond
	eval := &mockEvaluator{
		result: &cw.EvaluatorResult{HasUnjustifiedConclusion: false},
		sleep:  evalSleep,
	}
	w := cw.New(cw.WatcherConfig{
		Evaluator: eval,
	})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	start := time.Now()
	_, err := hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
		RunID: "run-concurrent-eval",
		Step:  1,
		Response: harness.CompletionResult{
			Content: "some neutral content",
		},
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Must have waited for evaluator.
	if elapsed < evalSleep-5*time.Millisecond {
		t.Errorf("AfterMessage returned too fast (%v), expected to wait for evaluator (%v)", elapsed, evalSleep)
	}
	// Must not have taken excessively long.
	if elapsed > 2*time.Second {
		t.Errorf("AfterMessage took too long (%v)", elapsed)
	}
	if eval.Calls() == 0 {
		t.Error("evaluator was not called")
	}
}

func TestWatcher_EvaluatorContextCancellation(t *testing.T) {
	// Evaluator blocks indefinitely; context cancels mid-flight.
	eval := &mockEvaluator{
		result: &cw.EvaluatorResult{HasUnjustifiedConclusion: false},
		sleep:  5 * time.Second,
	}
	w := cw.New(cw.WatcherConfig{
		Evaluator: eval,
	})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := hook.AfterMessage(ctx, harness.PostMessageHookInput{
		RunID: "run-ctx-cancel",
		Step:  1,
		Response: harness.CompletionResult{
			Content: "neutral content",
		},
	})
	// The hook should return (possibly with error or fallback) within the timeout.
	// Either error or safe fallback is acceptable — just must not hang.
	_ = err
}
