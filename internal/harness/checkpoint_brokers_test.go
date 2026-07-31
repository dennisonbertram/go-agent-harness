package harness

// Tests for the checkpoint-backed approval and ask-user brokers.
//
// These are the components that hold a run suspended while a human decides, so
// the interesting behaviour is all in the paths a happy-path test never takes:
// timeouts expiring the checkpoint, approve/deny with no pending record,
// answers that fail validation, and corrupt stored question payloads.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/checkpoints"
	htools "go-agent-harness/internal/harness/tools"
)

func receiveWithin[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for channel result")
		var zero T
		return zero
	}
}

func newTestCheckpointService() *checkpoints.Service {
	return checkpoints.NewService(nil, time.Now)
}

func sampleQuestions() []htools.AskUserQuestion {
	return []htools.AskUserQuestion{{
		Question: "Pick one",
		Header:   "Pick",
		Options: []htools.AskUserQuestionOption{
			{Label: "a", Description: "option a"},
			{Label: "b", Description: "option b"},
		},
	}}
}

// waitForPending polls until a checkpoint for runID is visible, so tests do not
// race the Ask goroutine that creates it.
func waitForPending(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for a pending checkpoint")
}

// --- approval broker --------------------------------------------------

func TestCheckpointApprovalBroker_ApproveWithOption(t *testing.T) {
	broker := NewCheckpointApprovalBroker(newTestCheckpointService())

	type result struct {
		approved bool
		option   string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		approved, option, err := broker.Ask(context.Background(), ApprovalRequest{
			RunID:   "run-1",
			CallID:  "call-1",
			Tool:    "plan_exit",
			Args:    "the plan",
			Timeout: 5 * time.Second,
			Options: []PlanApproachOption{{ID: "a", Label: "Approach A", Description: "first"}},
		})
		done <- result{approved, option, err}
	}()

	waitForPending(t, func() bool {
		_, ok := broker.Pending("run-1")
		return ok
	})

	// The presented options must survive the round trip through storage.
	pending, ok := broker.Pending("run-1")
	if !ok {
		t.Fatal("expected a pending approval")
	}
	if pending.Tool != "plan_exit" || pending.Args != "the plan" {
		t.Errorf("pending approval lost its request details: %+v", pending)
	}
	if len(pending.Options) != 1 || pending.Options[0].Label != "Approach A" {
		t.Errorf("pending options = %+v, want the presented option", pending.Options)
	}

	if err := broker.ApproveWithOption("run-1", "a"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	got := receiveWithin(t, done)
	if got.err != nil {
		t.Fatalf("Ask returned an error: %v", got.err)
	}
	if !got.approved {
		t.Error("expected the request to be approved")
	}
	if got.option != "a" {
		t.Errorf("selected option = %q, want %q", got.option, "a")
	}
}

func TestCheckpointApprovalBroker_Deny(t *testing.T) {
	broker := NewCheckpointApprovalBroker(newTestCheckpointService())

	done := make(chan bool, 1)
	go func() {
		approved, _, _ := broker.Ask(context.Background(), ApprovalRequest{
			RunID: "run-2", CallID: "c", Tool: "bash", Timeout: 5 * time.Second,
		})
		done <- approved
	}()

	waitForPending(t, func() bool { _, ok := broker.Pending("run-2"); return ok })
	if err := broker.Deny("run-2"); err != nil {
		t.Fatalf("deny: %v", err)
	}
	if receiveWithin(t, done) {
		t.Error("a denied request must not report approval")
	}
}

func TestCheckpointApprovalBroker_TimeoutExpiresTheCheckpoint(t *testing.T) {
	broker := NewCheckpointApprovalBroker(newTestCheckpointService())

	_, _, err := broker.Ask(context.Background(), ApprovalRequest{
		RunID: "run-3", CallID: "c", Tool: "bash", Timeout: 30 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	var timeoutErr *ApprovalTimeoutError
	if !asApprovalTimeout(err, &timeoutErr) {
		t.Fatalf("error %v is not an ApprovalTimeoutError", err)
	}
	if timeoutErr.RunID != "run-3" {
		t.Errorf("timeout error run id = %q, want run-3", timeoutErr.RunID)
	}
	// The checkpoint must be expired, not left pending forever.
	if _, ok := broker.Pending("run-3"); ok {
		t.Error("a timed-out approval must not remain pending")
	}
}

func TestCheckpointApprovalBroker_NoPendingRecord(t *testing.T) {
	broker := NewCheckpointApprovalBroker(newTestCheckpointService())

	if _, ok := broker.Pending("nobody"); ok {
		t.Error("there should be no pending approval for an unknown run")
	}
	if err := broker.Approve("nobody"); err != ErrNoPendingApproval {
		t.Errorf("Approve on an unknown run = %v, want ErrNoPendingApproval", err)
	}
	if err := broker.ApproveWithOption("nobody", "a"); err != ErrNoPendingApproval {
		t.Errorf("ApproveWithOption on an unknown run = %v, want ErrNoPendingApproval", err)
	}
	if err := broker.Deny("nobody"); err != ErrNoPendingApproval {
		t.Errorf("Deny on an unknown run = %v, want ErrNoPendingApproval", err)
	}
}

type gatedPendingLookupStore struct {
	checkpoints.Store
	captured chan struct{}
	release  chan struct{}
	once     sync.Once
}

func (s *gatedPendingLookupStore) PendingByRun(ctx context.Context, runID string) (*checkpoints.Record, error) {
	record, err := s.Store.PendingByRun(ctx, runID)
	s.once.Do(func() { close(s.captured) })
	<-s.release
	return record, err
}

func TestCheckpointApprovalBroker_ResolutionLostToExpiryIsNoPending(t *testing.T) {
	tests := []struct {
		name    string
		resolve func(*checkpointApprovalBroker, string) error
	}{
		{name: "approve", resolve: func(b *checkpointApprovalBroker, runID string) error {
			return b.Approve(runID)
		}},
		{name: "approve with option", resolve: func(b *checkpointApprovalBroker, runID string) error {
			return b.ApproveWithOption(runID, "a")
		}},
		{name: "deny", resolve: func(b *checkpointApprovalBroker, runID string) error {
			return b.Deny(runID)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &gatedPendingLookupStore{
				Store:    checkpoints.NewMemoryStore(),
				captured: make(chan struct{}),
				release:  make(chan struct{}),
			}
			service := checkpoints.NewService(store, time.Now)
			record, err := service.Create(context.Background(), checkpoints.CreateRequest{
				Kind:       checkpoints.KindApproval,
				RunID:      "run-expiry-race",
				DeadlineAt: time.Now().Add(time.Minute),
			})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			broker := &checkpointApprovalBroker{service: service}
			result := make(chan error, 1)
			go func() { result <- tt.resolve(broker, record.RunID) }()

			receiveWithin(t, store.captured)
			if expired, err := service.ExpirePending(context.Background(), record.ID); err != nil || !expired {
				t.Fatalf("ExpirePending = (%v, %v), want (true, nil)", expired, err)
			}
			close(store.release)
			if err := receiveWithin(t, result); !errors.Is(err, ErrNoPendingApproval) {
				t.Fatalf("resolution error = %v, want ErrNoPendingApproval", err)
			}
		})
	}
}

// --- ask-user broker --------------------------------------------------

func TestCheckpointAskUserQuestionBroker_SubmitAnswers(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	broker := NewCheckpointAskUserQuestionBroker(newTestCheckpointService(), func() time.Time { return fixed })

	type result struct {
		answers  map[string]string
		answered time.Time
		err      error
	}
	done := make(chan result, 1)
	go func() {
		a, ts, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
			RunID: "run-4", CallID: "c", Questions: sampleQuestions(), Timeout: 5 * time.Second,
		})
		done <- result{a, ts, err}
	}()

	waitForPending(t, func() bool { _, ok := broker.Pending("run-4"); return ok })

	pending, ok := broker.Pending("run-4")
	if !ok {
		t.Fatal("expected a pending question")
	}
	if pending.Tool != htools.AskUserQuestionToolName {
		t.Errorf("pending tool = %q", pending.Tool)
	}
	if len(pending.Questions) != 1 || pending.Questions[0].Question != "Pick one" {
		t.Errorf("pending questions did not survive storage: %+v", pending.Questions)
	}

	if err := broker.Submit("run-4", map[string]string{"Pick one": "a"}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	got := receiveWithin(t, done)
	if got.err != nil {
		t.Fatalf("Ask: %v", got.err)
	}
	if got.answers["Pick one"] != "a" {
		t.Errorf("answers = %+v, want the submitted answer", got.answers)
	}
	if !got.answered.Equal(fixed) {
		t.Errorf("answered-at = %v, want the injected clock value %v", got.answered, fixed)
	}
}

func TestCheckpointAskUserQuestionBroker_RejectsInvalidInput(t *testing.T) {
	broker := NewCheckpointAskUserQuestionBroker(newTestCheckpointService(), nil)

	// Questions that fail validation are rejected before any checkpoint is
	// created, so nothing is left pending.
	_, _, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
		RunID: "run-5", Questions: nil, Timeout: time.Second,
	})
	if err == nil {
		t.Error("invalid questions must be rejected")
	}
	if _, ok := broker.Pending("run-5"); ok {
		t.Error("a rejected request must not leave a pending checkpoint")
	}

	// Submitting for a run with nothing pending.
	if err := broker.Submit("nobody", map[string]string{"q": "a"}); err != ErrNoPendingUserQuestion {
		t.Errorf("Submit with nothing pending = %v, want ErrNoPendingUserQuestion", err)
	}
	if _, ok := broker.Pending("nobody"); ok {
		t.Error("there should be no pending question for an unknown run")
	}
}

