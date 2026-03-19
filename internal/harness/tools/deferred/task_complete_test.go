package deferred

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tools "go-agent-harness/internal/harness/tools"
)

func TestTaskCompleteTool_BasicCompletion(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	assert.Equal(t, "task_complete", tool.Definition.Name)
	assert.Equal(t, tools.TierDeferred, tool.Definition.Tier)

	// Run at depth > 0 (subagent context).
	ctx := tools.WithForkDepth(context.Background(), 1)

	args := map[string]any{
		"summary": "Task accomplished",
		"status":  "completed",
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	result, err := tool.Handler(ctx, raw)
	require.NoError(t, err)
	assert.Contains(t, result, "completed")
	assert.Contains(t, result, "Task accomplished")
}

func TestTaskCompleteTool_DefaultsStatusToCompleted(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	ctx := tools.WithForkDepth(context.Background(), 1)

	args := map[string]any{
		"summary": "Done without explicit status",
	}
	raw, _ := json.Marshal(args)

	result, err := tool.Handler(ctx, raw)
	require.NoError(t, err)
	assert.Contains(t, result, "completed")
}

func TestTaskCompleteTool_PartialStatus(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	ctx := tools.WithForkDepth(context.Background(), 2)

	args := map[string]any{
		"summary": "Did some but not all",
		"status":  "partial",
	}
	raw, _ := json.Marshal(args)

	result, err := tool.Handler(ctx, raw)
	require.NoError(t, err)
	assert.Contains(t, result, "partial")
}

func TestTaskCompleteTool_FailedStatus(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	ctx := tools.WithForkDepth(context.Background(), 1)

	args := map[string]any{
		"summary": "Could not complete",
		"status":  "failed",
	}
	raw, _ := json.Marshal(args)

	result, err := tool.Handler(ctx, raw)
	require.NoError(t, err)
	assert.Contains(t, result, "failed")
}

func TestTaskCompleteTool_WithFindings(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	ctx := tools.WithForkDepth(context.Background(), 1)

	args := map[string]any{
		"summary": "Completed with findings",
		"status":  "completed",
		"findings": []map[string]any{
			{"type": "finding", "content": "Important discovery"},
			{"type": "file_changed", "content": "internal/foo.go created"},
		},
	}
	raw, _ := json.Marshal(args)

	result, err := tool.Handler(ctx, raw)
	require.NoError(t, err)
	assert.Contains(t, result, "_task_complete")
}

func TestTaskCompleteTool_RejectsAtRootDepth(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	// depth == 0 means root agent — task_complete must be rejected.
	ctx := tools.WithForkDepth(context.Background(), 0)

	args := map[string]any{
		"summary": "Should fail at root",
	}
	raw, _ := json.Marshal(args)

	_, err := tool.Handler(ctx, raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth > 0")
}

func TestTaskCompleteTool_RejectsEmptySummary(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	ctx := tools.WithForkDepth(context.Background(), 1)

	args := map[string]any{
		"summary": "   ", // whitespace only
	}
	raw, _ := json.Marshal(args)

	_, err := tool.Handler(ctx, raw)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "summary")
}

func TestTaskCompleteTool_RejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	ctx := tools.WithForkDepth(context.Background(), 1)

	args := map[string]any{
		"summary": "Valid summary",
		"status":  "invalid_status",
	}
	raw, _ := json.Marshal(args)

	_, err := tool.Handler(ctx, raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestTaskCompleteTool_RejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	ctx := tools.WithForkDepth(context.Background(), 1)

	_, err := tool.Handler(ctx, json.RawMessage(`{not valid json`))
	require.Error(t, err)
}

func TestTaskCompleteTool_OutputContainsTaskCompleteMarker(t *testing.T) {
	t.Parallel()

	tool := TaskCompleteTool(&mockAgentRunner{})
	ctx := tools.WithForkDepth(context.Background(), 1)

	args := map[string]any{
		"summary": "Marker check",
		"status":  "completed",
	}
	raw, _ := json.Marshal(args)

	result, err := tool.Handler(ctx, raw)
	require.NoError(t, err)

	// The output must contain the _task_complete sentinel so spawn_agent can detect it.
	assert.Contains(t, result, "_task_complete")
	assert.Contains(t, result, "true")
}
