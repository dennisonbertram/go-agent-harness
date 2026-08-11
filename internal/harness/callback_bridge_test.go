package harness

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	istore "go-agent-harness/internal/store"
)

func TestRunnerNewCallbackManagerEmitsScheduledEvent(t *testing.T) {
	t.Parallel()

	provider := &blockingCallbackProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     1,
	})

	run, err := runner.StartRun(RunRequest{
		Prompt:         "keep callback stream open",
		ConversationID: "conv-callback",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	select {
	case <-provider.started:
	case <-time.After(2 * time.Second):
		t.Fatal("provider did not start")
	}

	history, stream, cancelSub, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancelSub()

	manager := runner.NewCallbackManager(callbackStarterStub{})
	defer manager.Shutdown()

	info, err := manager.Set(htools.SetRequest{
		ConversationID: "conv-callback",
		Delay:          htools.MinCallbackDelay,
		Prompt:         "check status later",
	})
	if err != nil {
		t.Fatalf("Set callback: %v", err)
	}

	event := waitForCallbackEvent(t, history, stream, EventCallbackScheduled)
	if event.RunID != run.ID {
		t.Fatalf("callback event run id = %q, want %q", event.RunID, run.ID)
	}
	if event.Payload["callback_id"] != info.ID {
		t.Fatalf("callback_id = %v, want %s", event.Payload["callback_id"], info.ID)
	}
	if event.Payload["conversation_id"] != "conv-callback" {
		t.Fatalf("conversation_id = %v, want conv-callback", event.Payload["conversation_id"])
	}

	close(provider.release)
}

func TestCallbackEventPayloadRejectsUntrustedDurableError(t *testing.T) {
	payload := callbackEventPayload(htools.CallbackInfo{
		ID: "failed", State: htools.CallbackStateFailed,
		LastError: "Authorization: Bearer sk-secret password=hunter2",
	})
	got, _ := payload["last_error"].(string)
	if got != "callback admission failed" || strings.Contains(got, "secret") || strings.Contains(got, "hunter2") {
		t.Fatalf("unsafe callback event error = %q", got)
	}
}

func TestCallbackLifecycleEventsSurviveSchedulingRunCompletion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		event   EventType
		starter *callbackLifecycleStarter
	}{
		{
			name:  "retry wait",
			event: EventCallbackRetryWait,
			starter: &callbackLifecycleStarter{errs: []error{
				errors.New("provider included Authorization: Bearer sk-secret"),
			}},
		},
		{
			name:  "failed",
			event: EventCallbackFailed,
			starter: &callbackLifecycleStarter{errs: []error{
				&htools.CallbackStartError{Err: errors.New("password=hunter2"), Retry: false, Summary: "invalid callback scope"},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := newCompletedCallbackConversation(t, "conv-"+tc.name)
			history, stream, cancel, err := runner.SubscribeConversation("conv-" + tc.name)
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()

			callbackStore := newCallbackBridgeStore(t)
			info := seedOverdueCallback(t, callbackStore, "conv-"+tc.name, "after completion")
			manager := runner.NewCallbackManager(tc.starter, htools.WithCallbackStore(callbackStore))
			defer manager.Shutdown()
			if err := manager.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}

			event := waitForCallbackEvent(t, history, stream, tc.event)
			if event.Payload["callback_id"] != info.ID || event.Payload["run_id"] != info.RunID {
				t.Fatalf("callback lifecycle linkage = %#v", event.Payload)
			}
		})
	}
}

