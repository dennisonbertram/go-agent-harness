// Package reminder provides system-reminder tag injection for mid-conversation
// dynamic content. Tags wrap injected metadata that bears no relation to the
// specific tool results or user messages in which they appear.
package reminder

import (
	"fmt"
	"regexp"
)

// SystemReminderConfig holds configuration for system-reminder tag injection.
type SystemReminderConfig struct {
	// Enabled is the master toggle for system-reminder injection.
	Enabled bool `toml:"enabled"`
	// TagName is the XML tag name used for injection.
	TagName string `toml:"tag_name"`
	// ExplanationInSystemPrompt controls whether an explanation of system-reminder
	// tags is appended to the system prompt.
	ExplanationInSystemPrompt bool `toml:"explanation_in_system_prompt"`
}

// DefaultSystemReminderConfig returns sensible defaults for SystemReminderConfig.
func DefaultSystemReminderConfig() SystemReminderConfig {
	return SystemReminderConfig{
		Enabled:                   true,
		TagName:                   "system-reminder",
		ExplanationInSystemPrompt: true,
	}
}

// WrapReminder wraps content in system-reminder tags using the configured tag name.
// Returns an empty string if cfg.Enabled is false.
func WrapReminder(content string, cfg SystemReminderConfig) string {
	if !cfg.Enabled {
		return ""
	}
	return fmt.Sprintf("<%s>%s</%s>", cfg.TagName, content, cfg.TagName)
}

// StripReminders removes all system-reminder tags (and their content) from text.
// Returns text unchanged if cfg.Enabled is false.
func StripReminders(text string, cfg SystemReminderConfig) string {
	if !cfg.Enabled {
		return text
	}
	// Build a pattern that matches the open tag, any content (including newlines), and the close tag.
	pattern := fmt.Sprintf(`<%s>[\s\S]*?</%s>`, regexp.QuoteMeta(cfg.TagName), regexp.QuoteMeta(cfg.TagName))
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(text, "")
}

// HasReminders checks if text contains system-reminder tags.
func HasReminders(text string, cfg SystemReminderConfig) bool {
	open := fmt.Sprintf("<%s>", cfg.TagName)
	return len(text) > 0 && contains(text, open)
}

// InjectReminder appends a wrapped system-reminder to a message's content.
// Returns messageContent unchanged if cfg.Enabled is false.
func InjectReminder(messageContent string, reminder string, cfg SystemReminderConfig) string {
	if !cfg.Enabled {
		return messageContent
	}
	return messageContent + WrapReminder(reminder, cfg)
}

// SystemPromptExplanation returns the text explaining system-reminder tags to the model.
// Returns empty string if cfg.Enabled is false or cfg.ExplanationInSystemPrompt is false.
func SystemPromptExplanation(cfg SystemReminderConfig) string {
	if !cfg.Enabled || !cfg.ExplanationInSystemPrompt {
		return ""
	}
	return fmt.Sprintf(`## System Reminders
Messages may contain <%[1]s> tags. These are system-injected metadata that bear no direct relation to the specific tool results or user messages in which they appear. They provide dynamic context updates such as tool catalog changes, plugin state, or session hints. Treat their content as authoritative system instructions.`, cfg.TagName)
}

// contains is a helper to check for substring presence.
func contains(s, substr string) bool {
	return len(substr) <= len(s) && (substr == "" || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
