package rollout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadReader_ValidJSONL(t *testing.T) {
	input := strings.Join([]string{
		`{"ts":"2026-03-12T10:00:00Z","seq":1,"type":"run.started","data":{"step":0,"run_id":"r1"}}`,
		`{"ts":"2026-03-12T10:00:01Z","seq":2,"type":"tool.call.started","data":{"step":1,"tool":"bash"}}`,
		`{"ts":"2026-03-12T10:00:02Z","seq":3,"type":"run.completed","data":{"step":2,"output":"done"}}`,
	}, "\n")

	events, err := LoadReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Verify first event.
	if events[0].Type != "run.started" {
		t.Errorf("expected type run.started, got %s", events[0].Type)
	}
	if events[0].ID != "1" {
		t.Errorf("expected ID 1, got %s", events[0].ID)
	}
	if events[0].Step != 0 {
		t.Errorf("expected step 0, got %d", events[0].Step)
	}
	expected := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	if !events[0].Timestamp.Equal(expected) {
		t.Errorf("expected timestamp %v, got %v", expected, events[0].Timestamp)
	}

	// Verify step extraction from payload.
	if events[1].Step != 1 {
		t.Errorf("expected step 1, got %d", events[1].Step)
	}
	if events[2].Step != 2 {
		t.Errorf("expected step 2, got %d", events[2].Step)
	}
}

func TestLoadReader_EmptyInput(t *testing.T) {
	events, err := LoadReader(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestLoadReader_BlankLines(t *testing.T) {
	input := "\n" + `{"ts":"2026-03-12T10:00:00Z","seq":1,"type":"run.started","data":{}}` + "\n\n"
	events, err := LoadReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestLoadReader_InvalidJSON(t *testing.T) {
	input := `{"ts":"2026-03-12T10:00:00Z","seq":1,"type":"run.started"` // missing closing brace
	_, err := LoadReader(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Errorf("expected error to mention line 1, got: %v", err)
	}
}

func TestLoadReader_NoStepInPayload(t *testing.T) {
	input := `{"ts":"2026-03-12T10:00:00Z","seq":1,"type":"run.started","data":{"run_id":"r1"}}`
	events, err := LoadReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events[0].Step != 0 {
		t.Errorf("expected step 0 when no step in payload, got %d", events[0].Step)
	}
}

func TestLoadReader_NilData(t *testing.T) {
	input := `{"ts":"2026-03-12T10:00:00Z","seq":1,"type":"run.started"}`
	events, err := LoadReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events[0].Step != 0 {
		t.Errorf("expected step 0 when nil data, got %d", events[0].Step)
	}
	if events[0].Payload != nil {
		t.Errorf("expected nil payload, got %v", events[0].Payload)
	}
}

func TestLoadFile_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	content := `{"ts":"2026-03-12T10:00:00Z","seq":1,"type":"run.started","data":{"step":0}}
{"ts":"2026-03-12T10:00:01Z","seq":2,"type":"run.completed","data":{"step":1}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/path/rollout.jsonl")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadReader_PayloadPreserved(t *testing.T) {
	input := `{"ts":"2026-03-12T10:00:00Z","seq":1,"type":"usage.delta","data":{"step":1,"turn_cost_usd":0.00123,"cumulative_cost_usd":0.00456}}`
	events, err := LoadReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	payload := events[0].Payload
	if payload == nil {
		t.Fatal("expected non-nil payload")
	}
	cost, ok := payload["turn_cost_usd"].(float64)
	if !ok || cost != 0.00123 {
		t.Errorf("expected turn_cost_usd=0.00123, got %v", payload["turn_cost_usd"])
	}
}
