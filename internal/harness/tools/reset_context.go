package tools

import (
	"context"
	"encoding/json"

	"go-agent-harness/internal/harness/tools/descriptions"
)

// ResetContextToolName is the canonical name of the reset_context tool.
const ResetContextToolName = "reset_context"

// resetContextSentinelKey is the JSON key the runner checks to detect a
// context reset result. This key is only checked when the tool name is
// "reset_context" to avoid false positives.
const resetContextSentinelKey = "__reset_context__"

// ResetContextResult is the sentinel JSON object returned by the reset_context
// tool handler. The runner inspects this to perform the actual reset.
type ResetContextResult struct {
	Sentinel bool            `json:"__reset_context__"`
	Persist  json.RawMessage `json:"persist,omitempty"`
}

// IsResetContextResult returns true when toolName is "reset_context" and the
// output JSON contains the sentinel key. Callers should pass the raw tool
// output string as the second argument.
func IsResetContextResult(toolName, output string) (json.RawMessage, bool) {
	if toolName != ResetContextToolName {
		return nil, false
	}
	var r ResetContextResult
	if err := json.Unmarshal([]byte(output), &r); err != nil {
		return nil, false
	}
	if !r.Sentinel {
		return nil, false
	}
	return r.Persist, true
}

type resetContextArgs struct {
	Persist json.RawMessage `json:"persist"`
}

// ResetContextTool returns the Tool definition and handler for reset_context.
func ResetContextTool() Tool {
	return Tool{
		Definition: Definition{
			Name:        ResetContextToolName,
			Description: descriptions.Load("reset_context"),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"persist": map[string]any{
						"type":        "object",
						"description": "Free-form JSON object containing information to carry forward into the next context segment.",
					},
				},
				"required":             []string{"persist"},
				"additionalProperties": false,
			},
			Action:       ActionExecute,
			Mutating:     true,
			ParallelSafe: false,
			Tags:         []string{"context", "reset", "memory", "transcript"},
			Tier:         TierCore,
		},
		Handler: handleResetContext(),
	}
}

func handleResetContext() Handler {
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		var a resetContextArgs
		if err := json.Unmarshal(args, &a); err != nil {
			return MarshalToolResult(map[string]any{"error": "invalid arguments: " + err.Error()})
		}
		if len(a.Persist) == 0 || string(a.Persist) == "null" {
			return MarshalToolResult(map[string]any{"error": "persist is required and must be a non-null JSON object"})
		}

		// Return the sentinel result. The runner intercepts this before
		// appending it to the message list and performs the actual reset.
		result := ResetContextResult{
			Sentinel: true,
			Persist:  a.Persist,
		}
		b, err := json.Marshal(result)
		if err != nil {
			return MarshalToolResult(map[string]any{"error": "internal error marshaling reset result"})
		}
		return string(b), nil
	}
}
