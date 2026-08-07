package workflows

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// snapshotGateStore captures GetEvents' result then blocks its return. This
// makes the former plural-engine gap deterministic: the old Subscribe took its
// history snapshot first, an emit happened here, and only then did Subscribe
// register its channel.
type snapshotGateStore struct {
	*MemoryStore
	snapshotTaken chan struct{}
	release       chan struct{}
	once          sync.Once
}

func newSnapshotGateStore() *snapshotGateStore {
	return &snapshotGateStore{
		MemoryStore:   NewMemoryStore(),
		snapshotTaken: make(chan struct{}),
		release:       make(chan struct{}),
	}
}

func (s *snapshotGateStore) GetEvents(ctx context.Context, runID string, afterSeq int64) ([]Event, error) {
	events, err := s.MemoryStore.GetEvents(ctx, runID, afterSeq)
	if err != nil {
		return nil, err
	}
	s.once.Do(func() { close(s.snapshotTaken) })
	<-s.release
	return events, nil
}

type getEventsErrorStore struct {
	*MemoryStore
	err error
}

func (s *getEventsErrorStore) GetEvents(context.Context, string, int64) ([]Event, error) {
	return nil, s.err
}

type subscriptionResult struct {
	history []Event
	live    <-chan Event
	cancel  func()
	err     error
}

func subscribeAsync(e *Engine, runID string) <-chan subscriptionResult {
	result := make(chan subscriptionResult, 1)
	go func() {
		history, live, cancel, err := e.Subscribe(runID)
		result <- subscriptionResult{history: history, live: live, cancel: cancel, err: err}
	}()
	return result
}

func waitSnapshot(t *testing.T, gate *snapshotGateStore) {
	t.Helper()
	select {
	case <-gate.snapshotTaken:
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe did not take its controlled history snapshot")
	}
}

func waitSubscription(t *testing.T, result <-chan subscriptionResult) subscriptionResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe did not return after controlled history release")
		return subscriptionResult{}
	}
}

func TestEngineSubscribeBridgesSnapshotToLiveExactlyOnce(t *testing.T) {
	store := newSnapshotGateStore()
	engine := NewEngine(Options{Store: store})
	const runID = "snapshot-live-gap"

	result := subscribeAsync(engine, runID)
	waitSnapshot(t, store)

	// This happens after the Store has captured its result and before that
	// result returns. The former implementation had not registered the live
	// channel yet, so it lost this event permanently.
	engine.emit(runID, "workflow.step.started", map[string]any{"step_id": "one"})
	close(store.release)

	got := waitSubscription(t, result)
	if got.err != nil {
		t.Fatalf("Subscribe: %v", got.err)
	}
	defer got.cancel()

	// A post-return event must remain live, rather than being folded into the
	// initialization history.
	engine.emit(runID, "workflow.step.completed", map[string]any{"step_id": "one"})
	live := <-got.live

	seen := make(map[string]int)
	for _, event := range append(got.history, live) {
		seen[event.Type]++
	}
	for _, eventType := range []string{"workflow.step.started", "workflow.step.completed"} {
		if seen[eventType] != 1 {
			t.Fatalf("%s observed %d times across history/live, want exactly once; history=%+v live=%+v", eventType, seen[eventType], got.history, live)
		}
	}
}

func TestEngineSubscribeBuffersBurstWhileHistoryIsUnavailable(t *testing.T) {
	store := newSnapshotGateStore()
	engine := NewEngine(Options{Store: store})
	const (
		runID = "slow-history-burst"
		burst = 64 // deliberately greater than the live channel's capacity (16)
	)

	result := subscribeAsync(engine, runID)
	waitSnapshot(t, store)

	emitted := make(chan struct{})
	go func() {
		defer close(emitted)
		for i := 0; i < burst; i++ {
			engine.emit(runID, "workflow.step.started", map[string]any{"index": i})
		}
	}()
	select {
	case <-emitted:
		// GetEvents must not hold the engine-wide mutex; the burst must also
		// fit the initializing buffer rather than the bounded live channel.
	case <-time.After(2 * time.Second):
		t.Fatal("emit burst blocked while Subscribe was reading history")
	}
	close(store.release)

	got := waitSubscription(t, result)
	if got.err != nil {
		t.Fatalf("Subscribe: %v", got.err)
	}
	defer got.cancel()

	seen := make(map[int64]int)
	for _, event := range got.history {
		seen[event.Seq]++
	}
	for len(got.live) > 0 {
		seen[(<-got.live).Seq]++
	}
	if len(seen) != burst {
		t.Fatalf("distinct events = %d, want %d; history=%d", len(seen), burst, len(got.history))
	}
	for seq, count := range seen {
		if count != 1 {
			t.Fatalf("event sequence %d observed %d times, want exactly once", seq, count)
		}
	}
}

func TestEngineSubscribeGetEventsErrorDeregistersInitializingSubscriber(t *testing.T) {
	wantErr := errors.New("history unavailable")
	engine := NewEngine(Options{Store: &getEventsErrorStore{MemoryStore: NewMemoryStore(), err: wantErr}})

	history, live, cancel, err := engine.Subscribe("get-events-error")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Subscribe error = %v, want %v", err, wantErr)
	}
	if history != nil || live != nil || cancel != nil {
		t.Fatalf("Subscribe success values nil = (%t, %t, %t), want all true on error", history == nil, live == nil, cancel == nil)
	}

	engine.mu.Lock()
	defer engine.mu.Unlock()
	if subscribers := engine.subs["get-events-error"]; len(subscribers) != 0 {
		t.Fatalf("GetEvents error left %d subscriber(s) registered", len(subscribers))
	}
}

func TestEngineSubscribeCancelClosesAndDeregistersSubscriber(t *testing.T) {
	engine := NewEngine(Options{Store: NewMemoryStore()})
	const runID = "cancel-cleanup"

	_, live, cancel, err := engine.Subscribe(runID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	cancel()
	cancel() // cancellation remains idempotent after map removal

	if _, ok := <-live; ok {
		t.Fatal("subscription channel remained open after cancel")
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if subscribers := engine.subs[runID]; len(subscribers) != 0 {
		t.Fatalf("cancel left %d subscriber(s) registered", len(subscribers))
	}
}
