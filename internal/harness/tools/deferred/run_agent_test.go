package deferred

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSubagentManager implements tools.SubagentManager for testing.
type mockSubagentManager struct {
	lastReq tools.SubagentRequest
	result  tools.SubagentResult
	err     error
}

func (m *mockSubagentManager) CreateAndWait(_ context.Context, req tools.SubagentRequest) (tools.SubagentResult, error) {
	m.lastReq = req
	return m.result, m.err
}

func TestRunAgentTool_BasicExecution(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{
			ID:     "subagent_abc123",
			RunID:  "run_xyz",
			Status: "completed",
			Output: "Task completed successfully",
		},
	}

	tool := RunAgentTool(manager, "")
	assert.Equal(t, "run_agent", tool.Definition.Name)
	assert.Equal(t, tools.TierDeferred, tool.Definition.Tier)

	args := map[string]any{
		"task": "Do something useful",
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)
	assert.Contains(t, result, "completed")
	assert.Contains(t, result, "run_xyz")
}

func TestRunAgentTool_DefaultsToFullProfile(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{Status: "completed", Output: "done"},
	}
	tool := RunAgentTool(manager, "")

	args := map[string]any{"task": "Do something"}
	raw, _ := json.Marshal(args)
	_, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, "full", manager.lastReq.ProfileName)
}

func TestRunAgentTool_AppliesProfileValues(t *testing.T) {
	dir := t.TempDir()

	// Write a custom profile.
	profileContent := `
[meta]
name = "fast"
description = "Fast profile"
created_by = "user"

[runner]
model = "gpt-4.1-mini"
max_steps = 5
max_cost_usd = 0.05
system_prompt = "Be fast."

[tools]
allow = ["bash", "read"]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fast.toml"), []byte(profileContent), 0644))

	manager := &mockSubagentManager{
		result: tools.SubagentResult{Status: "completed", Output: "done"},
	}
	tool := RunAgentTool(manager, dir)

	args := map[string]any{
		"task":    "Do something fast",
		"profile": "fast",
	}
	raw, _ := json.Marshal(args)
	_, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	// Profile values should be applied.
	assert.Equal(t, "fast", manager.lastReq.ProfileName)
	assert.Equal(t, "gpt-4.1-mini", manager.lastReq.Model)
	assert.Equal(t, 5, manager.lastReq.MaxSteps)
	assert.Equal(t, 0.05, manager.lastReq.MaxCostUSD)
	assert.Equal(t, "Be fast.", manager.lastReq.SystemPrompt)
	assert.Equal(t, []string{"bash", "read"}, manager.lastReq.AllowedTools)
}

func TestRunAgentTool_OverridesProfileModel(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{Status: "completed", Output: "done"},
	}
	tool := RunAgentTool(manager, "")

	args := map[string]any{
		"task":  "Do something",
		"model": "claude-opus-4-6",
	}
	raw, _ := json.Marshal(args)
	_, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	assert.Equal(t, "claude-opus-4-6", manager.lastReq.Model)
}

func TestRunAgentTool_OverridesProfileMaxSteps(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{Status: "completed", Output: "done"},
	}
	tool := RunAgentTool(manager, "")

	args := map[string]any{
		"task":      "Do something",
		"max_steps": 99,
	}
	raw, _ := json.Marshal(args)
	_, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	assert.Equal(t, 99, manager.lastReq.MaxSteps)
}

func TestRunAgentTool_RequiresTask(t *testing.T) {
	manager := &mockSubagentManager{}
	tool := RunAgentTool(manager, "")

	args := map[string]any{"profile": "github"}
	raw, _ := json.Marshal(args)
	_, err := tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task is required")
}

func TestRunAgentTool_NilManagerReturnsError(t *testing.T) {
	tool := RunAgentTool(nil, "")

	args := map[string]any{"task": "Do something"}
	raw, _ := json.Marshal(args)
	_, err := tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestRunAgentTool_BuiltinProfileGithub(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{Status: "completed", Output: "done"},
	}
	tool := RunAgentTool(manager, "")

	args := map[string]any{
		"task":    "Close stale issues",
		"profile": "github",
	}
	raw, _ := json.Marshal(args)
	_, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	// github built-in has max_steps=20 and allow=["bash","read"]
	assert.Equal(t, "github", manager.lastReq.ProfileName)
	assert.Equal(t, 20, manager.lastReq.MaxSteps)
	assert.Equal(t, []string{"bash", "read"}, manager.lastReq.AllowedTools)
}

// TestRunAgentTool_UnknownProfile verifies that run_agent returns an explicit error
// when a non-existent profile is requested, rather than silently falling back to
// an empty default profile.
func TestRunAgentTool_UnknownProfile(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{Status: "completed", Output: "done"},
	}
	// Use a temp dir that exists but has no profiles — ensures the profile name
	// is valid but truly not found anywhere.
	dir := t.TempDir()
	tool := RunAgentTool(manager, dir)

	args := map[string]any{
		"task":    "Do something",
		"profile": "nonexistent-profile-abc123",
	}
	raw, _ := json.Marshal(args)
	_, err := tool.Handler(context.Background(), raw)
	require.Error(t, err, "expected error for unknown profile, got nil")
	assert.Contains(t, err.Error(), "nonexistent-profile-abc123")
}

// TestRunAgentTool_InvalidProfileNamePathTraversal verifies that run_agent returns
// an explicit error when the profile name contains path traversal sequences.
func TestRunAgentTool_InvalidProfileNamePathTraversal(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{Status: "completed", Output: "done"},
	}
	tool := RunAgentTool(manager, "")

	args := map[string]any{
		"task":    "Do something",
		"profile": "../etc/passwd",
	}
	raw, _ := json.Marshal(args)
	_, err := tool.Handler(context.Background(), raw)
	require.Error(t, err, "expected error for path traversal profile name, got nil")
}

// TestRunAgentTool_InvalidProfileNameEmpty verifies that run_agent returns an
// explicit error when profile resolves to empty after trimming.
// Note: empty profile defaults to "full" which is a valid built-in; this test
// uses a profile consisting entirely of whitespace which has no default.
func TestRunAgentTool_BrokenToml(t *testing.T) {
	dir := t.TempDir()

	// Write a profile file with invalid TOML content.
	brokenContent := `[meta
name = "broken"  # missing closing bracket — invalid TOML
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.toml"), []byte(brokenContent), 0644))

	manager := &mockSubagentManager{
		result: tools.SubagentResult{Status: "completed", Output: "done"},
	}
	tool := RunAgentTool(manager, dir)

	args := map[string]any{
		"task":    "Do something",
		"profile": "broken",
	}
	raw, _ := json.Marshal(args)
	_, err := tool.Handler(context.Background(), raw)
	require.Error(t, err, "expected error for broken TOML profile, got nil")
	assert.Contains(t, err.Error(), "broken")
}
