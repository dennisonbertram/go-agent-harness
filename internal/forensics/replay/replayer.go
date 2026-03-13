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
	"unicode/utf8"

	"go-agent-harness/internal/forensics/rollout"
	"go-agent-harness/internal/harness"
)

// maxMismatchStringBytes caps the length of attacker-controlled strings that
// are embedded in mismatch messages.
const maxMismatchStringBytes = 1024 // 1 KiB

// sanitizeMismatch strips control characters and Unicode bidi/format characters
// from attacker-controlled strings before embedding them in mismatch messages.
// Caps FIRST to bound the strings.Map allocation.
func sanitizeMismatch(s string) string {
	s = capString(s, maxMismatchStringBytes)
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) ||
			r == '\u2028' || r == '\u2029' {
			return -1
		}
		return r
	}, s)
}

// errCapExceeded is returned by cappedWriter.Write when the byte cap is reached.
var errCapExceeded = errors.New("cap exceeded")

// maxDetailStringBytes caps individual string fields stored in ReplayEvent.Details
// and harness.Message.
const maxDetailStringBytes = 65536 // 64 KiB

// capString truncates s to at most limit bytes at a rune boundary.
func capString(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "...<truncated>"
}

// deepCapStrings recursively caps all string values in v at maxDetailStringBytes.
// This prevents attacker-controlled nested strings (inside maps or arrays) from
// being retained verbatim in ReplayEvent.Details.
// CRITICAL-2 fix: copyPayloadCapped previously only capped top-level strings;
// nested structures like {"x":{"y":"<16MiB>"}} passed through verbatim.
func deepCapStrings(v any) any {
	switch val := v.(type) {
	case string:
		return capString(val, maxDetailStringBytes)
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v2 := range val {
			out[k] = deepCapStrings(v2)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = deepCapStrings(elem)
		}
		return out
	default:
		return v
	}
}

// maxIDBytes is the hard limit on tool call ID length. IDs exceeding this
// limit are rejected as schema violations. Prefix-hashing oversized IDs
// (as done in earlier versions) allows two IDs sharing the same N-byte prefix
// to collide, bypassing announcement and lifecycle integrity checks.
const maxIDBytes = 256

// capID returns a map key for an ID that has already been validated ≤ maxIDBytes.
// Callers MUST reject oversized IDs before calling this function.
func capID(id string) string {
	return "l:" + id
}

// maxTotalCallIDs caps how many distinct call IDs are tracked per pass.
const maxTotalCallIDs = 10000

// maxMismatches caps how many mismatch strings are stored in ReplayResult.
// Beyond this limit a sentinel is appended once and further messages dropped.
const maxMismatches = 1000

