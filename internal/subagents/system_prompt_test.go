package subagents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-agent-harness/internal/harness"
	tools "go-agent-harness/internal/harness/tools"
)

// captureRunEngine is a RunEngine that captures the last RunRequest.
type captureRunEngine struct {
	lastReq harness.RunRequest
}

func (c *captureRunEngine) StartRun(req harness.RunRequest) (harness.Run, error) {
	c.lastReq = req
	return harness.Run{
		ID:     "test-run-id",
		Status: harness.RunStatusQueued,
	}, nil
}

func (c *captureRunEngine) GetRun(runID string) (harness.Run, bool) {
	return harness.Run{
		ID:     runID,
		Status: harness.RunStatusCompleted,
		Output: "done",
	}, true
}

func (c *captureRunEngine) Subscribe(runID string) ([]harness.Event, <-chan harness.Event, func(), error) {
	ch := make(chan harness.Event)
	close(ch)
	return []harness.Event{
		{Type: harness.EventRunCompleted},
	}, ch, func() {}, nil
}

func TestRequestSystemPromptForwarded(t *testing.T) {
	engine := &captureRunEngine{}

	mgr, err := NewManager(Options{
		InlineRunner: engine,
	})
	require.NoError(t, err)

	req := Request{
		Prompt:       "Do something",
		SystemPrompt: "Be a helpful specialist.",
		Isolation:    IsolationInline,
	}

	_, err = mgr.Create(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, "Be a helpful specialist.", engine.lastReq.SystemPrompt,
		"SystemPrompt should be forwarded to RunRequest")
}

func TestRequestSystemPromptEmpty(t *testing.T) {
	engine := &captureRunEngine{}

	mgr, err := NewManager(Options{
		InlineRunner: engine,
	})
	require.NoError(t, err)

	req := Request{
		Prompt:    "Do something",
		Isolation: IsolationInline,
	}

	_, err = mgr.Create(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, "", engine.lastReq.SystemPrompt,
		"Empty SystemPrompt should forward as empty string")
}

func TestRequestSystemPromptTrimsSpace(t *testing.T) {
	engine := &captureRunEngine{}

	mgr, err := NewManager(Options{
		InlineRunner: engine,
	})
	require.NoError(t, err)

	req := Request{
		Prompt:       "Do something",
		SystemPrompt: "  Leading and trailing spaces.  ",
		Isolation:    IsolationInline,
	}

	_, err = mgr.Create(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, "Leading and trailing spaces.", engine.lastReq.SystemPrompt)
}

// TestNewInlineManager verifies that NewInlineManager wraps a Manager correctly.
func TestNewInlineManager(t *testing.T) {
	engine := &captureRunEngine{}
	mgr, err := NewManager(Options{InlineRunner: engine})
	require.NoError(t, err)

	im := NewInlineManager(mgr)
	require.NotNil(t, im)
}

// TestInlineManagerCreateAndWait verifies that CreateAndWait creates a subagent
// and returns when it reaches a terminal status.
func TestInlineManagerCreateAndWait(t *testing.T) {
	engine := &captureRunEngine{}
	mgr, err := NewManager(Options{InlineRunner: engine})
	require.NoError(t, err)

	im := NewInlineManager(mgr)

	req := tools.SubagentRequest{
		Prompt:   "Do a thing",
		Model:    "gpt-4.1-mini",
		MaxSteps: 5,
	}

	result, err := im.CreateAndWait(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "done", result.Output)
}

// TestInlineManagerCreateAndWaitSystemPrompt verifies that the SystemPrompt is
// forwarded through CreateAndWait to the underlying RunRequest.
func TestInlineManagerCreateAndWaitSystemPrompt(t *testing.T) {
	engine := &captureRunEngine{}
	mgr, err := NewManager(Options{InlineRunner: engine})
	require.NoError(t, err)

	im := NewInlineManager(mgr)

	req := tools.SubagentRequest{
		Prompt:       "Do a thing",
		SystemPrompt: "Be specialized.",
	}

	_, err = im.CreateAndWait(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "Be specialized.", engine.lastReq.SystemPrompt)
}
