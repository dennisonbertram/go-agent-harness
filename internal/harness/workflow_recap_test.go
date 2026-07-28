package harness

import (
	"reflect"
	"strings"
	"testing"

	"go-agent-harness/internal/store"
)

func TestPatchFilesExtractsMutatedPaths(t *testing.T) {
	patch := `*** Begin Patch
*** Add File: cmd/harnesscli/improve.go
+package main
*** Update File: internal/harness/workflow_recap.go
@@
-old
+new
*** Delete File: docs/context/stale-note.md
*** End Patch`

	got := patchFiles(patch)
	want := []string{
		"cmd/harnesscli/improve.go",
		"internal/harness/workflow_recap.go",
		"docs/context/stale-note.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("patchFiles() = %#v, want %#v", got, want)
	}
}

// The tests below cover the rest of the recap builder — the summary a
// continuation run reads to pick up where a previous run stopped. Its value
// depends entirely on the trace extraction being right: a recap that misses
// changed files or test commands sends the next run in blind.

func recapToolCallEvent(tool, args string) Event {
	return Event{Type: EventToolCallStarted, Payload: map[string]any{"tool": tool, "arguments": args}}
}

func newTestRecap(cause string, changed, tests []string) *store.WorkflowRecap {
	return &store.WorkflowRecap{FailureCause: cause, ChangedFiles: changed, TestsRun: tests}
}

func TestBuildWorkflowRecap(t *testing.T) {
	run := Run{ID: "run-99", Prompt: "fix the failing parser test", Output: "done", Status: RunStatusCompleted}
	events := []Event{
		recapToolCallEvent("write", `{"path":"src/parser.go"}`),
		recapToolCallEvent("bash", `{"command":"go test ./internal/parser/"}`),
		recapToolCallEvent("bash", `{"command":"ls -la"}`),
	}

	recap := buildWorkflowRecap(run, nil, events)
	if recap == nil {
		t.Fatal("expected a recap")
	}
	if len(recap.ChangedFiles) != 1 || recap.ChangedFiles[0] != "src/parser.go" {
		t.Errorf("changed files = %v, want [src/parser.go]", recap.ChangedFiles)
	}
	if len(recap.TestsRun) != 1 || !strings.Contains(recap.TestsRun[0], "go test") {
		t.Errorf("tests run = %v, want the go test command", recap.TestsRun)
	}
	if recap.FixPattern != "changed files and ran regression checks" {
		t.Errorf("fix pattern = %q", recap.FixPattern)
	}
	if !strings.Contains(recap.NextContinuationPrompt, "run-99") ||
		!strings.Contains(recap.NextContinuationPrompt, "fix the failing parser test") {
		t.Errorf("next prompt should name the run and goal: %q", recap.NextContinuationPrompt)
	}

	// cloneWorkflowRecap must produce an independent copy: the recap is handed
	// to a store and to callers, and neither may corrupt the other.
	clone := cloneWorkflowRecap(recap)
	clone.ChangedFiles[0] = "mutated"
	if recap.ChangedFiles[0] != "src/parser.go" {
		t.Error("cloneWorkflowRecap must deep-copy the changed-file slice")
	}
	if cloneWorkflowRecap(nil) != nil {
		t.Error("cloning nil should yield nil")
	}
}

func TestLooksLikeTestCommand(t *testing.T) {
	for _, cmd := range []string{
		"go test ./...", "GO TEST ./...", "npm test", "pnpm test",
		"yarn test", "pytest -q", "cargo test", "make test-regression",
	} {
		if !looksLikeTestCommand(cmd) {
			t.Errorf("%q should be recognized as a test command", cmd)
		}
	}
	for _, cmd := range []string{"go build ./...", "ls -la", "git status", ""} {
		if looksLikeTestCommand(cmd) {
			t.Errorf("%q should NOT be recognized as a test command", cmd)
		}
	}
}

