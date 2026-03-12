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
// this limit are skipped with a warning rather than aborting the load.
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
		oversized := false
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
			if !oversized {
				line = append(line, chunk...)
				if len(line) > MaxLineBytes {
					// Stop accumulating — drain remaining chunks without storing.
					oversized = true
					line = line[:0] // release memory
				}
			}
			// When oversized, drain chunks until end-of-line without appending.
			if !isPrefix {
				break
			}
		}
		if oversized {
			// Oversized lines are treated as integrity failures in a forensics
			// tool: silently skipping them could allow an attacker to hide
			// critical events by placing them on intentionally large lines.
			return nil, fmt.Errorf("rollout: line %d exceeds maximum size (%d bytes)", lineNum, MaxLineBytes)
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if len(events) >= MaxEvents {
			return nil, fmt.Errorf("rollout: exceeded maximum event limit (%d)", MaxEvents)
		}
		totalBytes += len(line)
		if totalBytes > MaxTotalBytes {
			return nil, fmt.Errorf("rollout: exceeded maximum total byte budget (%d bytes)", MaxTotalBytes)
		}

		var raw rawEvent
		if err := json.Unmarshal(line, &raw); err != nil {
			return nil, fmt.Errorf("rollout: line %d: %w", lineNum, err)
		}

		// Extract step from data payload if present. Validate that the step is
		// a finite, integral, non-negative value within bounds to prevent
		// boundary-bypass attacks using negative, fractional, NaN, or overflowed
		// step values. Validation is performed on the float64 before truncation
		// so that e.g. -0.5 does not silently become 0.
		step := 0
		if raw.Data != nil {
			if s, ok := raw.Data["step"]; ok {
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
