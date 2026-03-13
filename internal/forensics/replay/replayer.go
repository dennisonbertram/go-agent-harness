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
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"go-agent-harness/internal/forensics/rollout"
	"go-agent-harness/internal/harness"
)

// sanitizeMismatch strips control characters and Unicode bidi/format characters
// from attacker-controlled strings before embedding them in mismatch messages.
// This prevents terminal/log injection if consumers print mismatch strings
// directly. The %q verb in fmt.Sprintf already escapes ASCII control chars,
// but unicode.IsControl misses Cf (bidi override), Zl (U+2028), and Zp (U+2029),
// which can spoof terminal output on some renderers.
//
// WARNING: ReplayEvent.Details values are NOT sanitized here — they are
// returned as-is for structured consumption. Callers that render Details
// directly to a terminal or log MUST sanitize the values themselves.
func sanitizeMismatch(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) ||
			r == '\u2028' || r == '\u2029' {
			return -1 // drop
		}
		return r
	}, s)
}

// errCapExceeded is returned by cappedWriter.Write when the byte cap is
// reached. Returning a real error (instead of silently succeeding) causes
// json.Encoder to abort traversal immediately, preventing CPU waste on
// encoding attacker-controlled structures that are beyond the cap.
var errCapExceeded = errors.New("cap exceeded")

// maxDetailStringBytes caps individual string fields stored in ReplayEvent.Details
// and harness.Message. Even though the rollout loader limits each line to
// MaxLineBytes (16 MiB), storing multiple MaxLineBytes strings per event
// would amplify memory usage. 64 KiB is generous for any legitimate field
// (content, arguments, result) while bounding worst-case per-field allocation.
const maxDetailStringBytes = 65536 // 64 KiB

// capString truncates s to at most limit bytes, appending a truncation marker.
// Used to prevent attacker-controlled strings from exhausting memory or
// producing oversized Details/harness.Message fields.
func capString(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "...<truncated>"
}

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
			fmt.Sprintf("duplicate tool.call.completed for call_id %q", sanitizeMismatch(dup)))
	}

	// announcedCallIDs is populated in file order as llm.turn.completed events
	// are processed in the main loop below. A tool call is only valid if its
	// announcing llm.turn.completed appears earlier in file order than the
	// tool.call.started event. This enforces causal ordering: a pre-scan would
	// be order-insensitive and allow a crafted rollout to place tool.call.started
	// before llm.turn.completed at the same step, retroactively validating it.
	announcedCallIDs := make(map[string]bool)

	maxStep := 0
	for i, ev := range events {
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
				"tool":      capString(toolName, maxDetailStringBytes),
				"call_id":   capString(callID, maxDetailStringBytes),
				"arguments": capString(args, maxDetailStringBytes),
			}

			// Treat a missing or non-string call_id as a schema violation — it
			// prevents reliable completion matching and is always a mismatch.
			safeCallID := sanitizeMismatch(callID)
			safeToolName := sanitizeMismatch(toolName)
			if !callIDOK || callID == "" {
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool call (%q) has missing or non-string call_id",
						ev.Step, safeToolName))
			} else if !announcedCallIDs[callID] {
				// Tool call was never announced by any llm.turn.completed.tool_calls.
				// This is a fabricated lifecycle: the model never requested this call.
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool call %q (%q) was never announced in llm.turn.completed.tool_calls",
						ev.Step, safeCallID, safeToolName))
			} else if comp, ok := idx.entries[callID]; ok {
				// Enforce lifecycle ordering: the completion must appear strictly
				// after the started event in file order. An attacker can place
				// tool.call.completed before tool.call.started at the same step
				// (satisfying the monotonic loader check) to inject a fabricated
				// result that was never actually produced by a tool execution.
				if comp.fileIndex <= i {
					re.Matched = false
					result.Matched = false
					result.Mismatches = append(result.Mismatches,
						fmt.Sprintf("step %d: tool call %q (%q) completion appears before started event in file order",
							ev.Step, safeCallID, safeToolName))
				} else if toolName != "" && comp.toolName != toolName {
					// Tool name mismatch: if the started event declares a tool name,
					// the completion MUST carry the same name. An absent comp.toolName
					// (empty string) is also a mismatch — an attacker can strip the
					// tool name from the completion event to bypass this check and
					// splice a result from a different tool under the same call_id.
					re.Matched = false
					result.Matched = false
					result.Mismatches = append(result.Mismatches,
						fmt.Sprintf("step %d: tool call %q: name mismatch between started (%q) and completed (%q)",
							ev.Step, safeCallID, safeToolName, sanitizeMismatch(comp.toolName)))
				} else {
					re.Details["result"] = comp.result
				}
			} else {
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool call %q (%q) has no recorded completion",
						ev.Step, safeCallID, safeToolName))
			}

		case "tool.call.completed":
			callID, _ := payloadString(ev.Payload, "call_id")
			toolResult, _ := payloadString(ev.Payload, "result")
			re.Details = map[string]any{
				"call_id": capString(callID, maxDetailStringBytes),
				"result":  capString(toolResult, maxDetailStringBytes),
			}

		case "llm.turn.completed":
			content, _ := payloadString(ev.Payload, "content")
			re.Details = map[string]any{"content": capString(content, maxDetailStringBytes)}
			// Announce tool calls in file order. Only call_ids announced by
			// a preceding (in file order) llm.turn.completed are valid for
			// tool.call.started events. This enforces causal ordering and
			// prevents a crafted rollout from using a retroactive announcement.
			for _, tc := range extractToolCalls(ev.Payload) {
				if tc.ID != "" {
					announcedCallIDs[tc.ID] = true
				}
			}

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

