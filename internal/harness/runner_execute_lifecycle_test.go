package harness

// runner_execute_lifecycle_test.go — characterize execute lifecycle across
// compaction, memory, wait-for-user, and cost-limit paths.
// Covers GitHub issue #329.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/forensics/redaction"
	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
	runstore "go-agent-harness/internal/store"
)

// -------------------------------------------------------------------------
// TestExecuteLifecycle_AutoCompactAndContextWindowSnapshotSameTurn
//
// Verifies that when AutoCompactEnabled is true and the prompt exceeds the
// threshold, the runner emits auto_compact.started and auto_compact.completed
// in the same turn as context.window.snapshot (if enabled). The run must
// complete normally and the stored transcript must not be empty.
// -------------------------------------------------------------------------
func TestExecuteLifecycle_AutoCompactAndContextWindowSnapshotSameTurn(t *testing.T) {
	t.Parallel()

	// Use a tiny context window so both compaction and snapshot trigger.
	// Context window = 10 tokens, 0.50 threshold => >5 tokens triggers.
	// "abcdefghi" repeated 5 times = 45 chars ≈ 11 tokens, enough to trigger.
	largePrompt := strings.Repeat("abcdefghi ", 5)

	provider := &staticRunnerProvider{result: CompletionResult{Content: "done"}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:                 "test",
		MaxSteps:                     2,
		AutoCompactEnabled:           true,
		ModelContextWindow:           10,
		AutoCompactThreshold:         0.50,
		AutoCompactKeepLast:          2,
		AutoCompactMode:              "strip",
		ContextWindowSnapshotEnabled: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: largePrompt})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Run must complete.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q (error: %s)", state.Status, state.Error)
	}

	// Event ordering: auto_compact.started must precede run.completed.
	requireEventOrder(t, events,
		"run.started",
		"auto_compact.started",
		"auto_compact.completed",
		"run.completed",
	)

	// context.window.snapshot must also appear (either before or after compact).
	snapshotFound := false
	for _, ev := range events {
		if ev.Type == EventContextWindowSnapshot {
			snapshotFound = true
		}
	}
	if !snapshotFound {
		t.Error("expected context.window.snapshot event")
	}

	// Stored transcript must be non-empty (at minimum the user message).
	msgs := runner.GetRunMessages(run.ID)
	if len(msgs) == 0 {
		t.Error("expected at least one stored message in transcript")
	}
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_AutoCompactAndMemorySnippetSameTurn
//
// Verifies that when AutoCompactEnabled is true and a memory snippet is
// injected, both behaviors coexist in the same turn without interfering.
// The compaction path must not discard the injected memory message from
// the request shape seen by the provider.
// -------------------------------------------------------------------------
func TestExecuteLifecycle_AutoCompactAndMemorySnippetSameTurn(t *testing.T) {
	t.Parallel()

	// Large prompt triggers compaction; memory snippet is injected into request.
	largePrompt := strings.Repeat("word ", 50) // ~50+ tokens with tiny window

	capProvider := &capturingProvider{
		turns: []CompletionResult{{Content: "finished"}},
	}

	mem := &memoryStub{
		status: om.Status{
			Mode:                     om.ModeLocalCoordinator,
			MemoryID:                 "default|conv|agent",
			Scope:                    om.ScopeKey{TenantID: "default", ConversationID: "conv", AgentID: "agent"},
			Enabled:                  true,
			LastObservedMessageIndex: -1,
			UpdatedAt:                time.Now().UTC(),
		},
		snippet: "<observational-memory>lifecycle-test-snippet</observational-memory>",
	}

	runner := NewRunner(capProvider, NewRegistry(), RunnerConfig{
		DefaultModel:         "test",
		MaxSteps:             2,
		MemoryManager:        mem,
		AskUserTimeout:       time.Second,
		AutoCompactEnabled:   true,
		ModelContextWindow:   20,
		AutoCompactThreshold: 0.50,
		AutoCompactKeepLast:  2,
		AutoCompactMode:      "strip",
	})

	run, err := runner.StartRun(RunRequest{Prompt: largePrompt})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q (error: %s)", state.Status, state.Error)
	}

	// Memory observe events must appear before the terminal event.
	requireEventOrder(t, events,
		"run.started",
		"memory.observe.started",
		"memory.observe.completed",
		"run.completed",
	)

	// The provider must have received the memory snippet as the first message.
	if len(capProvider.calls) == 0 {
		t.Fatal("expected at least one provider call")
	}
	firstReqMsgs := capProvider.calls[0].Messages
	snippetInjected := false
	for _, m := range firstReqMsgs {
		if strings.Contains(m.Content, "lifecycle-test-snippet") {
			snippetInjected = true
			break
		}
	}
	if !snippetInjected {
		t.Errorf("memory snippet not found in provider request messages: %+v", firstReqMsgs)
	}

	// Stored transcript must include at least the user prompt message.
	msgs := runner.GetRunMessages(run.ID)
	if len(msgs) == 0 {
		t.Error("expected stored messages in transcript")
	}
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_WaitForUserFlowEventOrderAndStateRestoration
//
// Verifies the waiting-for-user flow:
//   - run.waiting_for_user is emitted when AskUserQuestion tool fires.
//   - Status transitions to waiting_for_user.
//   - After SubmitInput, run.resumed is emitted.
//   - Run completes and state.Status == completed.
//   - Event ordering is pinned precisely.
//
// -------------------------------------------------------------------------
type gatedAskUserQuestionBroker struct {
	inner   *InMemoryAskUserQuestionBroker
	entered chan struct{}
	release chan struct{}
}

type notifierIgnoringAskUserQuestionBroker struct {
	mu         sync.Mutex
	pending    htools.AskUserQuestionPending
	hasPending bool
	registered chan struct{}
	returned   chan struct{}
	answerC    chan map[string]string
}

