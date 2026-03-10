package harness

import (
	"context"
	"log"
	"time"
)

// ConversationCleaner sweeps for and deletes old conversations.
// It respects the pinned flag — pinned conversations are never deleted.
// A retentionDays of 0 disables cleanup entirely.
type ConversationCleaner struct {
	store         ConversationStore
	retentionDays int
}

// NewConversationCleaner creates a cleaner that deletes conversations older
// than retentionDays. A value of 0 disables cleanup.
func NewConversationCleaner(store ConversationStore, retentionDays int) *ConversationCleaner {
	return &ConversationCleaner{
		store:         store,
		retentionDays: retentionDays,
	}
}

// RunOnce performs a single sweep, deleting non-pinned conversations older
// than the configured retention period. Returns the number of conversations
// deleted, or 0 and nil if retention is disabled (retentionDays == 0).
func (c *ConversationCleaner) RunOnce(ctx context.Context) (int, error) {
	if c.retentionDays <= 0 {
		return 0, nil
	}
	threshold := time.Now().UTC().Add(-time.Duration(c.retentionDays) * 24 * time.Hour)
	return c.store.DeleteOldConversations(ctx, threshold)
}

// Start runs a background goroutine that periodically sweeps for old
// conversations. The sweep happens once at startup and then every interval.
// The goroutine exits when ctx is cancelled.
func (c *ConversationCleaner) Start(ctx context.Context, interval time.Duration) {
	if c.retentionDays <= 0 {
		return
	}
	go func() {
		// Startup sweep
		n, err := c.RunOnce(ctx)
		if err != nil {
			log.Printf("conversation cleaner startup sweep error: %v", err)
		} else if n > 0 {
			log.Printf("conversation cleaner: deleted %d old conversation(s) (retention=%d days)", n, c.retentionDays)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := c.RunOnce(ctx)
				if err != nil {
					log.Printf("conversation cleaner sweep error: %v", err)
				} else if n > 0 {
					log.Printf("conversation cleaner: deleted %d old conversation(s) (retention=%d days)", n, c.retentionDays)
				}
			}
		}
	}()
}
