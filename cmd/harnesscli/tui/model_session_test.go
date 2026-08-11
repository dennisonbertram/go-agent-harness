package tui_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

// ─── BT-005: /new resets conversationID ──────────────────────────────────────

// TestSS005_NewCommandResetsConversationID verifies that issuing /new clears
// the conversationID field so the next run starts a fresh conversation.
func TestSS005_NewCommandResetsConversationID(t *testing.T) {
	m := initModel(t, 80, 24)

	// Simulate a run having started (sets conversationID).
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-111"})
	m = m2.(tui.Model)

	if m.ConversationID() != "run-111" {
		t.Fatalf("pre-condition: ConversationID must be %q, got %q", "run-111", m.ConversationID())
	}

	// Issue /new.
	m = sendSlashCommand(m, "/new")

	if m.ConversationID() != "" {
		t.Errorf("/new must reset ConversationID to '', got %q", m.ConversationID())
	}
}

// ─── BT-006: /sessions opens the session picker overlay ──────────────────────

// TestSS006_SessionsCommandOpensOverlay verifies that /sessions opens the
// sessions overlay.
func TestSS006_SessionsCommandOpensOverlay(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/sessions")

	if !m.OverlayActive() {
		t.Fatal("OverlayActive() must be true after /sessions")
	}
	if m.ActiveOverlay() != "sessions" {
		t.Errorf("ActiveOverlay(): want %q, got %q", "sessions", m.ActiveOverlay())
	}
}

// ─── BT-007: Selecting a session updates conversationID ──────────────────────

// TestSS007_SessionPickerSelectUpdatesConversationID verifies that handling a
// SessionSelectedMsg changes the conversationID to the selected entry's ID.
func TestSS007_SessionPickerSelectUpdatesConversationID(t *testing.T) {
	m := initModel(t, 80, 24)
	// Open sessions overlay first.
	m = sendSlashCommand(m, "/sessions")

	// Simulate the user picking a session from the picker.
	selectedMsg := tui.SessionPickerSelectedMsg{SessionID: "conv-abc-999"}
	m2, _ := m.Update(selectedMsg)
	m = m2.(tui.Model)

	if m.ConversationID() != "conv-abc-999" {
		t.Errorf("ConversationID after session select: want %q, got %q",
			"conv-abc-999", m.ConversationID())
	}

	// Overlay should close after selection.
	if m.OverlayActive() && m.ActiveOverlay() == "sessions" {
		t.Error("sessions overlay should close after a session is selected")
	}
}

// TestIssue1246_SessionSelectionStartsBoundaryReplay proves the real keyboard
// seam: /sessions plus Enter must produce the model's selection message and
// start its atomic replay-to-live handoff rather than silently swallowing Enter.
func TestIssue1246_SessionSelectionRehydratesDurableTranscript(t *testing.T) {
	boundaryRequest := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/conversations/conv-selected/messages" {
			t.Error("supported selected-session path must not GET message history")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/v1/conversations/conv-selected/events" {
			t.Errorf("boundary request = %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.Header.Get("X-Harness-Conversation-Replay-Boundary"); got != "snapshot" {
			t.Errorf("boundary header = %q, want snapshot", got)
		}
		if got := r.Header.Get("Last-Event-ID"); got != "" {
			t.Errorf("Last-Event-ID = %q, want empty initial boundary cursor", got)
		}
		w.Header().Set("X-Harness-Conversation-Replay-Boundary", "snapshot")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: conversation.replay.completed\ndata: {\"type\":\"conversation.replay.completed\",\"payload\":{\"messages\":[],\"last_event_id\":\"snapshot:1\"}}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		boundaryRequest <- struct{}{}
	}))
	defer srv.Close()

	cfg := tui.DefaultTUIConfig()
	cfg.BaseURL = srv.URL
	m := tui.New(cfg)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = m2.(tui.Model)
	m.SessionStore().Add(tui.StoredSessionEntry{ID: "conv-selected", StartedAt: time.Now()})
	m = sendSlashCommand(m, "/sessions")

	m2, enterCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)
	if enterCmd == nil {
		t.Fatal("Enter on sessions overlay must emit a selected-session command")
	}

	var selected tui.SessionPickerSelectedMsg
	selectedFound := false
	switch result := enterCmd().(type) {
	case tui.SessionPickerSelectedMsg:
		selected = result
		selectedFound = true
	case tea.BatchMsg:
		for _, sub := range result {
			if sub == nil {
				continue
			}
			if msg, ok := sub().(tui.SessionPickerSelectedMsg); ok {
				selected = msg
				selectedFound = true
				break
			}
		}
	default:
		t.Fatalf("sessions Enter command = %T, want SessionPickerSelectedMsg or batch", result)
	}
	if !selectedFound {
		t.Fatal("sessions Enter did not translate picker selection to SessionPickerSelectedMsg")
	}

	m2, selectionCmd := m.Update(selected)
	m = m2.(tui.Model)
	if selectionCmd == nil {
		t.Fatal("selected session must enqueue atomic conversation replay")
	}
	batch, ok := selectionCmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("selected session must batch status and boundary stream startup")
	}
	for _, cmd := range batch {
		if cmd != nil {
			_ = cmd()
		}
	}
	select {
	case <-boundaryRequest:
	case <-time.After(time.Second):
		t.Fatal("selected session did not start the supported boundary SSE stream")
	}
	if !strings.Contains(m.View(), "Loading conversation conv-selected") {
		t.Fatalf("selection must show replay loading state, got:\n%s", m.View())
	}
}

