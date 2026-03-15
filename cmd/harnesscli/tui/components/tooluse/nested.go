package tooluse

import (
	"strings"
)

// Node is a tree node representing one tool call.
type Node struct {
	// CallID is a unique identifier for this tool call.
	CallID string
	// ParentID is the CallID of the parent node. Empty string means root-level.
	ParentID string
	// Depth is the nesting level: 0 for root, 1 for first-level child, etc.
	// Set automatically by Flatten().
	Depth int
	// Collapsed holds the collapsed rendering of this tool call.
	Collapsed CollapsedView
	// Expanded holds the expanded rendering of this tool call.
	Expanded ExpandedView
	// Children are the direct child nodes of this node, in insertion order.
	Children []Node
}

// Tree holds a collection of root-level nodes with their children.
// Tree is an immutable value type — all mutations return a new Tree.
type Tree struct {
	roots []Node          // root-level nodes in insertion order
	byID  map[string]Node // flat index for fast lookup (includes all nodes)
}

// NewTree creates a new, empty Tree.
func NewTree() Tree {
	return Tree{
		byID: make(map[string]Node),
	}
}

// Add inserts a node into the tree. If the node has a ParentID that exists in
// the tree, it is attached as a child of that parent. If the ParentID is
// unknown or empty, the node is added at root level.
//
// If a node with the same CallID already exists, it is replaced (same position).
func (t Tree) Add(node Node) Tree {
	// Build a fresh tree by copying existing state.
	newTree := Tree{
		byID: make(map[string]Node, len(t.byID)+1),
	}
	// Copy existing byID (flat map).
	for k, v := range t.byID {
		newTree.byID[k] = v
	}

	// If this CallID already exists, remove the old one first so we can replace it.
	_, alreadyExists := newTree.byID[node.CallID]

	if alreadyExists {
		// Rebuild the whole tree without the duplicate, then add the new node.
		newTree = removeFromTree(t, node.CallID)
		// Recurse with the duplicate removed.
		return newTree.Add(node)
	}

	// Determine whether to attach to a parent or place at root.
	if node.ParentID != "" {
		if _, parentExists := newTree.byID[node.ParentID]; parentExists {
			// Rebuild the roots with the child attached to its parent.
			newTree.roots = attachChild(t.roots, node.ParentID, node)
			// Update the flat index.
			newTree.byID[node.CallID] = node
			// Also update the parent entry in byID to reflect the new child.
			updateParentInIndex(newTree.byID, node)
			return newTree
		}
		// Unknown parent — fall through to root-level placement.
	}

	// Add at root level.
	newTree.roots = make([]Node, len(t.roots), len(t.roots)+1)
	copy(newTree.roots, t.roots)
	newTree.roots = append(newTree.roots, node)
	newTree.byID[node.CallID] = node
	return newTree
}

// removeFromTree returns a new Tree with the node identified by callID removed
// (including from its parent's Children slice if applicable).
func removeFromTree(t Tree, callID string) Tree {
	newTree := Tree{
		byID: make(map[string]Node, len(t.byID)),
	}
	for k, v := range t.byID {
		if k != callID {
			newTree.byID[k] = v
		}
	}
	newTree.roots = removeNodeFromSlice(t.roots, callID)
	return newTree
}

// removeNodeFromSlice removes the node with callID from the slice, searching
// recursively through Children.
func removeNodeFromSlice(nodes []Node, callID string) []Node {
	result := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if n.CallID == callID {
			continue
		}
		n.Children = removeNodeFromSlice(n.Children, callID)
		result = append(result, n)
	}
	return result
}

// attachChild recursively searches for the node with parentID and appends
// child to its Children slice, returning a new copy of the nodes slice.
func attachChild(nodes []Node, parentID string, child Node) []Node {
	result := make([]Node, len(nodes))
	for i, n := range nodes {
		if n.CallID == parentID {
			// Attach child here.
			newChildren := make([]Node, len(n.Children), len(n.Children)+1)
			copy(newChildren, n.Children)
			newChildren = append(newChildren, child)
			n.Children = newChildren
			result[i] = n
		} else {
			// Recurse into children.
			n.Children = attachChild(n.Children, parentID, child)
			result[i] = n
		}
	}
	return result
}

// updateParentInIndex updates the flat byID index so that the parent node
// reflects having the given child added. This keeps byID consistent.
func updateParentInIndex(byID map[string]Node, child Node) {
	// Walk up the ancestor chain and update each ancestor.
	// For now we only need to update the direct parent since byID stores
	// a flat snapshot; the tree structure is authoritative in roots.
	// The byID map is used only for existence checks (Get), so we update the
	// parent entry to include the new child.
	parentID := child.ParentID
	for parentID != "" {
		parent, ok := byID[parentID]
		if !ok {
			break
		}
		// Add child to parent's Children in the flat index if not present.
		found := false
		for _, c := range parent.Children {
			if c.CallID == child.CallID {
				found = true
				break
			}
		}
		if !found {
			parent.Children = append(parent.Children, child)
			byID[parentID] = parent
		}
		// Move up to next ancestor.
		child = parent
		parentID = parent.ParentID
	}
}

