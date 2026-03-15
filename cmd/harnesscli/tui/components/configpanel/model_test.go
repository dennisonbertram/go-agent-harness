package configpanel_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/configpanel"
)

// sampleEntries returns a fixed set of config entries for testing.
func sampleEntries() []configpanel.ConfigEntry {
	return []configpanel.ConfigEntry{
		{Key: "model", Value: "gpt-4.1-mini", Description: "LLM model name", ReadOnly: false},
		{Key: "max_steps", Value: "8", Description: "Maximum steps per run", ReadOnly: false},
		{Key: "workspace", Value: "/tmp", Description: "Working directory", ReadOnly: true},
		{Key: "server_url", Value: ":8080", Description: "Server address", ReadOnly: false},
		{Key: "timeout", Value: "30s", Description: "Request timeout", ReadOnly: false},
	}
}

// TestTUI051_ConfigPanelRendersSettingsRows verifies key/value rows are visible in View.
func TestTUI051_ConfigPanelRendersSettingsRows(t *testing.T) {
	m := configpanel.New(sampleEntries())
	m = m.Open()
	out := m.View(80, 24)
	for _, e := range sampleEntries() {
		if !strings.Contains(out, e.Key) {
			t.Errorf("key %q not found in config panel view", e.Key)
		}
		if !strings.Contains(out, e.Value) {
			t.Errorf("value %q not found in config panel view", e.Value)
		}
	}
}

// TestTUI051_ConfigPanelSearchFiltersRows verifies that a query hides unrelated rows.
func TestTUI051_ConfigPanelSearchFiltersRows(t *testing.T) {
	m := configpanel.New(sampleEntries())
	m = m.Open()
	m = m.SetQuery("model")
	out := m.View(80, 24)

	// "model" key should be visible
	if !strings.Contains(out, "model") {
		t.Error("expected 'model' row to be visible after SetQuery('model')")
	}

	// "workspace" should not be visible (unrelated to "model")
	if strings.Contains(out, "workspace") {
		t.Error("expected 'workspace' row to be hidden after SetQuery('model')")
	}
}

// TestTUI051_ConfigPanelEditFlow verifies StartEdit → EditInput → CommitEdit → Dirty=true.
func TestTUI051_ConfigPanelEditFlow(t *testing.T) {
	m := configpanel.New(sampleEntries())
	m = m.Open()
	// Select the "model" entry (index 0)
	if !m.IsEditing() {
		// verify not editing initially
	}

	m = m.StartEdit()
	if !m.IsEditing() {
		t.Fatal("expected IsEditing() == true after StartEdit()")
	}

	// Type new value
	m = m.EditInput('g')
	m = m.EditInput('p')
	m = m.EditInput('t')
	m = m.EditInput('-')
	m = m.EditInput('5')

	m = m.CommitEdit()
	if m.IsEditing() {
		t.Error("expected IsEditing() == false after CommitEdit()")
	}

	entry, ok := m.SelectedEntry()
	if !ok {
		t.Fatal("SelectedEntry() returned false after CommitEdit()")
	}
	if entry.Value != "gpt-5" {
		t.Errorf("expected entry.Value == 'gpt-5', got %q", entry.Value)
	}
	if !entry.Dirty {
		t.Error("expected entry.Dirty == true after CommitEdit()")
	}
}

// TestTUI051_ConfigPanelCancelEdit verifies CancelEdit restores original value.
func TestTUI051_ConfigPanelCancelEdit(t *testing.T) {
	m := configpanel.New(sampleEntries())
	m = m.Open()

	entry, _ := m.SelectedEntry()
	originalValue := entry.Value

	m = m.StartEdit()
	m = m.EditInput('X')
	m = m.EditInput('Y')
	m = m.EditInput('Z')
	m = m.CancelEdit()

	if m.IsEditing() {
		t.Error("expected IsEditing() == false after CancelEdit()")
	}

	entry, ok := m.SelectedEntry()
	if !ok {
		t.Fatal("SelectedEntry() returned false after CancelEdit()")
	}
	if entry.Value != originalValue {
		t.Errorf("CancelEdit should restore original value %q, got %q", originalValue, entry.Value)
	}
	if entry.Dirty {
		t.Error("CancelEdit should not set Dirty=true")
	}
}

