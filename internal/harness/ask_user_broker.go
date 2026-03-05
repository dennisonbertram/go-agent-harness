package harness

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

var (
	ErrNoPendingUserQuestion    = errors.New("no pending user question")
	ErrInvalidUserQuestionInput = errors.New("invalid user question input")
)

type askUserSubmission struct {
	answers    map[string]string
	answeredAt time.Time
}

type pendingUserQuestion struct {
	pending htools.AskUserQuestionPending
	answerC chan askUserSubmission
}

type InMemoryAskUserQuestionBroker struct {
	mu      sync.Mutex
	pending map[string]*pendingUserQuestion
	now     func() time.Time
}

func NewInMemoryAskUserQuestionBroker(now func() time.Time) *InMemoryAskUserQuestionBroker {
	if now == nil {
		now = time.Now
	}
	return &InMemoryAskUserQuestionBroker{
		pending: make(map[string]*pendingUserQuestion),
		now:     now,
	}
}

func (b *InMemoryAskUserQuestionBroker) Ask(ctx context.Context, req htools.AskUserQuestionRequest) (map[string]string, time.Time, error) {
	if err := htools.ValidateAskUserQuestions(req.Questions); err != nil {
		return nil, time.Time{}, err
	}
	if req.RunID == "" {
		return nil, time.Time{}, fmt.Errorf("run id is required")
	}
	if req.Timeout <= 0 {
		req.Timeout = 5 * time.Minute
	}

	start := b.now().UTC()
	entry := &pendingUserQuestion{
		pending: htools.AskUserQuestionPending{
			RunID:      req.RunID,
			CallID:     req.CallID,
			Tool:       htools.AskUserQuestionToolName,
			Questions:  req.Questions,
			DeadlineAt: start.Add(req.Timeout),
		},
		answerC: make(chan askUserSubmission, 1),
	}

	b.mu.Lock()
	if _, exists := b.pending[req.RunID]; exists {
		b.mu.Unlock()
		return nil, time.Time{}, fmt.Errorf("pending user question already exists for run %q", req.RunID)
	}
	b.pending[req.RunID] = entry
	b.mu.Unlock()

	timer := time.NewTimer(req.Timeout)
	defer timer.Stop()

	select {
	case submission := <-entry.answerC:
		return submission.answers, submission.answeredAt, nil
	case <-timer.C:
		b.clearPendingIfMatch(req.RunID, entry)
		return nil, time.Time{}, &htools.AskUserQuestionTimeoutError{
			RunID:      req.RunID,
			CallID:     req.CallID,
			DeadlineAt: entry.pending.DeadlineAt,
		}
	case <-ctx.Done():
		b.clearPendingIfMatch(req.RunID, entry)
		return nil, time.Time{}, ctx.Err()
	}
}

func (b *InMemoryAskUserQuestionBroker) Pending(runID string) (htools.AskUserQuestionPending, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry, ok := b.pending[runID]
	if !ok {
		return htools.AskUserQuestionPending{}, false
	}
	return entry.pending, true
}

func (b *InMemoryAskUserQuestionBroker) Submit(runID string, answers map[string]string) error {
	b.mu.Lock()
	entry, ok := b.pending[runID]
	if !ok {
		b.mu.Unlock()
		return ErrNoPendingUserQuestion
	}

	normalized, err := htools.NormalizeAskUserAnswers(entry.pending.Questions, answers)
	if err != nil {
		b.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrInvalidUserQuestionInput, err)
	}
	delete(b.pending, runID)
	answeredAt := b.now().UTC()
	b.mu.Unlock()

	entry.answerC <- askUserSubmission{answers: normalized, answeredAt: answeredAt}
	return nil
}

func (b *InMemoryAskUserQuestionBroker) clearPendingIfMatch(runID string, entry *pendingUserQuestion) {
	b.mu.Lock()
	defer b.mu.Unlock()
	current, ok := b.pending[runID]
	if !ok {
		return
	}
	if current == entry {
		delete(b.pending, runID)
	}
}