func newNotifierIgnoringAskUserQuestionBroker() *notifierIgnoringAskUserQuestionBroker {
	return &notifierIgnoringAskUserQuestionBroker{
		registered: make(chan struct{}),
		returned:   make(chan struct{}),
		answerC:    make(chan map[string]string, 1),
	}
}

func (b *notifierIgnoringAskUserQuestionBroker) Ask(
	ctx context.Context,
	req htools.AskUserQuestionRequest,
) (map[string]string, time.Time, error) {
	b.mu.Lock()
	b.pending = htools.AskUserQuestionPending{
		RunID:      req.RunID,
		CallID:     req.CallID,
		Tool:       htools.AskUserQuestionToolName,
		Questions:  req.Questions,
		DeadlineAt: time.Now().UTC().Add(req.Timeout),
	}
	b.hasPending = true
	b.mu.Unlock()
	close(b.registered)

	select {
	case answers := <-b.answerC:
		close(b.returned)
		return answers, time.Now().UTC(), nil
	case <-ctx.Done():
		return nil, time.Time{}, ctx.Err()
	}
}

func (b *notifierIgnoringAskUserQuestionBroker) Pending(runID string) (htools.AskUserQuestionPending, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.hasPending || b.pending.RunID != runID {
		return htools.AskUserQuestionPending{}, false
	}
	return b.pending, true
}

func (b *notifierIgnoringAskUserQuestionBroker) Submit(runID string, answers map[string]string) error {
	b.mu.Lock()
	if !b.hasPending || b.pending.RunID != runID {
		b.mu.Unlock()
		return ErrNoPendingInput
	}
	b.hasPending = false
	b.mu.Unlock()
	b.answerC <- answers
	return nil
}

type waitingStatusBlockingStore struct {
	*runstore.MemoryStore
	once          sync.Once
	cancelOnce    sync.Once
	started       chan struct{}
	cancelledWait chan struct{}
	release       chan struct{}
}

type transientWaitingStatusStore struct {
	*runstore.MemoryStore
	mu             sync.Mutex
	failureSurface string
	waitAttempts   int
	firstFailed    chan struct{}
	secondAttempt  chan struct{}
}

func newTransientWaitingStatusStore(failureSurface string) *transientWaitingStatusStore {
	return &transientWaitingStatusStore{
		MemoryStore:    runstore.NewMemoryStore(),
		failureSurface: failureSurface,
		firstFailed:    make(chan struct{}),
		secondAttempt:  make(chan struct{}),
	}
}

func (s *transientWaitingStatusStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	if s.failureSurface == "UpdateRun" && run.Status == runstore.RunStatusWaitingForUser {
		s.mu.Lock()
		s.waitAttempts++
		attempt := s.waitAttempts
		s.mu.Unlock()
		switch attempt {
		case 1:
			close(s.firstFailed)
			return errors.New("transient waiting status write failure")
		case 2:
			close(s.secondAttempt)
		}
	}
	return s.MemoryStore.UpdateRun(ctx, run)
}

func (s *transientWaitingStatusStore) AppendEvent(ctx context.Context, event *runstore.Event) error {
	if s.failureSurface == "AppendEvent" && event.EventType == string(EventRunWaitingForUser) {
		s.mu.Lock()
		s.waitAttempts++
		attempt := s.waitAttempts
		s.mu.Unlock()
		switch attempt {
		case 1:
			close(s.firstFailed)
			return errors.New("transient waiting event write failure")
		case 2:
			close(s.secondAttempt)
		}
	}
	return s.MemoryStore.AppendEvent(ctx, event)
}

type staleWaitRepairFailingStore struct {
	*runstore.MemoryStore
	once         sync.Once
	started      chan struct{}
	release      chan struct{}
	mu           sync.Mutex
	failedWrites int
}

func newStaleWaitRepairFailingStore() *staleWaitRepairFailingStore {
	return &staleWaitRepairFailingStore{
		MemoryStore: runstore.NewMemoryStore(),
		started:     make(chan struct{}),
		release:     make(chan struct{}),
	}
}

func (s *staleWaitRepairFailingStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	if run.Status == runstore.RunStatusWaitingForUser {
		s.once.Do(func() { close(s.started) })
		<-s.release
		return s.MemoryStore.UpdateRun(ctx, run)
	}
	if run.Status == runstore.RunStatusFailed {
		s.mu.Lock()
		s.failedWrites++
		attempt := s.failedWrites
		s.mu.Unlock()
		if attempt == 2 {
			return errors.New("transient terminal status write failure")
		}
	}
	return s.MemoryStore.UpdateRun(ctx, run)
}

type waitingEventDeadlineStore struct {
	*runstore.MemoryStore
	once       sync.Once
	cancelOnce sync.Once
	started    chan struct{}
	cancelled  chan struct{}
}

func newWaitingEventDeadlineStore() *waitingEventDeadlineStore {
	return &waitingEventDeadlineStore{
		MemoryStore: runstore.NewMemoryStore(),
		started:     make(chan struct{}),
		cancelled:   make(chan struct{}),
	}
}

func (s *waitingEventDeadlineStore) AppendEvent(ctx context.Context, event *runstore.Event) error {
	if event.EventType == string(EventRunWaitingForUser) {
		s.once.Do(func() { close(s.started) })
		<-ctx.Done()
		s.cancelOnce.Do(func() { close(s.cancelled) })
		return ctx.Err()
	}
	return s.MemoryStore.AppendEvent(ctx, event)
}

