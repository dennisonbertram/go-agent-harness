package harness

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go-agent-harness/internal/checkpoints"
	htools "go-agent-harness/internal/harness/tools"
)

func TestCheckpointApprovalBrokerPersistsPendingApproval(t *testing.T) {
	t.Parallel()

	checkpointSvc := checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now)
	broker := NewCheckpointApprovalBroker(checkpointSvc)

	done := make(chan error, 1)
	go func() {
		approved, _, err := broker.Ask(context.Background(), ApprovalRequest{
			RunID:   "run-1",
			CallID:  "call-1",
			Tool:    "write",
			Args:    `{"path":"README.md"}`,
			Timeout: time.Minute,
		})
		if err != nil {
			done <- err
			return
		}
		if !approved {
			done <- context.Canceled
			return
		}
		done <- nil
	}()

	var pending ApprovalPendingView
	deadline := time.Now().Add(2 * time.Second)
	for {
		current, ok := broker.Pending("run-1")
		if ok {
			pending = ApprovalPendingView(current)
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for pending approval")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if pending.Tool != "write" {
		t.Fatalf("pending tool = %q, want write", pending.Tool)
	}
	record, ok, err := checkpointSvc.PendingByRun(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("PendingByRun: %v", err)
	}
	if !ok {
		t.Fatal("expected persisted checkpoint")
	}
	if record.Kind != checkpoints.KindApproval {
		t.Fatalf("kind = %q, want %q", record.Kind, checkpoints.KindApproval)
	}

	if err := broker.Approve("run-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Ask completion: %v", err)
	}
}

// TestCheckpointApprovalBrokerRegisterIsImmediatelyResolvable proves the
// durable broker creates its checkpoint before a caller publishes an approval
// event, and preserves an approval that arrives before Wait begins.
func TestCheckpointApprovalBrokerRegisterIsImmediatelyResolvable(t *testing.T) {
	t.Parallel()

	checkpointSvc := checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now)
	broker := NewCheckpointApprovalBroker(checkpointSvc)
	waiter, err := broker.Register(context.Background(), ApprovalRequest{
		RunID:   "run-checkpoint-ready",
		CallID:  "call-checkpoint-ready",
		Tool:    "write",
		Args:    `{"path":"ready.txt"}`,
		Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	pending := waiter.Pending()
	if pending.RunID != "run-checkpoint-ready" || pending.DeadlineAt.IsZero() {
		t.Fatalf("registered pending = %#v, want run and deadline", pending)
	}
	if err := broker.Deny("run-checkpoint-ready"); err != nil {
		t.Fatalf("Deny immediately after Register: %v", err)
	}
	approved, option, err := waiter.Wait(context.Background())
	if err != nil || approved || option != "" {
		t.Fatalf("Wait after immediate deny = (%v, %q, %v), want (false, \"\", nil)", approved, option, err)
	}
}

// TestCheckpointApprovalBrokerPreWaitResolutionSurvivesDeadline is the
// durable counterpart to the in-memory regression: an approve or deny that
// commits before Wait starts cannot be overwritten by delayed expiry.
func TestCheckpointApprovalBrokerPreWaitResolutionSurvivesDeadline(t *testing.T) {
	for _, tc := range []struct {
		name         string
		resolve      func(ApprovalBroker, string) error
		wantApproved bool
	}{
		{name: "approve", resolve: func(b ApprovalBroker, runID string) error { return b.Approve(runID) }, wantApproved: true},
		{name: "deny", resolve: func(b ApprovalBroker, runID string) error { return b.Deny(runID) }, wantApproved: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broker := NewCheckpointApprovalBroker(checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now))
			waiter, err := broker.Register(context.Background(), ApprovalRequest{
				RunID:   "run-checkpoint-pre-wait-" + tc.name,
				CallID:  "call-checkpoint-pre-wait-" + tc.name,
				Tool:    "write",
				Timeout: 20 * time.Millisecond,
			})
			if err != nil {
				t.Fatalf("Register: %v", err)
			}
			if err := tc.resolve(broker, "run-checkpoint-pre-wait-"+tc.name); err != nil {
				t.Fatalf("%s before Wait: %v", tc.name, err)
			}
			time.Sleep(40 * time.Millisecond)
			approved, _, err := waiter.Wait(context.Background())
			if err != nil || approved != tc.wantApproved {
				t.Fatalf("Wait after pre-deadline %s = (%v, %v), want (%v, nil)", tc.name, approved, err, tc.wantApproved)
			}
		})
	}
}

