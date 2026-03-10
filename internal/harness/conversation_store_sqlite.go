package harness

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

const conversationSchema = `
CREATE TABLE IF NOT EXISTS conversations (
    id                TEXT PRIMARY KEY,
    title             TEXT NOT NULL DEFAULT '',
    msg_count         INTEGER NOT NULL DEFAULT 0,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd          REAL NOT NULL DEFAULT 0.0,
    pinned            INTEGER NOT NULL DEFAULT 0
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
    is_meta         INTEGER NOT NULL DEFAULT 0,
    UNIQUE(conversation_id, step)
);

CREATE INDEX IF NOT EXISTS idx_conv_msgs_conv_id ON conversation_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversations_updated ON conversations(updated_at);

-- FTS5 virtual table for full-text search on message content.
CREATE VIRTUAL TABLE IF NOT EXISTS conversation_messages_fts
USING fts5(conversation_id UNINDEXED, role UNINDEXED, content, content='conversation_messages', content_rowid='id');
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

// Migrate creates the schema tables and applies incremental migrations.
func (s *SQLiteConversationStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, conversationSchema)
	if err != nil {
		return fmt.Errorf("sqlite conversation migrate: %w", err)
	}

	// Idempotent migration: add is_meta column if it doesn't exist.
	if !s.columnExists(ctx, "conversation_messages", "is_meta") {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE conversation_messages ADD COLUMN is_meta INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("migrate add is_meta column: %w", err)
		}
	}

	// Idempotent migration: add pinned column if it doesn't exist (Issue #34).
	if !s.columnExists(ctx, "conversations", "pinned") {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE conversations ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("migrate add pinned column: %w", err)
		}
	}

	// Idempotent migration: add token/cost columns to conversations if they don't exist (Issue #32).
	if !s.columnExists(ctx, "conversations", "prompt_tokens") {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE conversations ADD COLUMN prompt_tokens INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("migrate add prompt_tokens column: %w", err)
		}
	}
	if !s.columnExists(ctx, "conversations", "completion_tokens") {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE conversations ADD COLUMN completion_tokens INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("migrate add completion_tokens column: %w", err)
		}
	}
	if !s.columnExists(ctx, "conversations", "cost_usd") {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE conversations ADD COLUMN cost_usd REAL NOT NULL DEFAULT 0.0`); err != nil {
			return fmt.Errorf("migrate add cost_usd column: %w", err)
		}
	}

	// Idempotent migration: create FTS5 triggers if they don't exist.
	// Triggers keep conversation_messages_fts in sync with conversation_messages.
	triggers := []string{
		`CREATE TRIGGER IF NOT EXISTS conv_msgs_fts_insert AFTER INSERT ON conversation_messages BEGIN
  INSERT INTO conversation_messages_fts(rowid, conversation_id, role, content) VALUES (new.id, new.conversation_id, new.role, new.content);
END`,
		`CREATE TRIGGER IF NOT EXISTS conv_msgs_fts_delete AFTER DELETE ON conversation_messages BEGIN
  INSERT INTO conversation_messages_fts(conversation_messages_fts, rowid, conversation_id, role, content) VALUES ('delete', old.id, old.conversation_id, old.role, old.content);
END`,
		`CREATE TRIGGER IF NOT EXISTS conv_msgs_fts_update AFTER UPDATE ON conversation_messages BEGIN
  INSERT INTO conversation_messages_fts(conversation_messages_fts, rowid, conversation_id, role, content) VALUES ('delete', old.id, old.conversation_id, old.role, old.content);
  INSERT INTO conversation_messages_fts(rowid, conversation_id, role, content) VALUES (new.id, new.conversation_id, new.role, new.content);
END`,
	}
	for _, trigger := range triggers {
		if _, err := s.db.ExecContext(ctx, trigger); err != nil {
			return fmt.Errorf("migrate create fts trigger: %w", err)
		}
	}

	return nil
}

// columnExists checks if a column exists in a table using PRAGMA table_info.
func (s *SQLiteConversationStore) columnExists(ctx context.Context, table, column string) bool {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

// SaveConversation persists a conversation's messages, replacing any existing messages.
// Token/cost fields are left unchanged (or zero for new conversations).
func (s *SQLiteConversationStore) SaveConversation(ctx context.Context, convID string, msgs []Message) error {
	return s.SaveConversationWithCost(ctx, convID, msgs, ConversationTokenCost{})
}

// SaveConversationWithCost persists a conversation's messages along with cumulative
// token usage and cost totals. It replaces any existing messages and overwrites
// the token/cost fields with the provided values.
func (s *SQLiteConversationStore) SaveConversationWithCost(ctx context.Context, convID string, msgs []Message, cost ConversationTokenCost) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	title := extractTitle(msgs)

	// Upsert conversations row (preserves created_at and pinned on conflict).
	// Only set the title when the row is first inserted; subsequent saves
	// preserve whatever title was set previously (auto-generated or user-provided).
	_, err = tx.ExecContext(ctx, `
INSERT INTO conversations (id, title, msg_count, created_at, updated_at, prompt_tokens, completion_tokens, cost_usd, pinned)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
ON CONFLICT(id) DO UPDATE SET
    msg_count         = excluded.msg_count,
    updated_at        = excluded.updated_at,
    prompt_tokens     = excluded.prompt_tokens,
    completion_tokens = excluded.completion_tokens,
    cost_usd          = excluded.cost_usd,
    title             = CASE WHEN conversations.title = '' THEN excluded.title ELSE conversations.title END
`, convID, title, len(msgs), now, now, cost.PromptTokens, cost.CompletionTokens, cost.CostUSD)
	if err != nil {
		return fmt.Errorf("upsert conversation: %w", err)
	}

	// Delete old messages
	if _, err := tx.ExecContext(ctx, `DELETE FROM conversation_messages WHERE conversation_id = ?`, convID); err != nil {
		return fmt.Errorf("delete old messages: %w", err)
	}

	// Insert new messages
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO conversation_messages (conversation_id, step, role, content, tool_calls_json, tool_call_id, name, is_meta)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
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
			str := string(data)
			toolCallsJSON = &str
		}

		isMeta := 0
		if msg.IsMeta {
			isMeta = 1
		}

		if _, err := stmt.ExecContext(ctx, convID, i, msg.Role, msg.Content, toolCallsJSON, msg.ToolCallID, msg.Name, isMeta); err != nil {
			return fmt.Errorf("insert message at step %d: %w", i, err)
		}
	}

	return tx.Commit()
}

