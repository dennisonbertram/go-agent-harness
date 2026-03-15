package helpdialog_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/helpdialog"
)

// sampleCommands returns a fixed set of commands for testing.
func sampleCommands() []helpdialog.CommandEntry {
	return []helpdialog.CommandEntry{
		{Name: "clear", Description: "Clear conversation history"},
		{Name: "help", Description: "Show help dialog"},
		{Name: "context", Description: "Show context usage"},
		{Name: "stats", Description: "Show usage statistics"},
		{Name: "quit", Description: "Quit the TUI"},
	}
}

// sampleKeybindings returns a fixed set of key bindings for testing.
func sampleKeybindings() []helpdialog.KeyEntry {
	return []helpdialog.KeyEntry{
		{Keys: "enter", Description: "submit"},
		{Keys: "shift+enter", Description: "newline"},
		{Keys: "up / ctrl+p", Description: "scroll up"},
		{Keys: "down / ctrl+n", Description: "scroll down"},
		{Keys: "esc", Description: "interrupt"},
		{Keys: "?", Description: "help"},
		{Keys: "ctrl+c", Description: "quit"},
	}
}

// sampleAbout returns a fixed set of about lines for testing.
func sampleAbout() []string {
	return []string{
		"go-agent-harness v0.1.0",
		"Runtime: go1.25.0",
		"Model: gpt-4.1-mini",
	}
}

// TestTUI043_HelpdialogHasThreeTabs verifies all three Tab constants exist and are distinct.
func TestTUI043_HelpdialogHasThreeTabs(t *testing.T) {
	tabs := []helpdialog.Tab{
		helpdialog.TabCommands,
		helpdialog.TabKeybindings,
		helpdialog.TabAbout,
	}
	if len(tabs) != 3 {
		t.Fatalf("expected 3 tabs, got %d", len(tabs))
	}
	seen := make(map[helpdialog.Tab]bool)
	for _, tab := range tabs {
		if seen[tab] {
			t.Errorf("duplicate Tab value: %v", tab)
		}
		seen[tab] = true
	}
}

// TestTUI043_HelpdialogTabSwitching verifies NextTab and PrevTab cycle correctly.
func TestTUI043_HelpdialogTabSwitching(t *testing.T) {
	m := helpdialog.New(sampleCommands(), sampleKeybindings(), sampleAbout())

	// Should start on TabCommands (0)
	if m.ActiveTab() != helpdialog.TabCommands {
		t.Fatalf("expected TabCommands initial tab, got %v", m.ActiveTab())
	}

	// Forward cycle: 0 → 1 → 2 → 0
	m = m.NextTab()
	if m.ActiveTab() != helpdialog.TabKeybindings {
		t.Errorf("after 1 NextTab, expected TabKeybindings, got %v", m.ActiveTab())
	}
	m = m.NextTab()
	if m.ActiveTab() != helpdialog.TabAbout {
		t.Errorf("after 2 NextTab, expected TabAbout, got %v", m.ActiveTab())
	}
	m = m.NextTab()
	if m.ActiveTab() != helpdialog.TabCommands {
		t.Errorf("after 3 NextTab (wrap), expected TabCommands, got %v", m.ActiveTab())
	}

	// Backward cycle: 0 → 2 → 1 → 0
	m = m.PrevTab()
	if m.ActiveTab() != helpdialog.TabAbout {
		t.Errorf("after PrevTab from 0, expected TabAbout, got %v", m.ActiveTab())
	}
	m = m.PrevTab()
	if m.ActiveTab() != helpdialog.TabKeybindings {
		t.Errorf("after 2nd PrevTab, expected TabKeybindings, got %v", m.ActiveTab())
	}
	m = m.PrevTab()
	if m.ActiveTab() != helpdialog.TabCommands {
		t.Errorf("after 3rd PrevTab, expected TabCommands, got %v", m.ActiveTab())
	}
}

// TestTUI043_HelpdialogOpenClose verifies Open/Close/IsActive.
func TestTUI043_HelpdialogOpenClose(t *testing.T) {
	m := helpdialog.New(sampleCommands(), sampleKeybindings(), sampleAbout())

	if m.IsActive() {
		t.Error("new dialog should not be active")
	}

	m = m.Open()
	if !m.IsActive() {
		t.Error("after Open, dialog should be active")
	}

	m = m.Close()
	if m.IsActive() {
		t.Error("after Close, dialog should not be active")
	}
}

// TestTUI043_HelpdialogCommandsTabContent verifies command names appear in View.
func TestTUI043_HelpdialogCommandsTabContent(t *testing.T) {
	cmds := sampleCommands()
	m := helpdialog.New(cmds, sampleKeybindings(), sampleAbout())
	// Ensure we're on the commands tab
	for m.ActiveTab() != helpdialog.TabCommands {
		m = m.NextTab()
	}
	out := m.View(80, 24)
	for _, cmd := range cmds {
		if !strings.Contains(out, cmd.Name) {
			t.Errorf("command name %q not found in Commands tab view", cmd.Name)
		}
	}
}

