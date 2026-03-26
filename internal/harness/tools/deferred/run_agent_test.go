package deferred

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tools "go-agent-harness/internal/harness/tools"
)

type runAgentTranscriptReaderStub struct{}

func (runAgentTranscriptReaderStub) Snapshot(limit int, includeTools bool) tools.TranscriptSnapshot {
	return tools.TranscriptSnapshot{
		RunID: "run_parent",
		Messages: []tools.TranscriptMessage{{
			Index:   1,
			Role:    "user",
			Content: "Please investigate the parser failure before editing.",
		}},
		GeneratedAt: time.Now().UTC(),
	}
}

// mockSubagentManager implements tools.SubagentManager for testing.
type mockSubagentManager struct {
	lastReq tools.SubagentRequest
	result  tools.SubagentResult
	err     error

	startCalled  bool
	getCalled    bool
	waitCalled   bool
	cancelCalled bool

	cancelErr error
}

func (m *mockSubagentManager) CreateAndWait(_ context.Context, req tools.SubagentRequest) (tools.SubagentResult, error) {
	m.lastReq = req
	return m.result, m.err
}

func (m *mockSubagentManager) Start(_ context.Context, req tools.SubagentRequest) (tools.SubagentResult, error) {
	m.startCalled = true
	m.lastReq = req
	return m.result, m.err
}

func (m *mockSubagentManager) Get(_ context.Context, _ string) (tools.SubagentResult, error) {
	m.getCalled = true
	return m.result, m.err
}

func (m *mockSubagentManager) Wait(_ context.Context, _ string) (tools.SubagentResult, error) {
	m.waitCalled = true
	return m.result, m.err
}

func (m *mockSubagentManager) Cancel(_ context.Context, _ string) error {
	m.cancelCalled = true
	if m.cancelErr != nil {
		return m.cancelErr
	}
	return m.err
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

func TestRunAgentTool_AppliesProfileRuntimeDefaults(t *testing.T) {
	dir := t.TempDir()

	profileContent := `
isolation_mode = "worktree"
cleanup_policy = "delete_on_success"
base_ref = "release/test-base"

[meta]
name = "runtime-defaults"
description = "Runtime defaults profile"
created_by = "user"

[runner]
model = "gpt-4.1-mini"
max_steps = 5
reasoning_effort = "high"

[tools]
allow = ["bash", "read"]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "runtime-defaults.toml"), []byte(profileContent), 0o644))

	manager := &mockSubagentManager{
		result: tools.SubagentResult{Status: "completed", Output: "done"},
	}
	tool := RunAgentTool(manager, dir)

	raw, _ := json.Marshal(map[string]any{
		"task":    "Use profile runtime defaults",
		"profile": "runtime-defaults",
	})
	_, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	assert.Equal(t, "high", manager.lastReq.ReasoningEffort)
	assert.Equal(t, "worktree", manager.lastReq.IsolationMode)
	assert.Equal(t, "delete_on_success", manager.lastReq.CleanupPolicy)
	assert.Equal(t, "release/test-base", manager.lastReq.BaseRef)
}

func TestRunAgentTool_ForwardsParentContextHandoff(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{Status: "completed", Output: "done"},
	}
	tool := RunAgentTool(manager, "")

	ctx := context.Background()
	ctx = context.WithValue(ctx, tools.ContextKeyRunMetadata, tools.RunMetadata{
		RunID:          "run_parent",
		TenantID:       "tenant_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
	})
	ctx = context.WithValue(ctx, tools.ContextKeyTranscriptReader, runAgentTranscriptReaderStub{})

	raw, _ := json.Marshal(map[string]any{"task": "Fix the parser"})
	_, err := tool.Handler(ctx, raw)
	require.NoError(t, err)

	require.NotNil(t, manager.lastReq.ParentContextHandoff)
	assert.Equal(t, "run_parent", manager.lastReq.ParentContextHandoff.ParentRunID)
	assert.Contains(t, manager.lastReq.Prompt, "# Parent context handoff")
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
