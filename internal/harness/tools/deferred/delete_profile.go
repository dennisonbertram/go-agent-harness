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

// DeleteProfileTool returns a deferred tool that deletes a user-created profile
// from the given profiles directory. Built-in profiles are protected from deletion.
func DeleteProfileTool(profilesDir string) tools.Tool {
	def := tools.Definition{
		Name:         "delete_profile",
		Description:  descriptions.Load("delete_profile"),
		Action:       tools.ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierDeferred,
		Tags:         []string{"profile", "agent", "delete", "write"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the profile to delete. Must be a user-created profile, not a built-in.",
				},
			},
			"required":             []string{"name"},
			"additionalProperties": false,
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse delete_profile args: %w", err)
		}

		name := strings.TrimSpace(args.Name)
		if name == "" {
			return "", fmt.Errorf("delete_profile: name is required")
		}
		if profilesDir == "" {
			return "", fmt.Errorf("delete_profile: no profiles directory configured")
		}

		if err := profiles.DeleteProfileFromDir(name, profilesDir); err != nil {
			return "", fmt.Errorf("delete_profile: %w", err)
		}

		return tools.MarshalToolResult(map[string]any{
			"status": "deleted",
			"name":   name,
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}
