package permissionprompt_test

import (
	"os"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"go-agent-harness/cmd/harnesscli/tui/components/permissionprompt"
)

// writeSnapshot writes a visual snapshot to the package-local testdata/snapshots
// directory (adjacent to the permissionprompt package).
func writeSnapshot(t *testing.T, name, content string) {
	t.Helper()
	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating snapshot dir: %v", err)
	}
	path := dir + "/" + name
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing snapshot %s: %v", path, err)
	}
	t.Logf("snapshot written to %s", path)
}

// ─── TDD Tests ────────────────────────────────────────────────────────────────

// TestTUI033_FileEditPromptShowsYesNoScope verifies View() contains all three
// option labels.
func TestTUI033_FileEditPromptShowsYesNoScope(t *testing.T) {
	m := permissionprompt.New("WriteFile", "path/to/file.go", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
		permissionprompt.OptionAllowAll,
	})
	v := m.View(80)

	for _, want := range []string{"Yes", "No", "Allow all"} {
		if !strings.Contains(v, want) {
			t.Errorf("View() missing %q\ngot:\n%s", want, v)
		}
	}
}

// TestTUI033_PermissionPromptConsumesKeyInput verifies that Down arrow moves the
// cursor and Enter resolves the prompt.
func TestTUI033_PermissionPromptConsumesKeyInput(t *testing.T) {
	m := permissionprompt.New("ReadFile", "main.go", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
		permissionprompt.OptionAllowAll,
	})

	// Press Down once — cursor should move from 0 to 1.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	v := m2.View(80)
	// After one Down the second option (No) should have the cursor marker.
	lines := strings.Split(v, "\n")
	found := false
	for _, l := range lines {
		if strings.Contains(l, ">") && strings.Contains(l, "No") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("After Down, expected '> No' in view:\n%s", v)
	}

	// Press Enter — should resolve.
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m3.IsResolved() {
		t.Error("After Enter, IsResolved() should be true")
	}
	if m3.Result().Option != permissionprompt.OptionNo {
		t.Errorf("Expected OptionNo, got %v", m3.Result().Option)
	}
}

// TestTUI033_EscapeResolvesNo verifies that Escape resolves with OptionNo.
func TestTUI033_EscapeResolvesNo(t *testing.T) {
	m := permissionprompt.New("BashExec", "ls -la", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
	})

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m2.IsResolved() {
		t.Error("After Escape, IsResolved() should be true")
	}
	if m2.Result().Option != permissionprompt.OptionNo {
		t.Errorf("Expected OptionNo after Escape, got %v", m2.Result().Option)
	}
}

// TestTUI033_TabEntersAmendMode verifies Tab activates amend mode.
func TestTUI033_TabEntersAmendMode(t *testing.T) {
	m := permissionprompt.New("ReadFile", "old/path.go", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
	})

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m2.IsAmending() {
		t.Error("After Tab, IsAmending() should be true")
	}
}

// TestTUI033_AmendModeEditsResource verifies that typing in amend mode updates
// the amended string, and Enter in amend mode confirms the amendment.
func TestTUI033_AmendModeEditsResource(t *testing.T) {
	m := permissionprompt.New("ReadFile", "old/path.go", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
	})

	// Enter amend mode.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Type "new/path.go".
	for _, ch := range "new/path.go" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	v := m.View(80)
	if !strings.Contains(v, "new/path.go") {
		t.Errorf("View() should show amended resource 'new/path.go':\n%s", v)
	}

	// Confirm amendment with Enter.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.IsAmending() {
		t.Error("After Enter in amend mode, IsAmending() should be false")
	}
	if m.AmendedResource() != "new/path.go" {
		t.Errorf("AmendedResource() = %q, want %q", m.AmendedResource(), "new/path.go")
	}
}

