// Package rollout provides loading and canonicalization of JSONL rollout files
// produced by the rollout recorder. It is the shared foundation for forensics
// tools including run comparison, replay, and causal graph analysis.
package rollout

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"time"
)

// MaxLineBytes is the maximum size of a single JSONL line. Lines exceeding
// this limit cause an immediate error (not a silent skip) because silently
// omitting events would be a forensics integrity failure.
const MaxLineBytes = 16 * 1024 * 1024 // 16 MiB

// MaxEvents is the maximum number of events that can be loaded from a single
// rollout file to prevent unbounded memory consumption.
const MaxEvents = 100_000

// MaxTotalBytes is the total raw byte budget across all events in a single
// load. Even with per-line and per-event caps, many large-but-valid events
// could exhaust memory; this bound prevents that.
const MaxTotalBytes = 256 * 1024 * 1024 // 256 MiB

// MaxStep is the maximum allowed step value in a rollout event. Events with
// steps outside [0, MaxStep] are rejected to prevent boundary-bypass attacks
// using negative or astronomically large step numbers.
const MaxStep = 1_000_000

// stepRequiredTypes is the set of event types that must carry an explicit
// data.step field. Omitting step in these types would silently move the event
// to step 0, allowing Fork(events, 0) to include injected events.
var stepRequiredTypes = map[string]bool{
	"llm.turn.completed":     true,
	"tool.call.started":      true,
	"tool.call.completed":    true,
	"steering.received":      true,
	"conversation.continued": true,
	"run.completed":          true,
	"run.failed":             true,
}