// Get returns the node with the given callID, searching all nodes including
// children. Returns false if not found.
func (t Tree) Get(callID string) (Node, bool) {
	node, ok := t.byID[callID]
	return node, ok
}

// Roots returns the root-level nodes in insertion order.
func (t Tree) Roots() []Node {
	return t.roots
}

// Flatten returns all nodes in DFS order, with Depth fields correctly set.
func (t Tree) Flatten() []Node {
	result := make([]Node, 0, len(t.byID))
	for _, root := range t.roots {
		result = flattenNode(root, 0, result)
	}
	return result
}

// flattenNode recursively appends node and its descendants in DFS order.
func flattenNode(n Node, depth int, result []Node) []Node {
	n.Depth = depth
	result = append(result, n)
	for _, child := range n.Children {
		result = flattenNode(child, depth+1, result)
	}
	return result
}

// depth1Prefix is the indentation prefix for depth-1 children.
// Format: "  ⎿  " — 2 spaces + tree connector + 2 spaces.
const depth1Prefix = "  ⎿  "

// RenderTree renders all nodes with depth-based indentation.
//
// Root nodes (depth 0) are rendered with no indent.
// Each child depth level adds "  ⎿  " (or deeper indentation) as a prefix.
//
// Tree rendering format:
//
//	⏺ ParentTool(args)
//	  ⎿  ⏺ ChildTool(args)
//	  ⎿    ⏺ GrandchildTool(args)
//
// The expanded map controls whether each node uses its ExpandedView or
// CollapsedView. Nodes not present in expanded are rendered collapsed.
func RenderTree(roots []Node, expanded map[string]bool, width int) string {
	if len(roots) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, root := range roots {
		renderNode(&sb, root, 0, expanded, width)
	}
	return sb.String()
}

// renderNode renders a single node and its children recursively.
func renderNode(sb *strings.Builder, n Node, depth int, expanded map[string]bool, width int) {
	// Build the indent prefix based on depth.
	prefix := buildPrefix(depth)

	// Determine available width for the inner content.
	prefixRunes := countRunes(prefix)
	innerWidth := width - prefixRunes
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Set width on the view structs.
	c := n.Collapsed
	c.Width = innerWidth
	e := n.Expanded
	e.Width = innerWidth

	// Render the node using ToggleState.
	ts := ToggleState{}
	if expanded[n.CallID] {
		ts = ts.Toggle()
	}
	nodeView := ts.View(c, e)

	// Prepend prefix to each line of nodeView.
	if prefix != "" {
		nodeView = prefixLines(nodeView, prefix)
	}

	sb.WriteString(nodeView)

	// Render children.
	for _, child := range n.Children {
		renderNode(sb, child, depth+1, expanded, width)
	}
}

// buildPrefix returns the indentation prefix string for a given depth.
//
// Depth 0: "" (no prefix)
// Depth 1: "  ⎿  "
// Depth 2: "  ⎿    " (extra 2 spaces)
// Depth N: "  ⎿  " + "  "*(N-1)
func buildPrefix(depth int) string {
	if depth == 0 {
		return ""
	}
	// Base: "  ⎿  " + (depth-1)*"  " extra spaces
	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(treeSymbol)
	sb.WriteString("  ")
	for i := 1; i < depth; i++ {
		sb.WriteString("  ")
	}
	return sb.String()
}

// prefixLines prepends prefix to the first line of content only.
// Subsequent lines (for expanded views) are prefixed with equivalent
// blank padding to maintain alignment.
func prefixLines(content, prefix string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	// Calculate plain-text width of prefix for blank continuation lines.
	plainWidth := countRunes(prefix)
	blankPrefix := strings.Repeat(" ", plainWidth)

	var sb strings.Builder
	for i, line := range lines {
		if i == 0 {
			sb.WriteString(prefix)
			sb.WriteString(line)
		} else {
			// For the trailing newline, don't add a prefix to the empty segment.
			if i == len(lines)-1 && line == "" {
				// This is the trailing newline split artifact — just append newline.
				break
			}
			sb.WriteString("\n")
			sb.WriteString(blankPrefix)
			sb.WriteString(line)
		}
	}
	// Ensure we end with a newline if the original content did.
	if strings.HasSuffix(content, "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}

// countRunes returns the number of runes in s.
func countRunes(s string) int {
	count := 0
	for range s {
		count++
	}
	return count
}
