// Package db manages persistent user identity for the socialagent.
package db

import "time"

// User represents a row in the users table.
type User struct {
	ID             string    `json:"id"`
	TelegramID     int64     `json:"telegram_id"`
	ConversationID string    `json:"conversation_id"`
	DisplayName    string    `json:"display_name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// UserProfile holds an LLM-generated summary and interests for a user.
type UserProfile struct {
	UserID        string    `json:"user_id"`
	DisplayName   string    `json:"display_name"`
	Summary       string    `json:"summary"`
	Interests     []string  `json:"interests"`
	LookingFor    string    `json:"looking_for"`
	LastSummaryAt time.Time `json:"last_summary_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ActivityEntry records a single user activity event.
type ActivityEntry struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	DisplayName  string    `json:"display_name"`
	ActivityType string    `json:"activity_type"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserInsight is an agent observation about a user.
type UserInsight struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Insight   string    `json:"insight"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}
