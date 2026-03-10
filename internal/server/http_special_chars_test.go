package server

// TestPromptSpecialCharactersRoundTrip validates that prompts containing shell
// metacharacters, quotes, backslashes, newlines, and Unicode characters are
// preserved exactly when sent through the HTTP API.
//
// This is a regression test for Issue #27: shell-escaping bugs in manual
// testing scripts caused prompts with special characters to be silently
// corrupted before they reached the server. The server itself (via
// encoding/json) always handles special characters correctly. These tests
// confirm the server-side invariant so that any future regression — whether
// from a client-side escaping bug or an accidental server-side change — is
// caught immediately.
//
// The safe client-side pattern (using jq or python3 json.dumps) is documented
// in docs/runbooks/harnesscli-live-testing.md and scripts/curl-run.sh.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

// capturingProvider records the CompletionRequest it receives so tests can
// inspect how the prompt was passed through the full HTTP → runner → provider
// stack.
type capturingProvider struct {
	mu       sync.Mutex
	requests []harness.CompletionRequest
}

func (c *capturingProvider) Complete(_ context.Context, req harness.CompletionRequest) (harness.CompletionResult, error) {
	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.mu.Unlock()
	return harness.CompletionResult{Content: "ok"}, nil
}

func (c *capturingProvider) lastRequest() (harness.CompletionRequest, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.requests) == 0 {
		return harness.CompletionRequest{}, false
	}
	return c.requests[len(c.requests)-1], true
}

// postRunAndWait sends a POST /v1/runs request with the given prompt and waits
// for the run to reach a terminal state. Returns the run_id.
func postRunAndWait(t *testing.T, ts *httptest.Server, prompt string) string {
	t.Helper()

	// Use json.Marshal to safely construct the payload — no hand-built JSON strings.
	body, err := json.Marshal(map[string]string{"prompt": prompt})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	res, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /v1/runs: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", res.StatusCode)
	}

	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.RunID == "" {
		t.Fatal("expected non-empty run_id")
	}

	// Poll for terminal status.
	deadline := time.Now().Add(4 * time.Second)
	for {
		statusRes, err := http.Get(ts.URL + "/v1/runs/" + created.RunID)
		if err != nil {
			t.Fatalf("GET run: %v", err)
		}
		var state struct {
			Status string `json:"status"`
			Prompt string `json:"prompt"`
		}
		_ = json.NewDecoder(statusRes.Body).Decode(&state)
		_ = statusRes.Body.Close()

		if state.Status == "completed" || state.Status == "failed" {
			return created.RunID
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for run to complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// getRunPrompt fetches GET /v1/runs/{id} and returns the stored prompt field.
func getRunPrompt(t *testing.T, ts *httptest.Server, runID string) string {
	t.Helper()
	res, err := http.Get(ts.URL + "/v1/runs/" + runID)
	if err != nil {
		t.Fatalf("GET /v1/runs/%s: %v", runID, err)
	}
	defer res.Body.Close()

	var run struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(res.Body).Decode(&run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	return run.Prompt
}

// TestPromptSpecialCharactersRoundTrip is the primary regression test for
// Issue #27. It verifies that prompts with special characters survive the
// full HTTP API round-trip intact.
func TestPromptSpecialCharactersRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		prompt string
	}{
		{
			name:   "exclamation mark",
			prompt: `Hello! World`,
		},
		{
			name:   "single quotes",
			prompt: `It's a test`,
		},
		{
			name:   "double quotes",
			prompt: `Say "hello" to me`,
		},
		{
			name:   "backslash",
			prompt: `path\to\file`,
		},
		{
			name:   "backslash and exclamation",
			prompt: `Hello\! I'm building a recipe app...`,
		},
		{
			name:   "shell variable expansion chars",
			prompt: `echo $HOME && ls | grep foo`,
		},
		{
			name:   "backtick",
			prompt: "run `command` here",
		},
		{
			name:   "newline embedded",
			prompt: "line one\nline two",
		},
		{
			name:   "tab embedded",
			prompt: "col1\tcol2",
		},
		{
			name:   "unicode and emoji",
			prompt: "Hello 🌍 world — café résumé",
		},
		{
			name:   "embedded JSON fragment",
			prompt: `Parse this: {"key": "value", "nested": {"n": 1}}`,
		},
		{
			name:   "null byte escaped",
			prompt: "before\x00after",
		},
		{
			name:   "mixed special characters",
			prompt: `It's a "complex" path\to\file! $HOME && echo 🎉`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			prov := &capturingProvider{}
			runner := harness.NewRunner(prov, harness.NewRegistry(), harness.RunnerConfig{
				DefaultModel: "test-model",
				MaxSteps:     2,
			})
			ts := httptest.NewServer(New(runner))
			defer ts.Close()

			runID := postRunAndWait(t, ts, tc.prompt)

			// Verify the prompt is preserved in the stored run.
			gotPrompt := getRunPrompt(t, ts, runID)
			if gotPrompt != tc.prompt {
				t.Errorf("prompt mismatch via GET /v1/runs/%s\ngot:  %q\nwant: %q",
					runID, gotPrompt, tc.prompt)
			}

			// Verify the prompt reaches the provider in the first message.
			req, ok := prov.lastRequest()
			if !ok {
				t.Fatal("provider was never called")
			}
			if len(req.Messages) == 0 {
				t.Fatal("no messages in completion request")
			}

			// The user prompt must appear verbatim in at least one message
			// that has role "user".
			found := false
			for _, msg := range req.Messages {
				if msg.Role == "user" && msg.Content == tc.prompt {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("prompt not found verbatim in provider messages\nwant: %q\nmessages: %+v",
					tc.prompt, req.Messages)
			}
		})
	}
}

// TestPromptSpecialCharactersHTTPEncoding verifies that the HTTP request
// itself carries the correct JSON — i.e., json.Marshal encodes special
// characters as proper JSON escape sequences that survive decoding.
//
// This documents the correct client-side pattern: always use json.Marshal
// (or equivalent) rather than hand-building JSON strings.
func TestPromptSpecialCharactersHTTPEncoding(t *testing.T) {
	t.Parallel()

	tricky := []string{
		`Hello\! I'm "testing" $HOME`,
		"emoji: 🚀 and newline:\nend",
		`{"key": "val"}`,
		"tab\there",
	}

	for _, prompt := range tricky {
		// Encode as JSON and decode back — must be identity.
		encoded, err := json.Marshal(map[string]string{"prompt": prompt})
		if err != nil {
			t.Fatalf("json.Marshal(%q): %v", prompt, err)
		}
		var decoded map[string]string
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", string(encoded), err)
		}
		got := decoded["prompt"]
		if got != prompt {
			t.Errorf("JSON round-trip failed\ngot:  %q\nwant: %q\nJSON: %s",
				got, prompt, string(encoded))
		}
	}
}
