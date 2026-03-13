// Package replay provides offline simulation replay and fork-from-step
// capabilities for JSONL rollout files. It reuses the shared rollout loader
// from internal/forensics/rollout/.
//
// Phase 2 (Replayer): loads a rollout, reconstructs the message history,
// and for each tool call returns the recorded output instead of executing
// live. Verifies that the event sequence matches the original.
//
// Phase 3 (Fork): loads a rollout up to step N and reconstructs the
// []harness.Message history, ready to hand off to a live runner.
package replay

import (
	"encoding/json"
	"fmt"
	"sort"

	"go-agent-harness/internal/forensics/rollout"
	"go-agent-harness/internal/harness"
)

// ReplayEvent captures one step of an offline replay simulation.
type ReplayEvent struct {
	Step      int    `json:"step"`
	Type      string `json:"type"`
	EventType string `json:"event_type"`
	// Details holds event-specific information (tool name, args, result, etc.)
	Details map[string]any `json:"details,omitempty"`
	// Matched is true when the replayed event matches the original.
	Matched bool `json:"matched"`
}

// ReplayResult is the outcome of an offline simulation replay.
type ReplayResult struct {
	Events     []ReplayEvent `json:"events"`
	StepCount  int           `json:"step_count"`
	Matched    bool          `json:"matched"`
	Mismatches []string      `json:"mismatches,omitempty"`
}

// Replay performs an offline simulation of a rollout. For each tool call
// event it returns the recorded output (no live execution). It verifies
// the event sequence by checking that tool call starts have corresponding
// completions with matching call IDs.
func Replay(events []rollout.RolloutEvent) ReplayResult {
	var result ReplayResult
	result.Matched = true

	// Index tool call completions by call_id for lookup during replay.
	idx := indexToolCompletions(events)
	for _, dup := range idx.duplicates {
		result.Matched = false
		result.Mismatches = append(result.Mismatches,
			fmt.Sprintf("duplicate tool.call.completed for call_id %q", dup))
	}

	maxStep := 0
	for _, ev := range events {
		if ev.Step > maxStep {
			maxStep = ev.Step
		}

		re := ReplayEvent{
			Step:      ev.Step,
			Type:      "replay",
			EventType: ev.Type,
			Matched:   true,
		}

		switch ev.Type {
		case "tool.call.started":
			callID, callIDOK := payloadString(ev.Payload, "call_id")
			toolName, _ := payloadString(ev.Payload, "tool")
			args, _ := payloadString(ev.Payload, "arguments")

			re.Details = map[string]any{
				"tool":      toolName,
				"call_id":   callID,
				"arguments": args,
			}

			// Treat a missing or non-string call_id as a schema violation — it
			// prevents reliable completion matching and is always a mismatch.
			if !callIDOK || callID == "" {
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool call (%q) has missing or non-string call_id",
						ev.Step, toolName))
			} else if comp, ok := idx.results[callID]; ok {
				re.Details["result"] = comp
			} else {
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool call %q (%q) has no recorded completion",
						ev.Step, callID, toolName))
			}

		case "tool.call.completed":
			callID, _ := payloadString(ev.Payload, "call_id")
			toolResult, _ := payloadString(ev.Payload, "result")
			re.Details = map[string]any{
				"call_id": callID,
				"result":  toolResult,
			}

		case "llm.turn.completed":
			content, _ := payloadString(ev.Payload, "content")
			re.Details = map[string]any{"content": content}

		case "run.started", "run.completed", "run.failed":
			re.Details = copyPayload(ev.Payload)

		default:
			re.Details = copyPayload(ev.Payload)
		}

		result.Events = append(result.Events, re)
	}

	result.StepCount = maxStep
	return result
}

// completionIndex is the result of indexing tool completions from a rollout.
type completionIndex struct {
	// results maps call_id to the result string.
	results map[string]string
	// duplicates lists call_ids that appeared more than once.
	duplicates []string
}

// indexToolCompletions builds a map from call_id to the result string
// from tool.call.completed events. Only completions where call_id is a
// non-empty string are indexed. A non-string result is marshaled to JSON
// so that the replay output still reflects the actual recorded value.
// Duplicate call_ids are recorded for mismatch reporting.
func indexToolCompletions(events []rollout.RolloutEvent) completionIndex {
	m := make(map[string]string)
	seen := make(map[string]bool)
	var duplicates []string

	for _, ev := range events {
		if ev.Type != "tool.call.completed" || ev.Payload == nil {
			continue
		}
		callID, callIDOK := payloadString(ev.Payload, "call_id")
		if !callIDOK || callID == "" {
			continue
		}
		if seen[callID] {
			duplicates = append(duplicates, callID)
			continue // keep first result; flag as integrity failure
		}
		seen[callID] = true
		// Accept string result directly; for other types (object, array, number),
		// marshal to JSON so the content is not silently lost.
		result, ok := payloadString(ev.Payload, "result")
		if !ok {
			if raw, exists := ev.Payload["result"]; exists {
				if b, err := json.Marshal(raw); err == nil {
					result = string(b)
				}
			}
		}
		m[callID] = result
	}
	return completionIndex{results: m, duplicates: duplicates}
}

