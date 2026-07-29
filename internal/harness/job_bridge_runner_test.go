package harness

import (
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

type staticJobNoticeSource struct {
	notices []htools.JobCompletion
}

func (s *staticJobNoticeSource) TakeNotices(string) []htools.JobCompletion {
	return append([]htools.JobCompletion(nil), s.notices...)
}

func TestRunnerLiveRunState(t *testing.T) {
	runner := &Runner{runs: map[string]*runState{}}
	if runner.isLiveRun("") || runner.isLiveRun("missing") {
		t.Fatal("empty and missing run ids must not be live")
	}

	runner.runs["live"] = &runState{run: Run{ID: "live"}}
	runner.runs["done"] = &runState{run: Run{ID: "done"}, terminated: true}
	if !runner.isLiveRun("live") {
		t.Fatal("non-terminated run was not reported live")
	}
	if runner.isLiveRun("done") {
		t.Fatal("terminated run was reported live")
	}
}

func TestRunnerTakesAndFormatsQueuedJobNotices(t *testing.T) {
	runner := &Runner{}
	if got := runner.takeJobNoticeBlock("conv"); got != "" {
		t.Fatalf("nil source returned %q", got)
	}

	runner.SetJobNotices(&staticJobNoticeSource{notices: []htools.JobCompletion{{
		ShellID: "job-1",
		Command: "echo done",
		Output:  "done",
	}}})
	got := runner.takeJobNoticeBlock("conv")
	if got == "" {
		t.Fatal("queued completion rendered as an empty notice")
	}
}

func TestRunnerEmitsBackgroundEventToConversationSubscribers(t *testing.T) {
	ch := make(chan Event, 1)
	runner := &Runner{
		convSubscribers: map[string]map[chan Event]struct{}{
			"conv": {ch: {}},
		},
	}
	payload := map[string]any{"output": "done"}

	runner.emitToConversation("conv", "run-1", EventBackgroundJobCompleted, payload)
	payload["output"] = "mutated"

	select {
	case event := <-ch:
		if event.RunID != "run-1" || event.Type != EventBackgroundJobCompleted {
			t.Fatalf("unexpected event: %+v", event)
		}
		if event.Payload["output"] != "done" {
			t.Fatalf("event payload aliased caller data: %+v", event.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("conversation subscriber did not receive the event")
	}

	runner.emitToConversation("", "run-1", EventBackgroundJobCompleted, nil)
	runner.emitToConversation("unsubscribed", "run-1", EventBackgroundJobCompleted, nil)
}
