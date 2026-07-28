package deferred

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// stubFetcher is a tools.WebFetcher stub that records the URLs it was asked
// to fetch, so tests can assert the agentic_fetch handler calls (or does not
// call) it with the exact arguments given.
type stubFetcher struct {
	content string
	err     error
	calls   []string
}

func (s *stubFetcher) Fetch(_ context.Context, url string) (string, error) {
	s.calls = append(s.calls, url)
	if s.err != nil {
		return "", s.err
	}
	return s.content, nil
}

func (s *stubFetcher) Search(_ context.Context, query string, max int) ([]map[string]any, error) {
	return nil, nil
}

// stubRunner is a tools.AgentRunner stub that records the prompts it was
// asked to run.
type stubRunner struct {
	output string
	err    error
	calls  []string
}

func (s *stubRunner) RunPrompt(_ context.Context, prompt string) (string, error) {
	s.calls = append(s.calls, prompt)
	if s.err != nil {
		return "", s.err
	}
	return s.output, nil
}

// TestAgenticFetchTool_Handler_InvalidJSON verifies malformed args are
// rejected before either the fetcher or the runner is invoked.
func TestAgenticFetchTool_Handler_InvalidJSON(t *testing.T) {
	t.Parallel()

	fetcher := &stubFetcher{err: errors.New("must not be called")}
	runner := &stubRunner{err: errors.New("must not be called")}
	tool := AgenticFetchTool(fetcher, runner)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse agentic_fetch args") {
		t.Errorf("expected 'parse agentic_fetch args' in error, got %q", err.Error())
	}
	if len(fetcher.calls) != 0 || len(runner.calls) != 0 {
		t.Error("fetcher/runner must not be called when args fail to parse")
	}
}

// TestAgenticFetchTool_Handler_NoURLSkipsFetch verifies that when no url is
// given, the fetcher is never invoked (not even with an empty string) and the
// result has no url/content keys — only prompt+analysis.
func TestAgenticFetchTool_Handler_NoURLSkipsFetch(t *testing.T) {
	t.Parallel()

	fetcher := &stubFetcher{err: errors.New("fetch should not have been called")}
	runner := &stubRunner{output: "analysis text"}
	tool := AgenticFetchTool(fetcher, runner)

	result, err := tool.Handler(context.Background(), json.RawMessage(`{"prompt":"summarize"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("expected fetcher not to be called, got calls: %v", fetcher.calls)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "summarize" {
		t.Errorf("expected runner called once with 'summarize', got %v", runner.calls)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["prompt"] != "summarize" {
		t.Errorf("expected prompt 'summarize', got %v", out["prompt"])
	}
	if out["analysis"] != "analysis text" {
		t.Errorf("expected analysis 'analysis text', got %v", out["analysis"])
	}
	if _, has := out["url"]; has {
		t.Errorf("expected no 'url' key when url was not provided, got %v", out["url"])
	}
	if _, has := out["content"]; has {
		t.Errorf("expected no 'content' key when url was not provided, got %v", out["content"])
	}
}

// TestAgenticFetchTool_Handler_WithURLFetchesThenAnalyzes verifies that when
// a url is given, the fetcher is called with exactly that url, the fetched
// content and url are included in the result, and the runner is invoked with
// the original prompt (not the fetched content).
func TestAgenticFetchTool_Handler_WithURLFetchesThenAnalyzes(t *testing.T) {
	t.Parallel()

	fetcher := &stubFetcher{content: "page content"}
	runner := &stubRunner{output: "insight"}
	tool := AgenticFetchTool(fetcher, runner)

	raw, _ := json.Marshal(map[string]string{"prompt": "analyze", "url": "https://example.com"})
	result, err := tool.Handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fetcher.calls) != 1 || fetcher.calls[0] != "https://example.com" {
		t.Errorf("expected fetcher called once with the given url, got %v", fetcher.calls)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "analyze" {
		t.Errorf("expected runner called once with 'analyze' (not fetched content), got %v", runner.calls)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["url"] != "https://example.com" {
		t.Errorf("expected url in result, got %v", out["url"])
	}
	if out["content"] != "page content" {
		t.Errorf("expected fetched content in result, got %v", out["content"])
	}
	if out["analysis"] != "insight" {
		t.Errorf("expected analysis in result, got %v", out["analysis"])
	}
}

// TestAgenticFetchTool_Handler_FetchErrorShortCircuits verifies that when the
// fetch fails, the handler returns the fetch error directly and never calls
// the runner (no wasted/incorrect analysis of a failed fetch).
func TestAgenticFetchTool_Handler_FetchErrorShortCircuits(t *testing.T) {
	t.Parallel()

	fetcher := &stubFetcher{err: errors.New("fetch failed: 404")}
	runner := &stubRunner{err: errors.New("runner must not be called")}
	tool := AgenticFetchTool(fetcher, runner)

	raw, _ := json.Marshal(map[string]string{"prompt": "analyze", "url": "https://example.com/missing"})
	_, err := tool.Handler(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error when fetch fails")
	}
	if !strings.Contains(err.Error(), "fetch failed: 404") {
		t.Errorf("expected fetch error to propagate, got %q", err.Error())
	}
	if len(runner.calls) != 0 {
		t.Errorf("expected runner not to be called after a fetch error, got calls: %v", runner.calls)
	}
}

// TestAgenticFetchTool_Handler_RunPromptErrorPropagates verifies a runner
// failure is surfaced as the handler's error.
func TestAgenticFetchTool_Handler_RunPromptErrorPropagates(t *testing.T) {
	t.Parallel()

	fetcher := &stubFetcher{content: "page"}
	runner := &stubRunner{err: errors.New("model unavailable")}
	tool := AgenticFetchTool(fetcher, runner)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"prompt":"analyze"}`))
	if err == nil {
		t.Fatal("expected error when runner fails")
	}
	if !strings.Contains(err.Error(), "model unavailable") {
		t.Errorf("expected runner error to propagate, got %q", err.Error())
	}
}

var _ tools.WebFetcher = (*stubFetcher)(nil)
var _ tools.AgentRunner = (*stubRunner)(nil)
