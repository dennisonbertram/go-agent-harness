package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/forensics/rollout"
)

func TestMainDelegatesToRunWithoutExitOnSuccess(t *testing.T) {
	origRun := runCommand
	origExit := exitFunc
	origArgs := osArgs
	origStdout := stdout
	origStderr := stderr
	defer func() {
		runCommand = origRun
		exitFunc = origExit
		osArgs = origArgs
		stdout = origStdout
		stderr = origStderr
	}()

	runCalled := false
	runCommand = func(args []string) error {
		runCalled = true
		if len(args) != 3 || args[0] != "diff" || args[1] != "a.jsonl" || args[2] != "b.jsonl" {
			t.Fatalf("unexpected args: %v", args)
		}
		return nil
	}

	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	osArgs = []string{"forensics", "diff", "a.jsonl", "b.jsonl"}
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}

	main()

	if !runCalled {
		t.Fatal("expected main to call runCommand")
	}
	if exitCode != -1 {
		t.Fatalf("expected main not to exit on success, got %d", exitCode)
	}
}

func TestMainPrintsSanitizedErrorAndExits(t *testing.T) {
	origRun := runCommand
	origExit := exitFunc
	origArgs := osArgs
	origStdout := stdout
	origStderr := stderr
	defer func() {
		runCommand = origRun
		exitFunc = origExit
		osArgs = origArgs
		stdout = origStdout
		stderr = origStderr
	}()

	runCommand = func(args []string) error {
		return errors.New("bad\nnews")
	}

	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	osArgs = []string{"forensics", "diff"}
	stdout = &bytes.Buffer{}
	var errBuf bytes.Buffer
	stderr = &errBuf

	main()

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if strings.Count(errBuf.String(), "\n") != 1 {
		t.Fatalf("expected sanitized single-line stderr, got %q", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "forensics: badnews") {
		t.Fatalf("expected sanitized error output, got %q", errBuf.String())
	}
}

func TestRunDiffPrintsSummary(t *testing.T) {
	origStdout := stdout
	defer func() {
		stdout = origStdout
	}()

	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "a.jsonl")
	fileB := filepath.Join(tmpDir, "b.jsonl")
	writeRolloutFile(t, fileA, []rawRolloutEntry{
		{Seq: 0, Type: "run.started", Data: map[string]any{"step": 0}},
		{Seq: 1, Type: "usage.delta", Data: map[string]any{"step": 1, "cumulative_cost_usd": 0.1}},
		{Seq: 2, Type: "run.completed", Data: map[string]any{"step": 1}},
	})
	writeRolloutFile(t, fileB, []rawRolloutEntry{
		{Seq: 0, Type: "run.started", Data: map[string]any{"step": 0}},
		{Seq: 1, Type: "usage.delta", Data: map[string]any{"step": 1, "cumulative_cost_usd": 0.2}},
		{Seq: 2, Type: "run.completed", Data: map[string]any{"step": 1}},
	})

	var out bytes.Buffer
	stdout = &out

	if err := run([]string{"diff", fileA, fileB}); err != nil {
		t.Fatalf("run diff: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "Run A: 1 steps, $0.10000") {
		t.Fatalf("expected run A summary, got %q", text)
	}
	if !strings.Contains(text, "Run B: 1 steps, $0.20000") {
		t.Fatalf("expected run B summary, got %q", text)
	}
	if !strings.Contains(text, "Winner:") {
		t.Fatalf("expected winner summary, got %q", text)
	}
}

func TestCountMaxStepAndExtractCost(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "usage.delta", Step: 1, Payload: map[string]any{"cumulative_cost_usd": 0.25}},
		{Type: "tool.call.completed", Step: 3},
	}

	if got := countMaxStep(events); got != 3 {
		t.Fatalf("countMaxStep=%d, want 3", got)
	}
	if got := extractCost(events); got != 0.25 {
		t.Fatalf("extractCost=%f, want 0.25", got)
	}
}

type rawRolloutEntry struct {
	Seq  uint64
	Type string
	Data map[string]any
}

func writeRolloutFile(t *testing.T, path string, entries []rawRolloutEntry) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create rollout file: %v", err)
	}
	defer f.Close()

	baseTime := time.Date(2026, time.March, 18, 0, 0, 0, 0, time.UTC)
	for i, entry := range entries {
		line, err := json.Marshal(map[string]any{
			"ts":   baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano),
			"seq":  entry.Seq,
			"type": entry.Type,
			"data": entry.Data,
		})
		if err != nil {
			t.Fatalf("marshal entry %d: %v", i, err)
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			t.Fatalf("write entry %d: %v", i, err)
		}
	}
}