func TestCheckpointAskUserQuestionBroker_SubmitRejectsAnswersFailingValidation(t *testing.T) {
	broker := NewCheckpointAskUserQuestionBroker(newTestCheckpointService(), nil)

	go func() {
		_, _, _ = broker.Ask(context.Background(), htools.AskUserQuestionRequest{
			RunID: "run-6", CallID: "c", Questions: sampleQuestions(), Timeout: 5 * time.Second,
		})
	}()
	waitForPending(t, func() bool { _, ok := broker.Pending("run-6"); return ok })

	// "zzz" is not one of the offered labels.
	if err := broker.Submit("run-6", map[string]string{"Pick one": "zzz"}); err != ErrInvalidUserQuestionInput {
		t.Errorf("Submit with an unoffered answer = %v, want ErrInvalidUserQuestionInput", err)
	}
	// The checkpoint must still be pending so the operator can retry.
	if _, ok := broker.Pending("run-6"); !ok {
		t.Error("a rejected answer must leave the question pending for a retry")
	}
}

func TestCheckpointAskUserQuestionBroker_Timeout(t *testing.T) {
	broker := NewCheckpointAskUserQuestionBroker(newTestCheckpointService(), nil)

	_, _, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
		RunID: "run-7", CallID: "c", Questions: sampleQuestions(), Timeout: 30 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if !htools.IsAskUserQuestionTimeout(err) {
		t.Errorf("error %v should be an AskUserQuestionTimeoutError", err)
	}
	if _, ok := broker.Pending("run-7"); ok {
		t.Error("a timed-out question must not remain pending")
	}
}

