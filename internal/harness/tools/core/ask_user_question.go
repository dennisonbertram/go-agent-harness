package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tools "go-agent-harness/internal/harness/tools"
)

// AskUserQuestionTool returns a core tool that asks the user structured
// clarification questions and waits for answers.
func AskUserQuestionTool(broker tools.AskUserQuestionBroker, timeout time.Duration) tools.Tool {
	def := tools.Definition{
		Name:         tools.AskUserQuestionToolName,
		Description:  "Ask the user one to four structured clarification questions and wait for answers",
		Action:       tools.ActionRead,
		Mutating:     false,
		ParallelSafe: false,
		Tier:         tools.TierCore,
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

		questions, err := tools.ParseAskUserQuestionArgs(raw)
		if err != nil {
			return "", err
		}

		runID := tools.RunIDFromContext(ctx)
		if strings.TrimSpace(runID) == "" {
			return "", fmt.Errorf("run context is required")
		}

		callID := tools.ToolCallIDFromContext(ctx)
		answers, _, err := broker.Ask(ctx, tools.AskUserQuestionRequest{
			RunID:     runID,
			CallID:    callID,
			Questions: questions,
			Timeout:   timeout,
		})
		if err != nil {
			return "", err
		}

		return tools.MarshalToolResult(map[string]any{
			"questions": questions,
			"answers":   answers,
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}
