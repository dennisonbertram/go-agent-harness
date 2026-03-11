package symphd

import (
	"sort"
	"sync"
	"time"
)

// RetryPolicy defines the retry behavior for failed runs.
type RetryPolicy struct {
	MaxAttempts int // default: 5
	BaseDelayMs int // base delay in ms: 10000 (10s)
	MaxDelayMs  int // max delay cap: 300000 (5min)
}

// DefaultRetryPolicy returns sensible defaults.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 5,
		BaseDelayMs: 10000,
		MaxDelayMs:  300000,
	}
}

// ShouldRetry returns true if the issue should be retried given its attempt count.
func (p RetryPolicy) ShouldRetry(attempts int) bool {
	return attempts < p.MaxAttempts
}

// BackoffDelay returns the delay before the next retry attempt.
// attempt is 1-indexed (first retry = attempt 1).
// Formula: min(BaseDelayMs * 2^(attempt-1), MaxDelayMs)
func (p RetryPolicy) BackoffDelay(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	delayMs := p.BaseDelayMs
	for i := 1; i < attempt; i++ {
		delayMs *= 2
		if delayMs > p.MaxDelayMs {
			delayMs = p.MaxDelayMs
			break
		}
	}
	if delayMs > p.MaxDelayMs {
		delayMs = p.MaxDelayMs
	}
	return time.Duration(delayMs) * time.Millisecond
}

// DeadLetter represents an issue that has exhausted all retries.
type DeadLetter struct {
	IssueNumber int
	Title       string
	Attempts    int
	LastError   string
	ExhaustedAt time.Time
}

// DeadLetterQueue holds issues that have exhausted retries.
type DeadLetterQueue struct {
	mu    sync.RWMutex
	items []*DeadLetter
}

// NewDeadLetterQueue creates a new empty DeadLetterQueue.
func NewDeadLetterQueue() *DeadLetterQueue {
	return &DeadLetterQueue{}
}

// Add appends a dead letter entry for the given issue.
func (q *DeadLetterQueue) Add(issue *TrackedIssue, lastErr string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, &DeadLetter{
		IssueNumber: issue.Number,
		Title:       issue.Title,
		Attempts:    issue.Attempts,
		LastError:   lastErr,
		ExhaustedAt: time.Now(),
	})
}

// Items returns a copy of all dead letter entries, sorted by ExhaustedAt ascending.
func (q *DeadLetterQueue) Items() []*DeadLetter {
	q.mu.RLock()
	defer q.mu.RUnlock()

	out := make([]*DeadLetter, len(q.items))
	for i, item := range q.items {
		copy := *item
		out[i] = &copy
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ExhaustedAt.Before(out[j].ExhaustedAt)
	})
	return out
}

// Len returns the number of dead letter entries.
func (q *DeadLetterQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.items)
}
