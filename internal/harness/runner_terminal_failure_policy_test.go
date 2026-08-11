package harness

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/forensics/redaction"
	"go-agent-harness/internal/rollout"
	runstore "go-agent-harness/internal/store"
)

type failingNonTerminalStatusStore struct {
	*runstore.MemoryStore
}

func (s *failingNonTerminalStatusStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	if run.Status == runstore.RunStatusRunning {
		return errors.New("nonterminal status persistence failed")
	}
	return s.MemoryStore.UpdateRun(ctx, run)
}

func TestNonTerminalStatusPersistenceFailureKeepsLiveStateMoving(t *testing.T) {
	store := &failingNonTerminalStatusStore{MemoryStore: runstore.NewMemoryStore()}
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: store})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	const runID = "nonterminal-status-write-failure"
	run := Run{ID: runID, ConversationID: "nonterminal-status-write-failure-conv", Status: RunStatusQueued}
	runner.runs[runID] = &runState{run: run, subscribers: make(map[chan Event]struct{})}
	if err := store.CreateRun(context.Background(), runToStoreRun(run)); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	runner.setStatus(runID, RunStatusRunning, "", "")

	current, ok := runner.GetRun(runID)
	if !ok {
		t.Fatal("run disappeared")
	}
	if current.Status != RunStatusRunning {
		t.Fatalf("in-memory status = %q, want running despite best-effort store failure", current.Status)
	}
	stored, err := store.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if stored.Status != runstore.RunStatusQueued {
		t.Fatalf("durable status = %q, want queued after injected write failure", stored.Status)
	}
}

func TestTerminalAppendDoesNotBlockUnrelatedConversationOrAllowSameConversationOvertake(t *testing.T) {
	store := &blockingTerminalAppendStore{
		Store:   runstore.NewMemoryStore(),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	var releaseOnce sync.Once
	releaseStore := func() { releaseOnce.Do(func() { close(store.release) }) }
	t.Cleanup(releaseStore)

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: store})
	const (
		terminalRunID = "blocked-terminal-append"
		sameRunID     = "same-conversation-later"
		otherRunID    = "unrelated-conversation"
		terminalConv  = "terminal-conversation"
	)
	for runID, convID := range map[string]string{
		terminalRunID: terminalConv,
		sameRunID:     terminalConv,
		otherRunID:    "other-conversation",
	} {
		run := Run{ID: runID, ConversationID: convID, Status: RunStatusRunning}
		runner.runs[runID] = &runState{run: run, subscribers: make(map[chan Event]struct{})}
		runner.storeCreateRun(run)
	}

	history, stream, unsubscribe, err := runner.SubscribeConversation(terminalConv)
	if err != nil {
		t.Fatalf("SubscribeConversation: %v", err)
	}
	defer unsubscribe()
	if len(history) != 0 {
		t.Fatalf("initial history=%v, want empty", terminalAtomicityEventTypes(history))
	}

	store.setBlockRunID(terminalRunID)
	terminalDone := make(chan bool, 1)
	go func() {
		terminalDone <- runner.transitionTerminal(
			terminalRunID, RunStatusCompleted, "done", "",
			EventRunCompleted, map[string]any{"output": "done"},
		)
	}()
	select {
	case <-store.started:
	case <-time.After(2 * time.Second):
		t.Fatal("terminal AppendEvent did not block")
	}

	otherDone := make(chan struct{})
	go func() {
		runner.emit(otherRunID, EventAssistantMessage, map[string]any{"content": "other"})
		close(otherDone)
	}()
	select {
	case <-otherDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("unrelated conversation emit blocked behind terminal AppendEvent")
	}

	sameStarted := make(chan struct{})
	sameDone := make(chan struct{})
	go func() {
		close(sameStarted)
		runner.emit(sameRunID, EventAssistantMessage, map[string]any{"content": "later"})
		close(sameDone)
	}()
	<-sameStarted
	select {
	case <-sameDone:
		t.Fatal("same-conversation event overtook blocked terminal AppendEvent")
	default:
	}

	releaseStore()
	if won := <-terminalDone; !won {
		t.Fatal("terminal transition did not commit")
	}
	select {
	case <-sameDone:
	case <-time.After(2 * time.Second):
		t.Fatal("same-conversation event stayed blocked after terminal publication")
	}

	got := make([]Event, 0, 2)
	for len(got) < 2 {
		select {
		case event := <-stream:
			got = append(got, event)
		case <-time.After(2 * time.Second):
			t.Fatalf("conversation stream=%v, want terminal then later", terminalAtomicityEventTypes(got))
		}
	}
	if got[0].Type != EventRunCompleted || got[1].Type != EventAssistantMessage {
		t.Fatalf("conversation order=%v, want run.completed then assistant.message",
			terminalAtomicityEventTypes(got))
	}
}

