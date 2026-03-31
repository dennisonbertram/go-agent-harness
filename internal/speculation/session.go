package speculation

import (
	"time"

	"github.com/google/uuid"
)

// SessionStatus represents the lifecycle state of a speculation session.
type SessionStatus string

const (
	// StatusRunning means the speculation is actively executing.
	StatusRunning SessionStatus = "running"

	// StatusCompleted means the speculation finished naturally (hit turn/message limit
	// or reached the end of the predicted conversation).
	StatusCompleted SessionStatus = "completed"

	// StatusAborted means the speculation was stopped early (e.g., a write was detected).
	StatusAborted SessionStatus = "aborted"

	// StatusAccepted means the user's actual input matched the prediction and the
	// speculative result was promoted to the real conversation.
	StatusAccepted SessionStatus = "accepted"
)

// Session represents an active (or concluded) speculation session.
type Session struct {
	// ID is the unique identifier for this session.
	ID string

	// Prediction is the predicted user input that triggered this session.
	Prediction Prediction

	// Overlay is the isolated directory used for speculative writes.
	Overlay *Overlay

	// TurnCount is the number of assistant turns executed so far.
	TurnCount int

	// MessageCount is the total number of messages (user + assistant) so far.
	MessageCount int

	// Status is the current lifecycle state.
	Status SessionStatus

	// StartedAt is when this session was created.
	StartedAt time.Time
}

// NewSession creates a new speculation session in StatusRunning state.
func NewSession(prediction Prediction, overlay *Overlay) *Session {
	return &Session{
		ID:           uuid.New().String(),
		Prediction:   prediction,
		Overlay:      overlay,
		TurnCount:    0,
		MessageCount: 0,
		Status:       StatusRunning,
		StartedAt:    time.Now(),
	}
}

// ShouldStop checks if the session has exceeded its configured limits.
// Returns true if TurnCount > MaxTurns or MessageCount > MaxMessages.
func (s *Session) ShouldStop(cfg SpeculationConfig) bool {
	return s.TurnCount > cfg.MaxTurns || s.MessageCount > cfg.MaxMessages
}

// Abort marks the session as aborted and triggers overlay cleanup.
func (s *Session) Abort() error {
	s.Status = StatusAborted
	if s.Overlay != nil {
		return s.Overlay.Cleanup()
	}
	return nil
}

// Accept marks the session as accepted — the user's actual input matched
// the prediction and the speculative execution result is being promoted.
func (s *Session) Accept() {
	s.Status = StatusAccepted
}

// IncrementTurn records one additional assistant turn.
func (s *Session) IncrementTurn() {
	s.TurnCount++
}

// IncrementMessages records count additional messages.
func (s *Session) IncrementMessages(count int) {
	s.MessageCount += count
}
