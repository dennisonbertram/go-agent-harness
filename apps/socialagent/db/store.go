// Package db manages persistent user identity for the socialagent.
// It maps Telegram user IDs to internal UUIDs and conversation IDs.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	_ "github.com/lib/pq" // Postgres driver.
)

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

// GetUserByID looks up a user by their internal UUID.  Returns (nil, nil) when
// no matching row exists.
func (s *Store) GetUserByID(ctx context.Context, userID string) (*User, error) {
	const query = `
		SELECT id, telegram_id, conversation_id, display_name, created_at, updated_at
		FROM users
		WHERE id = $1`

	row := s.db.QueryRowContext(ctx, query, userID)
	u := &User{}
	err := row.Scan(&u.ID, &u.TelegramID, &u.ConversationID, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: GetUserByID: %w", err)
	}
	return u, nil
}

// GetUserByDisplayName looks up a user by display_name (case-insensitive).
// Returns (nil, nil) when no matching row exists.
func (s *Store) GetUserByDisplayName(ctx context.Context, displayName string) (*User, error) {
	const query = `
		SELECT id, telegram_id, conversation_id, display_name, created_at, updated_at
		FROM users
		WHERE lower(display_name) = lower($1)
		LIMIT 1`

	row := s.db.QueryRowContext(ctx, query, displayName)
	u := &User{}
	err := row.Scan(&u.ID, &u.TelegramID, &u.ConversationID, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: GetUserByDisplayName: %w", err)
	}
	return u, nil
}

// UpsertProfile creates or updates the user_profiles row for the given userID.
// last_summary_at is set to now() on every upsert.
func (s *Store) UpsertProfile(ctx context.Context, userID, summary string, interests []string, lookingFor string) error {
	const query = `
		INSERT INTO user_profiles (user_id, summary, interests, looking_for, last_summary_at, updated_at)
		VALUES ($1, $2, $3, $4, now(), now())
		ON CONFLICT (user_id) DO UPDATE
		SET summary        = EXCLUDED.summary,
		    interests      = EXCLUDED.interests,
		    looking_for    = EXCLUDED.looking_for,
		    last_summary_at = now(),
		    updated_at     = now()`

	if interests == nil {
		interests = []string{}
	}
	_, err := s.db.ExecContext(ctx, query, userID, summary, pq.Array(interests), lookingFor)
	if err != nil {
		return fmt.Errorf("db: UpsertProfile: %w", err)
	}
	return nil
}

// GetProfile retrieves the profile for userID.  Returns (nil, nil) when no row exists.
func (s *Store) GetProfile(ctx context.Context, userID string) (*UserProfile, error) {
	const query = `
		SELECT up.user_id, u.display_name, up.summary, up.interests, up.looking_for, up.last_summary_at, up.created_at, up.updated_at
		FROM user_profiles up
		JOIN users u ON u.id = up.user_id
		WHERE up.user_id = $1`

	row := s.db.QueryRowContext(ctx, query, userID)
	p := &UserProfile{}
	err := row.Scan(
		&p.UserID,
		&p.DisplayName,
		&p.Summary,
		pq.Array(&p.Interests),
		&p.LookingFor,
		&p.LastSummaryAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: GetProfile: %w", err)
	}
	return p, nil
}

// SearchProfiles performs a case-insensitive full-text search on summary and
// interests using ILIKE.  At most limit rows are returned.
func (s *Store) SearchProfiles(ctx context.Context, query string, limit int) ([]UserProfile, error) {
	const q = `
		SELECT up.user_id, u.display_name, up.summary, up.interests, up.looking_for, up.last_summary_at, up.created_at, up.updated_at
		FROM user_profiles up
		JOIN users u ON u.id = up.user_id
		WHERE up.summary ILIKE $1
		   OR EXISTS (
		       SELECT 1 FROM unnest(up.interests) AS i WHERE i ILIKE $1
		   )
		ORDER BY up.updated_at DESC
		LIMIT $2`

	pattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, q, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("db: SearchProfiles: %w", err)
	}
	defer rows.Close()

	return scanProfiles(rows)
}

