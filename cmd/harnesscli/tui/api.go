package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"go-agent-harness/cmd/harnesscli/tui/components/modelswitcher"
)

type runCreateRequest struct {
	Prompt          string `json:"prompt"`
	ConversationID  string `json:"conversation_id,omitempty"`
	Model           string `json:"model,omitempty"`
	ProviderName    string `json:"provider_name,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

type runCreateResponse struct {
	RunID string `json:"run_id"`
}

// startRunCmd returns a tea.Cmd that POSTs a run to the harness and emits
// RunStartedMsg on success or RunFailedMsg on error.
// conversationID may be empty for the first message in a new conversation;
// subsequent messages should pass the run ID returned by the first run so that
// the harness groups them under the same conversation.
func startRunCmd(baseURL, prompt, conversationID, model, provider, reasoningEffort string) tea.Cmd {
	return func() tea.Msg {
		body, _ := json.Marshal(runCreateRequest{
			Prompt:          prompt,
			ConversationID:  conversationID,
			Model:           model,
			ProviderName:    provider,
			ReasoningEffort: reasoningEffort,
		})
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

// modelsResponse matches the JSON body returned by GET /v1/models.
type modelsResponse struct {
	Models []modelswitcher.ServerModelEntry `json:"models"`
}

// fetchModelsCmd fetches the model list from the server's /v1/models endpoint.
// On success it emits ModelsFetchedMsg; on failure it emits ModelsFetchErrorMsg.
func fetchModelsCmd(baseURL string) tea.Cmd {
	return func() tea.Msg {
		url := strings.TrimRight(baseURL, "/") + "/v1/models"
		resp, err := http.Get(url) //nolint:noctx
		if err != nil {
			return ModelsFetchErrorMsg{Err: err.Error()}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return ModelsFetchErrorMsg{Err: fmt.Sprintf("server returned %d", resp.StatusCode)}
		}
		var mr modelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
			return ModelsFetchErrorMsg{Err: err.Error()}
		}
		return ModelsFetchedMsg{Models: mr.Models}
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

// formatRunError formats a run.failed error string for the viewport.
// The harness error looks like:
//
//	"provider completion failed: openai request failed (429): {\"error\":{...}}"
//
// We split at the first '{' to separate the prose prefix from any embedded JSON,
// then render the JSON fields as human-readable key: value lines.
func formatRunError(errStr string) []string {
	if errStr == "" {
		return []string{"✗ run failed"}
	}

	// Split prose prefix from embedded JSON object/array.
	prefix := errStr
	jsonPart := ""
	if idx := strings.Index(errStr, "{"); idx >= 0 {
		prefix = strings.TrimRight(errStr[:idx], ": ")
		jsonPart = errStr[idx:]
	}

	lines := []string{"✗ " + prefix}

	if jsonPart != "" {
		var obj map[string]any
		if err := json.Unmarshal([]byte(jsonPart), &obj); err == nil {
			for _, line := range flattenJSON(obj, "  ") {
				lines = append(lines, line)
			}
		} else {
			// Not valid JSON — just append as-is.
			lines = append(lines, "  "+jsonPart)
		}
	}

	return lines
}

// flattenJSON renders a JSON object as indented "key: value" lines.
// Nested objects are indented further. Arrays are shown as comma-joined values.
func flattenJSON(obj map[string]any, indent string) []string {
	var lines []string
	for k, v := range obj {
		switch val := v.(type) {
		case map[string]any:
			lines = append(lines, indent+k+":")
			lines = append(lines, flattenJSON(val, indent+"  ")...)
		case nil:
			// skip null fields
		default:
			lines = append(lines, fmt.Sprintf("%s%s: %v", indent, k, val))
		}
	}
	return lines
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
