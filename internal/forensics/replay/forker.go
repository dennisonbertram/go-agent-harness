package replay

import (
	"fmt"

	"go-agent-harness/internal/forensics/rollout"
	"go-agent-harness/internal/harness"
)

// ForkOptions controls how Fork() reconstructs the forked conversation state.
// The defaults are the safest options for untrusted rollouts.
type ForkOptions struct {
	// IncludeSystemPrompt, when true, includes the system prompt from the
	// run.started event in the forked messages. Defaults to false because
	// the system prompt in a rollout file is untrusted — an attacker-crafted
	// rollout could inject an arbitrary system prompt into the live runner.
	// Only set this to true if the rollout source has been verified.
	IncludeSystemPrompt bool
}

// ForkResult contains the reconstructed state needed to resume a run
// from a specific step.
type ForkResult struct {
	// Messages is the reconstructed conversation history up to FromStep.
	// Pending tool calls (announced but not yet completed) are stripped to
	// prevent attacker-crafted rollouts from forcing immediate tool execution
	// when this state is handed to a live runner.
	// System prompts are excluded by default (see ForkOptions.IncludeSystemPrompt).
	//
	// WARNING: Do not pass Messages to a live runner with tools enabled
	// unless the rollout source has been verified as trusted. The message
	// history may contain attacker-crafted assistant/user/tool messages that
	// can steer the agent into unintended behavior.
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
	// SystemPromptStripped is true if the system prompt was omitted from
	// the forked messages (default behavior for untrusted rollouts).
	SystemPromptStripped bool `json:"system_prompt_stripped,omitempty"`
}

// Fork loads a rollout up to the given step and reconstructs the
// []harness.Message history ready for handoff to a live runner.
// This is similar to ContinueRun but from an arbitrary step of
// a recorded rollout rather than from a live run's stored state.
//
// The fromStep parameter is inclusive: all events at step <= fromStep
// are included in the reconstructed message history.
//
// The system prompt is excluded by default (opts.IncludeSystemPrompt=false)
// to prevent injection from untrusted rollout files. Pass non-nil opts with
// IncludeSystemPrompt=true only if the rollout source has been verified.
//
// Returns an error if fromStep is negative or exceeds the rollout's
// maximum step number.
func Fork(events []rollout.RolloutEvent, fromStep int, opts *ForkOptions) (*ForkResult, error) {
	if opts == nil {
		opts = &ForkOptions{}
	}
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

	raw := ReconstructMessages(events, fromStep)

	// Strip the system prompt by default to prevent injection from untrusted
	// rollout files. The live runner should provide its own system prompt.
	var sysPromptStripped bool
	if !opts.IncludeSystemPrompt {
		var filtered []harness.Message
		for _, m := range raw {
			if m.Role == "system" {
				sysPromptStripped = true
				continue
			}
			filtered = append(filtered, m)
		}
		raw = filtered
	}

	messages, pendingStripped := stripPendingToolCalls(raw)

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
		PendingToolCallsStripped: pendingStripped,
		SystemPromptStripped:     sysPromptStripped,
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
