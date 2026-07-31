package harness

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-agent-harness/internal/forensics/redaction"
	runstore "go-agent-harness/internal/store"
)

func TestRunner_PruneCompletedRunsFromMemory(t *testing.T) {
	t.Parallel()

	runner := NewRunner(staticContentProvider{content: "done"}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 3,
		Store:                 runstore.NewMemoryStore(),
	})

	for i := 0; i < 8; i++ {
		run, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("run %d", i)})
		if err != nil {
			t.Fatalf("start run %d: %v", i, err)
		}
		if _, err := collectRunEvents(t, runner, run.ID); err != nil {
			t.Fatalf("collect run %d events: %v", i, err)
		}
	}

	waitForRunnerPrune(t, runner, func() bool {
		runner.mu.RLock()
		defer runner.mu.RUnlock()
		return len(runner.runs) <= 3
	})
}

func TestRunner_PruneWaitsForTerminalEventPersistence(t *testing.T) {
	release := make(chan struct{})
	runner := NewRunner(&blockingProvider{blocker: release}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
		Store:                 &terminalAppendFailStore{Store: runstore.NewMemoryStore()},
	})

	runIDs := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		run, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("unpersisted %d", i)})
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		runIDs = append(runIDs, run.ID)
	}
	close(release)
	for _, runID := range runIDs {
		waitForStatus(t, runner, runID, RunStatusCompleted)
	}

	runner.mu.RLock()
	if got := len(runner.runs); got != 3 {
		runner.mu.RUnlock()
		t.Fatalf("pruned terminal runs before terminal events persisted: got %d, want 3", got)
	}
	runner.mu.RUnlock()

	_, err := runner.StartRun(RunRequest{Prompt: "must fail closed"})
	requireTerminalDurabilityBackpressure(t, err, 3, 1)

	runner.mu.Lock()
	runner.runs["noncompleted-source"] = &runState{
		run:         Run{ID: "noncompleted-source", Status: RunStatusRunning},
		subscribers: make(map[chan Event]struct{}),
	}
	runner.mu.Unlock()
	if _, err := runner.ContinueRun("missing-source", "continue"); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("ContinueRun missing source error=%v, want ErrRunNotFound before backpressure", err)
	}
	if _, err := runner.ContinueRun("noncompleted-source", "continue"); !errors.Is(err, ErrRunNotCompleted) {
		t.Fatalf("ContinueRun noncompleted source error=%v, want ErrRunNotCompleted before backpressure", err)
	}
}

func TestRunner_PruneWaitsForTerminalStatusPersistence(t *testing.T) {
	release := make(chan struct{})
	store := &terminalFailureStore{
		Store:         runstore.NewMemoryStore(),
		failUpdateRun: true,
	}
	runner := NewRunner(&blockingProvider{blocker: release}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
		Store:                 store,
	})

	runIDs := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		run, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("status pending %d", i)})
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		runIDs = append(runIDs, run.ID)
	}
	close(release)
	for _, runID := range runIDs {
		waitForStatus(t, runner, runID, RunStatusCompleted)
	}

	runner.mu.RLock()
	if got := len(runner.runs); got != 3 {
		runner.mu.RUnlock()
		t.Fatalf("pruned terminal runs before terminal statuses persisted: got %d, want 3", got)
	}
	runner.mu.RUnlock()

	for _, runID := range runIDs {
		stored, err := store.Store.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetRun(%s): %v", runID, err)
		}
		if isTerminalStoreStatus(stored.Status) {
			t.Fatalf("durable run %s status=%s, want non-terminal after UpdateRun failure", runID, stored.Status)
		}
	}

	_, err := runner.StartRun(RunRequest{Prompt: "must fail closed"})
	requireTerminalDurabilityBackpressure(t, err, 3, 1)
}

