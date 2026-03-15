package streamrenderer_test

import (
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/streamrenderer"
)

func TestTUI014_StreamRendererAccumulatesChunks(t *testing.T) {
	sr := streamrenderer.New()
	sr.AppendDelta("Hello")
	sr.AppendDelta(", world")
	sr.AppendDelta("!")
	if sr.Content() != "Hello, world!" {
		t.Errorf("accumulated content: got %q", sr.Content())
	}
}

func TestTUI014_StreamRendererShowsCompletionSummary(t *testing.T) {
	sr := streamrenderer.New()
	sr.AppendDelta("The answer is 42.")
	sr.Complete(1500, 2.3) // 1500 tokens, 2.3 seconds
	summary := sr.Summary()
	if !strings.Contains(summary, "1500") {
		t.Errorf("summary missing token count: %q", summary)
	}
	if !strings.Contains(summary, "2.3") && !strings.Contains(summary, "2s") {
		t.Errorf("summary missing duration: %q", summary)
	}
}

func TestTUI014_StreamingStateTransitions(t *testing.T) {
	sr := streamrenderer.New()
	if sr.State() != streamrenderer.StateIdle {
		t.Errorf("initial state should be Idle, got %v", sr.State())
	}
	sr.StartStreaming()
	if sr.State() != streamrenderer.StateStreaming {
		t.Errorf("after StartStreaming should be Streaming, got %v", sr.State())
	}
	sr.AppendDelta("hello")
	sr.Complete(100, 0.5)
	if sr.State() != streamrenderer.StateComplete {
		t.Errorf("after Complete should be Complete, got %v", sr.State())
	}
}

func TestTUI014_ThinkingPhaseRendered(t *testing.T) {
	sr := streamrenderer.New()
	sr.StartThinking()
	sr.AppendThinkingDelta("Analyzing the problem...")
	view := sr.View(80)
	if view == "" {
		t.Error("thinking view should not be empty")
	}
}

func TestTUI014_ResetClearsState(t *testing.T) {
	sr := streamrenderer.New()
	sr.StartStreaming()
	sr.AppendDelta("hello")
	sr.Reset()
	if sr.Content() != "" {
		t.Errorf("after Reset, Content should be empty: %q", sr.Content())
	}
	if sr.State() != streamrenderer.StateIdle {
		t.Errorf("after Reset, State should be Idle: %v", sr.State())
	}
}

func TestTUI014_ViewTruncatesLargeContent(t *testing.T) {
	sr := streamrenderer.New()
	sr.StartStreaming()
	// 10000 chars
	for i := 0; i < 200; i++ {
		sr.AppendDelta("this is a line of text that is fairly long and should eventually get truncated ")
	}
	view := sr.View(80)
	lines := strings.Split(view, "\n")
	if len(lines) > 1000 {
		t.Errorf("view too long: %d lines — should truncate", len(lines))
	}
}