func TestInferFixPattern(t *testing.T) {
	withOutput := Run{Output: "some output"}
	bare := Run{}

	if got := inferFixPattern(newTestRecap("cause", nil, []string{"go test"}), withOutput); got != "captured failure cause and ran regression checks" {
		t.Errorf("failure cause with tests = %q", got)
	}
	if got := inferFixPattern(newTestRecap("cause", nil, nil), withOutput); got != "captured failure cause for continuation" {
		t.Errorf("failure cause alone = %q", got)
	}
	if got := inferFixPattern(newTestRecap("", []string{"a.go"}, []string{"go test"}), withOutput); got != "changed files and ran regression checks" {
		t.Errorf("files and tests = %q", got)
	}
	if got := inferFixPattern(newTestRecap("", []string{"a.go"}, nil), withOutput); got != "changed files for requested task" {
		t.Errorf("files alone = %q", got)
	}
	if got := inferFixPattern(newTestRecap("", nil, []string{"go test"}), withOutput); got != "ran verification checks" {
		t.Errorf("tests alone = %q", got)
	}
	if got := inferFixPattern(newTestRecap("", nil, nil), withOutput); got != "completed requested task" {
		t.Errorf("output only = %q", got)
	}
	if got := inferFixPattern(newTestRecap("", nil, nil), bare); got != "recorded terminal run state" {
		t.Errorf("nothing at all = %q", got)
	}
}

func TestNextContinuationPrompt(t *testing.T) {
	got := nextContinuationPrompt(Run{ID: "r1", Prompt: "  do the thing  "})
	if !strings.Contains(got, "do the thing") || !strings.Contains(got, "r1") {
		t.Errorf("prompt = %q, want run id and trimmed goal", got)
	}

	// A long goal is truncated by RUNES, not bytes, so a multi-byte goal is
	// not cut mid-character.
	long := strings.Repeat("é", 400)
	got = nextContinuationPrompt(Run{ID: "r2", Prompt: long})
	if !strings.Contains(got, "...") {
		t.Errorf("a long goal should be truncated with an ellipsis: %q", got)
	}
	if n := strings.Count(got, "é"); n > 160 {
		t.Errorf("truncation should cap at 160 runes, counted %d", n)
	}

	if got := nextContinuationPrompt(Run{ID: "r3"}); !strings.Contains(got, "the prior task") {
		t.Errorf("an empty goal should fall back to a placeholder: %q", got)
	}
}

func TestToolArgsMapAndFirstStringArg(t *testing.T) {
	if got := toolArgsMap(Event{Payload: map[string]any{}}); got != nil {
		t.Errorf("an event with no arguments should yield nil, got %v", got)
	}
	if got := toolArgsMap(Event{Payload: map[string]any{"arguments": "   "}}); got != nil {
		t.Errorf("blank arguments should yield nil, got %v", got)
	}
	if got := toolArgsMap(Event{Payload: map[string]any{"arguments": "not json"}}); got != nil {
		t.Errorf("unparseable arguments should yield nil, got %v", got)
	}
	if got := toolArgsMap(Event{Payload: map[string]any{"arguments": `{"path":"a.go"}`}}); got["path"] != "a.go" {
		t.Errorf("parsed args = %v", got)
	}

	args := map[string]any{"file": "b.go", "count": 3}
	if got := firstStringArg(args, "path", "file"); got != "b.go" {
		t.Errorf("firstStringArg = %q, want the first present string key", got)
	}
	if got := firstStringArg(args, "count"); got != "" {
		t.Errorf("a non-string value must not be returned, got %q", got)
	}
	if got := firstStringArg(nil, "path"); got != "" {
		t.Errorf("nil args should yield empty, got %q", got)
	}
}

func TestUsefulAndTestCommandsFromTrace(t *testing.T) {
	events := []Event{
		recapToolCallEvent("bash", `{"command":"go test ./..."}`),
		recapToolCallEvent("bash", `{"command":"go build ./..."}`),
		recapToolCallEvent("bash", `{"command":"go test ./..."}`), // duplicate
		recapToolCallEvent("write", `{"path":"a.go"}`),            // not a command
		{Type: EventRunStepStarted, Payload: map[string]any{"tool": "bash", "arguments": `{"command":"ignored"}`}},
	}

	useful := usefulCommandsFromTrace(events)
	if strings.Contains(strings.Join(useful, "|"), "ignored") {
		t.Errorf("useful commands = %v, must ignore non-tool-call events", useful)
	}

	tests := testCommandsFromTrace(events)
	if len(tests) != 1 || !strings.Contains(tests[0], "go test") {
		t.Errorf("test commands = %v, want a single deduped go test entry", tests)
	}
}