func TestRunner_TerminalStatusPersistenceRecoveryAllowsConcurrentAdmissions(t *testing.T) {
	store := &recoveringTerminalStatusStore{Store: runstore.NewMemoryStore()}
	store.fail.Store(true)
	runner := NewRunner(staticContentProvider{content: "done"}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
		Store:                 store,
	})

	first, err := runner.StartRun(RunRequest{Prompt: "create pending terminal status"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, first.ID, RunStatusCompleted)

	const callers = 16
	rejectStart := make(chan struct{})
	rejectErrs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func(i int) {
			<-rejectStart
			_, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("still unavailable %d", i)})
			rejectErrs <- err
		}(i)
	}
	close(rejectStart)
	for i := 0; i < callers; i++ {
		requireTerminalDurabilityBackpressure(t, <-rejectErrs, 1, 1)
	}

	store.fail.Store(false)
	start := make(chan struct{})
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func(i int) {
			<-start
			_, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("recovered %d", i)})
			errs <- err
		}(i)
	}
	close(start)
	for i := 0; i < callers; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent recovered StartRun %d: %v", i, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := runner.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if got := store.successfulTerminalUpdates.Load(); got == 0 {
		t.Fatal("recovery never persisted a pending terminal status")
	}
}

func TestRunner_TerminalStatusRecoveryImmediatelyRestoresRetentionWindow(t *testing.T) {
	release := make(chan struct{})
	store := &recoveringTerminalStatusStore{Store: runstore.NewMemoryStore()}
	store.fail.Store(true)
	runner := NewRunner(&blockingProvider{blocker: release}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
		Store:                 store,
	})

	runIDs := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		run, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("recover and prune %d", i)})
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		runIDs = append(runIDs, run.ID)
	}
	close(release)
	for _, runID := range runIDs {
		waitForStatus(t, runner, runID, RunStatusCompleted)
	}

	store.fail.Store(false)
	if err := runner.ensureTerminalDurabilityCapacity(""); err != nil {
		t.Fatalf("ensureTerminalDurabilityCapacity after recovery: %v", err)
	}
	runner.mu.RLock()
	retained := len(runner.runs)
	runner.mu.RUnlock()
	if retained != 1 {
		t.Fatalf("recovered terminal states retained=%d, want exact retention window 1", retained)
	}
	for _, runID := range runIDs {
		stored, err := store.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatalf("store.GetRun(%s): %v", runID, err)
		}
		if stored.Status != runstore.RunStatusCompleted {
			t.Fatalf("stored run %s status=%s, want completed after recovery", runID, stored.Status)
		}
	}

	if _, err := runner.StartRun(RunRequest{Prompt: "admitted after bounded recovery"}); err != nil {
		t.Fatalf("StartRun after recovery: %v", err)
	}
}

func TestRunner_TerminalStatusRecoveryPreservesContinuationSource(t *testing.T) {
	release := make(chan struct{})
	store := &recoveringTerminalStatusStore{Store: runstore.NewMemoryStore()}
	store.fail.Store(true)
	runner := NewRunner(&blockingProvider{blocker: release}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
		Store:                 store,
	})

	runIDs := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		run, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("continuation recovery %d", i)})
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		runIDs = append(runIDs, run.ID)
	}
	close(release)
	for _, runID := range runIDs {
		waitForStatus(t, runner, runID, RunStatusCompleted)
	}

	store.fail.Store(false)
	continued, err := runner.ContinueRun(runIDs[0], "continue after persistence recovery")
	if err != nil {
		t.Fatalf("ContinueRun after recovery: %v", err)
	}
	if continued.ID == "" {
		t.Fatal("ContinueRun returned an empty run ID")
	}
}

