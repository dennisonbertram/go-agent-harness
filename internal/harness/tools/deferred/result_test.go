package deferred

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tools "go-agent-harness/internal/harness/tools"
)

// TestTaskCompleteResult_MatchesChildResultSchema verifies that task_complete
// output parses as a ChildResult with all required fields populated.
func TestTaskCompleteResult_MatchesChildResultSchema(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	ctx := tools.WithForkDepth(context.Background(), 1)

	args := map[string]any{
		"summary": "Implemented JWT auth with 14 tests passing",
		"status":  "completed",
		"findings": []map[string]any{
			{"type": "file_changed", "content": "internal/auth/jwt.go created"},
			{"type": "test_result", "content": "14 passed, 0 failed"},
		},
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	result, err := tool.Handler(ctx, raw)
	require.NoError(t, err)

	// Must parse as ChildResult.
	var cr ChildResult
	require.NoError(t, json.Unmarshal([]byte(result), &cr), "task_complete output must parse as ChildResult")

	assert.Equal(t, "completed", cr.Status)
	assert.Equal(t, "Implemented JWT auth with 14 tests passing", cr.Summary)
	assert.Len(t, cr.Findings, 2)
	assert.Equal(t, "file_changed", cr.Findings[0].Type)
	assert.Equal(t, "internal/auth/jwt.go created", cr.Findings[0].Content)
}

// TestTaskCompleteResult_PartialStatus checks partial status is preserved in ChildResult.
func TestTaskCompleteResult_PartialStatus(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	ctx := tools.WithForkDepth(context.Background(), 1)

	args := map[string]any{
		"summary": "Did what was possible",
		"status":  "partial",
	}
	raw, _ := json.Marshal(args)

	result, err := tool.Handler(ctx, raw)
	require.NoError(t, err)

	var cr ChildResult
	require.NoError(t, json.Unmarshal([]byte(result), &cr))
	assert.Equal(t, "partial", cr.Status)
	assert.Equal(t, "Did what was possible", cr.Summary)
}

// TestSpawnAgentResult_MatchesChildResultSchema verifies that spawn_agent output
// parses as a ChildResult. The child called task_complete — structured result.
func TestSpawnAgentResult_MatchesChildResultSchema(t *testing.T) {
	t.Parallel()

	// Simulate a child that called task_complete with structured JSON.
	taskCompleteOutput := `{"_task_complete":true,"status":"completed","summary":"Auth module done","findings":[{"type":"test_result","content":"14 passed"}]}`
	runner := &mockSpawnForkedRunner{
		output: tools.ForkResult{Output: taskCompleteOutput},
	}
	tool := SpawnAgentTool(runner, "")

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"task":"implement auth"}`))
	require.NoError(t, err)

	// Must parse as ChildResult.
	var cr ChildResult
	require.NoError(t, json.Unmarshal([]byte(out), &cr), "spawn_agent output must parse as ChildResult")

	assert.Equal(t, "completed", cr.Status)
	assert.Equal(t, "Auth module done", cr.Summary)
	// Findings must be populated (not just jsonl).
	assert.Len(t, cr.Findings, 1)
	assert.Equal(t, "test_result", cr.Findings[0].Type)
	assert.Equal(t, "14 passed", cr.Findings[0].Content)
}

// TestSpawnAgentResult_PlainTextMatchesChildResultSchema verifies spawn_agent
// falls back gracefully when the child returns plain text (no task_complete).
func TestSpawnAgentResult_PlainTextMatchesChildResultSchema(t *testing.T) {
	t.Parallel()

	runner := &mockSpawnForkedRunner{
		output: tools.ForkResult{
			Output:  "The child finished and wrote output here",
			Summary: "",
		},
	}
	tool := SpawnAgentTool(runner, "")

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"task":"do something"}`))
	require.NoError(t, err)

	var cr ChildResult
	require.NoError(t, json.Unmarshal([]byte(out), &cr), "spawn_agent plain-text output must parse as ChildResult")

	assert.Equal(t, "completed", cr.Status)
	assert.NotEmpty(t, cr.Summary)
}

// TestSpawnAgentResult_ErrorMatchesChildResultSchema verifies spawn_agent errors
// produce a ChildResult with status=failed.
func TestSpawnAgentResult_ErrorMatchesChildResultSchema(t *testing.T) {
	t.Parallel()

	// ForkResult.Error path — parseChildResult must return ChildResult schema.
	// We test parseChildResult directly since the tool returns an error for
	// runner.RunForkedSkill failures (separate from result.Error).
	cr := parseChildResult(tools.ForkResult{Error: "child timed out"})
	raw, err := json.Marshal(cr)
	require.NoError(t, err)

	var result ChildResult
	require.NoError(t, json.Unmarshal(raw, &result), "error result must parse as ChildResult")
	assert.Equal(t, "failed", result.Status)
	assert.NotEmpty(t, result.Summary)
}

