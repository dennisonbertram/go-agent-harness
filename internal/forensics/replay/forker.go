package replay

import (
	"fmt"

	"go-agent-harness/internal/forensics/rollout"
	"go-agent-harness/internal/harness"
)

// ForkResult contains the reconstructed state needed to resume a run
// from a specific step.
type ForkResult struct {
	// Messages is the reconstructed conversation history up to FromStep.
	// Pending tool calls (announced but not yet completed) are stripped to
	// prevent attacker-crafted rollouts from forcing immediate tool execution
	// when this state is handed to a live runner.
	Messages []harness.Message `json:"messages"`
	// FromStep is the step from which the fork begins.
	FromStep int `json:"from_step"`
	// OriginalStepCount is the total number of steps in the original rollout.
	OriginalStepCount int `json:"original_step_count"`
	// OriginalOutcome is "completed", "failed", or "unknown".
	OriginalOutcome string `json:"original_outcome"`
	// PendingToolCallsStripped is true if any pending tool calls were removed
	// from the forked messages. Callers should be aware that the live runner
	// will not re-execute those calls.
	PendingToolCallsStripped bool `json:"pending_tool_calls_stripped,omitempty"`
}

// Fork loads a rollout up to the given step and reconstructs the
// []harness.Message history ready for handoff to a live runner.
// This is similar to ContinueRun but from an arbitrary step of
// a recorded rollout rather than from a live run's stored state.
//
// The fromStep parameter is inclusive: all events at step <= fromStep
// are included in the reconstructed message history.
//
// Returns an error if fromStep is negative or exceeds the rollout's
// maximum step number.
func Fork(events []rollout.RolloutEvent, fromStep int) (*ForkResult, error) {
	if fromStep < 0 {
		return nil, fmt.Errorf("replay: fromStep must be >= 0, got %d", fromStep)
	}

	maxStep := 0
	for _, ev := range events {
		if ev.Step > maxStep {
			maxStep = ev.Step
		}
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("replay: empty rollout, cannot fork")
	}

	if fromStep > maxStep {
		return nil, fmt.Errorf("replay: fromStep %d exceeds rollout max step %d", fromStep, maxStep)
	}

	messages, stripped := stripPendingToolCalls(ReconstructMessages(events, fromStep))

	outcome := "unknown"
	for i := len(events) - 1; i >= 0; i-- {
		switch events[i].Type {
		case "run.completed":
			outcome = "completed"
			goto done
		case "run.failed":
			outcome = "failed"
			goto done
		}
	}
done:

	return &ForkResult{
		Messages:                 messages,
		FromStep:                 fromStep,
		OriginalStepCount:        maxStep,
		OriginalOutcome:          outcome,
		PendingToolCallsStripped: stripped,
	}, nil
}

// stripPendingToolCalls removes tool call entries from assistant messages whose
// call_ids have not been completed (no corresponding tool role message exists).
// This prevents attacker-crafted rollouts from forcing a live runner to execute
// arbitrary tool calls upon receiving the forked state. Returns the cleaned
// message slice and a boolean indicating whether any calls were stripped.
func stripPendingToolCalls(messages []harness.Message) ([]harness.Message, bool) {
	// Collect call_ids that have been completed (present as tool role messages).
	completedIDs := make(map[string]bool)
	for _, m := range messages {
		if m.Role == "tool" && m.ToolCallID != "" {
			completedIDs[m.ToolCallID] = true
		}
	}

	stripped := false
	result := make([]harness.Message, len(messages))
	copy(result, messages)
	for i, m := range result {
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			continue
		}
		var kept []harness.ToolCall
		for _, tc := range m.ToolCalls {
			if completedIDs[tc.ID] {
				kept = append(kept, tc)
			} else {
				stripped = true
			}
		}
		result[i].ToolCalls = kept
		if len(kept) == 0 {
			result[i].ToolCalls = nil
		}
	}
	return result, stripped
}
