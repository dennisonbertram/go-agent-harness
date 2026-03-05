package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/systemprompt"
)

type promptStaticProvider struct{}

func (promptStaticProvider) Complete(_ context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	return harness.CompletionResult{Content: "done"}, nil
}

func TestRunsEndpointReturns400ForUnknownIntent(t *testing.T) {
	t.Parallel()
	engine := mustPromptEngine(t)
	runner := harness.NewRunner(promptStaticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel:       "gpt-5-nano",
		DefaultAgentIntent: "general",
		PromptEngine:       engine,
		MaxSteps:           2,
	})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":"hello","agent_intent":"missing"}`))
	if err != nil {
		t.Fatalf("create run request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, string(body))
	}
	var payload map[string]map[string]string
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"]["code"] != "invalid_request" {
		t.Fatalf("unexpected error payload: %+v", payload)
	}
}

func TestRunsEndpointAcceptsPromptExtensionsPayload(t *testing.T) {
	t.Parallel()
	engine := mustPromptEngine(t)
	runner := harness.NewRunner(promptStaticProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel:       "gpt-5-nano",
		DefaultAgentIntent: "general",
		PromptEngine:       engine,
		MaxSteps:           2,
	})
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := `{"prompt":"hello","agent_intent":"code_review","prompt_extensions":{"behaviors":["precise"],"talents":["review"],"custom":"Keep findings concise."}}`
	res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("create run request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, string(raw))
	}
}

func mustPromptEngine(t *testing.T) systemprompt.Engine {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	write("catalog.yaml", `version: 1
defaults:
  intent: general
  model_profile: default
intents:
  general: intents/general.md
  code_review: intents/code_review.md
model_profiles:
  - name: openai_gpt5
    match: gpt-5-*
    file: models/openai_gpt5.md
  - name: default
    match: "*"
    file: models/default.md
extensions:
  behaviors_dir: extensions/behaviors
  talents_dir: extensions/talents
`)
	write("base/main.md", "BASE")
	write("intents/general.md", "GENERAL")
	write("intents/code_review.md", "CODE_REVIEW")
	write("models/default.md", "MODEL_DEFAULT")
	write("models/openai_gpt5.md", "MODEL_GPT5")
	write("extensions/behaviors/precise.md", "BEHAVIOR_PRECISE")
	write("extensions/talents/review.md", "TALENT_REVIEW")

	engine, err := systemprompt.NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}
	return engine
}
