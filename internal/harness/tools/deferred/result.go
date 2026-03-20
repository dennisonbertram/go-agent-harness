package deferred

// ChildResult is the unified result schema returned by all child-agent
// completion paths: task_complete, spawn_agent, and run_agent.
//
// All three tools normalize their output to this schema so that parent agents
// can parse child completion results without branching on which tool was used.
//
// Backward compatibility: existing fields (output, jsonl, run_id, profile) are
// preserved so callers that already parse those fields continue to work.
type ChildResult struct {
	// Summary is a 1-3 sentence description of what the child accomplished.
	// Populated by all completion paths.
	Summary string `json:"summary"`

	// Status is the terminal state of the child run.
	// One of: "completed", "partial", "failed".
	Status string `json:"status"`

	// Findings holds structured key discoveries from task_complete.
	// Omitted when empty.
	Findings []TaskCompleteFinding `json:"findings,omitempty"`

	// Output holds the raw text output from run_agent / spawn_agent fallback.
	// Omitted when empty.
	Output string `json:"output,omitempty"`

	// Profile holds the profile name used by run_agent.
	// Omitted when empty.
	Profile string `json:"profile,omitempty"`
}
