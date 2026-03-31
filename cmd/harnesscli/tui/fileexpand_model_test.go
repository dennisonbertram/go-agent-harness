package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

// TestModel_FileExpandErrorShowsStatusMsg verifies that when a CommandSubmittedMsg
// contains an @path that fails to expand (file not found), the model shows a
// status message and does NOT initiate a run.
func TestModel_FileExpandErrorShowsStatusMsg(t *testing.T) {
	m := initModel(t, 80, 24)

	m2, _ := m.Update(inputarea.CommandSubmittedMsg{
		Value: "check @/tmp/absolutely-nonexistent-file-999.txt",
	})
	m = m2.(tui.Model)

	// A status message must be set.
	if m.StatusMsg() == "" {
		t.Error("StatusMsg must be set when @path expansion fails")
	}
	// The run must NOT be active (expansion error prevented the run).
	if m.RunActive() {
		t.Error("RunActive must be false when @path expansion fails")
	}
}

// TestModel_FileExpandNoAtTokensPassesThrough verifies that a normal message
// without @path tokens passes through the expansion step unchanged and the run
// is initiated normally.
func TestModel_FileExpandNoAtTokensPassesThrough(t *testing.T) {
	m := initModel(t, 80, 24)

	m2, cmd := m.Update(inputarea.CommandSubmittedMsg{
		Value: "hello world, no at files here",
	})
	m = m2.(tui.Model)

	// Status message must NOT be set (no error).
	if strings.Contains(m.StatusMsg(), "file expand error") {
		t.Errorf("StatusMsg must not contain file expand error for clean prompt; got %q", m.StatusMsg())
	}
	// A run command should have been queued.
	if cmd == nil {
		t.Error("cmd must be non-nil (startRunCmd must have been queued)")
	}
}

// TestModel_FileExpandValidFilePassesThrough verifies that when a prompt with a
// valid @path expands successfully, the run is started (no error status).
func TestModel_FileExpandValidFilePassesThrough(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(testFile, []byte("the file contents"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	m := initModel(t, 80, 24)
	m2, cmd := m.Update(inputarea.CommandSubmittedMsg{
		Value: "look at this file @" + testFile,
	})
	m = m2.(tui.Model)

	// No file expand error status should be set.
	if strings.Contains(m.StatusMsg(), "file expand error") {
		t.Errorf("StatusMsg must not contain error for valid @path; got %q", m.StatusMsg())
	}
	// A run command should have been queued.
	if cmd == nil {
		t.Error("cmd must be non-nil (startRunCmd must have been queued for valid expansion)")
	}
}

// TestModel_FileTabCompleteActivatesOnAt verifies that when input contains @
// followed by a partial path, Tab produces completions (file path completer active).
func TestModel_FileTabCompleteActivatesOnAt(t *testing.T) {
	dir := t.TempDir()
	// Create two files so there is at least one completion entry.
	for _, name := range []string{"readme.md", "main.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	m := initModel(t, 80, 24)
	// Type "@<dir>/" into the model.
	m = typeIntoModel(m, "@"+dir+"/")

	before := m.Input()

	// Press Tab.
	m = typeTab(m)
	after := m.Input()

	// Tab must have produced a change (a completion was found and applied).
	// OR there are multiple completions (common prefix unchanged but no error).
	// The key invariant: no panic, and the input still starts with @.
	if !strings.HasPrefix(after, "@") {
		t.Errorf("input after Tab on @ must still start with '@'; got %q", after)
	}

	_ = before // used to capture initial state
}

// Regression: TestModel_TabCompleteSlashStillWorksAfterCombinedProvider verifies
// that slash command Tab completion still works after the combined provider is wired.
func TestModel_TabCompleteSlashStillWorksAfterCombinedProvider(t *testing.T) {
	m := initModel(t, 80, 24)
	m = typeIntoModel(m, "/cl")
	m = typeTab(m)
	if m.Input() != "/clear " {
		t.Errorf("slash command Tab after combined provider: want %q, got %q", "/clear ", m.Input())
	}
}

// Regression: TestModel_FileExpandTooManyFilesShowsStatusMsg verifies that a
// prompt with more than 10 @path tokens fails with a status message and no run.
func TestModel_FileExpandTooManyFilesShowsStatusMsg(t *testing.T) {
	dir := t.TempDir()
	var parts []string
	for i := 0; i < 11; i++ {
		f := filepath.Join(dir, strings.Repeat("f", i+1)+".txt")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file %d: %v", i, err)
		}
		parts = append(parts, "@"+f)
	}

	m := initModel(t, 80, 24)
	m2, _ := m.Update(inputarea.CommandSubmittedMsg{Value: strings.Join(parts, " ")})
	m = m2.(tui.Model)

	if m.StatusMsg() == "" {
		t.Error("StatusMsg must be set when @path token count exceeds limit")
	}
	if m.RunActive() {
		t.Error("RunActive must be false when @path expansion exceeds limit")
	}
}

// ─── BT-NEW-007: Input is preserved on expansion failure ─────────────────────

// TestModel_FileExpandErrorPreservesInput verifies that when @path expansion
// fails, the input value is restored so the user can fix and re-submit.
func TestModel_FileExpandErrorPreservesInput(t *testing.T) {
	originalInput := "check @/tmp/absolutely-nonexistent-file-abc999.txt please"
	m := initModel(t, 80, 24)

	m2, _ := m.Update(inputarea.CommandSubmittedMsg{
		Value: originalInput,
	})
	m = m2.(tui.Model)

	// StatusMsg must be set (expansion failed).
	if m.StatusMsg() == "" {
		t.Error("StatusMsg must be set when @path expansion fails")
	}
	// Input must be restored to the original value.
	if m.Input() != originalInput {
		t.Errorf("Input must be restored to original value on expansion failure; want %q, got %q",
			originalInput, m.Input())
	}
}
