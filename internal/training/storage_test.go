package training

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStore_NewAndClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestStore_SaveAndGetTrace(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	bundle := TraceBundle{
		RunID:           "run_1",
		TaskID:          "task_1",
		Outcome:         "pass",
		Steps:           5,
		CostUSD:         0.10,
		EfficiencyScore: 0.85,
		FirstTryRate:    0.90,
		TokenCount:      5000,
	}
	score := ScoreResult{
		RunID:            "run_1",
		ToolQuality:      0.8,
		Efficiency:       0.7,
		FirstTryRate:     0.9,
		AntiPatternCount: 1,
		MaxContextRatio:  0.5,
	}

	if err := store.SaveTrace(bundle, score); err != nil {
		t.Fatalf("SaveTrace: %v", err)
	}

	got, err := store.GetTrace("run_1")
	if err != nil {
		t.Fatalf("GetTrace: %v", err)
	}
	if got.RunID != "run_1" {
		t.Errorf("RunID = %q, want run_1", got.RunID)
	}
	if got.TaskID != "task_1" {
		t.Errorf("TaskID = %q, want task_1", got.TaskID)
	}
	if got.Outcome != "pass" {
		t.Errorf("Outcome = %q, want pass", got.Outcome)
	}
	if got.Steps != 5 {
		t.Errorf("Steps = %d, want 5", got.Steps)
	}
	if got.CostUSD != 0.10 {
		t.Errorf("CostUSD = %f, want 0.10", got.CostUSD)
	}
}

func TestStore_GetTraceNotFound(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	_, err = store.GetTrace("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent trace")
	}
}

func TestStore_SaveAndGetFindings(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	findings := []Finding{
		{Type: "behavior", Priority: "high", Target: "bash tool", Issue: "retry loop", Proposed: "add backoff", Rationale: "seen 5 times", Confidence: ConfidenceCertain, EvidenceCount: 5},
		{Type: "system_prompt", Priority: "medium", Target: "system prompt", Issue: "too long", Proposed: "trim", Rationale: "wastes tokens", Confidence: ConfidenceProbable, EvidenceCount: 3},
	}

	if err := store.SaveFindings("run_1", findings); err != nil {
		t.Fatalf("SaveFindings: %v", err)
	}

	pending, err := store.GetPendingFindings()
	if err != nil {
		t.Fatalf("GetPendingFindings: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("pending len = %d, want 2", len(pending))
	}
	if pending[0].Type != "behavior" {
		t.Errorf("pending[0].Type = %q, want behavior", pending[0].Type)
	}
}

func TestStore_SaveAppliedChange(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if err := store.SaveAppliedChange("abc123", 1, "fixed retry loop"); err != nil {
		t.Fatalf("SaveAppliedChange: %v", err)
	}
}

func TestStore_SaveTraceDuplicate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	bundle := TraceBundle{RunID: "run_dup", Outcome: "pass", Steps: 1}
	score := ScoreResult{RunID: "run_dup"}

	if err := store.SaveTrace(bundle, score); err != nil {
		t.Fatalf("first SaveTrace: %v", err)
	}
	// Second save with same run_id should upsert or error gracefully
	err = store.SaveTrace(bundle, score)
	// We accept either no error (upsert) or an error — just don't panic
	_ = err
}

func TestStore_CountTraces(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	n, err := store.CountTraces()
	if err != nil {
		t.Fatalf("CountTraces: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}

	bundle := TraceBundle{RunID: "ct_1", Outcome: "pass", Steps: 1}
	score := ScoreResult{RunID: "ct_1"}
	if err := store.SaveTrace(bundle, score); err != nil {
		t.Fatalf("SaveTrace: %v", err)
	}

	n, err = store.CountTraces()
	if err != nil {
		t.Fatalf("CountTraces after insert: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestStore_CountFindings(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	n, err := store.CountFindings()
	if err != nil {
		t.Fatalf("CountFindings: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}

	if err := store.SaveFindings("run_cf", []Finding{{Type: "behavior", Priority: "high"}}); err != nil {
		t.Fatalf("SaveFindings: %v", err)
	}

	n, err = store.CountFindings()
	if err != nil {
		t.Fatalf("CountFindings after insert: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestStore_CountAppliedChanges(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	n, err := store.CountAppliedChanges()
	if err != nil {
		t.Fatalf("CountAppliedChanges: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}

	if err := store.SaveAppliedChange("abc123", 1, "test"); err != nil {
		t.Fatalf("SaveAppliedChange: %v", err)
	}

	n, err = store.CountAppliedChanges()
	if err != nil {
		t.Fatalf("CountAppliedChanges after insert: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestStore_QueryHistory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// Empty history.
	changes, err := store.QueryHistory(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("QueryHistory: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}

	// Add a change.
	if err := store.SaveAppliedChange("def456", 1, "updated prompt"); err != nil {
		t.Fatalf("SaveAppliedChange: %v", err)
	}

	// Query with old date should find it.
	changes, err = store.QueryHistory(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("QueryHistory: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].GitCommit != "def456" {
		t.Errorf("GitCommit = %q, want def456", changes[0].GitCommit)
	}
	if changes[0].Description != "updated prompt" {
		t.Errorf("Description = %q, want 'updated prompt'", changes[0].Description)
	}

	// Query with future date should find nothing.
	changes, err = store.QueryHistory(time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("QueryHistory future: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("expected 0 changes for future date, got %d", len(changes))
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	done := make(chan error, 10)
	for i := range 10 {
		go func(i int) {
			bundle := TraceBundle{
				RunID:   "run_" + string(rune('a'+i)),
				Outcome: "pass",
				Steps:   i + 1,
			}
			score := ScoreResult{RunID: bundle.RunID}
			done <- store.SaveTrace(bundle, score)
		}(i)
	}
	for range 10 {
		if err := <-done; err != nil {
			t.Errorf("concurrent SaveTrace: %v", err)
		}
	}
}