func TestRunner_TerminalDurabilityAdmissionRetryUsesSingleUnlockedDeadline(t *testing.T) {
	release := make(chan struct{})
	store := &blockingAdmissionRecoveryStore{
		Store:   runstore.NewMemoryStore(),
		started: make(chan struct{}),
	}
	runner := NewRunner(&blockingProvider{blocker: release}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
		Store:                 store,
	})
	runner.terminalStoreTimeout = 200 * time.Millisecond

	runIDs := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		run, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("deadline pending %d", i)})
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		runIDs = append(runIDs, run.ID)
	}
	close(release)
	for _, runID := range runIDs {
		waitForStatus(t, runner, runID, RunStatusCompleted)
	}

	const unrelatedID = "admission-lock-control"
	runner.mu.Lock()
	runner.runs[unrelatedID] = &runState{
		run: Run{
			ID:             unrelatedID,
			ConversationID: "admission-lock-control-conversation",
			Status:         RunStatusRunning,
		},
		subscribers: make(map[chan Event]struct{}),
	}
	runner.mu.Unlock()
	runner.storeCreateRun(Run{ID: unrelatedID, ConversationID: "admission-lock-control-conversation", Status: RunStatusRunning})

	store.block.Store(true)
	admissionDone := make(chan error, 1)
	startedAt := time.Now()
	go func() {
		_, err := runner.StartRun(RunRequest{Prompt: "bounded blocked retry"})
		admissionDone <- err
	}()
	select {
	case <-store.started:
	case <-time.After(2 * time.Second):
		t.Fatal("admission recovery did not reach UpdateRun")
	}

	runner.mu.RLock()
	pendingStates := make([]*runState, 0, len(runIDs))
	for _, runID := range runIDs {
		pendingStates = append(pendingStates, runner.runs[runID])
	}
	runner.mu.RUnlock()
	for i, state := range pendingStates {
		if state == nil || !state.statusMu.TryLock() {
			t.Fatalf("pending status lock %d held during admission store I/O", i)
		}
		state.statusMu.Unlock()
	}

	getDone := make(chan struct{})
	go func() {
		_, _ = runner.GetRun(unrelatedID)
		close(getDone)
	}()
	select {
	case <-getDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("GetRun blocked behind admission recovery store I/O")
	}

	emitDone := make(chan struct{})
	go func() {
		runner.emit(unrelatedID, EventAssistantMessage, map[string]any{"content": "unrelated"})
		close(emitDone)
	}()
	select {
	case <-emitDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("conversation event journal blocked behind admission recovery store I/O")
	}

	select {
	case err := <-admissionDone:
		requireTerminalDurabilityBackpressure(t, err, 3, 1)
		if elapsed := time.Since(startedAt); elapsed >= 500*time.Millisecond {
			t.Fatalf("admission retry took %s, want one shared 200ms deadline rather than per-run waits", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("admission retry exceeded its shared deadline")
	}
}

func TestRunner_PruneTreatsStorageModeNoneAsIntentionalEventSuppression(t *testing.T) {
	pipeline := redaction.NewPipeline(
		redaction.NewRedactor(nil),
		redaction.EventClassConfig{string(EventRunCompleted): redaction.StorageModeNone},
	)
	store := runstore.NewMemoryStore()
	runner := NewRunner(staticContentProvider{content: "done"}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
		RedactionPipeline:     pipeline,
		Store:                 store,
	})

	for i := 0; i < 3; i++ {
		run, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("suppressed %d", i)})
		if err != nil {
			t.Fatalf("StartRun %d: %v", i, err)
		}
		waitForStatus(t, runner, run.ID, RunStatusCompleted)
		stored, err := store.GetRun(context.Background(), run.ID)
		if err != nil {
			t.Fatalf("store.GetRun(%s): %v", run.ID, err)
		}
		if stored.Status != runstore.RunStatusCompleted {
			t.Fatalf("stored status=%s, want completed", stored.Status)
		}
	}

	waitForRunnerPrune(t, runner, func() bool {
		runner.mu.RLock()
		defer runner.mu.RUnlock()
		return len(runner.runs) <= 1
	})
}

func TestRunner_NoStorePreservesInMemoryTerminalRunsWithoutBackpressure(t *testing.T) {
	runner := NewRunner(staticContentProvider{content: "done"}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
	})

	for i := 0; i < 3; i++ {
		run, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("memory only %d", i)})
		if err != nil {
			t.Fatalf("StartRun %d: %v", i, err)
		}
		waitForStatus(t, runner, run.ID, RunStatusCompleted)
	}
	runner.mu.RLock()
	retained := len(runner.runs)
	runner.mu.RUnlock()
	if retained != 3 {
		t.Fatalf("no-store retained runs=%d, want 3", retained)
	}
	if _, err := runner.StartRun(RunRequest{Prompt: "no-store remains available"}); err != nil {
		t.Fatalf("no-store StartRun unexpectedly backpressured: %v", err)
	}
}

func requireTerminalDurabilityBackpressure(
	t *testing.T,
	err error,
	wantPending, wantLimit int,
) {
	t.Helper()
	var backpressure *TerminalDurabilityBackpressureError
	if !errors.As(err, &backpressure) {
		t.Fatalf("error=%v, want TerminalDurabilityBackpressureError", err)
	}
	if backpressure.Pending != wantPending || backpressure.Limit != wantLimit {
		t.Fatalf("backpressure=%+v, want pending=%d limit=%d", backpressure, wantPending, wantLimit)
	}
}

