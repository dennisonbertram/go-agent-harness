package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

type ToolManifestResolver func(profileName string) (map[string]any, error)

// GetProfileManifestTool returns a deferred tool that exposes the declared
// allowlist together with the resolved tool manifest for a named profile.
func GetProfileManifestTool(resolve ToolManifestResolver) tools.Tool {
	def := tools.Definition{
		Name:         "get_profile_manifest",
		Description:  descriptions.Load("get_profile_manifest"),
		Action:       tools.ActionRead,
		Mutating:     false,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"profiles", "tools", "manifest", "discovery"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"profile_name": map[string]any{
					"type":        "string",
					"description": "The profile name to inspect (for example 'full' or 'minimal').",
				},
			},
			"required": []string{"profile_name"},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ProfileName string `json:"profile_name"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse get_profile_manifest args: %w", err)
		}
		if strings.TrimSpace(args.ProfileName) == "" {
			return "", fmt.Errorf("get_profile_manifest: profile_name is required")
		}
		if resolve == nil {
			return "", fmt.Errorf("get_profile_manifest: resolver is not configured")
		}

		manifest, err := resolve(strings.TrimSpace(args.ProfileName))
		if err != nil {
			return "", fmt.Errorf("get_profile_manifest %q: %w", args.ProfileName, err)
		}
		return tools.MarshalToolResult(manifest)
	}

	return tools.Tool{Definition: def, Handler: handler}
}
