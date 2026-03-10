package harness

import (
	"context"
	"time"
)

// Conversation holds metadata for a conversation.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	MsgCount  int       `json:"message_count"`
}

// MessageSearchResult is a single result from a full-text search over messages.
type MessageSearchResult struct {
	ConversationID string `json:"conversation_id"`
	Role           string `json:"role"`
	Snippet        string `json:"snippet"` // short excerpt around the match
}

// ConversationStore persists conversation messages across server restarts.
type ConversationStore interface {
	Migrate(ctx context.Context) error
	Close() error
	SaveConversation(ctx context.Context, convID string, msgs []Message) error
	LoadMessages(ctx context.Context, convID string) ([]Message, error)
	ListConversations(ctx context.Context, limit, offset int) ([]Conversation, error)
	DeleteConversation(ctx context.Context, convID string) error
	// SearchMessages performs a full-text search over message content.
	// Returns up to limit results ordered by relevance. Returns empty slice (not error) for no matches.
	SearchMessages(ctx context.Context, query string, limit int) ([]MessageSearchResult, error)
}