type terminalAppendFailStore struct{ runstore.Store }

func (s *terminalAppendFailStore) AppendEvent(_ context.Context, event *runstore.Event) error {
	if event.EventType == string(EventRunCompleted) {
		return errors.New("terminal event store unavailable")
	}
	return s.Store.AppendEvent(context.Background(), event)
}

type recoveringTerminalStatusStore struct {
	runstore.Store
	fail                      atomic.Bool
	successfulTerminalUpdates atomic.Int64
}

func (s *recoveringTerminalStatusStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	if isTerminalStoreStatus(run.Status) {
		if s.fail.Load() {
			return errors.New("terminal status store unavailable")
		}
		s.successfulTerminalUpdates.Add(1)
	}
	return s.Store.UpdateRun(ctx, run)
}

type blockingAdmissionRecoveryStore struct {
	runstore.Store
	block       atomic.Bool
	started     chan struct{}
	startedOnce sync.Once
}

func (s *blockingAdmissionRecoveryStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	if !isTerminalStoreStatus(run.Status) {
		return s.Store.UpdateRun(ctx, run)
	}
	if s.block.Load() {
		s.startedOnce.Do(func() { close(s.started) })
		<-ctx.Done()
		return ctx.Err()
	}
	return errors.New("terminal status store unavailable")
}

func TestRunner_PruneKeepsCompletedRunWithActiveSubscriber(t *testing.T) {
	t.Parallel()

	runner := NewRunner(staticContentProvider{content: "done"}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
		Store:                 runstore.NewMemoryStore(),
	})

	pinned, err := runner.StartRun(RunRequest{Prompt: "keep subscriber"})
	if err != nil {
		t.Fatalf("start pinned run: %v", err)
	}
	history, stream, cancelPinned, err := runner.Subscribe(pinned.ID)
	if err != nil {
		t.Fatalf("subscribe pinned run: %v", err)
	}
	if !hasTerminalEvent(history) {
		for ev := range stream {
			if IsTerminalEvent(ev.Type) {
				break
			}
		}
	}

	extraRunIDs := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		run, err := runner.StartRun(RunRequest{Prompt: fmt.Sprintf("extra %d", i)})
		if err != nil {
			t.Fatalf("start extra run %d: %v", i, err)
		}
		extraRunIDs = append(extraRunIDs, run.ID)
		if _, err := collectRunEvents(t, runner, run.ID); err != nil {
			t.Fatalf("collect extra run %d events: %v", i, err)
		}
	}

	waitForRunnerPrune(t, runner, func() bool {
		runner.mu.RLock()
		defer runner.mu.RUnlock()
		if len(runner.runs) != 2 {
			return false
		}
		if _, ok := runner.runs[pinned.ID]; !ok {
			return false
		}
		if _, ok := runner.runs[extraRunIDs[len(extraRunIDs)-1]]; !ok {
			return false
		}
		for _, runID := range extraRunIDs[:len(extraRunIDs)-1] {
			if _, ok := runner.runs[runID]; ok {
				return false
			}
		}
		return true
	})

	cancelPinned()

	replacement, err := runner.StartRun(RunRequest{Prompt: "replacement"})
	if err != nil {
		t.Fatalf("start replacement run: %v", err)
	}
	if _, err := collectRunEvents(t, runner, replacement.ID); err != nil {
		t.Fatalf("collect replacement events: %v", err)
	}

	waitForRunnerPrune(t, runner, func() bool {
		runner.mu.RLock()
		defer runner.mu.RUnlock()
		_, stillPresent := runner.runs[pinned.ID]
		return !stillPresent && len(runner.runs) <= 1
	})
}