func TestIssue1246_StaleHistoryCannotPolluteNewSelection(t *testing.T) {
	m := initModel(t, 100, 30)
	m2, _ := m.Update(tui.SessionPickerSelectedMsg{SessionID: "conv-a"})
	m = m2.(tui.Model)
	m2, _ = m.Update(tui.SessionPickerSelectedMsg{SessionID: "conv-b"})
	m = m2.(tui.Model)

	m2, _ = m.Update(tui.ConversationHistoryMsg{ConversationID: "conv-a", Messages: []tui.ConversationMessage{{Role: "assistant", Content: "A_STALE_REPLY"}}})
	m = m2.(tui.Model)
	if strings.Contains(m.View(), "A_STALE_REPLY") {
		t.Fatal("late history from A must not render after selecting B")
	}

	// An older server reaches the existing GET fallback only after its selected
	// conversation boundary is declared unsupported.
	m2, _ = m.Update(tui.SSEConversationReplayBoundaryMsg{Conversation: true, ConversationID: "conv-b", Supported: false, StatusCode: 200})
	m = m2.(tui.Model)
	m2, _ = m.Update(tui.ConversationHistoryMsg{ConversationID: "conv-b", Messages: []tui.ConversationMessage{{Role: "assistant", Content: "B_CURRENT_REPLY"}}})
	m = m2.(tui.Model)
	if !strings.Contains(m.View(), "B_CURRENT_REPLY") {
		t.Fatal("current B history must render")
	}

	m2, _ = m.Update(tui.ConversationHistoryErrorMsg{ConversationID: "conv-a", Err: "late A error"})
	m = m2.(tui.Model)
	if strings.Contains(m.StatusMsg(), "late A error") {
		t.Fatal("late history error from A must not overwrite B status")
	}
}

// ─── BT-004: RunStartedMsg creates/updates session entry ─────────────────────

// TestSS004_RunStartedCreatesSessionEntry verifies that when RunStartedMsg is
// received, a session entry is created/updated in the store.
func TestSS004_RunStartedCreatesSessionEntry(t *testing.T) {
	m := initModel(t, 80, 24)

	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-xyz"})
	m = m2.(tui.Model)

	// The store should now have an entry with ID "run-xyz".
	entry, ok := m.SessionStore().Get("run-xyz")
	if !ok {
		t.Fatal("SessionStore should have an entry after RunStartedMsg")
	}
	if entry.ID != "run-xyz" {
		t.Errorf("entry ID: want %q, got %q", "run-xyz", entry.ID)
	}
}

// ─── Regression: /clear still works ──────────────────────────────────────────

