package harness

// Schema-migration tests for the SQLite conversation store.
//
// Migrate() is written as a sequence of idempotent ALTER TABLE steps guarded by
// columnExists(). The schema DDL it runs first uses CREATE TABLE IF NOT EXISTS,
// so against a database that already holds an OLDER table shape the DDL is a
// no-op and the ALTER steps are the only thing that brings it forward. That is
// precisely the path a real upgrade takes, and it was the least-covered code in
// the package because every other test starts from an empty directory where the
// DDL creates the current shape outright and every ALTER is skipped.
//
// These tests build the historical table shapes explicitly, put real rows in
// them, migrate forward, and assert the data survives with correct defaults.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// v1ConversationSchema is the conversation store's original table shape, before
// any of the incremental columns were added: no pinned, no token/cost columns,
// no workspace/tenant_id on conversations; no is_meta, no is_compact_summary on
// conversation_messages. It deliberately does NOT create the FTS table or its
// triggers, which also arrived later.
const v1ConversationSchema = `
CREATE TABLE conversations (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL DEFAULT '',
    msg_count  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE conversation_messages (
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
`

// seedV1Database writes a database at path using the v1 schema and populates it
// with one conversation and two messages, returning the path.
func seedV1Database(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open v1 db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(v1ConversationSchema); err != nil {
		t.Fatalf("create v1 schema: %v", err)
	}

	created := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	updated := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(
		`INSERT INTO conversations (id, title, msg_count, created_at, updated_at) VALUES (?,?,?,?,?)`,
		"conv-legacy", "A conversation from before the migrations", 2, created, updated,
	); err != nil {
		t.Fatalf("insert v1 conversation: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO conversation_messages (conversation_id, step, role, content, tool_call_id, name) VALUES (?,?,?,?,?,?)`,
		"conv-legacy", 1, "user", "hello from the old schema", "", "",
	); err != nil {
		t.Fatalf("insert v1 message 1: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO conversation_messages (conversation_id, step, role, content, tool_call_id, name) VALUES (?,?,?,?,?,?)`,
		"conv-legacy", 2, "assistant", "reply from the old schema", "", "",
	); err != nil {
		t.Fatalf("insert v1 message 2: %v", err)
	}
}

// TestMigrate_FromV1SchemaPreservesDataAndAddsColumns is the core upgrade case:
// an existing database in the original shape must gain every later column and
// keep every row it already had.
func TestMigrate_FromV1SchemaPreservesDataAndAddsColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conversations.db")
	seedV1Database(t, path)

	store, err := NewSQLiteConversationStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate from v1: %v", err)
	}

	// Every column added by an incremental migration must now exist.
	for _, c := range []struct{ table, column string }{
		{"conversations", "pinned"},
		{"conversations", "prompt_tokens"},
		{"conversations", "completion_tokens"},
		{"conversations", "cost_usd"},
		{"conversations", "workspace"},
		{"conversations", "tenant_id"},
		{"conversation_messages", "is_meta"},
		{"conversation_messages", "is_compact_summary"},
	} {
		if !store.columnExists(ctx, c.table, c.column) {
			t.Errorf("column %s.%s missing after migrating from v1", c.table, c.column)
		}
	}

	// The pre-existing conversation must survive, with the new columns
	// carrying their declared defaults rather than nulls.
	var (
		title            string
		msgCount         int
		pinned           int
		promptTokens     int
		completionTokens int
		costUSD          float64
		workspace        string
		tenantID         string
	)
	err = store.db.QueryRowContext(ctx,
		`SELECT title, msg_count, pinned, prompt_tokens, completion_tokens, cost_usd, workspace, tenant_id
		 FROM conversations WHERE id = ?`, "conv-legacy",
	).Scan(&title, &msgCount, &pinned, &promptTokens, &completionTokens, &costUSD, &workspace, &tenantID)
	if err != nil {
		t.Fatalf("read migrated conversation: %v", err)
	}
	if title != "A conversation from before the migrations" {
		t.Errorf("title = %q, want the pre-migration value", title)
	}
	if msgCount != 2 {
		t.Errorf("msg_count = %d, want 2", msgCount)
	}
	if pinned != 0 || promptTokens != 0 || completionTokens != 0 || costUSD != 0 {
		t.Errorf("new numeric columns should default to zero, got pinned=%d prompt=%d completion=%d cost=%v",
			pinned, promptTokens, completionTokens, costUSD)
	}
	if workspace != "" || tenantID != "" {
		t.Errorf("new text columns should default to empty, got workspace=%q tenant_id=%q", workspace, tenantID)
	}

	// Both pre-existing messages must survive and be loadable through the
	// normal API, which selects the post-migration column set.
	msgs, err := store.LoadMessages(ctx, "conv-legacy")
	if err != nil {
		t.Fatalf("load migrated messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("loaded %d messages after migration, want 2", len(msgs))
	}
	if msgs[0].Content != "hello from the old schema" || msgs[1].Content != "reply from the old schema" {
		t.Errorf("message contents did not survive migration: %+v", msgs)
	}
	for i, m := range msgs {
		if m.IsMeta {
			t.Errorf("message %d: is_meta should default to false after migration", i)
		}
		if m.IsCompactSummary {
			t.Errorf("message %d: is_compact_summary should default to false after migration", i)
		}
	}
}

// TestMigrate_IsIdempotent runs Migrate repeatedly against the same database.
// Every ALTER step is guarded by columnExists(); a missing guard would surface
// here as a "duplicate column name" error on the second pass.
func TestMigrate_IsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conversations.db")
	seedV1Database(t, path)

	store, err := NewSQLiteConversationStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := store.Migrate(ctx); err != nil {
			t.Fatalf("migrate pass %d: %v", i+1, err)
		}
	}

	msgs, err := store.LoadMessages(ctx, "conv-legacy")
	if err != nil {
		t.Fatalf("load after repeated migration: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("repeated migration changed message count to %d, want 2", len(msgs))
	}
}