// GetAllProfiles returns up to limit profiles, excluding the user with excludeUserID.
func (s *Store) GetAllProfiles(ctx context.Context, excludeUserID string, limit int) ([]UserProfile, error) {
	const q = `
		SELECT up.user_id, u.display_name, up.summary, up.interests, up.looking_for, up.last_summary_at, up.created_at, up.updated_at
		FROM user_profiles up
		JOIN users u ON u.id = up.user_id
		WHERE up.user_id != $1
		ORDER BY up.updated_at DESC
		LIMIT $2`

	rows, err := s.db.QueryContext(ctx, q, excludeUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: GetAllProfiles: %w", err)
	}
	defer rows.Close()

	return scanProfiles(rows)
}

// scanProfiles reads all UserProfile rows from a *sql.Rows cursor.
func scanProfiles(rows *sql.Rows) ([]UserProfile, error) {
	var profiles []UserProfile
	for rows.Next() {
		var p UserProfile
		if err := rows.Scan(
			&p.UserID,
			&p.DisplayName,
			&p.Summary,
			pq.Array(&p.Interests),
			&p.LookingFor,
			&p.LastSummaryAt,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("db: scanProfiles: %w", err)
		}
		profiles = append(profiles, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: scanProfiles rows: %w", err)
	}
	return profiles, nil
}

// LogActivity inserts a new row into activity_log.
func (s *Store) LogActivity(ctx context.Context, userID, displayName, activityType, content string) error {
	const query = `
		INSERT INTO activity_log (user_id, display_name, activity_type, content)
		VALUES ($1, $2, $3, $4)`

	_, err := s.db.ExecContext(ctx, query, userID, displayName, activityType, content)
	if err != nil {
		return fmt.Errorf("db: LogActivity: %w", err)
	}
	return nil
}

// GetRecentActivity returns up to limit activity entries ordered by created_at
// DESC, excluding entries that belong to excludeUserID.
func (s *Store) GetRecentActivity(ctx context.Context, limit int, excludeUserID string) ([]ActivityEntry, error) {
	const q = `
		SELECT id, user_id, display_name, activity_type, content, created_at
		FROM activity_log
		WHERE user_id != $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := s.db.QueryContext(ctx, q, excludeUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: GetRecentActivity: %w", err)
	}
	defer rows.Close()

	var entries []ActivityEntry
	for rows.Next() {
		var e ActivityEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.DisplayName, &e.ActivityType, &e.Content, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("db: GetRecentActivity scan: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: GetRecentActivity rows: %w", err)
	}
	return entries, nil
}

// SaveInsight inserts a new insight row for a user.
func (s *Store) SaveInsight(ctx context.Context, userID, insight, source string) error {
	const query = `
		INSERT INTO user_insights (user_id, insight, source)
		VALUES ($1, $2, $3)`

	_, err := s.db.ExecContext(ctx, query, userID, insight, source)
	if err != nil {
		return fmt.Errorf("db: SaveInsight: %w", err)
	}
	return nil
}

// GetInsights returns all insights for a user, ordered by created_at ASC.
func (s *Store) GetInsights(ctx context.Context, userID string) ([]UserInsight, error) {
	const q = `
		SELECT id, user_id, insight, source, created_at
		FROM user_insights
		WHERE user_id = $1
		ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("db: GetInsights: %w", err)
	}
	defer rows.Close()

	var insights []UserInsight
	for rows.Next() {
		var i UserInsight
		if err := rows.Scan(&i.ID, &i.UserID, &i.Insight, &i.Source, &i.CreatedAt); err != nil {
			return nil, fmt.Errorf("db: GetInsights scan: %w", err)
		}
		insights = append(insights, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: GetInsights rows: %w", err)
	}
	return insights, nil
}
