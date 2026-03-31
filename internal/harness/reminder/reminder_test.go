package reminder_test

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"go-agent-harness/internal/harness/reminder"
)

// TestDefaultConfig verifies that DefaultSystemReminderConfig returns expected defaults.
func TestDefaultConfig(t *testing.T) {
	cfg := reminder.DefaultSystemReminderConfig()
	if !cfg.Enabled {
		t.Error("expected Enabled=true by default")
	}
	if cfg.TagName != "system-reminder" {
		t.Errorf("expected TagName=%q, got %q", "system-reminder", cfg.TagName)
	}
	if !cfg.ExplanationInSystemPrompt {
		t.Error("expected ExplanationInSystemPrompt=true by default")
	}
}

// TestWrapReminder_Enabled verifies that WrapReminder wraps content in tags when enabled.
func TestWrapReminder_Enabled(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: true,
		TagName: "system-reminder",
	}
	result := reminder.WrapReminder("hello world", cfg)
	if !strings.HasPrefix(result, "<system-reminder>") {
		t.Errorf("expected result to start with <system-reminder>, got: %q", result)
	}
	if !strings.HasSuffix(result, "</system-reminder>") {
		t.Errorf("expected result to end with </system-reminder>, got: %q", result)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("expected result to contain content, got: %q", result)
	}
}

// TestWrapReminder_Disabled verifies that WrapReminder returns empty string when disabled.
func TestWrapReminder_Disabled(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: false,
		TagName: "system-reminder",
	}
	result := reminder.WrapReminder("hello world", cfg)
	if result != "" {
		t.Errorf("expected empty string when disabled, got: %q", result)
	}
}

// TestWrapReminder_CustomTag verifies that WrapReminder uses a custom tag name.
func TestWrapReminder_CustomTag(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: true,
		TagName: "my-custom-tag",
	}
	result := reminder.WrapReminder("content here", cfg)
	if !strings.Contains(result, "<my-custom-tag>") {
		t.Errorf("expected result to contain <my-custom-tag>, got: %q", result)
	}
	if !strings.Contains(result, "</my-custom-tag>") {
		t.Errorf("expected result to contain </my-custom-tag>, got: %q", result)
	}
	if strings.Contains(result, "<system-reminder>") {
		t.Errorf("expected result NOT to contain default tag, got: %q", result)
	}
}

// TestStripReminders_RemovesTags verifies that StripReminders strips reminder tags from text.
func TestStripReminders_RemovesTags(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: true,
		TagName: "system-reminder",
	}
	input := "before<system-reminder>injected content</system-reminder>after"
	result := reminder.StripReminders(input, cfg)
	if strings.Contains(result, "<system-reminder>") {
		t.Errorf("expected tags to be stripped, got: %q", result)
	}
	if strings.Contains(result, "injected content") {
		t.Errorf("expected inner content to be stripped, got: %q", result)
	}
	if !strings.Contains(result, "before") {
		t.Errorf("expected surrounding text 'before' preserved, got: %q", result)
	}
	if !strings.Contains(result, "after") {
		t.Errorf("expected surrounding text 'after' preserved, got: %q", result)
	}
}

// TestStripReminders_NoTags verifies that StripReminders returns text unchanged when no tags.
func TestStripReminders_NoTags(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: true,
		TagName: "system-reminder",
	}
	input := "plain text with no reminder tags"
	result := reminder.StripReminders(input, cfg)
	if result != input {
		t.Errorf("expected unchanged text, got: %q", result)
	}
}

// TestStripReminders_MultipleTags verifies that StripReminders strips multiple reminder blocks.
func TestStripReminders_MultipleTags(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: true,
		TagName: "system-reminder",
	}
	input := "start<system-reminder>first</system-reminder>middle<system-reminder>second</system-reminder>end"
	result := reminder.StripReminders(input, cfg)
	if strings.Contains(result, "first") {
		t.Errorf("expected first reminder content stripped, got: %q", result)
	}
	if strings.Contains(result, "second") {
		t.Errorf("expected second reminder content stripped, got: %q", result)
	}
	if !strings.Contains(result, "start") || !strings.Contains(result, "middle") || !strings.Contains(result, "end") {
		t.Errorf("expected surrounding text preserved, got: %q", result)
	}
}

// TestStripReminders_Disabled verifies that StripReminders returns text unchanged when disabled.
func TestStripReminders_Disabled(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: false,
		TagName: "system-reminder",
	}
	input := "text<system-reminder>content</system-reminder>more"
	result := reminder.StripReminders(input, cfg)
	if result != input {
		t.Errorf("expected unchanged text when disabled, got: %q", result)
	}
}

// TestHasReminders_True verifies that HasReminders detects reminder tags.
func TestHasReminders_True(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: true,
		TagName: "system-reminder",
	}
	input := "some text<system-reminder>payload</system-reminder>more"
	if !reminder.HasReminders(input, cfg) {
		t.Error("expected HasReminders to return true when tags present")
	}
}

// TestHasReminders_False verifies that HasReminders returns false when no tags present.
func TestHasReminders_False(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: true,
		TagName: "system-reminder",
	}
	input := "plain text without any reminder tags"
	if reminder.HasReminders(input, cfg) {
		t.Error("expected HasReminders to return false when no tags present")
	}
}

