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
// are embedded in mismatch messages. Without this cap, a 16 MiB call_id would
// cause fmt.Sprintf("%q", callID) to allocate at least 16 MiB per mismatch
// message, enabling DoS via many mismatches or a single enormous ID.
const maxMismatchStringBytes = 1024 // 1 KiB — generous for any legitimate ID

// sanitizeMismatch strips control characters and Unicode bidi/format characters
// from attacker-controlled strings before embedding them in mismatch messages,
// and caps the result at maxMismatchStringBytes to prevent DoS via enormous
// attacker-controlled strings being embedded in fmt.Sprintf calls.
//
// WARNING: ReplayEvent.Details values are NOT sanitized here — they are
// returned as-is for structured consumption. Callers that render Details
// directly to a terminal or log MUST sanitize the values themselves.
func sanitizeMismatch(s string) string {
	// Cap FIRST to bound the strings.Map allocation. strings.Map traverses the
	// full string before returning — for a 16 MiB attacker-controlled call_id
	// this allocates and scans 16 MiB per mismatch formatting. Capping here
	// limits the traversal to maxMismatchStringBytes regardless of input size.
	s = capString(s, maxMismatchStringBytes)
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

// capString truncates s to at most limit bytes at a rune boundary, appending
// a truncation marker. Truncating at a byte boundary (s[:limit]) can produce
// invalid UTF-8 if limit falls inside a multi-byte rune; scanning back to the
// last rune-start byte guarantees the output is always valid UTF-8.
func capString(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	// Find the last byte position at or before limit that starts a UTF-8 rune.
	// utf8.RuneStart returns true for the first byte of any rune (single-byte
	// ASCII or the leading byte of a multi-byte sequence), false for continuation
	// bytes. Scanning backwards at most 3 positions is sufficient because UTF-8
	// multi-byte sequences are at most 4 bytes; in practice we stop immediately
	// for ASCII-heavy strings.
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "...<truncated>"
}

// maxIDBytes is the hard limit on tool call ID length. Legitimate IDs
// (UUIDs, provider-generated identifiers) are always well under 128 chars.
// IDs exceeding this limit are treated as schema violations and rejected
// rather than hashed — prefix-hashing allows two IDs that share the same
// N-byte prefix to produce identical map keys, bypassing announcement and
// lifecycle integrity checks. Rejection is safe because no real tool runner
// produces IDs longer than a UUID (36 bytes).
const maxIDBytes = 256

// capID returns a map key for an ID that has already been validated to be
// within maxIDBytes. Callers MUST check len(id) <= maxIDBytes before calling
// this function and reject oversized IDs as schema violations. The "l:" prefix
// separates this key namespace from any future hashed key namespaces.
func capID(id string) string {
	return "l:" + id
}

// maxTotalCallIDs caps how many distinct call IDs can be tracked in the
// announcement, started, and completion maps during a single replay or
// reconstruct pass. An adversarial rollout with many llm.turn.completed
// events could otherwise force these maps to hold millions of entries,
// exhausting memory. The cap is generous relative to MaxEvents (100k).
const maxTotalCallIDs = 10000

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

	// announcedCallNames is populated in file order as llm.turn.completed events
	// are processed. The value is the tool name announced for each call_id.
	// A tool call is only valid if its announcing llm.turn.completed appears
	// earlier in file order than the tool.call.started event (causal ordering).
	// HIGH-2: we also store the announced tool name so we can cross-check it
	// against the tool name in tool.call.started — an attacker can announce
	// "read_file" but actually start "bash" to mislead forensics consumers.
	announcedCallNames := make(map[string]string) // capID(callID) → announced tool name

	// startedCallIDs tracks call_ids that have been seen in tool.call.started events.
	// After the main loop, completions without a corresponding started are flagged.
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
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool call (%q) has missing or non-string call_id",
						ev.Step, safeToolName))
			} else if len(callID) > maxIDBytes {
				// CRITICAL-1 fix: reject oversized IDs as schema violations rather
				// than hashing a prefix. Prefix-hashing allows two IDs that share the
				// same N-byte prefix to collide, silently bypassing announcement and
				// lifecycle integrity checks. Legitimate IDs are always short.
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool call (%q) call_id exceeds maximum length %d",
						ev.Step, safeToolName, maxIDBytes))
			} else if announcedName, ok := announcedCallNames[capID(callID)]; !ok {
				// Tool call was never announced by a preceding llm.turn.completed.
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool call %q (%q) was never announced in llm.turn.completed.tool_calls",
						ev.Step, safeCallID, safeToolName))
			} else if announcedName != "" && toolName != "" && announcedName != toolName {
				// HIGH-2 fix: the LLM-announced tool name must match the started name.
				// An attacker can craft a rollout where the LLM "announces" a safe tool
				// while the lifecycle events record a dangerous one, misleading consumers.
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool call %q: announced tool %q does not match started tool %q",
						ev.Step, safeCallID, sanitizeMismatch(announcedName), safeToolName))
			} else if startedCallIDs[capID(callID)] {
				// MEDIUM-7 fix: detect duplicate tool.call.started for the same call_id.
				// A rollout with multiple starts for the same call can mask corruption
				// by reusing one completion entry across multiple started events.
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: duplicate tool.call.started for call_id %q",
						ev.Step, safeCallID))
			} else {
				// All checks passed — mark as started before checking completion.
				startedCallIDs[capID(callID)] = true
				if comp, ok := idx.entries[capID(callID)]; ok {
					// Enforce lifecycle ordering: the completion must appear strictly
					// after the started event in file order.
					if comp.fileIndex <= i {
						re.Matched = false
						result.Matched = false
						result.Mismatches = append(result.Mismatches,
							fmt.Sprintf("step %d: tool call %q (%q) completion appears before started event in file order",
								ev.Step, safeCallID, safeToolName))
					} else if toolName != "" && comp.toolName != toolName {
						// Tool name mismatch between started and completed events.
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
			}

		case "tool.call.completed":
			callID, callIDOK := payloadString(ev.Payload, "call_id")
			toolResult, _ := payloadString(ev.Payload, "result")
			re.Details = map[string]any{
				"call_id": capString(callID, maxDetailStringBytes),
				"result":  capString(toolResult, maxDetailStringBytes),
			}
			// A tool.call.completed with missing/non-string call_id is a schema
			// violation: it cannot be matched to any started event.
			if !callIDOK || callID == "" {
				re.Matched = false
				result.Matched = false
				result.Mismatches = append(result.Mismatches,
					fmt.Sprintf("step %d: tool.call.completed has missing or non-string call_id", ev.Step))
			}

		case "llm.turn.completed":
			content, _ := payloadString(ev.Payload, "content")
			re.Details = map[string]any{"content": capString(content, maxDetailStringBytes)}
			// Announce tool calls in file order. Only call_ids announced by
			// a preceding llm.turn.completed are valid for tool.call.started.
			// HIGH-2: store the announced tool name alongside the call_id for
			// cross-checking against the tool name in tool.call.started.
			for _, tc := range extractToolCalls(ev.Payload) {
				if tc.ID == "" || len(tc.ID) > maxIDBytes {
					continue // skip invalid IDs
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

	// Post-loop: flag any tool.call.completed whose call_id was never seen in
	// a tool.call.started. A crafted rollout with llm.turn.completed +
	// tool.call.completed but no tool.call.started can produce Matched=true
	// (no started means no lifecycle checks) while injecting a tool result
	// into ReconstructMessages.
	// LOW-10 fix: use the capped+sanitized rawID stored in the entry for
	// human-readable reporting rather than the capID key (which has "l:" prefix).
	for key, entry := range idx.entries {
		if !startedCallIDs[key] {
			result.Matched = false
			result.Mismatches = append(result.Mismatches,
				fmt.Sprintf("tool.call.completed for call_id %q has no corresponding tool.call.started",
					entry.rawID))
		}
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

// completionEntry holds the result string, 0-based file-order index, tool
// name, and sanitized original call ID of a tool.call.completed event.
type completionEntry struct {
	result    string
	fileIndex int    // 0-based position in the events slice (file order)
	toolName  string // tool name from tool.call.completed for consistency check
	rawID     string // sanitized, capped original call_id for human-readable reporting
}

// completionIndex is the result of indexing tool completions from a rollout.
type completionIndex struct {
	// entries maps capID(call_id) to its completion entry.
	entries map[string]completionEntry
	// duplicates lists original call_ids that appeared more than once.
	duplicates []string
}

// indexToolCompletions builds a map from capID(call_id) to the completionEntry
// from tool.call.completed events. Call IDs exceeding maxIDBytes are skipped
// as schema violations — see capID comment for why prefix-hashing oversized IDs
// allows collision attacks on the announcement/lifecycle integrity checks.
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
		// Reject oversized IDs as schema violations (see capID comment).
		if len(callID) > maxIDBytes {
			continue
		}
		key := capID(callID)
		if seen[key] {
			duplicates = append(duplicates, callID)
			continue // keep first result; flag as integrity failure
		}
		seen[key] = true
		// Accept string result directly (with cap); for other types, use
		// cappedMarshal so at most maxResultMarshalBytes are ever allocated.
		const maxResultMarshalBytes = 65536 // 64 KiB cap on marshaled non-string results
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
			rawID:     sanitizeMismatch(callID), // capped+sanitized for reporting
		}
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
// previously announced in an llm.turn.completed.tool_calls list AND whose
// call_id was seen in a tool.call.started event are included. This prevents
// attacker-crafted rollouts from injecting fake tool results into a forked
// conversation that will be handed to a live runner.
func ReconstructMessages(events []rollout.RolloutEvent, upToStep int) []harness.Message {
	var messages []harness.Message
	// announcedCalls tracks call_ids announced in llm.turn.completed.tool_calls.
	announcedCalls := make(map[string]bool)
	// startedCalls tracks call_ids that have been seen in tool.call.started events
	// AND were previously announced. Both conditions must be met before a
	// tool.call.completed is accepted.
	startedCalls := make(map[string]bool)

	for _, ev := range sortEvents(events) {
		if ev.Step > upToStep {
			continue // skip out-of-window events
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
			// Record that this call was started — but ONLY if it was previously
			// announced in an llm.turn.completed.tool_calls list AND the ID is
			// within the valid size limit. Accepting a tool.call.started without
			// a prior announcement would allow a crafted rollout to inject a
			// started event before the llm.turn.completed, retroactively
			// validating a call that was never requested by the model.
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
			// Only include tool results for calls that were announced AND started.
			// Clearing announcedCalls after first use prevents re-injection.
			if callID != "" && len(callID) <= maxIDBytes &&
				announcedCalls[capID(callID)] && startedCalls[capID(callID)] {
				delete(announcedCalls, capID(callID)) // one completion per call_id
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

// copyPayloadCapped makes a shallow copy of a payload map, capping all string
// values at maxDetailStringBytes. Used for Details fields in replay events to
// prevent attacker-controlled strings from being retained verbatim in
// ReplayResult.Events[i].Details (which callers may log, print, or store).
// HIGH-3 fix: without this cap, a rollout with a 16 MiB string in run.started
// payload would be stored verbatim, enabling memory exhaustion in the consumer.
func copyPayloadCapped(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	out := make(map[string]any, len(payload))
	for k, v := range payload {
		if s, ok := v.(string); ok {
			out[k] = capString(s, maxDetailStringBytes)
		} else {
			out[k] = v
		}
	}
	return out
}
