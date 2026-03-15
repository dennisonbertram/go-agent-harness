package permissionspanel_test

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-agent-harness/cmd/harnesscli/tui/components/permissionspanel"
)

// writeSnapshot writes a visual snapshot to the package-local testdata/snapshots directory.
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

// ─── Constructor and initial state ────────────────────────────────────────────

func TestTUI052_New_InitialState(t *testing.T) {
	m := permissionspanel.New()
	assert.False(t, m.IsVisible(), "New model should not be visible")
	assert.Equal(t, 0, m.Selected, "Selected should start at 0")
	assert.Empty(t, m.Rules, "Rules should be empty")
}

// ─── Open / Close ─────────────────────────────────────────────────────────────

func TestTUI052_Open_SetsIsOpen(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true, Permanent: true},
	}
	m := permissionspanel.New().Open(rules)
	assert.True(t, m.IsVisible())
	assert.Equal(t, rules, m.Rules)
	assert.Equal(t, 0, m.Selected)
}

func TestTUI052_Open_EmptyRules(t *testing.T) {
	m := permissionspanel.New().Open(nil)
	assert.True(t, m.IsVisible())
	assert.Empty(t, m.Rules)
}

func TestTUI052_Close_ClearsIsOpen(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "read", Allowed: true, Permanent: false},
	}
	m := permissionspanel.New().Open(rules).Close()
	assert.False(t, m.IsVisible())
	// Rules still present after close (they are session state)
	assert.Equal(t, rules, m.Rules)
}

func TestTUI052_Open_ReturnsNewModel(t *testing.T) {
	original := permissionspanel.New()
	opened := original.Open([]permissionspanel.PermissionRule{{ToolName: "bash", Allowed: true}})
	// original must not be mutated
	assert.False(t, original.IsVisible())
	assert.True(t, opened.IsVisible())
}

// ─── SetRules ─────────────────────────────────────────────────────────────────

func TestTUI052_SetRules_UpdatesRules(t *testing.T) {
	m := permissionspanel.New()
	rules := []permissionspanel.PermissionRule{
		{ToolName: "write", Allowed: false, Permanent: true},
		{ToolName: "bash", Allowed: true, Permanent: false},
	}
	m2 := m.SetRules(rules)
	assert.Equal(t, rules, m2.Rules)
	// original not mutated
	assert.Empty(t, m.Rules)
}

func TestTUI052_SetRules_ClampsSelection(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
		{ToolName: "read", Allowed: true},
		{ToolName: "write", Allowed: true},
	}
	m := permissionspanel.New().Open(rules)
	m = m.SelectDown().SelectDown() // selected = 2
	assert.Equal(t, 2, m.Selected)

	// Shrink rules to 1 entry — selection must clamp
	m2 := m.SetRules([]permissionspanel.PermissionRule{{ToolName: "bash", Allowed: true}})
	assert.Equal(t, 0, m2.Selected)
}

// ─── Navigation: SelectUp / SelectDown ────────────────────────────────────────

func TestTUI052_SelectDown_MovesSelection(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
		{ToolName: "read", Allowed: false},
	}
	m := permissionspanel.New().Open(rules)
	assert.Equal(t, 0, m.Selected)
	m2 := m.SelectDown()
	assert.Equal(t, 1, m2.Selected)
}

func TestTUI052_SelectDown_WrapsAround(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
		{ToolName: "read", Allowed: false},
	}
	m := permissionspanel.New().Open(rules)
	m = m.SelectDown() // 0 -> 1
	m = m.SelectDown() // 1 -> 0 (wrap)
	assert.Equal(t, 0, m.Selected)
}

func TestTUI052_SelectUp_WrapsAround(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
		{ToolName: "read", Allowed: false},
		{ToolName: "write", Allowed: true},
	}
	m := permissionspanel.New().Open(rules)
	// At 0, SelectUp should wrap to last index (2)
	m2 := m.SelectUp()
	assert.Equal(t, 2, m2.Selected)
}

func TestTUI052_SelectUp_DecrementsSelection(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
		{ToolName: "read", Allowed: false},
	}
	m := permissionspanel.New().Open(rules).SelectDown() // at 1
	m2 := m.SelectUp()
	assert.Equal(t, 0, m2.Selected)
}

func TestTUI052_SelectDown_EmptyList_NoChange(t *testing.T) {
	m := permissionspanel.New().Open(nil)
	m2 := m.SelectDown()
	assert.Equal(t, 0, m2.Selected)
}

func TestTUI052_SelectUp_EmptyList_NoChange(t *testing.T) {
	m := permissionspanel.New().Open(nil)
	m2 := m.SelectUp()
	assert.Equal(t, 0, m2.Selected)
}

