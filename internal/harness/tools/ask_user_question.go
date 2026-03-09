package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-agent-harness/internal/harness/tools/descriptions"
)

const AskUserQuestionToolName = "AskUserQuestion"

type AskUserQuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type AskUserQuestion struct {
	Question    string                  `json:"question"`
	Header      string                  `json:"header"`
	Options     []AskUserQuestionOption `json:"options"`
	MultiSelect bool                    `json:"multiSelect"`
}

type askUserQuestionArgs struct {
	Questions []AskUserQuestion `json:"questions"`
	Answers   map[string]string `json:"answers,omitempty"`
}

type AskUserQuestionTimeoutError struct {
	RunID      string
	CallID     string
	DeadlineAt time.Time
}

func (e *AskUserQuestionTimeoutError) Error() string {
	if e == nil {
		return "ask user question timed out"
	}
	if e.DeadlineAt.IsZero() {
		return "ask user question timed out"
	}
	return fmt.Sprintf("ask user question timed out at %s", e.DeadlineAt.UTC().Format(time.RFC3339))
}

func IsAskUserQuestionTimeout(err error) bool {
	var timeoutErr *AskUserQuestionTimeoutError
	return errors.As(err, &timeoutErr)
}

func ParseAskUserQuestionArgs(raw json.RawMessage) ([]AskUserQuestion, error) {
	var args askUserQuestionArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse AskUserQuestion args: %w", err)
	}
	if err := ValidateAskUserQuestions(args.Questions); err != nil {
		return nil, err
	}
	return args.Questions, nil
}

func ValidateAskUserQuestions(questions []AskUserQuestion) error {
	if len(questions) < 1 || len(questions) > 4 {
		return fmt.Errorf("questions must contain 1-4 items")
	}

	seenQuestions := make(map[string]struct{}, len(questions))
	for i, q := range questions {
		qText := strings.TrimSpace(q.Question)
		if qText == "" {
			return fmt.Errorf("questions[%d].question is required", i)
		}
		if _, exists := seenQuestions[qText]; exists {
			return fmt.Errorf("questions must have unique question text")
		}
		seenQuestions[qText] = struct{}{}

		if strings.TrimSpace(q.Header) == "" {
			return fmt.Errorf("questions[%d].header is required", i)
		}
		if len(q.Options) < 2 || len(q.Options) > 4 {
			return fmt.Errorf("questions[%d].options must contain 2-4 items", i)
		}
		seenLabels := make(map[string]struct{}, len(q.Options))
		for j, opt := range q.Options {
			label := strings.TrimSpace(opt.Label)
			if label == "" {
				return fmt.Errorf("questions[%d].options[%d].label is required", i, j)
			}
			if _, exists := seenLabels[label]; exists {
				return fmt.Errorf("questions[%d].options labels must be unique", i)
			}
			seenLabels[label] = struct{}{}
			if strings.TrimSpace(opt.Description) == "" {
				return fmt.Errorf("questions[%d].options[%d].description is required", i, j)
			}
		}
	}
	return nil
}

func NormalizeAskUserAnswers(questions []AskUserQuestion, answers map[string]string) (map[string]string, error) {
	if err := ValidateAskUserQuestions(questions); err != nil {
		return nil, err
	}
	if len(answers) != len(questions) {
		return nil, fmt.Errorf("answers must contain exactly one answer for each question")
	}

	type questionDef struct {
		multiSelect bool
		labels      map[string]struct{}
	}
	defs := make(map[string]questionDef, len(questions))
	for _, q := range questions {
		labelSet := make(map[string]struct{}, len(q.Options))
		for _, opt := range q.Options {
			labelSet[strings.TrimSpace(opt.Label)] = struct{}{}
		}
		defs[strings.TrimSpace(q.Question)] = questionDef{multiSelect: q.MultiSelect, labels: labelSet}
	}

	normalized := make(map[string]string, len(answers))
	for question, rawValue := range answers {
		qKey := strings.TrimSpace(question)
		def, ok := defs[qKey]
		if !ok {
			return nil, fmt.Errorf("unexpected question %q", question)
		}
		value := strings.TrimSpace(rawValue)
		if value == "" {
			return nil, fmt.Errorf("answer for %q is required", qKey)
		}

		if !def.multiSelect {
			if _, ok := def.labels[value]; !ok {
				return nil, fmt.Errorf("invalid answer %q for question %q", value, qKey)
			}
			normalized[qKey] = value
			continue
		}

		parts := strings.Split(value, ",")
		if len(parts) == 0 {
			return nil, fmt.Errorf("answer for %q is required", qKey)
		}
		unique := make(map[string]struct{}, len(parts))
		resolved := make([]string, 0, len(parts))
		for _, part := range parts {
			label := strings.TrimSpace(part)
			if label == "" {
				continue
			}
			if _, ok := def.labels[label]; !ok {
				return nil, fmt.Errorf("invalid answer %q for question %q", label, qKey)
			}
			if _, seen := unique[label]; seen {
				continue
			}
			unique[label] = struct{}{}
			resolved = append(resolved, label)
		}
		if len(resolved) == 0 {
			return nil, fmt.Errorf("answer for %q is required", qKey)
		}
		sort.Strings(resolved)
		normalized[qKey] = strings.Join(resolved, ",")
	}

	for _, q := range questions {
		qKey := strings.TrimSpace(q.Question)
		if _, ok := normalized[qKey]; !ok {
			return nil, fmt.Errorf("missing answer for question %q", qKey)
		}
	}
	return normalized, nil
}

func askUserQuestionTool(broker AskUserQuestionBroker, timeout time.Duration) Tool {
	def := Definition{
		Name:         AskUserQuestionToolName,
		Description:  descriptions.Load("AskUserQuestion"),
		Action:       ActionRead,
		Mutating:     false,
		ParallelSafe: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"questions": map[string]any{
					"type":     "array",
					"minItems": 1,
					"maxItems": 4,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"question": map[string]any{"type": "string"},
							"header":   map[string]any{"type": "string"},
							"options": map[string]any{
								"type":     "array",
								"minItems": 2,
								"maxItems": 4,
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"label":       map[string]any{"type": "string"},
										"description": map[string]any{"type": "string"},
									},
									"required": []string{"label", "description"},
								},
							},
							"multiSelect": map[string]any{"type": "boolean"},
						},
						"required": []string{"question", "header", "options", "multiSelect"},
					},
				},
			},
			"required": []string{"questions"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		if broker == nil {
			return "", fmt.Errorf("AskUserQuestion broker is not configured")
		}

		questions, err := ParseAskUserQuestionArgs(raw)
		if err != nil {
			return "", err
		}

		runID := RunIDFromContext(ctx)
		if strings.TrimSpace(runID) == "" {
			return "", fmt.Errorf("run context is required")
		}

		callID := ToolCallIDFromContext(ctx)
		answers, _, err := broker.Ask(ctx, AskUserQuestionRequest{
			RunID:     runID,
			CallID:    callID,
			Questions: questions,
			Timeout:   timeout,
		})
		if err != nil {
			return "", err
		}

		return MarshalToolResult(map[string]any{
			"questions": questions,
			"answers":   answers,
		})
	}

	return Tool{Definition: def, Handler: handler}
}
