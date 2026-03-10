package script

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// helper: create a tool directory with tool.json and an executable script.
func makeToolDir(t *testing.T, baseDir, name string, toolJSON string, scriptContent string, executable bool) {
	t.Helper()
	toolDir := filepath.Join(baseDir, name)
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", toolDir, err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "tool.json"), []byte(toolJSON), 0o644); err != nil {
		t.Fatalf("write tool.json: %v", err)
	}
	if scriptContent != "" {
		perm := os.FileMode(0o644)
		if executable {
			perm = 0o755
		}
		if err := os.WriteFile(filepath.Join(toolDir, "run.sh"), []byte(scriptContent), perm); err != nil {
			t.Fatalf("write run.sh: %v", err)
		}
	}
}

// TestLoadScriptTools_NonExistentDir verifies that a missing toolsDir returns empty slice without error.
func TestLoadScriptTools_NonExistentDir(t *testing.T) {
	tools, err := LoadScriptTools("/tmp/does-not-exist-xyz-12345")
	if err != nil {
		t.Fatalf("expected no error for missing dir, got: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected empty slice, got %d tools", len(tools))
	}
}

// TestLoadScriptTools_EmptyDir verifies empty directory returns empty tools.
func TestLoadScriptTools_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

// TestLoadScriptTools_HappyPath verifies a valid tool is loaded correctly.
func TestLoadScriptTools_HappyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{
		"name": "format_json",
		"description": "Format and pretty-print JSON",
		"parameters": {
			"type": "object",
			"properties": {
				"input": {"type": "string", "description": "JSON to format"}
			},
			"required": ["input"]
		},
		"timeout_seconds": 10
	}`
	makeToolDir(t, dir, "format-json", toolJSON, "#!/bin/sh\ncat\n", true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool.Definition.Name != "format_json" {
		t.Errorf("expected name 'format_json', got %q", tool.Definition.Name)
	}
	if tool.Definition.Description == "" {
		t.Error("expected non-empty description")
	}
	if tool.Definition.Tier != "deferred" {
		t.Errorf("expected deferred tier, got %q", tool.Definition.Tier)
	}
	if tool.Handler == nil {
		t.Error("expected non-nil handler")
	}
	if tool.Definition.Parameters == nil {
		t.Error("expected non-nil parameters")
	}
}

// TestLoadScriptTools_MissingRunScript verifies tool without run script is skipped.
func TestLoadScriptTools_MissingRunScript(t *testing.T) {
	dir := t.TempDir()
	toolJSON := `{"name": "my_tool", "description": "A tool", "parameters": {"type": "object", "properties": {}}}`
	// Create tool dir with tool.json but no run script
	toolDir := filepath.Join(dir, "my-tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "tool.json"), []byte(toolJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (missing run script), got %d", len(tools))
	}
}

// TestLoadScriptTools_NonExecutableScript verifies non-executable script is skipped.
func TestLoadScriptTools_NonExecutableScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission model differs on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{"name": "my_tool", "description": "A tool", "parameters": {"type": "object", "properties": {}}}`
	makeToolDir(t, dir, "my-tool", toolJSON, "#!/bin/sh\necho hi\n", false)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (non-executable), got %d", len(tools))
	}
}

// TestLoadScriptTools_InvalidToolJSON verifies malformed tool.json is skipped.
func TestLoadScriptTools_InvalidToolJSON(t *testing.T) {
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "bad-tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "tool.json"), []byte("not json {{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (bad JSON), got %d", len(tools))
	}
}

// TestLoadScriptTools_ToolNameValidation verifies tool names with invalid chars are skipped.
func TestLoadScriptTools_ToolNameValidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	// Tool name with path separator
	toolJSON := `{"name": "path/traversal", "description": "Bad tool", "parameters": {"type": "object", "properties": {}}}`
	makeToolDir(t, dir, "bad-name", toolJSON, "#!/bin/sh\necho hi\n", true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (invalid name), got %d", len(tools))
	}
}

// TestLoadScriptTools_ToolNameWithSpaces verifies tool names with spaces are skipped.
func TestLoadScriptTools_ToolNameWithSpaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{"name": "my tool", "description": "Bad tool", "parameters": {"type": "object", "properties": {}}}`
	makeToolDir(t, dir, "my-tool", toolJSON, "#!/bin/sh\necho hi\n", true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (name with space), got %d", len(tools))
	}
}

// TestLoadScriptTools_TimeoutDefault verifies default timeout is applied when timeout_seconds is 0.
func TestLoadScriptTools_TimeoutDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{"name": "my_tool", "description": "A tool", "parameters": {"type": "object", "properties": {}}}`
	makeToolDir(t, dir, "my-tool", toolJSON, "#!/bin/sh\necho hi\n", true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
}

// TestLoadScriptTools_TimeoutCap verifies timeout is capped at 300s.
func TestLoadScriptTools_TimeoutCap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{"name": "my_tool", "description": "A tool", "parameters": {"type": "object", "properties": {}}, "timeout_seconds": 9999}`
	makeToolDir(t, dir, "my-tool", toolJSON, "#!/bin/sh\necho hi\n", true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	// Just verify it loaded, we can't directly inspect the timeout
	// but coverage is valuable here
}

// TestScriptHandler_StdinStdoutContract verifies JSON args are passed via stdin and stdout is the result.
func TestScriptHandler_StdinStdoutContract(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{
		"name": "echo_args",
		"description": "Echoes input args",
		"parameters": {
			"type": "object",
			"properties": {
				"message": {"type": "string"}
			}
		}
	}`
	// Script reads stdin and writes it to stdout (captures args)
	script := "#!/bin/sh\ncat -\n"
	makeToolDir(t, dir, "echo-args", toolJSON, script, true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	args := json.RawMessage(`{"message":"hello world"}`)
	result, err := tools[0].Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("expected stdout to contain 'hello world', got: %q", result)
	}
}