func TestTUI052_SelectDown_SingleEntry_StaysAtZero(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
	}
	m := permissionspanel.New().Open(rules)
	m2 := m.SelectDown()
	assert.Equal(t, 0, m2.Selected)
}

func TestTUI052_SelectUp_SingleEntry_StaysAtZero(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
	}
	m := permissionspanel.New().Open(rules)
	m2 := m.SelectUp()
	// single entry: wrap brings us back to 0
	assert.Equal(t, 0, m2.Selected)
}

// ─── ToggleSelected ───────────────────────────────────────────────────────────

func TestTUI052_ToggleSelected_FlipsAllowed(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
	}
	m := permissionspanel.New().Open(rules)
	m2 := m.ToggleSelected()
	require.Len(t, m2.Rules, 1)
	assert.False(t, m2.Rules[0].Allowed, "Allowed should be flipped to false")

	m3 := m2.ToggleSelected()
	assert.True(t, m3.Rules[0].Allowed, "Allowed should be flipped back to true")
}

func TestTUI052_ToggleSelected_EmptyList_NoChange(t *testing.T) {
	m := permissionspanel.New().Open(nil)
	m2 := m.ToggleSelected()
	assert.Empty(t, m2.Rules)
}

func TestTUI052_ToggleSelected_DoesNotMutateOriginal(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
	}
	m := permissionspanel.New().Open(rules)
	m.ToggleSelected() // discard result
	// original should be unchanged
	assert.True(t, m.Rules[0].Allowed)
}

func TestTUI052_ToggleSelected_SecondRule(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
		{ToolName: "read", Allowed: false},
	}
	m := permissionspanel.New().Open(rules).SelectDown() // select index 1
	m2 := m.ToggleSelected()
	require.Len(t, m2.Rules, 2)
	assert.True(t, m2.Rules[0].Allowed, "first rule unchanged")
	assert.True(t, m2.Rules[1].Allowed, "second rule flipped")
}

// ─── RemoveSelected ───────────────────────────────────────────────────────────

func TestTUI052_RemoveSelected_RemovesRule(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
		{ToolName: "read", Allowed: false},
		{ToolName: "write", Allowed: true},
	}
	m := permissionspanel.New().Open(rules).SelectDown() // select index 1 (read)
	m2 := m.RemoveSelected()
	require.Len(t, m2.Rules, 2)
	assert.Equal(t, "bash", m2.Rules[0].ToolName)
	assert.Equal(t, "write", m2.Rules[1].ToolName)
}

func TestTUI052_RemoveSelected_AdjustsSelectionWhenLastItem(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
		{ToolName: "read", Allowed: false},
	}
	m := permissionspanel.New().Open(rules).SelectDown() // select index 1
	m2 := m.RemoveSelected()
	require.Len(t, m2.Rules, 1)
	// Selection should move back to 0
	assert.Equal(t, 0, m2.Selected)
}

func TestTUI052_RemoveSelected_EmptyList_NoChange(t *testing.T) {
	m := permissionspanel.New().Open(nil)
	m2 := m.RemoveSelected()
	assert.Empty(t, m2.Rules)
	assert.Equal(t, 0, m2.Selected)
}

func TestTUI052_RemoveSelected_SingleEntry_LeavesEmpty(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
	}
	m := permissionspanel.New().Open(rules)
	m2 := m.RemoveSelected()
	assert.Empty(t, m2.Rules)
	assert.Equal(t, 0, m2.Selected)
}

func TestTUI052_RemoveSelected_DoesNotMutateOriginal(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
		{ToolName: "read", Allowed: false},
	}
	m := permissionspanel.New().Open(rules)
	m.RemoveSelected() // discard result
	assert.Len(t, m.Rules, 2, "original rules must not be mutated")
}

// ─── SelectedRule ─────────────────────────────────────────────────────────────

func TestTUI052_SelectedRule_ReturnsRule(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true, Permanent: true},
		{ToolName: "read", Allowed: false, Permanent: false},
	}
	m := permissionspanel.New().Open(rules).SelectDown()
	rule, ok := m.SelectedRule()
	require.True(t, ok)
	assert.Equal(t, "read", rule.ToolName)
	assert.False(t, rule.Allowed)
}

func TestTUI052_SelectedRule_EmptyList_ReturnsFalse(t *testing.T) {
	m := permissionspanel.New().Open(nil)
	_, ok := m.SelectedRule()
	assert.False(t, ok)
}

// ─── View rendering ───────────────────────────────────────────────────────────

func TestTUI052_View_ContainsTitle(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true, Permanent: true},
	}
	m := permissionspanel.New().Open(rules)
	m.Width = 80
	m.Height = 24
	v := m.View()
	assert.Contains(t, v, "Permissions")
}

