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
	completions := indexToolCompletions(events)

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
			callID, _ := payloadString(ev.Payload, "call_id")
			toolName, _ := payloadString(ev.Payload, "tool")
			args, _ := payloadString(ev.Payload, "arguments")

			re.Details = map[string]any{
				"tool":      toolName,
				"call_id":   callID,
				"arguments": args,
			}

			// Look up the recorded completion result.
			if comp, ok := completions[callID]; ok {
				re.Details["result"] = comp
			} else if callID != "" {
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool call %s (%s) has no recorded completion",
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

// indexToolCompletions builds a map from call_id to the result string
// from tool.call.completed events.
func indexToolCompletions(events []rollout.RolloutEvent) map[string]string {
	m := make(map[string]string)
	for _, ev := range events {
		if ev.Type == "tool.call.completed" && ev.Payload != nil {
			callID, _ := payloadString(ev.Payload, "call_id")
			result, _ := payloadString(ev.Payload, "result")
			if callID != "" {
				m[callID] = result
			}
		}
	}
	return m
}

// ReconstructMessages rebuilds the []harness.Message conversation history
// from rollout events up to and including the given step. This is the
// foundation for both replay verification and fork-from-step.
func ReconstructMessages(events []rollout.RolloutEvent, upToStep int) []harness.Message {
	var messages []harness.Message

	for _, ev := range events {
		if ev.Step > upToStep {
			break
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

			// Extract tool calls if present.
			if tcs := extractToolCalls(ev.Payload); len(tcs) > 0 {
				msg.ToolCalls = tcs
			}

			messages = append(messages, msg)

		case "tool.call.completed":
			callID, _ := payloadString(ev.Payload, "call_id")
			toolResult, _ := payloadString(ev.Payload, "result")
			toolName, _ := payloadString(ev.Payload, "tool")
			if callID != "" {
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