func TestCheckpointApprovalBrokerExpiryWinnerRejectsLateResolution(t *testing.T) {
	for _, tc := range []struct {
		name    string
		resolve func(ApprovalBroker, string) error
	}{
		{name: "approve", resolve: func(b ApprovalBroker, runID string) error { return b.Approve(runID) }},
		{name: "deny", resolve: func(b ApprovalBroker, runID string) error { return b.Deny(runID) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broker := NewCheckpointApprovalBroker(checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now))
			runID := "run-checkpoint-expiry-wins-" + tc.name
			waiter, err := broker.Register(context.Background(), ApprovalRequest{
				RunID: runID, CallID: "call-expiry", Tool: "write", Timeout: 20 * time.Millisecond,
			})
			if err != nil {
				t.Fatalf("Register: %v", err)
			}
			time.Sleep(40 * time.Millisecond)
			if _, _, err := waiter.Wait(context.Background()); !IsApprovalTimeout(err) {
				t.Fatalf("Wait = %v, want ApprovalTimeoutError", err)
			}
			if err := tc.resolve(broker, runID); !errors.Is(err, ErrNoPendingApproval) {
				t.Fatalf("late %s = %v, want ErrNoPendingApproval", tc.name, err)
			}
		})
	}
}

func TestCheckpointApprovalBrokerResolutionExpiryRaceIsConsistent(t *testing.T) {
	for _, tc := range []struct {
		name         string
		resolve      func(ApprovalBroker, string) error
		wantApproved bool
	}{
		{name: "approve", resolve: func(b ApprovalBroker, runID string) error { return b.Approve(runID) }, wantApproved: true},
		{name: "deny", resolve: func(b ApprovalBroker, runID string) error { return b.Deny(runID) }, wantApproved: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broker := NewCheckpointApprovalBroker(checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now))
			runID := "run-checkpoint-expiry-race-" + tc.name
			waiter, err := broker.Register(context.Background(), ApprovalRequest{
				RunID: runID, CallID: "call-expiry-race", Tool: "write", Timeout: 5 * time.Millisecond,
			})
			if err != nil {
				t.Fatalf("Register: %v", err)
			}
			time.Sleep(10 * time.Millisecond)
			start := make(chan struct{})
			waited := make(chan approvalBrokerResult, 1)
			resolved := make(chan error, 1)
			go func() {
				<-start
				approved, _, err := waiter.Wait(context.Background())
				waited <- approvalBrokerResult{approved: approved, err: err}
			}()
			go func() {
				<-start
				resolved <- tc.resolve(broker, runID)
			}()
			close(start)
			resolveErr := <-resolved
			waitResult := <-waited
			switch {
			case resolveErr == nil:
				if waitResult.err != nil || waitResult.approved != tc.wantApproved {
					t.Fatalf("successful %s discarded: Wait=(%v, %v)", tc.name, waitResult.approved, waitResult.err)
				}
			case errors.Is(resolveErr, ErrNoPendingApproval):
				if !IsApprovalTimeout(waitResult.err) {
					t.Fatalf("expiry won but Wait=%v, want ApprovalTimeoutError", waitResult.err)
				}
			default:
				t.Fatalf("%s returned unexpected error: %v", tc.name, resolveErr)
			}
		})
	}
}

