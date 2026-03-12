package causalgraph

// BlameNode represents a node in a blame chain with its causal relationship.
type BlameNode struct {
	// ID is the node identifier from the causal graph.
	ID string `json:"id"`
	// Type is the node type (e.g. "llm_turn", "tool_call").
	Type string `json:"type"`
	// Step is the 1-based step number this node belongs to.
	Step int `json:"step"`
	// Cause is the ID of the node that led to this node being included in
	// the blame chain. Empty for the target (first) node.
	Cause string `json:"cause,omitempty"`
}

// BlameChain performs a reverse BFS from targetNodeID back through the causal
// graph to all root causes. It returns the blame path as a slice of BlameNode
// ordered BFS-level by level (target first, then its predecessors, then their
// predecessors, etc.).
//
// Returns nil if targetNodeID does not exist in the graph. Handles cycles
// via a visited set.
func BlameChain(g CausalGraph, targetNodeID string) []BlameNode {
	// Build node lookup.
	nodeMap := make(map[string]Node, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeMap[n.ID] = n
	}

	// Check target exists.
	targetNode, exists := nodeMap[targetNodeID]
	if !exists {
		return nil
	}

	// Build reverse adjacency list: for each edge From->To, record To->From.
	reverse := make(map[string][]string)
	for _, e := range g.Edges {
		reverse[e.To] = append(reverse[e.To], e.From)
	}

	// BFS from target, following reverse edges.
	visited := make(map[string]bool)
	var chain []BlameNode

	type queueEntry struct {
		id    string
		cause string // the node that led us here
	}
	queue := []queueEntry{{id: targetNode.ID, cause: ""}}
	visited[targetNode.ID] = true

	for len(queue) > 0 {
		entry := queue[0]
		queue = queue[1:]

		node := nodeMap[entry.id]
		chain = append(chain, BlameNode{
			ID:    node.ID,
			Type:  string(node.Type),
			Step:  node.Step,
			Cause: entry.cause,
		})

		for _, predID := range reverse[entry.id] {
			if visited[predID] {
				continue
			}
			if _, ok := nodeMap[predID]; !ok {
				continue // skip edges to nodes not in the graph
			}
			visited[predID] = true
			queue = append(queue, queueEntry{id: predID, cause: entry.id})
		}
	}

	return chain
}

// ExportDAG exports the causal graph as a JSON-friendly adjacency list
// mapping each node ID to its list of outgoing neighbor IDs. This is an
// alias for CausalGraph.ToAdjacencyList provided for the Tier 3 API surface.
func ExportDAG(g CausalGraph) map[string][]string {
	return g.ToAdjacencyList()
}
