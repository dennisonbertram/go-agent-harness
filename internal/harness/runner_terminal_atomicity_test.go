package harness

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestTerminalStatusNeverPrecedesTerminalReplayEvent(t *testing.T) {
	tests := []struct {
		name          string
		provider      Provider
		wantStatus    RunStatus
		wantEvent     EventType
		cancelStarted <-chan struct{}
	}{
		{
			name:       "completed",
			provider:   staticContentProvider{content: "done"},
			wantStatus: RunStatusCompleted,
			wantEvent:  EventRunCompleted,
		},
		{
			name:       "failed_with_error_context",
			provider:   &errorProvider{err: errors.New("provider unavailable")},
			wantStatus: RunStatusFailed,
			wantEvent:  EventRunFailed,
		},
	}

	cancelProvider := &terminalCancellationProvider{started: make(chan struct{})}
	tests = append(tests, struct {
		name          string
		provider      Provider
		wantStatus    RunStatus
		wantEvent     EventType
		cancelStarted <-chan struct{}
	}{
		name:          "cancelled",
		provider:      cancelProvider,
		wantStatus:    RunStatusCancelled,
		wantEvent:     EventRunCancelled,
		cancelStarted: cancelProvider.started,
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reached := make(chan struct{})
			release := make(chan struct{})
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
			})

			runner := NewRunner(tt.provider, NewRegistry(), RunnerConfig{
				ErrorChainEnabled:  true,
				CausalGraphEnabled: true,
			})
			runner.terminalTransitionHook = func(_ string, status RunStatus, eventType EventType) {
				if status != tt.wantStatus || eventType != tt.wantEvent {
					return
				}
				close(reached)
				<-release
			}

			run, err := runner.StartRun(RunRequest{Prompt: "terminal atomicity " + tt.name})
			if err != nil {
				t.Fatalf("StartRun: %v", err)
			}
			if tt.cancelStarted != nil {
				select {
				case <-tt.cancelStarted:
				case <-time.After(2 * time.Second):
					t.Fatal("provider did not start")
				}
				if err := runner.CancelRun(run.ID); err != nil {
					t.Fatalf("CancelRun: %v", err)
				}
			}

			select {
			case <-reached:
			case <-time.After(2 * time.Second):
				t.Fatalf("terminal transition did not reach %s barrier", tt.wantStatus)
			}

			current, ok := runner.GetRun(run.ID)
			if !ok {
				t.Fatalf("GetRun(%q) returned not found", run.ID)
			}
			history, _, unsubscribe, err := runner.Subscribe(run.ID)
			if err != nil {
				t.Fatalf("Subscribe: %v", err)
			}
			unsubscribe()

			if isTerminalRunStatus(current.Status) && !containsEventType(history, tt.wantEvent) {
				t.Fatalf("GetRun returned %s before %s was replayable; history=%v",
					current.Status, tt.wantEvent, terminalAtomicityEventTypes(history))
			}
			if tt.wantStatus == RunStatusFailed &&
				containsEventType(history, EventRunFailed) &&
				!eventPrecedes(history, EventErrorContext, EventRunFailed) {
				t.Fatalf("failed status became observable without error.context before run.failed; history=%v",
					terminalAtomicityEventTypes(history))
			}

			close(release)
			waitForStatus(t, runner, run.ID, tt.wantStatus)

			finalHistory, _, finalUnsubscribe, err := runner.Subscribe(run.ID)
			if err != nil {
				t.Fatalf("Subscribe after terminal status: %v", err)
			}
			finalUnsubscribe()
			if !containsEventType(finalHistory, tt.wantEvent) {
				t.Fatalf("terminal status %s missing matching replay event %s; history=%v",
					tt.wantStatus, tt.wantEvent, terminalAtomicityEventTypes(finalHistory))
			}
			switch tt.wantStatus {
			case RunStatusCompleted:
				if !eventPrecedes(finalHistory, EventCausalGraphSnapshot, EventRunCompleted) {
					t.Fatalf("completed status missing causal snapshot before run.completed; history=%v",
						terminalAtomicityEventTypes(finalHistory))
				}
			case RunStatusFailed:
				if !eventPrecedes(finalHistory, EventErrorContext, EventRunFailed) {
					t.Fatalf("failed status missing error.context before run.failed; history=%v",
						terminalAtomicityEventTypes(finalHistory))
				}
			}
		})
	}
}

func TestCompetingTerminalTransitionsPublishMatchingStatusAndEvent(t *testing.T) {
	const iterations = 100
	for iteration := 0; iteration < iterations; iteration++ {
		runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{})
		runID := fmt.Sprintf("terminal-race-%d", iteration)
		runner.runs[runID] = &runState{
			run: Run{
				ID:             runID,
				ConversationID: "terminal-race-conversation",
				Status:         RunStatusRunning,
			},
			subscribers: make(map[chan Event]struct{}),
		}

		start := make(chan struct{})
		var transitions sync.WaitGroup
		transitions.Add(3)
		go func() {
			defer transitions.Done()
			<-start
			runner.completeRun(runID, "done")
		}()
		go func() {
			defer transitions.Done()
			<-start
			runner.failRun(runID, errors.New("failed"))
		}()
		go func() {
			defer transitions.Done()
			<-start
			runner.cancelledRun(runID)
		}()
		close(start)
		transitions.Wait()

		current, ok := runner.GetRun(runID)
		if !ok {
			t.Fatalf("iteration %d: GetRun returned not found", iteration)
		}
		history, _, unsubscribe, err := runner.Subscribe(runID)
		if err != nil {
			t.Fatalf("iteration %d: Subscribe: %v", iteration, err)
		}
		unsubscribe()

		terminalEvents := make([]EventType, 0, 1)
		for _, event := range history {
			if IsTerminalEvent(event.Type) {
				terminalEvents = append(terminalEvents, event.Type)
			}
		}
		if len(terminalEvents) != 1 {
			t.Fatalf("iteration %d: terminal events=%v, want exactly one; history=%v",
				iteration, terminalEvents, terminalAtomicityEventTypes(history))
		}
		if wantStatus := statusForTerminalEvent(terminalEvents[0]); current.Status != wantStatus {
			t.Fatalf("iteration %d: status=%s does not match sealed event=%s (want %s)",
				iteration, current.Status, terminalEvents[0], wantStatus)
		}
	}
}

