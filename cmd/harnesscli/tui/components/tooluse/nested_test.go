package tooluse

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// makeCollapsed returns a CollapsedView for testing.
func makeCollapsed(toolName, args string, width int) CollapsedView {
	return CollapsedView{
		ToolName: toolName,
		Args:     args,
		State:    StateCompleted,
		Width:    width,
	}
}

// makeExpanded returns an ExpandedView for testing.
func makeExpanded(toolName, args string, width int) ExpandedView {
	return ExpandedView{
		ToolName: toolName,
		Args:     args,
		State:    StateCompleted,
		Width:    width,
	}
}

// makeNode creates a Node with the given callID, parentID, toolName, and width.
func makeNode(callID, parentID, toolName string, width int) Node {
	return Node{
		CallID:    callID,
		ParentID:  parentID,
		Collapsed: makeCollapsed(toolName, "arg1", width),
		Expanded:  makeExpanded(toolName, "arg1", width),
	}
}

// TestTUI038_NestedToolCallsRenderHierarchy verifies that a parent appears
// before its child in the Flatten() output.
func TestTUI038_NestedToolCallsRenderHierarchy(t *testing.T) {
	tree := NewTree()
	tree = tree.Add(makeNode("parent-1", "", "ParentTool", 80))
	tree = tree.Add(makeNode("child-1", "parent-1", "ChildTool", 80))

	flat := tree.Flatten()
	if len(flat) != 2 {
		t.Fatalf("expected 2 nodes in Flatten(), got %d", len(flat))
	}
	if flat[0].CallID != "parent-1" {
		t.Errorf("expected parent first in Flatten(), got %q", flat[0].CallID)
	}
	if flat[1].CallID != "child-1" {
		t.Errorf("expected child second in Flatten(), got %q", flat[1].CallID)
	}
}

// TestTUI038_ChildStateUpdatesParent verifies that adding a child node to a
// parent causes the parent to have the child in its Children slice.
func TestTUI038_ChildStateUpdatesParent(t *testing.T) {
	tree := NewTree()
	tree = tree.Add(makeNode("parent-1", "", "ParentTool", 80))
	tree = tree.Add(makeNode("child-1", "parent-1", "ChildTool", 80))

	parent, ok := tree.Get("parent-1")
	if !ok {
		t.Fatal("parent-1 not found in tree")
	}
	if len(parent.Children) != 1 {
		t.Errorf("expected parent to have 1 child, got %d", len(parent.Children))
	}
	if parent.Children[0].CallID != "child-1" {
		t.Errorf("expected child CallID 'child-1', got %q", parent.Children[0].CallID)
	}
}

// TestTUI038_RenderTreeAddsIndentation verifies that child lines have extra
// indentation compared to root-level lines.
func TestTUI038_RenderTreeAddsIndentation(t *testing.T) {
	tree := NewTree()
	tree = tree.Add(makeNode("parent-1", "", "ParentTool", 80))
	tree = tree.Add(makeNode("child-1", "parent-1", "ChildTool", 80))

	expanded := map[string]bool{} // all collapsed
	output := RenderTree(tree.Roots(), expanded, 80)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	// We expect at least 2 lines: parent and child.
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines in RenderTree, got %d: %q", len(lines), output)
	}

	// Find parent line (should start with ⏺, no extra indent at root)
	parentLine := ""
	childLine := ""
	for _, l := range lines {
		stripped := stripANSI(l)
		if strings.Contains(stripped, "ParentTool") {
			parentLine = stripped
		}
		if strings.Contains(stripped, "ChildTool") {
			childLine = stripped
		}
	}

	if parentLine == "" {
		t.Fatal("ParentTool line not found in output")
	}
	if childLine == "" {
		t.Fatal("ChildTool line not found in output")
	}

	// The child line should have more leading whitespace/indent than the parent line.
	// Parent line starts with ⏺ (possibly with ANSI) — no leading spaces.
	// Child line starts with "  ⎿  ⏺" — has leading spaces.
	parentLeadSpaces := countLeadingSpaces(parentLine)
	childLeadSpaces := countLeadingSpaces(childLine)

	if childLeadSpaces <= parentLeadSpaces {
		t.Errorf("child line should have more indent than parent: parent=%d child=%d\nparent: %q\nchild: %q",
			parentLeadSpaces, childLeadSpaces, parentLine, childLine)
	}
}

// countLeadingSpaces counts leading space characters in a string.
func countLeadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r == ' ' {
			count++
		} else {
			break
		}
	}
	return count
}