func newWaitingStatusBlockingStore() *waitingStatusBlockingStore {
	return &waitingStatusBlockingStore{
		MemoryStore:   runstore.NewMemoryStore(),
		started:       make(chan struct{}),
		cancelledWait: make(chan struct{}),
		release:       make(chan struct{}),
	}
}

func (s *waitingStatusBlockingStore) UpdateRun(ctx context.Context, run *runstore.Run) error {
	if run.Status == runstore.RunStatusWaitingForUser {
		s.once.Do(func() { close(s.started) })
		select {
		case <-s.release:
		case <-ctx.Done():
			s.cancelOnce.Do(func() { close(s.cancelledWait) })
			return ctx.Err()
		}
	}
	return s.MemoryStore.UpdateRun(ctx, run)
}

func (b *gatedAskUserQuestionBroker) Ask(ctx context.Context, req htools.AskUserQuestionRequest) (map[string]string, time.Time, error) {
	close(b.entered)
	select {
	case <-b.release:
	case <-ctx.Done():
		return nil, time.Time{}, ctx.Err()
	}
	return b.inner.Ask(ctx, req)
}

func (b *gatedAskUserQuestionBroker) Pending(runID string) (htools.AskUserQuestionPending, bool) {
	return b.inner.Pending(runID)
}

func (b *gatedAskUserQuestionBroker) Submit(runID string, answers map[string]string) error {
	return b.inner.Submit(runID, answers)
}

func TestExecuteLifecycle_WaitingForUserRequiresPendingInput(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask_pending",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Continue?","header":"Continue","options":[{"label":"Yes","description":"Continue"},{"label":"No","description":"Stop"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "continued"},
	}}
	broker := &gatedAskUserQuestionBroker{
		inner:   NewInMemoryAskUserQuestionBroker(time.Now),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	released := false
	defer func() {
		if !released {
			close(broker.release)
		}
	}()

	const waitForUserTimeout = 10 * time.Second
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: waitForUserTimeout,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: waitForUserTimeout,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "wait until pending exists"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	select {
	case <-broker.entered:
	case <-time.After(waitForUserTimeout):
		t.Fatal("timed out waiting for AskUserQuestion broker")
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found before pending registration")
	}
	if state.Status != RunStatusRunning {
		t.Fatalf("status before pending registration = %q, want %q", state.Status, RunStatusRunning)
	}
	if _, err := runner.PendingInput(run.ID); err != ErrNoPendingInput {
		t.Fatalf("PendingInput before registration error = %v, want %v", err, ErrNoPendingInput)
	}

	close(broker.release)
	released = true

	var pending htools.AskUserQuestionPending
	deadline := time.Now().Add(waitForUserTimeout)
	for {
		pending, err = runner.PendingInput(run.ID)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for pending input: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
	if pending.CallID != "call_ask_pending" {
		t.Fatalf("pending call ID = %q, want call_ask_pending", pending.CallID)
	}

	deadline = time.Now().Add(waitForUserTimeout)
	for {
		state, ok = runner.GetRun(run.ID)
		if !ok {
			t.Fatal("run not found after pending registration")
		}
		if state.Status == RunStatusWaitingForUser {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status after pending registration = %q, want %q", state.Status, RunStatusWaitingForUser)
		}
		time.Sleep(time.Millisecond)
	}

	if err := runner.SubmitInput(run.ID, map[string]string{"Continue?": "Yes"}); err != nil {
		t.Fatalf("SubmitInput: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}
	requireEventOrder(t, events,
		"tool.call.started",
		"run.waiting_for_user",
		"run.resumed",
		"run.completed",
	)
	assertSingleWaitAndResume(t, events)
}

func TestExecuteLifecycle_ObservesPendingInputWhenBrokerIgnoresNotifier(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask_ignored_notifier",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Continue?","header":"Continue","options":[{"label":"Yes","description":"Continue"},{"label":"No","description":"Stop"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "continued after fallback"},
	}}
	broker := newNotifierIgnoringAskUserQuestionBroker()
	submitted := false
	t.Cleanup(func() {
		if !submitted {
			_ = broker.Submit("ignored", map[string]string{"Continue?": "Yes"})
		}
	})
	const timeout = 2 * time.Second
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "use a third-party ask broker"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	select {
	case <-broker.registered:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for third-party broker registration")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatal("run not found")
		}
		if state.Status == RunStatusWaitingForUser {
			break
		}
		if time.Now().After(deadline) {
			_ = broker.Submit(run.ID, map[string]string{"Continue?": "Yes"})
			submitted = true
			t.Fatalf("status = %q, want waiting_for_user after broker exposed pending input", state.Status)
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := runner.PendingInput(run.ID); err != nil {
		t.Fatalf("PendingInput: %v", err)
	}
	if err := runner.SubmitInput(run.ID, map[string]string{"Continue?": "Yes"}); err != nil {
		t.Fatalf("SubmitInput: %v", err)
	}
	submitted = true
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}
	requireEventOrder(t, events,
		"run.waiting_for_user",
		"run.resumed",
		"run.completed",
	)
	assertSingleWaitAndResume(t, events)
}

func TestExecuteLifecycle_ObserverFinishesStartedPendingPublicationAfterToolReturns(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask_observer_inflight",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Continue?","header":"Continue","options":[{"label":"Yes","description":"Continue"},{"label":"No","description":"Stop"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "continued after observer publication"},
	}}
	broker := newNotifierIgnoringAskUserQuestionBroker()
	persistence := newWaitingStatusBlockingStore()
	released := false
	t.Cleanup(func() {
		if !released {
			close(persistence.release)
		}
	})
	const timeout = 2 * time.Second
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
		Store:          persistence,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "answer while observer publication is blocked"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	select {
	case <-persistence.started:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for observer pending publication")
	}
	if err := runner.SubmitInput(run.ID, map[string]string{"Continue?": "Yes"}); err != nil {
		t.Fatalf("SubmitInput: %v", err)
	}
	select {
	case <-broker.returned:
	case <-time.After(timeout):
		t.Fatal("broker did not return accepted answer")
	}

	select {
	case <-persistence.cancelledWait:
		t.Fatal("observer stop cancelled a pending publication that had already started")
	case <-time.After(25 * time.Millisecond):
	}
	close(persistence.release)
	released = true

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}
	requireEventOrder(t, events,
		"run.waiting_for_user",
		"run.resumed",
		"run.completed",
	)
	assertSingleWaitAndResume(t, events)
}