// mismatch sets result.Matched=false and re.Matched=false (if re != nil),
// then appends msg to result.Mismatches up to maxMismatches.
func mismatch(result *ReplayResult, re *ReplayEvent, msg string) {
	result.Matched = false
	if re != nil {
		re.Matched = false
	}
	if len(result.Mismatches) < maxMismatches {
		result.Mismatches = append(result.Mismatches, msg)
	} else if len(result.Mismatches) == maxMismatches {
		result.Mismatches = append(result.Mismatches,
			"(further mismatches suppressed)")
	}
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

	idx := indexToolCompletions(events)
	for _, dup := range idx.duplicates {
		mismatch(&result, nil,
			fmt.Sprintf("duplicate tool.call.completed for call_id %q", sanitizeMismatch(dup)))
	}

	// announcedCallNames maps capID(callID) → announced tool name from
	// llm.turn.completed.tool_calls (in file order). Enforces causal ordering
	// and enables HIGH-2 cross-check: announced tool must match started tool.
	announcedCallNames := make(map[string]string)
	startedCallIDs := make(map[string]bool)

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

			safeCallID := sanitizeMismatch(callID)
			safeToolName := sanitizeMismatch(toolName)

			if !callIDOK || callID == "" {
				mismatch(&result, &re, fmt.Sprintf(
					"step %d: tool call (%q) has missing or non-string call_id",
					ev.Step, safeToolName))
			} else if len(callID) > maxIDBytes {
				mismatch(&result, &re, fmt.Sprintf(
					"step %d: tool call (%q) call_id exceeds maximum length %d",
					ev.Step, safeToolName, maxIDBytes))
			} else if toolName == "" {
				// CRITICAL-1 fix: an empty tool name in tool.call.started is a
				// schema violation that bypasses both the announced-name cross-check
				// and the started-vs-completed name-consistency check. An attacker
				// can exploit this to start an arbitrary tool while the LLM announced
				// a different (safe) one, without triggering any mismatch.
				mismatch(&result, &re, fmt.Sprintf(
					"step %d: tool call %q has missing or empty tool name in tool.call.started",
					ev.Step, safeCallID))
			} else if announcedName, ok := announcedCallNames[capID(callID)]; !ok {
				mismatch(&result, &re, fmt.Sprintf(
					"step %d: tool call %q (%q) was never announced in llm.turn.completed.tool_calls",
					ev.Step, safeCallID, safeToolName))
			} else if announcedName != "" && announcedName != toolName {
				// HIGH-2 fix: cross-check announced vs started tool name.
				mismatch(&result, &re, fmt.Sprintf(
					"step %d: tool call %q: announced tool %q does not match started tool %q",
					ev.Step, safeCallID, sanitizeMismatch(announcedName), safeToolName))
			} else if startedCallIDs[capID(callID)] {
				// MEDIUM-7 fix: detect duplicate starts.
				mismatch(&result, &re, fmt.Sprintf(
					"step %d: duplicate tool.call.started for call_id %q",
					ev.Step, safeCallID))
			} else {
				startedCallIDs[capID(callID)] = true
				if comp, ok := idx.entries[capID(callID)]; ok {
					if comp.fileIndex <= i {
						mismatch(&result, &re, fmt.Sprintf(
							"step %d: tool call %q (%q) completion appears before started event in file order",
							ev.Step, safeCallID, safeToolName))
					} else if toolName != "" && comp.toolName != toolName {
						mismatch(&result, &re, fmt.Sprintf(
							"step %d: tool call %q: name mismatch between started (%q) and completed (%q)",
							ev.Step, safeCallID, safeToolName, sanitizeMismatch(comp.toolName)))
					} else {
						re.Details["result"] = comp.result
					}
				} else {
					mismatch(&result, &re, fmt.Sprintf(
						"step %d: tool call %q (%q) has no recorded completion",
						ev.Step, safeCallID, safeToolName))
				}
			}

		case "tool.call.completed":
			callID, callIDOK := payloadString(ev.Payload, "call_id")
			toolResult, _ := payloadString(ev.Payload, "result")
			toolName, _ := payloadString(ev.Payload, "tool") // LOW-2: include tool in details
			re.Details = map[string]any{
				"call_id": capString(callID, maxDetailStringBytes),
				"result":  capString(toolResult, maxDetailStringBytes),
				"tool":    capString(toolName, maxDetailStringBytes),
			}
			if !callIDOK || callID == "" {
				mismatch(&result, &re, fmt.Sprintf(
					"step %d: tool.call.completed has missing or non-string call_id", ev.Step))
			} else if len(callID) > maxIDBytes {
				// HIGH-1 fix: flag oversized completion call_ids as schema violations.
				// Previously indexToolCompletions silently skipped them, allowing
				// large attacker-controlled IDs to appear in replay events without
				// being matched or post-loop checked. Matched could remain true.
				mismatch(&result, &re, fmt.Sprintf(
					"step %d: tool.call.completed call_id exceeds maximum length %d", ev.Step, maxIDBytes))
			}

		case "llm.turn.completed":
			content, _ := payloadString(ev.Payload, "content")
			re.Details = map[string]any{"content": capString(content, maxDetailStringBytes)}
			for _, tc := range extractToolCalls(ev.Payload) {
				if tc.ID == "" || len(tc.ID) > maxIDBytes {
					continue
				}
				if len(announcedCallNames) < maxTotalCallIDs {
					announcedCallNames[capID(tc.ID)] = tc.Name
				}
			}

		case "run.started", "run.completed", "run.failed":
			re.Details = copyPayloadCapped(ev.Payload)

		default:
			re.Details = copyPayloadCapped(ev.Payload)
		}

		result.Events = append(result.Events, re)
	}

	// Post-loop: flag completions without a corresponding started.
	for key, entry := range idx.entries {
		if !startedCallIDs[key] {
			mismatch(&result, nil, fmt.Sprintf(
				"tool.call.completed for call_id %q has no corresponding tool.call.started",
				entry.rawID))
		}
	}

	result.StepCount = maxStep
	return result
}