// TestMigrate_OnFreshDatabaseCreatesCurrentShape covers the other entry point:
// a brand-new database, where the schema DDL creates the current shape outright
// and every ALTER step is skipped.
func TestMigrate_OnFreshDatabaseCreatesCurrentShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")
	store, err := NewSQLiteConversationStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate fresh: %v", err)
	}

	for _, c := range []struct{ table, column string }{
		{"conversations", "pinned"},
		{"conversations", "tenant_id"},
		{"conversation_messages", "is_meta"},
		{"conversation_messages", "is_compact_summary"},
	} {
		if !store.columnExists(ctx, c.table, c.column) {
			t.Errorf("fresh database missing %s.%s", c.table, c.column)
		}
	}
	if store.columnExists(ctx, "conversations", "definitely_not_a_column") {
		t.Error("columnExists reported a column that does not exist")
	}
	if store.columnExists(ctx, "no_such_table", "id") {
		t.Error("columnExists should report false for a nonexistent table")
	}
}

// TestMigrate_FTSTriggersIndexPostMigrationWrites covers full-text search
// across an upgrade.
//
// This test originally failed, and the failure was a real bug:
// conversation_messages_fts is an EXTERNAL-CONTENT FTS5 table, so rows already
// present when the triggers were first created were never indexed, and the
// external-content 'delete' command issued by the delete trigger corrupted the
// index for them — SaveConversation (which deletes old messages first) failed
// with "database disk image is malformed" on the first save after any upgrade.
// Migrate now rebuilds the index once, at the migration that introduces the
// triggers, so both pre- and post-migration content is searchable.
func TestMigrate_FTSTriggersIndexPostMigrationWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conversations.db")
	seedV1Database(t, path)

	store, err := NewSQLiteConversationStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Content that existed BEFORE the migration must be searchable, which
	// requires the index rebuild — the triggers alone cannot backfill it.
	preExisting, err := store.SearchMessages(ctx, "", "old", 10)
	if err != nil {
		t.Fatalf("search pre-migration content: %v", err)
	}
	if len(preExisting) == 0 {
		t.Error("messages that predate the migration should be searchable after the index rebuild")
	}

	// Saving deletes the existing messages first, which fires the delete
	// trigger for rows the index must already know about. Before the rebuild
	// was added this call failed outright.
	if err := store.SaveConversation(ctx, "conv-legacy", []Message{
		{Role: "user", Content: "hello from the old schema"},
		{Role: "assistant", Content: "reply from the old schema"},
		{Role: "user", Content: "zarquon written after migrating"},
	}); err != nil {
		t.Fatalf("save post-migration message: %v", err)
	}

	results, err := store.SearchMessages(ctx, "", "zarquon", 10)
	if err != nil {
		t.Fatalf("search post-migration content: %v", err)
	}
	if len(results) == 0 {
		t.Error("a message written after migration should be full-text searchable")
	}

	// Verify the triggers themselves exist, which is the migration's
	// observable output regardless of what is in the index.
	for _, name := range []string{"conv_msgs_fts_insert", "conv_msgs_fts_delete", "conv_msgs_fts_update"} {
		var found string
		err := store.db.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='trigger' AND name=?`, name).Scan(&found)
		if err != nil {
			t.Errorf("FTS trigger %q not created by migration: %v", name, err)
		}
	}
}

// TestMigrate_PartiallyMigratedDatabase covers the in-between case a real
// deployment hits when it upgrades across several releases at once: some
// columns already added, others not. Each guard must be evaluated
// independently rather than short-circuiting on the first one it finds.
func TestMigrate_PartiallyMigratedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.db")
	seedV1Database(t, path)

	// Apply only SOME of the later migrations by hand, in the order history
	// added them, leaving the rest for Migrate to finish.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open partial db: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE conversation_messages ADD COLUMN is_meta INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE conversations ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE conversations ADD COLUMN prompt_tokens INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("apply partial migration %q: %v", stmt, err)
		}
	}
	db.Close()

	store, err := NewSQLiteConversationStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate partially-migrated database: %v", err)
	}

	for _, c := range []struct{ table, column string }{
		{"conversations", "pinned"},
		{"conversations", "prompt_tokens"},
		{"conversations", "completion_tokens"},
		{"conversations", "cost_usd"},
		{"conversations", "workspace"},
		{"conversations", "tenant_id"},
		{"conversation_messages", "is_meta"},
		{"conversation_messages", "is_compact_summary"},
	} {
		if !store.columnExists(ctx, c.table, c.column) {
			t.Errorf("column %s.%s missing after migrating a partially-migrated database", c.table, c.column)
		}
	}

	msgs, err := store.LoadMessages(ctx, "conv-legacy")
	if err != nil {
		t.Fatalf("load after partial migration: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("partial migration lost messages: got %d, want 2", len(msgs))
	}
}
