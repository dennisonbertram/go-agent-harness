package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunParsesPromptFlagsIntoRunCreateRequest(t *testing.T) {
	var captured runCreateRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"run_id":"run_prompt","status":"queued"}`)
	})
	mux.HandleFunc("/v1/runs/run_prompt/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = io.WriteString(w, "event: run.completed\n")
		_, _ = io.WriteString(w, "data: {\"id\":\"e1\",\"run_id\":\"run_prompt\",\"type\":\"run.completed\"}\n\n")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	origRequestClient := requestHTTPClient
	origStreamClient := streamHTTPClient
	origStdout := stdout
	origStderr := stderr
	defer func() {
		requestHTTPClient = origRequestClient
		streamHTTPClient = origStreamClient
		stdout = origStdout
		stderr = origStderr
	}()

	requestHTTPClient = ts.Client()
	streamHTTPClient = ts.Client()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}

	code := run([]string{
		"-base-url=" + ts.URL,
		"-prompt=review this",
		"-agent-intent=code_review",
		"-task-context=Review payment retry logic",
		"-prompt-profile=openai_gpt5",
		"-prompt-behavior=precise,safe",
		"-prompt-behavior=focused",
		"-prompt-talent=review",
		"-prompt-custom=Only produce findings.",
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	if captured.AgentIntent != "code_review" {
		t.Fatalf("unexpected agent intent: %q", captured.AgentIntent)
	}
	if captured.TaskContext != "Review payment retry logic" {
		t.Fatalf("unexpected task context: %q", captured.TaskContext)
	}
	if captured.PromptProfile != "openai_gpt5" {
		t.Fatalf("unexpected prompt profile: %q", captured.PromptProfile)
	}
	if captured.PromptExtensions == nil {
		t.Fatalf("expected prompt extensions payload")
	}
	if got := strings.Join(captured.PromptExtensions.Behaviors, ","); got != "precise,safe,focused" {
		t.Fatalf("unexpected behaviors: %q", got)
	}
	if got := strings.Join(captured.PromptExtensions.Talents, ","); got != "review" {
		t.Fatalf("unexpected talents: %q", got)
	}
	if captured.PromptExtensions.Custom != "Only produce findings." {
		t.Fatalf("unexpected custom text: %q", captured.PromptExtensions.Custom)
	}
}
