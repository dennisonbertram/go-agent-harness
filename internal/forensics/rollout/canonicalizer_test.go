package rollout

import (
	"testing"
	"time"
)

func TestCanonicalize_DefaultOptions(t *testing.T) {
	ts1 := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 3, 12, 10, 0, 1, 0, time.UTC)

	events := []RolloutEvent{
		{
			ID:        "1",
			Type:      "run.started",
			Step:      0,
			Timestamp: ts1,
			Payload:   map[string]any{"run_id": "r1", "step": float64(0)},
		},
		{
			ID:        "2",
			Type:      "tool.call.started",
			Step:      1,
			Timestamp: ts2,
			Payload:   map[string]any{"run_id": "r1", "step": float64(1), "tool": "bash"},
		},
	}

	result := Canonicalize(events, DefaultOptions)

	if len(result) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result))
	}

	// IDs should be stripped.
	for i, ev := range result {
		if ev.ID != "" {
			t.Errorf("event %d: expected empty ID, got %q", i, ev.ID)
		}
	}

	// Timestamps should be zero.
	for i, ev := range result {
		if !ev.Timestamp.IsZero() {
			t.Errorf("event %d: expected zero timestamp, got %v", i, ev.Timestamp)
		}
	}

	// run_id should be stripped from payload.
	for i, ev := range result {
		if _, ok := ev.Payload["run_id"]; ok {
			t.Errorf("event %d: expected run_id stripped from payload", i)
		}
	}

	// Other payload fields should be preserved.
	if result[1].Payload["tool"] != "bash" {
		t.Errorf("expected tool=bash preserved, got %v", result[1].Payload["tool"])
	}
}

func TestCanonicalize_NoStrip(t *testing.T) {
	ts := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	events := []RolloutEvent{
		{
			ID:        "1",
			Type:      "run.started",
			Step:      0,
			Timestamp: ts,
			Payload:   map[string]any{"run_id": "r1"},
		},
	}

	opts := CanonicalizationOptions{}
	result := Canonicalize(events, opts)

	if result[0].ID != "1" {
		t.Errorf("expected ID preserved, got %q", result[0].ID)
	}
	if !result[0].Timestamp.Equal(ts) {
		t.Errorf("expected timestamp preserved, got %v", result[0].Timestamp)
	}
	if result[0].Payload["run_id"] != "r1" {
		t.Errorf("expected run_id preserved, got %v", result[0].Payload["run_id"])
	}
}

func TestCanonicalize_SortsByStep(t *testing.T) {
	events := []RolloutEvent{
		{Type: "tool.call.completed", Step: 2},
		{Type: "run.started", Step: 0},
		{Type: "tool.call.started", Step: 1},
	}

	result := Canonicalize(events, DefaultOptions)

	expectedSteps := []int{0, 1, 2}
	for i, ev := range result {
		if ev.Step != expectedSteps[i] {
			t.Errorf("event %d: expected step %d, got %d", i, expectedSteps[i], ev.Step)
		}
	}
}

func TestCanonicalize_StableSortWithinStep(t *testing.T) {
	events := []RolloutEvent{
		{Type: "llm.turn.requested", Step: 1},
		{Type: "llm.turn.completed", Step: 1},
		{Type: "tool.call.started", Step: 1},
	}

	result := Canonicalize(events, DefaultOptions)

	// Order within step 1 should be preserved.
	expectedTypes := []string{"llm.turn.requested", "llm.turn.completed", "tool.call.started"}
	for i, ev := range result {
		if ev.Type != expectedTypes[i] {
			t.Errorf("event %d: expected type %s, got %s", i, expectedTypes[i], ev.Type)
		}
	}
}

func TestCanonicalize_NilPayload(t *testing.T) {
	events := []RolloutEvent{
		{Type: "run.started", Step: 0, Payload: nil},
	}

	result := Canonicalize(events, DefaultOptions)
	if result[0].Payload != nil {
		t.Errorf("expected nil payload preserved, got %v", result[0].Payload)
	}
}

func TestCanonicalize_DoesNotMutateOriginal(t *testing.T) {
	original := []RolloutEvent{
		{
			ID:        "1",
			Type:      "run.started",
			Timestamp: time.Now(),
			Payload:   map[string]any{"run_id": "r1", "tool": "bash"},
		},
	}

	_ = Canonicalize(original, DefaultOptions)

	// Original should be unchanged.
	if original[0].ID != "1" {
		t.Error("original ID was mutated")
	}
	if _, ok := original[0].Payload["run_id"]; !ok {
		t.Error("original payload was mutated")
	}
}
