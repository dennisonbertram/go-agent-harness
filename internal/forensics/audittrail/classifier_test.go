package audittrail_test

import (
	"testing"

	"go-agent-harness/internal/forensics/audittrail"
)

func TestIsStateModifying_StateModifyingTools(t *testing.T) {
	stateModifying := []string{
		"file_write",
		"file_delete",
		"bash",
		"git_commit",
		"git_push",
		"write_file",
		"delete_file",
		"create_file",
		"modify_config",
		"file_write_patch",
		"create_directory",
	}

	for _, tool := range stateModifying {
		t.Run(tool, func(t *testing.T) {
			if !audittrail.IsStateModifying(tool) {
				t.Errorf("IsStateModifying(%q) = false, want true", tool)
			}
		})
	}
}

func TestIsStateModifying_ReadOnlyTools(t *testing.T) {
	readOnly := []string{
		"file_read",
		"grep",
		"glob",
		"find_tool",
		"list_directory",
		"read_file",
		"search_code",
		"get_run_summary",
		"list_runs",
		"ask_user_question",
	}

	for _, tool := range readOnly {
		t.Run(tool, func(t *testing.T) {
			if audittrail.IsStateModifying(tool) {
				t.Errorf("IsStateModifying(%q) = true, want false", tool)
			}
		})
	}
}

func TestIsStateModifying_EmptyTool(t *testing.T) {
	if audittrail.IsStateModifying("") {
		t.Error("IsStateModifying(\"\") = true, want false")
	}
}

func TestIsStateModifying_ExactMatches(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		expected bool
	}{
		{"bash exact", "bash", true},
		{"file_write exact", "file_write", true},
		{"file_delete exact", "file_delete", true},
		{"git_commit exact", "git_commit", true},
		{"git_push exact", "git_push", true},
		{"file_read not modifying", "file_read", false},
		{"grep not modifying", "grep", false},
		{"glob not modifying", "glob", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := audittrail.IsStateModifying(tc.tool)
			if got != tc.expected {
				t.Errorf("IsStateModifying(%q) = %v, want %v", tc.tool, got, tc.expected)
			}
		})
	}
}

func TestIsStateModifying_SubstringKeywords(t *testing.T) {
	// Tools with write/delete/create/modify in name are state-modifying
	tests := []struct {
		tool     string
		expected bool
	}{
		{"custom_write_tool", true},
		{"custom_delete_tool", true},
		{"custom_create_tool", true},
		{"custom_modify_tool", true},
		{"write_something", true},
		{"delete_something", true},
		{"create_something", true},
		{"modify_something", true},
		// These should NOT match
		{"writer", false},  // "write" substring but not keyword separated
		{"readwriter", false},
	}

	for _, tc := range tests {
		t.Run(tc.tool, func(t *testing.T) {
			got := audittrail.IsStateModifying(tc.tool)
			if got != tc.expected {
				t.Errorf("IsStateModifying(%q) = %v, want %v", tc.tool, got, tc.expected)
			}
		})
	}
}
