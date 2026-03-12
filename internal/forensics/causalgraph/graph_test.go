package causalgraph

import (
	"encoding/json"
	"testing"
)

func TestNodeType_Constants(t *testing.T) {
	t.Parallel()
	if NodeTypeLLMTurn != "llm_turn" {
		t.Errorf("NodeTypeLLMTurn = %q, want %q", NodeTypeLLMTurn, "llm_turn")
	}
	if NodeTypeToolCall != "tool_call" {
		t.Errorf("NodeTypeToolCall = %q, want %q", NodeTypeToolCall, "tool_call")
	}
}

func TestEdgeType_Constants(t *testing.T) {
	t.Parallel()
	if EdgeTypeContext != "context" {
		t.Errorf("EdgeTypeContext = %q, want %q", EdgeTypeContext, "context")
	}
	if EdgeTypeDataFlow != "data_flow" {
		t.Errorf("EdgeTypeDataFlow = %q, want %q", EdgeTypeDataFlow, "data_flow")
	}
}

func TestCausalGraph_ToAdjacencyList_Empty(t *testing.T) {
	t.Parallel()
	g := CausalGraph{}
	adj := g.ToAdjacencyList()
	if len(adj) != 0 {
		t.Errorf("expected empty adjacency list, got %v", adj)
	}
}

func TestCausalGraph_ToAdjacencyList(t *testing.T) {
	t.Parallel()
	g := CausalGraph{
		Nodes: []Node{
			{ID: "a", Type: NodeTypeLLMTurn, Step: 1},
			{ID: "b", Type: NodeTypeToolCall, Step: 1, ToolName: "bash"},
			{ID: "c", Type: NodeTypeLLMTurn, Step: 2},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeTypeContext},
			{From: "b", To: "c", Type: EdgeTypeDataFlow, MatchedToken: "foobar"},
		},
	}
	adj := g.ToAdjacencyList()
	if len(adj) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(adj))
	}
	if len(adj["a"]) != 1 || adj["a"][0] != "b" {
		t.Errorf("adj[a] = %v, want [b]", adj["a"])
	}
	if len(adj["b"]) != 1 || adj["b"][0] != "c" {
		t.Errorf("adj[b] = %v, want [c]", adj["b"])
	}
	if len(adj["c"]) != 0 {
		t.Errorf("adj[c] = %v, want []", adj["c"])
	}
}

func TestCausalGraph_ToAdjacencyList_MultipleEdgesFromSameNode(t *testing.T) {
	t.Parallel()
	g := CausalGraph{
		Nodes: []Node{
			{ID: "a", Type: NodeTypeLLMTurn, Step: 1},
			{ID: "b", Type: NodeTypeToolCall, Step: 1},
			{ID: "c", Type: NodeTypeToolCall, Step: 1},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeTypeContext},
			{From: "a", To: "c", Type: EdgeTypeContext},
		},
	}
	adj := g.ToAdjacencyList()
	if len(adj["a"]) != 2 {
		t.Errorf("adj[a] should have 2 neighbors, got %d", len(adj["a"]))
	}
}

func TestCausalGraph_JSON_RoundTrip(t *testing.T) {
	t.Parallel()
	g := CausalGraph{
		Nodes: []Node{
			{ID: "turn-1", Type: NodeTypeLLMTurn, Step: 1},
			{ID: "call-1", Type: NodeTypeToolCall, Step: 1, ToolName: "read"},
		},
		Edges: []Edge{
			{From: "turn-1", To: "call-1", Type: EdgeTypeContext},
		},
	}
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var got CausalGraph
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 2 || len(got.Edges) != 1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.Nodes[1].ToolName != "read" {
		t.Errorf("ToolName = %q, want %q", got.Nodes[1].ToolName, "read")
	}
}

func TestNode_ToolNameOmittedForLLMTurn(t *testing.T) {
	t.Parallel()
	n := Node{ID: "turn-1", Type: NodeTypeLLMTurn, Step: 1}
	data, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(data, &m)
	if _, ok := m["tool_name"]; ok {
		t.Error("tool_name should be omitted for LLM turn nodes")
	}
}

func TestEdge_MatchedTokenOmittedForContext(t *testing.T) {
	t.Parallel()
	e := Edge{From: "a", To: "b", Type: EdgeTypeContext}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(data, &m)
	if _, ok := m["matched_token"]; ok {
		t.Error("matched_token should be omitted for context edges")
	}
}