func TestTerminalConversationFanoutCannotBeOvertaken(t *testing.T) {
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{})
	const (
		conversationID = "terminal-conversation-order"
		terminalRunID  = "terminal-conversation-order-terminal"
		laterRunID     = "terminal-conversation-order-later"
	)
	for _, runID := range []string{terminalRunID, laterRunID} {
		runner.runs[runID] = &runState{
			run: Run{
				ID:             runID,
				ConversationID: conversationID,
				Status:         RunStatusRunning,
			},
			subscribers: make(map[chan Event]struct{}),
		}
	}

	history, stream, unsubscribe, err := runner.SubscribeConversation(conversationID)
	if err != nil {
		t.Fatalf("SubscribeConversation: %v", err)
	}
	defer unsubscribe()
	if len(history) != 0 {
		t.Fatalf("initial conversation history = %v, want empty", terminalAtomicityEventTypes(history))
	}

	reachedDispatch := make(chan struct{})
	releaseDispatch := make(chan struct{})
	runner.terminalBeforeDispatchHook = func(runID string, eventType EventType) {
		if runID != terminalRunID || eventType != EventRunCompleted {
			return
		}
		close(reachedDispatch)
		<-releaseDispatch
	}

	terminalDone := make(chan bool, 1)
	go func() {
		terminalDone <- runner.transitionTerminal(
			terminalRunID,
			RunStatusCompleted,
			"done",
			"",
			EventRunCompleted,
			map[string]any{"output": "done"},
		)
	}()
	select {
	case <-reachedDispatch:
	case <-time.After(2 * time.Second):
		t.Fatal("terminal transition did not reach pre-dispatch barrier")
	}

	// Slow terminal persistence/recorder/status work must not monopolize the
	// global replay-to-live mutex. Same-conversation ordering is now carried by
	// the narrower keyed sequence lock for the complete terminal transition.
	if !runner.conversationEventMu.TryLock() {
		close(releaseDispatch)
		<-terminalDone
		t.Fatal("global conversation event lock held during terminal I/O")
	}
	runner.conversationEventMu.Unlock()

	laterStarted := make(chan struct{})
	laterDone := make(chan struct{})
	go func() {
		close(laterStarted)
		runner.emit(laterRunID, EventAssistantMessage, map[string]any{"content": "later"})
		close(laterDone)
	}()
	<-laterStarted
	select {
	case <-laterDone:
		close(releaseDispatch)
		<-terminalDone
		t.Fatal("later same-conversation event overtook terminal fanout")
	default:
	}
	close(releaseDispatch)
	if won := <-terminalDone; !won {
		t.Fatal("terminal transition did not win")
	}
	select {
	case <-laterDone:
	case <-time.After(2 * time.Second):
		t.Fatal("later conversation event remained blocked")
	}

	got := make([]Event, 0, 2)
	for len(got) < 2 {
		select {
		case event := <-stream:
			got = append(got, event)
		case <-time.After(2 * time.Second):
			t.Fatalf("conversation subscriber received %v, want terminal then later event",
				terminalAtomicityEventTypes(got))
		}
	}
	if got[0].RunID != terminalRunID || got[0].Type != EventRunCompleted ||
		got[1].RunID != laterRunID || got[1].Type != EventAssistantMessage {
		t.Fatalf("conversation subscriber order = %v, want %s/%s then %s/%s",
			terminalAtomicityEventTypes(got), terminalRunID, EventRunCompleted,
			laterRunID, EventAssistantMessage)
	}
}

type terminalCancellationProvider struct {
	started chan struct{}
}

func (p *terminalCancellationProvider) Complete(ctx context.Context, _ CompletionRequest) (CompletionResult, error) {
	select {
	case <-p.started:
	default:
		close(p.started)
	}
	<-ctx.Done()
	return CompletionResult{}, ctx.Err()
}

func containsEventType(events []Event, want EventType) bool {
	for _, event := range events {
		if event.Type == want {
			return true
		}
	}
	return false
}

func eventPrecedes(events []Event, first, second EventType) bool {
	firstIndex, secondIndex := -1, -1
	for i, event := range events {
		switch event.Type {
		case first:
			if firstIndex == -1 {
				firstIndex = i
			}
		case second:
			if secondIndex == -1 {
				secondIndex = i
			}
		}
	}
	return firstIndex >= 0 && secondIndex > firstIndex
}

func terminalAtomicityEventTypes(events []Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, fmt.Sprint(event.Type))
	}
	return types
}

func statusForTerminalEvent(eventType EventType) RunStatus {
	switch eventType {
	case EventRunCompleted:
		return RunStatusCompleted
	case EventRunFailed:
		return RunStatusFailed
	case EventRunCancelled:
		return RunStatusCancelled
	default:
		return ""
	}
}
