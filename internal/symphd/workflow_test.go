package symphd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadWorkflow_ValidFile tests that a well-formed WORKFLOW.md is parsed correctly.
func TestLoadWorkflow_ValidFile(t *testing.T) {
	dir := t.TempDir()
	content := `---
max_concurrent_agents: 3
max_turns: 15
turn_timeout_ms: 30000
retry_max_attempts: 2
workspace_type: worktree
track_label: symphd
---
Fix issue #{{ .issue_number }}: {{ .issue_title }}

{{ .issue_body }}
`
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	wf, err := LoadWorkflow(path)
	if err != nil {
		t.Fatalf("LoadWorkflow failed: %v", err)
	}
	if wf.MaxConcurrentAgents != 3 {
		t.Errorf("MaxConcurrentAgents = %d, want 3", wf.MaxConcurrentAgents)
	}
	if wf.MaxTurns != 15 {
		t.Errorf("MaxTurns = %d, want 15", wf.MaxTurns)
	}
	if wf.TurnTimeoutMs != 30000 {
		t.Errorf("TurnTimeoutMs = %d, want 30000", wf.TurnTimeoutMs)
	}
	if wf.RetryMaxAttempts != 2 {
		t.Errorf("RetryMaxAttempts = %d, want 2", wf.RetryMaxAttempts)
	}
	if wf.WorkspaceType != "worktree" {
		t.Errorf("WorkspaceType = %q, want worktree", wf.WorkspaceType)
	}
	if wf.TrackLabel != "symphd" {
		t.Errorf("TrackLabel = %q, want symphd", wf.TrackLabel)
	}
	if !strings.Contains(wf.Template, "{{ .issue_number }}") {
		t.Errorf("Template should contain '{{ .issue_number }}', got: %q", wf.Template)
	}
	if !strings.Contains(wf.Template, "Fix issue") {
		t.Errorf("Template should contain 'Fix issue', got: %q", wf.Template)
	}
}

// TestLoadWorkflow_NoFrontMatter tests that a file without front matter is treated as a pure template.
func TestLoadWorkflow_NoFrontMatter(t *testing.T) {
	dir := t.TempDir()
	content := `# Workflow: No Front Matter

Just a plain template with {{ .some_var }}.
`
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	wf, err := LoadWorkflow(path)
	if err != nil {
		t.Fatalf("LoadWorkflow failed: %v", err)
	}
	// Config fields should be zero values
	if wf.MaxConcurrentAgents != 0 {
		t.Errorf("MaxConcurrentAgents = %d, want 0", wf.MaxConcurrentAgents)
	}
	if wf.WorkspaceType != "" {
		t.Errorf("WorkspaceType = %q, want empty", wf.WorkspaceType)
	}
	// Template should be the full content
	if !strings.Contains(wf.Template, "{{ .some_var }}") {
		t.Errorf("Template should contain '{{ .some_var }}', got: %q", wf.Template)
	}
	if !strings.Contains(wf.Template, "No Front Matter") {
		t.Errorf("Template should contain 'No Front Matter', got: %q", wf.Template)
	}
}