// TestTUI038_OrphanFallsBackToRoot verifies that a node with an unknown
// ParentID is added at root level.
func TestTUI038_OrphanFallsBackToRoot(t *testing.T) {
	tree := NewTree()
	// No "missing-parent" in the tree
	tree = tree.Add(makeNode("orphan-1", "missing-parent", "OrphanTool", 80))

	roots := tree.Roots()
	if len(roots) != 1 {
		t.Fatalf("expected orphan to be at root level, got %d roots", len(roots))
	}
	if roots[0].CallID != "orphan-1" {
		t.Errorf("expected orphan-1 at root, got %q", roots[0].CallID)
	}
}

// TestTUI038_FlattenDFSOrder verifies DFS ordering: parent → child → sibling.
func TestTUI038_FlattenDFSOrder(t *testing.T) {
	tree := NewTree()
	tree = tree.Add(makeNode("root-1", "", "RootTool", 80))
	tree = tree.Add(makeNode("child-1", "root-1", "ChildTool1", 80))
	tree = tree.Add(makeNode("root-2", "", "RootTool2", 80))

	flat := tree.Flatten()
	if len(flat) != 3 {
		t.Fatalf("expected 3 nodes in Flatten(), got %d", len(flat))
	}

	// DFS order: root-1, child-1 (DFS visits child before moving to next root), root-2
	expected := []string{"root-1", "child-1", "root-2"}
	for i, id := range expected {
		if flat[i].CallID != id {
			t.Errorf("position %d: expected %q, got %q", i, id, flat[i].CallID)
		}
	}
}

// TestTUI038_DeepNesting verifies that 3+ levels of nesting render without panic.
func TestTUI038_DeepNesting(t *testing.T) {
	tree := NewTree()
	tree = tree.Add(makeNode("l0", "", "Level0", 80))
	tree = tree.Add(makeNode("l1", "l0", "Level1", 80))
	tree = tree.Add(makeNode("l2", "l1", "Level2", 80))
	tree = tree.Add(makeNode("l3", "l2", "Level3", 80))

	flat := tree.Flatten()
	if len(flat) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(flat))
	}

	// Check depths
	depths := []int{0, 1, 2, 3}
	for i, expected := range depths {
		if flat[i].Depth != expected {
			t.Errorf("node %d: expected depth %d, got %d", i, expected, flat[i].Depth)
		}
	}

	// Must not panic
	expanded := map[string]bool{}
	output := RenderTree(tree.Roots(), expanded, 80)
	if output == "" {
		t.Error("RenderTree returned empty string for deep nesting")
	}

	// All levels should appear
	for _, name := range []string{"Level0", "Level1", "Level2", "Level3"} {
		if !strings.Contains(stripANSI(output), name) {
			t.Errorf("%q not found in RenderTree output", name)
		}
	}
}

// TestTUI038_EmptyTree verifies that an empty tree returns no roots and
// RenderTree returns empty string.
func TestTUI038_EmptyTree(t *testing.T) {
	tree := NewTree()

	roots := tree.Roots()
	if len(roots) != 0 {
		t.Errorf("expected 0 roots for empty tree, got %d", len(roots))
	}

	output := RenderTree(roots, map[string]bool{}, 80)
	if output != "" {
		t.Errorf("expected empty string for empty tree, got %q", output)
	}
}

// TestTUI038_DuplicateCallID verifies that adding a second node with the same
// CallID replaces the first.
func TestTUI038_DuplicateCallID(t *testing.T) {
	tree := NewTree()
	tree = tree.Add(Node{
		CallID:    "dup-1",
		ParentID:  "",
		Collapsed: makeCollapsed("OriginalTool", "args", 80),
		Expanded:  makeExpanded("OriginalTool", "args", 80),
	})
	tree = tree.Add(Node{
		CallID:    "dup-1",
		ParentID:  "",
		Collapsed: makeCollapsed("ReplacedTool", "args", 80),
		Expanded:  makeExpanded("ReplacedTool", "args", 80),
	})

	roots := tree.Roots()
	if len(roots) != 1 {
		t.Fatalf("expected 1 root after duplicate add, got %d", len(roots))
	}
	if roots[0].Collapsed.ToolName != "ReplacedTool" {
		t.Errorf("expected 'ReplacedTool' after duplicate replace, got %q", roots[0].Collapsed.ToolName)
	}
}

