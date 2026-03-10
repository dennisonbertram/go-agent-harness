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
	// This list must be kept in sync with the .md files in this directory.
	names := []string{
		"AskUserQuestion",
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
		"download",
		"edit",
		"fetch",
		"find_tool",
		"git_diff",
		"git_status",
		"glob",
		"grep",
		"job_kill",
		"job_output",
		"list_delayed_callbacks",
		"list_mcp_resources",
		"list_models",
		"ls",
		"lsp_diagnostics",
		"lsp_references",
		"lsp_restart",
		"observational_memory",
		"read",
		"read_mcp_resource",
		"set_delayed_callback",
		"skill",
		"sourcegraph",
		"todos",
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

// TestAllEmbeddedDescriptionsAreNonEmpty dynamically discovers every .md file
// in the embedded FS and verifies it loads to a non-empty string. This catches
// newly-added files that are accidentally empty without requiring a manual update
// to TestLoadAllKnownDescriptions.
func TestAllEmbeddedDescriptionsAreNonEmpty(t *testing.T) {
	t.Parallel()

	entries, err := FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read embedded directory: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("embedded FS is empty — no description files found")
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		toolName := strings.TrimSuffix(name, ".md")
		t.Run(toolName, func(t *testing.T) {
			result := Load(toolName)
			if result == "" {
				t.Fatalf("description file %s exists but Load(%q) returned empty string", name, toolName)
			}
		})
	}
}

// TestEmbeddedFSAndKnownListAreInSync verifies that the hardcoded list in
// TestLoadAllKnownDescriptions matches exactly the .md files in the embedded FS.
// This prevents the two lists from drifting apart silently.
func TestEmbeddedFSAndKnownListAreInSync(t *testing.T) {
	t.Parallel()

	knownNames := map[string]bool{
		"AskUserQuestion":        true,
		"agent":                  true,
		"agentic_fetch":          true,
		"apply_patch":            true,
		"bash":                   true,
		"cancel_delayed_callback": true,
		"cron_create":            true,
		"cron_delete":            true,
		"cron_get":               true,
		"cron_list":              true,
		"cron_pause":             true,
		"cron_resume":            true,
		"deploy":                 true,
		"download":               true,
		"edit":                   true,
		"fetch":                  true,
		"find_tool":              true,
		"git_diff":               true,
		"git_status":             true,
		"glob":                   true,
		"grep":                   true,
		"job_kill":               true,
		"job_output":             true,
		"list_delayed_callbacks": true,
		"list_mcp_resources":     true,
		"list_models":            true,
		"ls":                     true,
		"lsp_diagnostics":        true,
		"lsp_references":         true,
		"lsp_restart":            true,
		"observational_memory":   true,
		"read":                   true,
		"read_mcp_resource":      true,
		"set_delayed_callback":   true,
		"skill":                  true,
		"sourcegraph":            true,
		"todos":                  true,
		"web_fetch":              true,
		"web_search":             true,
		"write":                  true,
	}

	entries, err := FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read embedded directory: %v", err)
	}

	fsNames := make(map[string]bool)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".md") {
			fsNames[strings.TrimSuffix(name, ".md")] = true
		}
	}

	for name := range fsNames {
		if !knownNames[name] {
			t.Errorf("FS contains %q but it is missing from the known list — add it to TestLoadAllKnownDescriptions", name)
		}
	}
	for name := range knownNames {
		if !fsNames[name] {
			t.Errorf("known list contains %q but no corresponding .md file exists in the embedded FS", name)
		}
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