// TestCheckpointApprovalBrokerWaitCancellationRetainsPendingCharacterizes the
// existing checkpoint contract: parent cancellation returns context.Canceled
// without expiring the durable record. Splitting Register from Wait must not
// alter that behavior; expiry remains owned by the timeout path.
func TestCheckpointApprovalBrokerWaitCancellationRetainsPending(t *testing.T) {
	t.Parallel()

	checkpointSvc := checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now)
	broker := NewCheckpointApprovalBroker(checkpointSvc)
	ctx, cancel := context.WithCancel(context.Background())
	waiter, err := broker.Register(ctx, ApprovalRequest{
		RunID:   "run-checkpoint-cancel",
		CallID:  "call-checkpoint-cancel",
		Tool:    "write",
		Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	cancel()
	_, _, err = waiter.Wait(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait after cancellation = %v, want context.Canceled", err)
	}
	if _, ok := broker.Pending("run-checkpoint-cancel"); !ok {
		t.Fatal("cancellation unexpectedly removed the checkpoint-backed pending approval")
	}
}

func TestCheckpointApprovalBrokerDenyRejectsPendingApproval(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	checkpointSvc := checkpoints.NewService(checkpoints.NewMemoryStore(), func() time.Time { return now })
	broker := NewCheckpointApprovalBroker(checkpointSvc)

	resultCh := make(chan bool, 1)
	errCh := make(chan error, 1)
	go func() {
		approved, _, err := broker.Ask(context.Background(), ApprovalRequest{
			RunID:   "run-denied",
			CallID:  "call-denied",
			Tool:    "write",
			Args:    `{"path":"blocked.txt"}`,
			Timeout: time.Minute,
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- approved
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, ok := broker.Pending("run-denied"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for pending approval")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := broker.Deny("run-denied"); err != nil {
		t.Fatalf("Deny: %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatalf("Ask returned error: %v", err)
	case approved := <-resultCh:
		if approved {
			t.Fatal("denied approval returned approved=true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for denied approval result")
	}
	if err := broker.Deny("run-denied"); err != ErrNoPendingApproval {
		t.Fatalf("Deny after resolution = %v, want ErrNoPendingApproval", err)
	}
}

func TestCheckpointAskUserBrokerPersistsQuestionsAndAnswers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	checkpointSvc := checkpoints.NewService(checkpoints.NewMemoryStore(), func() time.Time { return now })
	broker := NewCheckpointAskUserQuestionBroker(checkpointSvc, func() time.Time { return now })

	done := make(chan error, 1)
	pendingReady := make(chan htools.AskUserQuestionPending, 1)
	go func() {
		answers, answeredAt, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
			RunID:  "run-ask",
			CallID: "call-ask",
			Questions: []htools.AskUserQuestion{{
				Question: "Where next?",
				Header:   "Route",
				Options: []htools.AskUserQuestionOption{
					{Label: "Docs", Description: "Read docs"},
					{Label: "Code", Description: "Read code"},
				},
			}},
			Timeout: time.Minute,
			OnPending: func(_ context.Context, pending htools.AskUserQuestionPending) {
				if current, ok := broker.Pending("run-ask"); !ok || current.CallID != pending.CallID {
					done <- errors.New("persisted pending input was not readable inside OnPending")
					return
				}
				pendingReady <- pending
			},
		})
		if err != nil {
			done <- err
			return
		}
		if answeredAt != now {
			done <- context.Canceled
			return
		}
		if answers["Where next?"] != "Docs" {
			done <- context.Canceled
			return
		}
		done <- nil
	}()

	var pending htools.AskUserQuestionPending
	select {
	case err := <-done:
		t.Fatalf("unexpected readiness error: %v", err)
	case pending = <-pendingReady:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pending notification")
	}

	if pending.CallID != "call-ask" {
		t.Fatalf("pending call id = %q, want call-ask", pending.CallID)
	}
	record, ok, err := checkpointSvc.PendingByRun(context.Background(), "run-ask")
	if err != nil {
		t.Fatalf("PendingByRun: %v", err)
	}
	if !ok {
		t.Fatal("expected persisted checkpoint")
	}
	if record.Kind != checkpoints.KindUserInput {
		t.Fatalf("kind = %q, want %q", record.Kind, checkpoints.KindUserInput)
	}
	var questions []htools.AskUserQuestion
	if err := json.Unmarshal([]byte(record.Questions), &questions); err != nil {
		t.Fatalf("unmarshal questions: %v", err)
	}
	if len(questions) != 1 || questions[0].Question != "Where next?" {
		t.Fatalf("unexpected persisted questions: %+v", questions)
	}

	if err := broker.Submit("run-ask", map[string]string{"Where next?": "Docs"}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Ask completion: %v", err)
	}
}

func TestCheckpointAskUserBrokerTimeoutIncludesPendingNotification(t *testing.T) {
	t.Parallel()

	const timeout = 200 * time.Millisecond
	checkpointSvc := checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now)
	broker := NewCheckpointAskUserQuestionBroker(checkpointSvc, time.Now)
	notificationStarted := make(chan struct{})
	releaseNotification := make(chan struct{})
	result := make(chan error, 1)
	released := false
	defer func() {
		if !released {
			close(releaseNotification)
		}
	}()

	go func() {
		_, _, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
			RunID:     "run-slow-notification",
			CallID:    "call-slow-notification",
			Questions: askQuestionsFixture(),
			Timeout:   timeout,
			OnPending: func(_ context.Context, _ htools.AskUserQuestionPending) {
				close(notificationStarted)
				<-releaseNotification
			},
		})
		result <- err
	}()

	select {
	case <-notificationStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending notification")
	}
	time.Sleep(timeout + 50*time.Millisecond)

	select {
	case err := <-result:
		if !htools.IsAskUserQuestionTimeout(err) {
			t.Fatalf("Ask error = %v, want timeout", err)
		}
	case <-time.After(timeout / 2):
		t.Fatal("Ask did not honor its timeout while OnPending remained blocked")
	}
	close(releaseNotification)
	released = true
}