func TestRunner_PruneConversationMirrorFallsBackToPersistentStore(t *testing.T) {
	t.Parallel()

	store := newMemoryConversationStore()
	runner := NewRunner(staticContentProvider{content: "done"}, NewRegistry(), RunnerConfig{
		DefaultModel:             "test-model",
		MaxSteps:                 1,
		MaxCompletedRetention:    8,
		MaxConversationRetention: 2,
		ConversationStore:        store,
		Store:                    runstore.NewMemoryStore(),
	})

	const oldConvID = "conv-0"
	for i := 0; i < 5; i++ {
		run, err := runner.StartRun(RunRequest{
			Prompt:         fmt.Sprintf("conversation %d", i),
			ConversationID: fmt.Sprintf("conv-%d", i),
		})
		if err != nil {
			t.Fatalf("start run %d: %v", i, err)
		}
		if _, err := collectRunEvents(t, runner, run.ID); err != nil {
			t.Fatalf("collect run %d events: %v", i, err)
		}
	}

	waitForRunnerPrune(t, runner, func() bool {
		runner.mu.RLock()
		defer runner.mu.RUnlock()
		return len(runner.conversations) <= 2
	})

	runner.mu.RLock()
	_, inMemory := runner.conversations[oldConvID]
	runner.mu.RUnlock()
	if inMemory {
		t.Fatalf("%s still present in in-memory conversation mirror", oldConvID)
	}

	msgs, ok := runner.ConversationMessages(oldConvID)
	if !ok {
		t.Fatalf("%s should still load from the persistent conversation store", oldConvID)
	}
	if len(msgs) == 0 {
		t.Fatalf("%s loaded from store with no messages", oldConvID)
	}
}

func waitForRunnerPrune(t *testing.T, runner *Runner, done func() bool) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if done() {
			return
		}
		select {
		case <-deadline:
			runner.mu.RLock()
			runCount := len(runner.runs)
			conversationCount := len(runner.conversations)
			runner.mu.RUnlock()
			t.Fatalf("timed out waiting for prune; runs=%d conversations=%d", runCount, conversationCount)
		case <-ticker.C:
		}
	}
}

type staticContentProvider struct {
	content string
}

func (p staticContentProvider) Complete(context.Context, CompletionRequest) (CompletionResult, error) {
	return CompletionResult{Content: p.content}, nil
}

type memoryConversationStore struct {
	mu       sync.Mutex
	messages map[string][]Message
	owners   map[string]*Conversation
}

func newMemoryConversationStore() *memoryConversationStore {
	return &memoryConversationStore{
		messages: make(map[string][]Message),
		owners:   make(map[string]*Conversation),
	}
}

func (s *memoryConversationStore) Migrate(context.Context) error { return nil }

func (s *memoryConversationStore) Close() error { return nil }

func (s *memoryConversationStore) SaveConversation(ctx context.Context, convID string, msgs []Message) error {
	return s.SaveConversationWithCost(ctx, convID, msgs, ConversationTokenCost{})
}

func (s *memoryConversationStore) SaveConversationWithCost(_ context.Context, convID string, msgs []Message, _ ConversationTokenCost) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages[convID] = copyMessages(msgs)
	now := time.Now().UTC()
	conv := s.owners[convID]
	if conv == nil {
		conv = &Conversation{ID: convID, CreatedAt: now}
		s.owners[convID] = conv
	}
	conv.UpdatedAt = now
	conv.MsgCount = len(msgs)
	return nil
}

func (s *memoryConversationStore) LoadMessages(_ context.Context, convID string) ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return copyMessages(s.messages[convID]), nil
}

func (s *memoryConversationStore) ListConversations(context.Context, ConversationFilter, int, int) ([]Conversation, error) {
	return nil, nil
}

func (s *memoryConversationStore) DeleteConversation(_ context.Context, convID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.messages, convID)
	delete(s.owners, convID)
	return nil
}

func (s *memoryConversationStore) UpdateConversationMeta(_ context.Context, convID, workspace, tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	conv := s.owners[convID]
	if conv == nil {
		conv = &Conversation{ID: convID, CreatedAt: time.Now().UTC()}
		s.owners[convID] = conv
	}
	conv.Workspace = workspace
	conv.TenantID = tenantID
	return nil
}

func (s *memoryConversationStore) GetConversationOwner(_ context.Context, convID string) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	conv := s.owners[convID]
	if conv == nil {
		return nil, nil
	}
	out := *conv
	return &out, nil
}

func (s *memoryConversationStore) SearchMessages(context.Context, string, string, int) ([]MessageSearchResult, error) {
	return nil, nil
}

func (s *memoryConversationStore) DeleteOldConversations(context.Context, time.Time) (int, error) {
	return 0, nil
}

