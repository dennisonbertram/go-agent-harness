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

	// UnsafePreserveToolCalls, when true, keeps tool_calls in assistant
	// messages instead of stripping them. By default ALL tool calls are
	// stripped because many agent runners re-execute tool calls they see in
	// assistant messages; an attacker-crafted rollout could inject arbitrary
	// tool executions via fabricated ToolCall entries. Only set this to true
	// if the rollout source has been verified AND the consumer handles tool
	// calls safely.
	UnsafePreserveToolCalls bool

	// IncludeToolResults, when true, includes role=tool messages in the forked
	// output. By default tool messages are stripped because they contain
	// attacker-controlled "results" that were never actually executed — they are
	// fabricated in the rollout file. Many LLM providers inject these into context
	// and treat them as ground truth. Only set this to true if the rollout source
	// has been verified AND the consumer validates tool message authenticity.
	IncludeToolResults bool
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
	// ToolCallsStripped is true if tool_calls were removed from assistant
	// messages (default behavior). All tool calls are stripped unless
	// opts.UnsafePreserveToolCalls is set.
	ToolCallsStripped bool `json:"tool_calls_stripped,omitempty"`
	// ToolResultsStripped is true if role=tool messages were removed from
	// the forked output (default behavior for untrusted rollouts).
	ToolResultsStripped bool `json:"tool_results_stripped,omitempty"`
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
	// CRITICAL: UnsafePreserveToolCalls=true + IncludeToolResults=false is
	// semantically incoherent and dangerous. stripPendingToolCalls determines
	// "pending" by checking whether a role=tool message exists for each call_id.
	// When tool results are then stripped, the "completed" calls become pending
	// in the output — an attacker who fabricates a tool message to pass the
	// completion check can then have that call appear as a pending instruction
	// for the live runner after the results are removed. Refuse this combination
	// rather than silently producing unsafe output.
	if opts.UnsafePreserveToolCalls && !opts.IncludeToolResults {
		return nil, fmt.Errorf("replay: UnsafePreserveToolCalls=true requires IncludeToolResults=true: " +
			"stripping tool results after preserving completed calls re-exposes them as pending instructions")
	}

	// CRITICAL-1 fix: validate event ordering before sortEvents() can launder
	// a non-monotonic slice into an apparently-valid causal order.
	if err := validateEvents(events); err != nil {
		return nil, fmt.Errorf("replay: %w", err)
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

	// Strip all ToolCalls from assistant messages by default. Many agent runners
	// re-execute tool_calls they encounter in assistant messages; attacker-crafted
	// rollouts can fabricate completed calls to inject arbitrary tool executions.
	// UnsafePreserveToolCalls must be explicitly set for verified rollouts only.
	var toolCallsStripped bool
	var afterToolCallStrip []harness.Message
	if !opts.UnsafePreserveToolCalls {
		afterToolCallStrip, toolCallsStripped = stripAllToolCalls(raw)
	} else {
		// Even in unsafe mode, strip only pending calls (no completion recorded).
		// HIGH-8 fix (round 30): capture the stripped boolean instead of
		// discarding it. Discarding caused ForkResult.ToolCallsStripped to
		// always be false even when pending calls were removed, undermining
		// the audit trail integrity signal for consumers.
		afterToolCallStrip, toolCallsStripped = stripPendingToolCalls(raw)
	}

	// Strip role=tool messages by default. These contain attacker-fabricated
	// "results" that were never actually executed. Many LLM providers treat
	// tool messages as ground truth; keeping them in untrusted forks can
	// inject arbitrary tool outputs into a live runner's context.
	// IncludeToolResults must be explicitly set for verified rollouts only.
	var toolResultsStripped bool
	var messages []harness.Message
	if !opts.IncludeToolResults {
		for _, m := range afterToolCallStrip {
			if m.Role == "tool" {
				toolResultsStripped = true
				continue
			}
			messages = append(messages, m)
		}
	} else {
		messages = afterToolCallStrip
	}

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
		Messages:             messages,
		FromStep:             fromStep,
		OriginalStepCount:    maxStep,
		OriginalOutcome:      outcome,
		ToolCallsStripped:    toolCallsStripped,
		ToolResultsStripped:  toolResultsStripped,
		SystemPromptStripped: sysPromptStripped,
	}, nil
}

// stripAllToolCalls removes all ToolCalls from assistant messages. This is
// the default behavior for untrusted rollouts because many runners re-execute
// tool calls found in assistant messages; attacker-controlled rollouts can
// inject arbitrary tool executions via fabricated ToolCall entries.
// Returns the cleaned messages and whether any calls were stripped.
func stripAllToolCalls(messages []harness.Message) ([]harness.Message, bool) {
	stripped := false
	result := make([]harness.Message, len(messages))
	copy(result, messages)
	for i := range result {
		if result[i].Role == "assistant" && len(result[i].ToolCalls) > 0 {
			result[i].ToolCalls = nil
			stripped = true
		}
	}
	return result, stripped
}

// stripPendingToolCalls removes tool call entries from assistant messages whose
// call_ids have not been completed (no corresponding tool role message exists).
// This prevents attacker-crafted rollouts from forcing a live runner to execute
// arbitrary tool calls upon receiving the forked state. Returns the cleaned
// message slice and a boolean indicating whether any calls were stripped.
func stripPendingToolCalls(messages []harness.Message) ([]harness.Message, bool) {
	// Collect call_ids that have been completed (present as tool role messages).
	// HIGH-3 fix: validate len(ToolCallID) <= maxIDBytes before calling capID.
	// capID requires callers to pre-validate; ToolCallID can be up to 64 KiB
	// (maxDetailStringBytes), so oversized IDs must be skipped here.
	// An oversized ToolCallID is treated as "not completed" (strip it).
	completedIDs := make(map[string]bool)
	for _, m := range messages {
		if m.Role == "tool" && m.ToolCallID != "" {
			if len(m.ToolCallID) > maxIDBytes {
				continue // oversized — treat as not completed, will be stripped
			}
			completedIDs[capID(m.ToolCallID)] = true
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
			if len(tc.ID) > maxIDBytes {
				stripped = true // oversized IDs treated as pending (strip)
				continue
			}
			if completedIDs[capID(tc.ID)] {
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