func TestCheckpointAskUserBrokerKeepsAnswerSubmittedBeforeNotifierDeadline(t *testing.T) {
	t.Parallel()

	const timeout = 150 * time.Millisecond
	checkpointSvc := checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now)
	broker := NewCheckpointAskUserQuestionBroker(checkpointSvc, time.Now)
	started := make(chan struct{})
	release := make(chan struct{})
	type askResult struct {
		answers map[string]string
		err     error
	}
	result := make(chan askResult, 1)
	go func() {
		answers, _, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
			RunID:     "run-answered-before-deadline",
			CallID:    "call-answered-before-deadline",
			Questions: askQuestionsFixture(),
			Timeout:   timeout,
			OnPending: func(_ context.Context, _ htools.AskUserQuestionPending) {
				close(started)
				<-release
			},
		})
		result <- askResult{answers: answers, err: err}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pending publication")
	}
	if err := broker.Submit("run-answered-before-deadline", map[string]string{"Where next?": "Docs"}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	time.Sleep(timeout + 50*time.Millisecond)
	select {
	case out := <-result:
		t.Fatalf("Ask returned before pending publication completed: %+v", out)
	default:
	}
	close(release)
	var out askResult
	select {
	case out = <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Ask result after pending publication")
	}
	if out.err != nil {
		t.Fatalf("Ask returned error after timely answer: %v", out.err)
	}
	if got := out.answers["Where next?"]; got != "Docs" {
		t.Fatalf("answer = %q, want Docs", got)
	}
}

type ApprovalPendingView PendingApproval

// TestCheckpointApprovalBrokerOptionsRoundTrip proves plan approach options
// survive the checkpoint-backed broker: they are persisted on the pending
// record, and the operator's selected option comes back to the blocked Ask.
func TestCheckpointApprovalBrokerOptionsRoundTrip(t *testing.T) {
	t.Parallel()

	checkpointSvc := checkpoints.NewService(checkpoints.NewMemoryStore(), time.Now)
	broker := NewCheckpointApprovalBroker(checkpointSvc)
	options := []PlanApproachOption{{ID: "a", Label: "One"}, {ID: "b", Label: "Two"}}

	type askResult struct {
		approved bool
		option   string
		err      error
	}
	resultCh := make(chan askResult, 1)
	go func() {
		approved, option, err := broker.Ask(context.Background(), ApprovalRequest{
			RunID:   "run-opts",
			CallID:  "plan_exit",
			Tool:    "plan_exit",
			Options: options,
			Timeout: time.Minute,
		})
		resultCh <- askResult{approved: approved, option: option, err: err}
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, ok := broker.Pending("run-opts"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for pending approval")
		}
		time.Sleep(10 * time.Millisecond)
	}
	pending, ok := broker.Pending("run-opts")
	if !ok {
		t.Fatal("no pending approval")
	}
	if len(pending.Options) != 2 || pending.Options[0].ID != "a" || pending.Options[1].Label != "Two" {
		t.Fatalf("pending options = %#v", pending.Options)
	}

	if err := broker.ApproveWithOption("run-opts", "b"); err != nil {
		t.Fatalf("ApproveWithOption: %v", err)
	}
	res := <-resultCh
	if res.err != nil {
		t.Fatalf("Ask error: %v", res.err)
	}
	if !res.approved || res.option != "b" {
		t.Fatalf("Ask returned approved=%v option=%q, want approved=true option=%q", res.approved, res.option, "b")
	}
}
