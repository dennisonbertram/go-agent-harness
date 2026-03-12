package causalgraph

import (
	"testing"
)

func TestNewBuilder(t *testing.T) {
	t.Parallel()
	b := NewBuilder()
	if b == nil {
		t.Fatal("NewBuilder() returned nil")
	}
}

func TestBuilder_EmptyBuild(t *testing.T) {
	t.Parallel()
	b := NewBuilder()
	g := b.Build()
	if len(g.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(g.Edges))
	}
}

func TestBuilder_RecordTurn(t *testing.T) {
	t.Parallel()
	b := NewBuilder()
	b.RecordTurn(1, "turn-1", []string{"msg-0"})

	g := b.Build()
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
	n := g.Nodes[0]
	if n.ID != "turn-1" {
		t.Errorf("node ID = %q, want %q", n.ID, "turn-1")
	}
	if n.Type != NodeTypeLLMTurn {
		t.Errorf("node type = %q, want %q", n.Type, NodeTypeLLMTurn)
	}
	if n.Step != 1 {
		t.Errorf("node step = %d, want 1", n.Step)
	}
}

func TestBuilder_RecordToolCall(t *testing.T) {
	t.Parallel()
	b := NewBuilder()
	b.RecordToolCall(1, "call-1", "bash", `{"command":"ls"}`)

	g := b.Build()
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
	n := g.Nodes[0]
	if n.ID != "call-1" {
		t.Errorf("node ID = %q, want %q", n.ID, "call-1")
	}
	if n.Type != NodeTypeToolCall {
		t.Errorf("node type = %q, want %q", n.Type, NodeTypeToolCall)
	}
	if n.ToolName != "bash" {
		t.Errorf("tool name = %q, want %q", n.ToolName, "bash")
	}
}

func TestBuilder_ContextEdges(t *testing.T) {
	t.Parallel()
	b := NewBuilder()

	// Simulate: step 1 has tool call, step 2 LLM turn sees tool result
	b.RecordToolCall(1, "call-1", "bash", `{"command":"ls"}`)
	b.RecordToolResult(1, "call-1", "file1.go\nfile2.go")
	b.RecordTurn(2, "turn-2", []string{"call-1"})

	g := b.Build()
	// Should have 2 nodes (call-1, turn-2) and 1 context edge (call-1 -> turn-2)
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(g.Nodes))
	}

	contextEdges := filterEdges(g.Edges, EdgeTypeContext)
	if len(contextEdges) != 1 {
		t.Fatalf("expected 1 context edge, got %d", len(contextEdges))
	}
	e := contextEdges[0]
	if e.From != "call-1" || e.To != "turn-2" {
		t.Errorf("context edge = %s -> %s, want call-1 -> turn-2", e.From, e.To)
	}
}

func TestBuilder_ContextEdges_MultipleContextIDs(t *testing.T) {
	t.Parallel()
	b := NewBuilder()

	b.RecordToolCall(1, "call-1", "bash", `{"command":"ls"}`)
	b.RecordToolResult(1, "call-1", "result1")
	b.RecordToolCall(1, "call-2", "read", `{"path":"foo.go"}`)
	b.RecordToolResult(1, "call-2", "result2")
	b.RecordTurn(2, "turn-2", []string{"call-1", "call-2"})

	g := b.Build()
	contextEdges := filterEdges(g.Edges, EdgeTypeContext)
	if len(contextEdges) != 2 {
		t.Fatalf("expected 2 context edges, got %d", len(contextEdges))
	}
}

func TestBuilder_ContextEdges_UnknownContextID_Ignored(t *testing.T) {
	t.Parallel()
	b := NewBuilder()

	// Reference a contextID that was never recorded — should create edge anyway
	// (the node won't exist but the edge is still recorded for completeness)
	b.RecordTurn(1, "turn-1", []string{"unknown-id"})

	g := b.Build()
	// turn-1 should still be a node; edge to unknown-id should be recorded
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
	contextEdges := filterEdges(g.Edges, EdgeTypeContext)
	if len(contextEdges) != 1 {
		t.Fatalf("expected 1 context edge, got %d", len(contextEdges))
	}
}