// TestTUI051_ReadOnlyNotEditable verifies StartEdit on a ReadOnly entry is a no-op.
func TestTUI051_ReadOnlyNotEditable(t *testing.T) {
	m := configpanel.New(sampleEntries())
	m = m.Open()

	// Navigate to the "workspace" entry (index 2, ReadOnly=true)
	m = m.SelectDown()
	m = m.SelectDown()

	entry, ok := m.SelectedEntry()
	if !ok {
		t.Fatal("SelectedEntry() returned false")
	}
	if entry.Key != "workspace" {
		t.Fatalf("expected 'workspace' at index 2, got %q", entry.Key)
	}
	if !entry.ReadOnly {
		t.Fatal("expected entry to be ReadOnly")
	}

	m = m.StartEdit()
	if m.IsEditing() {
		t.Error("StartEdit on ReadOnly entry should be a no-op; IsEditing() should remain false")
	}
}

// TestTUI051_ConfigPanelOpenClose verifies Open/Close/IsActive states.
func TestTUI051_ConfigPanelOpenClose(t *testing.T) {
	m := configpanel.New(sampleEntries())

	if m.IsActive() {
		t.Error("new panel should not be active")
	}

	m = m.Open()
	if !m.IsActive() {
		t.Error("after Open(), IsActive() should be true")
	}

	m = m.Close()
	if m.IsActive() {
		t.Error("after Close(), IsActive() should be false")
	}
}

// TestTUI051_SelectionWraps verifies SelectUp at top wraps to bottom.
func TestTUI051_SelectionWraps(t *testing.T) {
	m := configpanel.New(sampleEntries())
	m = m.Open()

	// SelectUp at index 0 should wrap to the last entry.
	m = m.SelectUp()
	entry, ok := m.SelectedEntry()
	if !ok {
		t.Fatal("SelectedEntry() returned false after wrap")
	}

	entries := sampleEntries()
	lastKey := entries[len(entries)-1].Key
	if entry.Key != lastKey {
		t.Errorf("SelectUp at top should wrap to bottom (%q), got %q", lastKey, entry.Key)
	}

	// SelectDown at bottom should wrap to first entry.
	m = m.SelectDown()
	entry, ok = m.SelectedEntry()
	if !ok {
		t.Fatal("SelectedEntry() returned false after wrap down")
	}
	if entry.Key != entries[0].Key {
		t.Errorf("SelectDown at bottom should wrap to top (%q), got %q", entries[0].Key, entry.Key)
	}
}

// TestTUI051_ConcurrentPanels verifies 10 goroutines each with their own Model have no data races.
func TestTUI051_ConcurrentPanels(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := configpanel.New(sampleEntries())
			m = m.Open()
			m = m.SetQuery("mod")
			m = m.SelectDown()
			m = m.StartEdit()
			m = m.EditInput('x')
			m = m.EditInput('y')
			m = m.EditBackspace()
			m = m.CommitEdit()
			_ = m.View(80, 24)
			m = m.Close()
			_ = m.IsActive()
		}()
	}
	wg.Wait()
}

// TestTUI051_BoundaryDimensions verifies View(0,0) and View(200,50) don't panic.
func TestTUI051_BoundaryDimensions(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on boundary dimensions: %v", r)
		}
	}()
	m := configpanel.New(sampleEntries())
	m = m.Open()
	_ = m.View(0, 0)
	_ = m.View(200, 50)
	_ = m.View(1, 1)
	_ = m.View(40, 10)
}

// TestTUI051_EmptyEntriesListSafe verifies empty entries list doesn't panic.
func TestTUI051_EmptyEntriesListSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic with empty entries: %v", r)
		}
	}()
	m := configpanel.New(nil)
	m = m.Open()
	_ = m.View(80, 24)
	m = m.SelectUp()
	m = m.SelectDown()
	m = m.StartEdit()
	_, _ = m.SelectedEntry()
}

// snapshotTest renders View at given dimensions and compares/creates a snapshot file.
func snapshotTest(t *testing.T, width, height int, snapshotName string) {
	t.Helper()
	m := configpanel.New(sampleEntries())
	m = m.Open()
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

// TestTUI051_VisualSnapshot_80x24 captures the 80x24 snapshot.
func TestTUI051_VisualSnapshot_80x24(t *testing.T) {
	snapshotTest(t, 80, 24, "TUI-051-config-80x24.txt")
}

// TestTUI051_VisualSnapshot_120x40 captures the 120x40 snapshot.
func TestTUI051_VisualSnapshot_120x40(t *testing.T) {
	snapshotTest(t, 120, 40, "TUI-051-config-120x40.txt")
}

// TestTUI051_VisualSnapshot_200x50 captures the 200x50 snapshot.
func TestTUI051_VisualSnapshot_200x50(t *testing.T) {
	snapshotTest(t, 200, 50, "TUI-051-config-200x50.txt")
}
