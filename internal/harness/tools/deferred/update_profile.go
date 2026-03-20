package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
	"go-agent-harness/internal/profiles"
)

// UpdateProfileTool returns a deferred tool that updates an existing user-created
// profile in the given profiles directory. Built-in profiles are rejected.
func UpdateProfileTool(profilesDir string) tools.Tool {
	def := tools.Definition{
		Name:         "update_profile",
		Description:  descriptions.Load("update_profile"),
		Action:       tools.ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierDeferred,
		Tags:         []string{"profile", "agent", "update", "write"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the profile to update. Must exist in the user profiles directory.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "New human-readable description.",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "New LLM model name.",
				},
				"max_steps": map[string]any{
					"type":        "integer",
					"description": "New maximum step count.",
				},
				"max_cost_usd": map[string]any{
					"type":        "number",
					"description": "New cost ceiling in USD.",
				},
				"system_prompt": map[string]any{
					"type":        "string",
					"description": "New system prompt.",
				},
				"allowed_tools": map[string]any{
					"type":        "array",
					"description": "New list of allowed tool names. Empty = allow all tools.",
					"items":       map[string]any{"type": "string"},
				},
			},
			"required":             []string{"name"},
			"additionalProperties": false,
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Name         string   `json:"name"`
			Description  string   `json:"description,omitempty"`
			Model        string   `json:"model,omitempty"`
			MaxSteps     int      `json:"max_steps,omitempty"`
			MaxCostUSD   float64  `json:"max_cost_usd,omitempty"`
			SystemPrompt string   `json:"system_prompt,omitempty"`
			AllowedTools []string `json:"allowed_tools,omitempty"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse update_profile args: %w", err)
		}

		name := strings.TrimSpace(args.Name)
		if name == "" {
			return "", fmt.Errorf("update_profile: name is required")
		}
		if profilesDir == "" {
			return "", fmt.Errorf("update_profile: no profiles directory configured")
		}

		profilePath := filepath.Join(profilesDir, name+".toml")

		// Reject built-in profile names when there is no user file to update.
		if profiles.IsBuiltinProfile(name) {
			if _, err := os.Stat(profilePath); os.IsNotExist(err) {
				return "", fmt.Errorf("update_profile: %q is a built-in profile and cannot be modified", name)
			}
		}

		// Require that the user file actually exists.
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			return "", fmt.Errorf("update_profile: profile %q not found in user profiles directory", name)
		}

		// Load existing profile from the user dir only.
		existing, err := profiles.LoadProfileWithDirs(name, "", profilesDir)
		if err != nil {
			return "", fmt.Errorf("update_profile: load profile %q: %w", name, err)
		}

		// Apply updates (only override fields explicitly provided).
		if args.Description != "" {
			existing.Meta.Description = args.Description
		}
		if args.Model != "" {
			existing.Runner.Model = args.Model
		}
		if args.MaxSteps > 0 {
			existing.Runner.MaxSteps = args.MaxSteps
		}
		if args.MaxCostUSD > 0 {
			existing.Runner.MaxCostUSD = args.MaxCostUSD
		}
		if args.SystemPrompt != "" {
			existing.Runner.SystemPrompt = args.SystemPrompt
		}
		if args.AllowedTools != nil {
			existing.Tools.Allow = args.AllowedTools
		}

		// Validate the updated profile.
		if err := profiles.ValidateProfile(existing); err != nil {
			return "", fmt.Errorf("update_profile: validation failed: %w", err)
		}

		// Write back atomically using the exported SaveProfileToDir.
		if err := profiles.SaveProfileToDir(existing, profilesDir); err != nil {
			return "", fmt.Errorf("update_profile: save: %w", err)
		}

		return tools.MarshalToolResult(map[string]any{
			"status": "updated",
			"name":   name,
			"path":   profilePath,
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}