// TestTUI033_UnknownToolFallsBackToAsk verifies that a generic (unknown) tool
// name still produces a readable prompt with all options.
func TestTUI033_UnknownToolFallsBackToAsk(t *testing.T) {
	m := permissionprompt.New("SomeUnknownTool", "some-resource", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
		permissionprompt.OptionAllowAll,
	})
	v := m.View(80)

	if !strings.Contains(v, "SomeUnknownTool") {
		t.Errorf("View() should contain tool name 'SomeUnknownTool':\n%s", v)
	}
	for _, want := range []string{"Yes", "No", "Allow all"} {
		if !strings.Contains(v, want) {
			t.Errorf("View() missing %q:\n%s", want, v)
		}
	}
}

// TestTUI033_ConcurrentPrompts creates 10 independent Models and updates them
// concurrently to verify no data races.
func TestTUI033_ConcurrentPrompts(t *testing.T) {
	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			m := permissionprompt.New("Tool", "resource", []permissionprompt.PromptOption{
				permissionprompt.OptionYes,
				permissionprompt.OptionNo,
				permissionprompt.OptionAllowAll,
			})
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			_ = m.View(80)
			_ = m.IsResolved()
			_ = m.Result()
		}(i)
	}
	wg.Wait()
}

// TestTUI033_PromptTimeoutMissing verifies that a prompt with an empty Options
// slice renders a fallback message without panicking.
func TestTUI033_PromptTimeoutMissing(t *testing.T) {
	m := permissionprompt.New("ReadFile", "file.go", nil)
	v := m.View(80)
	// Should not panic and should produce something non-empty.
	if v == "" {
		t.Error("View() with no options should still render something")
	}
}

// TestTUI033_IsResolvedAfterEnter verifies IsResolved() becomes true after
// pressing Enter.
func TestTUI033_IsResolvedAfterEnter(t *testing.T) {
	m := permissionprompt.New("WriteFile", "output.txt", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
	})

	if m.IsResolved() {
		t.Error("IsResolved() should be false before any input")
	}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m2.IsResolved() {
		t.Error("IsResolved() should be true after Enter")
	}
}

// TestTUI033_IsActiveBeforeResolve verifies IsActive() is true until the prompt
// is resolved.
func TestTUI033_IsActiveBeforeResolve(t *testing.T) {
	m := permissionprompt.New("ReadFile", "config.yaml", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
	})

	if !m.IsActive() {
		t.Error("IsActive() should be true on a new prompt")
	}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.IsActive() {
		t.Error("IsActive() should be false after resolution")
	}
}

// TestTUI033_BoundaryWidths verifies View() does not panic at extreme widths.
func TestTUI033_BoundaryWidths(t *testing.T) {
	m := permissionprompt.New("ReadFile", "file.go", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
		permissionprompt.OptionAllowAll,
	})

	for _, w := range []int{10, 80, 200} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("View(%d) panicked: %v", w, r)
				}
			}()
			_ = m.View(w)
		}()
	}
}

// TestTUI033_VisualSnapshot_80x24 writes a visual snapshot for an 80-column
// terminal.
func TestTUI033_VisualSnapshot_80x24(t *testing.T) {
	m := permissionprompt.New("WriteFile", "cmd/harnesscli/tui/model.go", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
		permissionprompt.OptionAllowAll,
	})
	snapshot := m.View(80)
	writeSnapshot(t, "TUI-033-permission-80x24.txt", snapshot)
}

// TestTUI033_VisualSnapshot_120x40 writes a visual snapshot for a 120-column
// terminal.
func TestTUI033_VisualSnapshot_120x40(t *testing.T) {
	m := permissionprompt.New("WriteFile", "cmd/harnesscli/tui/model.go", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
		permissionprompt.OptionAllowAll,
	})
	snapshot := m.View(120)
	writeSnapshot(t, "TUI-033-permission-120x40.txt", snapshot)
}

// TestTUI033_VisualSnapshot_200x50 writes a visual snapshot for a 200-column
// terminal.
func TestTUI033_VisualSnapshot_200x50(t *testing.T) {
	m := permissionprompt.New("WriteFile", "cmd/harnesscli/tui/model.go", []permissionprompt.PromptOption{
		permissionprompt.OptionYes,
		permissionprompt.OptionNo,
		permissionprompt.OptionAllowAll,
	})
	snapshot := m.View(200)
	writeSnapshot(t, "TUI-033-permission-200x50.txt", snapshot)
}
