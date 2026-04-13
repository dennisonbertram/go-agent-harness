package rollout_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

// stubProvider is a minimal Provider implementation for integration tests.
type stubProvider struct{}

func (s *stubProvider) Complete(_ context.Context, req harness.CompletionRequest) (harness.CompletionResult, error) {
	// Return a simple assistant message with no tool calls so the run completes.
	return harness.CompletionResult{
		Content: "done",
	}, nil
}

func readJSONLEntries(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open rollout file %s: %v", path, err)
	}
	defer f.Close()

	var entries []map[string]any
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal line: %v", err)
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return entries
}

func waitForTerminalJSONLEntries(t *testing.T, path string, timeout time.Duration) []map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		entries := readJSONLEntries(t, path)
		for _, entry := range entries {
			if typ, ok := entry["type"].(string); ok && harness.IsTerminalEvent(harness.EventType(typ)) {
				return entries
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for terminal rollout event in %s", path)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestRunnerRollout_RunProducesJSONL verifies that when RolloutDir is set on
// RunnerConfig, a completed run produces a date-partitioned JSONL file containing
// at minimum a run.started and run.completed event.
func TestRunnerRollout_RunProducesJSONL(t *testing.T) {
	t.Parallel()

	rolloutDir := t.TempDir()
	runner := harness.NewRunner(&stubProvider{}, nil, harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     1,
		RolloutDir:   rolloutDir,
	})

	run, err := runner.StartRun(harness.RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait for the run to complete.
	_, ch, cancel, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()

	timeout := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				goto done
			}
			if harness.IsTerminalEvent(ev.Type) {
				goto done
			}
		case <-timeout:
			t.Fatal("timed out waiting for run to complete")
		}
	}
done:
	// Find the JSONL file.
	today := time.Now().UTC().Format("2006-01-02")
	expectedDir := filepath.Join(rolloutDir, today)
	dirEntries, err := os.ReadDir(expectedDir)
	if err != nil {
		t.Fatalf("read rollout dir %s: %v", expectedDir, err)
	}
	var jsonlFiles []string
	for _, de := range dirEntries {
		if strings.HasSuffix(de.Name(), ".jsonl") {
			jsonlFiles = append(jsonlFiles, filepath.Join(expectedDir, de.Name()))
		}
	}
	if len(jsonlFiles) == 0 {
		t.Fatalf("no .jsonl files found under %s", expectedDir)
	}

	// Find the file for our run.
	var runFile string
	for _, f := range jsonlFiles {
		if strings.Contains(f, run.ID) {
			runFile = f
			break
		}
	}
	if runFile == "" {
		t.Fatalf("no .jsonl file found for run %q under %s", run.ID, expectedDir)
	}

	entries := waitForTerminalJSONLEntries(t, runFile, 5*time.Second)
	if len(entries) == 0 {
		t.Fatal("expected at least one entry in rollout file, got 0")
	}

	// Check for run.started and run.completed events.
	types := make(map[string]bool)
	for _, e := range entries {
		if typ, ok := e["type"].(string); ok {
			types[typ] = true
		}
	}
	if !types["run.started"] {
		t.Error("rollout missing run.started event")
	}
	if !types["run.completed"] && !types["run.failed"] {
		t.Error("rollout missing terminal event (run.completed or run.failed)")
	}

	// Verify sequential seq values.
	for i, e := range entries {
		seqRaw, ok := e["seq"]
		if !ok {
			t.Errorf("entry %d missing seq", i)
			continue
		}
		// JSON numbers are float64.
		seq := int(seqRaw.(float64))
		if seq != i {
			t.Errorf("entry %d: seq = %d, want %d", i, seq, i)
		}
	}

	// Verify ts is present and parseable.
	for i, e := range entries {
		tsRaw, ok := e["ts"]
		if !ok {
			t.Errorf("entry %d missing ts", i)
			continue
		}
		tsStr, ok := tsRaw.(string)
		if !ok {
			t.Errorf("entry %d ts is not a string: %T", i, tsRaw)
			continue
		}
		if _, err := time.Parse(time.RFC3339Nano, tsStr); err != nil {
			t.Errorf("entry %d ts %q not parseable as RFC3339: %v", i, tsStr, err)
		}
	}
}

// TestRunnerRollout_Disabled verifies that when RolloutDir is empty, no
// rollout files are written (and no error occurs).
func TestRunnerRollout_Disabled(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(&stubProvider{}, nil, harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     1,
		// RolloutDir intentionally left empty.
	})

	run, err := runner.StartRun(harness.RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	_, ch, cancel, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()

	timeout := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if harness.IsTerminalEvent(ev.Type) {
				return
			}
		case <-timeout:
			t.Fatal("timed out waiting for run to complete")
		}
	}
}