// sortEvents returns a copy of events sorted by (Step, file-order index).
// File-order index (not seq) is used as the tie-breaker to prevent attackers
// from reordering events within a step by controlling seq values. Honest
// recorders write events in causal order, so file order is authoritative.
func sortEvents(events []rollout.RolloutEvent) []rollout.RolloutEvent {
	type indexed struct {
		ev  rollout.RolloutEvent
		idx int
	}
	tmp := make([]indexed, len(events))
	for i, ev := range events {
		tmp[i] = indexed{ev: ev, idx: i}
	}
	sort.SliceStable(tmp, func(i, j int) bool {
		if tmp[i].ev.Step != tmp[j].ev.Step {
			return tmp[i].ev.Step < tmp[j].ev.Step
		}
		return tmp[i].idx < tmp[j].idx // file order within step
	})
	sorted := make([]rollout.RolloutEvent, len(events))
	for i, ie := range tmp {
		sorted[i] = ie.ev
	}
	return sorted
}

// ReconstructMessages rebuilds the []harness.Message conversation history
// from rollout events up to and including the given step. This is the
// foundation for both replay verification and fork-from-step.
// Events are sorted by (step, seq) before reconstruction to ensure
// deterministic ordering independent of file order.
//
// Causal validation: only tool.call.completed events whose call_id was
// previously announced in an llm.turn.completed.tool_calls list are included.
// This prevents attacker-crafted rollouts from injecting fake tool results
// into a forked conversation that will be handed to a live runner.
func ReconstructMessages(events []rollout.RolloutEvent, upToStep int) []harness.Message {
	var messages []harness.Message
	// announcedCalls tracks call_ids announced in llm.turn.completed.tool_calls.
	announcedCalls := make(map[string]bool)

	for _, ev := range sortEvents(events) {
		if ev.Step > upToStep {
			continue // skip out-of-window events
		}

		switch ev.Type {
		case "run.started":
			// The system prompt and initial user message are implicit in run.started.
			prompt, _ := payloadString(ev.Payload, "prompt")
			systemPrompt, _ := payloadString(ev.Payload, "system_prompt")
			if systemPrompt != "" {
				messages = append(messages, harness.Message{
					Role:    "system",
					Content: systemPrompt,
				})
			}
			if prompt != "" {
				messages = append(messages, harness.Message{
					Role:    "user",
					Content: prompt,
				})
			}

		case "llm.turn.completed":
			content, _ := payloadString(ev.Payload, "content")
			msg := harness.Message{
				Role:    "assistant",
				Content: content,
			}

			// Extract tool calls if present and track announced call_ids
			// for causal validation of subsequent tool.call.completed events.
			if tcs := extractToolCalls(ev.Payload); len(tcs) > 0 {
				msg.ToolCalls = tcs
				for _, tc := range tcs {
					if tc.ID != "" {
						announcedCalls[tc.ID] = true
					}
				}
			}

			messages = append(messages, msg)

		case "tool.call.completed":
			callID, _ := payloadString(ev.Payload, "call_id")
			toolResult, _ := payloadString(ev.Payload, "result")
			toolName, _ := payloadString(ev.Payload, "tool")
			// Only include tool results for calls that were previously announced
			// by the assistant AND have not already been completed. Clearing
			// announcedCalls[callID] after first use prevents the same call_id
			// from being injected multiple times as separate tool messages.
			if callID != "" && announcedCalls[callID] {
				delete(announcedCalls, callID) // one completion per call_id
				messages = append(messages, harness.Message{
					Role:       "tool",
					Content:    toolResult,
					ToolCallID: callID,
					Name:       toolName,
				})
			}

		case "steering.received":
			content, _ := payloadString(ev.Payload, "content")
			if content != "" {
				messages = append(messages, harness.Message{
					Role:    "user",
					Content: content,
				})
			}

		case "conversation.continued":
			message, _ := payloadString(ev.Payload, "message")
			if message != "" {
				messages = append(messages, harness.Message{
					Role:    "user",
					Content: message,
				})
			}
		}
	}

	return messages
}

// extractToolCalls extracts tool call objects from an llm.turn.completed payload.
func extractToolCalls(payload map[string]any) []harness.ToolCall {
	raw, ok := payload["tool_calls"]
	if !ok {
		return nil
	}

	// tool_calls may come as []any from JSON unmarshalling.
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}

	var calls []harness.ToolCall
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tc := harness.ToolCall{}
		if id, ok := obj["id"].(string); ok {
			tc.ID = id
		}
		if name, ok := obj["name"].(string); ok {
			tc.Name = name
		}
		if args, ok := obj["arguments"].(string); ok {
			tc.Arguments = args
		} else if args, ok := obj["arguments"].(map[string]any); ok {
			b, _ := json.Marshal(args)
			tc.Arguments = string(b)
		}
		calls = append(calls, tc)
	}
	return calls
}

// payloadString extracts a string value from a payload map.
func payloadString(payload map[string]any, key string) (string, bool) {
	if payload == nil {
		return "", false
	}
	v, ok := payload[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// copyPayload makes a shallow copy of a payload map. Returns nil for nil input.
func copyPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	out := make(map[string]any, len(payload))
	for k, v := range payload {
		out[k] = v
	}
	return out
}
