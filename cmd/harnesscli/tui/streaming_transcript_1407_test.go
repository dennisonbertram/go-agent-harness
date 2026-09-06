package tui_test

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"go-agent-harness/cmd/harnesscli/tui"
)

// Issue #1407: a streamed markdown answer must end up complete in the
// transcript. Live, "Created calc_test.go with table-driven tests covering
// positive, negative, mixed signs, and zeros." rendered as two fragments with
// the middle missing.
func TestStreamedMarkdown_NothingLost(t *testing.T) {
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "make calc")
	m = sendKey(m, tea.KeyEnter)
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-1"})
	m = m2.(tui.Model)
	m = sse(m, "run.started", "run-1", `{"prompt":"make calc","step":0}`)
	full := "Done. Summary:\n\n- Created `calc.go` with an `Add` function that returns the sum of two integers.\n- Created `calc_test.go` with table-driven tests covering positive, negative, mixed signs, and zeros.\n- `go test -v ./...` — all four subtests pass.\n\n```\n=== RUN   TestAdd\n--- PASS: TestAdd (0.00s)\nPASS\nok  \tcalc\t0.123s\n```\n"
	// Stream in small chunks so the rendered line count changes many times.
	acc := ""
	for i := 0; i < len(full); i += 7 {
		end := i + 7
		if end > len(full) {
			end = len(full)
		}
		acc += full[i:end]
		payload, _ := json.Marshal(map[string]any{"delta": full[i:end], "content": full[i:end], "step": 1})
		m = sse(m, "assistant.message.delta", "run-1", string(payload))
	}
	payload, _ := json.Marshal(map[string]any{"content": acc, "step": 1})
	m = sse(m, "assistant.message", "run-1", string(payload))
	m = sse(m, "run.completed", "run-1", `{"output":"x","step":1}`)
	view := m.View()
	for _, want := range []string{"returns the sum of two integers", "mixed signs, and zeros", "all four subtests pass", "--- PASS: TestAdd"} {
		if !strings.Contains(stripANSI(view), want) {
			t.Errorf("transcript lost %q\n%s", want, view)
		}
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case inEsc:
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
		case r == '\x1b':
			inEsc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
