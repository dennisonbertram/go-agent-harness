package harness

import (
	"strings"
	"testing"

	"go-agent-harness/internal/forensics/redaction"
)

// TestRunnerEmit_RedactionPipeline_Wired verifies that when a RedactionPipeline
// is configured on the Runner, event payloads containing secrets are redacted
// before being appended to the run's event list.
func TestRunnerEmit_RedactionPipeline_Wired(t *testing.T) {
	t.Parallel()

	pipeline := redaction.NewPipeline(redaction.NewRedactor(nil), redaction.EventClassConfig{})

	prov := &stubProvider{turns: []CompletionResult{
		{Content: "done"},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
		RedactionPipeline:   pipeline,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	// All events should still be present; pipeline defaults to redacted mode (keep=true).
	for _, evt := range events {
		if evt.Payload == nil {
			t.Errorf("event %s (type=%s): nil payload", evt.ID, evt.Type)
		}
	}
}

// TestRunnerEmit_RedactionPipeline_RedactsSecrets verifies that a secret injected
// into an event payload via emit() is redacted in the stored event.
func TestRunnerEmit_RedactionPipeline_RedactsSecrets(t *testing.T) {
	t.Parallel()

	pipeline := redaction.NewPipeline(redaction.NewRedactor(nil), redaction.EventClassConfig{})

	prov := &stubProvider{turns: []CompletionResult{
		{Content: "done"},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
		RedactionPipeline:   pipeline,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	// Directly emit a synthetic event with a secret payload.
	runner.emit(run.ID, EventType("run.test"), map[string]any{
		"content": "postgres://user:secret@localhost:5432/prod",
	})

	events := collectEvents(t, runner, run.ID)
	var found bool
	for _, evt := range events {
		if string(evt.Type) != "run.test" {
			continue
		}
		found = true
		content, _ := evt.Payload["content"].(string)
		if !strings.Contains(content, "[REDACTED:connection_string]") {
			t.Errorf("secret not redacted in stored event payload: %q", content)
		}
	}
	if !found {
		t.Error("did not find synthetic run.test event")
	}
}

// TestRunnerEmit_NoRedactionPipeline_Passthrough verifies that without a
// RedactionPipeline, payloads are stored verbatim (no redaction).
func TestRunnerEmit_NoRedactionPipeline_Passthrough(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{
		{Content: "done"},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
		// No RedactionPipeline set.
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	secret := "postgres://user:secret@localhost:5432/prod"
	runner.emit(run.ID, EventType("run.test"), map[string]any{
		"content": secret,
	})

	events := collectEvents(t, runner, run.ID)
	var found bool
	for _, evt := range events {
		if string(evt.Type) != "run.test" {
			continue
		}
		found = true
		content, _ := evt.Payload["content"].(string)
		// Without a pipeline the payload is stored verbatim.
		if content != secret {
			t.Errorf("expected verbatim content %q, got %q", secret, content)
		}
	}
	if !found {
		t.Error("did not find synthetic run.test event")
	}
}

// TestRunnerEmit_RedactionPipeline_NoneMode verifies that an event whose type
// maps to StorageModeNone is dropped from the run's event list.
func TestRunnerEmit_RedactionPipeline_NoneMode(t *testing.T) {
	t.Parallel()

	cfg := redaction.EventClassConfig{
		"run.test.drop": redaction.StorageModeNone,
	}
	pipeline := redaction.NewPipeline(redaction.NewRedactor(nil), cfg)

	prov := &stubProvider{turns: []CompletionResult{
		{Content: "done"},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
		RedactionPipeline:   pipeline,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	runner.emit(run.ID, EventType("run.test.drop"), map[string]any{
		"content": "should be dropped",
	})

	events := collectEvents(t, runner, run.ID)
	for _, evt := range events {
		if string(evt.Type) == "run.test.drop" {
			t.Errorf("expected event run.test.drop to be dropped, but found it in run events")
		}
	}
}
