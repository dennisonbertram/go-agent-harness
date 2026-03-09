package harness

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const conversationSchema = `
CREATE TABLE IF NOT EXISTS conversations (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL DEFAULT '',
    msg_count  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS conversation_messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    step            INTEGER NOT NULL,
    role            TEXT NOT NULL,
    content         TEXT NOT NULL DEFAULT '',
    tool_calls_json TEXT,
    tool_call_id    TEXT NOT NULL DEFAULT '',
    name            TEXT NOT NULL DEFAULT '',
    UNIQUE(conversation_id, step)
);

CREATE INDEX IF NOT EXISTS idx_conv_msgs_conv_id ON conversation_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversations_updated ON conversations(updated_at);
`

// SQLiteConversationStore implements ConversationStore using SQLite.
type SQLiteConversationStore struct {
	db *sql.DB
}

// NewSQLiteConversationStore creates a new SQLite-backed conversation store.
func NewSQLiteConversationStore(path string) (*SQLiteConversationStore, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}
	dsn := path + "?_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Limit to 1 connection to avoid SQLITE_BUSY under concurrent writes.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set sqlite WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set sqlite foreign keys: %w", err)
	}
	return &SQLiteConversationStore{db: db}, nil
}

// Close closes the database connection.
func (s *SQLiteConversationStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Migrate creates the schema tables.
func (s *SQLiteConversationStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, conversationSchema)
	if err != nil {
		return fmt.Errorf("sqlite conversation migrate: %w", err)
	}
	return nil
}

// SaveConversation persists a conversation's messages, replacing any existing messages.
func (s *SQLiteConversationStore) SaveConversation(ctx context.Context, convID string, msgs []Message) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)

	// Upsert conversations row
	_, err = tx.ExecContext(ctx, `
INSERT INTO conversations (id, title, msg_count, created_at, updated_at)
VALUES (?, '', ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    msg_count = excluded.msg_count,
    updated_at = excluded.updated_at
`, convID, len(msgs), now, now)
	if err != nil {
		return fmt.Errorf("upsert conversation: %w", err)
	}

	// Delete old messages
	if _, err := tx.ExecContext(ctx, `DELETE FROM conversation_messages WHERE conversation_id = ?`, convID); err != nil {
		return fmt.Errorf("delete old messages: %w", err)
	}

	// Insert new messages
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO conversation_messages (conversation_id, step, role, content, tool_calls_json, tool_call_id, name)
VALUES (?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for i, msg := range msgs {
		var toolCallsJSON *string
		if len(msg.ToolCalls) > 0 {
			data, err := json.Marshal(msg.ToolCalls)
			if err != nil {
				return fmt.Errorf("marshal tool calls at step %d: %w", i, err)
			}
			s := string(data)
			toolCallsJSON = &s
		}

		if _, err := stmt.ExecContext(ctx, convID, i, msg.Role, msg.Content, toolCallsJSON, msg.ToolCallID, msg.Name); err != nil {
			return fmt.Errorf("insert message at step %d: %w", i, err)
		}
	}

	return tx.Commit()
}

// LoadMessages retrieves all messages for a conversation, ordered by step.
func (s *SQLiteConversationStore) LoadMessages(ctx context.Context, convID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT role, content, tool_calls_json, tool_call_id, name
FROM conversation_messages
WHERE conversation_id = ?
ORDER BY step ASC
`, convID)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var msg Message
		var toolCallsJSON sql.NullString
		if err := rows.Scan(&msg.Role, &msg.Content, &toolCallsJSON, &msg.ToolCallID, &msg.Name); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if toolCallsJSON.Valid && toolCallsJSON.String != "" {
			if err := json.Unmarshal([]byte(toolCallsJSON.String), &msg.ToolCalls); err != nil {
				return nil, fmt.Errorf("unmarshal tool calls: %w", err)
			}
		}
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	if msgs == nil {
		msgs = []Message{}
	}
	return msgs, nil
}

// ListConversations returns conversations ordered by updated_at DESC.
func (s *SQLiteConversationStore) ListConversations(ctx context.Context, limit, offset int) ([]Conversation, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, msg_count, created_at, updated_at
FROM conversations
ORDER BY updated_at DESC
LIMIT ? OFFSET ?
`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var c Conversation
		var createdText, updatedText string
		if err := rows.Scan(&c.ID, &c.Title, &c.MsgCount, &createdText, &updatedText); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdText)
		c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedText)
		convs = append(convs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	if convs == nil {
		convs = []Conversation{}
	}
	return convs, nil
}

// DeleteConversation removes a conversation and its messages (via CASCADE).
func (s *SQLiteConversationStore) DeleteConversation(ctx context.Context, convID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, convID)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}
