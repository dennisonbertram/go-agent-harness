// Package rollout implements a JSONL-based event recorder that captures the
// complete timeline of a run for replay, fork, and audit purposes.
//
// Each run's events are written to a file at:
//
//	<dir>/<YYYY-MM-DD>/<run_id>.jsonl
//
// where each line is a JSON object with fields: ts, seq, type, data.
//
// The file is compatible with standard JSONL readers and can be grepped,
// replayed, or forked without additional tooling.
package rollout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RecordableEvent is the subset of harness.Event fields the recorder needs.
// Using a local type avoids a circular dependency between rollout and harness.
type RecordableEvent struct {
	// ID is the per-run event ID, e.g. "run_1:42".
	ID string
	// RunID is the run this event belongs to.
	RunID string
	// Type is the event type string, e.g. "run.started".
	Type string
	// Timestamp is when the event occurred (UTC).
	Timestamp time.Time
	// Payload contains event-specific data. May be nil.
	Payload map[string]any
	// Seq is the monotonic sequence number assigned by the caller at
	// event-emission time (before any lock contention on the recorder).
	// Callers MUST populate this field so that the JSONL file faithfully
	// reflects the logical emission order even when concurrent goroutines
	// race to acquire the recorder's write mutex.  The recorder writes this
	// value directly to the "seq" field on disk; it no longer maintains its
	// own internal counter.
	Seq uint64
}

// entry is the on-disk representation of a recorded event.
type entry struct {
	Ts   time.Time      `json:"ts"`
	Seq  uint64         `json:"seq"`
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// RecorderConfig holds configuration for a Recorder.
type RecorderConfig struct {
	// Dir is the root directory where rollout files are stored.
	// Files are written under <Dir>/<YYYY-MM-DD>/<RunID>.jsonl.
	// Must be non-empty.
	Dir string
	// RunID is the identifier of the run being recorded. Must be non-empty.
	RunID string
}

// Recorder writes run events to a JSONL file in a date-partitioned directory.
// It is safe for concurrent use.
//
// Ordering guarantee: each RecordableEvent carries a caller-assigned Seq value
// that is written verbatim to the JSONL "seq" field.  The recorder does NOT
// maintain an internal counter; it is the caller's responsibility to assign
// monotonically increasing sequence numbers before calling Record (typically
// under the caller's own ordering mutex).  This design means that even if two
// goroutines arrive at Record in a different order than their seq values, the
// JSONL file will still contain the correct logical sequence numbers so a
// reader can sort by seq to recover the true emission order.
type Recorder struct {
	mu     sync.Mutex
	file   *os.File
	enc    *json.Encoder
	closed bool
}

// NewRecorder creates a Recorder that stores events under cfg.Dir, partitioned
// by the current UTC date. Call Close when the run completes to flush and
// release the file handle.
func NewRecorder(cfg RecorderConfig) (*Recorder, error) {
	return NewRecorderAt(cfg, time.Now().UTC())
}

// NewRecorderAt is like NewRecorder but uses the provided time to determine the
// date-partition directory. This is primarily useful for testing.
func NewRecorderAt(cfg RecorderConfig, now time.Time) (*Recorder, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("rollout: Dir must not be empty")
	}
	if cfg.RunID == "" {
		return nil, fmt.Errorf("rollout: RunID must not be empty")
	}

	dateDir := filepath.Join(cfg.Dir, now.UTC().Format("2006-01-02"))
	if err := os.MkdirAll(dateDir, 0o755); err != nil {
		return nil, fmt.Errorf("rollout: create directory %s: %w", dateDir, err)
	}

	path := filepath.Join(dateDir, cfg.RunID+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("rollout: open file %s: %w", path, err)
	}

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	return &Recorder{
		file: f,
		enc:  enc,
	}, nil
}

// Record writes a single event to the JSONL file. It is safe for concurrent
// use. Errors during encoding are silently dropped to avoid impacting the
// primary run flow.
//
// The ev.Seq value is written verbatim to the on-disk "seq" field.  Callers
// must assign seq numbers under their own ordering primitive (e.g. an
// upstream mutex) before calling Record so that the logical emission order is
// preserved in the JSONL regardless of the order in which concurrent goroutines
// actually acquire the recorder's write mutex.
func (r *Recorder) Record(ev RecordableEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	e := entry{
		Ts:   ev.Timestamp,
		Seq:  ev.Seq,
		Type: ev.Type,
		Data: ev.Payload,
	}

	// Encoding errors are intentionally ignored: the recorder must never
	// crash or block the run loop.
	_ = r.enc.Encode(e)
}

// Close flushes any buffered data and closes the underlying file. Calling
// Close more than once is safe and returns nil after the first call.
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true
	return r.file.Close()
}
