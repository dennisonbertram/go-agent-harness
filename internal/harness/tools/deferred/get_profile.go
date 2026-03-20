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

// GetProfileTool returns a deferred tool that fetches a single profile by name.
// profilesDir is the user-global profiles directory; pass empty string to use defaults.
func GetProfileTool(profilesDir string) tools.Tool {
	return GetProfileToolWithDirs("", profilesDir)
}

// GetProfileToolWithDirs is like GetProfileTool but accepts explicit project and user dirs.
func GetProfileToolWithDirs(projectDir, userDir string) tools.Tool {
	def := tools.Definition{
		Name:         "get_profile",
		Description:  descriptions.Load("get_profile"),
		Action:       tools.ActionRead,
		Mutating:     false,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"profiles", "agent", "subagent", "discovery"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The profile name to retrieve (e.g. 'full', 'github', 'researcher').",
				},
			},
			"required": []string{"name"},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse get_profile args: %w", err)
		}
		if strings.TrimSpace(args.Name) == "" {
			return "", fmt.Errorf("get_profile: name is required")
		}

		// Load the profile using the three-tier resolution.
		var p *profiles.Profile
		var err error
		if projectDir != "" || userDir != "" {
			p, err = profiles.LoadProfileWithDirs(args.Name, projectDir, userDir)
		} else if userDir != "" {
			p, err = profiles.LoadProfileFromUserDir(args.Name, userDir)
		} else {
			p, err = profiles.LoadProfile(args.Name)
		}
		if err != nil {
			return "", fmt.Errorf("get_profile %q: %w", args.Name, err)
		}
		if p == nil {
			return "", fmt.Errorf("get_profile: profile %q not found", args.Name)
		}

		// Determine source tier by re-checking where it was found.
		sourceTier := resolveSourceTier(args.Name, projectDir, userDir)

		response := map[string]any{
			"name":               p.Meta.Name,
			"description":        p.Meta.Description,
			"version":            p.Meta.Version,
			"model":              p.Runner.Model,
			"max_steps":          p.Runner.MaxSteps,
			"max_cost_usd":       p.Runner.MaxCostUSD,
			"allowed_tools":      p.Tools.Allow,
			"allowed_tool_count": len(p.Tools.Allow),
			"source_tier":        sourceTier,
			"created_by":         p.Meta.CreatedBy,
		}
		if p.Runner.SystemPrompt != "" {
			response["system_prompt"] = p.Runner.SystemPrompt
		}
		return tools.MarshalToolResult(response)
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// resolveSourceTier determines which tier a profile was resolved from.
func resolveSourceTier(name, projectDir, userDir string) string {
	if projectDir != "" {
		if _, err := profiles.LoadProfileWithDirs(name, projectDir, ""); err == nil {
			return "project"
		}
	}
	if userDir != "" {
		if _, err := profiles.LoadProfileWithDirs(name, "", userDir); err == nil {
			return "user"
		}
	}
	return "built-in"
}