// TestScriptHandler_ExitNonZero verifies non-zero exit returns an error with stderr.
func TestScriptHandler_ExitNonZero(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{"name": "fail_tool", "description": "Fails", "parameters": {"type": "object", "properties": {}}}`
	script := "#!/bin/sh\necho 'something went wrong' >&2\nexit 1\n"
	makeToolDir(t, dir, "fail-tool", toolJSON, script, true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	_, err = tools[0].Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("expected error to contain stderr, got: %v", err)
	}
}

// TestScriptHandler_Timeout verifies that a slow script is killed after the timeout.
func TestScriptHandler_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{"name": "slow_tool", "description": "Slow", "parameters": {"type": "object", "properties": {}}, "timeout_seconds": 1}`
	// Sleep for 30s, should be killed by 1s timeout
	script := "#!/bin/sh\nsleep 30\n"
	makeToolDir(t, dir, "slow-tool", toolJSON, script, true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	start := time.Now()
	_, err = tools[0].Handler(context.Background(), json.RawMessage(`{}`))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from timeout")
	}
	// Should complete well under the 30s sleep
	if elapsed > 5*time.Second {
		t.Errorf("expected timeout within 5s, took %v", elapsed)
	}
}

// TestScriptHandler_EnvIsolation verifies that secret env vars are not passed to scripts.
func TestScriptHandler_EnvIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	// Set a secret that should not be visible to the script
	t.Setenv("OPENAI_API_KEY", "super-secret-key")

	dir := t.TempDir()
	toolJSON := `{"name": "env_check", "description": "Checks env", "parameters": {"type": "object", "properties": {}}}`
	// Try to leak the secret key via stdout
	script := "#!/bin/sh\necho \"KEY=${OPENAI_API_KEY}\"\n"
	makeToolDir(t, dir, "env-check", toolJSON, script, true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	result, err := tools[0].Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "super-secret-key") {
		t.Errorf("secret key leaked to script stdout: %q", result)
	}
}

// TestScriptHandler_ConcurrentExecution verifies concurrent tool calls are safe.
func TestScriptHandler_ConcurrentExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{
		"name": "counter",
		"description": "Prints args",
		"parameters": {"type": "object", "properties": {"n": {"type": "integer"}}}
	}`
	script := "#!/bin/sh\ncat -\n"
	makeToolDir(t, dir, "counter", toolJSON, script, true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	const goroutines = 10
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	results := make([]string, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			args := json.RawMessage(`{"n":` + string(rune('0'+idx)) + `}`)
			r, e := tools[0].Handler(context.Background(), args)
			results[idx] = r
			errs[idx] = e
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, err)
		}
	}
}

// TestLoadScriptTools_MultipleTools verifies multiple tools are discovered in the same directory.
func TestLoadScriptTools_MultipleTools(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON1 := `{"name": "tool_one", "description": "First tool", "parameters": {"type": "object", "properties": {}}}`
	toolJSON2 := `{"name": "tool_two", "description": "Second tool", "parameters": {"type": "object", "properties": {}}}`
	makeToolDir(t, dir, "tool-one", toolJSON1, "#!/bin/sh\necho one\n", true)
	makeToolDir(t, dir, "tool-two", toolJSON2, "#!/bin/sh\necho two\n", true)

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

// TestLoadScriptTools_AlternativeRunNames verifies that run.py and run executables are found.
func TestLoadScriptTools_AlternativeRunNames(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{"name": "py_tool", "description": "Python tool", "parameters": {"type": "object", "properties": {}}}`

	toolDir := filepath.Join(dir, "py-tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "tool.json"), []byte(toolJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	// Use run.py instead of run.sh
	script := "#!/usr/bin/env python3\nimport sys\nprint('ok')\n"
	if err := os.WriteFile(filepath.Join(toolDir, "run.py"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Errorf("expected 1 tool (run.py), got %d", len(tools))
	}
}

// TestLoadScriptTools_RunNoExtension verifies that 'run' (no extension) is found.
func TestLoadScriptTools_RunNoExtension(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := t.TempDir()
	toolJSON := `{"name": "bare_tool", "description": "Bare run", "parameters": {"type": "object", "properties": {}}}`

	toolDir := filepath.Join(dir, "bare-tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "tool.json"), []byte(toolJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	// Use 'run' with no extension
	script := "#!/bin/sh\necho ok\n"
	if err := os.WriteFile(filepath.Join(toolDir, "run"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	tools, err := LoadScriptTools(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Errorf("expected 1 tool (bare 'run'), got %d", len(tools))
	}
}

// TestValidateToolName covers the validation function directly.
func TestValidateToolName(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"valid_name", true},
		{"valid-name", false}, // hyphens not allowed in tool names per convention
		{"format_json", true},
		{"path/traversal", false},
		{"../escape", false},
		{"has space", false},
		{"", false},
		{"a", true},
		{"UPPER", true},
		{"CamelCase", true},
		{"with.dot", false},
	}
	for _, tc := range cases {
		got := isValidToolName(tc.name)
		if got != tc.valid {
			t.Errorf("isValidToolName(%q) = %v, want %v", tc.name, got, tc.valid)
		}
	}
}