// TestSS_Regression_ClearStillWorks verifies that /clear still clears
// the viewport without error after session wiring is added.
func TestSS_Regression_ClearStillWorks(t *testing.T) {
	m := initModel(t, 80, 24)

	// Type some text into the viewport first to confirm it's not empty.
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "r1"})
	m = m2.(tui.Model)
	m3, _ := m.Update(tui.RunCompletedMsg{RunID: "r1"})
	m = m3.(tui.Model)

	// /clear should not panic.
	m = sendSlashCommand(m, "/clear")

	// Overlay should not be left open by /clear.
	if m.ActiveOverlay() == "sessions" {
		t.Error("/clear left sessions overlay open")
	}
}

// ─── Regression: existing /sessions command registered ───────────────────────

// TestSS_Regression_SessionsCommandRegistered verifies the /sessions command is
// registered in the command registry so it appears in /help.
func TestSS_Regression_SessionsCommandRegistered(t *testing.T) {
	m := initModel(t, 80, 24)
	// If /sessions is not registered, sendSlashCommand would have no effect.
	// We verify the overlay opens — if the command isn't registered the overlay
	// won't open.
	m = sendSlashCommand(m, "/sessions")
	if m.ActiveOverlay() != "sessions" {
		t.Error("/sessions command must be registered; overlay did not open")
	}
}

// ─── Regression: /new command registered ─────────────────────────────────────

// TestSS_Regression_NewCommandRegistered verifies /new is a registered command.
func TestSS_Regression_NewCommandRegistered(t *testing.T) {
	m := initModel(t, 80, 24)

	// Simulate a run to set a conversationID.
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-for-new-test"})
	m = m2.(tui.Model)

	if m.ConversationID() == "" {
		t.Fatal("pre-condition: conversationID must be set after RunStartedMsg")
	}

	// /new should clear it.
	m = sendSlashCommand(m, "/new")

	if m.ConversationID() != "" {
		t.Errorf("/new must reset conversationID, got %q", m.ConversationID())
	}
}

// ─── Regression: SessionPickerSelectedMsg closes overlay ─────────────────────

// TestSS_Regression_SessionPickerEscapeClosesOverlay verifies that pressing
// Escape while the sessions overlay is open closes it.
func TestSS_Regression_SessionPickerEscapeClosesOverlay(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/sessions")

	if !m.OverlayActive() {
		t.Fatal("pre-condition: overlay must be open")
	}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = m2.(tui.Model)

	if m.OverlayActive() && m.ActiveOverlay() == "sessions" {
		t.Error("sessions overlay must close on Escape")
	}
}

// ─── BT-013: 'd' key in session picker deletes session from store ─────────────

// TestSS013_DeleteKeyRemovesSessionFromStore verifies that pressing 'd' while
// the session picker is open removes the currently selected session from the
// persistent store.
func TestSS013_DeleteKeyRemovesSessionFromStore(t *testing.T) {
	m := initModel(t, 80, 24)

	// Seed the store with known sessions.
	m.SessionStore().Add(tui.StoredSessionEntry{
		ID:        "del-session-1",
		StartedAt: time.Now(),
	})
	m.SessionStore().Add(tui.StoredSessionEntry{
		ID:        "del-session-2",
		StartedAt: time.Now().Add(-time.Minute),
	})

	// Record the store size before delete.
	sizeBefore := len(m.SessionStore().List())
	if sizeBefore < 2 {
		t.Fatal("pre-condition: store must have at least 2 entries")
	}

	// Refresh the picker via /sessions (this populates the picker from the store).
	m = sendSlashCommand(m, "/sessions")
	if !m.OverlayActive() {
		t.Fatal("pre-condition: sessions overlay must be open")
	}

	// Press 'd' — returns a cmd that produces SessionDeletedMsg.
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = m2.(tui.Model)

	// The cmd wraps the sessionpicker.SessionDeletedMsg → SessionDeletedMsg.
	// Run the cmd and feed the resulting message back into the model.
	if cmd != nil {
		msg := cmd()
		m3, _ := m.Update(msg)
		m = m3.(tui.Model)
	}

	// The store should now have one fewer entry.
	sizeAfter := len(m.SessionStore().List())
	if sizeAfter != sizeBefore-1 {
		t.Errorf("after 'd': store size want %d, got %d", sizeBefore-1, sizeAfter)
	}
}

