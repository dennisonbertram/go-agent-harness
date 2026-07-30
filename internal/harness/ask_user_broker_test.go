package harness

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

func askQuestionsFixture() []htools.AskUserQuestion {
	return []htools.AskUserQuestion{
		{
			Question:    "Where next?",
			Header:      "Route",
			MultiSelect: false,
			Options:     []htools.AskUserQuestionOption{{Label: "Docs", Description: "Read docs"}, {Label: "Code", Description: "Read code"}},
		},
	}
}

func TestInMemoryAskUserQuestionBrokerLifecycle(t *testing.T) {
	t.Parallel()

	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	errCh := make(chan error, 1)
	answersCh := make(chan map[string]string, 1)
	pendingReady := make(chan htools.AskUserQuestionPending, 1)

	go func() {
		answers, _, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
			RunID:     "run_1",
			CallID:    "call_1",
			Questions: askQuestionsFixture(),
			Timeout:   2 * time.Second,
			OnPending: func(pending htools.AskUserQuestionPending) {
				if current, ok := broker.Pending("run_1"); !ok || current.CallID != pending.CallID {
					errCh <- errors.New("pending input was not readable inside OnPending")
					return
				}
				pendingReady <- pending
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		answersCh <- answers
	}()

	select {
	case err := <-errCh:
		t.Fatalf("unexpected readiness error: %v", err)
	case pending := <-pendingReady:
		if pending.CallID != "call_1" {
			t.Fatalf("unexpected call id: %q", pending.CallID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending notification")
	}

	if err := broker.Submit("run_1", map[string]string{"Where next?": "Docs"}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("unexpected ask error: %v", err)
	case answers := <-answersCh:
		if answers["Where next?"] != "Docs" {
			t.Fatalf("unexpected answers: %+v", answers)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for answer")
	}

	if _, ok := broker.Pending("run_1"); ok {
		t.Fatalf("expected no pending question after submit")
	}
}

func TestInMemoryAskUserQuestionBrokerTimeoutIncludesPendingNotification(t *testing.T) {
	t.Parallel()

	const timeout = 200 * time.Millisecond
	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	notificationStarted := make(chan struct{})
	releaseNotification := make(chan struct{})
	result := make(chan error, 1)

	go func() {
		_, _, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
			RunID:     "run_slow_notification",
			CallID:    "call_slow_notification",
			Questions: askQuestionsFixture(),
			Timeout:   timeout,
			OnPending: func(htools.AskUserQuestionPending) {
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
	close(releaseNotification)

	select {
	case err := <-result:
		if !htools.IsAskUserQuestionTimeout(err) {
			t.Fatalf("Ask error = %v, want timeout", err)
		}
	case <-time.After(timeout / 2):
		t.Fatal("Ask timeout clock did not run while OnPending was blocked")
	}
}

func TestInMemoryAskUserQuestionBrokerTimeoutAndValidation(t *testing.T) {
	t.Parallel()

	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	_, _, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
		RunID:     "run_timeout",
		CallID:    "call_timeout",
		Questions: askQuestionsFixture(),
		Timeout:   20 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !htools.IsAskUserQuestionTimeout(err) {
		t.Fatalf("expected timeout error type, got %v", err)
	}

	go func() {
		_, _, _ = broker.Ask(context.Background(), htools.AskUserQuestionRequest{
			RunID:     "run_invalid",
			CallID:    "call_invalid",
			Questions: askQuestionsFixture(),
			Timeout:   2 * time.Second,
		})
	}()

	deadline := time.Now().Add(1 * time.Second)
	for {
		if _, ok := broker.Pending("run_invalid"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pending question did not appear")
		}
		time.Sleep(5 * time.Millisecond)
	}

	err = broker.Submit("run_invalid", map[string]string{"Where next?": "Nope"})
	if err == nil {
		t.Fatalf("expected invalid submission error")
	}
	if !errors.Is(err, ErrInvalidUserQuestionInput) {
		t.Fatalf("expected ErrInvalidUserQuestionInput, got %v", err)
	}
	if _, ok := broker.Pending("run_invalid"); !ok {
		t.Fatalf("expected pending question to remain after invalid submission")
	}

	err = broker.Submit("missing", map[string]string{"x": "y"})
	if !errors.Is(err, ErrNoPendingUserQuestion) {
		t.Fatalf("expected ErrNoPendingUserQuestion, got %v", err)
	}

	err = broker.Submit("run_invalid", map[string]string{"Where next?": "Docs"})
	if err != nil {
		t.Fatalf("expected valid submit, got %v", err)
	}
	if _, ok := broker.Pending("run_invalid"); ok {
		t.Fatalf("expected pending to clear on valid submit")
	}
}

func TestInMemoryAskUserQuestionBrokerRejectsConcurrentPending(t *testing.T) {
	t.Parallel()

	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_, _, _ = broker.Ask(ctx, htools.AskUserQuestionRequest{
			RunID:     "run_dupe",
			CallID:    "call_1",
			Questions: askQuestionsFixture(),
			Timeout:   2 * time.Second,
		})
	}()

	deadline := time.Now().Add(1 * time.Second)
	for {
		if _, ok := broker.Pending("run_dupe"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pending question did not appear")
		}
		time.Sleep(5 * time.Millisecond)
	}

	_, _, err := broker.Ask(context.Background(), htools.AskUserQuestionRequest{
		RunID:     "run_dupe",
		CallID:    "call_2",
		Questions: askQuestionsFixture(),
		Timeout:   2 * time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "pending user question already exists") {
		t.Fatalf("expected duplicate pending error, got %v", err)
	}
}