// TestTUI043_HelpdialogKeybindingsTabContent verifies key descriptions appear in View.
func TestTUI043_HelpdialogKeybindingsTabContent(t *testing.T) {
	keys := sampleKeybindings()
	m := helpdialog.New(sampleCommands(), keys, sampleAbout())
	for m.ActiveTab() != helpdialog.TabKeybindings {
		m = m.NextTab()
	}
	out := m.View(80, 24)
	for _, k := range keys {
		if !strings.Contains(out, k.Description) {
			t.Errorf("key description %q not found in Keybindings tab view", k.Description)
		}
	}
}

// TestTUI043_HelpdialogAboutTabContent verifies about lines appear in View.
func TestTUI043_HelpdialogAboutTabContent(t *testing.T) {
	about := sampleAbout()
	m := helpdialog.New(sampleCommands(), sampleKeybindings(), about)
	for m.ActiveTab() != helpdialog.TabAbout {
		m = m.NextTab()
	}
	out := m.View(80, 24)
	for _, line := range about {
		if !strings.Contains(out, line) {
			t.Errorf("about line %q not found in About tab view", line)
		}
	}
}

// TestTUI043_HelpdialogUndefinedTabResets verifies that invalid/out-of-range Tab
// values don't cause panics and the view still renders.
func TestTUI043_HelpdialogUndefinedTabResets(t *testing.T) {
	m := helpdialog.New(sampleCommands(), sampleKeybindings(), sampleAbout())
	// Internal state is unexported, so we test via cycling that it never panics.
	// Cycle many times to ensure wrap-around is always safe.
	for i := 0; i < 100; i++ {
		m = m.NextTab()
		_ = m.View(80, 24)
	}
	for i := 0; i < 100; i++ {
		m = m.PrevTab()
		_ = m.View(80, 24)
	}
}

// TestTUI043_EmptyCommandsListSafe verifies no panic when commands is empty.
func TestTUI043_EmptyCommandsListSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic with empty commands: %v", r)
		}
	}()
	m := helpdialog.New(nil, nil, nil)
	_ = m.View(80, 24)
	m = m.NextTab()
	_ = m.View(80, 24)
	m = m.NextTab()
	_ = m.View(80, 24)
}

// TestTUI043_ConcurrentDialogs verifies that 10 goroutines can each hold their
// own Model and call methods without data races.
func TestTUI043_ConcurrentDialogs(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := helpdialog.New(sampleCommands(), sampleKeybindings(), sampleAbout())
			m = m.Open()
			m = m.NextTab()
			m = m.NextTab()
			m = m.PrevTab()
			m = m.ScrollDown(3)
			m = m.ScrollUp(1)
			_ = m.View(80, 24)
			m = m.Close()
			_ = m.IsActive()
		}()
	}
	wg.Wait()
}

// TestTUI043_BoundaryDimensions verifies View(0,0) and View(200,50) don't panic.
func TestTUI043_BoundaryDimensions(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on boundary dimensions: %v", r)
		}
	}()
	m := helpdialog.New(sampleCommands(), sampleKeybindings(), sampleAbout())
	_ = m.View(0, 0)
	_ = m.View(200, 50)
	m = m.NextTab()
	_ = m.View(0, 0)
	_ = m.View(200, 50)
	m = m.NextTab()
	_ = m.View(0, 0)
	_ = m.View(200, 50)
}

// snapshotTest renders View at given dimensions and compares/creates a snapshot file.
func snapshotTest(t *testing.T, width, height int, snapshotName string) {
	t.Helper()
	m := helpdialog.New(sampleCommands(), sampleKeybindings(), sampleAbout())
	out := m.View(width, height)

	snapshotDir := filepath.Join("testdata", "snapshots")
	snapshotPath := filepath.Join(snapshotDir, snapshotName)

	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		// Create snapshot on first run
		if err := os.MkdirAll(snapshotDir, 0755); err != nil {
			t.Fatalf("failed to create snapshot dir: %v", err)
		}
		if err := os.WriteFile(snapshotPath, []byte(out), 0644); err != nil {
			t.Fatalf("failed to write snapshot: %v", err)
		}
		t.Logf("created snapshot: %s", snapshotPath)
		return
	}

	existing, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("failed to read snapshot: %v", err)
	}
	if string(existing) != out {
		t.Errorf("snapshot mismatch for %s\ngot:\n%s\nwant:\n%s", snapshotName, out, string(existing))
	}
}

// TestTUI043_VisualSnapshot_80x24 captures the 80x24 snapshot.
func TestTUI043_VisualSnapshot_80x24(t *testing.T) {
	snapshotTest(t, 80, 24, "TUI-043-help-80x24.txt")
}

// TestTUI043_VisualSnapshot_120x40 captures the 120x40 snapshot.
func TestTUI043_VisualSnapshot_120x40(t *testing.T) {
	snapshotTest(t, 120, 40, "TUI-043-help-120x40.txt")
}

// TestTUI043_VisualSnapshot_200x50 captures the 200x50 snapshot.
func TestTUI043_VisualSnapshot_200x50(t *testing.T) {
	snapshotTest(t, 200, 50, "TUI-043-help-200x50.txt")
}
