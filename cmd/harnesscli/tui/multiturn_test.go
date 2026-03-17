package tui_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

// TestModel_ConversationID_SetAfterFirstRun verifies that after receiving a
// RunStartedMsg, the Model stores the RunID as its conversationID.
func TestModel_ConversationID_SetAfterFirstRun(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	// Send a window size to make the model ready.
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model := m2.(tui.Model)

	// Simulate a RunStartedMsg arriving.
	m3, _ := model.Update(tui.RunStartedMsg{RunID: "run-1"})
	result := m3.(tui.Model)

	if result.ConversationID() != "run-1" {
		t.Errorf("ConversationID should be 'run-1' after RunStartedMsg, got %q", result.ConversationID())
	}
}

// TestModel_ConversationID_PersistsAcrossRuns verifies that after a run
// completes (SSEDoneMsg), the conversationID is still set.
func TestModel_ConversationID_PersistsAcrossRuns(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model := m2.(tui.Model)

	// First run starts.
	m3, _ := model.Update(tui.RunStartedMsg{RunID: "conv-abc"})
	m4, _ := m3.(tui.Model).Update(tui.SSEDoneMsg{EventType: "run.completed"})
	result := m4.(tui.Model)

	if result.ConversationID() != "conv-abc" {
		t.Errorf("ConversationID should persist after SSEDoneMsg, got %q", result.ConversationID())
	}
}

// TestRegression_MultiTurnConversationIDPropagated uses a mock HTTP server to
// verify that:
//   - The first message POSTs with no conversation_id.
//   - After receiving RunStartedMsg{RunID: "conv-abc"}, the second message
//     POSTs with conversation_id = "conv-abc".
func TestRegression_MultiTurnConversationIDPropagated(t *testing.T) {
	type capturedBody struct {
		Prompt         string `json:"prompt"`
		ConversationID string `json:"conversation_id"`
	}

	var bodies []capturedBody

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/runs" {
			raw, _ := io.ReadAll(r.Body)
			var b capturedBody
			_ = json.Unmarshal(raw, &b)
			bodies = append(bodies, b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"run_id":"conv-abc"}`))
		}
	}))
	defer srv.Close()

	cfg := tui.DefaultTUIConfig()
	cfg.BaseURL = srv.URL
	m := tui.New(cfg)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model := m2.(tui.Model)

	// First message — no conversationID yet. Submit a user message and
	// execute the resulting tea.Cmd to fire the HTTP call.
	m3, cmd1 := model.Update(inputarea.CommandSubmittedMsg{Value: "hello"})
	model = m3.(tui.Model)

	if cmd1 != nil {
		// Execute the cmd to make the HTTP call.
		_ = cmd1()
	}

	if len(bodies) != 1 {
		t.Fatalf("expected 1 POST body captured, got %d", len(bodies))
	}
	if bodies[0].ConversationID != "" {
		t.Errorf("first POST should have no conversation_id, got %q", bodies[0].ConversationID)
	}

	// Simulate the harness replying with RunStartedMsg for run "conv-abc".
	m4, _ := model.Update(tui.RunStartedMsg{RunID: "conv-abc"})
	model = m4.(tui.Model)

	if model.ConversationID() != "conv-abc" {
		t.Fatalf("expected conversationID='conv-abc', got %q", model.ConversationID())
	}

	// Second message — should carry conversation_id = "conv-abc".
	_, cmd2 := model.Update(inputarea.CommandSubmittedMsg{Value: "follow-up"})

	if cmd2 != nil {
		_ = cmd2()
	}

	if len(bodies) != 2 {
		t.Fatalf("expected 2 POST bodies captured, got %d", len(bodies))
	}
	if bodies[1].ConversationID != "conv-abc" {
		t.Errorf("second POST should have conversation_id='conv-abc', got %q", bodies[1].ConversationID)
	}
}
