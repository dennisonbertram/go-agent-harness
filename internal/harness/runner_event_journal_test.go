package harness

import (
	"context"
	"errors"
	"testing"

	"go-agent-harness/internal/store"
)

type failingOrdinaryEventStore struct {
	*store.MemoryStore
}

func (s *failingOrdinaryEventStore) AppendEvent(context.Context, *store.Event) error {
	return errors.New("ordinary event persistence unavailable")
}

type terminalOrderingStore struct {
	*store.MemoryStore
	terminalAppendStarted chan struct{}
	releaseTerminalAppend chan struct{}
}

type allEventOrderingStore struct {
	*store.MemoryStore
	appendStarted chan struct{}
	releaseAppend chan struct{}
}

func (s *allEventOrderingStore) AppendEvent(ctx context.Context, ev *store.Event) error {
	select {
	case <-s.appendStarted:
	default:
		close(s.appendStarted)
	}
	<-s.releaseAppend
	return s.MemoryStore.AppendEvent(ctx, ev)
}

func newTerminalOrderingStore() *terminalOrderingStore {
	return &terminalOrderingStore{
		MemoryStore:           store.NewMemoryStore(),
		terminalAppendStarted: make(chan struct{}),
		releaseTerminalAppend: make(chan struct{}),
	}
}

func (s *terminalOrderingStore) AppendEvent(ctx context.Context, ev *store.Event) error {
	if IsTerminalEvent(EventType(ev.EventType)) {
		select {
		case <-s.terminalAppendStarted:
		default:
			close(s.terminalAppendStarted)
		}
		<-s.releaseTerminalAppend
	}
	return s.MemoryStore.AppendEvent(ctx, ev)
}

func TestEventJournalDispatch_TerminalStoreAppendPrecedesSubscriberNotification(t *testing.T) {
	t.Parallel()

	st := newTerminalOrderingStore()
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		Store:        st,
	})

	sub := make(chan Event, 1)
	state := &runState{
		run: Run{
			ID:             "run_terminal_order",
			ConversationID: "conv_terminal_order",
		},
		subscribers: map[chan Event]struct{}{
			sub: {},
		},
		nextEventSeq: 7,
	}

	journal := newEventJournal(runner)

	runner.mu.Lock()
	delivery, ok := journal.prepareLocked(state, state.run.ID, EventRunCompleted, map[string]any{
		"output": "done",
	})
	runner.mu.Unlock()
	if !ok {
		t.Fatal("prepareLocked returned ok=false for terminal event")
	}

	delivered := make(chan Event, 1)
	go func() {
		delivered <- <-sub
	}()

	go func() {
		journal.publishTerminal(delivery)
		journal.dispatch(delivery)
	}()

	select {
	case ev := <-delivered:
		t.Fatalf("subscriber observed terminal event %q before store append started", ev.Type)
	case <-st.terminalAppendStarted:
	}

	select {
	case ev := <-delivered:
		t.Fatalf("subscriber observed terminal event %q before terminal store append completed", ev.Type)
	default:
	}

	close(st.releaseTerminalAppend)

	ev := <-delivered
	if ev.Type != EventRunCompleted {
		t.Fatalf("subscriber event type = %q, want %q", ev.Type, EventRunCompleted)
	}
}

func TestEventJournalDispatch_NonTerminalStoreAppendPrecedesSubscriberNotification(t *testing.T) {
	t.Parallel()

	st := &allEventOrderingStore{
		MemoryStore:   store.NewMemoryStore(),
		appendStarted: make(chan struct{}),
		releaseAppend: make(chan struct{}),
	}
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		Store:        st,
	})
	sub := make(chan Event, 1)
	state := &runState{
		run: Run{
			ID:             "run_nonterminal_order",
			ConversationID: "conv_nonterminal_order",
		},
		subscribers: map[chan Event]struct{}{sub: {}},
	}
	journal := newEventJournal(runner)

	runner.mu.Lock()
	delivery, ok := journal.prepareLocked(
		state,
		state.run.ID,
		EventRunStarted,
		map[string]any{"status": "running"},
	)
	runner.mu.Unlock()
	if !ok {
		t.Fatal("prepareLocked returned ok=false for non-terminal event")
	}

	delivered := make(chan Event, 1)
	go func() { delivered <- <-sub }()
	go journal.dispatch(delivery)

	select {
	case event := <-delivered:
		t.Fatalf("subscriber observed non-terminal event %q before append started", event.Type)
	case <-st.appendStarted:
	}
	select {
	case event := <-delivered:
		t.Fatalf("subscriber observed non-terminal event %q before append completed", event.Type)
	default:
	}

	close(st.releaseAppend)
	if event := <-delivered; event.Type != EventRunStarted {
		t.Fatalf("subscriber event type = %q, want %q", event.Type, EventRunStarted)
	}
}

func TestEventJournalDispatch_OrdinaryEventRemainsVisibleWhenStoreAppendFails(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		Store:        &failingOrdinaryEventStore{MemoryStore: store.NewMemoryStore()},
	})
	const runID = "run_ordinary_append_failure"
	runner.runs[runID] = &runState{
		run: Run{ID: runID, ConversationID: "conv_ordinary_append_failure", Status: RunStatusRunning},
	}

	runner.emit(runID, EventToolCallStarted, map[string]any{"tool": "read"})

	events := runner.getEvents(runID)
	if len(events) != 1 || events[0].Type != EventToolCallStarted {
		t.Fatalf("ordinary event was removed after non-fatal store failure: %+v", events)
	}
}
