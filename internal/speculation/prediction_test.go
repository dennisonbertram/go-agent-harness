package speculation_test

import (
	"testing"

	"go-agent-harness/internal/speculation"
)

// TestPrediction_IsTooShort_OneWord verifies a single non-exception word is too short.
func TestPrediction_IsTooShort_OneWord(t *testing.T) {
	p := speculation.Prediction{Text: "refactor", Confidence: 0.9, Source: "test"}
	if !p.IsTooShort() {
		t.Error("IsTooShort(): got false for single non-exception word 'refactor', want true")
	}
}

// TestPrediction_IsTooShort_SingleWordException verifies "yes" is not too short.
func TestPrediction_IsTooShort_SingleWordException(t *testing.T) {
	p := speculation.Prediction{Text: "yes", Confidence: 0.9, Source: "test"}
	if p.IsTooShort() {
		t.Error("IsTooShort(): got true for exception word 'yes', want false")
	}
}

// TestPrediction_IsTooShort_TwoWords verifies "run tests" is not too short.
func TestPrediction_IsTooShort_TwoWords(t *testing.T) {
	p := speculation.Prediction{Text: "run tests", Confidence: 0.9, Source: "test"}
	if p.IsTooShort() {
		t.Error("IsTooShort(): got true for two-word prediction 'run tests', want false")
	}
}

// TestPrediction_IsTooLong verifies 15-word prediction is too long.
func TestPrediction_IsTooLong(t *testing.T) {
	text := "can you please run all the tests and show me the output now please"
	p := speculation.Prediction{Text: text, Confidence: 0.9, Source: "test"}
	if !p.IsTooLong() {
		t.Errorf("IsTooLong(): got false for 13-word prediction, want true; text: %q", text)
	}
}

// TestPrediction_IsTooLong_ExactlyAtLimit verifies 12 words is not too long.
func TestPrediction_IsTooLong_ExactlyAtLimit(t *testing.T) {
	text := "one two three four five six seven eight nine ten eleven twelve"
	p := speculation.Prediction{Text: text, Confidence: 0.9, Source: "test"}
	if p.IsTooLong() {
		t.Errorf("IsTooLong(): got true for exactly 12-word prediction, want false; text: %q", text)
	}
}

// TestPrediction_IsValid_GoodPrediction verifies a 5-word prediction is valid.
func TestPrediction_IsValid_GoodPrediction(t *testing.T) {
	p := speculation.Prediction{Text: "run all the unit tests", Confidence: 0.8, Source: "test"}
	if !p.IsValid() {
		t.Error("IsValid(): got false for valid 5-word prediction, want true")
	}
}

// TestPrediction_IsValid_TooShort verifies invalid prediction is not valid.
func TestPrediction_IsValid_TooShort(t *testing.T) {
	p := speculation.Prediction{Text: "refactor", Confidence: 0.9, Source: "test"}
	if p.IsValid() {
		t.Error("IsValid(): got true for single non-exception word, want false")
	}
}

// TestPrediction_IsValid_TooLong verifies too-long prediction is not valid.
func TestPrediction_IsValid_TooLong(t *testing.T) {
	text := "can you please run all the tests and show me the full output right now"
	p := speculation.Prediction{Text: text, Confidence: 0.9, Source: "test"}
	if p.IsValid() {
		t.Errorf("IsValid(): got true for overly long prediction, want false; text: %q", text)
	}
}

// TestDefaultPredictionValidator verifies min=2, max=12, exceptions include yes/push/commit.
func TestDefaultPredictionValidator(t *testing.T) {
	v := speculation.DefaultPredictionValidator()

	if v.MinWords != 2 {
		t.Errorf("MinWords: got %d, want 2", v.MinWords)
	}
	if v.MaxWords != 12 {
		t.Errorf("MaxWords: got %d, want 12", v.MaxWords)
	}

	exceptions := map[string]bool{}
	for _, w := range v.SingleWordOK {
		exceptions[w] = true
	}
	for _, expected := range []string{"yes", "push", "commit"} {
		if !exceptions[expected] {
			t.Errorf("SingleWordOK: want %q in exceptions list, got %v", expected, v.SingleWordOK)
		}
	}
}

// TestPredictionValidator_Validate_Valid verifies no violations for a good prediction.
func TestPredictionValidator_Validate_Valid(t *testing.T) {
	v := speculation.DefaultPredictionValidator()
	p := speculation.Prediction{Text: "run the unit tests now", Confidence: 0.8, Source: "test"}
	violations := v.Validate(p)
	if len(violations) != 0 {
		t.Errorf("Validate(): got %d violations for valid prediction, want 0; violations: %v", len(violations), violations)
	}
}

// TestPredictionValidator_Validate_TooShort verifies violation for too-short prediction.
func TestPredictionValidator_Validate_TooShort(t *testing.T) {
	v := speculation.DefaultPredictionValidator()
	p := speculation.Prediction{Text: "refactor", Confidence: 0.9, Source: "test"}
	violations := v.Validate(p)
	if len(violations) == 0 {
		t.Error("Validate(): got 0 violations for too-short prediction, want at least 1")
	}
}

// TestPredictionValidator_Validate_TooLong verifies violation for too-long prediction.
func TestPredictionValidator_Validate_TooLong(t *testing.T) {
	v := speculation.DefaultPredictionValidator()
	text := "can you please run all the tests and show the results to me now please"
	p := speculation.Prediction{Text: text, Confidence: 0.9, Source: "test"}
	violations := v.Validate(p)
	if len(violations) == 0 {
		t.Errorf("Validate(): got 0 violations for too-long prediction %q, want at least 1", text)
	}
}

// TestPredictionValidator_Validate_SingleWordException verifies no violation for exception word.
func TestPredictionValidator_Validate_SingleWordException(t *testing.T) {
	v := speculation.DefaultPredictionValidator()
	p := speculation.Prediction{Text: "push", Confidence: 0.9, Source: "test"}
	violations := v.Validate(p)
	if len(violations) != 0 {
		t.Errorf("Validate(): got %d violations for exception word 'push', want 0; violations: %v", len(violations), violations)
	}
}
