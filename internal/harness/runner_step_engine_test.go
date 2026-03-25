package harness

import (
	"sync"
	"testing"
)

// TestRunnerStepLoop_SteeringDrainBeforeTurnRequest characterizes the current
// step-boundary contract: a steering message is drained at the top of the next
// step, before the provider sees the next llm.turn.requested turn.
func TestRunnerStepLoop_SteeringDrainBeforeTurnRequest(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var capturedMessages [][]Message

	blockDuringFirst := make(chan struct{})
	releaseDuringFirst := make(chan struct{})

	provider := &steerGatingProvider{
		turns: []CompletionResult{
			{ToolCalls: []ToolCall{{ID: "tc1", Name: "noop_step_engine", Arguments: `{}`}}},
			{Content: "done"},
		},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockDuringFirst)
				<-releaseDuringFirst
			}
		},
		afterCall: func(_ int, req CompletionRequest) {
			mu.Lock()
			capturedMessages = append(capturedMessages, append([]Message(nil), req.Messages...))
			mu.Unlock()
		},
	}

	registry := NewRegistry()
	registerNoopTool(t, registry, "noop_step_engine")

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	<-blockDuringFirst

	if err := runner.SteerRun(run.ID, "please focus on the main issue"); err != nil {
		t.Fatalf("SteerRun: %v", err)
	}

	close(releaseDuringFirst)

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	stepTwoStart := -1
	stepTwoTurnRequested := -1
	steeringReceived := -1
	stepTwoCompleted := -1
	for i, ev := range events {
		switch ev.Type {
		case EventRunStepStarted:
			if step, _ := ev.Payload["step"].(int); step == 2 && stepTwoStart == -1 {
				stepTwoStart = i
			}
		case EventSteeringReceived:
			if steeringReceived == -1 {
				steeringReceived = i
			}
		case EventLLMTurnRequested:
			if step, _ := ev.Payload["step"].(int); step == 2 && stepTwoTurnRequested == -1 {
				stepTwoTurnRequested = i
			}
		case EventRunStepCompleted:
			if step, _ := ev.Payload["step"].(int); step == 2 && stepTwoCompleted == -1 {
				stepTwoCompleted = i
			}
		}
	}
	if stepTwoStart == -1 || steeringReceived == -1 || stepTwoTurnRequested == -1 || stepTwoCompleted == -1 {
		t.Fatalf("missing step 2 boundary events: stepTwoStart=%d steeringReceived=%d stepTwoTurnRequested=%d stepTwoCompleted=%d events=%v",
			stepTwoStart, steeringReceived, stepTwoTurnRequested, stepTwoCompleted, eventTypes(events))
	}
	if !(stepTwoStart < steeringReceived && steeringReceived < stepTwoTurnRequested && stepTwoTurnRequested < stepTwoCompleted) {
		t.Fatalf("unexpected step 2 boundary ordering: start=%d steering=%d turn=%d completed=%d events=%v",
			stepTwoStart, steeringReceived, stepTwoTurnRequested, stepTwoCompleted, eventTypes(events))
	}

	mu.Lock()
	defer mu.Unlock()
	if len(capturedMessages) < 2 {
		t.Fatalf("expected at least 2 LLM calls, got %d", len(capturedMessages))
	}

	secondCallMsgs := capturedMessages[1]
	found := false
	for _, msg := range secondCallMsgs {
		if msg.Role == "user" && msg.Content == "please focus on the main issue" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("steering message not found in second LLM call messages: %v", secondCallMsgs)
	}
}