// RolloutEvent represents a single event from a JSONL rollout file.
type RolloutEvent struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Step      int            `json:"step,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// rawEvent matches the on-disk JSONL format written by the rollout recorder:
//
//	{"ts":"...","seq":N,"type":"...","data":{...}}
type rawEvent struct {
	Ts   time.Time      `json:"ts"`
	Seq  uint64         `json:"seq"`
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// jsonNestingDepth returns the maximum bracket nesting depth of a JSON byte
// slice. It is a fast pre-scan to reject pathologically nested structures
// before passing them to encoding/json which uses recursive descent.
// String contents are correctly skipped (including escape sequences) so that
// { and [ characters inside strings do not inflate the measured depth.
func jsonNestingDepth(data []byte) int {
	depth, maxDepth := 0, 0
	inString := false
	escaped := false
	for _, b := range data {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch b {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		case '}', ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return maxDepth
}

// maxJSONElements caps how many JSON values (counted by comma separators)
// are permitted per line. A flat JSON array [0,0,0,...N...] within MaxLineBytes
// can decode into far more memory than its raw byte count: each decoded
// interface{} value costs ~24 bytes on 64-bit platforms, so 16 MiB of raw
// bytes can yield hundreds of MB of allocations. Capping at 100_000 elements
// bounds worst-case allocation to roughly 2.4 MiB per line (100k × 24 bytes).
const maxJSONElements = 100_000

// jsonElementCount returns the number of JSON values in a byte slice by
// counting commas outside strings. It mirrors the scan pattern of
// jsonNestingDepth. The returned count is commas+1, i.e. the minimum number
// of distinct JSON values present.
func jsonElementCount(data []byte) int {
	commas := 0
	inString := false
	escaped := false
	for _, b := range data {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch b {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case ',':
			commas++
		}
	}
	return commas + 1 // at least one value even with zero commas
}

// LoadFile reads a JSONL rollout file from disk and returns the events.
// It rejects non-regular files (FIFOs, devices, symlinks to special files)
// to prevent indefinite hangs on streams that never EOF. On Unix, the file
// is opened with O_NONBLOCK to prevent blocking if the path was swapped to a
// FIFO between Stat and Open (TOCTOU mitigation); a second Stat on the open
// fd then confirms the file is regular.
func LoadFile(path string) ([]RolloutEvent, error) {
	// Pre-check for early error on obvious non-files (not found, permission, etc.).
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("rollout: stat %q: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("rollout: %q is not a regular file (mode: %s)", path, fi.Mode().Type())
	}
	// openRegularFile (loader_unix.go / loader_other.go) opens non-blocking on
	// Unix and re-checks IsRegular on the open fd to close the TOCTOU window.
	f, err := openRegularFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadReader(f)
}

// LoadReader reads JSONL-encoded rollout events from the given reader.
// Each line must be a valid JSON object matching the recorder's on-disk format.
// Blank lines are silently skipped. Lines exceeding MaxLineBytes cause an error
// (not a silent skip) because silently omitting events would be a forensics
// integrity failure. Returns an error if more than MaxEvents events are present
// or if total raw bytes exceed MaxTotalBytes.
// byteCounter wraps an io.Reader and counts all bytes read, returning an error
// if the total exceeds limit. This ensures newline delimiters are counted
// against the budget — unlike tracking only line payload bytes, which can be
// bypassed by a file consisting entirely of empty newline-only lines.
type byteCounter struct {
	r     io.Reader
	count int64
	limit int64
}

func (bc *byteCounter) Read(p []byte) (int, error) {
	n, err := bc.r.Read(p)
	bc.count += int64(n)
	if bc.count > bc.limit {
		return n, fmt.Errorf("rollout: exceeded maximum total byte budget (%d bytes)", bc.limit)
	}
	return n, err
}

func LoadReader(r io.Reader) ([]RolloutEvent, error) {
	var events []RolloutEvent
	counter := &byteCounter{r: r, limit: MaxTotalBytes}
	br := bufio.NewReaderSize(counter, 64*1024)

	lineNum := 0
	lastStep := -1 // tracks highest observed step in file order (monotonic enforcement)
	for {
		lineNum++
		// ReadLine handles arbitrarily long lines: it returns isPrefix=true
		// for lines that overflow the buffer. We accumulate until we have a
		// full line or detect that it is oversized.
		var line []byte
		for {
			chunk, isPrefix, err := br.ReadLine()
			if err != nil {
				if err == io.EOF {
					if len(line) > 0 {
						break // process last line without trailing newline
					}
					return events, nil
				}
				return nil, fmt.Errorf("rollout: read: %w", err)
			}
			line = append(line, chunk...)
			if len(line) > MaxLineBytes {
				// Return immediately — do not drain. Draining could loop
				// forever on infinite streams (e.g., /dev/zero, named pipes).
				// Oversized lines are an integrity failure in a forensics tool:
				// an attacker can hide events by placing them on large lines.
				return nil, fmt.Errorf("rollout: line %d exceeds maximum size (%d bytes)", lineNum, MaxLineBytes)
			}
			if !isPrefix {
				break
			}
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if len(events) >= MaxEvents {
			return nil, fmt.Errorf("rollout: exceeded maximum event limit (%d)", MaxEvents)
		}

		// maxJSONDepth caps JSON nesting depth to prevent stack overflow in
		// encoding/json's recursive descent parser on deeply nested structures.
		const maxJSONDepth = 100
		if depth := jsonNestingDepth(line); depth > maxJSONDepth {
			return nil, fmt.Errorf("rollout: line %d: JSON nesting depth %d exceeds maximum %d", lineNum, depth, maxJSONDepth)
		}

		// maxJSONElements guards against JSON amplification attacks: a flat array
		// [0,0,0,...N...] within MaxLineBytes can cause encoding/json to allocate
		// 12-24x the raw byte count via interface{} boxing. Cap before unmarshal.
		if count := jsonElementCount(line); count > maxJSONElements {
			return nil, fmt.Errorf("rollout: line %d: JSON element count %d exceeds maximum %d", lineNum, count, maxJSONElements)
		}

		var raw rawEvent
		if err := json.Unmarshal(line, &raw); err != nil {
			return nil, fmt.Errorf("rollout: line %d: %w", lineNum, err)
		}


		// Extract step from data payload if present. Validate that the step is
		// a finite, integral, non-negative value within bounds to prevent
		// boundary-bypass attacks using negative, fractional, NaN, overflowed,
		// or wrong-typed step values. Validation is performed on the float64
		// before truncation so that e.g. -0.5 does not silently become 0.
		// Unknown types (string, bool, object) are rejected — not silently
		// defaulted to 0 — to prevent events being moved to step 0 by type confusion.
		step := 0
		if raw.Data != nil {
			s, hasStep := raw.Data["step"]
			if !hasStep && stepRequiredTypes[raw.Type] {
				return nil, fmt.Errorf("rollout: line %d: event type %q requires data.step", lineNum, raw.Type)
			}
			if hasStep {
				switch v := s.(type) {
				case float64:
					if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) {
						return nil, fmt.Errorf("rollout: line %d: step must be a non-negative integer, got %g", lineNum, v)
					}
					if v < 0 || v > float64(MaxStep) {
						return nil, fmt.Errorf("rollout: line %d: step %g out of range [0, %d]", lineNum, v, MaxStep)
					}
					step = int(v)
				case int:
					if v < 0 || v > MaxStep {
						return nil, fmt.Errorf("rollout: line %d: step %d out of range [0, %d]", lineNum, v, MaxStep)
					}
					step = v
				default:
					return nil, fmt.Errorf("rollout: line %d: step must be a number, got %T", lineNum, v)
				}
			}
		} else if stepRequiredTypes[raw.Type] {
			return nil, fmt.Errorf("rollout: line %d: event type %q requires data.step", lineNum, raw.Type)
		}

		// stepRequiredTypes events must have step >= 1. run.started is the only
		// event type that is legitimately at step 0; all message-producing event
		// types come after the initial prompt and cannot validly be at step 0.
		// Allowing step=0 here would let an attacker backdate llm.turn.completed
		// or tool.call.completed events to step 0, causing Fork(events, 0) to
		// include attacker-crafted conversation history.
		if stepRequiredTypes[raw.Type] && step == 0 {
			return nil, fmt.Errorf("rollout: line %d: event type %q must have step >= 1, got 0", lineNum, raw.Type)
		}

		// Enforce monotonically non-decreasing steps for events that carry an
		// explicit step field. This prevents an attacker from placing a
		// tool.call.completed at step N after llm.turn.completed at step N+1
		// in file order, then relying on sort-by-step to reorder causality
		// and inject tool results into a forked conversation.
		if raw.Data != nil {
			if _, hasStep := raw.Data["step"]; hasStep {
				if step < lastStep {
					return nil, fmt.Errorf("rollout: line %d: step %d < previous step %d (steps must be non-decreasing in file order)", lineNum, step, lastStep)
				}
				if step > lastStep {
					lastStep = step
				}
			}
		}

		ev := RolloutEvent{
			ID:        fmt.Sprintf("%d", raw.Seq),
			Type:      raw.Type,
			Step:      step,
			Payload:   raw.Data,
			Timestamp: raw.Ts,
		}
		events = append(events, ev)
	}
}