// cappedWriter is an io.Writer that writes at most cap bytes, then returns
// errCapExceeded to cause json.Encoder to abort traversal immediately.
// Aborting early prevents CPU waste: json.Encoder would otherwise continue
// encoding the entire attacker-controlled structure even though the output
// beyond the cap is discarded. The partial bytes accumulated before the cap
// are still available in buf for the caller to return as a truncated result.
type cappedWriter struct {
	buf []byte
	cap int
}

func (cw *cappedWriter) Write(p []byte) (int, error) {
	remaining := cw.cap - len(cw.buf)
	if remaining <= 0 {
		return 0, errCapExceeded
	}
	if len(p) > remaining {
		cw.buf = append(cw.buf, p[:remaining]...)
		return remaining, errCapExceeded
	}
	cw.buf = append(cw.buf, p...)
	return len(p), nil
}

// cappedMarshal encodes v to JSON allocating at most capSize bytes in the
// output buffer. The encoder aborts at the cap (via errCapExceeded) to avoid
// burning CPU on a structure that would be truncated anyway. Returns whatever
// bytes were written before the cap; returns nil only on non-cap encode errors.
// If len(result) == capSize the output is truncated and the caller should
// append a truncation marker.
func cappedMarshal(v any, capSize int) []byte {
	cw := &cappedWriter{cap: capSize}
	enc := json.NewEncoder(cw)
	if err := enc.Encode(v); err != nil && !errors.Is(err, errCapExceeded) {
		return nil
	}
	b := cw.buf
	// json.Encoder appends a trailing newline; strip it for clean output.
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b
}

// completionEntry holds the result string, 0-based file-order index, and tool
// name of a tool.call.completed event. The file index enforces lifecycle
// ordering (completion must appear after started). The tool name is used for
// identity consistency: an attacker can match call_ids across differently-named
// tools to splice fabricated results into a different tool's replay record.
type completionEntry struct {
	result    string
	fileIndex int    // 0-based position in the events slice (file order)
	toolName  string // tool name from tool.call.completed for consistency check
}

// completionIndex is the result of indexing tool completions from a rollout.
type completionIndex struct {
	// entries maps call_id to its completion entry (result + file position).
	entries map[string]completionEntry
	// duplicates lists call_ids that appeared more than once.
	duplicates []string
}

// indexToolCompletions builds a map from call_id to the completionEntry
// from tool.call.completed events. Only completions where call_id is a
// non-empty string are indexed. A non-string result is marshaled to JSON
// so that the replay output still reflects the actual recorded value.
// Duplicate call_ids are recorded for mismatch reporting.
func indexToolCompletions(events []rollout.RolloutEvent) completionIndex {
	m := make(map[string]completionEntry)
	seen := make(map[string]bool)
	var duplicates []string

	for i, ev := range events {
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
		// use cappedMarshal so at most maxResultMarshalBytes are ever allocated.
		// json.Marshal(raw) after json.Unmarshal can allocate up to MaxLineBytes
		// even when only 64 KiB is needed — the size cap check was too late.
		const maxResultMarshalBytes = 65536 // 64 KiB cap on marshaled non-string results
		result, ok := payloadString(ev.Payload, "result")
		if !ok {
			if raw, exists := ev.Payload["result"]; exists {
				if b := cappedMarshal(raw, maxResultMarshalBytes); b != nil {
					if len(b) >= maxResultMarshalBytes {
						b = append(b, []byte("...<truncated>")...)
					}
					result = string(b)
				}
			}
		}
		compToolName, _ := payloadString(ev.Payload, "tool")
		m[callID] = completionEntry{result: result, fileIndex: i, toolName: compToolName}
	}
	return completionIndex{entries: m, duplicates: duplicates}
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
					Content: capString(systemPrompt, maxDetailStringBytes),
				})
			}
			if prompt != "" {
				messages = append(messages, harness.Message{
					Role:    "user",
					Content: capString(prompt, maxDetailStringBytes),
				})
			}

		case "llm.turn.completed":
			content, _ := payloadString(ev.Payload, "content")
			msg := harness.Message{
				Role:    "assistant",
				Content: capString(content, maxDetailStringBytes),
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
					Content:    capString(toolResult, maxDetailStringBytes),
					ToolCallID: callID,
					Name:       toolName,
				})
			}

		case "steering.received":
			content, _ := payloadString(ev.Payload, "content")
			if content != "" {
				messages = append(messages, harness.Message{
					Role:    "user",
					Content: capString(content, maxDetailStringBytes),
				})
			}

		case "conversation.continued":
			message, _ := payloadString(ev.Payload, "message")
			if message != "" {
				messages = append(messages, harness.Message{
					Role:    "user",
					Content: capString(message, maxDetailStringBytes),
				})
			}
		}
	}

	return messages
}

// maxToolCallsPerTurn caps how many tool calls are extracted from a single
// llm.turn.completed event to prevent amplification from adversarially large
// tool_calls arrays.
const maxToolCallsPerTurn = 100

// maxToolArgMarshalBytes caps how many bytes a marshaled tool argument can
// occupy to prevent json.Marshal amplification on map[string]any arguments.
const maxToolArgMarshalBytes = 65536 // 64 KiB

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
		if len(calls) >= maxToolCallsPerTurn {
			break // bound iteration
		}
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
			// Use cappedMarshal so at most maxToolArgMarshalBytes are ever allocated.
			// json.Marshal(args) after json.Unmarshal can allocate up to MaxLineBytes
			// before the post-marshal size check could truncate it.
			if b := cappedMarshal(args, maxToolArgMarshalBytes); b != nil {
				if len(b) >= maxToolArgMarshalBytes {
					b = append(b, []byte("...<truncated>")...)
				}
				tc.Arguments = string(b)
			}
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