// TestHasReminders_CustomTag verifies that HasReminders detects custom tag names.
func TestHasReminders_CustomTag(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: true,
		TagName: "my-tag",
	}
	inputWithTag := "text<my-tag>content</my-tag>"
	if !reminder.HasReminders(inputWithTag, cfg) {
		t.Error("expected HasReminders to detect custom tag")
	}
	inputWithDefault := "text<system-reminder>content</system-reminder>"
	if reminder.HasReminders(inputWithDefault, cfg) {
		t.Error("expected HasReminders to NOT detect default tag when custom tag configured")
	}
}

// TestInjectReminder_AppendsToMessage verifies that InjectReminder appends the wrapped reminder.
func TestInjectReminder_AppendsToMessage(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: true,
		TagName: "system-reminder",
	}
	msg := "tool result content"
	reminderText := "today is Tuesday"
	result := reminder.InjectReminder(msg, reminderText, cfg)

	if !strings.HasPrefix(result, msg) {
		t.Errorf("expected result to start with original message, got: %q", result)
	}
	if !strings.Contains(result, "<system-reminder>") {
		t.Errorf("expected result to contain reminder open tag, got: %q", result)
	}
	if !strings.Contains(result, reminderText) {
		t.Errorf("expected result to contain reminder text, got: %q", result)
	}
	if !strings.Contains(result, "</system-reminder>") {
		t.Errorf("expected result to contain reminder close tag, got: %q", result)
	}
}

// TestInjectReminder_Disabled verifies that InjectReminder returns message unchanged when disabled.
func TestInjectReminder_Disabled(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled: false,
		TagName: "system-reminder",
	}
	msg := "tool result content"
	result := reminder.InjectReminder(msg, "reminder text", cfg)
	if result != msg {
		t.Errorf("expected unchanged message when disabled, got: %q", result)
	}
}

// TestSystemPromptExplanation_Enabled verifies explanation returns non-empty text when enabled.
func TestSystemPromptExplanation_Enabled(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled:                   true,
		TagName:                   "system-reminder",
		ExplanationInSystemPrompt: true,
	}
	result := reminder.SystemPromptExplanation(cfg)
	if result == "" {
		t.Error("expected non-empty explanation when enabled and ExplanationInSystemPrompt=true")
	}
}

// TestSystemPromptExplanation_Disabled verifies explanation returns empty string when feature disabled.
func TestSystemPromptExplanation_Disabled(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled:                   false,
		TagName:                   "system-reminder",
		ExplanationInSystemPrompt: true,
	}
	result := reminder.SystemPromptExplanation(cfg)
	if result != "" {
		t.Errorf("expected empty explanation when Enabled=false, got: %q", result)
	}
}

// TestSystemPromptExplanation_ExplanationFlagDisabled verifies explanation returns empty
// when ExplanationInSystemPrompt is false.
func TestSystemPromptExplanation_ExplanationFlagDisabled(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled:                   true,
		TagName:                   "system-reminder",
		ExplanationInSystemPrompt: false,
	}
	result := reminder.SystemPromptExplanation(cfg)
	if result != "" {
		t.Errorf("expected empty explanation when ExplanationInSystemPrompt=false, got: %q", result)
	}
}

// TestSystemPromptExplanation_ContainsTagName verifies that explanation mentions the tag name.
func TestSystemPromptExplanation_ContainsTagName(t *testing.T) {
	cfg := reminder.SystemReminderConfig{
		Enabled:                   true,
		TagName:                   "my-special-tag",
		ExplanationInSystemPrompt: true,
	}
	result := reminder.SystemPromptExplanation(cfg)
	if !strings.Contains(result, "my-special-tag") {
		t.Errorf("expected explanation to mention tag name %q, got: %q", "my-special-tag", result)
	}
}

// TestSystemReminderConfig_FromTOML verifies that the config can be parsed from TOML.
func TestSystemReminderConfig_FromTOML(t *testing.T) {
	type tomlRoot struct {
		SystemReminders reminder.SystemReminderConfig `toml:"system_reminders"`
	}

	input := `
[system_reminders]
enabled = true
tag_name = "system-reminder"
explanation_in_system_prompt = true
`
	var root tomlRoot
	if _, err := toml.Decode(input, &root); err != nil {
		t.Fatalf("failed to decode TOML: %v", err)
	}
	if !root.SystemReminders.Enabled {
		t.Error("expected Enabled=true from TOML")
	}
	if root.SystemReminders.TagName != "system-reminder" {
		t.Errorf("expected TagName=%q, got %q", "system-reminder", root.SystemReminders.TagName)
	}
	if !root.SystemReminders.ExplanationInSystemPrompt {
		t.Error("expected ExplanationInSystemPrompt=true from TOML")
	}
}

// TestSystemReminderConfig_FromTOML_NonDefault verifies custom values parse correctly from TOML.
func TestSystemReminderConfig_FromTOML_NonDefault(t *testing.T) {
	type tomlRoot struct {
		SystemReminders reminder.SystemReminderConfig `toml:"system_reminders"`
	}

	input := `
[system_reminders]
enabled = false
tag_name = "ctx-hint"
explanation_in_system_prompt = false
`
	var root tomlRoot
	if _, err := toml.Decode(input, &root); err != nil {
		t.Fatalf("failed to decode TOML: %v", err)
	}
	if root.SystemReminders.Enabled {
		t.Error("expected Enabled=false from TOML")
	}
	if root.SystemReminders.TagName != "ctx-hint" {
		t.Errorf("expected TagName=%q, got %q", "ctx-hint", root.SystemReminders.TagName)
	}
	if root.SystemReminders.ExplanationInSystemPrompt {
		t.Error("expected ExplanationInSystemPrompt=false from TOML")
	}
}
