package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_NoArgs(t *testing.T) {
	err := run(nil)
	if err == nil {
		t.Fatal("expected error for no args")
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := run([]string{"invalid"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestRunDiff_MissingArgs(t *testing.T) {
	err := runDiff(nil)
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestRunDiff_SingleArg(t *testing.T) {
	err := runDiff([]string{"only_one"})
	if err == nil {
		t.Fatal("expected error for single arg")
	}
}

func TestRunDiff_NonexistentFileA(t *testing.T) {
	err := runDiff([]string{"/nonexistent/a.jsonl", "/nonexistent/b.jsonl"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestRunDiff_ValidFiles(t *testing.T) {
	dir := t.TempDir()

	fileA := filepath.Join(dir, "a.jsonl")
	fileB := filepath.Join(dir, "b.jsonl")

	contentA := `{"ts":"2026-03-12T10:00:00Z","seq":1,"type":"run.started","data":{"step":0}}
{"ts":"2026-03-12T10:00:01Z","seq":2,"type":"usage.delta","data":{"step":1,"cumulative_cost_usd":0.00123}}
{"ts":"2026-03-12T10:00:02Z","seq":3,"type":"run.completed","data":{"step":2}}`

	contentB := `{"ts":"2026-03-12T11:00:00Z","seq":1,"type":"run.started","data":{"step":0}}
{"ts":"2026-03-12T11:00:01Z","seq":2,"type":"usage.delta","data":{"step":1,"cumulative_cost_usd":0.00198}}
{"ts":"2026-03-12T11:00:02Z","seq":3,"type":"tool.call.started","data":{"step":2,"tool":"bash"}}
{"ts":"2026-03-12T11:00:03Z","seq":4,"type":"tool.call.completed","data":{"step":3}}
{"ts":"2026-03-12T11:00:04Z","seq":5,"type":"run.completed","data":{"step":4}}`

	if err := os.WriteFile(fileA, []byte(contentA), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte(contentB), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runDiff([]string{fileA, fileB})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDiff_BFailed(t *testing.T) {
	dir := t.TempDir()

	fileA := filepath.Join(dir, "a.jsonl")
	fileB := filepath.Join(dir, "b.jsonl")

	contentA := `{"ts":"2026-03-12T10:00:00Z","seq":1,"type":"run.started","data":{"step":0}}
{"ts":"2026-03-12T10:00:01Z","seq":2,"type":"run.completed","data":{"step":1}}`

	contentB := `{"ts":"2026-03-12T11:00:00Z","seq":1,"type":"run.started","data":{"step":0}}
{"ts":"2026-03-12T11:00:01Z","seq":2,"type":"run.failed","data":{"step":1,"error":"timeout"}}`

	if err := os.WriteFile(fileA, []byte(contentA), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte(contentB), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runDiff([]string{fileA, fileB})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountMaxStep(t *testing.T) {
	tests := []struct {
		name     string
		events   []event
		expected int
	}{
		{"empty", nil, 0},
	}
	_ = tests
	// Already covered by differ package tests.
}

func TestExtractCost(t *testing.T) {
	// Already covered by differ package tests.
}

// event is a type alias for test convenience — not used in real code.
type event = struct {
	Type string
	Step int
}
