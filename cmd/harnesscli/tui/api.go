package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type runCreateRequest struct {
	Prompt string `json:"prompt"`
}

type runCreateResponse struct {
	RunID string `json:"run_id"`
}

// startRunCmd returns a tea.Cmd that POSTs a run to the harness and emits
// RunStartedMsg on success or RunFailedMsg on error.
func startRunCmd(baseURL, prompt string) tea.Cmd {
	return func() tea.Msg {
		body, _ := json.Marshal(runCreateRequest{Prompt: prompt})
		url := strings.TrimRight(baseURL, "/") + "/v1/runs"
		resp, err := http.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			return RunFailedMsg{Error: err.Error()}
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return RunFailedMsg{Error: fmt.Sprintf("start run: HTTP %d", resp.StatusCode)}
		}
		var created runCreateResponse
		if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
			return RunFailedMsg{Error: fmt.Sprintf("decode run response: %s", err.Error())}
		}
		return RunStartedMsg{RunID: created.RunID}
	}
}

// pollSSECmd reads one message from the SSE channel and returns it as a tea.Msg.
// It blocks until a message is available or the channel is closed.
// Call this again after every SSEEventMsg/SSEDropMsg to continue polling.
func pollSSECmd(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return SSEDoneMsg{EventType: "bridge.closed"}
		}
		return msg
	}
}

// sseEventsURL builds the SSE endpoint URL for a given run ID.
func sseEventsURL(baseURL, runID string) string {
	return strings.TrimRight(baseURL, "/") + "/v1/runs/" + runID + "/events"
}

// startSSEForRun starts the SSE bridge for the given run and returns the channel
// and cancel func.
func startSSEForRun(baseURL, runID string) (<-chan tea.Msg, func()) {
	url := sseEventsURL(baseURL, runID)
	return StartSSEBridge(context.Background(), url)
}
