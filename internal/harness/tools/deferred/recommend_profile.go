package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
	"go-agent-harness/internal/profiles"
)

// RecommendProfileTool returns a deferred tool that recommends the best built-in profile
// for a given task using deterministic keyword heuristics. No LLM inference is performed.
func RecommendProfileTool() tools.Tool {
	def := tools.Definition{
		Name:         "recommend_profile",
		Description:  descriptions.Load("recommend_profile"),
		Action:       tools.ActionList,
		Mutating:     false,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"profile", "recommend", "agent", "subagent"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The task description to evaluate. The recommender will keyword-match this to find the best profile.",
				},
			},
			"required": []string{"task"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Task string `json:"task"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("recommend_profile: parse args: %w", err)
		}
		if strings.TrimSpace(args.Task) == "" {
			return "", fmt.Errorf("recommend_profile: task is required")
		}

		rec := profiles.RecommendProfile(args.Task)

		return tools.MarshalToolResult(map[string]any{
			"profile_name": rec.ProfileName,
			"reason":       rec.Reason,
			"confidence":   rec.Confidence,
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}
