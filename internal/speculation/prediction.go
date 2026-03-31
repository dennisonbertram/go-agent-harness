package speculation

import "strings"

// Prediction represents a predicted next user input.
type Prediction struct {
	// Text is the predicted input text.
	Text string

	// Confidence is a score in [0, 1] indicating how confident the predictor is.
	Confidence float64

	// Source describes how the prediction was generated (e.g., "pattern-match", "llm-prediction").
	Source string
}

// wordCount returns the number of whitespace-separated words in s.
func wordCount(s string) int {
	fields := strings.Fields(s)
	return len(fields)
}

// IsTooShort checks if a prediction is below the minimum length.
// Minimum is 2 words, with exceptions for known single-word commands.
func (p *Prediction) IsTooShort() bool {
	v := DefaultPredictionValidator()
	words := strings.Fields(p.Text)
	if len(words) >= v.MinWords {
		return false
	}
	// Single word: check exceptions
	if len(words) == 1 {
		word := strings.ToLower(words[0])
		for _, ex := range v.SingleWordOK {
			if word == strings.ToLower(ex) {
				return false
			}
		}
	}
	return true
}

// IsTooLong checks if a prediction exceeds the maximum length (12 words).
func (p *Prediction) IsTooLong() bool {
	v := DefaultPredictionValidator()
	return wordCount(p.Text) > v.MaxWords
}

// IsValid checks all validation rules: not too short, not too long.
func (p *Prediction) IsValid() bool {
	return !p.IsTooShort() && !p.IsTooLong()
}

// PredictionValidator validates predictions against quality rules.
type PredictionValidator struct {
	// MinWords is the minimum number of words required (default 2).
	MinWords int

	// MaxWords is the maximum number of words allowed (default 12).
	MaxWords int

	// SingleWordOK contains words that are valid as single-word predictions.
	SingleWordOK []string
}

// DefaultPredictionValidator returns a validator with the standard rules.
func DefaultPredictionValidator() PredictionValidator {
	return PredictionValidator{
		MinWords: 2,
		MaxWords: 12,
		SingleWordOK: []string{
			"yes",
			"no",
			"push",
			"commit",
			"deploy",
			"continue",
			"stop",
			"help",
		},
	}
}

// Validate checks a prediction against all rules.
// Returns a slice of violation messages; empty means the prediction is valid.
func (v *PredictionValidator) Validate(p Prediction) []string {
	var violations []string

	words := strings.Fields(p.Text)
	wc := len(words)

	if wc < v.MinWords {
		// Check single-word exceptions
		if wc == 1 {
			word := strings.ToLower(words[0])
			isException := false
			for _, ex := range v.SingleWordOK {
				if word == strings.ToLower(ex) {
					isException = true
					break
				}
			}
			if !isException {
				violations = append(violations,
					"prediction is too short: single word '"+words[0]+"' is not in the exception list")
			}
		} else {
			violations = append(violations,
				"prediction is too short: fewer than 2 words")
		}
	}

	if wc > v.MaxWords {
		violations = append(violations,
			"prediction is too long: exceeds maximum word count")
	}

	return violations
}
