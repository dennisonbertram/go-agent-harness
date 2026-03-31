// Package db manages persistent user identity for the socialagent.
// It maps Telegram user IDs to internal UUIDs and conversation IDs.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // Postgres driver.
)

// User represents a row in the users table.
type User struct {
	ID             string
	TelegramID     int64
	ConversationID string
	DisplayName    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Store wraps a *sql.DB and provides user-identity operations.
type Store struct {
	db *sql.DB
}

// NewStore opens a connection to Postgres at databaseURL, verifies it, and
// runs all pending migrations.  The caller must call Close when done.
func NewStore(databaseURL string) (*Store, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the underlying database connection pool.
func (s *Store) Close() error {
	return s.db.Close()
}

// GetOrCreateUser looks up the user by telegramID.  If no row exists, a new
// user is inserted with a fresh UUID and a fresh conversation_id.  The
// operation is idempotent: concurrent callers for the same telegramID will
// both succeed and receive the same row.
func (s *Store) GetOrCreateUser(ctx context.Context, telegramID int64, displayName string) (*User, error) {
	// Use INSERT … ON CONFLICT DO NOTHING, then SELECT.  This is safe under
	// concurrent load because telegram_id has a UNIQUE constraint.
	const insert = `
		INSERT INTO users (telegram_id, display_name)
		VALUES ($1, $2)
		ON CONFLICT (telegram_id) DO NOTHING`

	if _, err := s.db.ExecContext(ctx, insert, telegramID, displayName); err != nil {
		return nil, fmt.Errorf("db: GetOrCreateUser insert: %w", err)
	}

	return s.GetUser(ctx, telegramID)
}

// GetUser looks up a user by telegramID.  It returns (nil, nil) when no
// matching row exists.
func (s *Store) GetUser(ctx context.Context, telegramID int64) (*User, error) {
	const query = `
		SELECT id, telegram_id, conversation_id, display_name, created_at, updated_at
		FROM users
		WHERE telegram_id = $1`

	row := s.db.QueryRowContext(ctx, query, telegramID)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: GetUser: %w", err)
	}
	return u, nil
}

// UpdateDisplayName changes the display_name for the user identified by
// userID (the internal UUID).  It also refreshes updated_at.
func (s *Store) UpdateDisplayName(ctx context.Context, userID string, name string) error {
	const query = `
		UPDATE users
		SET display_name = $1, updated_at = now()
		WHERE id = $2`

	result, err := s.db.ExecContext(ctx, query, name, userID)
	if err != nil {
		return fmt.Errorf("db: UpdateDisplayName: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: UpdateDisplayName rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("db: UpdateDisplayName: user %s not found", userID)
	}
	return nil
}

// DeleteUserByTelegramID removes a user row — used only by tests for
// clean-up / isolation.
func (s *Store) DeleteUserByTelegramID(ctx context.Context, telegramID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE telegram_id = $1`, telegramID)
	return err
}

// scanUser reads one User from a *sql.Row.
func scanUser(row *sql.Row) (*User, error) {
	u := &User{}
	err := row.Scan(
		&u.ID,
		&u.TelegramID,
		&u.ConversationID,
		&u.DisplayName,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	return u, err
}