// TestLoadWorkflow_MissingFile tests that loading a nonexistent file returns an error.
func TestLoadWorkflow_MissingFile(t *testing.T) {
	_, err := LoadWorkflow("/nonexistent/path/WORKFLOW.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestLoadWorkflow_MalformedFrontMatter tests that invalid YAML in front matter returns an error.
func TestLoadWorkflow_MalformedFrontMatter(t *testing.T) {
	dir := t.TempDir()
	content := `---
max_concurrent_agents: [bad yaml
---
Template body here.
`
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadWorkflow(path)
	if err == nil {
		t.Error("expected error for malformed front matter YAML")
	}
}

// TestLoadWorkflow_EmptyFrontMatter tests that ---\n---\n with a body works correctly.
func TestLoadWorkflow_EmptyFrontMatter(t *testing.T) {
	dir := t.TempDir()
	content := `---
---
Body content here with {{ .variable }}.
`
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	wf, err := LoadWorkflow(path)
	if err != nil {
		t.Fatalf("LoadWorkflow failed: %v", err)
	}
	if wf.MaxConcurrentAgents != 0 {
		t.Errorf("MaxConcurrentAgents = %d, want 0", wf.MaxConcurrentAgents)
	}
	if !strings.Contains(wf.Template, "Body content here") {
		t.Errorf("Template should contain body, got: %q", wf.Template)
	}
}

// TestLoadWorkflow_EmptyFile tests that an empty file returns an error or empty workflow.
func TestLoadWorkflow_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadWorkflow(path)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

// TestWorkflow_RenderPrompt_AllVars tests that all variables are substituted correctly.
func TestWorkflow_RenderPrompt_AllVars(t *testing.T) {
	wf := &Workflow{
		Template: "Fix issue #{{ .issue_number }}: {{ .issue_title }}\n\n{{ .issue_body }}",
	}
	vars := map[string]string{
		"issue_number": "42",
		"issue_title":  "Fix the bug",
		"issue_body":   "This is the body.",
	}
	result, err := wf.RenderPrompt(vars)
	if err != nil {
		t.Fatalf("RenderPrompt failed: %v", err)
	}
	expected := "Fix issue #42: Fix the bug\n\nThis is the body."
	if result != expected {
		t.Errorf("RenderPrompt result = %q, want %q", result, expected)
	}
}

// TestWorkflow_RenderPrompt_MissingVar tests that a missing variable returns an error (strict mode).
func TestWorkflow_RenderPrompt_MissingVar(t *testing.T) {
	wf := &Workflow{
		Template: "Hello {{ .name }}, your number is {{ .missing_var }}.",
	}
	vars := map[string]string{
		"name": "Alice",
	}
	_, err := wf.RenderPrompt(vars)
	if err == nil {
		t.Error("expected error for missing variable in strict mode")
	}
}

// TestWorkflow_RenderPrompt_NoVars tests that a template with no variables renders cleanly.
func TestWorkflow_RenderPrompt_NoVars(t *testing.T) {
	wf := &Workflow{
		Template: "This template has no variables. Just static text.",
	}
	result, err := wf.RenderPrompt(map[string]string{})
	if err != nil {
		t.Fatalf("RenderPrompt failed: %v", err)
	}
	if result != "This template has no variables. Just static text." {
		t.Errorf("RenderPrompt result = %q", result)
	}
}

// TestWorkflow_RenderPrompt_EmptyVars tests that a template referencing vars but receiving empty map returns error.
func TestWorkflow_RenderPrompt_EmptyVars(t *testing.T) {
	wf := &Workflow{
		Template: "Hello {{ .name }}.",
	}
	_, err := wf.RenderPrompt(map[string]string{})
	if err == nil {
		t.Error("expected error for template with vars but empty vars map")
	}
}

// TestWorkflow_FrontMatter_Defaults tests that unset fields are zero values (not config defaults).
func TestWorkflow_FrontMatter_Defaults(t *testing.T) {
	dir := t.TempDir()
	content := `---
max_concurrent_agents: 5
---
Template body.
`
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	wf, err := LoadWorkflow(path)
	if err != nil {
		t.Fatalf("LoadWorkflow failed: %v", err)
	}
	// Only set field should be non-zero
	if wf.MaxConcurrentAgents != 5 {
		t.Errorf("MaxConcurrentAgents = %d, want 5", wf.MaxConcurrentAgents)
	}
	// Unset fields must be zero values (not Config defaults)
	if wf.MaxTurns != 0 {
		t.Errorf("MaxTurns = %d, want 0 (zero value)", wf.MaxTurns)
	}
	if wf.WorkspaceType != "" {
		t.Errorf("WorkspaceType = %q, want empty string (zero value)", wf.WorkspaceType)
	}
	if wf.RetryMaxAttempts != 0 {
		t.Errorf("RetryMaxAttempts = %d, want 0 (zero value)", wf.RetryMaxAttempts)
	}
}

// TestWorkflow_FrontMatter_MaxConcurrentAgents tests parsing of integer fields.
func TestWorkflow_FrontMatter_MaxConcurrentAgents(t *testing.T) {
	dir := t.TempDir()
	content := `---
max_concurrent_agents: 7
max_turns: 25
turn_timeout_ms: 60000
retry_max_attempts: 3
---
Body.
`
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	wf, err := LoadWorkflow(path)
	if err != nil {
		t.Fatalf("LoadWorkflow failed: %v", err)
	}
	if wf.MaxConcurrentAgents != 7 {
		t.Errorf("MaxConcurrentAgents = %d, want 7", wf.MaxConcurrentAgents)
	}
	if wf.MaxTurns != 25 {
		t.Errorf("MaxTurns = %d, want 25", wf.MaxTurns)
	}
	if wf.TurnTimeoutMs != 60000 {
		t.Errorf("TurnTimeoutMs = %d, want 60000", wf.TurnTimeoutMs)
	}
	if wf.RetryMaxAttempts != 3 {
		t.Errorf("RetryMaxAttempts = %d, want 3", wf.RetryMaxAttempts)
	}
}

// TestLoadWorkflow_ExtraFields tests that unknown front matter fields are captured in Extra.
func TestLoadWorkflow_ExtraFields(t *testing.T) {
	dir := t.TempDir()
	content := `---
max_concurrent_agents: 2
custom_field: hello
another_field: 42
---
Body with {{ .var }}.
`
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	wf, err := LoadWorkflow(path)
	if err != nil {
		t.Fatalf("LoadWorkflow failed: %v", err)
	}
	if wf.MaxConcurrentAgents != 2 {
		t.Errorf("MaxConcurrentAgents = %d, want 2", wf.MaxConcurrentAgents)
	}
	// Extra fields should be captured
	if wf.Extra == nil {
		t.Error("Extra should not be nil when unknown fields are present")
	}
	if v, ok := wf.Extra["custom_field"]; !ok || v != "hello" {
		t.Errorf("Extra[custom_field] = %v, want 'hello'", wf.Extra["custom_field"])
	}
}

// TestLoadWorkflow_ErrorContainsPath tests that the error message contains the file path.
func TestLoadWorkflow_ErrorContainsPath(t *testing.T) {
	_, err := LoadWorkflow("/no/such/workflow.md")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "/no/such/workflow.md") {
		t.Errorf("error should contain path, got: %v", err)
	}
}

// TestWorkflow_RenderPrompt_MultipleUseOfSameVar tests that the same variable used multiple times renders correctly.
func TestWorkflow_RenderPrompt_MultipleUseOfSameVar(t *testing.T) {
	wf := &Workflow{
		Template: "Issue {{ .num }} is about {{ .title }}. See issue {{ .num }} for details.",
	}
	vars := map[string]string{
		"num":   "99",
		"title": "a bug",
	}
	result, err := wf.RenderPrompt(vars)
	if err != nil {
		t.Fatalf("RenderPrompt failed: %v", err)
	}
	expected := "Issue 99 is about a bug. See issue 99 for details."
	if result != expected {
		t.Errorf("RenderPrompt result = %q, want %q", result, expected)
	}
}

// TestLoadWorkflow_FrontMatterWithoutBody tests a file that has only front matter and no body.
func TestLoadWorkflow_FrontMatterWithoutBody(t *testing.T) {
	dir := t.TempDir()
	content := `---
max_concurrent_agents: 1
---
`
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	wf, err := LoadWorkflow(path)
	if err != nil {
		t.Fatalf("LoadWorkflow failed: %v", err)
	}
	if wf.MaxConcurrentAgents != 1 {
		t.Errorf("MaxConcurrentAgents = %d, want 1", wf.MaxConcurrentAgents)
	}
	// Template should be empty or just whitespace/newline
	trimmed := strings.TrimSpace(wf.Template)
	if trimmed != "" {
		t.Errorf("Template should be empty when no body, got: %q", wf.Template)
	}
}

// TestWorkflow_RenderPrompt_NilVars tests that nil vars map is handled gracefully.
func TestWorkflow_RenderPrompt_NilVars(t *testing.T) {
	wf := &Workflow{
		Template: "Static text only.",
	}
	result, err := wf.RenderPrompt(nil)
	if err != nil {
		t.Fatalf("RenderPrompt with nil vars failed: %v", err)
	}
	if result != "Static text only." {
		t.Errorf("RenderPrompt result = %q, want 'Static text only.'", result)
	}
}
