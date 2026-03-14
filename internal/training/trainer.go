package training

import "context"

// MockTrainer is a test double for the Trainer interface.
type MockTrainer struct {
	Report      *TrainerReport
	Err         error
	BatchReport *BatchReport
	BatchErr    error
}

// Analyze returns the pre-configured report or error.
func (m *MockTrainer) Analyze(_ context.Context, _ TraceBundle) (*TrainerReport, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Report, nil
}

// AnalyzeBatch returns the pre-configured batch report or error.
func (m *MockTrainer) AnalyzeBatch(_ context.Context, _ []TraceBundle) (*BatchReport, error) {
	if m.BatchErr != nil {
		return nil, m.BatchErr
	}
	return m.BatchReport, nil
}
