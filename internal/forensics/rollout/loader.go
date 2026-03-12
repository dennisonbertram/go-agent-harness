// Package rollout provides loading and canonicalization of JSONL rollout files
// produced by the rollout recorder. It is the shared foundation for forensics
// tools including run comparison, replay, and causal graph analysis.
package rollout

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// MaxLineBytes is the maximum size of a single JSONL line. Lines exceeding
// this limit are skipped with a warning rather than aborting the load.
const MaxLineBytes = 16 * 1024 * 1024 // 16 MiB

// MaxEvents is the maximum number of events that can be loaded from a single
// rollout file to prevent unbounded memory consumption.
const MaxEvents = 100_000

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
// Blank lines are silently skipped. Lines exceeding MaxLineBytes are skipped.
// Returns an error if more than MaxEvents events are present.
func LoadReader(r io.Reader) ([]RolloutEvent, error) {
	var events []RolloutEvent
	scanner := bufio.NewScanner(r)

	// Allow lines up to MaxLineBytes to handle large tool outputs without
	// aborting. Lines beyond this limit are skipped rather than failing.
	scanner.Buffer(make([]byte, 0, 64*1024), MaxLineBytes)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if len(events) >= MaxEvents {
			return nil, fmt.Errorf("rollout: exceeded maximum event limit (%d)", MaxEvents)
		}

		var raw rawEvent
		if err := json.Unmarshal(line, &raw); err != nil {
			return nil, fmt.Errorf("rollout: line %d: %w", lineNum, err)
		}

		// Extract step from data payload if present.
		step := 0
		if raw.Data != nil {
			if s, ok := raw.Data["step"]; ok {
				switch v := s.(type) {
				case float64:
					step = int(v)
				case int:
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
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("rollout: read: %w", err)
	}
	return events, nil
}