type blockResolvedUpdateStore struct {
	checkpoints.Store
	applied chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockResolvedUpdateStore) Update(ctx context.Context, record *checkpoints.Record) error {
	if err := s.Store.Update(ctx, record); err != nil {
		return err
	}
	if record.Status == checkpoints.StatusResumed {
		s.once.Do(func() { close(s.applied) })
		<-s.release
	}
	return nil
}

func (s *blockResolvedUpdateStore) ResolvePending(
	ctx context.Context,
	id string,
	status checkpoints.Status,
	resumePayload string,
	updatedAt time.Time,
) (*checkpoints.Record, bool, error) {
	record, won, err := s.Store.ResolvePending(ctx, id, status, resumePayload, updatedAt)
	if err != nil {
		return nil, false, err
	}
	if won && status == checkpoints.StatusResumed {
		s.once.Do(func() { close(s.applied) })
		<-s.release
	}
	return record, won, nil
}

func TestCheckpointAskUserQuestionBroker_WaitDeadlineReturnsAcceptedAnswer(t *testing.T) {
	store := &blockResolvedUpdateStore{
		Store:   checkpoints.NewMemoryStore(),
		applied: make(chan struct{}),
		release: make(chan struct{}),
	}
	service := checkpoints.NewService(store, time.Now)
	broker := NewCheckpointAskUserQuestionBroker(service, time.Now)
	type result struct {
		answers map[string]string
		err     error
	}
	asked := make(chan result, 1)
	go func() {
		answers, _, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
			RunID: "run-wait-deadline", CallID: "call-wait-deadline",
			Questions: sampleQuestions(), Timeout: 100 * time.Millisecond,
		})
		asked <- result{answers: answers, err: err}
	}()
	waitForPending(t, func() bool {
		_, ok := broker.Pending("run-wait-deadline")
		return ok
	})

	submitted := make(chan error, 1)
	go func() {
		submitted <- broker.Submit("run-wait-deadline", map[string]string{"Pick one": "a"})
	}()
	receiveWithin(t, store.applied)
	select {
	case outcome := <-asked:
		t.Fatalf("Ask returned before the accepted resume completed: %+v", outcome)
	case <-time.After(150 * time.Millisecond):
	}
	close(store.release)
	if err := receiveWithin(t, submitted); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	outcome := receiveWithin(t, asked)
	if outcome.err != nil {
		t.Fatalf("Ask returned error after accepted resume: %v", outcome.err)
	}
	if got := outcome.answers["Pick one"]; got != "a" {
		t.Fatalf("answer = %q, want a", got)
	}
}

func TestDecodeQuestions(t *testing.T) {
	if _, err := decodeQuestions("not json"); err == nil {
		t.Error("malformed stored questions must produce an error")
	}
	got, err := decodeQuestions(`[{"question":"q","header":"h","options":[{"label":"a","description":"d"},{"label":"b","description":"d"}]}]`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Question != "q" {
		t.Errorf("decoded questions = %+v", got)
	}
}

// asApprovalTimeout is a tiny errors.As wrapper kept local so the test reads
// plainly at its call site.
func asApprovalTimeout(err error, target **ApprovalTimeoutError) bool {
	for err != nil {
		if t, ok := err.(*ApprovalTimeoutError); ok {
			*target = t
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