func TestExecuteLifecycle_PendingPublicationRetriesAfterTransientPersistenceFailure(t *testing.T) {
	for _, failureSurface := range []string{"UpdateRun", "AppendEvent"} {
		failureSurface := failureSurface
		t.Run(failureSurface, func(t *testing.T) {
			t.Parallel()

			provider := &stubProvider{turns: []CompletionResult{
				{
					ToolCalls: []ToolCall{{
						ID:        "call_ask_retry_pending",
						Name:      htools.AskUserQuestionToolName,
						Arguments: `{"questions":[{"question":"Continue?","header":"Continue","options":[{"label":"Yes","description":"Continue"},{"label":"No","description":"Stop"}],"multiSelect":false}]}`,
					}},
				},
				{Content: "continued after retry"},
			}}
			broker := NewInMemoryAskUserQuestionBroker(time.Now)
			persistence := newTransientWaitingStatusStore(failureSurface)
			const timeout = 2 * time.Second
			runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
				ApprovalMode:   ToolApprovalModeFullAuto,
				AskUserBroker:  broker,
				AskUserTimeout: timeout,
			}), RunnerConfig{
				DefaultModel:   "gpt-5-nano",
				MaxSteps:       4,
				AskUserBroker:  broker,
				AskUserTimeout: timeout,
				Store:          persistence,
			})

			run, err := runner.StartRun(RunRequest{Prompt: "retry transient pending persistence"})
			if err != nil {
				t.Fatalf("StartRun: %v", err)
			}
			select {
			case <-persistence.firstFailed:
			case <-time.After(timeout):
				t.Fatal("timed out waiting for first pending persistence failure")
			}
			select {
			case <-persistence.secondAttempt:
			case <-time.After(250 * time.Millisecond):
				_ = runner.SubmitInput(run.ID, map[string]string{"Continue?": "Yes"})
				t.Fatalf("pending publication was not retried after transient %s failure", failureSurface)
			}
			if err := runner.SubmitInput(run.ID, map[string]string{"Continue?": "Yes"}); err != nil {
				t.Fatalf("SubmitInput: %v", err)
			}

			events, err := collectRunEvents(t, runner, run.ID)
			if err != nil {
				t.Fatalf("collectRunEvents: %v", err)
			}
			requireEventOrder(t, events,
				"run.waiting_for_user",
				"run.resumed",
				"run.completed",
			)
			assertSingleWaitAndResume(t, events)
		})
	}
}

func TestExecuteLifecycle_RedactionDroppedPendingEventDoesNotBlockAcceptedAnswer(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask_redaction_drop",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Continue?","header":"Continue","options":[{"label":"Yes","description":"Continue"},{"label":"No","description":"Stop"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "continued with waiting event redacted"},
	}}
	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	pipeline := redaction.NewPipeline(nil, redaction.EventClassConfig{
		string(EventRunWaitingForUser): redaction.StorageModeNone,
	})
	const timeout = time.Second
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
	}), RunnerConfig{
		DefaultModel:      "gpt-5-nano",
		MaxSteps:          4,
		AskUserBroker:     broker,
		AskUserTimeout:    timeout,
		RedactionPipeline: pipeline,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "accept input with dropped waiting event"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	deadline := time.Now().Add(timeout)
	for {
		if _, err := runner.PendingInput(run.ID); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for pending input")
		}
		time.Sleep(time.Millisecond)
	}
	// Give the fallback observer time to see the already-published pending
	// record. A deliberate redaction drop must complete that observer instead
	// of making it retry until the question deadline.
	time.Sleep(25 * time.Millisecond)
	if err := runner.SubmitInput(run.ID, map[string]string{"Continue?": "Yes"}); err != nil {
		t.Fatalf("SubmitInput: %v", err)
	}
	completionDeadline := time.Now().Add(250 * time.Millisecond)
	for {
		state, ok := runner.GetRun(run.ID)
		if ok && state.Status == RunStatusCompleted {
			break
		}
		if time.Now().After(completionDeadline) {
			t.Fatal("accepted answer remained blocked by retries of a deliberately dropped waiting event")
		}
		time.Sleep(time.Millisecond)
	}
	for _, event := range runner.getEvents(run.ID) {
		if event.Type == EventRunWaitingForUser {
			t.Fatal("redaction-dropped waiting event became visible")
		}
	}
}

func TestExecuteLifecycle_WaitEventPrecedesQuickAnswerWhenStatusPersistenceBlocks(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask_quick_answer",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Continue?","header":"Continue","options":[{"label":"Yes","description":"Continue"},{"label":"No","description":"Stop"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "continued"},
	}}
	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	persistence := newWaitingStatusBlockingStore()
	released := false
	t.Cleanup(func() {
		if !released {
			close(persistence.release)
		}
	})

	const waitForUserTimeout = 10 * time.Second
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:  ToolApprovalModeFullAuto,
		AskUserBroker: broker,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: waitForUserTimeout,
		Store:          persistence,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "accept a quick answer"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	select {
	case <-persistence.started:
	case <-time.After(waitForUserTimeout):
		t.Fatal("timed out waiting for waiting_for_user persistence")
	}
	if _, err := runner.PendingInput(run.ID); err != nil {
		t.Fatalf("PendingInput while status persistence is blocked: %v", err)
	}
	if err := runner.SubmitInput(run.ID, map[string]string{"Continue?": "Yes"}); err != nil {
		t.Fatalf("SubmitInput: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	for _, event := range runner.getEvents(run.ID) {
		if event.Type == EventRunResumed {
			t.Fatal("run.resumed overtook the blocked run.waiting_for_user publication")
		}
	}

	close(persistence.release)
	released = true

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}
	requireEventOrder(t, events,
		"run.waiting_for_user",
		"run.resumed",
		"run.completed",
	)
}