// cappedWriter is an io.Writer that writes at most cap bytes, then returns
// errCapExceeded to cause json.Encoder to abort traversal immediately.
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

// cappedMarshal encodes v to JSON allocating at most capSize bytes.
func cappedMarshal(v any, capSize int) []byte {
	cw := &cappedWriter{cap: capSize}
	enc := json.NewEncoder(cw)
	if err := enc.Encode(v); err != nil && !errors.Is(err, errCapExceeded) {
		return nil
	}
	b := cw.buf
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b
}

// completionEntry holds the result, file-order index, tool name, and sanitized
// original call ID of a tool.call.completed event.
type completionEntry struct {
	result    string
	fileIndex int
	toolName  string
	rawID     string // sanitized original call_id for human-readable reporting
}

// completionIndex is the result of indexing tool completions from a rollout.
type completionIndex struct {
	entries    map[string]completionEntry
	duplicates []string
}

// indexToolCompletions builds a map from capID(call_id) to completionEntry.
// IDs exceeding maxIDBytes are silently skipped; the Replay() case "tool.call.completed"
// branch separately flags them as schema violations.
func indexToolCompletions(events []rollout.RolloutEvent) completionIndex {
	m := make(map[string]completionEntry)
	seen := make(map[string]bool)
	var duplicates []string

	for i, ev := range events {
		if ev.Type != "tool.call.completed" || ev.Payload == nil {
			continue
		}
		callID, callIDOK := payloadString(ev.Payload, "call_id")
		if !callIDOK || callID == "" || len(callID) > maxIDBytes {
			continue
		}
		key := capID(callID)
		if seen[key] {
			duplicates = append(duplicates, callID)
			continue
		}
		seen[key] = true
		const maxResultMarshalBytes = 65536
		result, ok := payloadString(ev.Payload, "result")
		if ok {
			result = capString(result, maxDetailStringBytes)
		} else {
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
		m[key] = completionEntry{
			result:    result,
			fileIndex: i,
			toolName:  compToolName,
			rawID:     sanitizeMismatch(callID),
		}
	}
	return completionIndex{entries: m, duplicates: duplicates}
}

// sortEvents returns a copy of events sorted by (Step, file-order index).
// File order is used as the tie-breaker to preserve causal ordering within
// a step independent of attacker-controlled seq values.
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
		return tmp[i].idx < tmp[j].idx
	})
	sorted := make([]rollout.RolloutEvent, len(events))
	for i, ie := range tmp {
		sorted[i] = ie.ev
	}
	return sorted
}

