package causalgraph

import (
	"testing"
)

func TestBlameChain_SingleNode(t *testing.T) {
	t.Parallel()
	g := CausalGraph{
		Nodes: []Node{
			{ID: "turn-1", Type: NodeTypeLLMTurn, Step: 1},
		},
	}
	chain := BlameChain(g, "turn-1")
	if len(chain) != 1 {
		t.Fatalf("expected 1 node in blame chain, got %d", len(chain))
	}
	if chain[0].ID != "turn-1" {
		t.Errorf("chain[0].ID = %q, want %q", chain[0].ID, "turn-1")
	}
	if chain[0].Type != string(NodeTypeLLMTurn) {
		t.Errorf("chain[0].Type = %q, want %q", chain[0].Type, NodeTypeLLMTurn)
	}
	if chain[0].Step != 1 {
		t.Errorf("chain[0].Step = %d, want 1", chain[0].Step)
	}
	if chain[0].Cause != "" {
		t.Errorf("chain[0].Cause = %q, want empty (root)", chain[0].Cause)
	}
}

func TestBlameChain_LinearChain(t *testing.T) {
	t.Parallel()
	// a -> b -> c (context edges)
	g := CausalGraph{
		Nodes: []Node{
			{ID: "a", Type: NodeTypeLLMTurn, Step: 1},
			{ID: "b", Type: NodeTypeToolCall, Step: 1, ToolName: "bash"},
			{ID: "c", Type: NodeTypeLLMTurn, Step: 2},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeTypeContext},
			{From: "b", To: "c", Type: EdgeTypeContext},
		},
	}
	chain := BlameChain(g, "c")
	// Reverse BFS from c: c -> b -> a (ordered target first, then causes)
	if len(chain) != 3 {
		t.Fatalf("expected 3 nodes in blame chain, got %d: %+v", len(chain), chain)
	}
	// First element is the target
	if chain[0].ID != "c" {
		t.Errorf("chain[0].ID = %q, want %q", chain[0].ID, "c")
	}
	// The chain should trace back: c's cause is b, b's cause is a
	if chain[1].ID != "b" {
		t.Errorf("chain[1].ID = %q, want %q", chain[1].ID, "b")
	}
	if chain[1].Cause != "c" {
		t.Errorf("chain[1].Cause = %q, want %q (caused by traversal from c)", chain[1].Cause, "c")
	}
	if chain[2].ID != "a" {
		t.Errorf("chain[2].ID = %q, want %q", chain[2].ID, "a")
	}
	if chain[2].Cause != "b" {
		t.Errorf("chain[2].Cause = %q, want %q", chain[2].Cause, "b")
	}
}

func TestBlameChain_Branching(t *testing.T) {
	t.Parallel()
	// Both a and b feed into c
	//   a --\
	//        --> c
	//   b --/
	g := CausalGraph{
		Nodes: []Node{
			{ID: "a", Type: NodeTypeLLMTurn, Step: 1},
			{ID: "b", Type: NodeTypeToolCall, Step: 1, ToolName: "read"},
			{ID: "c", Type: NodeTypeLLMTurn, Step: 2},
		},
		Edges: []Edge{
			{From: "a", To: "c", Type: EdgeTypeContext},
			{From: "b", To: "c", Type: EdgeTypeDataFlow, MatchedToken: "foobar"},
		},
	}
	chain := BlameChain(g, "c")
	// Should find c, then both a and b (order among siblings is BFS-stable)
	if len(chain) != 3 {
		t.Fatalf("expected 3 nodes in blame chain, got %d: %+v", len(chain), chain)
	}
	if chain[0].ID != "c" {
		t.Errorf("chain[0].ID = %q, want %q", chain[0].ID, "c")
	}
	// Both a and b should appear and have cause "c"
	ids := make(map[string]bool)
	for _, bn := range chain[1:] {
		ids[bn.ID] = true
		if bn.Cause != "c" {
			t.Errorf("node %q has cause %q, want %q", bn.ID, bn.Cause, "c")
		}
	}
	if !ids["a"] || !ids["b"] {
		t.Errorf("expected both a and b in chain, got %v", ids)
	}
}

func TestBlameChain_Disconnected(t *testing.T) {
	t.Parallel()
	// d is not connected to any edges
	g := CausalGraph{
		Nodes: []Node{
			{ID: "a", Type: NodeTypeLLMTurn, Step: 1},
			{ID: "b", Type: NodeTypeToolCall, Step: 1, ToolName: "bash"},
			{ID: "d", Type: NodeTypeLLMTurn, Step: 3},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeTypeContext},
		},
	}
	chain := BlameChain(g, "d")
	// d has no predecessors, so the chain is just [d]
	if len(chain) != 1 {
		t.Fatalf("expected 1 node for disconnected target, got %d: %+v", len(chain), chain)
	}
	if chain[0].ID != "d" {
		t.Errorf("chain[0].ID = %q, want %q", chain[0].ID, "d")
	}
}

