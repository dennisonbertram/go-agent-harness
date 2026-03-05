package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type askBrokerStub struct {
	askAnswers map[string]string
	askErr     error
	lastReq    AskUserQuestionRequest
}

func (s *askBrokerStub) Ask(_ context.Context, req AskUserQuestionRequest) (map[string]string, time.Time, error) {
	s.lastReq = req
	if s.askErr != nil {
		return nil, time.Time{}, s.askErr
	}
	return s.askAnswers, time.Now().UTC(), nil
}

func (s *askBrokerStub) Pending(string) (AskUserQuestionPending, bool) {
	return AskUserQuestionPending{}, false
}

func (s *askBrokerStub) Submit(string, map[string]string) error {
	return nil
}

func TestAskUserQuestionToolReturnsQuestionsAndAnswers(t *testing.T) {
	t.Parallel()

	broker := &askBrokerStub{askAnswers: map[string]string{"Where next?": "Docs"}}
	tool := askUserQuestionTool(broker, 3*time.Minute)

	ctx := context.WithValue(context.Background(), ContextKeyRunID, "run_123")
	ctx = context.WithValue(ctx, ContextKeyToolCallID, "call_123")

	out, err := tool.Handler(ctx, json.RawMessage(`{"questions":[{"question":"Where next?","header":"Next","options":[{"label":"Docs","description":"Open docs"},{"label":"Code","description":"Open code"}],"multiSelect":false}]}`))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	if broker.lastReq.RunID != "run_123" || broker.lastReq.CallID != "call_123" {
		t.Fatalf("unexpected request ids: %+v", broker.lastReq)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	answers := payload["answers"].(map[string]any)
	if answers["Where next?"].(string) != "Docs" {
		t.Fatalf("unexpected answers payload: %+v", answers)
	}
}

func TestAskUserQuestionToolValidationAndTimeoutHelpers(t *testing.T) {
	t.Parallel()

	if _, err := ParseAskUserQuestionArgs(json.RawMessage(`{"questions":[]}`)); err == nil {
		t.Fatalf("expected validation error")
	}

	err := &AskUserQuestionTimeoutError{DeadlineAt: time.Now().UTC()}
	if err.Error() == "" {
		t.Fatalf("expected non-empty timeout error string")
	}
	var nilErr *AskUserQuestionTimeoutError
	if nilErr.Error() == "" {
		t.Fatalf("expected non-empty nil timeout error string")
	}
	if !IsAskUserQuestionTimeout(err) {
		t.Fatalf("expected timeout helper to match")
	}
	if IsAskUserQuestionTimeout(errors.New("x")) {
		t.Fatalf("did not expect timeout helper match")
	}
}

func TestNormalizeAskUserAnswersValidatesShape(t *testing.T) {
	t.Parallel()

	questions := []AskUserQuestion{
		{
			Question:    "Pick one",
			Header:      "One",
			MultiSelect: false,
			Options:     []AskUserQuestionOption{{Label: "A", Description: "a"}, {Label: "B", Description: "b"}},
		},
		{
			Question:    "Pick many",
			Header:      "Many",
			MultiSelect: true,
			Options:     []AskUserQuestionOption{{Label: "X", Description: "x"}, {Label: "Y", Description: "y"}},
		},
	}

	normalized, err := NormalizeAskUserAnswers(questions, map[string]string{"Pick one": "A", "Pick many": "Y, X, Y"})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized["Pick many"] != "X,Y" {
		t.Fatalf("unexpected normalized multi-select value: %q", normalized["Pick many"])
	}

	if _, err := NormalizeAskUserAnswers(questions, map[string]string{"Pick one": "Z", "Pick many": "X"}); err == nil {
		t.Fatalf("expected invalid label error")
	}
	if _, err := NormalizeAskUserAnswers(questions, map[string]string{"Pick one": "A"}); err == nil {
		t.Fatalf("expected missing answer error")
	}
}
