package deferred

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go-agent-harness/internal/harness/tools"
)

func TestStartSubagentTool_CreatesAndReturnsSubagentID(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{
			ID:     "subagent-1",
			RunID:  "run-1",
			Status: "running",
		},
	}
	tool := StartSubagentTool(manager, "")

	raw, _ := json.Marshal(map[string]any{
		"task":      "Handle refactor",
		"profile":   "full",
		"model":     "gpt-4.1-mini",
		"max_steps": 12,
	})
	got, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &payload))
	assert.Equal(t, "subagent-1", payload["subagent_id"])
	assert.Equal(t, "running", payload["status"])
	assert.Equal(t, "run-1", payload["run_id"])
	assert.Equal(t, 12, manager.lastReq.MaxSteps)
	assert.True(t, manager.startCalled)
}

func TestGetSubagentTool_ReturnsSubagentStatus(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{
			ID:     "subagent-1",
			Status: "running",
			Output: "working",
		},
	}
	tool := GetSubagentTool(manager)

	raw, _ := json.Marshal(map[string]any{"id": "subagent-1"})
	got, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &payload))
	assert.Equal(t, "subagent-1", payload["id"])
	assert.Equal(t, "running", payload["status"])
	assert.Equal(t, "working", payload["output"])
	assert.True(t, manager.getCalled)
}

func TestWaitSubagentTool_ReturnsTerminalSubagent(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{
			ID:     "subagent-1",
			Status: "completed",
			Output: "done",
		},
	}
	tool := WaitSubagentTool(manager)

	raw, _ := json.Marshal(map[string]any{"id": "subagent-1"})
	got, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &payload))
	assert.Equal(t, "completed", payload["status"])
	assert.Equal(t, "done", payload["output"])
	assert.True(t, manager.waitCalled)
}

func TestCancelSubagentTool_CallsManagerCancel(t *testing.T) {
	manager := &mockSubagentManager{
		result: tools.SubagentResult{},
		err:    nil,
	}

	tool := CancelSubagentTool(manager)
	raw, _ := json.Marshal(map[string]any{"id": "subagent-1"})
	got, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &payload))
	assert.Equal(t, "cancelling", payload["status"])
	assert.True(t, manager.cancelCalled)
}