// LoadMessages retrieves all messages for a conversation, ordered by step.
func (s *SQLiteConversationStore) LoadMessages(ctx context.Context, convID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT role, content, tool_calls_json, tool_call_id, name, is_meta
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
		var isMeta int
		if err := rows.Scan(&msg.Role, &msg.Content, &toolCallsJSON, &msg.ToolCallID, &msg.Name, &isMeta); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msg.IsMeta = isMeta == 1
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

// ListConversations returns conversations ordered by updated_at DESC,
// including cumulative token usage and cost totals.
func (s *SQLiteConversationStore) ListConversations(ctx context.Context, limit, offset int) ([]Conversation, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, msg_count, created_at, updated_at, prompt_tokens, completion_tokens, cost_usd, pinned
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
		var pinned int
		if err := rows.Scan(&c.ID, &c.Title, &c.MsgCount, &createdText, &updatedText, &c.PromptTokens, &c.CompletionTokens, &c.CostUSD, &pinned); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdText)
		c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedText)
		c.Pinned = pinned == 1
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

// DeleteOldConversations removes all non-pinned conversations whose updated_at is
// before olderThan. A zero olderThan is a no-op. Returns the number deleted.
func (s *SQLiteConversationStore) DeleteOldConversations(ctx context.Context, olderThan time.Time) (int, error) {
	if olderThan.IsZero() {
		return 0, nil
	}
	threshold := olderThan.UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM conversations WHERE updated_at < ? AND pinned = 0`,
		threshold,
	)
	if err != nil {
		return 0, fmt.Errorf("delete old conversations: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return int(n), nil
}

// PinConversation sets or clears the pinned flag on a conversation.
// Returns an error if the conversation does not exist.
func (s *SQLiteConversationStore) PinConversation(ctx context.Context, convID string, pin bool) error {
	pinVal := 0
	if pin {
		pinVal = 1
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE conversations SET pinned = ? WHERE id = ?`,
		pinVal, convID,
	)
	if err != nil {
		return fmt.Errorf("pin conversation: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("pin conversation rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("conversation %q not found", convID)
	}
	return nil
}

// SearchMessages performs a full-text search over message content using the FTS5 index.
func (s *SQLiteConversationStore) SearchMessages(ctx context.Context, query string, limit int) ([]MessageSearchResult, error) {
	if query == "" {
		return []MessageSearchResult{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT conversation_id, role, snippet(conversation_messages_fts, 2, '<b>', '</b>', '…', 20)
FROM conversation_messages_fts
WHERE conversation_messages_fts MATCH ?
ORDER BY rank
LIMIT ?
`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	var results []MessageSearchResult
	for rows.Next() {
		var r MessageSearchResult
		if err := rows.Scan(&r.ConversationID, &r.Role, &r.Snippet); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search rows error: %w", err)
	}
	if results == nil {
		results = []MessageSearchResult{}
	}
	return results, nil
}

// extractTitle derives a short title from the first user message in msgs.
// It returns the first sentence (up to the first ". ", "! ", or "? ") or the
// first 80 characters, whichever is shorter. Returns "" if no user message
// with non-empty content is found.
func extractTitle(msgs []Message) string {
	const maxLen = 80
	for _, m := range msgs {
		if m.Role != "user" || m.IsMeta {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		// Take only the first line.
		if idx := strings.IndexByte(content, '\n'); idx >= 0 {
			content = strings.TrimSpace(content[:idx])
		}
		// Find the first sentence boundary (". ", "! ", "? ").
		for _, sep := range []string{". ", "! ", "? "} {
			if idx := strings.Index(content, sep); idx >= 0 {
				candidate := content[:idx+1] // include the punctuation
				if utf8.RuneCountInString(candidate) <= maxLen {
					return candidate
				}
			}
		}
		// Truncate to maxLen runes.
		if utf8.RuneCountInString(content) > maxLen {
			runes := []rune(content)
			content = string(runes[:maxLen]) + "…"
		}
		return content
	}
	return ""
}
