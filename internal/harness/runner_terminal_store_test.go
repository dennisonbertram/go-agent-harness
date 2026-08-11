package harness

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	runstore "go-agent-harness/internal/store"
)

func TestTerminalStoreAppendDoesNotBlockRunnerQueries(t *testing.T) {
	holdProviderRelease := make(chan struct{})
	doneProviderRelease := make(chan struct{})
	provider := &promptGateProvider{
		gates: map[string]<-chan struct{}{
			"hold": holdProviderRelease,
			"done": doneProviderRelease,
		},
	}
	backingStore := &blockingTerminalAppendStore{
		Store:   runstore.NewMemoryStore(),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		Store: backingStore,
	})

	holdRun, err := runner.StartRun(RunRequest{Prompt: "hold"})
	if err != nil {
		t.Fatalf("StartRun hold: %v", err)
	}
	waitForStatus(t, runner, holdRun.ID, RunStatusRunning)

	doneRun, err := runner.StartRun(RunRequest{Prompt: "done"})
	if err != nil {
		t.Fatalf("StartRun done: %v", err)
	}
	backingStore.setBlockRunID(doneRun.ID)
	close(doneProviderRelease)

	select {
	case <-backingStore.started:
	case <-time.After(2 * time.Second):
		close(holdProviderRelease)
		t.Fatal("timed out waiting for terminal event store append to block")
	}

	blockedRun, ok := runner.GetRun(doneRun.ID)
	if !ok {
		t.Fatalf("GetRun(%q) returned false during terminal append", doneRun.ID)
	}
	if isTerminalRunStatus(blockedRun.Status) {
		close(backingStore.release)
		close(holdProviderRelease)
		t.Fatalf("GetRun exposed %s before terminal event store append completed", blockedRun.Status)
	}

	queryDone := make(chan struct{})
	var found bool
	go func() {
		_, found = runner.GetRun(holdRun.ID)
		close(queryDone)
	}()

	select {
	case <-queryDone:
	case <-time.After(200 * time.Millisecond):
		close(backingStore.release)
		close(holdProviderRelease)
		<-queryDone
		t.Fatal("GetRun blocked behind terminal event store append")
	}

	if !found {
		t.Fatalf("GetRun(%q) returned false while the run was still retained", holdRun.ID)
	}

	close(backingStore.release)
	close(holdProviderRelease)
	waitForStatus(t, runner, doneRun.ID, RunStatusCompleted)
	waitForStatus(t, runner, holdRun.ID, RunStatusCompleted)
}

func TestTerminalStoreAppendUsesBoundedContext(t *testing.T) {
	provider := staticContentProvider{content: "done"}
	store := &deadlineRecordingStore{Store: runstore.NewMemoryStore(), deadline: make(chan time.Duration, 1)}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{Store: store})

	run, err := runner.StartRun(RunRequest{Prompt: "bounded terminal append"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted)

	select {
	case remaining := <-store.deadline:
		if remaining <= 0 || remaining > 6*time.Second {
			t.Fatalf("terminal append deadline remaining = %s, want bounded deadline near 5s", remaining)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal AppendEvent did not receive a deadline")
	}
}

func TestTerminalStatusStoreUpdateDoesNotBlockOtherEventJournals(t *testing.T) {
	holdProviderRelease := make(chan struct{})
	doneProviderRelease := make(chan struct{})
	provider := &promptGateProvider{
		gates: map[string]<-chan struct{}{
			"hold": holdProviderRelease,
			"done": doneProviderRelease,
		},
	}
	backingStore := &blockingTerminalStatusStore{
		Store:   runstore.NewMemoryStore(),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{Store: backingStore})

	holdRun, err := runner.StartRun(RunRequest{Prompt: "hold"})
	if err != nil {
		t.Fatalf("StartRun hold: %v", err)
	}
	waitForStatus(t, runner, holdRun.ID, RunStatusRunning)

	doneRun, err := runner.StartRun(RunRequest{Prompt: "done"})
	if err != nil {
		t.Fatalf("StartRun done: %v", err)
	}
	history, terminalStream, unsubscribe, err := runner.Subscribe(doneRun.ID)
	if err != nil {
		t.Fatalf("Subscribe done: %v", err)
	}
	defer unsubscribe()
	for _, event := range history {
		if IsTerminalEvent(event.Type) {
			t.Fatalf("done run unexpectedly terminal before provider release: %s", event.Type)
		}
	}
	backingStore.setBlockRunID(doneRun.ID)
	close(doneProviderRelease)

	select {
	case <-backingStore.started:
	case <-time.After(2 * time.Second):
		close(holdProviderRelease)
		t.Fatal("timed out waiting for terminal status store update to block")
	}

	blockedRun, ok := runner.GetRun(doneRun.ID)
	if !ok {
		t.Fatalf("GetRun(%q) returned false during terminal status persistence", doneRun.ID)
	}
	if isTerminalRunStatus(blockedRun.Status) {
		t.Fatalf("GetRun exposed %s before terminal status persistence completed", blockedRun.Status)
	}
	for {
		select {
		case event := <-terminalStream:
			if IsTerminalEvent(event.Type) {
				t.Fatalf("terminal event %s fanned out before final status persistence completed", event.Type)
			}
		default:
			goto streamDrained
		}
	}

streamDrained:

	emitDone := make(chan struct{})
	go func() {
		runner.emit(holdRun.ID, EventAssistantMessage, map[string]any{"content": "still responsive"})
		close(emitDone)
	}()
	select {
	case <-emitDone:
	case <-time.After(200 * time.Millisecond):
		close(backingStore.release)
		close(holdProviderRelease)
		<-emitDone
		t.Fatal("unrelated event journal blocked behind terminal status store update")
	}

	close(backingStore.release)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-terminalStream:
			if event.Type == EventRunCompleted {
				goto terminalDelivered
			}
		case <-deadline:
			t.Fatal("terminal event was not delivered after final status persistence completed")
		}
	}