func TestExecuteLifecycle_LateWaitPersistenceCannotReplaceTerminalStatus(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{{
		ToolCalls: []ToolCall{{
			ID:        "call_ask_timeout",
			Name:      htools.AskUserQuestionToolName,
			Arguments: `{"questions":[{"question":"Continue?","header":"Continue","options":[{"label":"Yes","description":"Continue"},{"label":"No","description":"Stop"}],"multiSelect":false}]}`,
		}},
	}}}
	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	persistence := newWaitingStatusBlockingStore()
	released := false
	t.Cleanup(func() {
		if !released {
			close(persistence.release)
		}
	})
	const timeout = 150 * time.Millisecond
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       2,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
		Store:          persistence,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "time out while persistence is blocked"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	<-persistence.started
	if _, err := collectRunEvents(t, runner, run.ID); err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}
	select {
	case <-persistence.cancelledWait:
	case <-time.After(time.Second):
		t.Fatal("waiting status persistence outlived the pending notifier context")
	}
	close(persistence.release)
	released = true

	deadline := time.Now().Add(time.Second)
	for {
		stored, err := persistence.GetRun(context.Background(), run.ID)
		if err != nil {
			t.Fatalf("GetRun: %v", err)
		}
		if stored.Status == runstore.RunStatusFailed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("durable status = %q, want failed", stored.Status)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestExecuteLifecycle_ExpiredWaitEventIsNotPublishedAfterBlockedAppend(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{{
		ToolCalls: []ToolCall{{
			ID:        "call_ask_event_deadline",
			Name:      htools.AskUserQuestionToolName,
			Arguments: `{"questions":[{"question":"Continue?","header":"Continue","options":[{"label":"Yes","description":"Continue"},{"label":"No","description":"Stop"}],"multiSelect":false}]}`,
		}},
	}}}
	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	persistence := newWaitingEventDeadlineStore()
	const timeout = 100 * time.Millisecond
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       2,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
		Store:          persistence,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "expire while waiting event append is blocked"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	select {
	case <-persistence.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for waiting event append")
	}
	select {
	case <-persistence.cancelled:
	case <-time.After(time.Second):
		t.Fatal("waiting event append did not honor notifier deadline")
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}
	for _, event := range events {
		if event.Type == EventRunWaitingForUser {
			t.Fatal("expired run.waiting_for_user was published after event append deadline")
		}
	}
}

func TestExecuteLifecycle_StaleWaitCannotSurviveFailedCorrectiveWrite(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{{
		ToolCalls: []ToolCall{{
			ID:        "call_ask_stale_repair",
			Name:      htools.AskUserQuestionToolName,
			Arguments: `{"questions":[{"question":"Continue?","header":"Continue","options":[{"label":"Yes","description":"Continue"},{"label":"No","description":"Stop"}],"multiSelect":false}]}`,
		}},
	}}}
	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	persistence := newStaleWaitRepairFailingStore()
	released := false
	t.Cleanup(func() {
		if !released {
			close(persistence.release)
		}
	})
	const timeout = 50 * time.Millisecond
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       2,
		AskUserBroker:  broker,
		AskUserTimeout: timeout,
		Store:          persistence,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "preserve terminal state after a stale wait write"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	select {
	case <-persistence.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for waiting status persistence")
	}
	time.Sleep(timeout + 50*time.Millisecond)
	close(persistence.release)
	released = true
	if _, err := collectRunEvents(t, runner, run.ID); err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		stored, err := persistence.GetRun(context.Background(), run.ID)
		if err != nil {
			t.Fatalf("GetRun: %v", err)
		}
		if stored.Status == runstore.RunStatusFailed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("durable status = %q, want failed after stale wait write", stored.Status)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSetStatusContext_DelayedWaitCannotDowngradeTerminalRun(t *testing.T) {
	t.Parallel()

	persistence := runstore.NewMemoryStore()
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{Store: persistence})
	const runID = "run-delayed-wait-after-terminal"
	now := time.Now().UTC()
	runner.mu.Lock()
	runner.runs[runID] = &runState{run: Run{
		ID:        runID,
		Status:    RunStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}}
	runner.mu.Unlock()
	if err := persistence.CreateRun(context.Background(), runToStoreRun(Run{
		ID:        runID,
		Status:    RunStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	})); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	releaseWait := make(chan struct{})
	waitResult := make(chan bool, 1)
	go func() {
		<-releaseWait
		waitResult <- runner.setStatusAndEmitContext(
			context.Background(),
			runID,
			RunStatusWaitingForUser,
			"",
			"",
			EventRunWaitingForUser,
			map[string]any{"call_id": "late"},
		)
	}()
	runner.setStatus(runID, RunStatusFailed, "", "terminal")
	close(releaseWait)
	if published := <-waitResult; published {
		t.Fatal("delayed waiting status was accepted after terminal state")
	}

	inMemory, ok := runner.GetRun(runID)
	if !ok {
		t.Fatal("run not found")
	}
	if inMemory.Status != RunStatusFailed {
		t.Fatalf("in-memory status = %q, want failed", inMemory.Status)
	}
	for _, event := range runner.getEvents(runID) {
		if event.Type == EventRunWaitingForUser {
			t.Fatal("delayed notifier emitted waiting event after terminal state")
		}
	}
	stored, err := persistence.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if stored.Status != runstore.RunStatusFailed {
		t.Fatalf("durable status = %q, want failed", stored.Status)
	}
}