func TestTUI052_View_ContainsToolNames(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true, Permanent: true},
		{ToolName: "read", Allowed: false, Permanent: false},
	}
	m := permissionspanel.New().Open(rules)
	m.Width = 80
	m.Height = 24
	v := m.View()
	assert.Contains(t, v, "bash")
	assert.Contains(t, v, "read")
}

func TestTUI052_View_ContainsPermanentOnceLabels(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true, Permanent: true},
		{ToolName: "read", Allowed: false, Permanent: false},
	}
	m := permissionspanel.New().Open(rules)
	m.Width = 80
	m.Height = 24
	v := m.View()
	assert.Contains(t, v, "permanent")
	assert.Contains(t, v, "once")
}

func TestTUI052_View_EmptyStateMessage(t *testing.T) {
	m := permissionspanel.New().Open(nil)
	m.Width = 80
	m.Height = 24
	v := m.View()
	assert.Contains(t, v, "No permission rules active")
}

func TestTUI052_View_FooterHint(t *testing.T) {
	m := permissionspanel.New().Open(nil)
	m.Width = 80
	m.Height = 24
	v := m.View()
	// Footer should contain navigation hints
	assert.Contains(t, v, "navigate")
	assert.Contains(t, v, "toggle")
	assert.Contains(t, v, "delete")
	assert.Contains(t, v, "esc")
}

func TestTUI052_View_SelectedRowHasCursorPrefix(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
		{ToolName: "read", Allowed: false},
	}
	m := permissionspanel.New().Open(rules)
	m.Width = 80
	m.Height = 24
	m = m.SelectDown() // select index 1

	v := m.View()
	lines := strings.Split(v, "\n")
	found := false
	for _, l := range lines {
		if strings.Contains(l, ">") && strings.Contains(l, "read") {
			found = true
			break
		}
	}
	assert.True(t, found, "selected row should have '>' prefix near tool name\ngot:\n%s", v)
}

func TestTUI052_View_NoPanic_SmallWidth(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true},
	}
	m := permissionspanel.New().Open(rules)
	m.Width = 10
	m.Height = 5
	assert.NotPanics(t, func() { _ = m.View() })
}

func TestTUI052_View_NoPanic_ZeroWidth(t *testing.T) {
	m := permissionspanel.New().Open(nil)
	m.Width = 0
	m.Height = 0
	assert.NotPanics(t, func() { _ = m.View() })
}

// ─── Concurrency ──────────────────────────────────────────────────────────────

func TestTUI052_Concurrent_NoRace(t *testing.T) {
	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)

	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true, Permanent: true},
		{ToolName: "read", Allowed: false, Permanent: false},
		{ToolName: "write", Allowed: true, Permanent: true},
	}

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			m := permissionspanel.New().Open(rules)
			m.Width = 80
			m.Height = 24
			m = m.SelectDown()
			m = m.ToggleSelected()
			m = m.SelectUp()
			m = m.RemoveSelected()
			_ = m.View()
			_, _ = m.SelectedRule()
		}(i)
	}
	wg.Wait()
}

// ─── Visual snapshots ─────────────────────────────────────────────────────────

func TestTUI052_VisualSnapshot_80x24(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true, Permanent: true},
		{ToolName: "read", Allowed: true, Permanent: false},
		{ToolName: "write", Allowed: false, Permanent: true},
		{ToolName: "grep", Allowed: true, Permanent: false},
	}
	m := permissionspanel.New().Open(rules)
	m.Width = 80
	m.Height = 24
	m = m.SelectDown() // highlight second row
	snapshot := m.View()
	writeSnapshot(t, "TUI-052-permissions-80x24.txt", snapshot)
}

func TestTUI052_VisualSnapshot_120x40(t *testing.T) {
	rules := []permissionspanel.PermissionRule{
		{ToolName: "bash", Allowed: true, Permanent: true},
		{ToolName: "read", Allowed: true, Permanent: false},
		{ToolName: "write", Allowed: false, Permanent: true},
		{ToolName: "grep", Allowed: true, Permanent: false},
		{ToolName: "git", Allowed: false, Permanent: false},
	}
	m := permissionspanel.New().Open(rules)
	m.Width = 120
	m.Height = 40
	snapshot := m.View()
	writeSnapshot(t, "TUI-052-permissions-120x40.txt", snapshot)
}

func TestTUI052_VisualSnapshot_EmptyState_80x24(t *testing.T) {
	m := permissionspanel.New().Open(nil)
	m.Width = 80
	m.Height = 24
	snapshot := m.View()
	writeSnapshot(t, "TUI-052-permissions-empty-80x24.txt", snapshot)
}