// TestRunAgentResult_MatchesChildResultSchema verifies run_agent output parses
// as a ChildResult with summary, status, output, and profile populated.
func TestRunAgentResult_MatchesChildResultSchema(t *testing.T) {
	t.Parallel()

	mgr := &mockSubagentManager{
		result: tools.SubagentResult{
			ID:     "subagent_abc",
			RunID:  "run_xyz",
			Status: "completed",
			Output: "Analysis complete. Found 3 issues in the codebase.",
		},
	}

	tool := RunAgentTool(mgr, "")
	args := map[string]any{"task": "Analyze the codebase", "profile": "researcher"}
	raw, _ := json.Marshal(args)

	out, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	// Must parse as ChildResult.
	var cr ChildResult
	require.NoError(t, json.Unmarshal([]byte(out), &cr), "run_agent output must parse as ChildResult")

	assert.Equal(t, "completed", cr.Status)
	assert.NotEmpty(t, cr.Summary, "summary must be derived from output")
	assert.Equal(t, "Analysis complete. Found 3 issues in the codebase.", cr.Output)
	assert.Equal(t, "researcher", cr.Profile)
}

// TestRunAgentResult_FailedStatus verifies run_agent status=failed maps to ChildResult.
func TestRunAgentResult_FailedStatus(t *testing.T) {
	t.Parallel()

	mgr := &mockSubagentManager{
		result: tools.SubagentResult{
			Status: "failed",
			Output: "",
			Error:  "step budget exceeded",
		},
	}

	tool := RunAgentTool(mgr, "")
	args := map[string]any{"task": "Do something"}
	raw, _ := json.Marshal(args)

	out, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	var cr ChildResult
	require.NoError(t, json.Unmarshal([]byte(out), &cr), "failed run_agent output must parse as ChildResult")
	assert.Equal(t, "failed", cr.Status)
}

// TestRunAgentResult_EmptyOutputSummary verifies that when run_agent returns
// empty output, summary is set to a non-empty default.
func TestRunAgentResult_EmptyOutputSummary(t *testing.T) {
	t.Parallel()

	mgr := &mockSubagentManager{
		result: tools.SubagentResult{
			Status: "completed",
			Output: "",
		},
	}

	tool := RunAgentTool(mgr, "")
	args := map[string]any{"task": "Do something"}
	raw, _ := json.Marshal(args)

	out, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	var cr ChildResult
	require.NoError(t, json.Unmarshal([]byte(out), &cr))
	// Summary must never be empty — even when output is empty.
	assert.NotEmpty(t, cr.Summary)
}

// TestChildResultSchema_BackwardCompatibility verifies that the ChildResult fields
// are a superset of what callers already expect (no regressions).
func TestChildResultSchema_BackwardCompatibility(t *testing.T) {
	t.Parallel()

	// spawn_agent used to return jsonl — verify it still appears in the raw output.
	taskCompleteOutput := `{"_task_complete":true,"status":"completed","summary":"done","findings":[{"type":"conclusion","content":"all good"}]}`
	runner := &mockSpawnForkedRunner{
		output: tools.ForkResult{Output: taskCompleteOutput},
	}
	tool := SpawnAgentTool(runner, "")

	out, err := tool.Handler(context.Background(), json.RawMessage(`{"task":"test"}`))
	require.NoError(t, err)

	// Backward compat: jsonl key must still be present for existing callers.
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &raw))
	_, hasJSONL := raw["jsonl"]
	assert.True(t, hasJSONL, "jsonl field must be present for backward compatibility")

	// New field: findings must also be present.
	_, hasFindings := raw["findings"]
	assert.True(t, hasFindings, "findings field must be present in unified schema")
}

// TestRunAgentResult_BackwardCompatibility verifies run_agent still returns
// run_id and profile fields that existing callers may depend on.
func TestRunAgentResult_BackwardCompatibility(t *testing.T) {
	t.Parallel()

	mgr := &mockSubagentManager{
		result: tools.SubagentResult{
			ID:     "subagent_abc",
			RunID:  "run_xyz",
			Status: "completed",
			Output: "done",
		},
	}

	tool := RunAgentTool(mgr, "")
	args := map[string]any{"task": "Do something", "profile": "github"}
	raw, _ := json.Marshal(args)

	out, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))

	// Old fields must still exist.
	assert.Equal(t, "run_xyz", result["run_id"])
	assert.Equal(t, "github", result["profile"])
	assert.NotNil(t, result["output"])

	// New unified fields must also exist.
	assert.NotNil(t, result["summary"])
	assert.NotNil(t, result["status"])
}