func TestTerminalAppendFailureNeverPersistsTerminalStatusButStillPublishesInMemory(t *testing.T) {
	store := &terminalFailureStore{
		Store:           runstore.NewMemoryStore(),
		failAppendEvent: true,
	}
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: store})
	run := Run{ID: "append-failure", ConversationID: "append-failure-conv", Status: RunStatusRunning}
	runner.runs[run.ID] = &runState{run: run, subscribers: make(map[chan Event]struct{})}
	runner.storeCreateRun(run)
	history, stream, unsubscribe, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsubscribe()
	if len(history) != 0 {
		t.Fatalf("initial history=%v, want empty", terminalAtomicityEventTypes(history))
	}
	if won := runner.transitionTerminal(
		run.ID, RunStatusCompleted, "done", "",
		EventRunCompleted, map[string]any{"output": "done"},
	); !won {
		t.Fatal("terminal transition did not commit after append failure")
	}
	requireTerminalSubscriberEvent(t, stream, EventRunCompleted)
	current, _ := runner.GetRun(run.ID)
	if current.Status != RunStatusCompleted {
		t.Fatalf("in-memory status=%s, want completed after append failure", current.Status)
	}

	stored, err := store.Store.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("store.GetRun: %v", err)
	}
	if stored.Status == runstore.RunStatusCompleted {
		t.Fatal("durable terminal status was written after terminal AppendEvent failed")
	}
	replayHistory, _, replayUnsubscribe, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	replayUnsubscribe()
	if !containsEventType(replayHistory, EventRunCompleted) {
		t.Fatal("in-memory replay did not publish run.completed after bounded append failure")
	}
}

func TestTerminalStatusUpdateFailureLeavesDurableRunNonTerminalAndPublishesInMemory(t *testing.T) {
	store := &terminalFailureStore{
		Store:         runstore.NewMemoryStore(),
		failUpdateRun: true,
	}
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: store})
	run := Run{ID: "status-failure", ConversationID: "status-failure-conv", Status: RunStatusRunning}
	runner.runs[run.ID] = &runState{run: run, subscribers: make(map[chan Event]struct{})}
	runner.storeCreateRun(run)
	_, stream, unsubscribe, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsubscribe()
	if won := runner.transitionTerminal(
		run.ID, RunStatusCompleted, "done", "",
		EventRunCompleted, map[string]any{"output": "done"},
	); !won {
		t.Fatal("terminal transition did not commit after status update failure")
	}
	requireTerminalSubscriberEvent(t, stream, EventRunCompleted)
	current, _ := runner.GetRun(run.ID)
	if current.Status != RunStatusCompleted {
		t.Fatalf("in-memory status=%s, want completed after status update failure", current.Status)
	}

	stored, err := store.Store.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("store.GetRun: %v", err)
	}
	if stored.Status == runstore.RunStatusCompleted {
		t.Fatal("durable run unexpectedly became terminal after UpdateRun failure")
	}
	events, err := store.Store.GetEvents(context.Background(), run.ID, -1)
	if err != nil {
		t.Fatalf("store.GetEvents: %v", err)
	}
	foundTerminal := false
	for _, event := range events {
		if event.EventType == string(EventRunCompleted) {
			foundTerminal = true
		}
	}
	if !foundTerminal {
		t.Fatal("durable terminal event missing when only UpdateRun failed")
	}
}

