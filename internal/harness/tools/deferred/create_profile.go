package deferred

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
	"go-agent-harness/internal/profiles"
)

// CreateProfileTool returns a deferred tool that creates a new named agent profile
// in the given profiles directory. Built-in profile names are rejected.
func CreateProfileTool(profilesDir string) tools.Tool {
	def := tools.Definition{
		Name:         "create_profile",
		Description:  descriptions.Load("create_profile"),
		Action:       tools.ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierDeferred,
		Tags:         []string{"profile", "agent", "create", "write"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Unique profile name. Kebab-case recommended (e.g. 'code-reviewer'). Must not conflict with built-in names.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Human-readable description of what this profile is for.",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "LLM model name (e.g. 'gpt-4.1-mini', 'claude-opus-4-6').",
				},
				"max_steps": map[string]any{
					"type":        "integer",
					"description": "Maximum number of steps the agent may take. Default: 30.",
				},
				"max_cost_usd": map[string]any{
					"type":        "number",
					"description": "Maximum spend in USD. Default: 2.0.",
				},
				"system_prompt": map[string]any{
					"type":        "string",
					"description": "Custom system prompt to prepend to the agent's context.",
				},
				"allowed_tools": map[string]any{
					"type":        "array",
					"description": "List of tool names the agent may use. Empty = all tools allowed.",
					"items":       map[string]any{"type": "string"},
				},
			},
			"required":             []string{"name", "description"},
			"additionalProperties": false,
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Name         string   `json:"name"`
			Description  string   `json:"description"`
			Model        string   `json:"model,omitempty"`
			MaxSteps     int      `json:"max_steps,omitempty"`
			MaxCostUSD   float64  `json:"max_cost_usd,omitempty"`
			SystemPrompt string   `json:"system_prompt,omitempty"`
			AllowedTools []string `json:"allowed_tools,omitempty"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse create_profile args: %w", err)
		}

		name := strings.TrimSpace(args.Name)
		if name == "" {
			return "", fmt.Errorf("create_profile: name is required")
		}
		if strings.TrimSpace(args.Description) == "" {
			return "", fmt.Errorf("create_profile: description is required")
		}
		if profilesDir == "" {
			return "", fmt.Errorf("create_profile: no profiles directory configured")
		}

		// Reject built-in profile names.
		if profiles.IsBuiltinProfile(name) {
			return "", fmt.Errorf("create_profile: %q is a built-in profile and cannot be created or overwritten via this tool", name)
		}

		// Validate the profile before writing.
		p := &profiles.Profile{
			Meta: profiles.ProfileMeta{
				Name:           name,
				Description:    strings.TrimSpace(args.Description),
				Version:        1,
				CreatedAt:      time.Now().UTC().Format(time.RFC3339),
				CreatedBy:      "agent",
				ReviewEligible: true,
			},
			Runner: profiles.ProfileRunner{
				Model:        args.Model,
				MaxSteps:     args.MaxSteps,
				MaxCostUSD:   args.MaxCostUSD,
				SystemPrompt: args.SystemPrompt,
			},
			Tools: profiles.ProfileTools{
				Allow: args.AllowedTools,
			},
		}
		if err := profiles.ValidateProfile(p); err != nil {
			return "", fmt.Errorf("create_profile: validation failed: %w", err)
		}

		// Ensure the directory exists.
		if err := os.MkdirAll(profilesDir, 0o755); err != nil {
			return "", fmt.Errorf("create_profile: create profiles dir: %w", err)
		}

		// Use O_EXCL to refuse overwriting an existing profile.
		path := filepath.Join(profilesDir, name+".toml")
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("create_profile: profile %q already exists", name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("create_profile: check existing profile: %w", err)
		}

		if err := profiles.SaveProfileToDir(p, profilesDir); err != nil {
			return "", fmt.Errorf("create_profile: save: %w", err)
		}

		return tools.MarshalToolResult(map[string]any{
			"status": "created",
			"name":   name,
			"path":   path,
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}