func (s *memoryConversationStore) PinConversation(context.Context, string, bool) error {
	return nil
}

func (s *memoryConversationStore) CompactConversation(context.Context, string, int, Message) error {
	return nil
}

func (s *memoryConversationStore) UndoPrompts(context.Context, string, int) (int, error) {
	return 0, nil
}

func (s *memoryConversationStore) ForkConversation(_ context.Context, srcID, newID string) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.owners[srcID]
	if !ok {
		return nil, fmt.Errorf("fork: source conversation %q not found", srcID)
	}
	if _, taken := s.owners[newID]; taken {
		return nil, fmt.Errorf("fork: target conversation %q already exists", newID)
	}
	s.messages[newID] = copyMessages(s.messages[srcID])
	now := time.Now().UTC()
	fork := &Conversation{
		ID:        newID,
		Title:     src.Title,
		CreatedAt: now,
		UpdatedAt: now,
		MsgCount:  len(s.messages[newID]),
		Workspace: src.Workspace,
		TenantID:  src.TenantID,
	}
	s.owners[newID] = fork
	out := *fork
	return &out, nil
}

// TestRunner_DropConversationCacheFallsBackToStore (epic #805 slice 3 bug
// regression): after an external truncation of the persistent store (what
// POST /v1/conversations/{id}/undo does), the in-memory conversation mirror
// keeps serving the stale pre-undo history until the cache entry is dropped.
// DropConversationCache must force the next ConversationMessages call to fall
// back to the truncated store.
func TestRunner_DropConversationCacheFallsBackToStore(t *testing.T) {
	t.Parallel()

	store := newTestConversationStore(t) // real SQLite so UndoPrompts truncates
	runner := NewRunner(staticContentProvider{content: "done"}, NewRegistry(), RunnerConfig{
		DefaultModel:      "test-model",
		MaxSteps:          1,
		ConversationStore: store,
		Store:             runstore.NewMemoryStore(),
	})

	for _, prompt := range []string{"first-prompt", "second-prompt"} {
		run, err := runner.StartRun(RunRequest{Prompt: prompt, ConversationID: "conv-drop"})
		if err != nil {
			t.Fatalf("start run %q: %v", prompt, err)
		}
		if _, err := collectRunEvents(t, runner, run.ID); err != nil {
			t.Fatalf("collect run %q events: %v", prompt, err)
		}
	}

	// Wait for the terminal persistence to land in the store (the mirror is
	// written first; the store write trails it in the same cleanup path).
	// Each run also injects an is_meta "1 step remaining" nudge (MaxSteps: 1),
	// so two runs persist 6 messages.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if msgs, err := store.LoadMessages(context.Background(), "conv-drop"); err == nil && len(msgs) == 6 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for store persistence")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Sanity: the mirror serves all 6 messages.
	if msgs, ok := runner.ConversationMessages("conv-drop"); !ok || len(msgs) != 6 {
		t.Fatalf("pre-undo mirror: got %d messages (ok=%v), want 6", len(msgs), ok)
	}

	// External truncation behind the runner's back (what /undo does).
	if _, err := store.UndoPrompts(context.Background(), "conv-drop", 1); err != nil {
		t.Fatalf("UndoPrompts: %v", err)
	}

	// The mirror is stale until the cache entry is dropped.
	if msgs, _ := runner.ConversationMessages("conv-drop"); len(msgs) != 6 {
		t.Fatalf("expected stale mirror of 6 messages before drop, got %d", len(msgs))
	}

	runner.DropConversationCache("conv-drop")

	// Now the view falls back to the truncated store: kept run + is_meta marker.
	msgs, ok := runner.ConversationMessages("conv-drop")
	if !ok {
		t.Fatal("ConversationMessages after drop: not found")
	}
	if len(msgs) != 4 {
		t.Fatalf("post-drop messages: got %d, want 4 (kept run + marker): %+v", len(msgs), msgs)
	}
	if msgs[0].Content != "first-prompt" {
		t.Errorf("msgs[0].Content: got %q, want %q", msgs[0].Content, "first-prompt")
	}
	if !msgs[3].IsMeta {
		t.Errorf("msgs[3]: expected the is_meta undo-boundary marker, got %+v", msgs[3])
	}
}
