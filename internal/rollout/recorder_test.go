package rollout_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/rollout"
)

// entry matches the JSONL format written by the recorder.
type entry struct {
	Ts   time.Time      `json:"ts"`
	Seq  uint64         `json:"seq"`
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

func readEntries(t *testing.T, path string) []entry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open rollout file: %v", err)
	}
	defer f.Close()

	var entries []entry
	sc := bufio.NewScanner(f)
	// Increase buffer for potentially large lines.
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal entry: %v", err)
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return entries
}

// TestRecorder_BasicWrite verifies the recorder creates a JSONL file with the
// correct format and content for a simple sequence of events.
func TestRecorder_BasicWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec, err := rollout.NewRecorder(rollout.RecorderConfig{
		Dir:   dir,
		RunID: "run_1",
	})
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	defer rec.Close()

	events := []rollout.RecordableEvent{
		{ID: "run_1:0", RunID: "run_1", Type: "run.started", Timestamp: time.Now().UTC(), Payload: map[string]any{"prompt": "hello"}},
		{ID: "run_1:1", RunID: "run_1", Type: "llm.turn.requested", Timestamp: time.Now().UTC(), Payload: map[string]any{"step": 1}},
		{ID: "run_1:2", RunID: "run_1", Type: "run.completed", Timestamp: time.Now().UTC(), Payload: map[string]any{"output": "done"}},
	}
	for _, e := range events {
		rec.Record(e)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Find the file written.
	var files []string
	if err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(p, ".jsonl") {
			files = append(files, p)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one .jsonl file, found none")
	}

	entries := readEntries(t, files[0])
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify first entry.
	e0 := entries[0]
	if e0.Type != "run.started" {
		t.Errorf("entry[0].type = %q, want %q", e0.Type, "run.started")
	}
	if e0.Ts.IsZero() {
		t.Error("entry[0].ts is zero")
	}
	if e0.Data["prompt"] != "hello" {
		t.Errorf("entry[0].data[prompt] = %v, want %q", e0.Data["prompt"], "hello")
	}

	// Verify sequence ordering.
	if entries[1].Type != "llm.turn.requested" {
		t.Errorf("entry[1].type = %q, want %q", entries[1].Type, "llm.turn.requested")
	}
	if entries[2].Type != "run.completed" {
		t.Errorf("entry[2].type = %q, want %q", entries[2].Type, "run.completed")
	}
}

// TestRecorder_FileLayout verifies the JSONL file is stored at the expected path:
// <dir>/<YYYY-MM-DD>/<runID>.jsonl
func TestRecorder_FileLayout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Date(2026, 3, 9, 14, 30, 0, 0, time.UTC)

	rec, err := rollout.NewRecorderAt(rollout.RecorderConfig{
		Dir:   dir,
		RunID: "run_abc",
	}, now)
	if err != nil {
		t.Fatalf("NewRecorderAt: %v", err)
	}
	rec.Record(rollout.RecordableEvent{ID: "run_abc:0", RunID: "run_abc", Type: "run.started", Timestamp: now})
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	expectedPath := filepath.Join(dir, "2026-03-09", "run_abc.jsonl")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected file at %s: %v", expectedPath, err)
	}
}

// TestRecorder_Seq verifies that the seq field increments correctly.
func TestRecorder_Seq(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec, err := rollout.NewRecorder(rollout.RecorderConfig{
		Dir:   dir,
		RunID: "run_seq",
	})
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	for i := uint64(0); i < 5; i++ {
		rec.Record(rollout.RecordableEvent{
			ID:        "run_seq:" + string(rune('0'+i)),
			RunID:     "run_seq",
			Type:      "run.started",
			Timestamp: time.Now().UTC(),
		})
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var files []string
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error { //nolint:errcheck
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".jsonl") {
			files = append(files, p)
		}
		return nil
	})
	entries := readEntries(t, files[0])
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
	for i, e := range entries {
		if e.Seq != uint64(i) {
			t.Errorf("entries[%d].seq = %d, want %d", i, e.Seq, i)
		}
	}
}

// TestRecorder_Concurrent verifies the recorder is safe under concurrent writes.
func TestRecorder_Concurrent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec, err := rollout.NewRecorder(rollout.RecorderConfig{
		Dir:   dir,
		RunID: "run_concurrent",
	})
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	const workers = 20
	const eventsPerWorker = 50

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < eventsPerWorker; i++ {
				rec.Record(rollout.RecordableEvent{
					ID:        "run_concurrent:x",
					RunID:     "run_concurrent",
					Type:      "tool.call.started",
					Timestamp: time.Now().UTC(),
					Payload:   map[string]any{"tool": "bash"},
				})
			}
		}()
	}
	wg.Wait()
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var files []string
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error { //nolint:errcheck
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".jsonl") {
			files = append(files, p)
		}
		return nil
	})
	entries := readEntries(t, files[0])
	want := workers * eventsPerWorker
	if len(entries) != want {
		t.Errorf("expected %d entries, got %d", want, len(entries))
	}
}

// TestRecorder_NilPayload verifies entries with nil payload are handled gracefully.
func TestRecorder_NilPayload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec, err := rollout.NewRecorder(rollout.RecorderConfig{
		Dir:   dir,
		RunID: "run_nil",
	})
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	rec.Record(rollout.RecordableEvent{
		ID:        "run_nil:0",
		RunID:     "run_nil",
		Type:      "run.started",
		Timestamp: time.Now().UTC(),
		Payload:   nil,
	})
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var files []string
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error { //nolint:errcheck
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".jsonl") {
			files = append(files, p)
		}
		return nil
	})
	entries := readEntries(t, files[0])
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

// TestRecorder_CloseIdempotent verifies calling Close multiple times is safe.
func TestRecorder_CloseIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec, err := rollout.NewRecorder(rollout.RecorderConfig{
		Dir:   dir,
		RunID: "run_idempotent",
	})
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	rec.Record(rollout.RecordableEvent{
		ID:        "run_idempotent:0",
		RunID:     "run_idempotent",
		Type:      "run.started",
		Timestamp: time.Now().UTC(),
	})
	if err := rec.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should not panic or return error.
	if err := rec.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestNewRecorder_InvalidDir verifies that NewRecorder returns an error when
// the directory cannot be created (e.g. empty dir path).
func TestNewRecorder_EmptyDir(t *testing.T) {
	t.Parallel()

	_, err := rollout.NewRecorder(rollout.RecorderConfig{
		Dir:   "",
		RunID: "run_x",
	})
	if err == nil {
		t.Fatal("expected error for empty dir, got nil")
	}
}

// TestNewRecorder_EmptyRunID verifies that NewRecorder returns an error when
// RunID is empty.
func TestNewRecorder_EmptyRunID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := rollout.NewRecorder(rollout.RecorderConfig{
		Dir:   dir,
		RunID: "",
	})
	if err == nil {
		t.Fatal("expected error for empty RunID, got nil")
	}
}
