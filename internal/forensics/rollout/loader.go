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
func jsonNestingDepth(data []byte) int {
	depth, maxDepth := 0, 0
	for _, b := range data {
		switch b {
		case '{', '[':
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		case '}', ']':
			depth--
		}
	}
	return maxDepth
}

// LoadFile reads a JSONL rollout file from disk and returns the events.
func LoadFile(path string) ([]RolloutEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("rollout: open %s: %w", path, err)
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
func LoadReader(r io.Reader) ([]RolloutEvent, error) {
	var events []RolloutEvent
	br := bufio.NewReaderSize(r, 64*1024)
	totalBytes := 0

	lineNum := 0
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
		// Count raw bytes before trimming to prevent whitespace-padding attacks
		// that would otherwise bypass the total byte budget.
		totalBytes += len(line)
		if totalBytes > MaxTotalBytes {
			return nil, fmt.Errorf("rollout: exceeded maximum total byte budget (%d bytes)", MaxTotalBytes)
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

		var raw rawEvent
		if err := json.Unmarshal(line, &raw); err != nil {
			return nil, fmt.Errorf("rollout: line %d: %w", lineNum, err)
		}

		// stepRequiredTypes lists event types that affect message reconstruction
		// or run outcome determination. These must have an explicit data.step value
		// to prevent step-omission attacks that move events to step 0.
		stepRequiredTypes := map[string]bool{
			"llm.turn.completed":     true,
			"tool.call.started":      true,
			"tool.call.completed":    true,
			"steering.received":      true,
			"conversation.continued": true,
			"run.completed":          true,
			"run.failed":             true,
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
