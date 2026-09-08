package tui

import (
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/tooluse"
)

// TestCurrentSpinnerActionLadder pins issue #1415: the spinner says what is
// actually happening, and when several things are true at once the most
// specific one wins.
//
// Before this change currentSpinnerAction reported only a running tool and
// returned "" for every other state, leaving a rotating pool of fifteen
// synonyms for "thinking" to fill the silence.
func TestCurrentSpinnerActionLadder(t *testing.T) {
	const model = "gpt-4.1-mini"

	runningTool := func(m *Model) {
		m.activeToolCallID = "call_1"
		m.toolViews = map[string]tooluse.Model{
			"call_1": {ToolName: "bash", Status: "running"},
		}
	}

	for _, tc := range []struct {
		name  string
		setup func(*Model)
		want  string
	}{
		{
			name:  "nothing back yet names the model we are waiting on",
			setup: func(m *Model) {},
			want:  "Waiting for " + model,
		},
		{
			name:  "reasoning arrived but no text yet",
			setup: func(m *Model) { m.thinkingText = "considering the file layout" },
			want:  "Thinking",
		},
		{
			name: "assistant text streaming outranks reasoning",
			setup: func(m *Model) {
				m.thinkingText = "considering the file layout"
				m.responseStarted = true
			},
			want: "Writing response",
		},
		{
			name: "a running tool outranks everything",
			setup: func(m *Model) {
				m.thinkingText = "considering the file layout"
				m.responseStarted = true
				runningTool(m)
			},
			want: "Running bash",
		},
		{
			name: "a finished tool no longer counts",
			setup: func(m *Model) {
				m.activeToolCallID = "call_1"
				m.toolViews = map[string]tooluse.Model{
					"call_1": {ToolName: "bash", Status: "completed"},
				}
				m.responseStarted = true
			},
			want: "Writing response",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Model{selectedModel: model}
			tc.setup(m)
			if got := m.currentSpinnerAction(); got != tc.want {
				t.Fatalf("currentSpinnerAction() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCurrentSpinnerActionNeverEmptyWhileRunning guards the contract that makes
// the verb pool unnecessary: there is always something true to say.
func TestCurrentSpinnerActionNeverEmptyWhileRunning(t *testing.T) {
	m := &Model{}
	if got := m.currentSpinnerAction(); got == "" {
		t.Fatal("currentSpinnerAction() returned empty; the spinner would fall back to a generic label")
	}
}