// ─── BT-014: Session switch clears viewport and shows system message ──────────

// TestSS014_SessionSwitchClearsViewport verifies that selecting a session via
// SessionPickerSelectedMsg clears the viewport and appends a system message
// containing the session ID.
func TestSS014_SessionSwitchClearsViewport(t *testing.T) {
	m := initModel(t, 80, 24)
	// Add some content to the viewport first.
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "old-run"})
	m = m2.(tui.Model)

	// Switch to a different session.
	m3, _ := m.Update(tui.SessionPickerSelectedMsg{SessionID: "new-session-888"})
	m = m3.(tui.Model)

	// The view should contain the system message about the resumed session.
	view := m.View()
	if !strings.Contains(view, "new-session-888") {
		t.Errorf("viewport after session switch must mention session ID %q, got view:\n%s",
			"new-session-888", view)
	}
}

// ─── BT-015: CommandSubmittedMsg populates LastMsg on subsequent RunStartedMsg ─

// TestSS015_LastMsgPopulatedFromUserInput verifies that when the user submits a
// message and a RunStartedMsg is received, the session entry's LastMsg contains
// a truncated form of the user's input.
func TestSS015_LastMsgPopulatedFromUserInput(t *testing.T) {
	m := initModel(t, 80, 24)

	userInput := "What is the weather like today in San Francisco?"

	// Submit the message.
	m2, _ := m.Update(inputarea.CommandSubmittedMsg{Value: userInput})
	m = m2.(tui.Model)

	// Simulate the run starting.
	m3, _ := m.Update(tui.RunStartedMsg{RunID: "run-lastmsg"})
	m = m3.(tui.Model)

	entry, ok := m.SessionStore().Get("run-lastmsg")
	if !ok {
		t.Fatal("session entry must exist after RunStartedMsg")
	}
	if entry.LastMsg == "" {
		t.Error("LastMsg must be populated from user input, got empty string")
	}
	// The message should start with (at least part of) the user input.
	if !strings.HasPrefix(userInput, entry.LastMsg) && !strings.HasPrefix(entry.LastMsg, userInput[:10]) {
		t.Errorf("LastMsg %q does not match user input prefix of %q", entry.LastMsg, userInput)
	}
}

// ─── BT-016: Long user input is truncated in LastMsg ─────────────────────────

// TestSS016_LongLastMsgTruncated verifies that when the user submits a message
// longer than 60 characters, LastMsg is truncated to exactly 60 characters.
func TestSS016_LongLastMsgTruncated(t *testing.T) {
	m := initModel(t, 80, 24)

	// 70-rune input — must be truncated to 60.
	longInput := strings.Repeat("x", 70)

	m2, _ := m.Update(inputarea.CommandSubmittedMsg{Value: longInput})
	m = m2.(tui.Model)

	m3, _ := m.Update(tui.RunStartedMsg{RunID: "run-truncate"})
	m = m3.(tui.Model)

	entry, ok := m.SessionStore().Get("run-truncate")
	if !ok {
		t.Fatal("session entry must exist after RunStartedMsg")
	}
	if len([]rune(entry.LastMsg)) != 60 {
		t.Errorf("LastMsg rune length: want 60, got %d (value=%q)", len([]rune(entry.LastMsg)), entry.LastMsg)
	}
}

// ─── Regression: file permissions 0o600/0o700 on sessionstore ────────────────

// TestSS_Regression_SessionStoreFilePermissions verifies that sessions.json is
// written with 0o600 permissions (owner read/write only).
func TestSS_Regression_SessionStoreFilePermissions(t *testing.T) {
	dir := t.TempDir()
	store := tui.NewSessionStore(dir)
	store.Add(tui.StoredSessionEntry{ID: "perm-test", StartedAt: time.Now()})

	if err := store.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	info, err := tui.SessionStoreFileInfo(dir)
	if err != nil {
		t.Fatalf("stat sessions.json: %v", err)
	}
	mode := info.Mode().Perm()
	if mode != 0o600 {
		t.Errorf("sessions.json permissions: want 0o600, got %04o", mode)
	}
}