func TestDelayedNonTerminalStatusCannotOverwriteTerminalTransition(t *testing.T) {
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{})
	const runID = "status-vs-terminal"
	runner.runs[runID] = &runState{
		run:         Run{ID: runID, ConversationID: "status-vs-terminal-conv", Status: RunStatusRunning},
		subscribers: make(map[chan Event]struct{}),
	}

	waitingPrepared := make(chan struct{})
	releaseWaiting := make(chan struct{})
	runner.statusBeforeCommitHook = func(gotRunID string, status RunStatus) {
		if gotRunID == runID && status == RunStatusWaitingForUser {
			close(waitingPrepared)
			<-releaseWaiting
		}
	}
	waitingDone := make(chan struct{})
	go func() {
		runner.setStatus(runID, RunStatusWaitingForUser, "", "")
		close(waitingDone)
	}()
	<-waitingPrepared

	terminalAttempted := make(chan struct{})
	runner.terminalTransitionHook = func(gotRunID string, status RunStatus, _ EventType) {
		if gotRunID == runID && status == RunStatusCompleted {
			close(terminalAttempted)
		}
	}
	terminalDone := make(chan bool, 1)
	go func() {
		terminalDone <- runner.transitionTerminal(
			runID, RunStatusCompleted, "done", "", EventRunCompleted, map[string]any{"output": "done"},
		)
	}()
	<-terminalAttempted
	close(releaseWaiting)
	<-waitingDone
	if won := <-terminalDone; !won {
		t.Fatal("terminal transition did not commit")
	}

	current, ok := runner.GetRun(runID)
	if !ok {
		t.Fatal("run disappeared")
	}
	if current.Status != RunStatusCompleted {
		t.Fatalf("delayed non-terminal status overwrote terminal status: got %s", current.Status)
	}
}

func TestTerminalStatusUpdateTimeoutStillPublishesAndDoesNotBlockUnrelatedConversation(t *testing.T) {
	store := &contextBlockingTerminalStatusStore{
		Store:        runstore.NewMemoryStore(),
		started:      make(chan struct{}),
		forceRelease: make(chan struct{}),
	}
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: store})
	runner.terminalStoreTimeout = 25 * time.Millisecond
	const (
		terminalRunID = "terminal-status-timeout"
		otherRunID    = "terminal-status-timeout-other"
	)
	for runID, convID := range map[string]string{
		terminalRunID: "terminal-status-timeout-conv",
		otherRunID:    "terminal-status-timeout-other-conv",
	} {
		run := Run{ID: runID, ConversationID: convID, Status: RunStatusRunning}
		runner.runs[runID] = &runState{run: run, subscribers: make(map[chan Event]struct{})}
		runner.storeCreateRun(run)
	}
	store.blockRunID = terminalRunID
	_, terminalStream, unsubscribe, err := runner.Subscribe(terminalRunID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsubscribe()

	terminalDone := make(chan bool, 1)
	go func() {
		terminalDone <- runner.transitionTerminal(
			terminalRunID, RunStatusCompleted, "done", "",
			EventRunCompleted, map[string]any{"output": "done"},
		)
	}()
	select {
	case <-store.started:
	case <-time.After(2 * time.Second):
		t.Fatal("terminal UpdateRun did not block")
	}

	otherDone := make(chan struct{})
	go func() {
		runner.emit(otherRunID, EventAssistantMessage, map[string]any{"content": "other"})
		close(otherDone)
	}()
	select {
	case <-otherDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("unrelated conversation blocked behind terminal UpdateRun")
	}

	select {
	case won := <-terminalDone:
		if !won {
			t.Fatal("terminal transition did not publish after UpdateRun timeout")
		}
	case <-time.After(200 * time.Millisecond):
		close(store.forceRelease)
		t.Fatal("terminal transition did not recover from bounded UpdateRun timeout")
	}
	current, _ := runner.GetRun(terminalRunID)
	if current.Status != RunStatusCompleted {
		t.Fatalf("in-memory status=%s, want completed after UpdateRun timeout", current.Status)
	}
	requireTerminalSubscriberEvent(t, terminalStream, EventRunCompleted)
}

