package harness

import (
	"strings"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

// The reported failure in one sentence: a background job ran, exited, printed
// correctly, and told nobody — so the model reported to the user that it had
// never fired. These pin the two paths that now carry the result.

func TestJobCompletionIsQueuedWhenNoRunIsListening(t *testing.T) {
	bridge := NewJobEventBridge()

	// No runner bound: this is the normal case, because a background job
	// usually finishes after the run that started it has ended.
	bridge.JobCompleted(htools.JobCompletion{
		ShellID:        "job_1",
		Command:        "sleep 15; echo hello dennison",
		ConversationID: "conv-a",
		Output:         "hello dennison",
	})

	notices := bridge.TakeNotices("conv-a")
	if len(notices) != 1 {
		t.Fatalf("got %d queued notices, want 1 — a completion with no live run was dropped", len(notices))
	}
	if notices[0].Output != "hello dennison" {
		t.Errorf("queued output = %q, want %q", notices[0].Output, "hello dennison")
	}
}

func TestJobNoticesAreConsumedSoTheyReportOnce(t *testing.T) {
	bridge := NewJobEventBridge()
	bridge.JobCompleted(htools.JobCompletion{ShellID: "job_1", ConversationID: "conv-a", Output: "done"})

	if got := len(bridge.TakeNotices("conv-a")); got != 1 {
		t.Fatalf("first take returned %d notices, want 1", got)
	}
	if got := len(bridge.TakeNotices("conv-a")); got != 0 {
		t.Errorf("second take returned %d notices, want 0 — the same job would be "+
			"reported to the model on every subsequent turn", got)
	}
}

func TestJobNoticesAreScopedToTheirConversation(t *testing.T) {
	bridge := NewJobEventBridge()
	bridge.JobCompleted(htools.JobCompletion{ShellID: "job_1", ConversationID: "conv-a", Output: "a"})
	bridge.JobCompleted(htools.JobCompletion{ShellID: "job_2", ConversationID: "conv-b", Output: "b"})

	if got := len(bridge.TakeNotices("conv-a")); got != 1 {
		t.Errorf("conv-a got %d notices, want 1", got)
	}
	if got := len(bridge.TakeNotices("conv-b")); got != 1 {
		t.Errorf("conv-b got %d notices, want 1", got)
	}
}

func TestFormatJobNoticesReportsCommandStatusAndOutput(t *testing.T) {
	out := FormatJobNotices([]htools.JobCompletion{{
		ShellID:  "job_1",
		Command:  "sleep 15; echo hello dennison",
		ExitCode: 0,
		Output:   "hello dennison",
	}})

	for _, want := range []string{"hello dennison", "job_1", "exit 0", "sleep 15"} {
		if !strings.Contains(out, want) {
			t.Errorf("notice %q does not mention %q", out, want)
		}
	}
}

// A job that produced nothing must say so, rather than rendering an empty
// bullet the model has to interpret.
func TestFormatJobNoticesMarksSilentJobs(t *testing.T) {
	out := FormatJobNotices([]htools.JobCompletion{{ShellID: "job_1", Command: "true", ExitCode: 0}})
	if !strings.Contains(out, "(no output)") {
		t.Errorf("notice %q does not mark a silent job", out)
	}
}

func TestFormatJobNoticesEmptyWhenNothingFinished(t *testing.T) {
	if got := FormatJobNotices(nil); got != "" {
		t.Errorf("FormatJobNotices(nil) = %q, want empty so no turn is injected", got)
	}
}

// A background job can print megabytes; the model needs to know what happened,
// not to have its context filled by one command.
func TestJobNoticeOutputIsBounded(t *testing.T) {
	huge := strings.Repeat("x", maxQueuedJobNoticeBytes*3)
	out := FormatJobNotices([]htools.JobCompletion{{ShellID: "job_1", Command: "spew", Output: huge}})
	if len(out) > maxQueuedJobNoticeBytes*2 {
		t.Errorf("notice is %d bytes, want it bounded near %d", len(out), maxQueuedJobNoticeBytes)
	}
	if !strings.Contains(out, "truncated") {
		t.Error("a truncated notice must say so")
	}
}

func TestQueuedNoticesAreBounded(t *testing.T) {
	bridge := NewJobEventBridge()
	for i := 0; i < maxQueuedJobNotices*3; i++ {
		bridge.JobCompleted(htools.JobCompletion{ShellID: "job", ConversationID: "conv-a"})
	}
	if got := len(bridge.TakeNotices("conv-a")); got > maxQueuedJobNotices {
		t.Errorf("queued %d notices, want at most %d — an unbounded queue is a leak",
			got, maxQueuedJobNotices)
	}
}

func TestRunnerIsLiveRun(t *testing.T) {
	runner := &Runner{
		runs: map[string]*runState{
			"live":       {terminated: false},
			"terminated": {terminated: true},
			"nil":        nil,
		},
	}

	for _, tc := range []struct {
		name  string
		runID string
		want  bool
	}{
		{name: "empty", runID: "", want: false},
		{name: "unknown", runID: "unknown", want: false},
		{name: "nil state", runID: "nil", want: false},
		{name: "live", runID: "live", want: true},
		{name: "terminated", runID: "terminated", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runner.isLiveRun(tc.runID); got != tc.want {
				t.Fatalf("isLiveRun(%q) = %v, want %v", tc.runID, got, tc.want)
			}
		})
	}
}

func TestRunnerTakeJobNoticeBlockConsumesQueuedCompletion(t *testing.T) {
	runner := &Runner{}
	if got := runner.takeJobNoticeBlock("conv-a"); got != "" {
		t.Fatalf("take without source = %q, want empty", got)
	}

	bridge := NewJobEventBridge()
	bridge.JobCompleted(htools.JobCompletion{
		ShellID:        "job_1",
		Command:        "echo done",
		ConversationID: "conv-a",
		Output:         "done",
	})
	runner.SetJobNotices(bridge)

	got := runner.takeJobNoticeBlock("conv-a")
	for _, want := range []string{"echo done", "done", "job_1"} {
		if !strings.Contains(got, want) {
			t.Errorf("notice %q does not contain %q", got, want)
		}
	}
	if got := runner.takeJobNoticeBlock("conv-a"); got != "" {
		t.Fatalf("second take = %q, want empty after consuming notice", got)
	}
}

func TestRunnerEmitToConversationClonesPayload(t *testing.T) {
	ch := make(chan Event, 1)
	runner := &Runner{
		convSubscribers: map[string]map[chan Event]struct{}{
			"conv-a": {ch: {}},
		},
	}
	payload := map[string]any{
		"output": "done",
		"nested": map[string]any{"state": "original"},
	}

	runner.emitToConversation("conv-a", "run-a", EventBackgroundJobCompleted, payload)
	payload["output"] = "mutated"
	payload["nested"].(map[string]any)["state"] = "mutated"

	select {
	case event := <-ch:
		if event.RunID != "run-a" || event.Type != EventBackgroundJobCompleted {
			t.Fatalf("event identity = run:%q type:%q", event.RunID, event.Type)
		}
		if got := event.Payload["output"]; got != "done" {
			t.Fatalf("event output = %v, want cloned value %q", got, "done")
		}
		nested, ok := event.Payload["nested"].(map[string]any)
		if !ok || nested["state"] != "original" {
			t.Fatalf("nested payload = %#v, want cloned original", event.Payload["nested"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for conversation event")
	}
}