terminalDelivered:
	storedRun, err := backingStore.Store.GetRun(context.Background(), doneRun.ID)
	if err != nil {
		t.Fatalf("store.GetRun(%q): %v", doneRun.ID, err)
	}
	if storedRun.Status != runstore.RunStatusCompleted || storedRun.Output != "ok: done" {
		t.Fatalf("stored run after terminal fanout = status %q output %q, want completed/ok: done",
			storedRun.Status, storedRun.Output)
	}
	close(holdProviderRelease)
	waitForStatus(t, runner, doneRun.ID, RunStatusCompleted)
	waitForStatus(t, runner, holdRun.ID, RunStatusCompleted)
}

type promptGateProvider struct {
	gates map[string]<-chan struct{}
}

func (p *promptGateProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	prompt := lastUserPrompt(req.Messages)
	if gate := p.gates[prompt]; gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return CompletionResult{}, ctx.Err()
		}
	}
	return CompletionResult{Content: "ok: " + prompt}, nil
}

func lastUserPrompt(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

type blockingTerminalAppendStore struct {
	runstore.Store

	mu         sync.Mutex
	blockRunID string
	once       sync.Once
	started    chan struct{}
	release    chan struct{}
}

type deadlineRecordingStore struct {
	runstore.Store
	deadline chan time.Duration
}

type blockingTerminalStatusStore struct {
	runstore.Store

	mu         sync.Mutex
	blockRunID string
	once       sync.Once
	started    chan struct{}
	release    chan struct{}
}

func (s *blockingTerminalStatusStore) setBlockRunID(runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockRunID = runID
}

func (s *blockingTerminalStatusStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	s.mu.Lock()
	blockRunID := s.blockRunID
	s.mu.Unlock()
	if run.ID == blockRunID && run.Status == runstore.RunStatusCompleted {
		s.once.Do(func() { close(s.started) })
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.Store.UpdateRun(ctx, run)
}

func (s *deadlineRecordingStore) AppendEvent(ctx context.Context, event *runstore.Event) error {
	if event.EventType == string(EventRunCompleted) {
		if deadline, ok := ctx.Deadline(); ok {
			select {
			case s.deadline <- time.Until(deadline):
			default:
			}
		}
	}
	return s.Store.AppendEvent(ctx, event)
}

func (s *blockingTerminalAppendStore) setBlockRunID(runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockRunID = runID
}

func (s *blockingTerminalAppendStore) AppendEvent(ctx context.Context, event *runstore.Event) error {
	s.mu.Lock()
	blockRunID := s.blockRunID
	s.mu.Unlock()

	if event.RunID == blockRunID && event.EventType == string(EventRunCompleted) {
		s.once.Do(func() { close(s.started) })
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.Store.AppendEvent(ctx, event)
}