// TestTUI038_ConcurrentTrees verifies that 10 goroutines each with their own
// Tree produce no data races when run with -race.
func TestTUI038_ConcurrentTrees(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tree := NewTree()
			tree = tree.Add(makeNode("parent", "", "ParentTool", 80))
			tree = tree.Add(makeNode("child", "parent", "ChildTool", 80))
			flat := tree.Flatten()
			if len(flat) != 2 {
				t.Errorf("goroutine %d: expected 2 nodes, got %d", id, len(flat))
			}
			output := RenderTree(tree.Roots(), map[string]bool{}, 80)
			if output == "" {
				t.Errorf("goroutine %d: RenderTree returned empty", id)
			}
		}(i)
	}
	wg.Wait()
}

// TestTUI038_BoundaryWidths verifies RenderTree at width=10, 80, 200
// does not panic and produces non-empty output for non-empty trees.
func TestTUI038_BoundaryWidths(t *testing.T) {
	widths := []int{10, 80, 200}
	for _, w := range widths {
		t.Run("width_"+itoa(w), func(t *testing.T) {
			tree := NewTree()
			tree = tree.Add(makeNode("parent-1", "", "ParentTool", w))
			tree = tree.Add(makeNode("child-1", "parent-1", "ChildTool", w))

			output := RenderTree(tree.Roots(), map[string]bool{}, w)
			if output == "" {
				t.Errorf("width=%d: RenderTree returned empty string", w)
			}
		})
	}
}

// itoa converts an int to string without importing strconv (avoids extra dep).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// TestTUI038_VisualSnapshot_80x24 renders a nested tree at 80 width and
// writes it to testdata/snapshots/TUI-038-nested-80x24.txt.
func TestTUI038_VisualSnapshot_80x24(t *testing.T) {
	output := buildNestedSnapshot(80)
	writeSnapshot(t, "testdata/snapshots/TUI-038-nested-80x24.txt", output)
}

// TestTUI038_VisualSnapshot_120x40 renders a nested tree at 120 width.
func TestTUI038_VisualSnapshot_120x40(t *testing.T) {
	output := buildNestedSnapshot(120)
	writeSnapshot(t, "testdata/snapshots/TUI-038-nested-120x40.txt", output)
}

// TestTUI038_VisualSnapshot_200x50 renders a nested tree at 200 width.
func TestTUI038_VisualSnapshot_200x50(t *testing.T) {
	output := buildNestedSnapshot(200)
	writeSnapshot(t, "testdata/snapshots/TUI-038-nested-200x50.txt", output)
}

// buildNestedSnapshot builds a representative nested tree at the given width
// and returns the RenderTree output.
func buildNestedSnapshot(width int) string {
	tree := NewTree()
	// Root level calls
	tree = tree.Add(Node{
		CallID:   "call-1",
		ParentID: "",
		Collapsed: CollapsedView{
			ToolName: "BashExec",
			Args:     "go test ./...",
			State:    StateCompleted,
			Width:    width,
		},
		Expanded: ExpandedView{
			ToolName: "BashExec",
			Args:     "go test ./...",
			State:    StateCompleted,
			Width:    width,
		},
	})
	// Child of call-1
	tree = tree.Add(Node{
		CallID:   "call-2",
		ParentID: "call-1",
		Collapsed: CollapsedView{
			ToolName: "ReadFile",
			Args:     "cmd/harnesscli/tui/theme.go",
			State:    StateCompleted,
			Width:    width,
		},
		Expanded: ExpandedView{
			ToolName: "ReadFile",
			Args:     "cmd/harnesscli/tui/theme.go",
			State:    StateCompleted,
			Width:    width,
		},
	})
	// Grandchild of call-2
	tree = tree.Add(Node{
		CallID:   "call-3",
		ParentID: "call-2",
		Collapsed: CollapsedView{
			ToolName: "GrepSearch",
			Args:     "lipgloss, tui/",
			State:    StateError,
			Width:    width,
		},
		Expanded: ExpandedView{
			ToolName: "GrepSearch",
			Args:     "lipgloss, tui/",
			State:    StateError,
			Width:    width,
		},
	})
	// Another root
	tree = tree.Add(Node{
		CallID:   "call-4",
		ParentID: "",
		Collapsed: CollapsedView{
			ToolName: "WriteFile",
			Args:     "nested.go, <content>",
			State:    StateRunning,
			Width:    width,
		},
		Expanded: ExpandedView{
			ToolName: "WriteFile",
			Args:     "nested.go, <content>",
			State:    StateRunning,
			Width:    width,
		},
	})

	expanded := map[string]bool{} // all collapsed for snapshot
	return RenderTree(tree.Roots(), expanded, width)
}

// writeSnapshot writes content to path, creating parent directories as needed.
func writeSnapshot(t *testing.T, path, content string) {
	t.Helper()
	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write snapshot %s: %v", path, err)
	}
	t.Logf("snapshot written to %s", path)
}