func TestCallbackStartedEventAndTranscriptSurviveFastAdmission(t *testing.T) {
	runStore := istore.NewMemoryStore()
	runner := NewRunner(callbackLifecycleProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     1,
		Store:        runStore,
	})
	origin, err := runner.StartRun(RunRequest{Prompt: "origin", ConversationID: "conv-started"})
	if err != nil {
		t.Fatal(err)
	}
	waitForCallbackRunTerminal(t, runner, origin.ID)
	history, stream, cancel, err := runner.SubscribeConversation("conv-started")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	callbackStore := newCallbackBridgeStore(t)
	info := seedOverdueCallback(t, callbackStore, "conv-started", "say hello")
	starter := &callbackLifecycleStarter{runner: runner}
	manager := runner.NewCallbackManager(starter, htools.WithCallbackStore(callbackStore))
	defer manager.Shutdown()
	if err := manager.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	event := waitForCallbackEvent(t, history, stream, EventCallbackStarted)
	if event.Payload["run_id"] != info.RunID || event.RunID != info.RunID {
		t.Fatalf("started linkage event=%#v callback=%#v", event, info)
	}
	waitForCallbackRunTerminal(t, runner, info.RunID)
	messages, ok := runner.ConversationMessages("conv-started")
	if !ok {
		t.Fatal("callback conversation transcript missing")
	}
	var sawPrompt, sawAnswer bool
	for _, message := range messages {
		sawPrompt = sawPrompt || message.Role == "user" && message.Content == "say hello"
		sawAnswer = sawAnswer || message.Role == "assistant" && message.Content == "hello"
	}
	if !sawPrompt || !sawAnswer {
		t.Fatalf("callback transcript prompt=%v answer=%v messages=%#v", sawPrompt, sawAnswer, messages)
	}
}

func TestCallbackRecoveryRepublishesDurableLifecycleAfterRunnerRestart(t *testing.T) {
	conversationStore := newTestConversationStore(t)
	first := NewRunner(callbackLifecycleProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model", MaxSteps: 1, ConversationStore: conversationStore,
	})
	origin, err := first.StartRun(RunRequest{Prompt: "origin", ConversationID: "conv-restart"})
	if err != nil {
		t.Fatal(err)
	}
	waitForCallbackRunTerminal(t, first, origin.ID)
	first.Shutdown(context.Background())

	callbackStore := newCallbackBridgeStore(t)
	now := time.Now().UTC()
	for _, info := range []htools.CallbackInfo{
		{
			ID: "retry", ConversationID: "conv-restart", Prompt: "retry later", Delay: "5s",
			State: htools.CallbackStateRetryWait, FiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute),
			RunID: "run_callback_retry", Attempt: 1, NextAttemptAt: now.Add(time.Hour), LastError: "callback admission unavailable",
		},
		{
			ID: "failed", ConversationID: "conv-restart", Prompt: "failed", Delay: "5s",
			State: htools.CallbackStateFailed, FiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute),
			RunID: "run_callback_failed", Attempt: 3, LastError: "callback admission failed",
		},
		{
			ID: "started", ConversationID: "conv-restart", Prompt: "started", Delay: "5s",
			State: htools.CallbackStateStarted, FiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute),
			RunID: "run_callback_started", Attempt: 1,
		},
	} {
		if err := callbackStore.Create(context.Background(), info); err != nil {
			t.Fatal(err)
		}
	}

	// A replacement Runner begins with no process-local event journal. Recover
	// must rebuild callback lifecycle visibility from the durable callback rows.
	second := NewRunner(callbackLifecycleProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model", MaxSteps: 1, ConversationStore: conversationStore,
	})
	manager := second.NewCallbackManager(&callbackLifecycleStarter{}, htools.WithCallbackStore(callbackStore))
	defer manager.Shutdown()
	if err := manager.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	history, _, cancel, err := second.SubscribeConversation("conv-restart")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	want := map[EventType]string{
		EventCallbackRetryWait: "run_callback_retry",
		EventCallbackFailed:    "run_callback_failed",
		EventCallbackStarted:   "run_callback_started",
	}
	for _, event := range history {
		if runID, ok := want[event.Type]; ok && event.Payload["run_id"] == runID {
			delete(want, event.Type)
		}
	}
	if len(want) != 0 {
		t.Fatalf("recovered callback lifecycle missing %#v from history %#v", want, history)
	}
}

