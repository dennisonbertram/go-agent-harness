package deferred

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// newTestExtensionTool creates a CreatePromptExtensionTool backed by a temp directory.
func newTestExtensionTool(t *testing.T) (tools.Tool, string, string) {
	t.Helper()
	root := t.TempDir()
	behaviorsDir := filepath.Join(root, "behaviors")
	talentsDir := filepath.Join(root, "talents")
	if err := os.MkdirAll(behaviorsDir, 0o755); err != nil {
		t.Fatalf("create behaviors dir: %v", err)
	}
	if err := os.MkdirAll(talentsDir, 0o755); err != nil {
		t.Fatalf("create talents dir: %v", err)
	}
	dirs := tools.PromptExtensionDirs{BehaviorsDir: behaviorsDir, TalentsDir: talentsDir}
	return CreatePromptExtensionTool(dirs), behaviorsDir, talentsDir
}

// TestCreatePromptExtensionTool_Definition verifies the tool definition.
func TestCreatePromptExtensionTool_Definition(t *testing.T) {
	tool, _, _ := newTestExtensionTool(t)
	assertToolDef(t, tool, "create_prompt_extension", tools.TierDeferred)
	assertHasTags(t, tool, "prompt", "extension", "behavior", "talent")
}

// TestCreatePromptExtensionTool_NoDirs verifies the tool returns an error when dirs are not configured.
func TestCreatePromptExtensionTool_NoDirs(t *testing.T) {
	tool := CreatePromptExtensionTool(tools.PromptExtensionDirs{})
	args := `{"extension_type":"behavior","name":"test","title":"Test","content":"# Test"}`
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for unconfigured extension dirs")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' in error, got %q", err.Error())
	}
}

// TestCreatePromptExtensionTool_InvalidJSON verifies the tool returns an error for invalid JSON.
func TestCreatePromptExtensionTool_InvalidJSON(t *testing.T) {
	tool, _, _ := newTestExtensionTool(t)
	_, err := tool.Handler(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// TestCreatePromptExtensionTool_MissingExtensionType verifies the tool returns an error when extension_type is missing.
func TestCreatePromptExtensionTool_MissingExtensionType(t *testing.T) {
	tool, _, _ := newTestExtensionTool(t)
	args := `{"name":"test","title":"Test","content":"# Test"}`
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for missing extension_type")
	}
}

// TestCreatePromptExtensionTool_InvalidExtensionType verifies the tool rejects unknown extension types.
func TestCreatePromptExtensionTool_InvalidExtensionType(t *testing.T) {
	tool, _, _ := newTestExtensionTool(t)
	args := `{"extension_type":"skill","name":"test","title":"Test","content":"# Test"}`
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for invalid extension_type")
	}
	if !strings.Contains(err.Error(), "extension_type") {
		t.Errorf("expected 'extension_type' in error, got %q", err.Error())
	}
}

