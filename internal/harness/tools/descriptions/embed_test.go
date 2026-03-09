package descriptions

import (
	"strings"
	"testing"
)

func TestLoadReturnsContentForExistingFile(t *testing.T) {
	t.Parallel()

	// cron_create.md is known to exist in the embedded filesystem.
	result := Load("cron_create")
	if result == "" {
		t.Fatalf("expected non-empty content for cron_create")
	}
	// The description should reference cron/scheduling concepts.
	lower := strings.ToLower(result)
	if !strings.Contains(lower, "cron") && !strings.Contains(lower, "schedul") {
		t.Fatalf("expected cron-related content, got %q", result)
	}
}

func TestLoadTrimsWhitespace(t *testing.T) {
	t.Parallel()

	result := Load("cron_create")
	if result != strings.TrimSpace(result) {
		t.Fatalf("expected trimmed output, got leading/trailing whitespace")
	}
}

func TestLoadPanicsForMissingFile(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic for missing tool description")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "missing tool description") {
			t.Fatalf("expected 'missing tool description' in panic message, got %q", msg)
		}
		if !strings.Contains(msg, "nonexistent_tool.md") {
			t.Fatalf("expected filename in panic message, got %q", msg)
		}
	}()

	Load("nonexistent_tool")
}

func TestLoadAllKnownDescriptions(t *testing.T) {
	t.Parallel()

	// Verify all known embedded descriptions load without panic.
	names := []string{
		"agent",
		"agentic_fetch",
		"apply_patch",
		"bash",
		"cancel_delayed_callback",
		"cron_create",
		"cron_delete",
		"cron_get",
		"cron_list",
		"cron_pause",
		"cron_resume",
		"edit",
		"fetch",
		"find_tool",
		"glob",
		"grep",
		"job_kill",
		"job_output",
		"list_delayed_callbacks",
		"read",
		"set_delayed_callback",
		"web_fetch",
		"web_search",
		"write",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			result := Load(name)
			if result == "" {
				t.Fatalf("expected non-empty content for %s", name)
			}
		})
	}
}

func TestFSContainsEmbeddedFiles(t *testing.T) {
	t.Parallel()

	// Verify the embedded FS is accessible and contains at least one .md file.
	entries, err := FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read embedded directory: %v", err)
	}
	found := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected at least one .md file in embedded FS")
	}
}