func TestRedactedTerminalWaitsForRecorderDrainBeforeStatus(t *testing.T) {
	pipeline := redaction.NewPipeline(
		redaction.NewRedactor(nil),
		redaction.EventClassConfig{string(EventRunCompleted): redaction.StorageModeNone},
	)
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{RedactionPipeline: pipeline})
	const runID = "redacted-recorder-drain"
	recorderDone := make(chan struct{})
	recorderClosed := make(chan struct{})
	var closeOnce sync.Once
	runner.runs[runID] = &runState{
		run:          Run{ID: runID, ConversationID: "redacted-recorder-drain-conv", Status: RunStatusRunning},
		subscribers:  make(map[chan Event]struct{}),
		recorderCh:   make(chan rollout.RecordableEvent, 1),
		recorderDone: recorderDone,
		closeRecorderOnce: func() {
			closeOnce.Do(func() {
				close(recorderClosed)
			})
		},
	}

	terminalDone := make(chan bool, 1)
	go func() {
		terminalDone <- runner.transitionTerminal(
			runID, RunStatusCompleted, "done", "", EventRunCompleted, map[string]any{"output": "done"},
		)
	}()
	select {
	case <-recorderClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("redacted terminal did not close recorder")
	}
	current, _ := runner.GetRun(runID)
	if current.Status == RunStatusCompleted {
		close(recorderDone)
		<-terminalDone
		t.Fatal("redacted terminal status became visible before recorder drain")
	}
	close(recorderDone)
	if won := <-terminalDone; !won {
		t.Fatal("redacted terminal transition did not commit")
	}
}

func TestConversationSequenceLockReclaimedAfterContention(t *testing.T) {
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{})
	unlockFirst := runner.lockConversationSequence("reclaim")
	waiterDone := make(chan struct{})
	go func() {
		unlockWaiter := runner.lockConversationSequence("reclaim")
		unlockWaiter()
		close(waiterDone)
	}()
	waitForConversationSequenceRefs(t, runner, "reclaim", 2)
	unlockFirst()
	select {
	case <-waiterDone:
	case <-time.After(2 * time.Second):
		t.Fatal("conversation sequence waiter did not finish")
	}

	runner.conversationSequenceMu.Lock()
	remaining := len(runner.conversationSequence)
	runner.conversationSequenceMu.Unlock()
	if remaining != 0 {
		t.Fatalf("conversation sequence entries=%d, want 0 after owners and waiters release", remaining)
	}

	for i := 0; i < 100; i++ {
		unlock := runner.lockConversationSequence(fmt.Sprintf("distinct-%d", i))
		unlock()
	}
	runner.conversationSequenceMu.Lock()
	remaining = len(runner.conversationSequence)
	runner.conversationSequenceMu.Unlock()
	if remaining != 0 {
		t.Fatalf("conversation sequence entries=%d, want 0 after distinct-key releases", remaining)
	}
}

func waitForConversationSequenceRefs(t *testing.T, runner *Runner, key string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		runner.conversationSequenceMu.Lock()
		entry := runner.conversationSequence[key]
		got := 0
		if entry != nil {
			got = entry.refs
		}
		runner.conversationSequenceMu.Unlock()
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("conversation sequence refs=%d, want %d", got, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func requireTerminalSubscriberEvent(t *testing.T, stream <-chan Event, want EventType) {
	t.Helper()
	select {
	case event := <-stream:
		if event.Type != want {
			t.Fatalf("subscriber event=%s, want %s", event.Type, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for subscriber event %s", want)
	}
}

type terminalFailureStore struct {
	runstore.Store
	failAppendEvent bool
	failUpdateRun   bool
}

func (s *terminalFailureStore) AppendEvent(ctx context.Context, event *runstore.Event) error {
	if s.failAppendEvent && IsTerminalEvent(EventType(event.EventType)) {
		return errors.New("terminal append failed")
	}
	return s.Store.AppendEvent(ctx, event)
}

func (s *terminalFailureStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	if s.failUpdateRun && isTerminalStoreStatus(run.Status) {
		return errors.New("terminal status update failed")
	}
	return s.Store.UpdateRun(ctx, run)
}

type contextBlockingTerminalStatusStore struct {
	runstore.Store
	blockRunID   string
	started      chan struct{}
	startedOnce  sync.Once
	forceRelease chan struct{}
}

func (s *contextBlockingTerminalStatusStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	if run.ID == s.blockRunID && isTerminalStoreStatus(run.Status) {
		s.startedOnce.Do(func() { close(s.started) })
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.forceRelease:
			return errors.New("forced release")
		}
	}
	return s.Store.UpdateRun(ctx, run)
}

func isTerminalStoreStatus(status runstore.RunStatus) bool {
	return status == runstore.RunStatusCompleted ||
		status == runstore.RunStatusFailed ||
		status == runstore.RunStatus("cancelled")
}