// ReconstructMessages rebuilds the []harness.Message conversation history
// from rollout events up to and including the given step.
// Events are sorted by (step, file-order index) before reconstruction.
//
// Causal validation: only tool.call.completed events whose call_id was
// previously announced in an llm.turn.completed.tool_calls list AND whose
// call_id was seen in a tool.call.started event are included in the output.
// This prevents attacker-crafted rollouts from injecting fake tool results.
//
// NOTE: events are assumed to come from rollout.LoadReader. Passing
// non-loader-validated events may produce unexpected results because the
// loader enforces monotonic steps, size limits, and line count bounds.
func ReconstructMessages(events []rollout.RolloutEvent, upToStep int) []harness.Message {
	var messages []harness.Message
	announcedCalls := make(map[string]bool)
	startedCalls := make(map[string]bool)

	for _, ev := range sortEvents(events) {
		if ev.Step > upToStep {
			continue
		}

		switch ev.Type {
		case "run.started":
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

			if tcs := extractToolCalls(ev.Payload); len(tcs) > 0 {
				msg.ToolCalls = tcs
				for _, tc := range tcs {
					if tc.ID != "" && len(tc.ID) <= maxIDBytes {
						if len(announcedCalls) < maxTotalCallIDs {
							announcedCalls[capID(tc.ID)] = true
						}
					}
				}
			}

			messages = append(messages, msg)

		case "tool.call.started":
			// Only accept if announced AND within size limit.
			if callID, ok := payloadString(ev.Payload, "call_id"); ok && callID != "" {
				if len(callID) <= maxIDBytes && announcedCalls[capID(callID)] {
					if len(startedCalls) < maxTotalCallIDs {
						startedCalls[capID(callID)] = true
					}
				}
			}

		case "tool.call.completed":
			callID, _ := payloadString(ev.Payload, "call_id")
			toolResult, _ := payloadString(ev.Payload, "result")
			toolName, _ := payloadString(ev.Payload, "tool")
			if callID != "" && len(callID) <= maxIDBytes &&
				announcedCalls[capID(callID)] && startedCalls[capID(callID)] {
				delete(announcedCalls, capID(callID)) // one completion per call_id
				// HIGH-2 fix: also clear startedCalls to prevent re-use of a
				// previously-started call_id for a subsequent completion without
				// a new tool.call.started event. Without this, re-announcing c1
				// after it completed allows a second tool.call.completed to be
				// accepted (startedCalls[c1] was still true from the first cycle).
				delete(startedCalls, capID(callID))
				messages = append(messages, harness.Message{
					Role:       "tool",
					Content:    capString(toolResult, maxDetailStringBytes),
					ToolCallID: capString(callID, maxDetailStringBytes),
					Name:       capString(toolName, maxDetailStringBytes),
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
// llm.turn.completed event.
const maxToolCallsPerTurn = 100

// maxToolArgMarshalBytes caps how many bytes a marshaled tool argument can occupy.
const maxToolArgMarshalBytes = 65536 // 64 KiB

// extractToolCalls extracts tool call objects from an llm.turn.completed payload.
func extractToolCalls(payload map[string]any) []harness.ToolCall {
	raw, ok := payload["tool_calls"]
	if !ok {
		return nil
	}

	arr, ok := raw.([]any)
	if !ok {
		return nil
	}

	var calls []harness.ToolCall
	for _, item := range arr {
		if len(calls) >= maxToolCallsPerTurn {
			break
		}
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tc := harness.ToolCall{}
		if id, ok := obj["id"].(string); ok {
			tc.ID = capString(id, maxDetailStringBytes)
		}
		if name, ok := obj["name"].(string); ok {
			tc.Name = capString(name, maxDetailStringBytes)
		}
		if args, ok := obj["arguments"].(string); ok {
			tc.Arguments = capString(args, maxToolArgMarshalBytes)
		} else if args, ok := obj["arguments"].(map[string]any); ok {
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

// copyPayloadCapped deep-caps all string values in a payload map at
// maxDetailStringBytes, recursing into nested maps and arrays.
// CRITICAL-2 fix: the earlier shallow cap only protected top-level strings;
// nested structures like {"x":{"y":"<16MiB>"}} passed through verbatim.
func copyPayloadCapped(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	out := make(map[string]any, len(payload))
	for k, v := range payload {
		out[k] = deepCapStrings(v)
	}
	return out
}
