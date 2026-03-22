package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/profilepicker"
)

// TestProfilesCommand_Opens_Overlay verifies that /profiles sets overlayActive
// and activeOverlay == "profiles".
func TestProfilesCommand_Opens_Overlay(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/profiles")

	if !m.OverlayActive() {
		t.Fatal("overlayActive must be true after /profiles")
	}
	if m.ActiveOverlay() != "profiles" {
		t.Errorf("activeOverlay: want %q, got %q", "profiles", m.ActiveOverlay())
	}
}

// TestProfilesCommand_IsRegistered verifies that /profiles is a registered command.
func TestProfilesCommand_IsRegistered(t *testing.T) {
	reg := tui.NewCommandRegistry()
	_, ok := reg.Lookup("profiles")
	if !ok {
		t.Fatal("profiles command must be registered in NewCommandRegistry()")
	}
}

// TestProfilesLoaded_OpensPicker verifies that ProfilesLoadedMsg populates and
// opens the picker when there is no error.
func TestProfilesLoaded_OpensPicker(t *testing.T) {
	m := initModel(t, 80, 24)
	// Trigger the profiles overlay first.
	m = sendSlashCommand(m, "/profiles")

	entries := []tui.ProfileEntry{
		{Name: "test-profile", Description: "Test", Model: "gpt-4"},
	}
	m2, _ := m.Update(tui.ProfilesLoadedMsg{Entries: entries})
	m = m2.(tui.Model)

	if !m.OverlayActive() {
		t.Error("overlay should still be active after profiles loaded")
	}
	if m.ActiveOverlay() != "profiles" {
		t.Errorf("activeOverlay: want %q, got %q", "profiles", m.ActiveOverlay())
	}

	// View should render the picker with the profile name.
	view := m.View()
	if !strings.Contains(view, "test-profile") {
		t.Errorf("View() should show profile 'test-profile' after ProfilesLoadedMsg; got:\n%s", view)
	}
}

// TestProfilesLoaded_Error_ClosesOverlay verifies that a ProfilesLoadedMsg
// with an error closes the overlay and shows an error in the status.
func TestProfilesLoaded_Error_ClosesOverlay(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/profiles")
	if !m.OverlayActive() {
		t.Fatal("overlay should be active")
	}

	m2, _ := m.Update(tui.ProfilesLoadedMsg{Err: &profileLoadErr{"connection refused"}})
	m = m2.(tui.Model)

	if m.OverlayActive() {
		t.Error("overlay should be closed after ProfilesLoadedMsg with error")
	}
}

// profileLoadErr is a simple test error type.
type profileLoadErr struct{ msg string }

func (e *profileLoadErr) Error() string { return e.msg }

// TestProfileSelected_UpdatesModel verifies that ProfileSelectedMsg closes the
// overlay and stores the selected profile name.
func TestProfileSelected_UpdatesModel(t *testing.T) {
	m := initModel(t, 80, 24)
	// Open profiles overlay.
	m = sendSlashCommand(m, "/profiles")

	// Send a ProfilesLoadedMsg to populate the picker.
	entries := []tui.ProfileEntry{
		{Name: "chosen-profile", Description: "Desc", Model: "gpt-4"},
	}
	m2, _ := m.Update(tui.ProfilesLoadedMsg{Entries: entries})
	m = m2.(tui.Model)

	// Simulate selection.
	msg := profilepicker.ProfileSelectedMsg{
		Entry: profilepicker.ProfileEntry{Name: "chosen-profile"},
	}
	m3, _ := m.Update(msg)
	m = m3.(tui.Model)

	if m.OverlayActive() {
		t.Error("overlay should be closed after ProfileSelectedMsg")
	}
	if m.ActiveOverlay() != "" {
		t.Errorf("activeOverlay should be empty after selection; got %q", m.ActiveOverlay())
	}
	if m.SelectedProfile() != "chosen-profile" {
		t.Errorf("selectedProfile: want %q, got %q", "chosen-profile", m.SelectedProfile())
	}
}

// TestProfiles_EscapeClosesOverlay verifies that Escape while the profiles
// overlay is open closes it without selecting a profile.
func TestProfiles_EscapeClosesOverlay(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/profiles")
	if !m.OverlayActive() {
		t.Fatal("overlay must be active after /profiles")
	}

	// Send a profiles loaded message to have something in the picker.
	entries := []tui.ProfileEntry{
		{Name: "test", Description: "Test", Model: "gpt-4"},
	}
	m2, _ := m.Update(tui.ProfilesLoadedMsg{Entries: entries})
	m = m2.(tui.Model)

	// Press Escape.
	m, _ = sendEscape(m)

	if m.OverlayActive() {
		t.Error("overlay must be closed after Escape")
	}
	if m.SelectedProfile() != "" {
		t.Errorf("selectedProfile must remain empty after Escape; got %q", m.SelectedProfile())
	}
}

// TestProfilesOverlay_KeyboardNavigation verifies that Up/Down keys navigate
// the profile picker when it is open, and Enter selects the profile.
func TestProfilesOverlay_KeyboardNavigation(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/profiles")

	entries := []tui.ProfileEntry{
		{Name: "alpha", Description: "Alpha", Model: "gpt-4"},
		{Name: "beta", Description: "Beta", Model: "gpt-4"},
	}
	m2, _ := m.Update(tui.ProfilesLoadedMsg{Entries: entries})
	m = m2.(tui.Model)

	// Press Down — should move to beta.
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m3.(tui.Model)

	// Press Enter — picker emits ProfileSelectedMsg via returned cmd.
	m4, batchCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m4.(tui.Model)

	// The batch cmd contains the ProfileSelectedMsg — unwrap it by running all
	// inner cmds. tea.Batch returns a BatchMsg when called; we need to iterate.
	if batchCmd != nil {
		msg := batchCmd()
		// Handle BatchMsg from tea.Batch
		if batchMsgs, ok := msg.(tea.BatchMsg); ok {
			for _, innerCmd := range batchMsgs {
				if innerCmd != nil {
					innerMsg := innerCmd()
					if innerMsg != nil {
						mx, _ := m.Update(innerMsg)
						m = mx.(tui.Model)
					}
				}
			}
		} else if msg != nil {
			mx, _ := m.Update(msg)
			m = mx.(tui.Model)
		}
	}

	if m.SelectedProfile() != "beta" {
		t.Errorf("SelectedProfile: want %q, got %q", "beta", m.SelectedProfile())
	}
}

// TestSelectedProfile_PassedInRunRequest verifies that SelectedProfile is initially empty.
func TestSelectedProfile_InitiallyEmpty(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	if m.SelectedProfile() != "" {
		t.Errorf("SelectedProfile should be empty on new model; got %q", m.SelectedProfile())
	}
}