func TestBuilder_DataFlowEdges(t *testing.T) {
	t.Parallel()
	b := NewBuilder()

	// Step 1: tool produces a result containing "important_value"
	b.RecordToolCall(1, "call-1", "bash", `{"command":"echo important_value"}`)
	b.RecordToolResult(1, "call-1", "important_value")

	// Step 2: LLM uses "important_value" in tool args
	b.RecordTurn(2, "turn-2", []string{"call-1"})
	b.RecordToolCall(2, "call-2", "write", `{"content":"important_value here"}`)
	b.RecordToolResult(2, "call-2", "done")

	g := b.Build()

	dataFlowEdges := filterEdges(g.Edges, EdgeTypeDataFlow)
	if len(dataFlowEdges) != 1 {
		t.Fatalf("expected 1 data flow edge, got %d: %+v", len(dataFlowEdges), dataFlowEdges)
	}
	e := dataFlowEdges[0]
	if e.From != "call-1" || e.To != "call-2" {
		t.Errorf("data flow edge = %s -> %s, want call-1 -> call-2", e.From, e.To)
	}
	if e.MatchedToken != "important_value" {
		t.Errorf("matched token = %q, want %q", e.MatchedToken, "important_value")
	}
}

func TestBuilder_NoDataFlowForShortTokens(t *testing.T) {
	t.Parallel()
	b := NewBuilder()

	// Short tokens (< 6 chars) should not create data flow edges
	b.RecordToolCall(1, "call-1", "bash", `{}`)
	b.RecordToolResult(1, "call-1", "hi ok")
	b.RecordTurn(2, "turn-2", []string{"call-1"})
	b.RecordToolCall(2, "call-2", "write", `{"content":"hi ok done"}`)
	b.RecordToolResult(2, "call-2", "done")

	g := b.Build()

	dataFlowEdges := filterEdges(g.Edges, EdgeTypeDataFlow)
	if len(dataFlowEdges) != 0 {
		t.Errorf("expected no data flow edges for short tokens, got %d: %+v", len(dataFlowEdges), dataFlowEdges)
	}
}

func TestBuilder_MultipleTurns_EndToEnd(t *testing.T) {
	t.Parallel()
	b := NewBuilder()

	// Step 1: LLM turn with no prior context
	b.RecordTurn(1, "turn-1", nil)
	b.RecordToolCall(1, "call-1", "bash", `{"command":"pwd"}`)
	b.RecordToolResult(1, "call-1", "/home/user/project")

	// Step 2: LLM turn sees call-1's result
	b.RecordTurn(2, "turn-2", []string{"call-1"})
	b.RecordToolCall(2, "call-2", "read", `{"path":"/home/user/project/main.go"}`)
	b.RecordToolResult(2, "call-2", "package main\nfunc main() {}")

	// Step 3: LLM turn sees call-2's result
	b.RecordTurn(3, "turn-3", []string{"call-2"})

	g := b.Build()

	// Check node count: 3 turns + 2 tool calls = 5
	if len(g.Nodes) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(g.Nodes))
	}

	// Check context edges: call-1->turn-2, call-2->turn-3
	contextEdges := filterEdges(g.Edges, EdgeTypeContext)
	if len(contextEdges) != 2 {
		t.Errorf("expected 2 context edges, got %d", len(contextEdges))
	}

	// Check data flow edges: /home/user/project from call-1 result appears in call-2 args
	dataFlowEdges := filterEdges(g.Edges, EdgeTypeDataFlow)
	if len(dataFlowEdges) == 0 {
		t.Error("expected at least 1 data flow edge for /home/user/project")
	}
}

func TestBuilder_DuplicateNodeIDs_NoDoubleCount(t *testing.T) {
	t.Parallel()
	b := NewBuilder()
	b.RecordTurn(1, "turn-1", nil)
	b.RecordTurn(1, "turn-1", nil) // duplicate

	g := b.Build()
	if len(g.Nodes) != 1 {
		t.Errorf("expected 1 node (deduped), got %d", len(g.Nodes))
	}
}

func filterEdges(edges []Edge, edgeType EdgeType) []Edge {
	var result []Edge
	for _, e := range edges {
		if e.Type == edgeType {
			result = append(result, e)
		}
	}
	return result
}
