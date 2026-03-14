package training

import (
	"context"
	"testing"
)

func TestMockTrainer_Analyze(t *testing.T) {
	report := &TrainerReport{RunID: "run_1"}
	report.Scores.ToolQuality = 0.9
	report.Scores.Efficiency = 0.8

	m := &MockTrainer{Report: report}
	got, err := m.Analyze(context.Background(), TraceBundle{RunID: "run_1"})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got.RunID != "run_1" {
		t.Errorf("RunID = %q, want run_1", got.RunID)
	}
	if got.Scores.ToolQuality != 0.9 {
		t.Errorf("ToolQuality = %f, want 0.9", got.Scores.ToolQuality)
	}
}

func TestMockTrainer_AnalyzeError(t *testing.T) {
	m := &MockTrainer{Err: context.Canceled}
	_, err := m.Analyze(context.Background(), TraceBundle{})
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestMockTrainer_AnalyzeBatch(t *testing.T) {
	br := &BatchReport{
		BatchID: "batch_1",
		RunIDs:  []string{"run_1", "run_2"},
	}
	m := &MockTrainer{BatchReport: br}
	got, err := m.AnalyzeBatch(context.Background(), []TraceBundle{
		{RunID: "run_1"},
		{RunID: "run_2"},
	})
	if err != nil {
		t.Fatalf("AnalyzeBatch: %v", err)
	}
	if got.BatchID != "batch_1" {
		t.Errorf("BatchID = %q, want batch_1", got.BatchID)
	}
	if len(got.RunIDs) != 2 {
		t.Errorf("RunIDs len = %d, want 2", len(got.RunIDs))
	}
}

func TestMockTrainer_AnalyzeBatchError(t *testing.T) {
	m := &MockTrainer{BatchErr: context.DeadlineExceeded}
	_, err := m.AnalyzeBatch(context.Background(), nil)
	if err != context.DeadlineExceeded {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestMockTrainer_ImplementsInterface(t *testing.T) {
	var _ Trainer = (*MockTrainer)(nil)
}
