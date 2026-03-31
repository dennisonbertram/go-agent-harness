package speculation_test

import (
	"testing"
	"time"

	"go-agent-harness/internal/speculation"
)

func makeTestOverlay(t *testing.T) *speculation.Overlay {
	t.Helper()
	baseDir := t.TempDir()
	cfg := speculation.DefaultSpeculationConfig()
	overlay, err := speculation.NewOverlay(baseDir, cfg)
	if err != nil {
		t.Fatalf("NewOverlay() error: %v", err)
	}
	t.Cleanup(func() { overlay.Cleanup() }) //nolint:errcheck
	return overlay
}

// TestNewSession_InitialState verifies status=running, counts=0.
func TestNewSession_InitialState(t *testing.T) {
	overlay := makeTestOverlay(t)
	pred := speculation.Prediction{Text: "run the tests now", Confidence: 0.9, Source: "test"}

	session := speculation.NewSession(pred, overlay)

	if session.Status != speculation.StatusRunning {
		t.Errorf("Status: got %q, want %q", session.Status, speculation.StatusRunning)
	}
	if session.TurnCount != 0 {
		t.Errorf("TurnCount: got %d, want 0", session.TurnCount)
	}
	if session.MessageCount != 0 {
		t.Errorf("MessageCount: got %d, want 0", session.MessageCount)
	}
	if session.ID == "" {
		t.Error("ID: got empty string, want non-empty ID")
	}
	if session.StartedAt.IsZero() {
		t.Error("StartedAt: got zero time, want a set timestamp")
	}
}

// TestSession_ShouldStop_UnderLimits verifies false when under both limits.
func TestSession_ShouldStop_UnderLimits(t *testing.T) {
	overlay := makeTestOverlay(t)
	pred := speculation.Prediction{Text: "run the tests now", Confidence: 0.9, Source: "test"}
	session := speculation.NewSession(pred, overlay)

	cfg := speculation.DefaultSpeculationConfig()
	// MaxTurns=20, MaxMessages=100, session has 0 of each
	if session.ShouldStop(cfg) {
		t.Error("ShouldStop(): got true for fresh session under limits, want false")
	}
}

// TestSession_ShouldStop_TurnLimit verifies true when turns exceed max.
func TestSession_ShouldStop_TurnLimit(t *testing.T) {
	overlay := makeTestOverlay(t)
	pred := speculation.Prediction{Text: "run the tests now", Confidence: 0.9, Source: "test"}
	session := speculation.NewSession(pred, overlay)

	cfg := speculation.DefaultSpeculationConfig()
	// Exceed turn limit
	for i := 0; i < cfg.MaxTurns+1; i++ {
		session.IncrementTurn()
	}

	if !session.ShouldStop(cfg) {
		t.Errorf("ShouldStop(): got false with TurnCount=%d exceeding MaxTurns=%d, want true",
			session.TurnCount, cfg.MaxTurns)
	}
}

// TestSession_ShouldStop_MessageLimit verifies true when messages exceed max.
func TestSession_ShouldStop_MessageLimit(t *testing.T) {
	overlay := makeTestOverlay(t)
	pred := speculation.Prediction{Text: "run the tests now", Confidence: 0.9, Source: "test"}
	session := speculation.NewSession(pred, overlay)

	cfg := speculation.DefaultSpeculationConfig()
	// Exceed message limit
	session.IncrementMessages(cfg.MaxMessages + 1)

	if !session.ShouldStop(cfg) {
		t.Errorf("ShouldStop(): got false with MessageCount=%d exceeding MaxMessages=%d, want true",
			session.MessageCount, cfg.MaxMessages)
	}
}

// TestSession_Abort_SetsStatus verifies status becomes aborted.
func TestSession_Abort_SetsStatus(t *testing.T) {
	overlay := makeTestOverlay(t)
	pred := speculation.Prediction{Text: "run the tests now", Confidence: 0.9, Source: "test"}
	session := speculation.NewSession(pred, overlay)

	if err := session.Abort(); err != nil {
		t.Fatalf("Abort() error: %v", err)
	}
	if session.Status != speculation.StatusAborted {
		t.Errorf("Status after Abort(): got %q, want %q", session.Status, speculation.StatusAborted)
	}
}

// TestSession_Accept_SetsStatus verifies status becomes accepted.
func TestSession_Accept_SetsStatus(t *testing.T) {
	overlay := makeTestOverlay(t)
	pred := speculation.Prediction{Text: "run the tests now", Confidence: 0.9, Source: "test"}
	session := speculation.NewSession(pred, overlay)

	session.Accept()
	if session.Status != speculation.StatusAccepted {
		t.Errorf("Status after Accept(): got %q, want %q", session.Status, speculation.StatusAccepted)
	}
}

// TestSession_IncrementTurn verifies turn count increases.
func TestSession_IncrementTurn(t *testing.T) {
	overlay := makeTestOverlay(t)
	pred := speculation.Prediction{Text: "run the tests now", Confidence: 0.9, Source: "test"}
	session := speculation.NewSession(pred, overlay)

	session.IncrementTurn()
	session.IncrementTurn()
	session.IncrementTurn()

	if session.TurnCount != 3 {
		t.Errorf("TurnCount after 3 increments: got %d, want 3", session.TurnCount)
	}
}

// TestSession_IncrementMessages verifies message count increases.
func TestSession_IncrementMessages(t *testing.T) {
	overlay := makeTestOverlay(t)
	pred := speculation.Prediction{Text: "run the tests now", Confidence: 0.9, Source: "test"}
	session := speculation.NewSession(pred, overlay)

	session.IncrementMessages(5)
	session.IncrementMessages(3)

	if session.MessageCount != 8 {
		t.Errorf("MessageCount after IncrementMessages(5)+IncrementMessages(3): got %d, want 8",
			session.MessageCount)
	}
}

// TestSession_CompletedStatus verifies StatusCompleted is a valid non-running state.
func TestSession_CompletedStatus(t *testing.T) {
	// Ensure StatusCompleted constant exists and has a distinct value
	if speculation.StatusCompleted == speculation.StatusRunning {
		t.Error("StatusCompleted must differ from StatusRunning")
	}
	if speculation.StatusCompleted == speculation.StatusAborted {
		t.Error("StatusCompleted must differ from StatusAborted")
	}
	if speculation.StatusCompleted == speculation.StatusAccepted {
		t.Error("StatusCompleted must differ from StatusAccepted")
	}
}

// TestSession_StartedAt_IsSetOnCreation verifies StartedAt is within a second of now.
func TestSession_StartedAt_IsSetOnCreation(t *testing.T) {
	before := time.Now()
	overlay := makeTestOverlay(t)
	pred := speculation.Prediction{Text: "run the tests", Confidence: 0.9, Source: "test"}
	session := speculation.NewSession(pred, overlay)
	after := time.Now()

	if session.StartedAt.Before(before) || session.StartedAt.After(after) {
		t.Errorf("StartedAt %v is outside the expected range [%v, %v]",
			session.StartedAt, before, after)
	}
}