// TestCreatePromptExtensionTool_MissingName verifies the tool returns an error when name is missing.
func TestCreatePromptExtensionTool_MissingName(t *testing.T) {
	tool, _, _ := newTestExtensionTool(t)
	args := `{"extension_type":"behavior","title":"Test","content":"# Test"}`
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

// TestCreatePromptExtensionTool_MissingContent verifies the tool returns an error when content is missing.
func TestCreatePromptExtensionTool_MissingContent(t *testing.T) {
	tool, _, _ := newTestExtensionTool(t)
	args := `{"extension_type":"behavior","name":"test","title":"Test"}`
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

// TestCreatePromptExtensionTool_WriteBehavior verifies writing a behavior extension.
func TestCreatePromptExtensionTool_WriteBehavior(t *testing.T) {
	tool, behaviorsDir, _ := newTestExtensionTool(t)
	args := `{"extension_type":"behavior","name":"prefer-short","title":"Prefer Short Answers","content":"# Prefer Short Answers\nBe concise."}`
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Verify file was written
	target := filepath.Join(behaviorsDir, "prefer-short.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(data), "Prefer Short Answers") {
		t.Errorf("expected content in file, got %q", string(data))
	}
}

// TestCreatePromptExtensionTool_WriteTalent verifies writing a talent extension.
func TestCreatePromptExtensionTool_WriteTalent(t *testing.T) {
	tool, _, talentsDir := newTestExtensionTool(t)
	args := `{"extension_type":"talent","name":"go-expert","title":"Go Expert","content":"# Go Expert\nKnows Go deeply."}`
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	target := filepath.Join(talentsDir, "go-expert.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(data), "Go Expert") {
		t.Errorf("expected content in file, got %q", string(data))
	}
}

// TestCreatePromptExtensionTool_NoOverwriteByDefault verifies existing files are not overwritten by default.
func TestCreatePromptExtensionTool_NoOverwriteByDefault(t *testing.T) {
	tool, behaviorsDir, _ := newTestExtensionTool(t)

	// Write an existing file
	existing := filepath.Join(behaviorsDir, "my-behavior.md")
	if err := os.WriteFile(existing, []byte("original"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	args := `{"extension_type":"behavior","name":"my-behavior","title":"My Behavior","content":"new content"}`
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error when file exists and overwrite=false")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got %q", err.Error())
	}

	// Verify original is unchanged
	data, _ := os.ReadFile(existing)
	if string(data) != "original" {
		t.Errorf("expected original content preserved, got %q", string(data))
	}
}

// TestCreatePromptExtensionTool_OverwriteFlag verifies existing files can be overwritten with overwrite=true.
func TestCreatePromptExtensionTool_OverwriteFlag(t *testing.T) {
	tool, behaviorsDir, _ := newTestExtensionTool(t)

	// Write an existing file
	existing := filepath.Join(behaviorsDir, "my-behavior.md")
	if err := os.WriteFile(existing, []byte("original"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	args := `{"extension_type":"behavior","name":"my-behavior","title":"My Behavior","content":"new content","overwrite":true}`
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Verify file was overwritten
	data, _ := os.ReadFile(existing)
	if string(data) != "new content" {
		t.Errorf("expected new content, got %q", string(data))
	}
}

// TestCreatePromptExtensionTool_NameSanitization verifies names with unsafe characters are sanitized.
func TestCreatePromptExtensionTool_NameSanitization(t *testing.T) {
	tool, behaviorsDir, _ := newTestExtensionTool(t)
	// Use a name with uppercase and spaces — should be sanitized
	args := `{"extension_type":"behavior","name":"My Behavior Name","title":"My Behavior","content":"content"}`
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Verify a sanitized file was created (exact name TBD by implementation)
	entries, err := os.ReadDir(behaviorsDir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected a file to be created")
	}
}

// TestCreatePromptExtensionTool_NamePathTraversal verifies path traversal in names is rejected.
func TestCreatePromptExtensionTool_NamePathTraversal(t *testing.T) {
	tool, _, _ := newTestExtensionTool(t)
	args := `{"extension_type":"behavior","name":"../../etc/passwd","title":"Evil","content":"evil"}`
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for path traversal name")
	}
}

// TestCreatePromptExtensionTool_EmptyNameAfterSanitize verifies empty names after sanitization are rejected.
func TestCreatePromptExtensionTool_EmptyNameAfterSanitize(t *testing.T) {
	tool, _, _ := newTestExtensionTool(t)
	args := `{"extension_type":"behavior","name":"...","title":"Test","content":"content"}`
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for name that sanitizes to empty")
	}
}

// TestCreatePromptExtensionTool_ResultContainsPath verifies the result JSON contains the written path.
func TestCreatePromptExtensionTool_ResultContainsPath(t *testing.T) {
	tool, _, _ := newTestExtensionTool(t)
	args := `{"extension_type":"talent","name":"testing-expert","title":"Testing Expert","content":"# Testing"}`
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "testing-expert") {
		t.Errorf("expected file name in result, got %q", result)
	}
}

// TestCreatePromptExtensionTool_Concurrent verifies the tool is safe under concurrent use.
func TestCreatePromptExtensionTool_Concurrent(t *testing.T) {
	root := t.TempDir()
	behaviorsDir := filepath.Join(root, "behaviors")
	talentsDir := filepath.Join(root, "talents")
	_ = os.MkdirAll(behaviorsDir, 0o755)
	_ = os.MkdirAll(talentsDir, 0o755)
	dirs := tools.PromptExtensionDirs{BehaviorsDir: behaviorsDir, TalentsDir: talentsDir}
	tool := CreatePromptExtensionTool(dirs)

	n := 8
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			args, _ := json.Marshal(map[string]any{
				"extension_type": "behavior",
				"name":           strings.Repeat("a", i+1),
				"title":          "Test",
				"content":        "# Content",
			})
			_, err := tool.Handler(context.Background(), json.RawMessage(args))
			errs <- err
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent write error: %v", err)
		}
	}
}
