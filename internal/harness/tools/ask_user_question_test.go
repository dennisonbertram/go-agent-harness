package tools

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// TestAskUserQuestionToolReturnsQuestionsAndAnswers was ported to
// internal/harness/tools/core (core.AskUserQuestionTool is the surviving
// constructor); see core/core_test.go.

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