func TestBlameChain_CycleGuard(t *testing.T) {
	t.Parallel()
	// Artificial cycle: a -> b -> a (should not loop forever)
	g := CausalGraph{
		Nodes: []Node{
			{ID: "a", Type: NodeTypeLLMTurn, Step: 1},
			{ID: "b", Type: NodeTypeToolCall, Step: 1, ToolName: "bash"},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeTypeContext},
			{From: "b", To: "a", Type: EdgeTypeContext},
		},
	}
	chain := BlameChain(g, "b")
	// Should visit b and a, but not loop. Visited set prevents revisit.
	if len(chain) != 2 {
		t.Fatalf("expected 2 nodes (cycle broken), got %d: %+v", len(chain), chain)
	}
	ids := make(map[string]bool)
	for _, bn := range chain {
		ids[bn.ID] = true
	}
	if !ids["a"] || !ids["b"] {
		t.Errorf("expected both a and b, got %v", ids)
	}
}

func TestBlameChain_NonExistentTarget(t *testing.T) {
	t.Parallel()
	g := CausalGraph{
		Nodes: []Node{
			{ID: "a", Type: NodeTypeLLMTurn, Step: 1},
		},
	}
	chain := BlameChain(g, "nonexistent")
	if len(chain) != 0 {
		t.Errorf("expected empty chain for nonexistent target, got %d: %+v", len(chain), chain)
	}
}

func TestBlameChain_DeepChain(t *testing.T) {
	t.Parallel()
	// Linear chain of 5 nodes: a -> b -> c -> d -> e
	g := CausalGraph{
		Nodes: []Node{
			{ID: "a", Type: NodeTypeLLMTurn, Step: 1},
			{ID: "b", Type: NodeTypeToolCall, Step: 1, ToolName: "bash"},
			{ID: "c", Type: NodeTypeLLMTurn, Step: 2},
			{ID: "d", Type: NodeTypeToolCall, Step: 2, ToolName: "read"},
			{ID: "e", Type: NodeTypeLLMTurn, Step: 3},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeTypeContext},
			{From: "b", To: "c", Type: EdgeTypeDataFlow},
			{From: "c", To: "d", Type: EdgeTypeContext},
			{From: "d", To: "e", Type: EdgeTypeDataFlow},
		},
	}
	chain := BlameChain(g, "e")
	if len(chain) != 5 {
		t.Fatalf("expected 5 nodes, got %d: %+v", len(chain), chain)
	}
	// BFS order from e: e, d, c, b, a
	expectedOrder := []string{"e", "d", "c", "b", "a"}
	for i, want := range expectedOrder {
		if chain[i].ID != want {
			t.Errorf("chain[%d].ID = %q, want %q", i, chain[i].ID, want)
		}
	}
}

func TestBlameChain_PreservesNodeMetadata(t *testing.T) {
	t.Parallel()
	g := CausalGraph{
		Nodes: []Node{
			{ID: "call-1", Type: NodeTypeToolCall, Step: 3, ToolName: "write"},
		},
	}
	chain := BlameChain(g, "call-1")
	if len(chain) != 1 {
		t.Fatalf("expected 1 node, got %d", len(chain))
	}
	bn := chain[0]
	if bn.ID != "call-1" {
		t.Errorf("ID = %q, want %q", bn.ID, "call-1")
	}
	if bn.Type != string(NodeTypeToolCall) {
		t.Errorf("Type = %q, want %q", bn.Type, NodeTypeToolCall)
	}
	if bn.Step != 3 {
		t.Errorf("Step = %d, want 3", bn.Step)
	}
}

func TestExportDAG_Empty(t *testing.T) {
	t.Parallel()
	g := CausalGraph{}
	dag := ExportDAG(g)
	if len(dag) != 0 {
		t.Errorf("expected empty DAG, got %v", dag)
	}
}

func TestExportDAG_Basic(t *testing.T) {
	t.Parallel()
	g := CausalGraph{
		Nodes: []Node{
			{ID: "a", Type: NodeTypeLLMTurn, Step: 1},
			{ID: "b", Type: NodeTypeToolCall, Step: 1, ToolName: "bash"},
			{ID: "c", Type: NodeTypeLLMTurn, Step: 2},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeTypeContext},
			{From: "b", To: "c", Type: EdgeTypeDataFlow},
		},
	}
	dag := ExportDAG(g)
	if len(dag) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(dag))
	}
	if len(dag["a"]) != 1 || dag["a"][0] != "b" {
		t.Errorf("dag[a] = %v, want [b]", dag["a"])
	}
	if len(dag["b"]) != 1 || dag["b"][0] != "c" {
		t.Errorf("dag[b] = %v, want [c]", dag["b"])
	}
	if len(dag["c"]) != 0 {
		t.Errorf("dag[c] = %v, want []", dag["c"])
	}
}