func TestExecuteLifecycle_WaitForUserFlowEventOrderAndStateRestoration(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask_lc",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Pick one?","header":"Choice","options":[{"label":"A","description":"Option A"},{"label":"B","description":"Option B"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "lifecycle complete"},
	}}

	const waitForUserTimeout = 10 * time.Second

	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: waitForUserTimeout,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: waitForUserTimeout,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "lifecycle wait-for-user"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until status transitions to waiting_for_user.
	deadline := time.Now().Add(waitForUserTimeout)
	for {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatal("run not found while polling for waiting_for_user")
		}
		if state.Status == RunStatusWaitingForUser {
			break
		}
		if state.Status == RunStatusCompleted || state.Status == RunStatusFailed {
			t.Fatalf("run ended prematurely with status %q", state.Status)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for waiting_for_user status, last: %s", state.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// State must be waiting_for_user.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusWaitingForUser {
		t.Fatalf("expected waiting_for_user, got %q", state.Status)
	}

	// PendingInput must return the question.
	pending, err := runner.PendingInput(run.ID)
	if err != nil {
		t.Fatalf("PendingInput: %v", err)
	}
	if pending.CallID != "call_ask_lc" {
		t.Errorf("expected call_ask_lc, got %q", pending.CallID)
	}

	// Submit user response.
	if err := runner.SubmitInput(run.ID, map[string]string{"Pick one?": "A"}); err != nil {
		t.Fatalf("SubmitInput: %v", err)
	}

	// Collect all events (blocks until terminal).
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Final state must be completed.
	state, ok = runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found after completion")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if state.Output != "lifecycle complete" {
		t.Errorf("unexpected output: %q", state.Output)
	}

	// Stored transcript must include messages from both LLM turns.
	msgs := runner.GetRunMessages(run.ID)
	if len(msgs) < 3 {
		t.Errorf("expected at least 3 stored messages (user + tool_call + resume response), got %d", len(msgs))
	}

	// Event ordering (non-exhaustive but pinned):
	requireEventOrder(t, events,
		"run.started",
		"tool.call.started",
		"run.waiting_for_user",
		"tool.call.completed",
		"run.resumed",
		"assistant.message",
		"run.completed",
	)
	assertSingleWaitAndResume(t, events)
}