type callbackLifecycleStarter struct {
	mu     sync.Mutex
	runner *Runner
	errs   []error
	calls  int
}

func (s *callbackLifecycleStarter) StartRun(string, string, string, string) error { return nil }

func (s *callbackLifecycleStarter) StartCallback(ctx context.Context, info htools.CallbackInfo) (string, error) {
	s.mu.Lock()
	call := s.calls
	s.calls++
	var err error
	if call < len(s.errs) {
		err = s.errs[call]
	}
	runner := s.runner
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	if runner == nil {
		return info.RunID, nil
	}
	run, err := runner.EnsureRunWithIDContext(ctx, RunRequest{
		Prompt: info.Prompt, ConversationID: info.ConversationID,
		TenantID: info.TenantID, AgentID: info.AgentID,
	}, info.RunID)
	return run.ID, err
}

type callbackLifecycleProvider struct{}

func (callbackLifecycleProvider) Complete(context.Context, CompletionRequest) (CompletionResult, error) {
	return CompletionResult{Content: "hello"}, nil
}

func newCompletedCallbackConversation(t *testing.T, conversationID string) *Runner {
	t.Helper()
	runner := NewRunner(callbackLifecycleProvider{}, NewRegistry(), RunnerConfig{DefaultModel: "test-model", MaxSteps: 1})
	run, err := runner.StartRun(RunRequest{Prompt: "origin", ConversationID: conversationID})
	if err != nil {
		t.Fatal(err)
	}
	waitForCallbackRunTerminal(t, runner, run.ID)
	return runner
}

func newCallbackBridgeStore(t *testing.T) *htools.SQLiteCallbackStore {
	t.Helper()
	store, err := htools.NewSQLiteCallbackStore(filepath.Join(t.TempDir(), "callbacks.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedOverdueCallback(t *testing.T, store *htools.SQLiteCallbackStore, conversationID, prompt string) htools.CallbackInfo {
	t.Helper()
	now := time.Now().UTC()
	info := htools.CallbackInfo{
		ID: "callback-" + conversationID, ConversationID: conversationID,
		Prompt: prompt, Delay: "5s", State: htools.CallbackStatePending,
		FiresAt: now.Add(-time.Second), CreatedAt: now,
	}
	info.RunID = "run_callback_" + info.ID
	if err := store.Create(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	return info
}

func waitForCallbackRunTerminal(t *testing.T, runner *Runner, runID string) Run {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if run, ok := runner.GetRun(runID); ok && (run.Status == RunStatusCompleted || run.Status == RunStatusFailed || run.Status == RunStatusCancelled) {
			return run
		}
		time.Sleep(time.Millisecond)
	}
	run, _ := runner.GetRun(runID)
	t.Fatalf("run %s did not become terminal: %#v", runID, run)
	return Run{}
}

type blockingCallbackProvider struct {
	started chan struct{}
	release chan struct{}
}

func (p *blockingCallbackProvider) Complete(ctx context.Context, _ CompletionRequest) (CompletionResult, error) {
	select {
	case <-p.started:
	default:
		close(p.started)
	}
	select {
	case <-p.release:
		return CompletionResult{Content: "done"}, nil
	case <-ctx.Done():
		return CompletionResult{}, ctx.Err()
	}
}

type callbackStarterStub struct{}

func (callbackStarterStub) StartRun(string, string, string, string) error { return nil }

func waitForCallbackEvent(t *testing.T, history []Event, stream <-chan Event, want EventType) Event {
	t.Helper()
	for _, event := range history {
		if event.Type == want {
			return event
		}
	}
	timeout := time.After(2 * time.Second)
	for {
		select {
		case event, ok := <-stream:
			if !ok {
				t.Fatalf("stream closed before %s", want)
			}
			if event.Type == want {
				return event
			}
		case <-timeout:
			t.Fatalf("timed out waiting for %s", want)
		}
	}
}
