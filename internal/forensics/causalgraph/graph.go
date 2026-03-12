// Package causalgraph provides causal event graph construction for the agent
// harness forensics subsystem. It tracks mechanical context dependencies
// (Tier 1) and data-flow heuristics (Tier 2) between LLM turns and tool calls
// within a single run.
package causalgraph

// NodeType identifies the kind of causal graph node.
type NodeType string

const (
	// NodeTypeLLMTurn represents an LLM completion turn.
	NodeTypeLLMTurn NodeType = "llm_turn"
	// NodeTypeToolCall represents a tool call execution.
	NodeTypeToolCall NodeType = "tool_call"
)

// Node is a vertex in the causal graph.
type Node struct {
	ID       string   `json:"id"`
	Type     NodeType `json:"type"`
	Step     int      `json:"step"`
	ToolName string   `json:"tool_name,omitempty"`
}

// EdgeType identifies the kind of causal relationship.
type EdgeType string

const (
	// EdgeTypeContext means the source node was in context when the target was produced.
	EdgeTypeContext EdgeType = "context"
	// EdgeTypeDataFlow means an output token from the source appeared in the target's args.
	EdgeTypeDataFlow EdgeType = "data_flow"
)

// Edge is a directed relationship between two nodes in the causal graph.
type Edge struct {
	From         string   `json:"from"`
	To           string   `json:"to"`
	Type         EdgeType `json:"type"`
	MatchedToken string   `json:"matched_token,omitempty"`
}

// CausalGraph is the complete causal dependency graph for a run.
type CausalGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// ToAdjacencyList exports the graph as a JSON-friendly adjacency list
// mapping each node ID to its list of neighbor IDs (outgoing edges).
func (g CausalGraph) ToAdjacencyList() map[string][]string {
	adj := make(map[string][]string, len(g.Nodes))
	for _, n := range g.Nodes {
		if _, ok := adj[n.ID]; !ok {
			adj[n.ID] = []string{}
		}
	}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	return adj
}