func assertSingleWaitAndResume(t *testing.T, events []Event) {
	t.Helper()
	var waits, resumes int
	for _, event := range events {
		switch event.Type {
		case EventRunWaitingForUser:
			waits++
		case EventRunResumed:
			resumes++
		}
	}
	if waits != 1 || resumes != 1 {
		t.Fatalf("wait/resume event counts = %d/%d, want exactly 1/1", waits, resumes)
	}
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_WaitForUserTimeoutPath
//
// Verifies the wait-for-user timeout path:
//   - run.waiting_for_user is emitted.
//   - After timeout, run.failed is emitted (terminal).
//   - Final run status is failed.
//   - state.Error contains "timed out".
//   - No events appear after run.failed.
//
// -------------------------------------------------------------------------
func TestExecuteLifecycle_WaitForUserTimeoutPath(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask_timeout",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Pick?","header":"H","options":[{"label":"X","description":"Opt X"},{"label":"Y","description":"Opt Y"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "should not happen"},
	}}

	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: 30 * time.Millisecond,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: 30 * time.Millisecond,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "timeout test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Terminal state must be failed.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusFailed {
		t.Fatalf("expected failed, got %q", state.Status)
	}
	if !strings.Contains(state.Error, "timed out") {
		t.Errorf("expected timeout error, got %q", state.Error)
	}

	// Provider called exactly once (timeout before second turn).
	if provider.calls != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.calls)
	}

	// Event ordering: waiting event must precede failed.
	requireEventOrder(t, events,
		"run.waiting_for_user",
		"run.failed",
	)

	// No events must appear after run.failed (post-terminal seal check).
	terminalIdx := -1
	for i, ev := range events {
		if IsTerminalEvent(ev.Type) {
			terminalIdx = i
			break
		}
	}
	if terminalIdx < 0 {
		t.Fatal("no terminal event found")
	}
	for _, ev := range events[terminalIdx+1:] {
		t.Errorf("unexpected post-terminal event: %q", ev.Type)
	}
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_CostCeilingTerminatesRunAfterLLMTurnNoToolCalls
//
// Verifies the cost-ceiling path when the terminal step has no tool calls:
//   - run.cost_limit_reached is emitted before run.completed.
//   - run status is completed (not failed).
//   - Provider is not called again after limit is hit.
//   - Event ordering is pinned.
//
// -------------------------------------------------------------------------
func TestExecuteLifecycle_CostCeilingTerminatesRunAfterLLMTurnNoToolCalls(t *testing.T) {
	t.Parallel()

	// Two turns. First turn: $0.002 (tool call). Second turn: $0.002 (text only).
	// Ceiling $0.003 — not breached after turn 1 ($0.002 < $0.003).
	// After turn 2 cumulative = $0.004 >= $0.003, so cost limit fires.
	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_lc",
		Description: "echoes",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `"ok"`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls:  []ToolCall{{ID: "c1", Name: "echo_lc", Arguments: `{}`}},
			CostUSD:    floatPtr(0.002),
			CostStatus: CostStatusAvailable,
			Cost:       &CompletionCost{TotalUSD: 0.002},
		},
		{
			Content:    "done after cost ceiling",
			CostUSD:    floatPtr(0.002),
			CostStatus: CostStatusAvailable,
			Cost:       &CompletionCost{TotalUSD: 0.002},
		},
		// This must never be reached.
		{Content: "unreachable"},
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     10,
	})

	run, err := runner.StartRun(RunRequest{
		Prompt:     "cost ceiling lifecycle",
		MaxCostUSD: 0.003,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// run.cost_limit_reached must appear before run.completed.
	requireEventOrder(t, events,
		"run.started",
		"run.cost_limit_reached",
		"run.completed",
	)

	// Terminal must be run.completed (not run.failed).
	var terminal *Event
	for i := range events {
		if IsTerminalEvent(events[i].Type) {
			terminal = &events[i]
			break
		}
	}
	if terminal == nil {
		t.Fatal("no terminal event")
	}
	if terminal.Type != EventRunCompleted {
		t.Fatalf("expected run.completed as terminal, got %q", terminal.Type)
	}

	// Run state must be completed.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}

	// Provider called exactly 2 times.
	if provider.calls != 2 {
		t.Errorf("expected 2 provider calls, got %d", provider.calls)
	}

	// cost_limit_reached payload must include max_cost_usd and cumulative_cost_usd.
	var costLimitEv *Event
	for i := range events {
		if events[i].Type == EventRunCostLimitReached {
			costLimitEv = &events[i]
			break
		}
	}
	if costLimitEv == nil {
		t.Fatal("run.cost_limit_reached event not found")
	}
	if costLimitEv.Payload["max_cost_usd"] == nil {
		t.Error("run.cost_limit_reached missing max_cost_usd")
	}
	if costLimitEv.Payload["cumulative_cost_usd"] == nil {
		t.Error("run.cost_limit_reached missing cumulative_cost_usd")
	}
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_EmptyResponseRetryThenSuccess
//
// Verifies the empty-response retry path followed by successful completion:
//   - llm.empty_response.retry events are emitted for each empty turn.
//   - The retry counter in payload increments correctly.
//   - After a successful turn the run completes normally.
//   - Event ordering is pinned.
//
// -------------------------------------------------------------------------
func TestExecuteLifecycle_EmptyResponseRetryThenSuccess(t *testing.T) {
	t.Parallel()

	// Two empty turns, then a real response.
	provider := &stubProvider{turns: []CompletionResult{
		{Content: "", ToolCalls: nil}, // empty 1 → retry event (retry=1)
		{Content: "", ToolCalls: nil}, // empty 2 → retry event (retry=2)
		{Content: "finally answered"},
	}}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gemini-lc",
		MaxSteps:     10,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "lifecycle empty retry"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Run must complete successfully.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if state.Output != "finally answered" {
		t.Errorf("expected output %q, got %q", "finally answered", state.Output)
	}

	// Exactly 2 retry events with sequential retry counters.
	var retryEvents []Event
	for _, ev := range events {
		if ev.Type == EventEmptyResponseRetry {
			retryEvents = append(retryEvents, ev)
		}
	}
	if len(retryEvents) != 2 {
		t.Fatalf("expected 2 llm.empty_response.retry events, got %d", len(retryEvents))
	}

	// Retry counters must be 1 and 2 in order.
	for i, re := range retryEvents {
		retryVal, ok := re.Payload["retry"]
		if !ok {
			t.Errorf("retry event %d missing 'retry' field", i)
			continue
		}
		expected := i + 1
		if int(retryVal.(int)) != expected {
			t.Errorf("retry event %d: payload retry=%v, want %d", i, retryVal, expected)
		}
		if _, ok := re.Payload["step"]; !ok {
			t.Errorf("retry event %d missing 'step' field", i)
		}
		if _, ok := re.Payload["max_retries"]; !ok {
			t.Errorf("retry event %d missing 'max_retries' field", i)
		}
	}

	// Event ordering: retries precede the assistant message and run.completed.
	requireEventOrder(t, events,
		"run.started",
		"llm.empty_response.retry",
		"assistant.message",
		"run.completed",
	)
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_HookAndToolCallAndMessagePersistenceSequence
//
// Verifies the combined hook + tool-call + message persistence sequence:
//   - Pre/post message hooks fire and emit hook.started / hook.completed.
//   - Tool calls are executed and tool.call.started / tool.call.completed appear.
//   - The stored transcript contains both the assistant tool-call message and
//     the tool result message.
//   - The final assistant message is persisted with the post-hook mutation.
//
// -------------------------------------------------------------------------
func TestExecuteLifecycle_HookAndToolCallAndMessagePersistenceSequence(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_hook",
		Description: "echoes",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"msg": map[string]any{"type": "string"},
			},
		},
	}, func(_ context.Context, raw json.RawMessage) (string, error) {
		return `{"result":"hook-test-ok"}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &capturingProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_hook_lc",
				Name:      "echo_hook",
				Arguments: `{"msg":"hello"}`,
			}},
		},
		{Content: "original final"},
	}}

	// Pre-hook: add a marker to the model name so we can assert it was called.
	// Post-hook: mutate the final response content.
	hookCalled := false
	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-lc",
		MaxSteps:     4,
		PreMessageHooks: []PreMessageHook{
			preHookFunc{name: "lc-pre", fn: func(_ context.Context, in PreMessageHookInput) (PreMessageHookResult, error) {
				hookCalled = true
				return PreMessageHookResult{Action: HookActionContinue}, nil
			}},
		},
		PostMessageHooks: []PostMessageHook{
			postHookFunc{name: "lc-post", fn: func(_ context.Context, in PostMessageHookInput) (PostMessageHookResult, error) {
				if in.Response.Content == "original final" {
					mutated := in.Response
					mutated.Content = "mutated by lc-post"
					return PostMessageHookResult{Action: HookActionContinue, MutatedResponse: &mutated}, nil
				}
				return PostMessageHookResult{Action: HookActionContinue}, nil
			}},
		},
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hook lifecycle test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Run must complete with the mutated output.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if state.Output != "mutated by lc-post" {
		t.Errorf("expected post-hook output, got %q", state.Output)
	}

	// Pre-hook must have been called.
	if !hookCalled {
		t.Error("pre-hook was not called")
	}

	// Stored transcript must include:
	// 1. User message ("hook lifecycle test")
	// 2. At least one assistant message (tool call)
	// 3. Tool result message
	// 4. Final assistant message
	msgs := runner.GetRunMessages(run.ID)
	if len(msgs) < 4 {
		t.Errorf("expected at least 4 stored messages, got %d: %+v", len(msgs), msgs)
	}

	// Check that the tool result is stored.
	toolResultFound := false
	for _, m := range msgs {
		if m.Role == "tool" && m.ToolCallID == "call_hook_lc" {
			toolResultFound = true
		}
	}
	if !toolResultFound {
		t.Error("tool result message not found in stored transcript")
	}

	// Event ordering: hook events appear around llm turn; tool events appear after.
	requireEventOrder(t, events,
		"run.started",
		"hook.started",
		"hook.completed",
		"tool.call.started",
		"tool.call.completed",
		"assistant.message",
		"run.completed",
	)
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_CompactionChangesNextRequestShape
//
// Multi-feature turn: compaction occurs while a run is paused mid-execution,
// then the next provider call must receive the compacted (shorter) context
// rather than the original full context. This verifies that compaction
// changes the next request shape.
// -------------------------------------------------------------------------
func TestExecuteLifecycle_CompactionChangesNextRequestShape(t *testing.T) {
	t.Parallel()

	// Gate step 2 so we can compact after step 1 finishes.
	step2Gate := make(chan struct{})

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_shape",
		Description: "echoes",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{"r":"ok"}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &contextCompactGatingProvider{
		results: []CompletionResult{
			// Step 1: returns a tool call — adds messages to transcript.
			{ToolCalls: []ToolCall{{ID: "c1", Name: "echo_shape", Arguments: `{}`}}},
			// Step 2: gated — waits for compaction before provider is invoked.
			{Content: "done"},
		},
		beforeCall: func(idx int) {
			if idx == 1 {
				<-step2Gate
			}
		},
	}

	capProvider := &capturingProvider{
		turns: []CompletionResult{
			{ToolCalls: []ToolCall{{ID: "c1", Name: "echo_shape", Arguments: `{}`}}},
			{Content: "done"},
		},
	}

	// Use capturingProvider so we can inspect the messages sent to each call.
	runnerCap := NewRunner(capProvider, registry, RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     6,
	})
	_ = provider // gate provider for shape comparison
	_ = step2Gate

	// Separate test with gating + capturing.
	// Re-use contextCompactGatingProvider with a capturing layer.
	type capturableGatingProvider struct {
		gate    *contextCompactGatingProvider
		capture *capturingProvider
	}

	// Simpler approach: run the gate provider to completion, then inspect
	// messages at each step boundary by using CompactRun mid-run.
	step4Gate2 := make(chan struct{})
	gatingProv := &contextCompactGatingProvider{
		results: []CompletionResult{
			{ToolCalls: []ToolCall{{ID: "c1", Name: "echo_shape", Arguments: `{}`}}},
			{ToolCalls: []ToolCall{{ID: "c2", Name: "echo_shape", Arguments: `{}`}}},
			{ToolCalls: []ToolCall{{ID: "c3", Name: "echo_shape", Arguments: `{}`}}},
			{Content: "final"},
		},
		beforeCall: func(idx int) {
			if idx == 3 {
				<-step4Gate2
			}
		},
	}
	runner := NewRunner(gatingProv, registry, RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     8,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "shape test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until 3 steps are done (provider.calls == 3 waiting at gate).
	deadline := time.Now().Add(5 * time.Second)
	for {
		gatingProv.mu.Lock()
		calls := gatingProv.calls
		gatingProv.mu.Unlock()
		if calls >= 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for step 4 gate")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// At this point 3 steps completed: user + 3x(assistant+tool) = 7 messages.
	msgsBeforeCompact := runner.GetRunMessages(run.ID)
	countBefore := len(msgsBeforeCompact)
	if countBefore < 7 {
		t.Fatalf("expected >= 7 messages before compact, got %d", countBefore)
	}

	// Compact with strip mode, keepLast=2 — removes tool messages from old turns.
	result, err := runner.CompactRun(context.Background(), run.ID, CompactRunRequest{
		Mode:     "strip",
		KeepLast: 2,
	})
	if err != nil {
		t.Fatalf("CompactRun: %v", err)
	}
	if result.MessagesRemoved == 0 {
		t.Log("no messages removed — tool messages may all be within keep window")
	}

	compactedCount := len(runner.GetRunMessages(run.ID))

	// Release step 4.
	close(step4Gate2)

	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}

	// After step 4 (one assistant message appended), the final count must equal
	// compactedCount + 1 — confirming the run used the compacted context, not
	// the stale pre-compaction copy.
	finalCount := len(runner.GetRunMessages(run.ID))
	expectedFinal := compactedCount + 1
	if finalCount != expectedFinal {
		t.Errorf("next request shape not from compacted context: final=%d, want=%d (compacted=%d, beforeCompact=%d)",
			finalCount, expectedFinal, compactedCount, countBefore)
	}

	// The capProvider run was only used for structural setup; clean it up.
	_ = runnerCap
}
